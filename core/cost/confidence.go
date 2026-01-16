// Package cost - Confidence-degrading cost calculations
// Every cost unit MUST carry confidence and degradation reasons.
package cost

import (
	"fmt"

	"terraform-cost/core/determinism"
	"terraform-cost/core/model"
	"terraform-cost/core/pricing"
)

// ConfidenceBoundCost is a cost value that ALWAYS carries confidence.
// No cost can exist without explicit confidence tracking.
type ConfidenceBoundCost struct {
	// The cost value
	Monthly determinism.Money
	Hourly  determinism.Money

	// Confidence (0.0 - 1.0)
	Confidence float64

	// Why confidence is what it is
	Factors []ConfidenceFactor

	// Is this degraded from full confidence?
	IsDegraded bool

	// Snapshot reference (for traceability)
	SnapshotID pricing.SnapshotID
}

// ConfidenceFactor explains one contribution to confidence
type ConfidenceFactor struct {
	Source     string  // What affected confidence
	Reason     string  // Why
	Impact     float64 // How much (0.0-1.0, where 1.0 removes all confidence)
	IsUnknown  bool    // Is this due to an unknown value?
	Component  string  // Which component?
}

// NewConfidenceBoundCost creates a new cost with full confidence
func NewConfidenceBoundCost(monthly, hourly determinism.Money, snapshotID pricing.SnapshotID) *ConfidenceBoundCost {
	return &ConfidenceBoundCost{
		Monthly:    monthly,
		Hourly:     hourly,
		Confidence: 1.0,
		Factors:    []ConfidenceFactor{},
		IsDegraded: false,
		SnapshotID: snapshotID,
	}
}

// Zero creates a zero cost
func ZeroCost(snapshotID pricing.SnapshotID) *ConfidenceBoundCost {
	return &ConfidenceBoundCost{
		Monthly:    determinism.Zero("USD"),
		Hourly:     determinism.Zero("USD"),
		Confidence: 1.0,
		SnapshotID: snapshotID,
	}
}

// DegradeForUnknown reduces confidence due to an unknown value
func (c *ConfidenceBoundCost) DegradeForUnknown(source, reason string, impact float64, component string) {
	c.Confidence *= (1.0 - impact)
	c.IsDegraded = true
	c.Factors = append(c.Factors, ConfidenceFactor{
		Source:    source,
		Reason:    reason,
		Impact:    impact,
		IsUnknown: true,
		Component: component,
	})
}

// DegradeForMissing reduces confidence due to missing data
func (c *ConfidenceBoundCost) DegradeForMissing(source, reason string, impact float64, component string) {
	c.Confidence *= (1.0 - impact)
	c.IsDegraded = true
	c.Factors = append(c.Factors, ConfidenceFactor{
		Source:    source,
		Reason:    reason,
		Impact:    impact,
		IsUnknown: false,
		Component: component,
	})
}

// Add adds two costs, combining confidence
func (c *ConfidenceBoundCost) Add(other *ConfidenceBoundCost) *ConfidenceBoundCost {
	return &ConfidenceBoundCost{
		Monthly:    c.Monthly.Add(other.Monthly),
		Hourly:     c.Hourly.Add(other.Hourly),
		Confidence: c.Confidence * other.Confidence, // Compound confidence
		Factors:    append(c.Factors, other.Factors...),
		IsDegraded: c.IsDegraded || other.IsDegraded,
		SnapshotID: c.SnapshotID,
	}
}

// ConfidenceLevel returns a human-readable confidence level
func (c *ConfidenceBoundCost) ConfidenceLevel() string {
	switch {
	case c.Confidence >= 0.9:
		return "high"
	case c.Confidence >= 0.7:
		return "medium"
	case c.Confidence >= 0.5:
		return "low"
	default:
		return "very_low"
	}
}

// UnknownDrivenDegradation tracks degradation from unknown values
type UnknownDrivenDegradation struct {
	// Instance affected
	InstanceID model.InstanceID
	Address    model.CanonicalAddress

	// What was unknown
	UnknownAttribute string
	UnknownReason    string

	// Impact on cost
	ConfidenceImpact float64
	CostImpact       *CostImpactEstimate
}

// CostImpactEstimate estimates how much an unknown affects cost
type CostImpactEstimate struct {
	// Range of possible costs
	MinCost determinism.Money
	MaxCost determinism.Money

	// Most likely cost
	LikelyCost determinism.Money

	// How we estimated
	Method string
}

// CostWithProvenance is a cost with full provenance chain
type CostWithProvenance struct {
	// The cost
	Cost *ConfidenceBoundCost

	// Instance identity
	Identity *model.InstanceIdentity

	// Component name
	Component string

	// Rate used
	Rate *RateProvenance

	// Usage applied
	Usage *UsageProvenance

	// Formula
	Formula *FormulaProvenance
}

