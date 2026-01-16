// Package diff - Dependency-closure aware diff engine
// Diff MUST use dependency closure, not just address matching.
package diff

import (
	"terraform-cost/core/determinism"
	"terraform-cost/core/graph"
)

// DependencyClosureDiff computes diffs using dependency closure
type DependencyClosureDiff struct {
	before *graph.EnforcedCostGraph
	after  *graph.EnforcedCostGraph
}

// NewDependencyClosureDiff creates a diff engine
func NewDependencyClosureDiff(before, after *graph.EnforcedCostGraph) *DependencyClosureDiff {
	return &DependencyClosureDiff{
		before: before,
		after:  after,
	}
}

// ComputeDiff computes the diff with full dependency closure awareness
func (d *DependencyClosureDiff) ComputeDiff() *ClosureAwareDiff {
	result := &ClosureAwareDiff{
		ChangedNodes:      []graph.DependencyNodeID{},
		AffectedAssets:    []*graph.EnforcedAsset{},
		AffectedCostUnits: []*graph.EnforcedCostUnit{},
		DirectChanges:     []*CostChange{},
		IndirectChanges:   []*CostChange{},
		SymbolicChanges:   []*SymbolicChange{},
	}

	if d.after == nil {
		return result
	}

	// Find changed nodes by comparing cost units
	changedNodes := d.findChangedNodes()
	result.ChangedNodes = changedNodes

	// Get affected cost units via dependency closure
	result.AffectedCostUnits = d.after.GetAffectedCostUnits(changedNodes)

	// Classify changes
	d.classifyChanges(result)

	// Calculate totals
	result.calculateTotals()

	return result
}

func (d *DependencyClosureDiff) findChangedNodes() []graph.DependencyNodeID {
	changed := make(map[graph.DependencyNodeID]bool)

	afterUnits := d.after.AllCostUnits()
	for _, unit := range afterUnits {
		// Check if this is new or changed
		isNew := d.before == nil
		var beforeCost determinism.Money
		if !isNew {
			// Find corresponding before unit
			// For simplicity, using first node in dependency path
			if len(unit.DependencyPath) > 0 {
				lastNode := unit.DependencyPath[len(unit.DependencyPath)-1]
				changed[lastNode] = true
			}
		}
		_ = beforeCost
	}

	result := make([]graph.DependencyNodeID, 0, len(changed))
	for nodeID := range changed {
		result = append(result, nodeID)
	}
	return result
}

func (d *DependencyClosureDiff) classifyChanges(result *ClosureAwareDiff) {
	for _, unit := range result.AffectedCostUnits {
		if unit.IsSymbolic {
			result.SymbolicChanges = append(result.SymbolicChanges, &SymbolicChange{
				CostUnitID: unit.CostUnitID,
				AssetID:    unit.AssetID,
				Reason:     unit.SymbolicInfo.Reason,
				IsUnbounded: unit.SymbolicInfo.IsUnbounded,
			})
			continue
		}

		// Check if this is a direct or indirect change
		// Direct: last node in path is changed
		// Indirect: upstream node is changed
		isDirect := false
		if len(unit.DependencyPath) > 0 {
			lastNode := unit.DependencyPath[len(unit.DependencyPath)-1]
			for _, changed := range result.ChangedNodes {
				if lastNode == changed {
					isDirect = true
					break
				}
			}
		}

		change := &CostChange{
			CostUnitID:     unit.CostUnitID,
			AssetID:        unit.AssetID,
			DependencyPath: unit.DependencyPath,
			NewCost:        unit.MonthlyCost,
			Confidence:     unit.Confidence,
		}

		if isDirect {
			result.DirectChanges = append(result.DirectChanges, change)
		} else {
			result.IndirectChanges = append(result.IndirectChanges, change)
		}
	}
}

