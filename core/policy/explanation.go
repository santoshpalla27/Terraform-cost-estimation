// Package policy - Explainable policy results
// Policies explain WHY they failed, not just THAT they failed.
package policy

import (
	"fmt"
	"sort"
	"strings"

	"terraform-cost/core/determinism"
	"terraform-cost/core/model"
)

// ExplainedResult is a policy result with full explanation
type ExplainedResult struct {
	PolicyName string
	Passed     bool

	// Short summary
	Summary string

	// Detailed explanation
	Explanation *PolicyExplanation

	// Recommendations
	Recommendations []Recommendation
}

// PolicyExplanation provides deep explanation of policy outcome
type PolicyExplanation struct {
	// What the policy checks
	PolicyDescription string

	// What threshold/limit was applied
	Threshold *ThresholdInfo

	// What was analyzed
	AnalyzedScope AnalysisScope

	// What caused the violation (if failed)
	Violations []ViolationDetail

	// What contributed most to the outcome
	TopContributors []Contributor

	// Confidence considerations
	ConfidenceImpact *ConfidenceImpactInfo
}

// ThresholdInfo describes policy thresholds
type ThresholdInfo struct {
	Name      string // e.g., "monthly_budget"
	Value     string // e.g., "$1000.00"
	Actual    string // e.g., "$1234.56"
	Exceeded  bool
	ExcessBy  string // e.g., "$234.56 (23.5%)"
}

// AnalysisScope describes what was analyzed
type AnalysisScope struct {
	TotalInstances   int
	TotalComponents  int
	AnalyzedTypes    []string
	ExcludedTypes    []string
	TimeRange        string // if applicable
}

// ViolationDetail explains a specific violation
type ViolationDetail struct {
	// What violated
	InstanceAddress model.CanonicalAddress
	Component       string

	// Why it violated
	Reason string

	// How much impact
	CostImpact determinism.Money

	// What would fix it
	SuggestedFix string

	// Related lineage
	FormulaUsed   string
	RateUsed      string
	UsageAssumed  string
}

// Contributor identifies what contributed to outcome
type Contributor struct {
	Category string // "instance", "component", "rate", "usage"
	Name     string
	Impact   determinism.Money
	Percent  float64
	Reason   string
}

// ConfidenceImpactInfo describes how confidence affected the policy
type ConfidenceImpactInfo struct {
	OverallConfidence float64
	LowConfidenceItems int
	AffectedByUnknowns bool
	UnknownCount       int
	Caveat             string // e.g., "Result may change when unknowns resolve"
}

// Recommendation is a suggested action
type Recommendation struct {
	Priority    int    // 1=critical, 2=high, 3=medium
	Action      string
	Rationale   string
	EstimatedSavings *determinism.Money
}

// ExplainablePolicy is a policy that provides explanations
type ExplainablePolicy interface {
	Name() string
	Description() string
	EvaluateWithExplanation(ctx *FullLineageContext) (*ExplainedResult, error)
}

// ExplainedBudgetPolicy is a budget policy with explanations
type ExplainedBudgetPolicy struct {
	name          string
	description   string
	monthlyBudget determinism.Money
	warnThreshold float64
}

// NewExplainedBudgetPolicy creates an explainable budget policy
func NewExplainedBudgetPolicy(name string, budget determinism.Money, warnAt float64) *ExplainedBudgetPolicy {
	return &ExplainedBudgetPolicy{
		name:          name,
		description:   fmt.Sprintf("Ensures monthly costs stay within $%s budget", budget.String()),
		monthlyBudget: budget,
		warnThreshold: warnAt,
	}
}

func (p *ExplainedBudgetPolicy) Name() string        { return p.name }
func (p *ExplainedBudgetPolicy) Description() string { return p.description }

