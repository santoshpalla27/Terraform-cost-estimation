// Package policy provides the policy evaluation engine with deep context access.
// Policies can access cost lineage, usage confidence, and instance identity.
package policy

import (
	"context"
	"fmt"
	"sort"

	"terraform-cost/core/determinism"
	"terraform-cost/core/model"
	"terraform-cost/core/pricing"
)

// DeepEvaluator evaluates cost policies with full context access
type DeepEvaluator struct {
	policies []DeepPolicy
}

// NewDeepEvaluator creates a new policy evaluator
func NewDeepEvaluator() *DeepEvaluator {
	return &DeepEvaluator{
		policies: []DeepPolicy{},
	}
}

// Register adds a policy
func (e *DeepEvaluator) Register(p DeepPolicy) {
	e.policies = append(e.policies, p)
}

// DeepPolicy is an interface for cost policies with deep context
type DeepPolicy interface {
	Name() string
	Evaluate(ctx context.Context, input *PolicyInput) (*PolicyOutput, error)
}

// PolicyInput provides DEEP context for policy evaluation
type PolicyInput struct {
	// Instance costs (per-instance, not aggregated)
	InstanceCosts map[model.InstanceID]*InstanceCostDetail

	// Total costs
	TotalMonthlyCost determinism.Money
	TotalHourlyCost  determinism.Money

	// Confidence information
	OverallConfidence float64
	LowConfidenceItems []LowConfidenceItem

	// Unknowns
	UnknownValues []UnknownValueInfo

	// Definitions (for grouping)
	Definitions map[model.DefinitionID]*model.AssetDefinition

	// Full lineage for tracing
	AllLineage []*pricing.CostLineage

	// Pricing snapshot used
	Snapshot *pricing.PricingSnapshot
}

// InstanceCostDetail provides deep detail for a single instance
type InstanceCostDetail struct {
	// Instance identity
	InstanceID   model.InstanceID
	Address      model.InstanceAddress
	DefinitionID model.DefinitionID
	InstanceKey  model.InstanceKey

	// Provider and region
	Provider string
	Region   string

	// Costs
	MonthlyCost determinism.Money
	HourlyCost  determinism.Money
	Components  []ComponentDetail

	// Confidence
	Confidence float64
	Factors    []ConfidenceFactor

	// Full lineage
	Lineage []*pricing.CostLineage

	// Usage information
	Usage UsageInfo

	// Tags/labels for targeting
	Tags map[string]string
}

// ComponentDetail provides detail for a cost component
type ComponentDetail struct {
	Name        string
	MonthlyCost determinism.Money
	HourlyCost  determinism.Money

	// Rate used
	Rate RateInfo

	// Usage
	UsageValue float64
	UsageUnit  string

	// Formula applied
	Formula pricing.FormulaApplication

	// Confidence
	Confidence float64
}

// RateInfo describes the pricing rate used
type RateInfo struct {
	ID       pricing.RateID
	Key      pricing.RateKey
	Price    string
	Unit     string
	Currency string
}

// UsageInfo provides usage details
type UsageInfo struct {
	Source      pricing.UsageSource
	Confidence  float64
	Assumptions []string
	Overridden  bool
}

// ConfidenceFactor explains why confidence is reduced
type ConfidenceFactor struct {
	Reason    string
	Impact    float64
	Component string
	IsUnknown bool
}

// LowConfidenceItem identifies items with low confidence
type LowConfidenceItem struct {
	InstanceID model.InstanceID
	Address    model.InstanceAddress
	Component  string
	Confidence float64
	Reason     string
}

// UnknownValueInfo describes an unknown value
type UnknownValueInfo struct {
	Address string
	Reason  string
	Impact  string
}

// PolicyOutput is the result of a single policy
type PolicyOutput struct {
	Passed  bool
	Message string

	// Affected instances (for targeting)
	AffectedInstances []model.InstanceID

	// Cost impact
	AffectedCost determinism.Money

	// Lineage references (for explainability)
	LineageRefs []*pricing.CostLineage

	// Suggested actions
	Suggestions []string
}

// DeepResult is the complete result of all policies
type DeepResult struct {
	Passed   bool
	Policies []DeepPolicyResult
}

// DeepPolicyResult is the result of a single policy
type DeepPolicyResult struct {
	Name    string
	Passed  bool
	Message string

	AffectedInstances []model.InstanceID
	AffectedCost      determinism.Money
	LineageRefs       []*pricing.CostLineage
	Suggestions       []string
}

