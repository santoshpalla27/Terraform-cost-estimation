// Package policy - Diff-aware policy engine
// Policies can reason over change sets, not just aggregates.
// Can block "new unknown costs" and evaluate "new resources only".
package policy

import (
	"terraform-cost/core/graph"
	"terraform-cost/core/model"
)

// DiffAwarePolicy evaluates changes, not just totals
type DiffAwarePolicy interface {
	// Name returns the policy name
	Name() string

	// EvaluateDiff evaluates a diff
	EvaluateDiff(ctx *DiffPolicyContext) *DiffPolicyResult
}

// DiffPolicyContext provides diff context to policies
type DiffPolicyContext struct {
	// Before state (nil for new infrastructure)
	Before *CostSnapshot

	// After state
	After *CostSnapshot

	// Change analysis
	Changes *graph.ChangeCostAnalysis

	// Scope filter
	Scope DiffScope

	// Confidence context
	ConfidenceInfo *DiffConfidenceInfo
}

// CostSnapshot is a point-in-time cost state
type CostSnapshot struct {
	TotalMonthly  float64
	Resources     map[model.InstanceID]float64
	ByService     map[string]float64
	Confidence    float64
	Timestamp     string
}

// DiffScope defines what to evaluate
type DiffScope struct {
	// Only evaluate new resources
	NewResourcesOnly bool

	// Only evaluate production changes
	ProductionOnly bool

	// Only evaluate specific services
	Services []string

	// Only evaluate specific resource types
	ResourceTypes []string
}

// DiffConfidenceInfo tracks confidence changes
type DiffConfidenceInfo struct {
	// Before confidence
	BeforeConfidence float64

	// After confidence
	AfterConfidence float64

	// New unknowns introduced
	NewUnknowns []NewUnknownItem

	// Low confidence items
	LowConfidenceItems []LowConfItem
}

// NewUnknownItem is a new unknown cost
type NewUnknownItem struct {
	Address    string
	Reason     string
	CostImpact float64
}

// LowConfItem is a low confidence item
type LowConfItem struct {
	Address    string
	Confidence float64
	Reason     string
	Cost       float64
}

// DiffPolicyResult is the result of diff policy evaluation
type DiffPolicyResult struct {
	PolicyName    string
	Passed        bool
	Violations    []DiffViolation
	Warnings      []DiffWarning
	CostImpact    float64
	Recommendation string
}

// DiffViolation is a policy violation in a diff
type DiffViolation struct {
	Type        ViolationType
	Address     string
	Reason      string
	CostImpact  float64
	Blocking    bool
}

// ViolationType classifies violations
type ViolationType int

const (
	ViolationBudgetExceeded    ViolationType = iota
	ViolationNewUnknown
	ViolationConfidenceDropped
	ViolationNewHighCost
	ViolationUnauthorizedService
)

// String returns the violation type name
func (v ViolationType) String() string {
	switch v {
	case ViolationBudgetExceeded:
		return "budget_exceeded"
	case ViolationNewUnknown:
		return "new_unknown"
	case ViolationConfidenceDropped:
		return "confidence_dropped"
	case ViolationNewHighCost:
		return "new_high_cost"
	case ViolationUnauthorizedService:
		return "unauthorized_service"
	default:
		return "unknown"
	}
}

// DiffWarning is a warning in a diff
type DiffWarning struct {
	Type    string
	Message string
	Address string
}

// NewUnknownsPolicy blocks new unknown costs
type NewUnknownsPolicy struct {
	// Block on any new unknowns
	BlockOnNew bool

	// Minimum confidence for new resources
	MinConfidence float64
}

// NewNewUnknownsPolicy creates a policy
func NewNewUnknownsPolicy(block bool, minConfidence float64) *NewUnknownsPolicy {
	return &NewUnknownsPolicy{
		BlockOnNew:    block,
		MinConfidence: minConfidence,
	}
}

// Name returns the policy name
func (p *NewUnknownsPolicy) Name() string {
	return "new-unknowns"
}

// EvaluateDiff evaluates for new unknowns
func (p *NewUnknownsPolicy) EvaluateDiff(ctx *DiffPolicyContext) *DiffPolicyResult {
	result := &DiffPolicyResult{
		PolicyName: p.Name(),
		Passed:     true,
		Violations: []DiffViolation{},
		Warnings:   []DiffWarning{},
	}

	if ctx.ConfidenceInfo == nil {
		return result
	}

	// Check for new unknowns
	for _, unknown := range ctx.ConfidenceInfo.NewUnknowns {
		if p.BlockOnNew {
			result.Passed = false
			result.Violations = append(result.Violations, DiffViolation{
				Type:       ViolationNewUnknown,
				Address:    unknown.Address,
				Reason:     unknown.Reason,
				CostImpact: unknown.CostImpact,
				Blocking:   true,
			})
		} else {
			result.Warnings = append(result.Warnings, DiffWarning{
				Type:    "new_unknown",
				Message: unknown.Reason,
				Address: unknown.Address,
			})
		}
	}

	// Check confidence threshold
	for _, item := range ctx.ConfidenceInfo.LowConfidenceItems {
		if item.Confidence < p.MinConfidence {
			result.Violations = append(result.Violations, DiffViolation{
				Type:       ViolationConfidenceDropped,
				Address:    item.Address,
				Reason:     item.Reason,
				CostImpact: item.Cost,
				Blocking:   false,
			})
		}
	}

	return result
}

// DeltaBudgetPolicy checks change amounts against budgets
type DeltaBudgetPolicy struct {
	// Maximum monthly increase
	MaxMonthlyIncrease float64

	// Maximum percentage increase
	MaxPercentIncrease float64

	// Per-service limits
	ServiceLimits map[string]float64
}

