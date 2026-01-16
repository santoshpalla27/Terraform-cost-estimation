// Package policy - Full lineage access for policies
// Policies can see the COMPLETE derivation chain.
package policy

import (
	"fmt"

	"terraform-cost/core/determinism"
	"terraform-cost/core/model"
	"terraform-cost/core/pricing"
)

// FullLineageContext provides COMPLETE cost derivation information to policies
type FullLineageContext struct {
	// Per-instance costs with full derivation
	Instances map[model.InstanceID]*InstanceLineage

	// Rollups
	TotalMonthlyCost determinism.Money
	TotalHourlyCost  determinism.Money

	// Confidence breakdown
	OverallConfidence    float64
	ConfidenceByInstance map[model.InstanceID]float64
	ConfidenceByComponent map[string]float64

	// Low confidence items
	LowConfidenceItems []LowConfidenceItem

	// Unknown tracking
	Unknowns []UnknownInfo
	UnknownsByInstance map[model.InstanceID][]UnknownInfo

	// Snapshot reference
	Snapshot SnapshotInfo
}

// InstanceLineage is the FULL derivation for a single instance
type InstanceLineage struct {
	// Instance identity
	InstanceID   model.InstanceID
	Address      model.InstanceAddress
	DefinitionID model.DefinitionID
	InstanceKey  model.InstanceKey

	// Provider context
	Provider string
	Region   string

	// Cost summary
	MonthlyCost determinism.Money
	HourlyCost  determinism.Money
	Confidence  float64

	// FULL component breakdown
	Components []ComponentLineage

	// Resource tags
	Tags map[string]string

	// Unknowns affecting this instance
	Unknowns []UnknownInfo

	// Degradation reasons
	DegradedParts []DegradationInfo
}

// ComponentLineage is the FULL derivation for a cost component
type ComponentLineage struct {
	// Component identity
	Name        string
	ResourceType string

	// Cost
	MonthlyCost determinism.Money
	HourlyCost  determinism.Money
	Confidence  float64

	// RATE: Which pricing rate was used
	Rate RateLineage

	// USAGE: Where usage data came from
	Usage UsageLineage

	// FORMULA: How cost was calculated
	Formula FormulaLineage

	// Is this component degraded?
	IsDegraded       bool
	DegradationReason string
}

// RateLineage tracks the pricing rate used
type RateLineage struct {
	// Rate identity
	RateID      pricing.RateID
	RateKey     pricing.RateKey

	// Rate details
	Price       string
	Unit        string
	Currency    string
	Description string

	// Tier info (for tiered pricing)
	Tier        *TierInfo

	// Was this rate found?
	Found       bool
	MissingReason string
}

// TierInfo describes pricing tier
type TierInfo struct {
	TierIndex   int
	StartUsage  string
	EndUsage    string
	TierPrice   string
}

// UsageLineage tracks where usage data came from
type UsageLineage struct {
	// Source of usage data
	Source UsageSourceType

	// The value used
	Value       float64
	Unit        string

	// Confidence in this value
	Confidence  float64

	// Was this overridden by user?
	IsOverridden bool

	// Profile used (if any)
	Profile     string

	// Assumptions made
	Assumptions []string

	// Was this unknown?
	IsUnknown   bool
	UnknownReason string
}

// UsageSourceType indicates where usage came from
type UsageSourceType int

const (
	UsageFromDefault UsageSourceType = iota
	UsageFromProfile
	UsageFromOverride
	UsageFromHistorical
	UsageFromEstimate
	UsageUnknown
)

