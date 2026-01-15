// Package types - Usage estimation types
package types

// UsageMetric identifies a measurable usage dimension
type UsageMetric string

// Common usage metrics across all providers
const (
	// Time-based metrics
	MetricMonthlyHours UsageMetric = "monthly_hours"
	MetricDailyHours   UsageMetric = "daily_hours"

	// Storage metrics
	MetricMonthlyGB        UsageMetric = "monthly_gb"
	MetricMonthlyGBStorage UsageMetric = "monthly_gb_storage"
	MetricMonthlySnapshots UsageMetric = "monthly_snapshots"

	// Transfer metrics
	MetricMonthlyGBTransferOut   UsageMetric = "monthly_gb_transfer_out"
	MetricMonthlyGBTransferIn    UsageMetric = "monthly_gb_transfer_in"
	MetricMonthlyGBInterRegion   UsageMetric = "monthly_gb_inter_region"
	MetricMonthlyGBInterAZ       UsageMetric = "monthly_gb_inter_az"

	// Request metrics
	MetricMonthlyRequests       UsageMetric = "monthly_requests"
	MetricMonthlyReadRequests   UsageMetric = "monthly_read_requests"
	MetricMonthlyWriteRequests  UsageMetric = "monthly_write_requests"
	MetricMonthlyAPIRequests    UsageMetric = "monthly_api_requests"

	// Operation metrics
	MetricMonthlyOperations     UsageMetric = "monthly_operations"
	MetricMonthlyGetOperations  UsageMetric = "monthly_get_operations"
	MetricMonthlyPutOperations  UsageMetric = "monthly_put_operations"
	MetricMonthlyListOperations UsageMetric = "monthly_list_operations"

	// Compute metrics
	MetricMonthlyCPUCredits UsageMetric = "monthly_cpu_credits"
	MetricMonthlyVCPUHours  UsageMetric = "monthly_vcpu_hours"
	MetricMonthlyGBHours    UsageMetric = "monthly_gb_hours"

	// Database metrics
	MetricMonthlyIORequests       UsageMetric = "monthly_io_requests"
	MetricMonthlyBackupStorageGB  UsageMetric = "monthly_backup_storage_gb"

	// Serverless metrics
	MetricMonthlyInvocations   UsageMetric = "monthly_invocations"
	MetricMonthlyGBSeconds     UsageMetric = "monthly_gb_seconds"
	MetricMonthlyDurationMs    UsageMetric = "monthly_duration_ms"
)

// String returns the string representation
func (m UsageMetric) String() string {
	return string(m)
}

// UsageVector represents estimated usage for a specific metric
type UsageVector struct {
	// Metric is the usage dimension being measured
	Metric UsageMetric `json:"metric"`

	// Value is the estimated usage amount
	Value float64 `json:"value"`

	// Confidence is the confidence level (0.0 to 1.0)
	Confidence float64 `json:"confidence"`

	// Source indicates where this estimate came from
	Source UsageSource `json:"source"`

	// Min is the minimum expected value (for range estimates)
	Min *float64 `json:"min,omitempty"`

	// Max is the maximum expected value (for range estimates)
	Max *float64 `json:"max,omitempty"`

	// Description explains the usage estimate
	Description string `json:"description,omitempty"`
}

// UsageSource indicates the origin of usage data
type UsageSource string

const (
	// SourceDefault uses built-in default values
	SourceDefault UsageSource = "default"

	// SourceOverride uses user-provided values
	SourceOverride UsageSource = "override"

	// SourceProfile uses a pre-defined usage profile
	SourceProfile UsageSource = "profile"

	// SourceHistorical uses historical data analysis
	SourceHistorical UsageSource = "historical"

	// SourceML uses machine learning predictions
	SourceML UsageSource = "ml"

	// SourceTerraform uses values from Terraform configuration
	SourceTerraform UsageSource = "terraform"
)

// String returns the string representation
func (s UsageSource) String() string {
	return string(s)
}

// UsageProfile represents a complete usage configuration for an estimation
type UsageProfile struct {
	// Name is the profile identifier
	Name string `json:"name"`

	// Description explains the profile
	Description string `json:"description,omitempty"`

	// Environment is the target environment (dev, staging, prod)
	Environment string `json:"environment,omitempty"`

	// Defaults are default usage values by resource type
	Defaults map[string][]UsageVector `json:"defaults,omitempty"`

	// Overrides are resource-specific usage overrides
	Overrides map[ResourceAddress][]UsageVector `json:"overrides,omitempty"`

	// Multipliers scale usage values for scenario analysis
	Multipliers map[UsageMetric]float64 `json:"multipliers,omitempty"`
}

// NewUsageProfile creates a new usage profile with the given name
func NewUsageProfile(name string) *UsageProfile {
	return &UsageProfile{
		Name:        name,
		Defaults:    make(map[string][]UsageVector),
		Overrides:   make(map[ResourceAddress][]UsageVector),
		Multipliers: make(map[UsageMetric]float64),
	}
}

// GetOverrides returns usage overrides for a specific resource
func (p *UsageProfile) GetOverrides(addr ResourceAddress) []UsageVector {
	if p == nil {
		return nil
	}
	return p.Overrides[addr]
}

// GetDefaults returns default usage for a resource type
func (p *UsageProfile) GetDefaults(resourceType string) []UsageVector {
	if p == nil {
		return nil
	}
	return p.Defaults[resourceType]
}

// ApplyMultiplier applies a multiplier to usage vectors
func (p *UsageProfile) ApplyMultiplier(vectors []UsageVector) []UsageVector {
	if p == nil || len(p.Multipliers) == 0 {
		return vectors
	}

	result := make([]UsageVector, len(vectors))
	for i, v := range vectors {
		result[i] = v
		if multiplier, ok := p.Multipliers[v.Metric]; ok {
			result[i].Value = v.Value * multiplier
			if v.Min != nil {
				min := *v.Min * multiplier
				result[i].Min = &min
			}
			if v.Max != nil {
				max := *v.Max * multiplier
				result[i].Max = &max
			}
		}
	}
	return result
}

// UsageContext provides context for usage estimation
type UsageContext struct {
	// Profile is the active usage profile
	Profile *UsageProfile

	// Environment is the target environment
	Environment string

	// Region is the deployment region
	Region Region

	// Scenario is the estimation scenario (min, typical, max)
	Scenario UsageScenario
}

// UsageScenario represents different usage estimation scenarios
type UsageScenario string

const (
	ScenarioMin     UsageScenario = "min"
	ScenarioTypical UsageScenario = "typical"
	ScenarioMax     UsageScenario = "max"
)
