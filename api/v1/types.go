// Package v1 - Versioned API types
// These DTOs are the STABLE contract - no engine imports allowed
// Changes to these types are breaking changes
package v1

import "time"

// EstimateRequest is the stable request contract for POST /api/v1/estimate
type EstimateRequest struct {
	// Source configuration
	Source SourceConfig `json:"source"`

	// Estimation mode
	Mode Mode `json:"mode"`

	// Usage profile (optional)
	UsageProfile string `json:"usage_profile,omitempty"`

	// Options
	Options EstimateOptions `json:"options,omitempty"`
}

// SourceConfig defines the Terraform source
type SourceConfig struct {
	// Type: "git", "upload", "local"
	Type string `json:"type"`

	// For git sources
	Repo string `json:"repo,omitempty"`
	Ref  string `json:"ref,omitempty"`

	// Path within source
	Path string `json:"path,omitempty"`

	// For upload sources
	UploadID string `json:"upload_id,omitempty"`
}

// Mode is the estimation mode
type Mode string

const (
	ModeStrict     Mode = "strict"
	ModePermissive Mode = "permissive"
)

// EstimateOptions are optional request parameters
type EstimateOptions struct {
	IncludeDependencyGraph bool `json:"include_dependency_graph,omitempty"`
	IncludeCostLineage     bool `json:"include_cost_lineage,omitempty"`
	IncludeAssumptions     bool `json:"include_assumptions,omitempty"`
}

// EstimateResponse is the stable response contract for POST /api/v1/estimate
type EstimateResponse struct {
	// Metadata for reproducibility
	Metadata ResponseMetadata `json:"metadata"`

	// Summary
	Summary CostSummary `json:"summary"`

	// Cost graph (if requested)
	CostGraph *CostGraph `json:"cost_graph,omitempty"`

	// Symbolic costs (unknown cardinality)
	SymbolicCosts []SymbolicCost `json:"symbolic_costs,omitempty"`

	// Warnings
	Warnings []Warning `json:"warnings,omitempty"`

	// Policy results
	PolicyResults []PolicyResult `json:"policy_results,omitempty"`
}

// ResponseMetadata provides audit/reproducibility information
type ResponseMetadata struct {
	// InputHash is SHA256 of normalized input
	InputHash string `json:"input_hash"`

	// EngineVersion is the engine version
	EngineVersion string `json:"engine_version"`

	// PricingSnapshot identifies the pricing data used
	PricingSnapshot string `json:"pricing_snapshot"`

	// Mode used for estimation
	Mode string `json:"mode"`

	// Timestamp of estimation
	Timestamp time.Time `json:"timestamp"`

	// DurationMs is the processing time
	DurationMs int64 `json:"duration_ms"`
}

// CostSummary is the top-level cost summary
type CostSummary struct {
	// TotalMonthlyCost in decimal string format
	TotalMonthlyCost string `json:"total_monthly_cost"`

	// TotalHourlyCost in decimal string format
	TotalHourlyCost string `json:"total_hourly_cost,omitempty"`

	// Currency code
	Currency string `json:"currency"`

	// Confidence score (0.0 to 1.0)
	Confidence float64 `json:"confidence"`

	// ConfidenceLevel is human-readable
	ConfidenceLevel string `json:"confidence_level"`

	// ConfidenceReason explains the confidence
	ConfidenceReason string `json:"confidence_reason,omitempty"`

	// ResourceCount is the number of resources
	ResourceCount int `json:"resource_count"`

	// SymbolicCount is resources with unknown cost
	SymbolicCount int `json:"symbolic_count"`
}

// CostGraph is the hierarchical cost structure
type CostGraph struct {
	// Modules in the graph
	Modules []ModuleCost `json:"modules"`
}

// ModuleCost is the cost of a module
type ModuleCost struct {
	// Address is the module address
	Address string `json:"address"`

	// MonthlyCost in decimal string
	MonthlyCost string `json:"monthly_cost"`

	// Confidence for this module
	Confidence float64 `json:"confidence"`

	// Resources in this module
	Resources []ResourceCost `json:"resources"`
}

// ResourceCost is the cost of a single resource
type ResourceCost struct {
	// Address is the resource address
	Address string `json:"address"`

	// ResourceType is the Terraform type
	ResourceType string `json:"resource_type"`

	// ProviderAlias is the provider identity
	ProviderAlias string `json:"provider_alias"`

	// MonthlyCost in decimal string
	MonthlyCost string `json:"monthly_cost"`

	// HourlyCost in decimal string
	HourlyCost string `json:"hourly_cost,omitempty"`

	// Confidence for this resource
	Confidence float64 `json:"confidence"`

	// IsSymbolic indicates unknown cardinality
	IsSymbolic bool `json:"is_symbolic,omitempty"`

	// Components are cost sub-components
	Components []CostComponent `json:"components,omitempty"`

	// Lineage shows dependency path
	Lineage *CostLineage `json:"lineage,omitempty"`
}

// CostComponent is a sub-component of resource cost
type CostComponent struct {
	// Name of the component
	Name string `json:"name"`

	// MonthlyCost in decimal string
	MonthlyCost string `json:"monthly_cost"`

	// Unit of measurement
	Unit string `json:"unit,omitempty"`

	// Quantity used
	Quantity string `json:"quantity,omitempty"`

	// UnitPrice
	UnitPrice string `json:"unit_price,omitempty"`
}

// CostLineage explains why a cost exists
type CostLineage struct {
	// DependencyPath from root to this resource
	DependencyPath []string `json:"dependency_path"`

	// Explanation is human-readable
	Explanation string `json:"explanation"`
}

// SymbolicCost represents a cost that cannot be computed
type SymbolicCost struct {
	// Address of the resource
	Address string `json:"address"`

	// Reason why cost is unknown
	Reason string `json:"reason"`

	// Expression that caused unknowability
	Expression string `json:"expression,omitempty"`

	// LowerBound if estimable
	LowerBound string `json:"lower_bound,omitempty"`

	// UpperBound if estimable
	UpperBound string `json:"upper_bound,omitempty"`

	// IsUnbounded if no upper limit
	IsUnbounded bool `json:"is_unbounded,omitempty"`
}

// Warning is a non-fatal warning
type Warning struct {
	// Code is machine-readable
	Code string `json:"code"`

	// Message is human-readable
	Message string `json:"message"`

	// ResourceAddress if applicable
	ResourceAddress string `json:"resource_address,omitempty"`
}

// PolicyResult is the result of policy evaluation
type PolicyResult struct {
	// PolicyName
	PolicyName string `json:"policy_name"`

	// Passed indicates if policy passed
	Passed bool `json:"passed"`

	// Severity: "info", "warning", "error", "block"
	Severity string `json:"severity"`

	// Message explains the result
	Message string `json:"message"`
}
