// Package diff provides instance-level cost diffing.
// Compares two cost estimation results at the instance level.
package diff

import (
	"sort"

	"terraform-cost/core/cost"
	"terraform-cost/core/determinism"
	"terraform-cost/core/model"
)

// DiffResult is the complete diff between two estimation results
type DiffResult struct {
	// Overall summary
	TotalBefore    determinism.Money
	TotalAfter     determinism.Money
	TotalDelta     determinism.Money
	DeltaPercent   float64

	// Instance-level changes
	Added      []*InstanceDiff
	Removed    []*InstanceDiff
	Changed    []*InstanceDiff
	Unchanged  []*InstanceDiff

	// Counts
	AddedCount    int
	RemovedCount  int
	ChangedCount  int
	UnchangedCount int

	// Confidence impact
	ConfidenceBefore float64
	ConfidenceAfter  float64
}

// InstanceDiff describes changes to a single instance
type InstanceDiff struct {
	// Identity
	Identity *model.InstanceIdentity
	Address  model.CanonicalAddress

	// Change type
	ChangeType ChangeType

	// Costs
	Before *cost.ConfidenceBoundCost
	After  *cost.ConfidenceBoundCost
	Delta  determinism.Money

	// Component-level changes
	ComponentDiffs []*ComponentDiff

	// What drove the change
	ChangeReasons []ChangeReason
}

// ChangeType indicates the type of change
type ChangeType int

const (
	ChangeAdded    ChangeType = iota // New instance
	ChangeRemoved                     // Instance removed
	ChangeModified                    // Instance cost changed
	ChangeUnchanged                   // No cost change
)

// String returns the change type name
func (c ChangeType) String() string {
	switch c {
	case ChangeAdded:
		return "added"
	case ChangeRemoved:
		return "removed"
	case ChangeModified:
		return "modified"
	case ChangeUnchanged:
		return "unchanged"
	default:
		return "unknown"
	}
}

// ComponentDiff describes changes to a cost component
type ComponentDiff struct {
	ComponentName string
	ChangeType    ChangeType

	Before *cost.CostWithProvenance
	After  *cost.CostWithProvenance
	Delta  determinism.Money

	// What changed
	RateChanged   bool
	UsageChanged  bool
	OldRate       string
	NewRate       string
	OldUsage      float64
	NewUsage      float64
}

// ChangeReason explains why a cost changed
type ChangeReason struct {
	Category string // "rate", "usage", "quantity", "configuration"
	What     string // What changed
	Impact   determinism.Money
}

// Differ computes diffs between estimation results
type Differ struct {
	// Threshold for "unchanged" (e.g., 0.01 = 1%)
	ChangeThreshold float64
}

// NewDiffer creates a new differ
func NewDiffer(changeThreshold float64) *Differ {
	if changeThreshold <= 0 {
		changeThreshold = 0.001 // 0.1% default
	}
	return &Differ{ChangeThreshold: changeThreshold}
}

