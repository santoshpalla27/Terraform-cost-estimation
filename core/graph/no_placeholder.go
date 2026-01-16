// Package graph - Placeholder expansion removal
// This file REMOVES all placeholder expansion.
// Unknown cardinality = symbolic cost, NEVER instances.
package graph

import (
	"fmt"

	"terraform-cost/core/determinism"
)

// ExpansionResult is the result of attempting expansion
type ExpansionResult struct {
	// Success case
	Instances []string // Only populated if cardinality is known

	// Failure case (unknown cardinality)
	IsSymbolic bool
	Symbolic   *SymbolicCostOutput

	// Blocked in strict mode
	BlockedError error
}

// StrictExpansionGate is the ONLY way to expand resources
// Placeholder expansion does not exist
type StrictExpansionGate struct {
	strictMode bool
	symbolic   []*SymbolicCostOutput
}

// NewStrictExpansionGate creates a gate
func NewStrictExpansionGate(strictMode bool) *StrictExpansionGate {
	return &StrictExpansionGate{
		strictMode: strictMode,
		symbolic:   []*SymbolicCostOutput{},
	}
}

// TryExpand attempts expansion - returns symbolic if unknown
// NEVER creates placeholder instances
func (g *StrictExpansionGate) TryExpand(address string, cardinality Cardinality, count int, keys []string) *ExpansionResult {
	result := &ExpansionResult{}

	if cardinality == CardinalityKnownValue {
		// Known cardinality - expand normally
		if len(keys) > 0 {
			// for_each
			result.Instances = make([]string, len(keys))
			for i, key := range keys {
				result.Instances[i] = fmt.Sprintf("%s[%q]", address, key)
			}
		} else if count >= 0 {
			// count
			result.Instances = make([]string, count)
			for i := 0; i < count; i++ {
				result.Instances[i] = fmt.Sprintf("%s[%d]", address, i)
			}
		} else {
			// No count/for_each - single instance
			result.Instances = []string{address}
		}
		return result
	}

	// Unknown cardinality - NEVER expand
	result.IsSymbolic = true
	result.Symbolic = &SymbolicCostOutput{
		AssetAddress: address,
		Reason:       fmt.Sprintf("cardinality is %s", cardinality),
		Cardinality:  cardinality,
		IsUnbounded:  cardinality == CardinalityUnknownValue,
	}
	g.symbolic = append(g.symbolic, result.Symbolic)

	if g.strictMode {
		result.BlockedError = fmt.Errorf("STRICT MODE: cannot expand %s - cardinality unknown", address)
	}

	return result
}

// TryExpandForEach attempts for_each expansion
func (g *StrictExpansionGate) TryExpandForEach(address string, expression string, keys interface{}) *ExpansionResult {
	// Check cardinality
	checker := NewCardinalityChecker()
	cardinality := checker.CheckForEach(expression, keys)

	if keySlice, ok := keys.([]string); ok && cardinality == CardinalityKnownValue {
		return g.TryExpand(address, cardinality, 0, keySlice)
	}

	return g.TryExpand(address, cardinality, 0, nil)
}

// TryExpandCount attempts count expansion
func (g *StrictExpansionGate) TryExpandCount(address string, expression string, count interface{}) *ExpansionResult {
	// Check cardinality
	checker := NewCardinalityChecker()
	cardinality := checker.CheckCount(expression, count)

	if countInt, ok := count.(int); ok && cardinality == CardinalityKnownValue {
		return g.TryExpand(address, cardinality, countInt, nil)
	}

	return g.TryExpand(address, CardinalityUnknownValue, 0, nil)
}

// GetSymbolicCosts returns all symbolic costs
func (g *StrictExpansionGate) GetSymbolicCosts() []*SymbolicCostOutput {
	return g.symbolic
}

// HasSymbolicCosts returns true if any expansion resulted in symbolic cost
func (g *StrictExpansionGate) HasSymbolicCosts() bool {
	return len(g.symbolic) > 0
}

// SymbolicCostBudget represents a budget for symbolic costs
type SymbolicCostBudget struct {
	Reason      string
	LowerBound  determinism.Money
	UpperBound  *determinism.Money // nil = unbounded
	Expression  string
	Cardinality Cardinality
}

// ToOutput converts to output format
func (b *SymbolicCostBudget) ToOutput() map[string]interface{} {
	result := map[string]interface{}{
		"type":        "symbolic",
		"reason":      b.Reason,
		"cardinality": b.Cardinality.String(),
	}
	if b.Expression != "" {
		result["expression"] = b.Expression
	}
	result["lower_bound"] = b.LowerBound.String()
	if b.UpperBound != nil {
		result["upper_bound"] = b.UpperBound.String()
	} else {
		result["upper_bound"] = "unbounded"
	}
	return result
}

// BLOCKED PLACEHOLDERS - These panic to prevent accidental use

// CreatePlaceholderInstance is REMOVED - use TryExpand instead
func CreatePlaceholderInstance(address string) {
	panic("REMOVED: CreatePlaceholderInstance - placeholder expansion no longer exists")
}

// ExpandWithDefaultCount is REMOVED - defaults are epistemically dishonest
func ExpandWithDefaultCount(address string, defaultCount int) {
	panic("REMOVED: ExpandWithDefaultCount - unknown cardinality must use SymbolicCost")
}

// InferCardinalityFromUsage is REMOVED - inference is not knowledge
func InferCardinalityFromUsage(address string) {
	panic("REMOVED: InferCardinalityFromUsage - cardinality must be explicitly known")
}

// RetainCardinalityFromState is REMOVED - state cardinality may have changed
func RetainCardinalityFromState(address string) {
	panic("REMOVED: RetainCardinalityFromState - state is not authoritative for pre-apply estimation")
}
