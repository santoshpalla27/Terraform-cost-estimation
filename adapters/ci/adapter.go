// Package adapter provides production-grade CI adapter for the cost estimation engine.
// This adapter provides CI-specific output formats and policy enforcement.
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"terraform-cost/core/engine"
	"terraform-cost/core/model"
	"terraform-cost/core/pricing"
	"terraform-cost/core/terraform"
)

// isStrict returns true if strict mode is enabled (unified check)
func (a *CIAdapter) isStrict() bool {
	return a.config.StrictMode || a.config.Mode == ModeStrict
}

// CIAdapter is a production-grade CI adapter
type CIAdapter struct {
	engine   *engine.Engine
	pipeline *terraform.Pipeline
	output   io.Writer
	config   *CIConfig
}

// CIConfig configures CI behavior
type CIConfig struct {
	// Mode controls CI behavior
	Mode CIMode `json:"mode"`

	// OutputFormat for results
	OutputFormat CIOutputFormat `json:"output_format"`

	// Budget threshold (monthly)
	BudgetLimit float64 `json:"budget_limit"`

	// MaxSymbolicPercent allowed
	MaxSymbolicPercent float64 `json:"max_symbolic_percent"`

	// MaxUnsupportedPercent allowed
	MaxUnsupportedPercent float64 `json:"max_unsupported_percent"`

	// MinConfidence required
	MinConfidence float64 `json:"min_confidence"`

	// FailOnWarnings treats warnings as errors
	FailOnWarnings bool `json:"fail_on_warnings"`

	// CommentPrefix for PR comments
	CommentPrefix string `json:"comment_prefix"`

	// StrictMode fails on symbolic costs
	StrictMode bool `json:"strict_mode"`
}

// CIMode controls CI behavior
type CIMode string

const (
	// ModeInformational only comments, no check result
	ModeInformational CIMode = "informational"

	// ModeWarning comments and warns but doesn't block
	ModeWarning CIMode = "warning"

	// ModeBlocking fails the check on policy violations
	ModeBlocking CIMode = "blocking"

	// ModeStrict fails on any symbolic or unsupported
	ModeStrict CIMode = "strict"
)

// CIOutputFormat specifies output format
type CIOutputFormat string

const (
	FormatJSON     CIOutputFormat = "json"
	FormatMarkdown CIOutputFormat = "markdown"
	FormatTable    CIOutputFormat = "table"
	FormatGitHub   CIOutputFormat = "github"
	FormatGitLab   CIOutputFormat = "gitlab"
)

// DefaultCIConfig returns production defaults
func DefaultCIConfig() *CIConfig {
	return &CIConfig{
		Mode:                  ModeBlocking,
		OutputFormat:          FormatMarkdown,
		BudgetLimit:           0, // No limit
		MaxSymbolicPercent:    20,
		MaxUnsupportedPercent: 10,
		MinConfidence:         0.7,
		FailOnWarnings:        false,
		CommentPrefix:         "💰 Terraform Cost Estimate",
		StrictMode:            false,
	}
}

// NewCIAdapter creates a CI adapter
func NewCIAdapter(eng *engine.Engine, pipeline *terraform.Pipeline, config *CIConfig) *CIAdapter {
	if config == nil {
		config = DefaultCIConfig()
	}
	return &CIAdapter{
		engine:   eng,
		pipeline: pipeline,
		output:   os.Stdout,
		config:   config,
	}
}

// SetOutput sets the output writer
func (a *CIAdapter) SetOutput(w io.Writer) {
	a.output = w
}

// CIRequest is the CI input
type CIRequest struct {
	// Path to Terraform project
	Path string

	// PlanFile is the tfplan file path
	PlanFile string

	// Provider (aws, azure, gcp)
	Provider string

	// Region
	Region string

	// SnapshotID to use
	SnapshotID string

	// UsageFile for overrides
	UsageFile string

	// BaseFile for diff comparison
	BaseFile string
}

