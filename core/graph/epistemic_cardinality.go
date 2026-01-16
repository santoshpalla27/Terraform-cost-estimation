// Package graph - Epistemic cardinality enforcement
// Unknown cardinality NEVER produces numeric costs.
// This is epistemic honesty, not a feature.
package graph

import (
	"fmt"

	"terraform-cost/core/determinism"
)

// ErrUnknownCardinalityStrict is returned in strict mode
var ErrUnknownCardinalityStrict = fmt.Errorf("STRICT MODE: unknown cardinality - estimation blocked")

// Cardinality represents cardinality knowledge state
type Cardinality int

const (
	CardinalityKnownValue   Cardinality = iota // Cardinality is known
	CardinalityUnknownValue                    // Cardinality is unknowable pre-apply
	CardinalityRangeValue                      // Cardinality is a range
)

// String returns name
func (c Cardinality) String() string {
	names := []string{"known", "unknown", "range"}
	if int(c) < len(names) {
		return names[c]
	}
	return "invalid"
}

// IsUnknown returns true if cardinality is unknown
func (c Cardinality) IsUnknown() bool {
	return c == CardinalityUnknownValue || c == CardinalityRangeValue
}

// GuardExpansion is the SINGLE gate for all expansion
// All expansion paths MUST call this
func GuardExpansion(cardinality Cardinality) error {
	if cardinality == CardinalityUnknownValue {
		return ErrUnknownCardinalityStrict
	}
	return nil
}

// GuardExpansionWithMode checks expansion with mode awareness
func GuardExpansionWithMode(cardinality Cardinality, strictMode bool) (bool, error) {
	if cardinality != CardinalityUnknownValue {
		return true, nil // OK to expand
	}

	if strictMode {
		return false, ErrUnknownCardinalityStrict
	}

	// Permissive: no expansion, but no error - use symbolic instead
	return false, nil
}

// SymbolicCostOutput is the ONLY valid output for unknown cardinality
type SymbolicCostOutput struct {
	AssetAddress string
	Reason       string
	Cardinality  Cardinality
	Expression   string      // The expression that is unknown
	LowerBound   *determinism.Money
	UpperBound   *determinism.Money
	IsUnbounded  bool
}

// ToJSON returns JSON-safe output
func (s *SymbolicCostOutput) ToJSON() map[string]interface{} {
	result := map[string]interface{}{
		"cost":        "unknown",
		"reason":      s.Reason,
		"cardinality": s.Cardinality.String(),
	}
	if s.Expression != "" {
		result["expression"] = s.Expression
	}
	if s.LowerBound != nil {
		result["lower_bound"] = s.LowerBound.String()
	}
	if s.UpperBound != nil {
		result["upper_bound"] = s.UpperBound.String()
	}
	if s.IsUnbounded {
		result["is_unbounded"] = true
	}
	return result
}

// ToCLI returns CLI-friendly output
func (s *SymbolicCostOutput) ToCLI() string {
	return fmt.Sprintf("UNKNOWN (%s)", s.Reason)
}

// CardinalityChecker checks cardinality for common unknown sources
type CardinalityChecker struct {
	unknownSources []string
}

// NewCardinalityChecker creates a checker
func NewCardinalityChecker() *CardinalityChecker {
	return &CardinalityChecker{
		unknownSources: []string{},
	}
}

// CheckForEach checks for_each cardinality
func (c *CardinalityChecker) CheckForEach(expression string, keys interface{}) Cardinality {
	// Unknown sources
	if isDataSourceReference(expression) {
		c.unknownSources = append(c.unknownSources, "data source: "+expression)
		return CardinalityUnknownValue
	}
	if isModuleOutputReference(expression) {
		c.unknownSources = append(c.unknownSources, "module output: "+expression)
		return CardinalityUnknownValue
	}
	if containsImpureFunction(expression) {
		c.unknownSources = append(c.unknownSources, "impure function in: "+expression)
		return CardinalityUnknownValue
	}
	if keys == nil {
		c.unknownSources = append(c.unknownSources, "nil keys: "+expression)
		return CardinalityUnknownValue
	}

	return CardinalityKnownValue
}

// CheckCount checks count cardinality
func (c *CardinalityChecker) CheckCount(expression string, count interface{}) Cardinality {
	if isDataSourceReference(expression) {
		c.unknownSources = append(c.unknownSources, "data source: "+expression)
		return CardinalityUnknownValue
	}
	if isModuleOutputReference(expression) {
		c.unknownSources = append(c.unknownSources, "module output: "+expression)
		return CardinalityUnknownValue
	}
	if count == nil {
		c.unknownSources = append(c.unknownSources, "nil count: "+expression)
		return CardinalityUnknownValue
	}

	return CardinalityKnownValue
}

// GetUnknownSources returns sources of unknown cardinality
func (c *CardinalityChecker) GetUnknownSources() []string {
	return c.unknownSources
}

func isDataSourceReference(expr string) bool {
	return len(expr) > 5 && expr[:5] == "data."
}

func isModuleOutputReference(expr string) bool {
	return len(expr) > 7 && expr[:7] == "module."
}

func containsImpureFunction(expr string) bool {
	impureFunctions := []string{"fileset", "file", "templatefile", "timestamp", "uuid"}
	for _, fn := range impureFunctions {
		if containsSubstring(expr, fn+"(") {
			return true
		}
	}
	return false
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// BLOCKED: Placeholder expansion functions

// CreatePlaceholderInstances is BLOCKED - use SymbolicCostOutput instead
func CreatePlaceholderInstances(address string, count int) {
	BlockBypassAttempt("CreatePlaceholderInstances - unknown cardinality must use SymbolicCostOutput")
}

// CreateSyntheticCount is BLOCKED - unknown means unknown
func CreateSyntheticCount(address string) {
	BlockBypassAttempt("CreateSyntheticCount - unknown cardinality cannot have synthetic count")
}

// DefaultToOne is BLOCKED - defaulting to 1 is epistemically dishonest  
func DefaultToOne(address string) {
	BlockBypassAttempt("DefaultToOne - unknown cardinality cannot default to 1")
}

// DegradeConfidenceInsteadOfBlocking is BLOCKED - confidence degradation is not a substitute
func DegradeConfidenceInsteadOfBlocking(address string) {
	BlockBypassAttempt("DegradeConfidenceInsteadOfBlocking - unknown cardinality must block or use symbolic")
}
