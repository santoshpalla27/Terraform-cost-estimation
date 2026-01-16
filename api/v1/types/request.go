// Package types - Request DTOs
package types

// EstimateRequest is the public request for POST /api/v1/estimate
type EstimateRequest struct {
	// Source configuration
	Source SourceDTO `json:"source"`

	// Mode for estimation: "strict" or "permissive"
	Mode string `json:"mode"`

	// UsageProfile: "prod", "staging", "dev" (optional)
	UsageProfile string `json:"usage_profile,omitempty"`

	// Options
	Options EstimateOptionsDTO `json:"options,omitempty"`
}

// SourceDTO defines the Terraform source
type SourceDTO struct {
	// Type: "git", "upload", "local"
	Type string `json:"type"`

	// Repo is the git repository URL (for git sources)
	Repo string `json:"repo,omitempty"`

	// Ref is the git ref: branch, tag, or commit (for git sources)
	Ref string `json:"ref,omitempty"`

	// Path within source
	Path string `json:"path,omitempty"`

	// UploadID for upload sources
	UploadID string `json:"upload_id,omitempty"`
}

// EstimateOptionsDTO are optional request parameters
type EstimateOptionsDTO struct {
	// IncludeDependencyGraph includes the full graph in response
	IncludeDependencyGraph bool `json:"include_dependency_graph,omitempty"`

	// IncludeCostLineage includes lineage for each cost
	IncludeCostLineage bool `json:"include_cost_lineage,omitempty"`

	// IncludeComponents includes cost sub-components
	IncludeComponents bool `json:"include_components,omitempty"`
}
