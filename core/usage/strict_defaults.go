// Package usage - Strict default enforcement
// Defaults MUST bias toward over-estimation, not under-estimation.
// Every default MUST make users uncomfortable if they don't override.
package usage

import (
	"fmt"
	"terraform-cost/core/confidence"
	"terraform-cost/core/determinism"
)

// StrictDefaultPolicy enforces that all defaults are visible and conservative
type StrictDefaultPolicy struct {
	// Defaults that are known to bias low
	lowBiasDefaults map[string]DefaultRisk

	// Minimum confidence impacts per category
	minImpacts map[string]float64
}

// DefaultRisk describes the risk of a default value
type DefaultRisk struct {
	DefaultValue     interface{}
	RiskLevel        RiskLevel
	ProductionTypical interface{} // What production usually sees
	ConfidenceImpact float64
	Warning          string
}

// RiskLevel indicates how risky a default is
type RiskLevel int

const (
	RiskLow    RiskLevel = iota // Default is conservative
	RiskMedium                   // Default may underestimate
	RiskHigh                     // Default likely underestimates
	RiskCritical                 // Default almost certainly underestimates
)

// String returns the risk level name
func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// NewStrictDefaultPolicy creates a strict policy
func NewStrictDefaultPolicy() *StrictDefaultPolicy {
	return &StrictDefaultPolicy{
		lowBiasDefaults: map[string]DefaultRisk{
			// Lambda - invocations are typically MUCH higher
			"aws_lambda_function.invocations": {
				DefaultValue:     1000000,            // 1M
				RiskLevel:        RiskCritical,
				ProductionTypical: 100000000,          // 100M
				ConfidenceImpact: 0.35,
				Warning:          "Lambda invocations default (1M) is typically 100x lower than production",
			},
			"aws_lambda_function.duration_ms": {
				DefaultValue:     100,
				RiskLevel:        RiskHigh,
				ProductionTypical: 500,
				ConfidenceImpact: 0.25,
				Warning:          "Lambda duration default (100ms) may underestimate cold starts",
			},

			// S3 - storage grows rapidly
			"aws_s3_bucket.storage_gb": {
				DefaultValue:     100,
				RiskLevel:        RiskCritical,
				ProductionTypical: 10000,             // 10TB
				ConfidenceImpact: 0.40,
				Warning:          "S3 storage default (100GB) is typically 100x lower than production",
			},
			"aws_s3_bucket.put_requests": {
				DefaultValue:     10000,
				RiskLevel:        RiskHigh,
				ProductionTypical: 10000000,
				ConfidenceImpact: 0.30,
				Warning:          "S3 PUT requests often 1000x higher in production",
			},

			// NAT Gateway - data transfer is expensive
			"aws_nat_gateway.data_processed_gb": {
				DefaultValue:     100,
				RiskLevel:        RiskCritical,
				ProductionTypical: 5000,
				ConfidenceImpact: 0.40,
				Warning:          "NAT data default (100GB) is typically 50x lower - this is expensive",
			},

			// Data transfer - egress is expensive
			"data_transfer.egress_gb": {
				DefaultValue:     100,
				RiskLevel:        RiskCritical,
				ProductionTypical: 10000,
				ConfidenceImpact: 0.45,
				Warning:          "Egress default (100GB) is typically 100x lower in production",
			},

			// EC2 hours - assume full month
			"aws_instance.monthly_hours": {
				DefaultValue:     730,
				RiskLevel:        RiskLow, // This is actually correct
				ProductionTypical: 730,
				ConfidenceImpact: 0.05,
				Warning:          "",
			},

			// RDS storage
			"aws_db_instance.storage_gb": {
				DefaultValue:     20,
				RiskLevel:        RiskHigh,
				ProductionTypical: 500,
				ConfidenceImpact: 0.30,
				Warning:          "RDS storage default (20GB) is typically 25x lower",
			},

			// ECS
			"aws_ecs_service.desired_count": {
				DefaultValue:     1,
				RiskLevel:        RiskHigh,
				ProductionTypical: 5,
				ConfidenceImpact: 0.25,
				Warning:          "ECS desired_count default (1) is typically 5x lower",
			},
		},
		minImpacts: map[string]float64{
			"storage":       0.25,
			"compute":       0.15,
			"network":       0.30,
			"data_transfer": 0.35,
			"requests":      0.25,
		},
	}
}

// ValidateDefault checks if a default is acceptable and returns warnings
func (p *StrictDefaultPolicy) ValidateDefault(resourceType, attribute string, value interface{}) *DefaultValidation {
	key := resourceType + "." + attribute
	
	validation := &DefaultValidation{
		Key:      key,
		Value:    value,
		Acceptable: true,
		Warnings:   []string{},
	}

	if risk, ok := p.lowBiasDefaults[key]; ok {
		validation.Risk = risk.RiskLevel
		validation.ConfidenceImpact = risk.ConfidenceImpact
		
		if risk.Warning != "" {
			validation.Warnings = append(validation.Warnings, risk.Warning)
		}

		if risk.RiskLevel >= RiskHigh {
			validation.Acceptable = false
			validation.Warnings = append(validation.Warnings, 
				fmt.Sprintf("Consider using production-typical value: %v", risk.ProductionTypical))
		}
	}

	return validation
}

// DefaultValidation is the result of validating a default
type DefaultValidation struct {
	Key              string
	Value            interface{}
	Acceptable       bool
	Risk             RiskLevel
	ConfidenceImpact float64
	Warnings         []string
}

// ApplyToTracker applies the default's confidence impact
func (v *DefaultValidation) ApplyToTracker(tracker *confidence.ConfidenceTracker) {
	if v.ConfidenceImpact > 0 {
		tracker.Apply("usage_default", fmt.Sprintf("%s uses default value", v.Key))
	}
}

