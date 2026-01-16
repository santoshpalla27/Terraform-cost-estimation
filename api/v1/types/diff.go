// Package types - Diff DTOs for POST /api/v1/diff
package types

// DiffRequest is the public request for POST /api/v1/diff
type DiffRequest struct {
	// Base ref for comparison
	Base RefSpecDTO `json:"base"`

	// Head ref for comparison
	Head RefSpecDTO `json:"head"`

	// Mode for estimation
	Mode string `json:"mode"`

	// Options
	Options DiffOptionsDTO `json:"options,omitempty"`
}

// RefSpecDTO identifies a git ref or source
type RefSpecDTO struct {
	// Ref is the git ref (branch, tag, commit)
	Ref string `json:"ref"`

	// Path within the ref (optional)
	Path string `json:"path,omitempty"`
}

// DiffOptionsDTO are optional diff parameters
type DiffOptionsDTO struct {
	// IncludeUnchanged includes unchanged resources
	IncludeUnchanged bool `json:"include_unchanged,omitempty"`

	// IncludeExplanations includes dependency explanations
	IncludeExplanations bool `json:"include_explanations,omitempty"`
}

// DiffResponse is the public response for POST /api/v1/diff
type DiffResponse struct {
	// Metadata for reproducibility
	Metadata MetadataDTO `json:"metadata"`

	// Base summary
	Base DiffSideDTO `json:"base"`

	// Head summary
	Head DiffSideDTO `json:"head"`

	// Delta between base and head
	Delta DeltaDTO `json:"delta"`

	// Changes are individual resource changes
	Changes []ChangeDTO `json:"changes"`

	// Explanations explain why costs changed
	Explanations []ExplanationDTO `json:"explanations,omitempty"`

	// Policies are policy evaluation results
	Policies []PolicyResultDTO `json:"policy_results,omitempty"`
}

// DiffSideDTO summarizes one side of the diff
type DiffSideDTO struct {
	// Ref that was estimated
	Ref string `json:"ref"`

	// TotalMonthlyCost as decimal string
	TotalMonthlyCost string `json:"total_monthly_cost"`

	// Confidence
	Confidence float64 `json:"confidence"`

	// ResourceCount
	ResourceCount int `json:"resource_count"`

	// SymbolicCount
	SymbolicCount int `json:"symbolic_count"`
}

// DeltaDTO represents the change between base and head
type DeltaDTO struct {
	// MonthlyCostDelta as signed decimal string (e.g., "+123.45", "-67.89")
	MonthlyCostDelta string `json:"monthly_cost_delta"`

	// PercentChange if calculable (e.g., "+15.2%")
	PercentChange string `json:"percent_change,omitempty"`

	// ConfidenceDelta
	ConfidenceDelta float64 `json:"confidence_delta"`

	// AddedCount
	AddedCount int `json:"added_count"`

	// RemovedCount
	RemovedCount int `json:"removed_count"`

	// ChangedCount
	ChangedCount int `json:"changed_count"`
}

// ChangeDTO represents a single resource change
type ChangeDTO struct {
	// Type: "added", "removed", "changed"
	Type string `json:"type"`

	// Address of the resource
	Address string `json:"address"`

	// ResourceType
	ResourceType string `json:"resource_type"`

	// CostBefore (empty for added)
	CostBefore string `json:"cost_before,omitempty"`

	// CostAfter (empty for removed)
	CostAfter string `json:"cost_after,omitempty"`

	// CostDelta as signed string
	CostDelta string `json:"cost_delta"`

	// ConfidenceBefore
	ConfidenceBefore float64 `json:"confidence_before,omitempty"`

	// ConfidenceAfter
	ConfidenceAfter float64 `json:"confidence_after,omitempty"`

	// IsSymbolic
	IsSymbolic bool `json:"is_symbolic,omitempty"`

	// DependencyPath shows lineage
	DependencyPath []string `json:"dependency_path,omitempty"`
}

// ExplanationDTO explains why cost changed
type ExplanationDTO struct {
	// Address of the resource
	Address string `json:"address"`

	// Explanation is human-readable
	Explanation string `json:"explanation"`

	// CausalChain shows dependency causation
	CausalChain []string `json:"causal_chain,omitempty"`
}