// Evaluate runs all policies
func (e *DeepEvaluator) Evaluate(ctx context.Context, input *PolicyInput) (*DeepResult, error) {
	result := &DeepResult{
		Passed:   true,
		Policies: make([]DeepPolicyResult, 0, len(e.policies)),
	}

	for _, p := range e.policies {
		output, err := p.Evaluate(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("policy %s failed: %w", p.Name(), err)
		}

		pr := DeepPolicyResult{
			Name:              p.Name(),
			Passed:            output.Passed,
			Message:           output.Message,
			AffectedInstances: output.AffectedInstances,
			AffectedCost:      output.AffectedCost,
			LineageRefs:       output.LineageRefs,
			Suggestions:       output.Suggestions,
		}
		result.Policies = append(result.Policies, pr)

		if !output.Passed {
			result.Passed = false
		}
	}

	return result, nil
}

// BudgetPolicy checks if total cost exceeds a budget
type BudgetPolicy struct {
	name          string
	monthlyBudget determinism.Money
	threshold     float64 // 0.0-1.0, warn when at this percentage
}

// NewBudgetPolicy creates a budget policy
func NewBudgetPolicy(name string, monthlyBudget determinism.Money, threshold float64) *BudgetPolicy {
	return &BudgetPolicy{
		name:          name,
		monthlyBudget: monthlyBudget,
		threshold:     threshold,
	}
}

func (p *BudgetPolicy) Name() string { return p.name }

func (p *BudgetPolicy) Evaluate(ctx context.Context, input *PolicyInput) (*PolicyOutput, error) {
	output := &PolicyOutput{
		Passed: true,
	}

	// Check if over budget
	if input.TotalMonthlyCost.Cmp(p.monthlyBudget) > 0 {
		output.Passed = false
		output.Message = fmt.Sprintf("Monthly cost $%s exceeds budget $%s",
			input.TotalMonthlyCost.String(), p.monthlyBudget.String())

		// Find top cost contributors
		output.AffectedInstances = p.findTopContributors(input, 5)
		output.AffectedCost = input.TotalMonthlyCost.Sub(p.monthlyBudget)
		output.Suggestions = []string{
			"Consider using smaller instance types",
			"Review usage assumptions for accuracy",
			"Check for unused resources",
		}
	} else {
		// Check threshold warning
		thresholdAmount := p.monthlyBudget.MulFloat(p.threshold)
		if input.TotalMonthlyCost.Cmp(thresholdAmount) > 0 {
			output.Message = fmt.Sprintf("Monthly cost $%s is at %.0f%% of budget $%s",
				input.TotalMonthlyCost.String(),
				(input.TotalMonthlyCost.Float64()/p.monthlyBudget.Float64())*100,
				p.monthlyBudget.String())
		} else {
			output.Message = fmt.Sprintf("Monthly cost $%s is within budget $%s",
				input.TotalMonthlyCost.String(), p.monthlyBudget.String())
		}
	}

	return output, nil
}

func (p *BudgetPolicy) findTopContributors(input *PolicyInput, n int) []model.InstanceID {
	// Sort instances by cost
	type costItem struct {
		id   model.InstanceID
		cost determinism.Money
	}

	items := make([]costItem, 0, len(input.InstanceCosts))
	for id, detail := range input.InstanceCosts {
		items = append(items, costItem{id: id, cost: detail.MonthlyCost})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].cost.Cmp(items[j].cost) > 0
	})

	result := make([]model.InstanceID, 0, n)
	for i := 0; i < n && i < len(items); i++ {
		result = append(result, items[i].id)
	}
	return result
}

// ConfidencePolicy checks if estimation confidence is acceptable
type ConfidencePolicy struct {
	name          string
	minConfidence float64
}

// NewConfidencePolicy creates a confidence policy
func NewConfidencePolicy(name string, minConfidence float64) *ConfidencePolicy {
	return &ConfidencePolicy{
		name:          name,
		minConfidence: minConfidence,
	}
}

func (p *ConfidencePolicy) Name() string { return p.name }