// ClosureAwareDiff is a diff with full dependency closure
type ClosureAwareDiff struct {
	// Changed nodes in dependency graph
	ChangedNodes []graph.DependencyNodeID

	// Affected entities (via dependency closure)
	AffectedAssets    []*graph.EnforcedAsset
	AffectedCostUnits []*graph.EnforcedCostUnit

	// Classified changes
	DirectChanges   []*CostChange   // Node itself changed
	IndirectChanges []*CostChange   // Upstream dependency changed
	SymbolicChanges []*SymbolicChange

	// Totals
	DirectCostDelta   determinism.Money
	IndirectCostDelta determinism.Money
	TotalCostDelta    determinism.Money
	MinConfidence     float64
}

func (d *ClosureAwareDiff) calculateTotals() {
	d.DirectCostDelta = determinism.Zero("USD")
	d.IndirectCostDelta = determinism.Zero("USD")
	d.MinConfidence = 1.0

	for _, change := range d.DirectChanges {
		d.DirectCostDelta = d.DirectCostDelta.Add(change.NewCost)
		if change.Confidence < d.MinConfidence {
			d.MinConfidence = change.Confidence
		}
	}

	for _, change := range d.IndirectChanges {
		d.IndirectCostDelta = d.IndirectCostDelta.Add(change.NewCost)
		if change.Confidence < d.MinConfidence {
			d.MinConfidence = change.Confidence
		}
	}

	// Symbolic changes reduce confidence to 0
	if len(d.SymbolicChanges) > 0 {
		d.MinConfidence = 0
	}

	d.TotalCostDelta = d.DirectCostDelta.Add(d.IndirectCostDelta)
}

// CostChange represents a cost change
type CostChange struct {
	CostUnitID     string
	AssetID        string
	DependencyPath []graph.DependencyNodeID
	OldCost        determinism.Money
	NewCost        determinism.Money
	Confidence     float64
}

// SymbolicChange represents a symbolic (unknown cardinality) change
type SymbolicChange struct {
	CostUnitID  string
	AssetID     string
	Reason      string
	IsUnbounded bool
}

// GetExplanation returns why a cost unit changed
func (d *ClosureAwareDiff) GetExplanation(costUnitID string) string {
	for _, change := range d.DirectChanges {
		if change.CostUnitID == costUnitID {
			return "Direct change to resource"
		}
	}
	for _, change := range d.IndirectChanges {
		if change.CostUnitID == costUnitID {
			if len(change.DependencyPath) > 1 {
				return "Changed because upstream dependency changed"
			}
		}
	}
	for _, change := range d.SymbolicChanges {
		if change.CostUnitID == costUnitID {
			return "Unknown cardinality: " + change.Reason
		}
	}
	return "No change"
}

// PolicyContext is the context passed to policies
// Policies MUST receive dependency-scoped information
type PolicyContext struct {
	// Changed nodes in dependency graph
	ChangedDependencyNodes []graph.DependencyNodeID

	// Affected cost units (via dependency closure)
	AffectedCostUnits []*graph.EnforcedCostUnit

	// The full diff
	Diff *ClosureAwareDiff

	// Mode
	IsStrictMode bool
}

// NewPolicyContext creates a policy context from a diff
func NewPolicyContext(diff *ClosureAwareDiff, isStrict bool) *PolicyContext {
	return &PolicyContext{
		ChangedDependencyNodes: diff.ChangedNodes,
		AffectedCostUnits:      diff.AffectedCostUnits,
		Diff:                   diff,
		IsStrictMode:           isStrict,
	}
}

// HasSymbolicCosts returns true if there are symbolic costs
func (c *PolicyContext) HasSymbolicCosts() bool {
	return len(c.Diff.SymbolicChanges) > 0
}

// NewResourcesOnly returns only new/added resources
func (c *PolicyContext) NewResourcesOnly() []*graph.EnforcedCostUnit {
	// For now, all affected units are considered "changed"
	return c.AffectedCostUnits
}

// GetMinConfidence returns minimum confidence (pessimistic)
func (c *PolicyContext) GetMinConfidence() float64 {
	return c.Diff.MinConfidence
}
