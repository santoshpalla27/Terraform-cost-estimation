// Package adapter provides thin adapters over the core engine.
// CLI, HTTP, and CI adapters are all thin wrappers.
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"terraform-cost/core/engine"
	"terraform-cost/core/model"
	"terraform-cost/core/pricing"
	"terraform-cost/core/terraform"
)

// CLIAdapter is a THIN wrapper around the core engine.
// It handles input/output only - all logic is in the engine.
type CLIAdapter struct {
	engine   *engine.Engine
	pipeline *terraform.Pipeline
	output   io.Writer
	format   OutputFormat
}

// OutputFormat specifies the output format
type OutputFormat int

const (
	FormatTable OutputFormat = iota
	FormatJSON
	FormatMarkdown
)

// NewCLIAdapter creates a new CLI adapter
func NewCLIAdapter(eng *engine.Engine, pipeline *terraform.Pipeline) *CLIAdapter {
	return &CLIAdapter{
		engine:   eng,
		pipeline: pipeline,
		output:   os.Stdout,
		format:   FormatTable,
	}
}

// SetOutput sets the output writer
func (a *CLIAdapter) SetOutput(w io.Writer) {
	a.output = w
}

// SetFormat sets the output format
func (a *CLIAdapter) SetFormat(f OutputFormat) {
	a.format = f
}

// CLIRequest is the CLI input
type CLIRequest struct {
	// Path to Terraform project
	Path string

	// Variables from CLI
	Variables map[string]any

	// Snapshot specification
	SnapshotID string
	Provider   string
	Region     string

	// Usage overrides file
	UsageFile string

	// Output options
	Format     string
	ShowLineage bool
}

// Run executes the estimation
func (a *CLIAdapter) Run(ctx context.Context, req *CLIRequest) error {
	// 1. Run Terraform pipeline
	scanInput := &terraform.ScanInput{
		RootPath:  req.Path,
		Workspace: "default",
	}

	pipelineResult, err := a.pipeline.Execute(ctx, scanInput)
	if err != nil {
		return fmt.Errorf("failed to scan terraform: %w", err)
	}

	// 2. Build estimation request
	snapshotReq := engine.SnapshotRequest{
		Provider: req.Provider,
		Region:   req.Region,
	}
	if req.SnapshotID != "" {
		snapshotReq.SnapshotID = pricing.SnapshotID(req.SnapshotID)
	}

	// Load usage overrides if provided
	overrides := make(map[model.InstanceID]map[string]float64)
	if req.UsageFile != "" {
		var err error
		overrides, err = a.loadUsageOverrides(req.UsageFile)
		if err != nil {
			fmt.Fprintf(a.output, "Warning: could not load usage file: %v\n", err)
		}
	}

	// 3. Delegate to engine
	estimateReq := &engine.EstimateRequest{
		Graph:           pipelineResult.Graph,
		SnapshotRequest: snapshotReq,
		UsageOverrides:  overrides,
	}

	result, err := a.engine.Estimate(ctx, estimateReq)
	if err != nil {
		return fmt.Errorf("estimation failed: %w", err)
	}

	// 4. Format and output
	switch a.format {
	case FormatJSON:
		return a.outputJSON(result)
	case FormatMarkdown:
		return a.outputMarkdown(result)
	default:
		return a.outputTable(result, req.ShowLineage)
	}
}

func (a *CLIAdapter) loadUsageOverrides(path string) (map[model.InstanceID]map[string]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]map[string]float64
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	result := make(map[model.InstanceID]map[string]float64)
	for k, v := range raw {
		result[model.InstanceID(k)] = v
	}
	return result, nil
}

