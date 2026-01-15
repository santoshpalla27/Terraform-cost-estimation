// Package policy provides the policy evaluation interface.
// This package enforces cost guardrails before deployment.
package policy

import (
	"context"

	"terraform-cost/core/types"
)

// Rule defines a single policy rule
type Rule interface {
	// Name returns the rule identifier
	Name() string

	// Description returns a human-readable description
	Description() string

	// Evaluate checks the rule against the cost graph
	// prev is the previous cost graph for diff-based policies (can be nil)
	Evaluate(ctx context.Context, current *types.CostGraph, prev *types.CostGraph) (*RuleResult, error)
}

// RuleResult contains the evaluation output for a single rule
type RuleResult struct {
	// RuleName is the rule that was evaluated
	RuleName string `json:"rule_name"`

	// Passed indicates if the rule passed
	Passed bool `json:"passed"`

	// Severity is the rule severity
	Severity Severity `json:"severity"`

	// Message is a human-readable result message
	Message string `json:"message"`

	// Details contains additional context
	Details map[string]interface{} `json:"details,omitempty"`

	// Violations lists specific violations
	Violations []Violation `json:"violations,omitempty"`
}

// Violation represents a specific policy violation
type Violation struct {
	// Resource is the violating resource
	Resource string `json:"resource"`

	// Message describes the violation
	Message string `json:"message"`

	// Details contains additional context
	Details map[string]interface{} `json:"details,omitempty"`
}

// Severity levels for policy violations
type Severity string

const (
	// SeverityInfo is informational only
	SeverityInfo Severity = "info"

	// SeverityWarning is a warning that doesn't block
	SeverityWarning Severity = "warning"

	// SeverityError is an error but doesn't block
	SeverityError Severity = "error"

	// SeverityBlock blocks deployment
	SeverityBlock Severity = "block"
)

// Evaluator runs all policy rules
type Evaluator interface {
	// RegisterRule adds a rule to the evaluator
	RegisterRule(rule Rule) error

	// Evaluate runs all rules and returns results
	Evaluate(ctx context.Context, current *types.CostGraph, prev *types.CostGraph) (*EvaluationResult, error)

	// EvaluateRules runs specific rules
	EvaluateRules(ctx context.Context, ruleNames []string, current *types.CostGraph, prev *types.CostGraph) (*EvaluationResult, error)
}

// EvaluationResult contains all rule results
type EvaluationResult struct {
	// Results contains individual rule results
	Results []*RuleResult `json:"results"`

	// PassedCount is the number of passed rules
	PassedCount int `json:"passed_count"`

	// FailedCount is the number of failed rules
	FailedCount int `json:"failed_count"`

	// Blocked indicates if any rule blocks deployment
	Blocked bool `json:"blocked"`

	// BlockReason explains why deployment is blocked
	BlockReason string `json:"block_reason,omitempty"`
}

// HasFailures returns true if any rules failed
func (r *EvaluationResult) HasFailures() bool {
	return r.FailedCount > 0
}

// GetBlockingRules returns rules that block deployment
func (r *EvaluationResult) GetBlockingRules() []*RuleResult {
	var blocking []*RuleResult
	for _, result := range r.Results {
		if !result.Passed && result.Severity == SeverityBlock {
			blocking = append(blocking, result)
		}
	}
	return blocking
}

// PolicyConfig contains policy configuration
type PolicyConfig struct {
	// Rules are the rules to evaluate
	Rules []RuleConfig `json:"rules"`

	// StopOnBlock stops evaluation on first blocking rule
	StopOnBlock bool `json:"stop_on_block"`
}

// RuleConfig contains configuration for a specific rule
type RuleConfig struct {
	// Name is the rule name
	Name string `json:"name"`

	// Enabled indicates if the rule is enabled
	Enabled bool `json:"enabled"`

	// Severity overrides the default severity
	Severity *Severity `json:"severity,omitempty"`

	// Parameters contains rule-specific parameters
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}
