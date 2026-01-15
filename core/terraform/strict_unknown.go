// Package terraform - Strict unknown value semantics
// Unknowns NEVER collapse. They ALWAYS propagate. Cost ALWAYS degrades.
package terraform

import (
	"fmt"
)

// StrictUnknown represents an unknown value that CANNOT be collapsed.
// This is not a nil, not a zero, not an empty string - it is EXPLICITLY unknown.
type StrictUnknown struct {
	// Why this is unknown
	Reason UnknownReason

	// What type we expected
	ExpectedType ValueType

	// Where it came from
	Source string

	// What depends on this
	DependentCount int

	// How many levels deep (for debugging)
	PropagationDepth int
}

// IsUnknown always returns true for StrictUnknown
func (u *StrictUnknown) IsUnknown() bool { return true }

// String returns a debug representation
func (u *StrictUnknown) String() string {
	return fmt.Sprintf("(unknown: %s from %s)", u.Reason, u.Source)
}

// StrictValue is a value that is EITHER known OR unknown, never both, never nil.
type StrictValue struct {
	known   interface{}
	unknown *StrictUnknown
}

// MustBeKnown panics if the value is unknown - use for programming errors only
func (v StrictValue) MustBeKnown(context string) interface{} {
	if v.IsUnknown() {
		panic(fmt.Sprintf("BUG: expected known value but got unknown in %s: %s", context, v.unknown.String()))
	}
	return v.known
}

// StrictKnown creates a known value
func StrictKnown(val interface{}) StrictValue {
	if val == nil {
		// nil is a VALID known value (null)
		return StrictValue{known: nil, unknown: nil}
	}
	return StrictValue{known: val, unknown: nil}
}

// StrictUnknownValue creates an unknown value
func StrictUnknownValue(reason UnknownReason, expectedType ValueType, source string) StrictValue {
	return StrictValue{
		unknown: &StrictUnknown{
			Reason:       reason,
			ExpectedType: expectedType,
			Source:       source,
		},
	}
}

// IsKnown returns true ONLY if value is known
func (v StrictValue) IsKnown() bool {
	return v.unknown == nil
}

// IsUnknown returns true ONLY if value is unknown
func (v StrictValue) IsUnknown() bool {
	return v.unknown != nil
}

// Get returns the value if known, nil if unknown
// IMPORTANT: Check IsKnown() first!
func (v StrictValue) Get() interface{} {
	if v.IsUnknown() {
		return nil
	}
	return v.known
}

// GetUnknown returns the unknown info
func (v StrictValue) GetUnknown() *StrictUnknown {
	return v.unknown
}

// Propagate creates a NEW unknown that depends on this one
// The original unknown is preserved, depth is increased
func (v StrictValue) Propagate(newSource string) StrictValue {
	if v.IsKnown() {
		return v // Known values don't propagate
	}
	return StrictValue{
		unknown: &StrictUnknown{
			Reason:           ReasonDependsOnUnknown,
			ExpectedType:     v.unknown.ExpectedType,
			Source:           fmt.Sprintf("%s (via %s)", newSource, v.unknown.Source),
			PropagationDepth: v.unknown.PropagationDepth + 1,
		},
	}
}

// UnknownSet tracks all unresolved unknowns in a context
type UnknownSet struct {
	unknowns map[string]*StrictUnknown
}

// NewUnknownSet creates an empty unknown set
func NewUnknownSet() *UnknownSet {
	return &UnknownSet{
		unknowns: make(map[string]*StrictUnknown),
	}
}

// Add records an unknown
func (s *UnknownSet) Add(address string, u *StrictUnknown) {
	s.unknowns[address] = u
}

// Has checks if an address is unknown
func (s *UnknownSet) Has(address string) bool {
	_, ok := s.unknowns[address]
	return ok
}

// Get returns the unknown for an address
func (s *UnknownSet) Get(address string) *StrictUnknown {
	return s.unknowns[address]
}

// Merge combines two unknown sets
func (s *UnknownSet) Merge(other *UnknownSet) {
	if other == nil {
		return
	}
	for k, v := range other.unknowns {
		s.unknowns[k] = v
	}
}

// Count returns the number of unknowns
func (s *UnknownSet) Count() int {
	return len(s.unknowns)
}

// All returns all unknowns
func (s *UnknownSet) All() map[string]*StrictUnknown {
	result := make(map[string]*StrictUnknown)
	for k, v := range s.unknowns {
		result[k] = v
	}
	return result
}

// UnknownPropagator ensures unknowns propagate through operations
type UnknownPropagator struct {
	set *UnknownSet
}

// NewUnknownPropagator creates a propagator
func NewUnknownPropagator() *UnknownPropagator {
	return &UnknownPropagator{
		set: NewUnknownSet(),
	}
}

// CheckAndPropagate checks if any input is unknown, and if so returns propagated unknown
func (p *UnknownPropagator) CheckAndPropagate(inputs []StrictValue, operation string) (StrictValue, bool) {
	for _, input := range inputs {
		if input.IsUnknown() {
			// Any unknown input → entire result is unknown
			propagated := input.Propagate(operation)
			return propagated, true
		}
	}
	return StrictValue{}, false
}

// Set returns the underlying unknown set
func (p *UnknownPropagator) Set() *UnknownSet {
	return p.set
}

// StrictArithmetic performs arithmetic that respects unknowns
type StrictArithmetic struct{}

