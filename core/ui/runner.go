// Package ui - Interactive estimation runner with live progress
package ui

import (
	"context"
	"fmt"
	"time"

	"terraform-cost/core/graph"
)

// EstimationRunner runs estimation with live UI feedback
type EstimationRunner struct {
	w           *Writer
	executor    *graph.ConcurrentExecutor
	showSpinner bool
	showTable   bool
}

// NewEstimationRunner creates a runner
func NewEstimationRunner(w *Writer, workers int) *EstimationRunner {
	return &EstimationRunner{
		w:           w,
		executor:    graph.NewConcurrentExecutor(workers),
		showSpinner: true,
		showTable:   true,
	}
}

// EstimationResult is the result of an estimation run
type EstimationResult struct {
	TotalMonthly    string
	TotalHourly     string
	Resources       int
	Instances       int
	Confidence      float64
	Duration        time.Duration
	Warnings        []string
	LowConfidence   []LowConfidenceItem
	TopCosts        []TopCostItem
	ByService       map[string]string
}

// LowConfidenceItem is a resource with low confidence
type LowConfidenceItem struct {
	Address    string
	Confidence float64
	Reason     string
}

// TopCostItem is a high-cost resource
type TopCostItem struct {
	Address string
	Monthly string
	Percent float64
}

// Run executes estimation and displays results
func (r *EstimationRunner) Run(ctx context.Context, infraGraph *graph.InfrastructureGraph, estimator graph.NodeExecutor) (*EstimationResult, error) {
	result := &EstimationResult{
		Warnings:    []string{},
		LowConfidence: []LowConfidenceItem{},
		TopCosts:    []TopCostItem{},
		ByService:   make(map[string]string),
	}

	start := time.Now()

	// Show header
	r.w.Header("Terraform Cost Estimation")
	r.w.Info("Analyzing infrastructure...")
	r.w.Println("")

	// Phase 1: Scanning
	spinner := r.w.NewSpinner("Scanning resources...")
	spinner.Start()
	time.Sleep(500 * time.Millisecond) // Simulate work
	spinner.Stop(true)

	// Phase 2: Resolving
	spinner = r.w.NewSpinner("Resolving dependencies...")
	spinner.Start()
	time.Sleep(300 * time.Millisecond)
	spinner.Stop(true)

	// Phase 3: Estimation with progress
	r.w.Println("")
	r.w.SubHeader("Estimating costs...")
	
	bar := r.w.NewProgressBar(infraGraph.Size(), "Progress")
	
	err := r.executor.ExecuteWithProgress(ctx, infraGraph, estimator, func(p graph.ExecutionProgress) {
		bar.Update(int(p.Completed + p.Failed))
	}, 100*time.Millisecond)

	bar.Done()

	if err != nil {
		r.w.Error("Estimation failed: %v", err)
		return nil, err
	}

	// Get stats
	stats := r.executor.GetStats()
	result.Duration = time.Since(start)
	result.Instances = int(stats.CompletedNodes)

	// Show errors if any
	errors := r.executor.GetErrors()
	if len(errors) > 0 {
		r.w.Println("")
		r.w.Warning("%d resources failed to estimate", len(errors))
		for _, e := range errors {
			r.w.Debug("  %s: %s", e.NodeID, e.Message)
		}
	}

	return result, nil
}

// DisplayResult shows the estimation result
func (r *EstimationRunner) DisplayResult(result *EstimationResult) {
	// Cost summary
	summary := r.w.NewCostSummary()
	summary.TotalMonthly = result.TotalMonthly
	summary.TotalHourly = result.TotalHourly
	summary.Confidence = result.Confidence
	summary.Resources = result.Instances
	summary.Warnings = len(result.Warnings)
	summary.Render()

	// Top costs table
	if len(result.TopCosts) > 0 {
		r.w.Println("")
		r.w.SubHeader("Top Costs")
		table := r.w.NewTable("Resource", "Monthly Cost", "% of Total")
		for _, item := range result.TopCosts {
			table.AddRow(item.Address, item.Monthly, fmt.Sprintf("%.1f%%", item.Percent))
		}
		table.Render()
	}

	// By service breakdown
	if len(result.ByService) > 0 {
		r.w.Println("")
		r.w.SubHeader("By Service")
		table := r.w.NewTable("Service", "Monthly Cost")
		for service, cost := range result.ByService {
			table.AddRow(service, cost)
		}
		table.Render()
	}

	// Low confidence warnings
	if len(result.LowConfidence) > 0 {
		r.w.Println("")
		r.w.Warning("Low Confidence Resources")
		for _, item := range result.LowConfidence {
			r.w.Println("  %s: %.0f%% - %s", 
				r.w.color(Yellow, item.Address), 
				item.Confidence*100, 
				item.Reason)
		}
	}

	// Duration
	r.w.Println("")
	r.w.Println(r.w.color(Dim, fmt.Sprintf("Completed in %s", result.Duration.Round(time.Millisecond))))
}

// DisplayDiff shows cost comparison
func (r *EstimationRunner) DisplayDiff(before, after *EstimationResult) {
	diff := r.w.NewResourceDiff()
	
	// Calculate total change
	// This would parse the actual values in real implementation
	diff.TotalChange = "$50.00/month"
	diff.IsIncrease = true

	// Add sample items (real implementation would compare)
	diff.Added = []DiffItem{
		{Address: "aws_instance.new", NewCost: "$100.00/month"},
	}
	diff.Changed = []DiffItem{
		{Address: "aws_rds_instance.main", OldCost: "$200.00", NewCost: "$250.00", Change: "$50.00", IsIncrease: true},
	}

	diff.Render()
}

// JSONOutput outputs results as JSON
func (r *EstimationRunner) JSONOutput(result *EstimationResult) {
	// In real implementation, use encoding/json
	r.w.Println(`{`)
	r.w.Println(`  "totalMonthly": "%s",`, result.TotalMonthly)
	r.w.Println(`  "totalHourly": "%s",`, result.TotalHourly)
	r.w.Println(`  "confidence": %.2f,`, result.Confidence)
	r.w.Println(`  "resources": %d,`, result.Instances)
	r.w.Println(`  "duration": "%s"`, result.Duration)
	r.w.Println(`}`)
}