// Diff computes the diff between before and after
func (d *Differ) Diff(before, after *cost.AggregatedCostResult) *DiffResult {
	result := &DiffResult{
		TotalBefore:      before.TotalMonthly,
		TotalAfter:       after.TotalMonthly,
		ConfidenceBefore: before.TotalConfidence,
		ConfidenceAfter:  after.TotalConfidence,
		Added:            []*InstanceDiff{},
		Removed:          []*InstanceDiff{},
		Changed:          []*InstanceDiff{},
		Unchanged:        []*InstanceDiff{},
	}

	// Calculate delta
	result.TotalDelta = after.TotalMonthly.Sub(before.TotalMonthly)
	if !before.TotalMonthly.IsZero() {
		result.DeltaPercent = (after.TotalMonthly.Float64() - before.TotalMonthly.Float64()) / before.TotalMonthly.Float64() * 100
	}

	// Index before instances
	beforeMap := make(map[model.CanonicalAddress]*cost.InstanceCostResult)
	for _, inst := range before.Instances {
		beforeMap[inst.Identity.Canonical] = inst
	}

	// Index after instances
	afterMap := make(map[model.CanonicalAddress]*cost.InstanceCostResult)
	for _, inst := range after.Instances {
		afterMap[inst.Identity.Canonical] = inst
	}

	// Find added, changed, unchanged
	for addr, afterInst := range afterMap {
		beforeInst, existed := beforeMap[addr]

		if !existed {
			// Added
			diff := d.createInstanceDiff(nil, afterInst, ChangeAdded)
			result.Added = append(result.Added, diff)
			result.AddedCount++
		} else {
			// Existed before - check if changed
			diff := d.compareInstances(beforeInst, afterInst)
			if diff.ChangeType == ChangeModified {
				result.Changed = append(result.Changed, diff)
				result.ChangedCount++
			} else {
				result.Unchanged = append(result.Unchanged, diff)
				result.UnchangedCount++
			}
		}
	}

	// Find removed
	for addr, beforeInst := range beforeMap {
		if _, exists := afterMap[addr]; !exists {
			diff := d.createInstanceDiff(beforeInst, nil, ChangeRemoved)
			result.Removed = append(result.Removed, diff)
			result.RemovedCount++
		}
	}

	// Sort all lists by address for determinism
	d.sortDiffs(result.Added)
	d.sortDiffs(result.Removed)
	d.sortDiffs(result.Changed)
	d.sortDiffs(result.Unchanged)

	return result
}

func (d *Differ) createInstanceDiff(before, after *cost.InstanceCostResult, changeType ChangeType) *InstanceDiff {
	diff := &InstanceDiff{
		ChangeType:     changeType,
		ComponentDiffs: []*ComponentDiff{},
		ChangeReasons:  []ChangeReason{},
	}

	switch changeType {
	case ChangeAdded:
		diff.Identity = after.Identity
		diff.Address = after.Identity.Canonical
		diff.After = after.Total
		diff.Delta = after.Total.Monthly
		diff.ChangeReasons = append(diff.ChangeReasons, ChangeReason{
			Category: "quantity",
			What:     "new instance",
			Impact:   after.Total.Monthly,
		})

	case ChangeRemoved:
		diff.Identity = before.Identity
		diff.Address = before.Identity.Canonical
		diff.Before = before.Total
		diff.Delta = determinism.Zero("USD").Sub(before.Total.Monthly)
		diff.ChangeReasons = append(diff.ChangeReasons, ChangeReason{
			Category: "quantity",
			What:     "instance removed",
			Impact:   diff.Delta,
		})
	}

	return diff
}