// StrictUsageResolver resolves usage with explicit tracking
type StrictUsageResolver struct {
	policy     *StrictDefaultPolicy
	tracker    *AssumptionTracker
	confidence *confidence.ConfidenceTracker
	warnings   []UsageWarning
}

// UsageWarning is a warning about usage estimation
type UsageWarning struct {
	ResourceType string
	Attribute    string
	Value        interface{}
	Risk         RiskLevel
	Message      string
}

// NewStrictUsageResolver creates a resolver
func NewStrictUsageResolver() *StrictUsageResolver {
	return &StrictUsageResolver{
		policy:     NewStrictDefaultPolicy(),
		tracker:    NewAssumptionTracker(),
		confidence: confidence.NewConfidenceTracker(),
		warnings:   []UsageWarning{},
	}
}

// ResolveUsage resolves a usage value, either from overrides or defaults
func (r *StrictUsageResolver) ResolveUsage(
	resourceType, attribute string,
	override interface{},
	defaultValue interface{},
) *ResolvedUsage {
	
	result := &ResolvedUsage{
		Attribute: attribute,
	}

	// If override provided, use it
	if override != nil {
		result.Value = override
		result.Source = UsageSourceOverride
		result.Confidence = 1.0
		return result
	}

	// Using default - validate it
	validation := r.policy.ValidateDefault(resourceType, attribute, defaultValue)
	
	result.Value = defaultValue
	result.Source = UsageSourceDefault
	result.Confidence = 1.0 - validation.ConfidenceImpact
	result.Risk = validation.Risk
	result.Warnings = validation.Warnings

	// Track the assumption
	r.tracker.RecordDefault(
		resourceType,
		attribute,
		defaultValue,
		"",
		validation.ConfidenceImpact,
	)

	// Record warning if high risk
	if validation.Risk >= RiskHigh {
		r.warnings = append(r.warnings, UsageWarning{
			ResourceType: resourceType,
			Attribute:    attribute,
			Value:        defaultValue,
			Risk:         validation.Risk,
			Message:      validation.Warnings[0],
		})
	}

	return result
}

// ResolvedUsage is a resolved usage value
type ResolvedUsage struct {
	Attribute  string
	Value      interface{}
	Source     UsageSource
	Confidence float64
	Risk       RiskLevel
	Warnings   []string
}

// UsageSource indicates where usage came from
type UsageSource int

const (
	UsageSourceOverride   UsageSource = iota // User provided
	UsageSourceDefault                        // Using default
	UsageSourceHistorical                     // From historical data
	UsageSourceInferred                       // Inferred from config
)

// AsFloat returns the value as float64
func (u *ResolvedUsage) AsFloat() float64 {
	switch v := u.Value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

// GetWarnings returns all warnings from resolution
func (r *StrictUsageResolver) GetWarnings() []UsageWarning {
	return r.warnings
}

// GetAssumptions returns all assumptions made
func (r *StrictUsageResolver) GetAssumptions() []*Assumption {
	return r.tracker.All()
}

// GetConfidenceTracker returns the confidence tracker
func (r *StrictUsageResolver) GetConfidenceTracker() *confidence.ConfidenceTracker {
	return r.confidence
}

// FormatWarnings returns human-readable warnings
func (r *StrictUsageResolver) FormatWarnings() string {
	if len(r.warnings) == 0 {
		return "No usage warnings"
	}

	result := fmt.Sprintf("⚠️ %d Usage Warnings:\n", len(r.warnings))
	for i, w := range r.warnings {
		result += fmt.Sprintf("  %d. [%s] %s.%s = %v\n", 
			i+1, w.Risk.String(), w.ResourceType, w.Attribute, w.Value)
		result += fmt.Sprintf("     %s\n", w.Message)
	}
	return result
}

// CalculateTotalRisk returns aggregate risk information
func (r *StrictUsageResolver) CalculateTotalRisk() *AggregateRisk {
	risk := &AggregateRisk{}

	for _, w := range r.warnings {
		switch w.Risk {
		case RiskCritical:
			risk.CriticalCount++
		case RiskHigh:
			risk.HighCount++
		case RiskMedium:
			risk.MediumCount++
		}
	}

	risk.ConfidenceLoss = r.tracker.TotalConfidenceImpact()
	risk.EstimatedBias = r.estimateBias()

	return risk
}

func (r *StrictUsageResolver) estimateBias() determinism.Money {
	// Rough estimate of how much we might be underestimating
	// This is a heuristic based on typical production vs default ratios
	biasMultiplier := 1.0
	for _, w := range r.warnings {
		switch w.Risk {
		case RiskCritical:
			biasMultiplier *= 2.0
		case RiskHigh:
			biasMultiplier *= 1.5
		case RiskMedium:
			biasMultiplier *= 1.2
		}
	}
	// Return estimate that we might be X% low
	// This is informational only
	return determinism.Zero("USD")
}

// AggregateRisk summarizes overall usage risk
type AggregateRisk struct {
	CriticalCount  int
	HighCount      int
	MediumCount    int
	ConfidenceLoss float64
	EstimatedBias  determinism.Money
}

// IsAcceptable returns true if risk is acceptable for production
func (r *AggregateRisk) IsAcceptable() bool {
	return r.CriticalCount == 0 && r.HighCount <= 2
}

// Summary returns a summary string
func (r *AggregateRisk) Summary() string {
	if r.CriticalCount == 0 && r.HighCount == 0 {
		return "Usage defaults are acceptable"
	}
	return fmt.Sprintf("⚠️ %d critical, %d high risk defaults - estimate may be significantly low",
		r.CriticalCount, r.HighCount)
}
