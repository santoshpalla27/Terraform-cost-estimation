// Package graph - Dependency-aware cost graph
// Cost graph MUST consume dependency graph authoritatively.
// Every CostUnit is traceable through the asset dependency chain.
package graph

import (
	"terraform-cost/core/cost"
	"terraform-cost/core/model"
)

// DependencyAwareCostGraph integrates cost with dependency semantics
type DependencyAwareCostGraph struct {
	// Infrastructure graph (authoritative)
	infra *InfrastructureGraph

	// Cost graph (derived)
	costs *cost.CostGraph

	// Mapping: instance ID → node ID
	instanceToNode map[model.InstanceID]string

	// Dependency costs: how much each node contributes to dependents
	dependencyCosts map[string]*DependencyCost
}

// DependencyCost tracks cost through dependency chains
type DependencyCost struct {
	NodeID          string
	DirectCost      float64 // This node's cost
	DependentCost   float64 // Cost of nodes depending on this
	TransitiveCost  float64 // Full transitive dependent cost
	DependencyDepth int     // How deep in dependency chain
	AffectedNodes   []string
}

// NewDependencyAwareCostGraph creates an integrated graph
func NewDependencyAwareCostGraph(infra *InfrastructureGraph, costs *cost.CostGraph) *DependencyAwareCostGraph {
	g := &DependencyAwareCostGraph{
		infra:           infra,
		costs:           costs,
		instanceToNode:  make(map[model.InstanceID]string),
		dependencyCosts: make(map[string]*DependencyCost),
	}

	// Build mappings
	g.buildMappings()

	// Calculate dependency costs
	g.calculateDependencyCosts()

	return g
}

func (g *DependencyAwareCostGraph) buildMappings() {
	for nodeID, node := range g.infra.nodes {
		if node.IsExpanded {
			// Map expanded instances to their node
			// Instance ID is derived from definition + key
			instID := model.InstanceID(nodeID)
			g.instanceToNode[instID] = nodeID
		}
	}
}

func (g *DependencyAwareCostGraph) calculateDependencyCosts() {
	// Get topological order (dependencies first)
	order, err := g.infra.TopologicalSort()
	if err != nil {
		return
	}

	// Process in reverse order (dependents first)
	for i := len(order) - 1; i >= 0; i-- {
		nodeID := order[i]
		g.calculateNodeDependencyCost(nodeID)
	}
}

func (g *DependencyAwareCostGraph) calculateNodeDependencyCost(nodeID string) {
	// Get this node's direct cost
	instID := model.InstanceID(nodeID)
	costNode := g.costs.GetNode(instID)

	directCost := 0.0
	if costNode != nil {
		directCost = costNode.TotalMonthly.Float64()
	}

	// Get dependents
	dependents := g.infra.GetDependents(nodeID)

	dependentCost := 0.0
	transitiveCost := 0.0
	maxDepth := 0
	affected := []string{}

	for _, depID := range dependents {
		affected = append(affected, depID)

		// Get dependent's cost info
		if depCost, ok := g.dependencyCosts[depID]; ok {
			dependentCost += depCost.DirectCost
			transitiveCost += depCost.DirectCost + depCost.TransitiveCost
			if depCost.DependencyDepth+1 > maxDepth {
				maxDepth = depCost.DependencyDepth + 1
			}
			affected = append(affected, depCost.AffectedNodes...)
		}
	}

	g.dependencyCosts[nodeID] = &DependencyCost{
		NodeID:          nodeID,
		DirectCost:      directCost,
		DependentCost:   dependentCost,
		TransitiveCost:  transitiveCost,
		DependencyDepth: maxDepth,
		AffectedNodes:   affected,
	}
}

// GetBlastRadius returns the cost impact if a node changes
func (g *DependencyAwareCostGraph) GetBlastRadius(nodeID string) *BlastRadius {
	depCost := g.dependencyCosts[nodeID]
	if depCost == nil {
		return nil
	}

	return &BlastRadius{
		NodeID:              nodeID,
		DirectCost:          depCost.DirectCost,
		AffectedNodesCost:   depCost.TransitiveCost,
		TotalPotentialCost:  depCost.DirectCost + depCost.TransitiveCost,
		AffectedNodesCount:  len(depCost.AffectedNodes),
		MaxDependencyDepth:  depCost.DependencyDepth,
		AffectedNodes:       depCost.AffectedNodes,
	}
}