func (d *Differ) compareInstances(before, after *cost.InstanceCostResult) *InstanceDiff {
	diff := &InstanceDiff{
		Identity:       after.Identity,
		Address:        after.Identity.Canonical,
		Before:         before.Total,
		After:          after.Total,
		Delta:          after.Total.Monthly.Sub(before.Total.Monthly),
		ComponentDiffs: []*ComponentDiff{},
		ChangeReasons:  []ChangeReason{},
	}

	// Check if cost changed significantly
	beforeCost := before.Total.Monthly.Float64()
	afterCost := after.Total.Monthly.Float64()

	if beforeCost == 0 && afterCost == 0 {
		diff.ChangeType = ChangeUnchanged
		return diff
	}

	var percentChange float64
	if beforeCost > 0 {
		percentChange = (afterCost - beforeCost) / beforeCost
	} else {
		percentChange = 1.0 // Infinite increase from 0
	}

	if abs(percentChange) <= d.ChangeThreshold {
		diff.ChangeType = ChangeUnchanged
		return diff
	}

	diff.ChangeType = ChangeModified

	// Compare components
	beforeComponents := make(map[string]*cost.CostWithProvenance)
	for _, c := range before.Components {
		beforeComponents[c.Component] = c
	}

	for _, afterComp := range after.Components {
		beforeComp, existed := beforeComponents[afterComp.Component]

		compDiff := &ComponentDiff{
			ComponentName: afterComp.Component,
			After:         afterComp,
		}

		if !existed {
			compDiff.ChangeType = ChangeAdded
			compDiff.Delta = afterComp.Cost.Monthly
			diff.ChangeReasons = append(diff.ChangeReasons, ChangeReason{
				Category: "configuration",
				What:     "new component: " + afterComp.Component,
				Impact:   afterComp.Cost.Monthly,
			})
		} else {
			compDiff.Before = beforeComp
			compDiff.Delta = afterComp.Cost.Monthly.Sub(beforeComp.Cost.Monthly)

			if !compDiff.Delta.IsZero() {
				compDiff.ChangeType = ChangeModified

				// What changed?
				if beforeComp.Rate != nil && afterComp.Rate != nil {
					if beforeComp.Rate.Price != afterComp.Rate.Price {
						compDiff.RateChanged = true
						compDiff.OldRate = beforeComp.Rate.Price
						compDiff.NewRate = afterComp.Rate.Price
						diff.ChangeReasons = append(diff.ChangeReasons, ChangeReason{
							Category: "rate",
							What:     afterComp.Component + " rate changed",
							Impact:   compDiff.Delta,
						})
					}
				}

				if beforeComp.Usage != nil && afterComp.Usage != nil {
					if beforeComp.Usage.Value != afterComp.Usage.Value {
						compDiff.UsageChanged = true
						compDiff.OldUsage = beforeComp.Usage.Value
						compDiff.NewUsage = afterComp.Usage.Value
						diff.ChangeReasons = append(diff.ChangeReasons, ChangeReason{
							Category: "usage",
							What:     afterComp.Component + " usage changed",
							Impact:   compDiff.Delta,
						})
					}
				}
			} else {
				compDiff.ChangeType = ChangeUnchanged
			}
		}

		diff.ComponentDiffs = append(diff.ComponentDiffs, compDiff)
	}

	// Check for removed components
	for name, beforeComp := range beforeComponents {
		found := false
		for _, afterComp := range after.Components {
			if afterComp.Component == name {
				found = true
				break
			}
		}
		if !found {
			compDiff := &ComponentDiff{
				ComponentName: name,
				ChangeType:    ChangeRemoved,
				Before:        beforeComp,
				Delta:         determinism.Zero("USD").Sub(beforeComp.Cost.Monthly),
			}
			diff.ComponentDiffs = append(diff.ComponentDiffs, compDiff)
			diff.ChangeReasons = append(diff.ChangeReasons, ChangeReason{
				Category: "configuration",
				What:     "component removed: " + name,
				Impact:   compDiff.Delta,
			})
		}
	}

	return diff
}

func (d *Differ) sortDiffs(diffs []*InstanceDiff) {
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Address < diffs[j].Address
	})
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// DiffSummary provides a human-readable summary
func (r *DiffResult) Summary() string {
	var summary string

	// Overall change
	if r.TotalDelta.IsZero() {
		summary = "No cost change\n"
	} else if r.TotalDelta.IsNegative() {
		summary = "Cost decreased by " + r.TotalDelta.String() + "\n"
	} else {
		summary = "Cost increased by " + r.TotalDelta.String() + "\n"
	}

	// Instance changes
	if r.AddedCount > 0 {
		summary += "  + " + string(rune('0'+r.AddedCount)) + " instances added\n"
	}
	if r.RemovedCount > 0 {
		summary += "  - " + string(rune('0'+r.RemovedCount)) + " instances removed\n"
	}
	if r.ChangedCount > 0 {
		summary += "  ~ " + string(rune('0'+r.ChangedCount)) + " instances changed\n"
	}

	return summary
}

// TopChanges returns the instances with largest cost impact
func (r *DiffResult) TopChanges(n int) []*InstanceDiff {
	all := append(r.Added, r.Removed...)
	all = append(all, r.Changed...)

	sort.Slice(all, func(i, j int) bool {
		iAbs := abs(all[i].Delta.Float64())
		jAbs := abs(all[j].Delta.Float64())
		return iAbs > jAbs
	})

	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}
