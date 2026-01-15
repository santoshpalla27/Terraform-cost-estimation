// Package output provides output formatting interfaces.
// This package produces human and machine-readable outputs.
package output

import (
	"io"

	"terraform-cost/core/policy"
	"terraform-cost/core/types"
)

// Format represents output format type
type Format string

const (
	// FormatCLI is a human-readable CLI table
	FormatCLI Format = "cli"

	// FormatJSON is machine-readable JSON
	FormatJSON Format = "json"

	// FormatHTML is an HTML report
	FormatHTML Format = "html"

	// FormatMarkdown is a markdown report
	FormatMarkdown Format = "markdown"

	// FormatPR is a PR comment format
	FormatPR Format = "pr"
)

// Formatter produces output in a specific format
type Formatter interface {
	// Format returns the format type
	Format() Format

	// Render produces output for the given result
	Render(w io.Writer, result *EstimationResult) error
}

// EstimationResult contains the complete estimation output
type EstimationResult struct {
	// CostGraph is the calculated cost graph
	CostGraph *types.CostGraph `json:"cost_graph"`

	// AssetGraph is the source asset graph
	AssetGraph *types.AssetGraph `json:"asset_graph,omitempty"`

	// PolicyResult contains policy evaluation results
	PolicyResult *policy.EvaluationResult `json:"policy_result,omitempty"`

	// PricingSnapshot identifies the pricing data used
	PricingSnapshot *types.PricingSnapshot `json:"pricing_snapshot"`

	// UsageProfile is the usage profile that was applied
	UsageProfile *types.UsageProfile `json:"usage_profile,omitempty"`

	// Assumptions documents estimation assumptions
	Assumptions []Assumption `json:"assumptions,omitempty"`

	// Confidence is the overall confidence level (0.0 to 1.0)
	Confidence float64 `json:"confidence"`

	// Metadata contains execution context
	Metadata EstimationMetadata `json:"metadata"`

	// Diff contains comparison with previous estimate
	Diff *EstimationDiff `json:"diff,omitempty"`
}

// Assumption documents an estimation assumption
type Assumption struct {
	// Resource is the resource this assumption applies to
	Resource types.ResourceAddress `json:"resource,omitempty"`

	// Category is the assumption category
	Category string `json:"category"`

	// Description explains the assumption
	Description string `json:"description"`

	// Impact describes the potential impact
	Impact string `json:"impact,omitempty"`

	// Confidence is the confidence in this assumption
	Confidence float64 `json:"confidence,omitempty"`
}

// EstimationMetadata contains execution context
type EstimationMetadata struct {
	// Timestamp is when the estimation was performed
	Timestamp string `json:"timestamp"`

	// Duration is how long the estimation took
	Duration string `json:"duration"`

	// InputHash is a hash of the input for caching
	InputHash string `json:"input_hash"`

	// SnapshotID is the pricing snapshot ID
	SnapshotID string `json:"snapshot_id"`

	// Version is the tool version
	Version string `json:"version"`

	// Source is the input source
	Source types.InputSource `json:"source"`

	// Environment is the target environment
	Environment string `json:"environment,omitempty"`
}

// EstimationDiff contains comparison with a previous estimate
type EstimationDiff struct {
	// Previous is the previous cost
	Previous types.CostGraph `json:"previous"`

	// Added are new resources
	Added []DiffItem `json:"added,omitempty"`

	// Removed are removed resources
	Removed []DiffItem `json:"removed,omitempty"`

	// Changed are resources with cost changes
	Changed []DiffItem `json:"changed,omitempty"`

	// TotalChange is the difference in total cost
	TotalChange string `json:"total_change"`

	// PercentChange is the percentage change
	PercentChange float64 `json:"percent_change"`
}

// DiffItem represents a single diff entry
type DiffItem struct {
	// Resource is the resource address
	Resource string `json:"resource"`

	// PreviousCost is the old cost
	PreviousCost string `json:"previous_cost,omitempty"`

	// CurrentCost is the new cost
	CurrentCost string `json:"current_cost,omitempty"`

	// Change is the cost difference
	Change string `json:"change,omitempty"`
}

// FormatterRegistry manages formatter registration
type FormatterRegistry interface {
	// Register adds a formatter to the registry
	Register(formatter Formatter) error

	// GetFormatter returns a formatter for a format type
	GetFormatter(format Format) (Formatter, bool)

	// GetAll returns all registered formatters
	GetAll() []Formatter
}