func (p *ConfidencePolicy) Evaluate(ctx context.Context, input *PolicyInput) (*PolicyOutput, error) {
	output := &PolicyOutput{
		Passed: true,
	}

	if input.OverallConfidence < p.minConfidence {
		output.Passed = false
		output.Message = fmt.Sprintf("Estimation confidence %.2f%% is below minimum %.2f%%",
			input.OverallConfidence*100, p.minConfidence*100)

		// Find low confidence items
		for _, item := range input.LowConfidenceItems {
			output.AffectedInstances = append(output.AffectedInstances, item.InstanceID)
		}

		// Collect lineage for affected items
		for _, lineage := range input.AllLineage {
			if lineage.Confidence < p.minConfidence {
				output.LineageRefs = append(output.LineageRefs, lineage)
			}
		}

		output.Suggestions = []string{
			"Provide usage overrides for low-confidence components",
			"Check that all required variables are provided",
			"Review unknown values in the configuration",
		}
	} else {
		output.Message = fmt.Sprintf("Estimation confidence %.2f%% meets minimum %.2f%%",
			input.OverallConfidence*100, p.minConfidence*100)
	}

	return output, nil
}

// ResourceTypePolicy checks limits on specific resource types
type ResourceTypePolicy struct {
	name         string
	resourceType string
	maxInstances int
	maxCost      *determinism.Money
}

// NewResourceTypePolicy creates a resource type policy
func NewResourceTypePolicy(name, resourceType string, maxInstances int, maxCost *determinism.Money) *ResourceTypePolicy {
	return &ResourceTypePolicy{
		name:         name,
		resourceType: resourceType,
		maxInstances: maxInstances,
		maxCost:      maxCost,
	}
}

func (p *ResourceTypePolicy) Name() string { return p.name }

func (p *ResourceTypePolicy) Evaluate(ctx context.Context, input *PolicyInput) (*PolicyOutput, error) {
	output := &PolicyOutput{
		Passed: true,
	}

	// Count instances and sum cost for this resource type
	var count int
	totalCost := determinism.Zero("USD")
	affected := []model.InstanceID{}

	for id, detail := range input.InstanceCosts {
		// Check if this instance matches the resource type
		addr := string(detail.Address)
		if len(addr) >= len(p.resourceType) && addr[:len(p.resourceType)] == p.resourceType {
			count++
			totalCost = totalCost.Add(detail.MonthlyCost)
			affected = append(affected, id)
		}
	}

	// Check instance count
	if p.maxInstances > 0 && count > p.maxInstances {
		output.Passed = false
		output.Message = fmt.Sprintf("%s: %d instances exceeds limit of %d",
			p.resourceType, count, p.maxInstances)
		output.AffectedInstances = affected
	}

	// Check cost
	if p.maxCost != nil && totalCost.Cmp(*p.maxCost) > 0 {
		output.Passed = false
		if output.Message != "" {
			output.Message += "; "
		}
		output.Message += fmt.Sprintf("%s cost $%s exceeds limit $%s",
			p.resourceType, totalCost.String(), p.maxCost.String())
		output.AffectedInstances = affected
		output.AffectedCost = totalCost.Sub(*p.maxCost)
	}

	if output.Passed {
		output.Message = fmt.Sprintf("%s: %d instances, $%s/month - within limits",
			p.resourceType, count, totalCost.String())
	}

	return output, nil
}

// TagRequirementPolicy checks that instances have required tags
type TagRequirementPolicy struct {
	name         string
	requiredTags []string
}

// NewTagRequirementPolicy creates a tag requirement policy
func NewTagRequirementPolicy(name string, requiredTags []string) *TagRequirementPolicy {
	return &TagRequirementPolicy{
		name:         name,
		requiredTags: requiredTags,
	}
}

func (p *TagRequirementPolicy) Name() string { return p.name }

func (p *TagRequirementPolicy) Evaluate(ctx context.Context, input *PolicyInput) (*PolicyOutput, error) {
	output := &PolicyOutput{
		Passed: true,
	}

	missingTags := make(map[model.InstanceID][]string)

	for id, detail := range input.InstanceCosts {
		for _, req := range p.requiredTags {
			if _, ok := detail.Tags[req]; !ok {
				missingTags[id] = append(missingTags[id], req)
			}
		}
	}

	if len(missingTags) > 0 {
		output.Passed = false
		output.Message = fmt.Sprintf("%d instances missing required tags", len(missingTags))

		for id := range missingTags {
			output.AffectedInstances = append(output.AffectedInstances, id)
		}

		output.Suggestions = []string{
			"Add required tags: " + fmt.Sprintf("%v", p.requiredTags),
		}
	} else {
		output.Message = "All instances have required tags"
	}

	return output, nil
}
