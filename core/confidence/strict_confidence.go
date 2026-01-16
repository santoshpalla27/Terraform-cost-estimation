// Package confidence - Strictly pessimistic confidence
// Aggregate confidence = MIN(all children)
// No averaging, weighting, or smoothing allowed.
package confidence

import (
	"fmt"
	"sort"
)

// StrictAggregateConfidence returns the minimum confidence
// This is the ONLY valid aggregation function
func StrictAggregateConfidence(values []float64) float64 {
	if len(values) == 0 {
		return 0.0 // No data = no confidence
	}

	min := 1.0
	for _, v := range values {
		if v < min {
			min = v
		}
	}
	return min
}

// BLOCKED: These functions are epistemically dishonest

// AvgConfidence is BLOCKED - averaging hides low confidence
func AvgConfidence(values []float64) float64 {
	panic("BLOCKED: AvgConfidence - use StrictAggregateConfidence instead")
}

// WeightedConfidence is BLOCKED - weighting masks weak assumptions
func WeightedConfidence(values []float64, weights []float64) float64 {
	panic("BLOCKED: WeightedConfidence - use StrictAggregateConfidence instead")
}

// SmoothedConfidence is BLOCKED - smoothing is cosmetic
func SmoothedConfidence(values []float64) float64 {
	panic("BLOCKED: SmoothedConfidence - use StrictAggregateConfidence instead")
}

// ConfidenceWithCause tracks confidence with explanation
type ConfidenceWithCause struct {
	Value  float64
	Cause  string
	Source string // e.g., "aws_instance.web", "module.vpc"
}

// ConfidenceAggregator aggregates confidence pessimistically with cause tracking
type ConfidenceAggregator struct {
	entries []ConfidenceWithCause
	min     float64
	minCause string
	minSource string
}

// NewConfidenceAggregator creates an aggregator
func NewConfidenceAggregator() *ConfidenceAggregator {
	return &ConfidenceAggregator{
		entries:   []ConfidenceWithCause{},
		min:       1.0,
		minCause:  "",
		minSource: "",
	}
}

// Add adds a confidence value with cause
func (a *ConfidenceAggregator) Add(value float64, cause, source string) {
	entry := ConfidenceWithCause{
		Value:  value,
		Cause:  cause,
		Source: source,
	}
	a.entries = append(a.entries, entry)

	if value < a.min {
		a.min = value
		a.minCause = cause
		a.minSource = source
	}
}

// Aggregate returns the minimum confidence (pessimistic)
func (a *ConfidenceAggregator) Aggregate() float64 {
	return a.min
}

// GetLowestCause returns the cause of lowest confidence
func (a *ConfidenceAggregator) GetLowestCause() string {
	return a.minCause
}

// GetLowestSource returns the source of lowest confidence
func (a *ConfidenceAggregator) GetLowestSource() string {
	return a.minSource
}

// GetAllCauses returns all causes sorted by confidence (lowest first)
func (a *ConfidenceAggregator) GetAllCauses() []ConfidenceWithCause {
	sorted := make([]ConfidenceWithCause, len(a.entries))
	copy(sorted, a.entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value < sorted[j].Value
	})
	return sorted
}

// ConfidenceResult is the result of confidence aggregation
type ConfidenceResult struct {
	FinalConfidence   float64
	LowestConfidence  float64
	LowestCause       string
	LowestSource      string
	Level             string // "high", "medium", "low", "unknown"
	AllContributors   []ConfidenceWithCause
}

// GetResult returns the full confidence result
func (a *ConfidenceAggregator) GetResult() *ConfidenceResult {
	return &ConfidenceResult{
		FinalConfidence:  a.min,
		LowestConfidence: a.min,
		LowestCause:      a.minCause,
		LowestSource:     a.minSource,
		Level:            ConfidenceLevel(a.min),
		AllContributors:  a.GetAllCauses(),
	}
}

// ConfidenceLevel returns human-readable level
func ConfidenceLevel(confidence float64) string {
	switch {
	case confidence >= 0.9:
		return "high"
	case confidence >= 0.7:
		return "medium"
	case confidence >= 0.5:
		return "low"
	default:
		return "unknown"
	}
}

// Common confidence causes
const (
	CauseUnknownUsage       = "unknown usage assumption"
	CauseUnknownCardinality = "unknown cardinality"
	CauseImpureFunction     = "impure function in expression"
	CauseDataSource         = "data source dependency"
	CauseModuleOutput       = "module output dependency"
	CauseDefaultValue       = "default value used"
	CauseMissingPricing     = "pricing data unavailable"
)

// Standard confidence values
const (
	ConfidenceKnown      = 1.0
	ConfidenceHigh       = 0.9
	ConfidenceMedium     = 0.7
	ConfidenceLow        = 0.5
	ConfidenceVeryLow    = 0.3
	ConfidenceUnknown    = 0.0
)

// RollupConfidence rolls up confidence from children to parent
// This MUST be used at every boundary (CostUnit→Asset→Module→Project)
func RollupConfidence(children []float64) float64 {
	return StrictAggregateConfidence(children)
}

// AssertPessimistic panics if aggregate exceeds minimum
func AssertPessimistic(aggregate float64, components []float64) {
	if len(components) == 0 {
		return
	}
	min := 1.0
	for _, c := range components {
		if c < min {
			min = c
		}
	}
	if aggregate > min {
		panic(fmt.Sprintf("INVARIANT VIOLATED: aggregate confidence %.2f exceeds minimum component %.2f", aggregate, min))
	}
}

// ConfidenceTracker is an alias for ConfidenceAggregator (backwards compatibility)
type ConfidenceTracker = ConfidenceAggregator

// NewConfidenceTracker creates a new tracker (backwards compatibility)
func NewConfidenceTracker() *ConfidenceTracker {
	return NewConfidenceAggregator()
}

// AggregateConfidence is the public function for MIN aggregation
func AggregateConfidence(values []float64) float64 {
	return StrictAggregateConfidence(values)
}

// Apply applies a degradation (for backwards compatibility)
func (a *ConfidenceAggregator) Apply(factor string, reason string) {
	a.Add(0.9, reason, factor)
}
