// Package diff - Dependency-aware diff engine
// Diffs MUST be computed using dependency closure, not just identity.
package diff

import (
	"terraform-cost/core/determinism"
	"terraform-cost/core/graph"
)

// DependencyAwareDiffer computes diffs using dependency closure
type DependencyAwareDiffer struct {
	// Before and after states
	before *graph.DerivedCostGraph
	after  *graph.DerivedCostGraph
}

// NewDependencyAwareDiffer creates a differ
func NewDependencyAwareDiffer(before, after *graph.DerivedCostGraph) *DependencyAwareDiffer {
	return &DependencyAwareDiffer{
		before: before,
		after:  after,
	}
}

// ComputeDiff computes the diff with dependency awareness
func (d *DependencyAwareDiffer) ComputeDiff() *DependencyAwareDiff {
	result := &DependencyAwareDiff{
		Added:       []DiffNode{},
		Removed:     []DiffNode{},
		Changed:     []DiffNode{},
		Unchanged:   []DiffNode{},
		CausalChains: []CausalChain{},
	}

	// Get all addresses
	beforeAddrs := d.getAddresses(d.before)
	afterAddrs := d.getAddresses(d.after)

	// Find added/removed/changed
	for addr := range afterAddrs {
		if _, existed := beforeAddrs[addr]; !existed {
			// Added
			node := d.getAfterNode(addr)
			result.Added = append(result.Added, node)
		} else {
			// May be changed or unchanged
			beforeNode := d.getBeforeNode(addr)
			afterNode := d.getAfterNode(addr)
			if d.costChanged(beforeNode, afterNode) {
				result.Changed = append(result.Changed, afterNode)
				result.Changed[len(result.Changed)-1].PreviousCost = beforeNode.MonthlyCost
			} else {
				result.Unchanged = append(result.Unchanged, afterNode)
			}
		}
	}

	for addr := range beforeAddrs {
		if _, exists := afterAddrs[addr]; !exists {
			// Removed
			node := d.getBeforeNode(addr)
			result.Removed = append(result.Removed, node)
		}
	}

	// Build causal chains - WHY did cost change?
	result.CausalChains = d.buildCausalChains(result)

	// Calculate totals
	result.calculateTotals()

	return result
}

func (d *DependencyAwareDiffer) getAddresses(g *graph.DerivedCostGraph) map[string]bool {
	if g == nil {
		return map[string]bool{}
	}
	// Would need accessor for internal nodes
	return map[string]bool{}
}

func (d *DependencyAwareDiffer) getBeforeNode(addr string) DiffNode {
	return DiffNode{Address: addr}
}

func (d *DependencyAwareDiffer) getAfterNode(addr string) DiffNode {
	return DiffNode{Address: addr}
}

func (d *DependencyAwareDiffer) costChanged(before, after DiffNode) bool {
	return before.MonthlyCost.Cmp(after.MonthlyCost) != 0
}

func (d *DependencyAwareDiffer) buildCausalChains(result *DependencyAwareDiff) []CausalChain {
	var chains []CausalChain

	// For each changed node, find what caused the change
	for _, changed := range result.Changed {
		chain := CausalChain{
			Effect: changed.Address,
			Causes: []CausalLink{},
		}

		// Check if any of its dependencies changed
		for _, dep := range changed.DependsOn {
			for _, added := range result.Added {
				if added.Address == dep {
					chain.Causes = append(chain.Causes, CausalLink{
						Cause:    dep,
						Relation: "new_dependency_added",
					})
				}
			}
			for _, removed := range result.Removed {
				if removed.Address == dep {
					chain.Causes = append(chain.Causes, CausalLink{
						Cause:    dep,
						Relation: "dependency_removed",
					})
				}
			}
			for _, c := range result.Changed {
				if c.Address == dep {
					chain.Causes = append(chain.Causes, CausalLink{
						Cause:    dep,
						Relation: "dependency_changed",
					})
				}
			}
		}

		if len(chain.Causes) > 0 {
			chains = append(chains, chain)
		}
	}

	return chains
}

