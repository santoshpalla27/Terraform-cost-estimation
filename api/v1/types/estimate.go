// Package types - Public API DTOs
// This package contains ONLY data transfer objects for the public API.
// NO ENGINE IMPORTS ALLOWED - this is the stable API contract.
package types

import "time"

// EstimateResponse is the public response for POST /api/v1/estimate
// This struct is the API contract - changes are breaking changes.
type EstimateResponse struct {
	Metadata MetadataDTO       `json:"metadata"`
	Summary  SummaryDTO        `json:"summary"`
	Costs    []CostNodeDTO     `json:"costs"`
	Symbolic []SymbolicCostDTO `json:"symbolic_costs,omitempty"`
	Warnings []string          `json:"warnings,omitempty"`
	Policies []PolicyResultDTO `json:"policy_results,omitempty"`
}

// MetadataDTO provides audit and reproducibility information
type MetadataDTO struct {
	// InputHash is SHA256 of normalized input - enables reproducibility
	InputHash string `json:"input_hash"`

	// EngineVersion identifies the engine that produced this result
	EngineVersion string `json:"engine_version"`

	// PricingSnapshot identifies the pricing data used
	PricingSnapshot string `json:"pricing_snapshot"`

	// Mode is "strict" or "permissive"
	Mode string `json:"mode"`

	// Timestamp is when the estimation was performed
	Timestamp time.Time `json:"timestamp"`

	// DurationMs is processing time in milliseconds
	DurationMs int64 `json:"duration_ms"`

	// APIVersion is the API version (e.g., "v1")
	APIVersion string `json:"api_version"`
}

// SummaryDTO is the top-level cost summary
type SummaryDTO struct {
	// TotalMonthlyCost as decimal string (e.g., "1234.56")
	TotalMonthlyCost string `json:"total_monthly_cost"`

	// TotalHourlyCost as decimal string
	TotalHourlyCost string `json:"total_hourly_cost,omitempty"`

	// Currency is the currency code (e.g., "USD")
	Currency string `json:"currency"`

	// Confidence is 0.0 to 1.0
	Confidence float64 `json:"confidence"`

	// ConfidenceLevel is human-readable: "high", "medium", "low", "unknown"
	ConfidenceLevel string `json:"confidence_level"`

	// ConfidenceReason explains why confidence is not 100%
	ConfidenceReason string `json:"confidence_reason,omitempty"`

	// ResourceCount is total resources estimated
	ResourceCount int `json:"resource_count"`

	// SymbolicCount is resources with unknown cardinality
	SymbolicCount int `json:"symbolic_count"`
}

// CostNodeDTO represents a cost node in the graph
type CostNodeDTO struct {
	// Address is the Terraform resource address
	Address string `json:"address"`

	// Type is the resource type (e.g., "aws_instance")
	Type string `json:"type"`

	// ProviderAlias is the provider identity (e.g., "aws.prod")
	ProviderAlias string `json:"provider_alias"`

	// MonthlyCost as decimal string
	MonthlyCost string `json:"monthly_cost"`

	// HourlyCost as decimal string
	HourlyCost string `json:"hourly_cost,omitempty"`

	// Confidence for this specific resource
	Confidence float64 `json:"confidence"`

	// IsSymbolic indicates this resource has unknown cardinality
	IsSymbolic bool `json:"is_symbolic,omitempty"`

	// Components are cost sub-components
	Components []CostComponentDTO `json:"components,omitempty"`

	// Lineage explains dependency chain
	Lineage *LineageDTO `json:"lineage,omitempty"`

	// Children are nested resources (for modules)
	Children []CostNodeDTO `json:"children,omitempty"`
}

// CostComponentDTO is a sub-component of resource cost
type CostComponentDTO struct {
	// Name of the component (e.g., "compute", "storage")
	Name string `json:"name"`

	// MonthlyCost as decimal string
	MonthlyCost string `json:"monthly_cost"`

	// Unit of measurement (e.g., "hours", "GB-month")
	Unit string `json:"unit,omitempty"`

	// Quantity used
	Quantity string `json:"quantity,omitempty"`

	// UnitPrice per unit
	UnitPrice string `json:"unit_price,omitempty"`
}

// LineageDTO explains why a cost exists
type LineageDTO struct {
	// DependencyPath from root to this node
	DependencyPath []string `json:"dependency_path"`

	// Explanation is human-readable
	Explanation string `json:"explanation"`
}

// SymbolicCostDTO represents a cost that cannot be computed numerically
type SymbolicCostDTO struct {
	// Address of the resource
	Address string `json:"address"`

	// Reason why cost cannot be computed
	Reason string `json:"reason"`

	// Expression that caused unknowability
	Expression string `json:"expression,omitempty"`

	// LowerBound if estimable
	LowerBound string `json:"lower_bound,omitempty"`

	// UpperBound if estimable (omit for unbounded)
	UpperBound string `json:"upper_bound,omitempty"`

	// IsUnbounded if no upper limit can be estimated
	IsUnbounded bool `json:"is_unbounded,omitempty"`
}

// PolicyResultDTO is the result of a policy evaluation
type PolicyResultDTO struct {
	// Name of the policy
	Name string `json:"name"`

	// Passed indicates if the policy passed
	Passed bool `json:"passed"`

	// Severity: "info", "warning", "error", "block"
	Severity string `json:"severity"`

	// Message explains the result
	Message string `json:"message"`
}
