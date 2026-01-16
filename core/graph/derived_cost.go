// Package graph - Derived cost graph
// Costs MUST be derived from the dependency graph.
// This is not optional - diffs are meaningless without it.
package graph

import (
	"terraform-cost/core/determinism"
)

// DerivedCostGraph is a cost graph that MUST be derived from a dependency graph
type DerivedCostGraph struct {
	// Source dependency graph (required)
	sourceGraph *InfrastructureGraph

	// Cost nodes indexed by address
	nodes map[string]*DerivedCostNode

	// Dependency edges (from → to) with cost impact
	edges map[string][]*CostEdge

	// Symbolic costs (unknown cardinality)
	symbolicCosts map[string]*SymbolicCost
}

// DerivedCostNode is a cost node with dependency lineage
type DerivedCostNode struct {
	Address         string
	InfraNodeID     string
	DependencyDepth int

	MonthlyCost     determinism.Money
	HourlyCost      determinism.Money
	DirectCost      determinism.Money
	TransitiveCost  determinism.Money
	BlastRadiusCost determinism.Money

	DependsOn  []string
	DependedBy []string

	Confidence float64
}

// CostEdge is a cost-aware dependency edge
type CostEdge struct {
	From     string
	To       string
	CostFrom determinism.Money
	CostTo   determinism.Money
	Relation string
}

// SymbolicCost represents cost for unknown cardinality
type SymbolicCost struct {
	Address      string
	Expression   string
	MinInstances int
	MaxInstances int
	CostPerUnit  determinism.Money
	MinCost      determinism.Money
	MaxCost      determinism.Money
	IsUnbounded  bool
	Confidence   float64
	Warning      string
}

// NewDerivedCostGraph creates a cost graph from a dependency graph
// THIS IS THE ONLY WAY TO CREATE A COST GRAPH
func NewDerivedCostGraph(depGraph *InfrastructureGraph) (*DerivedCostGraph, error) {
	if depGraph == nil {
		return nil, &NoDependencyGraphError{}
	}

	g := &DerivedCostGraph{
		sourceGraph:   depGraph,
		nodes:         make(map[string]*DerivedCostNode),
		edges:         make(map[string][]*CostEdge),
		symbolicCosts: make(map[string]*SymbolicCost),
	}

	// Initialize nodes from dependency graph
	for nodeID := range depGraph.nodes {
		g.nodes[nodeID] = &DerivedCostNode{
			Address:         nodeID,
			InfraNodeID:     nodeID,
			DependencyDepth: 0, // Computed later
			DependsOn:       depGraph.GetDependencies(nodeID),
			DependedBy:      depGraph.GetDependents(nodeID),
			MonthlyCost:     determinism.Zero("USD"),
			HourlyCost:      determinism.Zero("USD"),
			DirectCost:      determinism.Zero("USD"),
			TransitiveCost:  determinism.Zero("USD"),
			BlastRadiusCost: determinism.Zero("USD"),
			Confidence:      1.0,
		}
	}

	// Create cost edges from dependency edges
	for nodeID := range g.nodes {
		for _, depID := range depGraph.GetDependencies(nodeID) {
			edge := &CostEdge{
				From:     nodeID,
				To:       depID,
				CostFrom: determinism.Zero("USD"),
				CostTo:   determinism.Zero("USD"),
				Relation: "depends_on",
			}
			g.edges[nodeID] = append(g.edges[nodeID], edge)
		}
	}

	return g, nil
}

// NoDependencyGraphError indicates cost graph was created without dependency graph
type NoDependencyGraphError struct{}

func (e *NoDependencyGraphError) Error() string {
	return "cost graph MUST be derived from dependency graph"
}

// SetNodeCost sets the cost for a node
func (g *DerivedCostGraph) SetNodeCost(address string, monthly, hourly determinism.Money, confidence float64) error {
	node, ok := g.nodes[address]
	if !ok {
		return &NodeNotInGraphError{Address: address}
	}

	node.MonthlyCost = monthly
	node.HourlyCost = hourly
	node.DirectCost = monthly
	node.Confidence = confidence

	for _, edge := range g.edges[address] {
		edge.CostFrom = monthly
	}

	return nil
}

// NodeNotInGraphError indicates a node doesn't exist
type NodeNotInGraphError struct {
	Address string
}

func (e *NodeNotInGraphError) Error() string {
	return "node " + e.Address + " not in dependency graph"
}

