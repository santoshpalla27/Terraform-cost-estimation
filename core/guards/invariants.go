// Package guards - Runtime assertion guards
// These assertions PANIC if violated - there is no recovery.
package guards

import (
	"fmt"

	"terraform-cost/core/graph"
	"terraform-cost/core/terraform"
)

// InvariantEnforcer enforces critical architectural invariants
type InvariantEnforcer struct {
	dependencyGraphBuilt bool
	providersFrozen      bool
	expansionComplete    bool
	costGraphBuilt       bool
	pricingComplete      bool
	depGraph            *graph.InfrastructureGraph
	finalizer           *terraform.ProviderFinalizer
}

// NewInvariantEnforcer creates an enforcer
func NewInvariantEnforcer() *InvariantEnforcer {
	return &InvariantEnforcer{}
}

// MarkDependencyGraphBuilt marks the dependency graph as built
func (e *InvariantEnforcer) MarkDependencyGraphBuilt(g *graph.InfrastructureGraph) {
	if g == nil {
		panic("INVARIANT VIOLATED: dependency graph cannot be nil")
	}
	e.depGraph = g
	e.dependencyGraphBuilt = true
}

// MarkProvidersFrozen marks providers as frozen
func (e *InvariantEnforcer) MarkProvidersFrozen(f *terraform.ProviderFinalizer) {
	if !e.dependencyGraphBuilt {
		panic("INVARIANT VIOLATED: providers cannot be frozen before dependency graph is built")
	}
	if f == nil {
		panic("INVARIANT VIOLATED: provider finalizer cannot be nil")
	}
	if !f.IsFinalized() {
		panic("INVARIANT VIOLATED: provider finalizer must be finalized before marking frozen")
	}
	e.finalizer = f
	e.providersFrozen = true
}

// MarkExpansionComplete marks expansion as complete
func (e *InvariantEnforcer) MarkExpansionComplete() {
	if !e.providersFrozen {
		panic("INVARIANT VIOLATED: expansion cannot complete before providers are frozen")
	}
	e.expansionComplete = true
}

// MarkCostGraphBuilt marks cost graph as built
func (e *InvariantEnforcer) MarkCostGraphBuilt() {
	if !e.expansionComplete {
		panic("INVARIANT VIOLATED: cost graph cannot be built before expansion is complete")
	}
	e.costGraphBuilt = true
}

// MarkPricingComplete marks pricing as complete
func (e *InvariantEnforcer) MarkPricingComplete() {
	if !e.costGraphBuilt {
		panic("INVARIANT VIOLATED: pricing cannot complete before cost graph is built")
	}
	e.pricingComplete = true
}

// AssertDependencyGraphBuilt asserts dependency graph is built
func (e *InvariantEnforcer) AssertDependencyGraphBuilt() {
	if !e.dependencyGraphBuilt {
		panic("ASSERTION FAILED: dependency graph not built")
	}
}

// AssertProvidersFrozen asserts providers are frozen
func (e *InvariantEnforcer) AssertProvidersFrozen() {
	if !e.providersFrozen {
		panic("ASSERTION FAILED: providers not frozen")
	}
}

// AssertExpansionComplete asserts expansion is complete
func (e *InvariantEnforcer) AssertExpansionComplete() {
	if !e.expansionComplete {
		panic("ASSERTION FAILED: expansion not complete")
	}
}

// AssertCostGraphBuilt asserts cost graph is built
func (e *InvariantEnforcer) AssertCostGraphBuilt() {
	if !e.costGraphBuilt {
		panic("ASSERTION FAILED: cost graph not built")
	}
}

// AssertCanPrice asserts pricing is allowed for an address
func (e *InvariantEnforcer) AssertCanPrice(address string) {
	e.AssertProvidersFrozen()
	e.AssertExpansionComplete()
	e.AssertCostGraphBuilt()
}

// AssertNodeInDependencyGraph asserts a node exists in the dependency graph
func (e *InvariantEnforcer) AssertNodeInDependencyGraph(address string) {
	e.AssertDependencyGraphBuilt()
	if e.depGraph.GetNode(address) == nil {
		panic(fmt.Sprintf("ASSERTION FAILED: node %s not in dependency graph", address))
	}
}

// GuardedExpansion ensures expansion only happens after providers are frozen
type GuardedExpansion struct {
	enforcer            *InvariantEnforcer
	cardinalityWarnings []string
}

// NewGuardedExpansion creates guarded expansion
func NewGuardedExpansion(enforcer *InvariantEnforcer) *GuardedExpansion {
	return &GuardedExpansion{
		enforcer:            enforcer,
		cardinalityWarnings: []string{},
	}
}

// ExpandResource expands a resource, respecting invariants
func (g *GuardedExpansion) ExpandResource(address string, count int, isCountKnown bool) ([]string, error) {
	g.enforcer.AssertProvidersFrozen()

	if !isCountKnown {
		g.cardinalityWarnings = append(g.cardinalityWarnings,
			fmt.Sprintf("CARDINALITY UNKNOWN: %s - not expanded", address))
		return nil, &UnknownCardinalityError{Address: address, Type: "count"}
	}

	instances := make([]string, count)
	for i := 0; i < count; i++ {
		instances[i] = fmt.Sprintf("%s[%d]", address, i)
	}
	return instances, nil
}

// ExpandForEach expands for_each, respecting invariants
func (g *GuardedExpansion) ExpandForEach(address string, keys []string, isKeysKnown bool) ([]string, error) {
	g.enforcer.AssertProvidersFrozen()

	if !isKeysKnown {
		g.cardinalityWarnings = append(g.cardinalityWarnings,
			fmt.Sprintf("CARDINALITY UNKNOWN: %s - not expanded", address))
		return nil, &UnknownCardinalityError{Address: address, Type: "for_each"}
	}

	instances := make([]string, len(keys))
	for i, key := range keys {
		instances[i] = fmt.Sprintf("%s[%q]", address, key)
	}
	return instances, nil
}

// GetWarnings returns cardinality warnings
func (g *GuardedExpansion) GetWarnings() []string {
	return g.cardinalityWarnings
}

// UnknownCardinalityError indicates expansion was blocked
type UnknownCardinalityError struct {
	Address string
	Type    string
}

func (e *UnknownCardinalityError) Error() string {
	return fmt.Sprintf("%s at %s is unknown - expansion blocked", e.Type, e.Address)
}