// BlastRadius describes the cost impact of a change
type BlastRadius struct {
	NodeID              string
	DirectCost          float64
	AffectedNodesCost   float64
	TotalPotentialCost  float64
	AffectedNodesCount  int
	MaxDependencyDepth  int
	AffectedNodes       []string
}

// GetCostLineage returns the full lineage of a cost
func (g *DependencyAwareCostGraph) GetCostLineage(instID model.InstanceID) *CostLineage {
	nodeID := string(instID)
	node := g.infra.GetNode(nodeID)
	costNode := g.costs.GetNode(instID)

	if node == nil || costNode == nil {
		return nil
	}

	lineage := &CostLineage{
		InstanceID:     instID,
		InstanceAddress: string(costNode.InstanceAddress),
		ResourceType:   costNode.ResourceType,
		DirectCost:     costNode.TotalMonthly.Float64(),
		Dependencies:   []DependencyLink{},
		Dependents:     []DependencyLink{},
	}

	// Get dependencies
	for _, depID := range g.infra.GetDependencies(nodeID) {
		depInstID := model.InstanceID(depID)
		depCostNode := g.costs.GetNode(depInstID)
		cost := 0.0
		if depCostNode != nil {
			cost = depCostNode.TotalMonthly.Float64()
		}
		lineage.Dependencies = append(lineage.Dependencies, DependencyLink{
			NodeID:   depID,
			Cost:     cost,
			Relation: "depends_on",
		})
	}

	// Get dependents
	for _, depID := range g.infra.GetDependents(nodeID) {
		depInstID := model.InstanceID(depID)
		depCostNode := g.costs.GetNode(depInstID)
		cost := 0.0
		if depCostNode != nil {
			cost = depCostNode.TotalMonthly.Float64()
		}
		lineage.Dependents = append(lineage.Dependents, DependencyLink{
			NodeID:   depID,
			Cost:     cost,
			Relation: "required_by",
		})
	}

	return lineage
}

// CostLineage is the full lineage of a cost
type CostLineage struct {
	InstanceID      model.InstanceID
	InstanceAddress string
	ResourceType    string
	DirectCost      float64
	Dependencies    []DependencyLink
	Dependents      []DependencyLink
}

// DependencyLink is a link in the dependency chain
type DependencyLink struct {
	NodeID   string
	Cost     float64
	Relation string
}

// CalculateChangeCost calculates cost change for a set of changed nodes
func (g *DependencyAwareCostGraph) CalculateChangeCost(changedNodes []string) *ChangeCostAnalysis {
	analysis := &ChangeCostAnalysis{
		ChangedNodes:    changedNodes,
		DirectChanges:   []NodeCostChange{},
		IndirectChanges: []NodeCostChange{},
		TotalDirect:     0,
		TotalIndirect:   0,
	}

	affected := make(map[string]bool)

	// Process each changed node
	for _, nodeID := range changedNodes {
		affected[nodeID] = true

		// Get this node's cost
		instID := model.InstanceID(nodeID)
		costNode := g.costs.GetNode(instID)
		cost := 0.0
		if costNode != nil {
			cost = costNode.TotalMonthly.Float64()
		}

		analysis.DirectChanges = append(analysis.DirectChanges, NodeCostChange{
			NodeID: nodeID,
			Cost:   cost,
			Type:   "direct",
		})
		analysis.TotalDirect += cost

		// Get transitive dependents
		for _, depID := range g.infra.GetTransitiveDependents(nodeID) {
			if !affected[depID] {
				affected[depID] = true

				depInstID := model.InstanceID(depID)
				depCostNode := g.costs.GetNode(depInstID)
				depCost := 0.0
				if depCostNode != nil {
					depCost = depCostNode.TotalMonthly.Float64()
				}

				analysis.IndirectChanges = append(analysis.IndirectChanges, NodeCostChange{
					NodeID: depID,
					Cost:   depCost,
					Type:   "indirect",
				})
				analysis.TotalIndirect += depCost
			}
		}
	}

	return analysis
}

// ChangeCostAnalysis is the result of change cost calculation
type ChangeCostAnalysis struct {
	ChangedNodes    []string
	DirectChanges   []NodeCostChange
	IndirectChanges []NodeCostChange
	TotalDirect     float64
	TotalIndirect   float64
}

// NodeCostChange is a cost change for a node
type NodeCostChange struct {
	NodeID string
	Cost   float64
	Type   string
}