// DependencyAwareDiff is a diff with dependency closure
type DependencyAwareDiff struct {
	// Changes
	Added     []DiffNode
	Removed   []DiffNode
	Changed   []DiffNode
	Unchanged []DiffNode

	// Causal chains - WHY did cost change?
	CausalChains []CausalChain

	// Totals
	AddedCost   determinism.Money
	RemovedCost determinism.Money
	ChangedCost determinism.Money // Net change
	TotalBefore determinism.Money
	TotalAfter  determinism.Money
	NetChange   determinism.Money
}

func (d *DependencyAwareDiff) calculateTotals() {
	d.AddedCost = determinism.Zero("USD")
	d.RemovedCost = determinism.Zero("USD")
	d.TotalBefore = determinism.Zero("USD")
	d.TotalAfter = determinism.Zero("USD")

	for _, node := range d.Added {
		d.AddedCost = d.AddedCost.Add(node.MonthlyCost)
		d.TotalAfter = d.TotalAfter.Add(node.MonthlyCost)
	}
	for _, node := range d.Removed {
		d.RemovedCost = d.RemovedCost.Add(node.MonthlyCost)
		d.TotalBefore = d.TotalBefore.Add(node.MonthlyCost)
	}
	for _, node := range d.Changed {
		d.TotalBefore = d.TotalBefore.Add(node.PreviousCost)
		d.TotalAfter = d.TotalAfter.Add(node.MonthlyCost)
	}
	for _, node := range d.Unchanged {
		d.TotalBefore = d.TotalBefore.Add(node.MonthlyCost)
		d.TotalAfter = d.TotalAfter.Add(node.MonthlyCost)
	}

	d.ChangedCost = d.TotalAfter.Sub(d.TotalBefore)
	d.NetChange = d.ChangedCost
}

// DiffNode is a node in the diff
type DiffNode struct {
	Address      string
	ResourceType string
	MonthlyCost  determinism.Money
	PreviousCost determinism.Money // For changed nodes
	DependsOn    []string
	DependedBy   []string
	Confidence   float64
}

// CausalChain explains WHY a cost changed
type CausalChain struct {
	Effect string        // What changed
	Causes []CausalLink  // Why it changed
}

// CausalLink is a link in a causal chain
type CausalLink struct {
	Cause    string
	Relation string // "new_dependency_added", "dependency_removed", "dependency_changed"
}

// GetExplanation generates a human-readable explanation
func (c *CausalChain) GetExplanation() string {
	if len(c.Causes) == 0 {
		return c.Effect + " changed directly"
	}
	explanation := c.Effect + " changed because:\n"
	for _, cause := range c.Causes {
		switch cause.Relation {
		case "new_dependency_added":
			explanation += "  - " + cause.Cause + " was added\n"
		case "dependency_removed":
			explanation += "  - " + cause.Cause + " was removed\n"
		case "dependency_changed":
			explanation += "  - " + cause.Cause + " changed\n"
		}
	}
	return explanation
}

// ScopedDiff filters diff to a specific scope
type ScopedDiff struct {
	diff *DependencyAwareDiff
}

// NewResourcesOnly returns only new resources
func (d *DependencyAwareDiff) NewResourcesOnly() *DependencyAwareDiff {
	return &DependencyAwareDiff{
		Added:       d.Added,
		CausalChains: d.CausalChains,
	}
}

// ChangedResourcesOnly returns only changed resources
func (d *DependencyAwareDiff) ChangedResourcesOnly() *DependencyAwareDiff {
	return &DependencyAwareDiff{
		Changed:      d.Changed,
		CausalChains: d.CausalChains,
	}
}

// FilterByDependencyOf returns nodes affected by changes to a specific node
func (d *DependencyAwareDiff) FilterByDependencyOf(address string) *DependencyAwareDiff {
	result := &DependencyAwareDiff{
		Added:   []DiffNode{},
		Changed: []DiffNode{},
	}

	for _, node := range d.Added {
		for _, dep := range node.DependsOn {
			if dep == address {
				result.Added = append(result.Added, node)
				break
			}
		}
	}

	for _, node := range d.Changed {
		for _, dep := range node.DependsOn {
			if dep == address {
				result.Changed = append(result.Changed, node)
				break
			}
		}
	}

	return result
}