// NewDeltaBudgetPolicy creates a policy
func NewDeltaBudgetPolicy(maxIncrease, maxPercent float64) *DeltaBudgetPolicy {
	return &DeltaBudgetPolicy{
		MaxMonthlyIncrease: maxIncrease,
		MaxPercentIncrease: maxPercent,
		ServiceLimits:      make(map[string]float64),
	}
}

// Name returns the policy name
func (p *DeltaBudgetPolicy) Name() string {
	return "delta-budget"
}

// EvaluateDiff evaluates budget against changes
func (p *DeltaBudgetPolicy) EvaluateDiff(ctx *DiffPolicyContext) *DiffPolicyResult {
	result := &DiffPolicyResult{
		PolicyName: p.Name(),
		Passed:     true,
		Violations: []DiffViolation{},
	}

	if ctx.Before == nil || ctx.After == nil {
		return result
	}

	// Calculate delta
	delta := ctx.After.TotalMonthly - ctx.Before.TotalMonthly
	result.CostImpact = delta

	// Check absolute increase
	if p.MaxMonthlyIncrease > 0 && delta > p.MaxMonthlyIncrease {
		result.Passed = false
		result.Violations = append(result.Violations, DiffViolation{
			Type:       ViolationBudgetExceeded,
			Reason:     "monthly increase exceeds limit",
			CostImpact: delta,
			Blocking:   true,
		})
	}

	// Check percentage increase
	if ctx.Before.TotalMonthly > 0 && p.MaxPercentIncrease > 0 {
		percentIncrease := (delta / ctx.Before.TotalMonthly) * 100
		if percentIncrease > p.MaxPercentIncrease {
			result.Passed = false
			result.Violations = append(result.Violations, DiffViolation{
				Type:       ViolationBudgetExceeded,
				Reason:     "percentage increase exceeds limit",
				CostImpact: delta,
				Blocking:   true,
			})
		}
	}

	// Check per-service limits
	for service, limit := range p.ServiceLimits {
		beforeCost := ctx.Before.ByService[service]
		afterCost := ctx.After.ByService[service]
		serviceDelta := afterCost - beforeCost

		if serviceDelta > limit {
			result.Passed = false
			result.Violations = append(result.Violations, DiffViolation{
				Type:       ViolationBudgetExceeded,
				Reason:     service + " service increase exceeds limit",
				CostImpact: serviceDelta,
				Blocking:   true,
			})
		}
	}

	return result
}

// NewResourcesOnlyPolicy evaluates only new resources
type NewResourcesOnlyPolicy struct {
	// Inner policy to apply
	inner DiffAwarePolicy
}

// NewNewResourcesOnlyPolicy creates a wrapper
func NewNewResourcesOnlyPolicy(inner DiffAwarePolicy) *NewResourcesOnlyPolicy {
	return &NewResourcesOnlyPolicy{inner: inner}
}

// Name returns the policy name
func (p *NewResourcesOnlyPolicy) Name() string {
	return "new-resources-only:" + p.inner.Name()
}

// EvaluateDiff evaluates only new resources
func (p *NewResourcesOnlyPolicy) EvaluateDiff(ctx *DiffPolicyContext) *DiffPolicyResult {
	// Create filtered context with only new resources
	filteredCtx := &DiffPolicyContext{
		Before: &CostSnapshot{
			TotalMonthly: 0,
			Resources:    make(map[model.InstanceID]float64),
			ByService:    make(map[string]float64),
			Confidence:   1.0,
		},
		After:          ctx.After,
		ConfidenceInfo: ctx.ConfidenceInfo,
		Scope:          ctx.Scope,
	}

	// Remove resources that existed before
	if ctx.Before != nil {
		for id := range ctx.Before.Resources {
			delete(filteredCtx.After.Resources, id)
		}
	}

	// Recalculate total
	filteredCtx.After.TotalMonthly = 0
	for _, cost := range filteredCtx.After.Resources {
		filteredCtx.After.TotalMonthly += cost
	}

	return p.inner.EvaluateDiff(filteredCtx)
}

// DiffPolicyEngine evaluates multiple diff-aware policies
type DiffPolicyEngine struct {
	policies []DiffAwarePolicy
}

// NewDiffPolicyEngine creates an engine
func NewDiffPolicyEngine() *DiffPolicyEngine {
	return &DiffPolicyEngine{
		policies: []DiffAwarePolicy{},
	}
}

// AddPolicy adds a policy
func (e *DiffPolicyEngine) AddPolicy(policy DiffAwarePolicy) {
	e.policies = append(e.policies, policy)
}

// Evaluate evaluates all policies
func (e *DiffPolicyEngine) Evaluate(ctx *DiffPolicyContext) *DiffPolicyEngineResult {
	result := &DiffPolicyEngineResult{
		Passed:  true,
		Results: []*DiffPolicyResult{},
	}

	for _, policy := range e.policies {
		policyResult := policy.EvaluateDiff(ctx)
		result.Results = append(result.Results, policyResult)

		if !policyResult.Passed {
			result.Passed = false
		}
	}

	return result
}

// DiffPolicyEngineResult is the result of evaluating all policies
type DiffPolicyEngineResult struct {
	Passed  bool
	Results []*DiffPolicyResult
}

// BlockingViolations returns all blocking violations
func (r *DiffPolicyEngineResult) BlockingViolations() []DiffViolation {
	var violations []DiffViolation
	for _, result := range r.Results {
		for _, v := range result.Violations {
			if v.Blocking {
				violations = append(violations, v)
			}
		}
	}
	return violations
}