// CIResult is the CI output
type CIResult struct {
	// Success indicates estimation succeeded
	Success bool `json:"success"`

	// ExitCode for CI
	ExitCode int `json:"exit_code"`

	// CheckConclusion for GitHub
	CheckConclusion string `json:"check_conclusion"` // success, neutral, failure

	// Summary for check run
	Summary string `json:"summary"`

	// TotalCost monthly
	TotalCost float64 `json:"total_cost"`

	// Confidence (0-1)
	Confidence float64 `json:"confidence"`

	// Coverage breakdown
	Coverage CICoverage `json:"coverage"`

	// PolicyViolations
	PolicyViolations []PolicyViolation `json:"policy_violations,omitempty"`

	// Warnings
	Warnings []string `json:"warnings,omitempty"`

	// Diff if comparing
	Diff *CIDiff `json:"diff,omitempty"`

	// Resources with costs
	Resources []CIResourceCost `json:"resources"`

	// Snapshot used
	Snapshot CISnapshot `json:"snapshot"`

	// Metadata
	Metadata CIMetadata `json:"metadata"`
}

// CICoverage is coverage breakdown
type CICoverage struct {
	NumericPercent     float64 `json:"numeric_percent"`
	SymbolicPercent    float64 `json:"symbolic_percent"`
	IndirectPercent    float64 `json:"indirect_percent"`
	UnsupportedPercent float64 `json:"unsupported_percent"`
}

// CIDiff is cost comparison
type CIDiff struct {
	OldCost       float64 `json:"old_cost"`
	NewCost       float64 `json:"new_cost"`
	Delta         float64 `json:"delta"`
	DeltaPercent  float64 `json:"delta_percent"`
	CreatedCount  int     `json:"created_count"`
	DestroyedCount int    `json:"destroyed_count"`
	UpdatedCount  int     `json:"updated_count"`
}

// CIResourceCost is per-resource cost
type CIResourceCost struct {
	Address      string  `json:"address"`
	Type         string  `json:"type"`
	MonthlyCost  float64 `json:"monthly_cost"`
	Confidence   float64 `json:"confidence"`
	CoverageType string  `json:"coverage_type"`
	ChangeType   string  `json:"change_type,omitempty"` // create, destroy, update
	Delta        float64 `json:"delta,omitempty"`
}

// CISnapshot is snapshot info
type CISnapshot struct {
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	Region      string `json:"region"`
	ContentHash string `json:"content_hash"`
}

// CIMetadata is execution context
type CIMetadata struct {
	Timestamp time.Time `json:"timestamp"`
	Duration  string    `json:"duration"`
	Version   string    `json:"version"`
	Mode      string    `json:"mode"`
}

// PolicyViolation is a policy failure
type PolicyViolation struct {
	Rule      string  `json:"rule"`
	Message   string  `json:"message"`
	Severity  string  `json:"severity"` // error, warning
	Threshold float64 `json:"threshold,omitempty"`
	Actual    float64 `json:"actual,omitempty"`
}

// Run executes the CI estimation
func (a *CIAdapter) Run(ctx context.Context, req *CIRequest) (*CIResult, error) {
	start := time.Now()

	// 1. Run Terraform pipeline
	scanInput := &terraform.ScanInput{
		RootPath:  req.Path,
		Workspace: "default",
	}

	pipelineResult, err := a.pipeline.Execute(ctx, scanInput)
	if err != nil {
		return a.failResult(fmt.Sprintf("Failed to scan terraform: %v", err), start), nil
	}

	// 2. Build snapshot request
	snapshotReq := engine.SnapshotRequest{
		Provider: req.Provider,
		Region:   req.Region,
	}
	if req.SnapshotID != "" {
		snapshotReq.SnapshotID = pricing.SnapshotID(req.SnapshotID)
	}

	// 3. Load usage overrides
	overrides := make(map[model.InstanceID]map[string]float64)
	if req.UsageFile != "" {
		if data, err := os.ReadFile(req.UsageFile); err == nil {
			var raw map[string]map[string]float64
			if json.Unmarshal(data, &raw) == nil {
				for k, v := range raw {
					overrides[model.InstanceID(k)] = v
				}
			}
		}
	}

	// 4. Execute estimation
	engineReq := &engine.EstimateRequest{
		Graph:           pipelineResult.Graph,
		SnapshotRequest: snapshotReq,
		UsageOverrides:  overrides,
	}

	result, err := a.engine.Estimate(ctx, engineReq)
	if err != nil {
		return a.failResult(fmt.Sprintf("Estimation failed: %v", err), start), nil
	}

	// 5. Build CI result
	ciResult := a.buildCIResult(result, start)

	// 6. Evaluate policies
	a.evaluatePolicies(ciResult)

	// 7. Output in requested format
	if err := a.output(ciResult); err != nil {
		return nil, err
	}

	return ciResult, nil
}