// String returns the source name
func (s UsageSourceType) String() string {
	switch s {
	case UsageFromDefault:
		return "default"
	case UsageFromProfile:
		return "profile"
	case UsageFromOverride:
		return "override"
	case UsageFromHistorical:
		return "historical"
	case UsageFromEstimate:
		return "estimate"
	case UsageUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// FormulaLineage tracks how cost was calculated
type FormulaLineage struct {
	// Formula name
	Name       string

	// Readable expression
	Expression string

	// All inputs used
	Inputs     map[string]FormulaInput

	// The output
	Output     string
}

// FormulaInput is a single input to a formula
type FormulaInput struct {
	Name       string
	Value      string
	Source     string // "rate", "usage", "constant"
	Confidence float64
}

// UnknownInfo describes an unknown value
type UnknownInfo struct {
	// What is unknown
	Address   string
	Component string
	Attribute string

	// Why it's unknown
	Reason    string

	// Impact on cost
	Impact    UnknownImpact
}

// UnknownImpact describes how an unknown affects cost
type UnknownImpact int

const (
	ImpactHigh   UnknownImpact = iota // Cost is unreliable
	ImpactMedium                       // Cost is approximate
	ImpactLow                          // Cost is slightly affected
)

// String returns the impact level
func (i UnknownImpact) String() string {
	switch i {
	case ImpactHigh:
		return "high"
	case ImpactMedium:
		return "medium"
	case ImpactLow:
		return "low"
	default:
		return "unknown"
	}
}

// DegradationInfo describes why a cost is degraded
type DegradationInfo struct {
	Component string
	Reason    string
	Impact    float64
}

// SnapshotInfo is the pricing snapshot reference
type SnapshotInfo struct {
	ID          pricing.SnapshotID
	ContentHash string
	Provider    string
	Region      string
	CreatedAt   string
	EffectiveAt string
	IsStale     bool
}

// LineageAwarePolicy is a policy that uses full lineage
type LineageAwarePolicy interface {
	Name() string
	EvaluateWithLineage(ctx *FullLineageContext) (*LineageAwarePolicyResult, error)
}

// LineageAwarePolicyResult is the result of a lineage-aware policy
type LineageAwarePolicyResult struct {
	Passed  bool
	Message string

	// What was analyzed
	AnalyzedInstances int
	AnalyzedComponents int

	// What failed
	FailedInstances   []model.InstanceID
	FailedComponents  []string

	// Cost impact
	AffectedCost determinism.Money

	// Full lineage for failures (for explainability)
	FailureLineage []*InstanceLineage

	// Recommendations
	Recommendations []string
}

// UsageConfidencePolicy fails if usage confidence is too low
type UsageConfidencePolicy struct {
	name          string
	minConfidence float64
}

// NewUsageConfidencePolicy creates a usage confidence policy
func NewUsageConfidencePolicy(name string, minConfidence float64) *UsageConfidencePolicy {
	return &UsageConfidencePolicy{name: name, minConfidence: minConfidence}
}

// Name returns the policy name
func (p *UsageConfidencePolicy) Name() string { return p.name }

// EvaluateWithLineage evaluates with full lineage access
func (p *UsageConfidencePolicy) EvaluateWithLineage(ctx *FullLineageContext) (*LineageAwarePolicyResult, error) {
	result := &LineageAwarePolicyResult{
		Passed: true,
	}

	var lowConfidenceInstances []*InstanceLineage
	var lowConfidenceComponents []string

	for _, inst := range ctx.Instances {
		result.AnalyzedInstances++

		for _, comp := range inst.Components {
			result.AnalyzedComponents++

			// Check USAGE confidence specifically
			if comp.Usage.Confidence < p.minConfidence {
				result.Passed = false

				if !containsID(result.FailedInstances, inst.InstanceID) {
					result.FailedInstances = append(result.FailedInstances, inst.InstanceID)
					lowConfidenceInstances = append(lowConfidenceInstances, inst)
				}

				lowConfidenceComponents = append(lowConfidenceComponents,
					string(inst.Address)+"/"+comp.Name)

				result.AffectedCost = result.AffectedCost.Add(comp.MonthlyCost)
			}
		}
	}

	if !result.Passed {
		result.Message = fmt.Sprintf(
			"%d components have usage confidence below %.0f%%",
			len(lowConfidenceComponents), p.minConfidence*100)

		result.FailedComponents = lowConfidenceComponents
		result.FailureLineage = lowConfidenceInstances

		result.Recommendations = []string{
			"Provide usage overrides for low-confidence components",
			"Use a usage profile to set expected values",
			"Review assumptions in the usage estimation",
		}
	} else {
		result.Message = fmt.Sprintf(
			"All %d components meet %.0f%% usage confidence",
			result.AnalyzedComponents, p.minConfidence*100)
	}

	return result, nil
}

// FormulaAuditPolicy checks that all formulas are documented
type FormulaAuditPolicy struct {
	name string
}

// NewFormulaAuditPolicy creates a formula audit policy
func NewFormulaAuditPolicy(name string) *FormulaAuditPolicy {
	return &FormulaAuditPolicy{name: name}
}

// Name returns the policy name
func (p *FormulaAuditPolicy) Name() string { return p.name }

// EvaluateWithLineage evaluates with full lineage access
func (p *FormulaAuditPolicy) EvaluateWithLineage(ctx *FullLineageContext) (*LineageAwarePolicyResult, error) {
	result := &LineageAwarePolicyResult{
		Passed: true,
	}

	var undocumentedFormulas []string

	for _, inst := range ctx.Instances {
		result.AnalyzedInstances++

		for _, comp := range inst.Components {
			result.AnalyzedComponents++

			// Check formula documentation
			if comp.Formula.Name == "" || comp.Formula.Expression == "" {
				result.Passed = false

				if !containsID(result.FailedInstances, inst.InstanceID) {
					result.FailedInstances = append(result.FailedInstances, inst.InstanceID)
				}

				undocumentedFormulas = append(undocumentedFormulas,
					string(inst.Address)+"/"+comp.Name)
			}
		}
	}

	if !result.Passed {
		result.Message = fmt.Sprintf(
			"%d formulas are undocumented",
			len(undocumentedFormulas))

		result.FailedComponents = undocumentedFormulas

		result.Recommendations = []string{
			"Ensure all pricing formulas are documented",
			"Review cost calculation logic",
		}
	} else {
		result.Message = fmt.Sprintf(
			"All %d formulas are documented",
			result.AnalyzedComponents)
	}

	return result, nil
}

func containsID(slice []model.InstanceID, id model.InstanceID) bool {
	for _, item := range slice {
		if item == id {
			return true
		}
	}
	return false
}