// Add adds two values - if either is unknown, result is unknown
func (StrictArithmetic) Add(a, b StrictValue) StrictValue {
	if a.IsUnknown() {
		return a.Propagate("add.left")
	}
	if b.IsUnknown() {
		return b.Propagate("add.right")
	}

	// Both known - do arithmetic
	aNum, aOk := toFloat(a.Get())
	bNum, bOk := toFloat(b.Get())
	if !aOk || !bOk {
		return StrictUnknownValue(ReasonExpressionError, TypeNumber, "add: non-numeric operand")
	}
	return StrictKnown(aNum + bNum)
}

// Mul multiplies two values - if either is unknown, result is unknown
func (StrictArithmetic) Mul(a, b StrictValue) StrictValue {
	if a.IsUnknown() {
		return a.Propagate("mul.left")
	}
	if b.IsUnknown() {
		return b.Propagate("mul.right")
	}

	aNum, aOk := toFloat(a.Get())
	bNum, bOk := toFloat(b.Get())
	if !aOk || !bOk {
		return StrictUnknownValue(ReasonExpressionError, TypeNumber, "mul: non-numeric operand")
	}
	return StrictKnown(aNum * bNum)
}

// Div divides two values - if either is unknown, result is unknown
func (StrictArithmetic) Div(a, b StrictValue) StrictValue {
	if a.IsUnknown() {
		return a.Propagate("div.left")
	}
	if b.IsUnknown() {
		return b.Propagate("div.right")
	}

	aNum, aOk := toFloat(a.Get())
	bNum, bOk := toFloat(b.Get())
	if !aOk || !bOk {
		return StrictUnknownValue(ReasonExpressionError, TypeNumber, "div: non-numeric operand")
	}
	if bNum == 0 {
		return StrictUnknownValue(ReasonExpressionError, TypeNumber, "div: division by zero")
	}
	return StrictKnown(aNum / bNum)
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}

// StrictCondition evaluates conditions respecting unknowns
type StrictCondition struct{}

// IfThenElse evaluates a conditional - if condition is unknown, result is unknown
func (StrictCondition) IfThenElse(condition, thenVal, elseVal StrictValue) StrictValue {
	if condition.IsUnknown() {
		// Unknown condition → result is unknown
		return condition.Propagate("condition")
	}

	// Condition is known - evaluate
	cond, ok := condition.Get().(bool)
	if !ok {
		return StrictUnknownValue(ReasonExpressionError, TypeUnknown, "if: non-boolean condition")
	}

	if cond {
		return thenVal
	}
	return elseVal
}

// Equals compares two values - if either is unknown, result is unknown
func (StrictCondition) Equals(a, b StrictValue) StrictValue {
	if a.IsUnknown() {
		return a.Propagate("equals.left")
	}
	if b.IsUnknown() {
		return b.Propagate("equals.right")
	}
	return StrictKnown(a.Get() == b.Get())
}

// StrictList handles list operations with unknown propagation
type StrictList struct{}

// Index gets an element - if list or index is unknown, result is unknown
func (StrictList) Index(list StrictValue, index StrictValue) StrictValue {
	if list.IsUnknown() {
		return list.Propagate("list.index")
	}
	if index.IsUnknown() {
		return index.Propagate("list.index")
	}

	l, ok := list.Get().([]interface{})
	if !ok {
		return StrictUnknownValue(ReasonExpressionError, TypeUnknown, "index: not a list")
	}

	idx, ok := toInt(index.Get())
	if !ok {
		return StrictUnknownValue(ReasonExpressionError, TypeUnknown, "index: non-integer index")
	}

	if idx < 0 || idx >= len(l) {
		return StrictUnknownValue(ReasonExpressionError, TypeUnknown, "index: out of bounds")
	}

	return StrictKnown(l[idx])
}

// Length gets list length - if list is unknown, result is unknown
func (StrictList) Length(list StrictValue) StrictValue {
	if list.IsUnknown() {
		return list.Propagate("list.length")
	}

	switch v := list.Get().(type) {
	case []interface{}:
		return StrictKnown(len(v))
	case string:
		return StrictKnown(len(v))
	case map[string]interface{}:
		return StrictKnown(len(v))
	default:
		return StrictUnknownValue(ReasonExpressionError, TypeNumber, "length: unsupported type")
	}
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// CostDegradation tracks how costs are degraded due to unknowns
type CostDegradation struct {
	IsDegraded bool
	Confidence float64 // 0.0 - 1.0

	Reasons []DegradationReason
}

// DegradationReason explains why cost is degraded
type DegradationReason struct {
	Component string
	Reason    string
	Impact    float64 // How much this reduces confidence
	IsUnknown bool    // Is this due to an unknown value?
}

// NewCostDegradation creates a fresh (non-degraded) state
func NewCostDegradation() *CostDegradation {
	return &CostDegradation{
		IsDegraded: false,
		Confidence: 1.0,
		Reasons:    []DegradationReason{},
	}
}

// RecordUnknown records that an unknown caused degradation
func (d *CostDegradation) RecordUnknown(component, source string, impact float64) {
	d.IsDegraded = true
	d.Confidence *= (1.0 - impact)
	d.Reasons = append(d.Reasons, DegradationReason{
		Component: component,
		Reason:    fmt.Sprintf("unknown value: %s", source),
		Impact:    impact,
		IsUnknown: true,
	})
}

// RecordMissing records that a missing value caused degradation
func (d *CostDegradation) RecordMissing(component, what string, impact float64) {
	d.IsDegraded = true
	d.Confidence *= (1.0 - impact)
	d.Reasons = append(d.Reasons, DegradationReason{
		Component: component,
		Reason:    fmt.Sprintf("missing: %s", what),
		Impact:    impact,
		IsUnknown: false,
	})
}