func (a *CIAdapter) buildCIResult(result *engine.EstimationResult, start time.Time) *CIResult {
	ciResult := &CIResult{
		Success:    true,
		ExitCode:   0,
		TotalCost:  result.TotalMonthlyCost.Float64(),
		Confidence: result.Confidence.Score,
		Warnings:   result.Warnings,
		Metadata: CIMetadata{
			Timestamp: time.Now(),
			Duration:  time.Since(start).String(),
			Version:   "1.0.0",
			Mode:      string(a.config.Mode),
		},
	}

	// FIX #1: Populate coverage from engine result
	if result.CoverageReport != nil {
		ciResult.Coverage = CICoverage{
			NumericPercent:     result.CoverageReport.NumericPercent,
			SymbolicPercent:    result.CoverageReport.SymbolicPercent,
			IndirectPercent:    result.CoverageReport.IndirectPercent,
			UnsupportedPercent: result.CoverageReport.UnsupportedPercent,
		}
	} else {
		// Fallback: calculate from resources if CoverageReport not available
		numericCount := 0
		symbolicCount := 0
		unsupportedCount := 0
		totalCount := 0
		result.InstanceCosts.Range(func(id model.InstanceID, cost *engine.InstanceCost) bool {
			totalCount++
			switch cost.CoverageType {
			case engine.CoverageTypeNumeric:
				numericCount++
			case engine.CoverageTypeSymbolic:
				symbolicCount++
			case engine.CoverageTypeUnsupported:
				unsupportedCount++
			}
			return true
		})
		if totalCount > 0 {
			ciResult.Coverage = CICoverage{
				NumericPercent:     float64(numericCount) / float64(totalCount) * 100,
				SymbolicPercent:    float64(symbolicCount) / float64(totalCount) * 100,
				UnsupportedPercent: float64(unsupportedCount) / float64(totalCount) * 100,
			}
		}
	}

	// Snapshot
	if result.Snapshot != nil {
		ciResult.Snapshot = CISnapshot{
			ID:          string(result.Snapshot.ID),
			Provider:    result.Snapshot.Provider,
			Region:      result.Snapshot.Region,
			ContentHash: result.Snapshot.ContentHash.Hex(),
		}
	}

	// Resources: collect all first
	var resources []CIResourceCost
	result.InstanceCosts.Range(func(id model.InstanceID, cost *engine.InstanceCost) bool {
		// FIX #3: Set CoverageType from core coverage classification
		coverageType := "numeric" // default
		switch cost.CoverageType {
		case engine.CoverageTypeSymbolic:
			coverageType = "symbolic"
		case engine.CoverageTypeUnsupported:
			coverageType = "unsupported"
		case engine.CoverageTypeIndirect:
			coverageType = "indirect"
		}

		rc := CIResourceCost{
			Address:      string(cost.Address),
			Type:         string(cost.ResourceType),
			MonthlyCost:  cost.MonthlyCost.Float64(),
			Confidence:   cost.Confidence.Score,
			CoverageType: coverageType,
		}
		resources = append(resources, rc)
		return true
	})

	// FIX #2: Sort resources by cost descending for consistent output
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].MonthlyCost > resources[j].MonthlyCost
	})

	ciResult.Resources = resources

	return ciResult
}

