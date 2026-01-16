// Package api - API types for cost estimation
// These types define the contract for /estimate endpoint.
// API is stateless, idempotent, and deterministic.
package api

import (
	"time"

	"terraform-cost/core/graph"
)

// EstimateRequest is the input to POST /estimate
type EstimateRequest struct {
	// Source configuration
	Source SourceConfig `json:"source"`

	// Estimation mode
	Mode EstimationMode `json:"mode"`

	// Usage overrides (optional)
	UsageOverrides map[string]UsageOverride `json:"usage_overrides,omitempty"`

	// Pricing snapshot ID (optional, uses latest if empty)
	PricingSnapshotID string `json:"pricing_snapshot_id,omitempty"`

	// Policy configuration (optional)
	PolicyConfig *PolicyConfig `json:"policy_config,omitempty"`
}

// SourceConfig defines the Terraform source
type SourceConfig struct {
	// Type of source: "directory", "git", "inline"
	Type string `json:"type"`

	// Path or URL
	Path string `json:"path,omitempty"`
	URL  string `json:"url,omitempty"`
	Ref  string `json:"ref,omitempty"` // git ref

	// Inline HCL (for type="inline")
	InlineHCL map[string]string `json:"inline_hcl,omitempty"`

	// Variables
	Variables map[string]interface{} `json:"variables,omitempty"`

	// Workspace
	Workspace string `json:"workspace,omitempty"`
}

// EstimationMode controls strictness
type EstimationMode string

const (
	ModeStrict     EstimationMode = "strict"
	ModePermissive EstimationMode = "permissive"
)

// UsageOverride overrides default usage assumptions
type UsageOverride struct {
	ResourceAddress string                 `json:"resource_address"`
	Values          map[string]interface{} `json:"values"`
}

// PolicyConfig configures policy evaluation
type PolicyConfig struct {
	// Policies to evaluate
	Policies []PolicyRule `json:"policies"`

	// Baseline for diff (optional)
	BaselineID string `json:"baseline_id,omitempty"`
}

// PolicyRule defines a policy
type PolicyRule struct {
	Name      string                 `json:"name"`
	Type      string                 `json:"type"` // "budget", "change", "unknown"
	Threshold interface{}            `json:"threshold,omitempty"`
	Scope     string                 `json:"scope,omitempty"` // "all", "new", "changed"
	Config    map[string]interface{} `json:"config,omitempty"`
}

// EstimateResponse is the output of POST /estimate
type EstimateResponse struct {
	// Request tracking
	RequestID string    `json:"request_id"`
	Timestamp time.Time `json:"timestamp"`

	// Status
	Status  string `json:"status"` // "success", "partial", "error"
	Message string `json:"message,omitempty"`

	// Cost summary
	TotalMonthlyCost  *CostValue `json:"total_monthly_cost,omitempty"`
	TotalHourlyCost   *CostValue `json:"total_hourly_cost,omitempty"`

	// Confidence (pessimistic)
	Confidence       float64  `json:"confidence"`
	ConfidenceLevel  string   `json:"confidence_level"` // "high", "medium", "low", "unknown"
	ConfidenceReason string   `json:"confidence_reason,omitempty"`

	// Cost breakdown
	Resources []ResourceCost `json:"resources,omitempty"`

	// Unknowns (symbolic costs)
	Unknowns []UnknownCost `json:"unknowns,omitempty"`

	// Assumptions made
	Assumptions []Assumption `json:"assumptions,omitempty"`

	// Policy results
	PolicyResults []PolicyResult `json:"policy_results,omitempty"`

	// Pricing reference
	PricingSnapshot *PricingReference `json:"pricing_snapshot,omitempty"`

	// Errors (for partial results)
	Errors []ErrorDetail `json:"errors,omitempty"`
}

// CostValue represents a cost with currency
type CostValue struct {
	Amount   string `json:"amount"` // Decimal string for precision
	Currency string `json:"currency"`
}

// ResourceCost is the cost of a single resource
type ResourceCost struct {
	Address        string                      `json:"address"`
	ResourceType   string                      `json:"resource_type"`
	MonthlyCost    *CostValue                  `json:"monthly_cost"`
	HourlyCost     *CostValue                  `json:"hourly_cost"`
	Confidence     float64                     `json:"confidence"`
	DependsOn      []string                    `json:"depends_on,omitempty"`
	Components     []CostComponent             `json:"components,omitempty"`
	DependencyPath []graph.DependencyNodeID    `json:"dependency_path,omitempty"`
}

// CostComponent is a component of resource cost
type CostComponent struct {
	Name        string     `json:"name"`
	MonthlyCost *CostValue `json:"monthly_cost"`
	Unit        string     `json:"unit,omitempty"`
	Quantity    float64    `json:"quantity,omitempty"`
}

// UnknownCost represents a symbolic cost bucket
type UnknownCost struct {
	Address     string     `json:"address"`
	Reason      string     `json:"reason"`
	Expression  string     `json:"expression,omitempty"`
	MinCost     *CostValue `json:"min_cost,omitempty"`
	MaxCost     *CostValue `json:"max_cost,omitempty"`
	IsUnbounded bool       `json:"is_unbounded"`
}

// Assumption is a usage assumption made
type Assumption struct {
	ResourceAddress string  `json:"resource_address"`
	Attribute       string  `json:"attribute"`
	Value           string  `json:"value"`
	Unit            string  `json:"unit,omitempty"`
	Reason          string  `json:"reason"`
	Impact          float64 `json:"impact"` // Confidence impact
	Overrideable    bool    `json:"overrideable"`
	OverrideKey     string  `json:"override_key,omitempty"`
}

// PolicyResult is the result of a policy evaluation
type PolicyResult struct {
	PolicyName string `json:"policy_name"`
	Passed     bool   `json:"passed"`
	Message    string `json:"message,omitempty"`
	Severity   string `json:"severity,omitempty"` // "error", "warning", "info"
}

// PricingReference identifies the pricing snapshot used
type PricingReference struct {
	ID          string    `json:"id"`
	Version     string    `json:"version"`
	EffectiveAt time.Time `json:"effective_at"`
	ContentHash string    `json:"content_hash"`
}

// ErrorDetail provides error information
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}