// AddSymbolicCost adds a symbolic cost for unknown cardinality
func (g *DerivedCostGraph) AddSymbolicCost(address string, costPerUnit determinism.Money, minInst, maxInst int, expr string) {
	minCost := costPerUnit.MulFloat(float64(minInst))

	maxCost := determinism.Zero("USD")
	isUnbounded := maxInst < 0
	if !isUnbounded {
		maxCost = costPerUnit.MulFloat(float64(maxInst))
	}

	warning := ""
	if isUnbounded {
		warning = "cardinality is unbounded"
	} else if maxInst > minInst {
		warning = "cardinality is uncertain - cost is a range"
	}

	g.symbolicCosts[address] = &SymbolicCost{
		Address:      address,
		Expression:   expr,
		MinInstances: minInst,
		MaxInstances: maxInst,
		CostPerUnit:  costPerUnit,
		MinCost:      minCost,
		MaxCost:      maxCost,
		IsUnbounded:  isUnbounded,
		Confidence:   0.3,
		Warning:      warning,
	}
}

// CalculateTransitiveCosts calculates costs through the dependency chain
func (g *DerivedCostGraph) CalculateTransitiveCosts() {
	order, err := g.sourceGraph.TopologicalSort()
	if err != nil {
		return
	}

	for i := len(order) - 1; i >= 0; i-- {
		nodeID := order[i]
		node := g.nodes[nodeID]
		if node == nil {
			continue
		}

		transitive := determinism.Zero("USD")
		for _, depByID := range node.DependedBy {
			if depBy := g.nodes[depByID]; depBy != nil {
				transitive = transitive.Add(depBy.DirectCost)
				transitive = transitive.Add(depBy.TransitiveCost)
			}
		}
		node.TransitiveCost = transitive
		node.BlastRadiusCost = node.DirectCost.Add(transitive)
	}
}

// GetChangeImpact calculates the cost impact of changing nodes
func (g *DerivedCostGraph) GetChangeImpact(changedAddresses []string) *CostChangeImpact {
	impact := &CostChangeImpact{
		DirectCost:       determinism.Zero("USD"),
		IndirectCost:     determinism.Zero("USD"),
		TotalCost:        determinism.Zero("USD"),
		AffectedNodes:    []string{},
		DependencyChains: [][]string{},
	}

	affected := make(map[string]bool)

	for _, addr := range changedAddresses {
		node := g.nodes[addr]
		if node == nil {
			continue
		}

		impact.DirectCost = impact.DirectCost.Add(node.DirectCost)
		affected[addr] = true

		dependents := g.sourceGraph.GetTransitiveDependents(addr)
		chain := []string{addr}
		for _, dep := range dependents {
			if !affected[dep] {
				affected[dep] = true
				if depNode := g.nodes[dep]; depNode != nil {
					impact.IndirectCost = impact.IndirectCost.Add(depNode.DirectCost)
				}
				chain = append(chain, dep)
			}
		}
		impact.DependencyChains = append(impact.DependencyChains, chain)
	}

	for addr := range affected {
		impact.AffectedNodes = append(impact.AffectedNodes, addr)
	}

	impact.TotalCost = impact.DirectCost.Add(impact.IndirectCost)
	return impact
}

// CostChangeImpact describes the cost impact of changes
type CostChangeImpact struct {
	DirectCost       determinism.Money
	IndirectCost     determinism.Money
	TotalCost        determinism.Money
	AffectedNodes    []string
	DependencyChains [][]string
}

// GetSymbolicCosts returns all symbolic costs
func (g *DerivedCostGraph) GetSymbolicCosts() []*SymbolicCost {
	result := make([]*SymbolicCost, 0, len(g.symbolicCosts))
	for _, sc := range g.symbolicCosts {
		result = append(result, sc)
	}
	return result
}

// HasUnboundedCosts returns true if any costs are unbounded
func (g *DerivedCostGraph) HasUnboundedCosts() bool {
	for _, sc := range g.symbolicCosts {
		if sc.IsUnbounded {
			return true
		}
	}
	return false
}

// GetTotalCostRange returns the total cost as a range
func (g *DerivedCostGraph) GetTotalCostRange() *CostBounds {
	minTotal := determinism.Zero("USD")
	maxTotal := determinism.Zero("USD")
	hasUnbounded := false

	for _, node := range g.nodes {
		minTotal = minTotal.Add(node.DirectCost)
		maxTotal = maxTotal.Add(node.DirectCost)
	}

	for _, sc := range g.symbolicCosts {
		minTotal = minTotal.Add(sc.MinCost)
		if sc.IsUnbounded {
			hasUnbounded = true
		} else {
			maxTotal = maxTotal.Add(sc.MaxCost)
		}
	}

	return &CostBounds{
		Min:         minTotal,
		Max:         maxTotal,
		IsUnbounded: hasUnbounded,
	}
}

// CostBounds represents a cost range for uncertain cardinality
type CostBounds struct {
	Min         determinism.Money
	Max         determinism.Money
	IsUnbounded bool
}

// String returns the range as a string
func (r *CostBounds) String() string {
	if r.IsUnbounded {
		return r.Min.String() + " - ∞"
	}
	if r.Min.Cmp(r.Max) == 0 {
		return r.Min.String()
	}
	return r.Min.String() + " - " + r.Max.String()
}