func (a *CIAdapter) evaluatePolicies(result *CIResult) {
	// Budget check
	if a.config.BudgetLimit > 0 && result.TotalCost > a.config.BudgetLimit {
		result.PolicyViolations = append(result.PolicyViolations, PolicyViolation{
			Rule:      "budget_limit",
			Message:   fmt.Sprintf("Monthly cost $%.2f exceeds budget $%.2f", result.TotalCost, a.config.BudgetLimit),
			Severity:  "error",
			Threshold: a.config.BudgetLimit,
			Actual:    result.TotalCost,
		})
	}

	// Symbolic check
	if result.Coverage.SymbolicPercent > a.config.MaxSymbolicPercent {
		severity := "warning"
		if a.isStrict() { // FIX #4: Use unified isStrict() helper
			severity = "error"
		}
		result.PolicyViolations = append(result.PolicyViolations, PolicyViolation{
			Rule:      "symbolic_limit",
			Message:   fmt.Sprintf("Symbolic coverage %.1f%% exceeds limit %.1f%%", result.Coverage.SymbolicPercent, a.config.MaxSymbolicPercent),
			Severity:  severity,
			Threshold: a.config.MaxSymbolicPercent,
			Actual:    result.Coverage.SymbolicPercent,
		})
	}

	// Unsupported check
	if result.Coverage.UnsupportedPercent > a.config.MaxUnsupportedPercent {
		result.PolicyViolations = append(result.PolicyViolations, PolicyViolation{
			Rule:      "unsupported_limit",
			Message:   fmt.Sprintf("Unsupported coverage %.1f%% exceeds limit %.1f%%", result.Coverage.UnsupportedPercent, a.config.MaxUnsupportedPercent),
			Severity:  "warning",
			Threshold: a.config.MaxUnsupportedPercent,
			Actual:    result.Coverage.UnsupportedPercent,
		})
	}

	// Confidence check
	if result.Confidence < a.config.MinConfidence {
		result.PolicyViolations = append(result.PolicyViolations, PolicyViolation{
			Rule:      "confidence_minimum",
			Message:   fmt.Sprintf("Confidence %.0f%% below minimum %.0f%%", result.Confidence*100, a.config.MinConfidence*100),
			Severity:  "warning",
			Threshold: a.config.MinConfidence,
			Actual:    result.Confidence,
		})
	}

	// Determine exit code and check conclusion
	hasErrors := false
	hasWarnings := false
	for _, v := range result.PolicyViolations {
		if v.Severity == "error" {
			hasErrors = true
		} else {
			hasWarnings = true
		}
	}

	switch a.config.Mode {
	case ModeInformational:
		result.ExitCode = 0
		result.CheckConclusion = "success"
	case ModeWarning:
		result.ExitCode = 0
		if hasErrors || hasWarnings {
			result.CheckConclusion = "neutral"
		} else {
			result.CheckConclusion = "success"
		}
	case ModeBlocking, ModeStrict:
		if hasErrors {
			result.ExitCode = 1
			result.CheckConclusion = "failure"
		} else if hasWarnings && a.config.FailOnWarnings {
			result.ExitCode = 1
			result.CheckConclusion = "failure"
		} else {
			result.ExitCode = 0
			result.CheckConclusion = "success"
		}
	}

	// Build summary
	result.Summary = a.buildSummary(result)
}

func (a *CIAdapter) buildSummary(result *CIResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Total Monthly Cost: $%.2f\n", result.TotalCost))
	sb.WriteString(fmt.Sprintf("Confidence: %.0f%%\n", result.Confidence*100))
	sb.WriteString(fmt.Sprintf("Coverage: %.0f%% numeric | %.0f%% symbolic | %.0f%% unsupported\n",
		result.Coverage.NumericPercent,
		result.Coverage.SymbolicPercent,
		result.Coverage.UnsupportedPercent,
	))

	if len(result.PolicyViolations) > 0 {
		sb.WriteString("\nPolicy Violations:\n")
		for _, v := range result.PolicyViolations {
			icon := "⚠️"
			if v.Severity == "error" {
				icon = "❌"
			}
			sb.WriteString(fmt.Sprintf("%s %s: %s\n", icon, v.Rule, v.Message))
		}
	}

	return sb.String()
}