func (a *CLIAdapter) outputTable(result *engine.EstimationResult, showLineage bool) error {
	fmt.Fprintln(a.output, "")
	fmt.Fprintln(a.output, "╔══════════════════════════════════════════════════════════════════╗")
	fmt.Fprintln(a.output, "║                     COST ESTIMATION REPORT                        ║")
	fmt.Fprintln(a.output, "╚══════════════════════════════════════════════════════════════════╝")
	fmt.Fprintln(a.output, "")

	// Snapshot info
	fmt.Fprintf(a.output, "Pricing Snapshot: %s (verified: %s)\n",
		result.Snapshot.ID, result.Snapshot.ContentHash.String())
	fmt.Fprintf(a.output, "Effective Date:   %s\n", result.Snapshot.EffectiveAt.Format(time.RFC3339))
	fmt.Fprintf(a.output, "Provider/Region:  %s / %s\n", result.Snapshot.Provider, result.Snapshot.Region)
	fmt.Fprintln(a.output, "")

	// Instance costs
	fmt.Fprintln(a.output, "COSTS BY INSTANCE")
	fmt.Fprintln(a.output, "─────────────────────────────────────────────────────────────────────")
	fmt.Fprintf(a.output, "%-40s %12s %10s\n", "INSTANCE", "MONTHLY", "CONFIDENCE")
	fmt.Fprintln(a.output, "─────────────────────────────────────────────────────────────────────")

	result.InstanceCosts.Range(func(id model.InstanceID, cost *engine.InstanceCost) bool {
		confStr := fmt.Sprintf("%.0f%%", cost.Confidence.Score*100)
		if cost.Confidence.Score < 0.7 {
			confStr += " ⚠"
		}
		fmt.Fprintf(a.output, "%-40s %12s %10s\n",
			truncate(string(cost.Address), 40),
			cost.MonthlyCost.String(),
			confStr)

		// Show components if requested
		if showLineage {
			for _, comp := range cost.Components {
				fmt.Fprintf(a.output, "  └─ %-36s %12s\n",
					comp.Name, comp.MonthlyCost.String())
			}
		}
		return true
	})

	fmt.Fprintln(a.output, "─────────────────────────────────────────────────────────────────────")
	fmt.Fprintf(a.output, "%-40s %12s %10s\n",
		"TOTAL",
		result.TotalMonthlyCost.String(),
		fmt.Sprintf("%.0f%%", result.Confidence.Score*100))
	fmt.Fprintln(a.output, "")

	// Warnings
	if len(result.Warnings) > 0 {
		fmt.Fprintln(a.output, "WARNINGS")
		fmt.Fprintln(a.output, "─────────────────────────────────────────────────────────────────────")
		for _, w := range result.Warnings {
			fmt.Fprintf(a.output, "⚠ %s\n", w)
		}
		fmt.Fprintln(a.output, "")
	}

	// Policy results
	if result.PolicyResult != nil {
		fmt.Fprintln(a.output, "POLICY RESULTS")
		fmt.Fprintln(a.output, "─────────────────────────────────────────────────────────────────────")
		for _, p := range result.PolicyResult.Policies {
			status := "✓ PASS"
			if !p.Passed {
				status = "✗ FAIL"
			}
			fmt.Fprintf(a.output, "%s  %s: %s\n", status, p.Name, p.Message)
		}
		fmt.Fprintln(a.output, "")
	}

	return nil
}

func (a *CLIAdapter) outputJSON(result *engine.EstimationResult) error {
	// Convert to JSON-friendly structure
	output := map[string]interface{}{
		"snapshot": map[string]interface{}{
			"id":           result.Snapshot.ID,
			"content_hash": result.Snapshot.ContentHash.Hex(),
			"effective_at": result.Snapshot.EffectiveAt,
			"provider":     result.Snapshot.Provider,
			"region":       result.Snapshot.Region,
		},
		"total_monthly_cost": result.TotalMonthlyCost.StringRaw(),
		"total_hourly_cost":  result.TotalHourlyCost.StringRaw(),
		"confidence":         result.Confidence.Score,
		"instance_count":     result.InstanceCosts.Len(),
		"estimated_at":       result.EstimatedAt,
		"duration_ms":        result.Duration.Milliseconds(),
		"warnings":           result.Warnings,
		"degraded":           result.Degraded,
	}

	// Add instance costs
	instances := make(map[string]interface{})
	result.InstanceCosts.Range(func(id model.InstanceID, cost *engine.InstanceCost) bool {
		components := make([]map[string]interface{}, len(cost.Components))
		for i, c := range cost.Components {
			components[i] = map[string]interface{}{
				"name":         c.Name,
				"monthly_cost": c.MonthlyCost.StringRaw(),
				"hourly_cost":  c.HourlyCost.StringRaw(),
				"usage_value":  c.UsageValue,
				"usage_unit":   c.UsageUnit,
				"confidence":   c.Confidence,
			}
		}

		instances[string(id)] = map[string]interface{}{
			"address":       cost.Address,
			"definition_id": cost.DefinitionID,
			"monthly_cost":  cost.MonthlyCost.StringRaw(),
			"hourly_cost":   cost.HourlyCost.StringRaw(),
			"confidence":    cost.Confidence.Score,
			"components":    components,
		}
		return true
	})
	output["instances"] = instances

	encoder := json.NewEncoder(a.output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func (a *CLIAdapter) outputMarkdown(result *engine.EstimationResult) error {
	fmt.Fprintln(a.output, "# Cost Estimation Report")
	fmt.Fprintln(a.output, "")
	fmt.Fprintf(a.output, "**Total Monthly Cost:** %s\n", result.TotalMonthlyCost.String())
	fmt.Fprintf(a.output, "**Confidence:** %.0f%%\n", result.Confidence.Score*100)
	fmt.Fprintln(a.output, "")

	fmt.Fprintln(a.output, "## Summary")
	fmt.Fprintln(a.output, "")
	fmt.Fprintln(a.output, "| Instance | Monthly Cost | Confidence |")
	fmt.Fprintln(a.output, "|----------|-------------|------------|")

	result.InstanceCosts.Range(func(id model.InstanceID, cost *engine.InstanceCost) bool {
		fmt.Fprintf(a.output, "| `%s` | %s | %.0f%% |\n",
			cost.Address, cost.MonthlyCost.String(), cost.Confidence.Score*100)
		return true
	})

	fmt.Fprintln(a.output, "")
	fmt.Fprintf(a.output, "| **Total** | **%s** | **%.0f%%** |\n",
		result.TotalMonthlyCost.String(), result.Confidence.Score*100)

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// HTTPAdapter would be similar - thin wrapper delegating to engine
// type HTTPAdapter struct { ... }

// CIAdapter would be similar - outputs in CI-friendly format
// type CIAdapter struct { ... }