func (p *ExplainedBudgetPolicy) EvaluateWithExplanation(ctx *FullLineageContext) (*ExplainedResult, error) {
	result := &ExplainedResult{
		PolicyName: p.name,
		Passed:     true,
		Explanation: &PolicyExplanation{
			PolicyDescription: p.description,
			AnalyzedScope: AnalysisScope{
				TotalInstances: len(ctx.Instances),
			},
		},
	}

	// Count components
	for _, inst := range ctx.Instances {
		result.Explanation.AnalyzedScope.TotalComponents += len(inst.Components)
	}

	// Set threshold info
	result.Explanation.Threshold = &ThresholdInfo{
		Name:   "monthly_budget",
		Value:  "$" + p.monthlyBudget.String(),
		Actual: "$" + ctx.TotalMonthlyCost.String(),
	}

	// Check budget
	if ctx.TotalMonthlyCost.Cmp(p.monthlyBudget) > 0 {
		result.Passed = false
		excess := ctx.TotalMonthlyCost.Sub(p.monthlyBudget)
		percent := (ctx.TotalMonthlyCost.Float64() / p.monthlyBudget.Float64() - 1) * 100

		result.Summary = fmt.Sprintf("Monthly cost $%s exceeds budget $%s by $%s (%.1f%%)",
			ctx.TotalMonthlyCost.String(), p.monthlyBudget.String(), excess.String(), percent)

		result.Explanation.Threshold.Exceeded = true
		result.Explanation.Threshold.ExcessBy = fmt.Sprintf("$%s (%.1f%%)", excess.String(), percent)

		// Find top contributors
		result.Explanation.TopContributors = p.findTopContributors(ctx, 5)
		result.Explanation.Violations = p.buildViolations(ctx, excess)

		// Add recommendations
		result.Recommendations = p.generateRecommendations(ctx, excess)
	} else {
		percent := ctx.TotalMonthlyCost.Float64() / p.monthlyBudget.Float64() * 100
		result.Summary = fmt.Sprintf("Monthly cost $%s is within budget $%s (%.1f%% utilized)",
			ctx.TotalMonthlyCost.String(), p.monthlyBudget.String(), percent)

		// Warn if close to threshold
		if percent >= p.warnThreshold*100 {
			result.Recommendations = append(result.Recommendations, Recommendation{
				Priority:  3,
				Action:    "Monitor cost growth",
				Rationale: fmt.Sprintf("Currently at %.1f%% of budget", percent),
			})
		}
	}

	// Add confidence impact
	result.Explanation.ConfidenceImpact = &ConfidenceImpactInfo{
		OverallConfidence: ctx.OverallConfidence,
		LowConfidenceItems: len(ctx.LowConfidenceItems),
		UnknownCount:       len(ctx.Unknowns),
	}

	if ctx.OverallConfidence < 0.9 {
		result.Explanation.ConfidenceImpact.Caveat = 
			"Cost estimate has reduced confidence; actual costs may vary"
	}
	if len(ctx.Unknowns) > 0 {
		result.Explanation.ConfidenceImpact.AffectedByUnknowns = true
	}

	return result, nil
}

func (p *ExplainedBudgetPolicy) findTopContributors(ctx *FullLineageContext, n int) []Contributor {
	type item struct {
		addr model.CanonicalAddress
		cost determinism.Money
	}

	var items []item
	for _, inst := range ctx.Instances {
		items = append(items, item{
			addr: model.CanonicalAddress(inst.Address),
			cost: inst.MonthlyCost,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].cost.Cmp(items[j].cost) > 0
	})

	if n > len(items) {
		n = len(items)
	}

	contributors := make([]Contributor, n)
	for i := 0; i < n; i++ {
		percent := items[i].cost.Float64() / ctx.TotalMonthlyCost.Float64() * 100
		contributors[i] = Contributor{
			Category: "instance",
			Name:     string(items[i].addr),
			Impact:   items[i].cost,
			Percent:  percent,
			Reason:   fmt.Sprintf("%.1f%% of total cost", percent),
		}
	}

	return contributors
}

func (p *ExplainedBudgetPolicy) buildViolations(ctx *FullLineageContext, excess determinism.Money) []ViolationDetail {
	var violations []ViolationDetail

	// Find top 3 cost drivers as "violations"
	type item struct {
		inst *InstanceLineage
		cost determinism.Money
	}

	var items []item
	for _, inst := range ctx.Instances {
		items = append(items, item{inst: inst, cost: inst.MonthlyCost})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].cost.Cmp(items[j].cost) > 0
	})

	n := 3
	if n > len(items) {
		n = len(items)
	}

	for i := 0; i < n; i++ {
		inst := items[i].inst
		violations = append(violations, ViolationDetail{
			InstanceAddress: model.CanonicalAddress(inst.Address),
			Reason:          fmt.Sprintf("High cost: $%s/month", inst.MonthlyCost.String()),
			CostImpact:      inst.MonthlyCost,
			SuggestedFix:    p.suggestFix(inst),
		})
	}

	return violations
}

