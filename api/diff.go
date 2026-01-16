// Package api - Additional API types for diff and metadata
package api

import "fmt"

// ResponseMetadata contains audit/reproducibility metadata
type ResponseMetadata struct {
	InputHash       string `json:"input_hash"`
	EngineVersion   string `json:"engine_version"`
	PricingSnapshot string `json:"pricing_snapshot"`
	Mode            string `json:"mode"`
	DurationMs      int64  `json:"duration_ms"`
}

// DiffRequest is the request for POST /diff
type DiffRequest struct {
	Base DiffRef        `json:"base"`
	Head DiffRef        `json:"head"`
	Mode EstimationMode `json:"mode"`
}

// DiffRef identifies a ref for comparison
type DiffRef struct {
	Ref  string `json:"ref"`
	Path string `json:"path,omitempty"`
}

// DiffResponse is the response for POST /diff
type DiffResponse struct {
	Base          DiffSummary    `json:"base"`
	Head          DiffSummary    `json:"head"`
	Delta         DiffDelta      `json:"delta"`
	Changes       []DiffChange   `json:"changes"`
	Explanations  []string       `json:"explanations,omitempty"`
	PolicyResults []PolicyResult `json:"policy_results,omitempty"`
	DurationMs    int64          `json:"duration_ms"`
}

// DiffSummary summarizes one side of a diff
type DiffSummary struct {
	Ref        string     `json:"ref"`
	TotalCost  *CostValue `json:"total_cost"`
	Confidence float64    `json:"confidence"`
}

// DiffDelta represents the change between base and head
type DiffDelta struct {
	MonthlyCost     string  `json:"monthly_cost"`
	ConfidenceDelta float64 `json:"confidence_delta"`
}

// DiffChange represents a single change
type DiffChange struct {
	Type           string   `json:"type"` // "added", "removed", "changed"
	ResourceAddr   string   `json:"resource_address"`
	CostBefore     string   `json:"cost_before,omitempty"`
	CostAfter      string   `json:"cost_after,omitempty"`
	CostDelta      string   `json:"cost_delta"`
	DependencyPath []string `json:"dependency_path,omitempty"`
	Explanation    string   `json:"explanation,omitempty"`
}

// PRCommentData contains data for rendering PR comments
type PRCommentData struct {
	DeltaCost     string       `json:"delta_cost"`
	Confidence    float64      `json:"confidence"`
	Changes       []DiffChange `json:"changes"`
	Unknowns      []string     `json:"unknowns"`
	PolicyBlocked bool         `json:"policy_blocked"`
}

// RenderMarkdown renders the PR comment as markdown
func (p *PRCommentData) RenderMarkdown() string {
	md := "### Terraform Cost Impact\n\n"
	md += p.DeltaCost + " / month\n"
	md += fmt.Sprintf("Confidence: %.0f%%\n\n", p.Confidence*100)

	if len(p.Changes) > 0 {
		md += "#### Why?\n"
		for _, c := range p.Changes {
			md += fmt.Sprintf("- %s %s\n", c.Type, c.ResourceAddr)
		}
		md += "\n"
	}

	if len(p.Unknowns) > 0 {
		md += "⚠️ Unknown cost components present\n"
	}

	if p.PolicyBlocked {
		md += "\n❌ **Blocked by policy**\n"
	}

	return md
}