// RateProvenance tracks which rate was used
type RateProvenance struct {
	RateID       pricing.RateID
	RateKey      pricing.RateKey
	Price        string
	Unit         string
	Currency     string
	WasFound     bool
	MissingReason string
}

// UsageProvenance tracks where usage came from
type UsageProvenance struct {
	Value       float64
	Unit        string
	Source      UsageSource
	Confidence  float64
	WasUnknown  bool
	Assumptions []string
}

// UsageSource indicates usage data origin
type UsageSource int

const (
	UsageSourceDefault UsageSource = iota
	UsageSourceOverride
	UsageSourceProfile
	UsageSourceHistorical
	UsageSourceUnknown
)

// FormulaProvenance tracks how cost was calculated
type FormulaProvenance struct {
	Name       string
	Expression string
	Inputs     map[string]FormulaInput
	Output     string
}

// FormulaInput is a formula input with source
type FormulaInput struct {
	Value      string
	Source     string // "rate", "usage", "constant"
	Confidence float64
}

// InstanceCostResult is the complete result for one instance
type InstanceCostResult struct {
	// Identity
	Identity *model.InstanceIdentity

	// Total cost with confidence
	Total *ConfidenceBoundCost

	// Per-component costs
	Components []*CostWithProvenance

	// All degradation factors
	Degradations []*UnknownDrivenDegradation

	// Summary
	IsFullConfidence bool
	DegradedCount    int
	UnknownCount     int
}

// NewInstanceCostResult creates a new result
func NewInstanceCostResult(identity *model.InstanceIdentity, snapshotID pricing.SnapshotID) *InstanceCostResult {
	return &InstanceCostResult{
		Identity:         identity,
		Total:            ZeroCost(snapshotID),
		Components:       []*CostWithProvenance{},
		Degradations:     []*UnknownDrivenDegradation{},
		IsFullConfidence: true,
	}
}

// AddComponent adds a component cost
func (r *InstanceCostResult) AddComponent(comp *CostWithProvenance) {
	r.Components = append(r.Components, comp)
	r.Total = r.Total.Add(comp.Cost)

	if comp.Cost.IsDegraded {
		r.IsFullConfidence = false
		r.DegradedCount++
	}

	if comp.Usage != nil && comp.Usage.WasUnknown {
		r.UnknownCount++
		r.Degradations = append(r.Degradations, &UnknownDrivenDegradation{
			InstanceID:       r.Identity.ID,
			Address:          r.Identity.Canonical,
			UnknownAttribute: fmt.Sprintf("%s.usage", comp.Component),
			UnknownReason:    "usage value was unknown",
			ConfidenceImpact: 1.0 - comp.Cost.Confidence,
		})
	}
}

// AggregatedCostResult is the complete estimation result
type AggregatedCostResult struct {
	// Instance results
	Instances []*InstanceCostResult

	// Totals with confidence
	TotalMonthly    determinism.Money
	TotalHourly     determinism.Money
	TotalConfidence float64

	// Aggregated degradation info
	TotalDegraded  int
	TotalUnknowns  int
	DegradedCost   determinism.Money // Amount of cost that's degraded

	// Snapshot
	SnapshotID pricing.SnapshotID
}

// NewAggregatedCostResult creates a new aggregated result
func NewAggregatedCostResult(snapshotID pricing.SnapshotID) *AggregatedCostResult {
	return &AggregatedCostResult{
		Instances:       []*InstanceCostResult{},
		TotalMonthly:    determinism.Zero("USD"),
		TotalHourly:     determinism.Zero("USD"),
		TotalConfidence: 1.0,
		DegradedCost:    determinism.Zero("USD"),
		SnapshotID:      snapshotID,
	}
}

// Add adds an instance result
func (r *AggregatedCostResult) Add(inst *InstanceCostResult) {
	r.Instances = append(r.Instances, inst)
	r.TotalMonthly = r.TotalMonthly.Add(inst.Total.Monthly)
	r.TotalHourly = r.TotalHourly.Add(inst.Total.Hourly)
	r.TotalConfidence *= inst.Total.Confidence

	if !inst.IsFullConfidence {
		r.TotalDegraded++
		r.DegradedCost = r.DegradedCost.Add(inst.Total.Monthly)
	}
	r.TotalUnknowns += inst.UnknownCount
}

// ConfidenceLevel returns overall confidence level
func (r *AggregatedCostResult) ConfidenceLevel() string {
	switch {
	case r.TotalConfidence >= 0.9:
		return "high"
	case r.TotalConfidence >= 0.7:
		return "medium"
	case r.TotalConfidence >= 0.5:
		return "low"
	default:
		return "very_low"
	}
}