func (a *CIAdapter) failResult(message string, start time.Time) *CIResult {
	return &CIResult{
		Success:         false,
		ExitCode:        1,
		CheckConclusion: "failure",
		Summary:         message,
		Metadata: CIMetadata{
			Timestamp: time.Now(),
			Duration:  time.Since(start).String(),
			Version:   "1.0.0",
		},
	}
}

func (a *CIAdapter) output(result *CIResult) error {
	switch a.config.OutputFormat {
	case FormatJSON:
		return a.outputJSON(result)
	case FormatMarkdown, FormatGitHub:
		return a.outputMarkdown(result)
	case FormatTable:
		return a.outputTable(result)
	default:
		return a.outputMarkdown(result)
	}
}

func (a *CIAdapter) outputJSON(result *CIResult) error {
	enc := json.NewEncoder(a.output)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func (a *CIAdapter) outputMarkdown(result *CIResult) error {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## %s\n\n", a.config.CommentPrefix))

	// Summary
	delta := ""
	if result.Diff != nil {
		sign := "+"
		if result.Diff.Delta < 0 {
			sign = ""
		}
		delta = fmt.Sprintf(" (%s$%.2f)", sign, result.Diff.Delta)
	}

	sb.WriteString(fmt.Sprintf("**Total Monthly Cost:** $%.2f%s\n", result.TotalCost, delta))
	sb.WriteString(fmt.Sprintf("**Confidence:** %.0f%%\n", result.Confidence*100))
	sb.WriteString(fmt.Sprintf("**Coverage:** %.0f%% numeric | %.0f%% symbolic | %.0f%% unsupported\n\n",
		result.Coverage.NumericPercent,
		result.Coverage.SymbolicPercent,
		result.Coverage.UnsupportedPercent,
	))

	// Top resources
	sb.WriteString("### Top Resources by Cost\n")
	count := 5
	if len(result.Resources) < count {
		count = len(result.Resources)
	}
	for i := 0; i < count; i++ {
		r := result.Resources[i]
		icon := "🟢"
		if r.CoverageType == "symbolic" {
			icon = "🟡"
		} else if r.CoverageType == "unsupported" {
			icon = "🔴"
		}
		sb.WriteString(fmt.Sprintf("- %s `%s`: $%.2f\n", icon, r.Address, r.MonthlyCost))
	}
	sb.WriteString("\n")

	// Policy violations
	if len(result.PolicyViolations) > 0 {
		sb.WriteString("### Policy Violations\n")
		for _, v := range result.PolicyViolations {
			icon := "⚠️"
			if v.Severity == "error" {
				icon = "❌"
			}
			sb.WriteString(fmt.Sprintf("- %s **%s**: %s\n", icon, v.Rule, v.Message))
		}
		sb.WriteString("\n")
	}

	// Snapshot
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("📦 Snapshot: `%s/%s` @ %s\n",
		result.Snapshot.Provider,
		result.Snapshot.Region,
		result.Snapshot.ID[:8],
	))

	_, err := a.output.Write([]byte(sb.String()))
	return err
}

func (a *CIAdapter) outputTable(result *CIResult) error {
	var sb strings.Builder

	sb.WriteString("┌────────────────────────────────────────────────────────────┐\n")
	sb.WriteString(fmt.Sprintf("│ Total Monthly Cost           $%-28.2f │\n", result.TotalCost))
	sb.WriteString(fmt.Sprintf("│ Confidence                   %-29.0f%% │\n", result.Confidence*100))
	sb.WriteString("├────────────────────────────────────────────────────────────┤\n")
	sb.WriteString(fmt.Sprintf("│ Numeric: %.0f%%  Symbolic: %.0f%%  Unsupported: %.0f%%            │\n",
		result.Coverage.NumericPercent,
		result.Coverage.SymbolicPercent,
		result.Coverage.UnsupportedPercent,
	))
	sb.WriteString("└────────────────────────────────────────────────────────────┘\n")

	_, err := a.output.Write([]byte(sb.String()))
	return err
}
