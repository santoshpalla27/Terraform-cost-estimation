// Package coverage - Strict mode enforcement using cost-weighted coverage
// Enforces quality thresholds based on cost impact, not resource count.
package coverage

import "fmt"

// StrictPolicy defines enforceable coverage thresholds
type StrictPolicy struct {
	// Cost-weighted thresholds
	MaxUnsupportedCostPercent float64
	MaxSymbolicCostPercent    float64
	MinNumericCostPercent     float64

	// Absolute thresholds
	MaxUnsupportedDollars float64
	MaxSymbolicDollars    float64

	// Zero-tolerance flags
	BlockOnAnyUnsupported bool
	BlockOnAnySymbolic    bool

	// Resource count thresholds (secondary)
	MaxUnsupportedResources int
}

// DefaultPolicy returns sensible defaults
func DefaultPolicy() StrictPolicy {
	return StrictPolicy{
		MaxUnsupportedCostPercent: 5.0,
		MaxSymbolicCostPercent:    10.0,
		MinNumericCostPercent:     80.0,
		MaxUnsupportedDollars:     100.0,
		MaxSymbolicDollars:        500.0,
		BlockOnAnyUnsupported:     false,
		BlockOnAnySymbolic:        false,
		MaxUnsupportedResources:   10,
	}
}

// ProductionPolicy returns strict production thresholds
func ProductionPolicy() StrictPolicy {
	return StrictPolicy{
		MaxUnsupportedCostPercent: 0.0,
		MaxSymbolicCostPercent:    5.0,
		MinNumericCostPercent:     95.0,
		MaxUnsupportedDollars:     0.0,
		MaxSymbolicDollars:        100.0,
		BlockOnAnyUnsupported:     true,
		BlockOnAnySymbolic:        false,
		MaxUnsupportedResources:   0,
	}
}

// ZeroTolerancePolicy blocks on any non-numeric cost
func ZeroTolerancePolicy() StrictPolicy {
	return StrictPolicy{
		MaxUnsupportedCostPercent: 0.0,
		MaxSymbolicCostPercent:    0.0,
		MinNumericCostPercent:     100.0,
		MaxUnsupportedDollars:     0.0,
		MaxSymbolicDollars:        0.0,
		BlockOnAnyUnsupported:     true,
		BlockOnAnySymbolic:        true,
		MaxUnsupportedResources:   0,
	}
}

// Violation represents a policy violation
type Violation struct {
	Rule     string
	Actual   float64
	Limit    float64
	Message  string
	Severity string // "error", "warning"
}

// EnforceResult contains all violations
type EnforceResult struct {
	Passed     bool
	Violations []Violation
}

// Enforce checks a coverage report against a policy
func (p StrictPolicy) Enforce(report *WeightedCoverageReport) EnforceResult {
	result := EnforceResult{Passed: true}

	// Cost-weighted percentage checks
	if report.UnsupportedCostPercent > p.MaxUnsupportedCostPercent {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:     "max_unsupported_cost_percent",
			Actual:   report.UnsupportedCostPercent,
			Limit:    p.MaxUnsupportedCostPercent,
			Message:  fmt.Sprintf("Unsupported cost (%.1f%%) exceeds limit (%.1f%%)", report.UnsupportedCostPercent, p.MaxUnsupportedCostPercent),
			Severity: "error",
		})
	}

	if report.SymbolicCostPercent > p.MaxSymbolicCostPercent {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:     "max_symbolic_cost_percent",
			Actual:   report.SymbolicCostPercent,
			Limit:    p.MaxSymbolicCostPercent,
			Message:  fmt.Sprintf("Symbolic cost (%.1f%%) exceeds limit (%.1f%%)", report.SymbolicCostPercent, p.MaxSymbolicCostPercent),
			Severity: "error",
		})
	}

	if report.NumericCostPercent < p.MinNumericCostPercent {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:     "min_numeric_cost_percent",
			Actual:   report.NumericCostPercent,
			Limit:    p.MinNumericCostPercent,
			Message:  fmt.Sprintf("Numeric coverage (%.1f%%) below required (%.1f%%)", report.NumericCostPercent, p.MinNumericCostPercent),
			Severity: "error",
		})
	}

	// Absolute dollar checks
	if p.MaxUnsupportedDollars > 0 && report.TotalUnsupportedEst > p.MaxUnsupportedDollars {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:     "max_unsupported_dollars",
			Actual:   report.TotalUnsupportedEst,
			Limit:    p.MaxUnsupportedDollars,
			Message:  fmt.Sprintf("Unsupported cost ($%.2f) exceeds limit ($%.2f)", report.TotalUnsupportedEst, p.MaxUnsupportedDollars),
			Severity: "error",
		})
	}

	if p.MaxSymbolicDollars > 0 && report.TotalSymbolicBound > p.MaxSymbolicDollars {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:     "max_symbolic_dollars",
			Actual:   report.TotalSymbolicBound,
			Limit:    p.MaxSymbolicDollars,
			Message:  fmt.Sprintf("Symbolic cost ($%.2f) exceeds limit ($%.2f)", report.TotalSymbolicBound, p.MaxSymbolicDollars),
			Severity: "error",
		})
	}

	// Zero-tolerance checks
	if p.BlockOnAnyUnsupported && report.UnsupportedResourceCount > 0 {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:     "block_on_any_unsupported",
			Actual:   float64(report.UnsupportedResourceCount),
			Limit:    0,
			Message:  fmt.Sprintf("Found %d unsupported resources (policy: block on any)", report.UnsupportedResourceCount),
			Severity: "error",
		})
	}

	if p.BlockOnAnySymbolic && report.SymbolicResourceCount > 0 {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:     "block_on_any_symbolic",
			Actual:   float64(report.SymbolicResourceCount),
			Limit:    0,
			Message:  fmt.Sprintf("Found %d symbolic resources (policy: block on any)", report.SymbolicResourceCount),
			Severity: "error",
		})
	}

	// Resource count check
	if p.MaxUnsupportedResources >= 0 && report.UnsupportedResourceCount > p.MaxUnsupportedResources {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:     "max_unsupported_resources",
			Actual:   float64(report.UnsupportedResourceCount),
			Limit:    float64(p.MaxUnsupportedResources),
			Message:  fmt.Sprintf("Unsupported resource count (%d) exceeds limit (%d)", report.UnsupportedResourceCount, p.MaxUnsupportedResources),
			Severity: "error",
		})
	}

	return result
}

// ToCLI formats violations for CLI output
func (r EnforceResult) ToCLI() string {
	if r.Passed {
		return "✅ All coverage policies passed\n"
	}

	result := "❌ STRICT MODE VIOLATIONS:\n\n"

	for _, v := range r.Violations {
		result += fmt.Sprintf("   • [%s] %s\n", v.Rule, v.Message)
	}

	return result
}
