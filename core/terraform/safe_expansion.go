// Package terraform - Safe for_each handling
// When for_each keys are unknown: DO NOT EXPAND.
// Replace with symbolic range, surface warning, block in strict mode.
package terraform

import (
	"fmt"
)

// ForEachResult represents the result of for_each evaluation
type ForEachResult struct {
	// Is the for_each value known?
	IsKnown bool

	// Keys if known
	Keys []string

	// If unknown, symbolic range
	SymbolicRange *SymbolicRange

	// Warning to surface
	Warning string

	// Block estimation in strict mode?
	BlocksEstimation bool
}

// SymbolicRange represents an unknown cardinality
type SymbolicRange struct {
	// Minimum instances (conservative)
	Min int

	// Maximum instances (if bounded, -1 for unbounded)
	Max int

	// Expression that could not be evaluated
	Expression string

	// References that caused unknown
	UnknownReferences []string

	// Confidence impact
	ConfidenceImpact float64
}

// SafeForEachEvaluator evaluates for_each safely
type SafeForEachEvaluator struct {
	mode           EvaluationMode
	enforcer       *StrictModeEnforcer
	unknownHandler UnknownForEachHandler
}

// UnknownForEachHandler defines how to handle unknown for_each
type UnknownForEachHandler int

const (
	// HandlerBlock blocks estimation
	HandlerBlock UnknownForEachHandler = iota

	// HandlerSymbolic uses symbolic range
	HandlerSymbolic

	// HandlerMinimum uses minimum assumption (0 or 1)
	HandlerMinimum
)

// NewSafeForEachEvaluator creates an evaluator
func NewSafeForEachEvaluator(mode EvaluationMode) *SafeForEachEvaluator {
	handler := HandlerSymbolic
	if mode == ModeStrict {
		handler = HandlerBlock
	}

	return &SafeForEachEvaluator{
		mode:           mode,
		enforcer:       NewStrictModeEnforcer(mode),
		unknownHandler: handler,
	}
}

// Evaluate evaluates a for_each expression
func (e *SafeForEachEvaluator) Evaluate(address string, expr *ExpressionValue) *ForEachResult {
	result := &ForEachResult{
		IsKnown:          false,
		Keys:             []string{},
		BlocksEstimation: false,
	}

	// If expression is known, extract keys
	if expr != nil && expr.IsKnown {
		result.IsKnown = true
		result.Keys = extractForEachKeys(expr.Value)
		return result
	}

	// Unknown for_each - handle according to mode
	result.IsKnown = false

	// Check for blocking conditions
	if e.mode == ModeStrict {
		result.BlocksEstimation = true
		result.Warning = fmt.Sprintf("for_each at %s is unknown - blocking estimation in strict mode", address)
		return result
	}

	// Create symbolic range
	result.SymbolicRange = e.createSymbolicRange(address, expr)
	result.Warning = fmt.Sprintf(
		"for_each at %s has unknown cardinality (estimated %d-%d instances)",
		address, result.SymbolicRange.Min, result.SymbolicRange.Max,
	)

	return result
}

func (e *SafeForEachEvaluator) createSymbolicRange(address string, expr *ExpressionValue) *SymbolicRange {
	sr := &SymbolicRange{
		Min:              0,
		Max:              -1, // Unbounded
		ConfidenceImpact: 0.5,
	}

	if expr != nil {
		sr.Expression = expr.Expression
		sr.UnknownReferences = expr.References
	}

	// Infer bounds from expression type if possible
	if expr != nil && len(expr.References) > 0 {
		for _, ref := range expr.References {
			sr.UnknownReferences = append(sr.UnknownReferences, ref)

			// Data source references are fully unknown
			if isDataSourceRef(ref) {
				sr.Min = 0
				sr.Max = -1
				sr.ConfidenceImpact = 0.6
				continue
			}

			// Variable references might have bounds
			if isVariableRef(ref) {
				sr.Min = 1
				sr.Max = 10 // Conservative assumption
				sr.ConfidenceImpact = 0.4
			}
		}
	}

	return sr
}

func extractForEachKeys(value interface{}) []string {
	switch v := value.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		return keys
	case []interface{}:
		keys := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				keys = append(keys, s)
			}
		}
		return keys
	case []string:
		return v
	default:
		return []string{}
	}
}

func isDataSourceRef(ref string) bool {
	return len(ref) >= 5 && ref[:5] == "data."
}

func isVariableRef(ref string) bool {
	return len(ref) >= 4 && ref[:4] == "var."
}