func (p *ExplainedBudgetPolicy) suggestFix(inst *InstanceLineage) string {
	// Generate context-aware suggestions
	resourceType := strings.Split(string(inst.Address), ".")[0]

	switch {
	case strings.Contains(resourceType, "instance"):
		return "Consider using a smaller instance type or Spot instances"
	case strings.Contains(resourceType, "db") || strings.Contains(resourceType, "rds"):
		return "Consider Reserved Instances or Aurora Serverless"
	case strings.Contains(resourceType, "nat"):
		return "Consider VPC endpoints to reduce NAT traffic"
	case strings.Contains(resourceType, "lb"):
		return "Review target groups and consider consolidation"
	default:
		return "Review configuration for cost optimization"
	}
}

func (p *ExplainedBudgetPolicy) generateRecommendations(ctx *FullLineageContext, excess determinism.Money) []Recommendation {
	var recs []Recommendation

	recs = append(recs, Recommendation{
		Priority:  1,
		Action:    "Review top cost contributors",
		Rationale: "Focus on highest-cost instances for maximum impact",
	})

	// If many low confidence items
	if len(ctx.LowConfidenceItems) > len(ctx.Instances)/4 {
		recs = append(recs, Recommendation{
			Priority:  2,
			Action:    "Provide usage overrides for low-confidence components",
			Rationale: fmt.Sprintf("%d components have uncertain cost estimates", len(ctx.LowConfidenceItems)),
		})
	}

	// If unknowns present
	if len(ctx.Unknowns) > 0 {
		recs = append(recs, Recommendation{
			Priority:  2,
			Action:    "Resolve unknown Terraform values",
			Rationale: fmt.Sprintf("%d unknown values may affect final cost", len(ctx.Unknowns)),
		})
	}

	return recs
}

// FormatExplanation returns a human-readable explanation
func FormatExplanation(result *ExplainedResult) string {
	var sb strings.Builder

	// Header
	status := "✓ PASS"
	if !result.Passed {
		status = "✗ FAIL"
	}
	sb.WriteString(fmt.Sprintf("Policy: %s [%s]\n", result.PolicyName, status))
	sb.WriteString(fmt.Sprintf("Summary: %s\n\n", result.Summary))

	if result.Explanation == nil {
		return sb.String()
	}

	// Threshold
	if t := result.Explanation.Threshold; t != nil {
		sb.WriteString("Threshold:\n")
		sb.WriteString(fmt.Sprintf("  %s: %s (actual: %s)\n", t.Name, t.Value, t.Actual))
		if t.Exceeded {
			sb.WriteString(fmt.Sprintf("  EXCEEDED by: %s\n", t.ExcessBy))
		}
		sb.WriteString("\n")
	}

	// Top contributors
	if len(result.Explanation.TopContributors) > 0 {
		sb.WriteString("Top Cost Contributors:\n")
		for i, c := range result.Explanation.TopContributors {
			sb.WriteString(fmt.Sprintf("  %d. %s: $%s (%.1f%%)\n", i+1, c.Name, c.Impact.String(), c.Percent))
		}
		sb.WriteString("\n")
	}

	// Violations
	if len(result.Explanation.Violations) > 0 {
		sb.WriteString("Violations:\n")
		for _, v := range result.Explanation.Violations {
			sb.WriteString(fmt.Sprintf("  • %s\n", v.InstanceAddress))
			sb.WriteString(fmt.Sprintf("    Reason: %s\n", v.Reason))
			sb.WriteString(fmt.Sprintf("    Suggestion: %s\n", v.SuggestedFix))
		}
		sb.WriteString("\n")
	}

	// Recommendations
	if len(result.Recommendations) > 0 {
		sb.WriteString("Recommendations:\n")
		for _, r := range result.Recommendations {
			priority := "!"
			if r.Priority == 1 {
				priority = "!!!"
			} else if r.Priority == 2 {
				priority = "!!"
			}
			sb.WriteString(fmt.Sprintf("  [%s] %s\n", priority, r.Action))
			sb.WriteString(fmt.Sprintf("       %s\n", r.Rationale))
		}
		sb.WriteString("\n")
	}

	// Confidence caveat
	if c := result.Explanation.ConfidenceImpact; c != nil && c.Caveat != "" {
		sb.WriteString(fmt.Sprintf("Note: %s\n", c.Caveat))
	}

	return sb.String()
}
