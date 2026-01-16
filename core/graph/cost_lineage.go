// Package graph - Single authoritative CostGraph constructor
// This is the ONLY way to create a cost graph.
// All other paths are BLOCKED.
package graph

import (
	"fmt"

	"terraform-cost/core/determinism"
)

// AuthoritativeCostGraph is a cost graph that MUST derive from dependency graph
type AuthoritativeCostGraph struct {
	// REQUIRED source graphs
	depGraph   *CanonicalDependencyGraph
	assetGraph *EnforcedAssetGraph

	// Cost units with mandatory lineage
	units map[string]*AuthoritativeCostUnit

	// Symbolic buckets (for unknown cardinality)
	symbolicBuckets []*SymbolicBucket

	// State
	sealed bool
}

// AuthoritativeCostUnit is a cost unit with MANDATORY lineage
type AuthoritativeCostUnit struct {
	ID      string
	Lineage *MandatoryLineage

	MonthlyCost determinism.Money
	HourlyCost  determinism.Money
	Confidence  float64

	Components []*AuthoritativeCostComponent
}

// MandatoryLineage is REQUIRED for every cost unit
type MandatoryLineage struct {
	AssetID        string
	InstanceID     string
	DependencyPath []DependencyEdge
	ProviderKey    string
}

// Validate returns error if lineage is incomplete
func (l *MandatoryLineage) Validate() error {
	if l.AssetID == "" {
		return fmt.Errorf("AssetID is empty")
	}
	if len(l.DependencyPath) == 0 {
		return fmt.Errorf("DependencyPath is empty for %s", l.AssetID)
	}
	return nil
}

// MustBeValid panics if invalid
func (l *MandatoryLineage) MustBeValid() {
	if err := l.Validate(); err != nil {
		panic("COST UNIT WITHOUT DEPENDENCY LINEAGE: " + err.Error())
	}
}

// AuthoritativeCostComponent is a component
type AuthoritativeCostComponent struct {
	Name        string
	MonthlyCost determinism.Money
	HourlyCost  determinism.Money
	RateKey     string
	Confidence  float64
}

// SymbolicBucket for unknown cardinality
type SymbolicBucket struct {
	AssetID     string
	Reason      string
	Expression  string
	LowerBound  *determinism.Money
	UpperBound  *determinism.Money
	IsUnbounded bool
}

// BuildAuthoritativeCostGraph is the SINGLE AUTHORITATIVE CONSTRUCTOR
// This is the ONLY way to create a cost graph.
func BuildAuthoritativeCostGraph(
	depGraph *CanonicalDependencyGraph,
	assetGraph *EnforcedAssetGraph,
) (*AuthoritativeCostGraph, error) {
	// INVARIANT: depGraph is required
	if depGraph == nil {
		return nil, fmt.Errorf("INVARIANT VIOLATED: depGraph is nil")
	}

	// INVARIANT: assetGraph is required
	if assetGraph == nil {
		return nil, fmt.Errorf("INVARIANT VIOLATED: assetGraph is nil")
	}

	// INVARIANT: depGraph must be sealed
	if !depGraph.IsSealed() {
		return nil, fmt.Errorf("INVARIANT VIOLATED: depGraph is not sealed")
	}

	// INVARIANT: depGraph must be transitively closed
	if !depGraph.IsTransitivelyClosed() {
		return nil, fmt.Errorf("INVARIANT VIOLATED: dependency graph is not transitively closed")
	}

	return &AuthoritativeCostGraph{
		depGraph:        depGraph,
		assetGraph:      assetGraph,
		units:           make(map[string]*AuthoritativeCostUnit),
		symbolicBuckets: []*SymbolicBucket{},
		sealed:          false,
	}, nil
}

// AddUnit adds a cost unit with MANDATORY lineage validation
func (g *AuthoritativeCostGraph) AddUnit(unit *AuthoritativeCostUnit) error {
	if g.sealed {
		return fmt.Errorf("cannot add to sealed graph")
	}
	if unit.Lineage == nil {
		panic("COST UNIT WITHOUT LINEAGE - this is a bug")
	}
	unit.Lineage.MustBeValid()

	g.units[unit.ID] = unit
	return nil
}

// AddSymbolicBucket adds a symbolic cost for unknown cardinality
func (g *AuthoritativeCostGraph) AddSymbolicBucket(bucket *SymbolicBucket) {
	if g.sealed {
		panic("cannot add to sealed graph")
	}
	g.symbolicBuckets = append(g.symbolicBuckets, bucket)
}

// Seal seals the graph
func (g *AuthoritativeCostGraph) Seal() {
	g.sealed = true
}

// IsSealed returns seal state
func (g *AuthoritativeCostGraph) IsSealed() bool {
	return g.sealed
}

// GetDependencyGraph returns the dependency graph
func (g *AuthoritativeCostGraph) GetDependencyGraph() *CanonicalDependencyGraph {
	return g.depGraph
}

// GetAssetGraph returns the asset graph
func (g *AuthoritativeCostGraph) GetAssetGraph() *EnforcedAssetGraph {
	return g.assetGraph
}

// AllUnits returns all cost units
func (g *AuthoritativeCostGraph) AllUnits() []*AuthoritativeCostUnit {
	units := make([]*AuthoritativeCostUnit, 0, len(g.units))
	for _, u := range g.units {
		units = append(units, u)
	}
	return units
}

// GetSymbolicBuckets returns symbolic buckets
func (g *AuthoritativeCostGraph) GetSymbolicBuckets() []*SymbolicBucket {
	return g.symbolicBuckets
}

// HasSymbolicCosts returns true if there are symbolic costs
func (g *AuthoritativeCostGraph) HasSymbolicCosts() bool {
	return len(g.symbolicBuckets) > 0
}

// GetAffectedByChange returns cost units affected by dependency changes
func (g *AuthoritativeCostGraph) GetAffectedByChange(changedNodeIDs []DependencyNodeID) []*AuthoritativeCostUnit {
	affected := make(map[string]*AuthoritativeCostUnit)

	for _, nodeID := range changedNodeIDs {
		// Get transitive dependents
		dependents := g.depGraph.GetTransitiveDependents(nodeID)
		allAffected := append([]DependencyNodeID{nodeID}, dependents...)

		// Find cost units with these nodes in their path
		for _, unit := range g.units {
			for _, edge := range unit.Lineage.DependencyPath {
				for _, affectedID := range allAffected {
					if edge.From == affectedID || edge.To == affectedID {
						affected[unit.ID] = unit
						break
					}
				}
			}
		}
	}

	result := make([]*AuthoritativeCostUnit, 0, len(affected))
	for _, u := range affected {
		result = append(result, u)
	}
	return result
}

// BLOCKED CONSTRUCTORS - these panic immediately

// BuildCostGraphFromAssets is BLOCKED
func BuildCostGraphFromAssets(assets interface{}) {
	BlockBypassAttempt("BuildCostGraphFromAssets - use BuildAuthoritativeCostGraph instead")
}

// BuildCostGraphFromInstances is BLOCKED
func BuildCostGraphFromInstances(instances interface{}) {
	BlockBypassAttempt("BuildCostGraphFromInstances - use BuildAuthoritativeCostGraph instead")
}

// NewCostGraphDirect is BLOCKED
func NewCostGraphDirect() {
	BlockBypassAttempt("NewCostGraphDirect - use BuildAuthoritativeCostGraph instead")
}
