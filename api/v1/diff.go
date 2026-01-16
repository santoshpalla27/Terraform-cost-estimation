// Package v1 - Diff types for POST /api/v1/diff
package v1

// DiffRequest is the stable request contract for POST /api/v1/diff
type DiffRequest struct {
	// Base ref for comparison
	Base RefSpec `json:"base"`

	// Head ref for comparison
	Head RefSpec `json:"head"`

	// Mode for estimation
	Mode Mode `json:"mode"`

	// Options
	Options DiffOptions `json:"options,omitempty"`
}

// RefSpec identifies a git ref or source
type RefSpec struct {
	// Ref is the git ref (branch, tag, commit)
	Ref string `json:"ref"`

	// Path within the ref
	Path string `json:"path,omitempty"`
}

// DiffOptions are optional diff parameters
type DiffOptions struct {
	// IncludeUnchanged includes unchanged resources
	IncludeUnchanged bool `json:"include_unchanged,omitempty"`

	// IncludeExplanations includes dependency explanations
	IncludeExplanations bool `json:"include_explanations,omitempty"`
}

// DiffResponse is the stable response contract for POST /api/v1/diff
type DiffResponse struct {
	// Metadata for reproducibility
	Metadata ResponseMetadata `json:"metadata"`

	// Base summary
	Base DiffSummary `json:"base"`

	// Head summary
	Head DiffSummary `json:"head"`

	// Delta between base and head
	Delta DiffDelta `json:"delta"`

	// Changes are individual resource changes
	Changes []DiffChange `json:"changes"`

	// Explanations explain why costs changed
	Explanations []DiffExplanation `json:"explanations,omitempty"`

	// PolicyResults are policy evaluation results
	PolicyResults []PolicyResult `json:"policy_results,omitempty"`
}

// DiffSummary summarizes one side of the diff
type DiffSummary struct {
	// Ref that was estimated
	Ref string `json:"ref"`

	// TotalMonthlyCost
	TotalMonthlyCost string `json:"total_monthly_cost"`

	// Confidence
	Confidence float64 `json:"confidence"`

	// ResourceCount
	ResourceCount int `json:"resource_count"`

	// SymbolicCount
	SymbolicCount int `json:"symbolic_count"`
}

// DiffDelta represents the change between base and head
type DiffDelta struct {
	// MonthlyCostDelta as signed decimal string (e.g., "+123.45", "-67.89")
	MonthlyCostDelta string `json:"monthly_cost_delta"`

	// PercentChange if calculable
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

// DiffChange represents a single resource change
type DiffChange struct {
	// Type: "added", "removed", "changed"
	Type string `json:"type"`

	// ResourceAddress
	ResourceAddress string `json:"resource_address"`

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

// DiffExplanation explains why cost changed
type DiffExplanation struct {
	// ResourceAddress
	ResourceAddress string `json:"resource_address"`

	// Explanation is human-readable
	Explanation string `json:"explanation"`

	// CausalChain shows dependency causation
	CausalChain []string `json:"causal_chain,omitempty"`
}

// PRComment is the data for rendering a PR comment
type PRComment struct {
	// DeltaCost formatted with sign
	DeltaCost string `json:"delta_cost"`

	// Confidence percentage
	Confidence float64 `json:"confidence"`

	// ChangeSummary
	ChangeSummary string `json:"change_summary"`

	// Unknowns list
	Unknowns []string `json:"unknowns,omitempty"`

	// PolicyBlocked
	PolicyBlocked bool `json:"policy_blocked"`

	// PolicyMessages
	PolicyMessages []string `json:"policy_messages,omitempty"`
}

// RenderMarkdown renders the PR comment
func (p *PRComment) RenderMarkdown() string {
	md := "### Terraform Cost Impact\n\n"
	md += p.DeltaCost + " / month\n"
	md += "Confidence: " + formatPercent(p.Confidence) + "\n\n"

	if p.ChangeSummary != "" {
		md += "#### Summary\n"
		md += p.ChangeSummary + "\n\n"
	}

	if len(p.Unknowns) > 0 {
		md += "⚠️ **Unknown cost components:**\n"
		for _, u := range p.Unknowns {
			md += "- " + u + "\n"
		}
		md += "\n"
	}

	if p.PolicyBlocked {
		md += "❌ **Blocked by policy**\n"
		for _, m := range p.PolicyMessages {
			md += "- " + m + "\n"
		}
	}

	return md
}

func formatPercent(f float64) string {
	return string(rune(int(f*100))) + "%"
}