// SafeCountEvaluator evaluates count safely
type SafeCountEvaluator struct {
	mode     EvaluationMode
	enforcer *StrictModeEnforcer
}

// CountResult represents the result of count evaluation
type CountResult struct {
	IsKnown          bool
	Value            int
	SymbolicRange    *SymbolicRange
	Warning          string
	BlocksEstimation bool
}

// NewSafeCountEvaluator creates an evaluator
func NewSafeCountEvaluator(mode EvaluationMode) *SafeCountEvaluator {
	return &SafeCountEvaluator{
		mode:     mode,
		enforcer: NewStrictModeEnforcer(mode),
	}
}

// Evaluate evaluates a count expression
func (e *SafeCountEvaluator) Evaluate(address string, expr *ExpressionValue) *CountResult {
	result := &CountResult{
		IsKnown:          false,
		Value:            0,
		BlocksEstimation: false,
	}

	// If expression is known, extract value
	if expr != nil && expr.IsKnown {
		result.IsKnown = true
		result.Value = extractCountValue(expr.Value)
		return result
	}

	// Unknown count
	if e.mode == ModeStrict {
		result.BlocksEstimation = true
		result.Warning = fmt.Sprintf("count at %s is unknown - blocking estimation in strict mode", address)
		return result
	}

	// Create symbolic range
	result.SymbolicRange = e.inferCountRange(address, expr)
	result.Warning = fmt.Sprintf(
		"count at %s has unknown value (estimated %d-%d instances)",
		address, result.SymbolicRange.Min, result.SymbolicRange.Max,
	)

	// Use minimum for estimation
	result.Value = result.SymbolicRange.Min

	return result
}

func (e *SafeCountEvaluator) inferCountRange(address string, expr *ExpressionValue) *SymbolicRange {
	sr := &SymbolicRange{
		Min:              0,
		Max:              10, // Conservative upper bound
		ConfidenceImpact: 0.4,
	}

	if expr != nil {
		sr.Expression = expr.Expression
		sr.UnknownReferences = expr.References

		// Check for common patterns
		if containsLengthCall(expr.Expression) {
			sr.Min = 0
			sr.Max = 20
			sr.ConfidenceImpact = 0.45
		}

		if containsConditional(expr.Expression) {
			sr.Min = 0
			sr.Max = 1
			sr.ConfidenceImpact = 0.3
		}
	}

	return sr
}

func extractCountValue(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 1
	}
}

func containsLengthCall(expr string) bool {
	return len(expr) >= 6 && (contains(expr, "length(") || contains(expr, "len("))
}

func containsConditional(expr string) bool {
	return contains(expr, "?") || contains(expr, "if ")
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// CardinalityWarning is a warning about unknown cardinality
type CardinalityWarning struct {
	Address          string
	Type             string // "count" or "for_each"
	Expression       string
	SymbolicRange    *SymbolicRange
	Message          string
	BlocksEstimation bool
}

// CardinalityWarnings collects cardinality warnings
type CardinalityWarnings struct {
	warnings []CardinalityWarning
}

// NewCardinalityWarnings creates a collector
func NewCardinalityWarnings() *CardinalityWarnings {
	return &CardinalityWarnings{
		warnings: []CardinalityWarning{},
	}
}

// AddForEach adds a for_each warning
func (w *CardinalityWarnings) AddForEach(address string, result *ForEachResult) {
	if result.IsKnown {
		return
	}
	w.warnings = append(w.warnings, CardinalityWarning{
		Address:          address,
		Type:             "for_each",
		SymbolicRange:    result.SymbolicRange,
		Message:          result.Warning,
		BlocksEstimation: result.BlocksEstimation,
	})
}

// AddCount adds a count warning
func (w *CardinalityWarnings) AddCount(address string, result *CountResult) {
	if result.IsKnown {
		return
	}
	w.warnings = append(w.warnings, CardinalityWarning{
		Address:          address,
		Type:             "count",
		SymbolicRange:    result.SymbolicRange,
		Message:          result.Warning,
		BlocksEstimation: result.BlocksEstimation,
	})
}

// All returns all warnings
func (w *CardinalityWarnings) All() []CardinalityWarning {
	return w.warnings
}

// HasBlocking returns true if any warning blocks estimation
func (w *CardinalityWarnings) HasBlocking() bool {
	for _, warn := range w.warnings {
		if warn.BlocksEstimation {
			return true
		}
	}
	return false
}
