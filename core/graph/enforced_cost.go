// Package graph - Enforced cost units with dependency path
// Every CostUnit MUST carry its dependency path.
// If the path cannot be constructed, estimation is blocked.
package graph

import (
	"fmt"

	"terraform-cost/core/determinism"
)

// EnforcedCostUnit represents a single unit of cost with REQUIRED dependency lineage
type EnforcedCostUnit struct {
	// Identity
	CostUnitID string
	AssetID    string

	// REQUIRED: Dependency path from root to this cost unit
	DependencyPath []DependencyNodeID

	// Cost values
	MonthlyCost determinism.Money
	HourlyCost  determinism.Money

	// Components
	Components []*EnforcedCostComponent

	// Confidence
	Confidence float64

	// Symbolic (for unknown cardinality)
	IsSymbolic   bool
	SymbolicInfo *SymbolicInfo
}

// EnforcedCostComponent is a component of a cost unit
type EnforcedCostComponent struct {
	Name        string
	MonthlyCost determinism.Money
	HourlyCost  determinism.Money
	Confidence  float64
}

// SymbolicInfo represents cost when cardinality is unknown
type SymbolicInfo struct {
	Reason           string
	MinCost          *determinism.Money
	MaxCost          *determinism.Money
	IsUnbounded      bool
	Expression       string
	CardinalityState CardinalityStateType
}

// CardinalityStateType indicates the cardinality knowledge state
type CardinalityStateType int

const (
	CardinalityKnown   CardinalityStateType = iota
	CardinalityUnknown
	CardinalityRange
)

// String returns the state name
func (s CardinalityStateType) String() string {
	names := []string{"known", "unknown", "range"}
	if int(s) < len(names) {
		return names[s]
	}
	return "invalid"
}

// NewEnforcedCostUnit creates a cost unit with REQUIRED dependency path
func NewEnforcedCostUnit(
	costUnitID string,
	asset *EnforcedAsset,
) *EnforcedCostUnit {
	// ASSERTION: Asset must have dependency lineage
	if err := asset.ValidateLineage(); err != nil {
		panic(err.Error())
	}

	return &EnforcedCostUnit{
		CostUnitID:     costUnitID,
		AssetID:        asset.AssetID,
		DependencyPath: asset.UpstreamDeps,
		MonthlyCost:    determinism.Zero("USD"),
		HourlyCost:     determinism.Zero("USD"),
		Components:     []*EnforcedCostComponent{},
		Confidence:     1.0,
		IsSymbolic:     false,
	}
}

// NewSymbolicCostUnit creates a cost unit for unknown cardinality
func NewSymbolicCostUnit(
	costUnitID string,
	asset *EnforcedAsset,
	reason string,
	state CardinalityStateType,
) *EnforcedCostUnit {
	if err := asset.ValidateLineage(); err != nil {
		panic(err.Error())
	}

	return &EnforcedCostUnit{
		CostUnitID:     costUnitID,
		AssetID:        asset.AssetID,
		DependencyPath: asset.UpstreamDeps,
		MonthlyCost:    determinism.Zero("USD"),
		HourlyCost:     determinism.Zero("USD"),
		Confidence:     0.0,
		IsSymbolic:     true,
		SymbolicInfo: &SymbolicInfo{
			Reason:           reason,
			IsUnbounded:      state == CardinalityUnknown,
			CardinalityState: state,
		},
	}
}

// ValidateDependencyPath ensures the cost unit has a dependency path
func (c *EnforcedCostUnit) ValidateDependencyPath() error {
	if len(c.DependencyPath) == 0 {
		return fmt.Errorf("INVARIANT VIOLATED: CostUnit %s has no dependency path", c.CostUnitID)
	}
	return nil
}

// EnforcedCostGraph is a cost graph with REQUIRED dependency lineage
type EnforcedCostGraph struct {
	assetGraph    *EnforcedAssetGraph
	costUnits     map[string]*EnforcedCostUnit
	assetToCosts  map[string][]*EnforcedCostUnit
	nodeToCosts   map[DependencyNodeID][]*EnforcedCostUnit
	symbolicCosts []*EnforcedCostUnit
}

// NewEnforcedCostGraph creates a cost graph from an asset graph
func NewEnforcedCostGraph(assetGraph *EnforcedAssetGraph) (*EnforcedCostGraph, error) {
	if assetGraph == nil {
		return nil, fmt.Errorf("INVARIANT VIOLATED: cannot create cost graph without asset graph")
	}

	return &EnforcedCostGraph{
		assetGraph:    assetGraph,
		costUnits:     make(map[string]*EnforcedCostUnit),
		assetToCosts:  make(map[string][]*EnforcedCostUnit),
		nodeToCosts:   make(map[DependencyNodeID][]*EnforcedCostUnit),
		symbolicCosts: []*EnforcedCostUnit{},
	}, nil
}

// AddCostUnit adds a cost unit with dependency validation
func (g *EnforcedCostGraph) AddCostUnit(unit *EnforcedCostUnit) error {
	if err := unit.ValidateDependencyPath(); err != nil {
		panic(err.Error())
	}

	g.costUnits[unit.CostUnitID] = unit
	g.assetToCosts[unit.AssetID] = append(g.assetToCosts[unit.AssetID], unit)

	for _, nodeID := range unit.DependencyPath {
		g.nodeToCosts[nodeID] = append(g.nodeToCosts[nodeID], unit)
	}

	if unit.IsSymbolic {
		g.symbolicCosts = append(g.symbolicCosts, unit)
	}

	return nil
}

// GetAffectedCostUnits returns cost units affected by changes to specified nodes
func (g *EnforcedCostGraph) GetAffectedCostUnits(changedNodes []DependencyNodeID) []*EnforcedCostUnit {
	affected := make(map[string]*EnforcedCostUnit)
	canonical := g.assetGraph.GetCanonicalGraph()

	for _, nodeID := range changedNodes {
		for _, unit := range g.nodeToCosts[nodeID] {
			affected[unit.CostUnitID] = unit
		}

		dependents := canonical.GetTransitiveDependents(nodeID)
		for _, depID := range dependents {
			for _, unit := range g.nodeToCosts[depID] {
				affected[unit.CostUnitID] = unit
			}
		}
	}

	result := make([]*EnforcedCostUnit, 0, len(affected))
	for _, unit := range affected {
		result = append(result, unit)
	}
	return result
}

// GetSymbolicCosts returns all symbolic costs
func (g *EnforcedCostGraph) GetSymbolicCosts() []*EnforcedCostUnit {
	return g.symbolicCosts
}

// HasSymbolicCosts returns true if there are symbolic costs
func (g *EnforcedCostGraph) HasSymbolicCosts() bool {
	return len(g.symbolicCosts) > 0
}

// AllCostUnits returns all cost units
func (g *EnforcedCostGraph) AllCostUnits() []*EnforcedCostUnit {
	result := make([]*EnforcedCostUnit, 0, len(g.costUnits))
	for _, unit := range g.costUnits {
		result = append(result, unit)
	}
	return result
}

// GetTotalCost returns total cost (excludes symbolic)
func (g *EnforcedCostGraph) GetTotalCost() determinism.Money {
	total := determinism.Zero("USD")
	for _, unit := range g.costUnits {
		if !unit.IsSymbolic {
			total = total.Add(unit.MonthlyCost)
		}
	}
	return total
}

// GetMinConfidence returns minimum confidence (pessimistic)
func (g *EnforcedCostGraph) GetMinConfidence() float64 {
	min := 1.0
	for _, unit := range g.costUnits {
		if unit.Confidence < min {
			min = unit.Confidence
		}
	}
	return min
}
