// Package coverage - Coverage report generation
// Computes coverage by cost contribution, not resource count.
package coverage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CoverageReport represents the cost coverage analysis
type CoverageReport struct {
	// Summary percentages
	NumericCostPercent     float64 `json:"numeric_cost_percent"`
	SymbolicCostPercent    float64 `json:"symbolic_cost_percent"`
	UnsupportedCostPercent float64 `json:"unsupported_cost_percent"`
	IndirectCostPercent    float64 `json:"indirect_cost_percent"`

	// Resource counts
	TotalResources       int `json:"total_resources"`
	NumericResources     int `json:"numeric_resources"`
	SymbolicResources    int `json:"symbolic_resources"`
	UnsupportedResources int `json:"unsupported_resources"`
	IndirectResources    int `json:"indirect_resources"`
	FreeResources        int `json:"free_resources"`

	// Details
	Details []ResourceCoverage `json:"details"`

	// Warnings
	Warnings []string `json:"warnings,omitempty"`
}

// ResourceCoverage represents coverage for a single resource
type ResourceCoverage struct {
	Address      string       `json:"address"`
	ResourceType string       `json:"resource_type"`
	Behavior     CostBehavior `json:"behavior"`
	Status       string       `json:"status"` // "numeric", "symbolic", "unsupported", "indirect", "free"
	Reason       string       `json:"reason,omitempty"`
}

// CoverageAnalyzer analyzes cost coverage
type CoverageAnalyzer struct {
	registry *Registry
}

// NewCoverageAnalyzer creates an analyzer
func NewCoverageAnalyzer(registry *Registry) *CoverageAnalyzer {
	return &CoverageAnalyzer{registry: registry}
}

// ResourceInput represents a resource to analyze
type ResourceInput struct {
	Address      string
	ResourceType string
	HasCost      bool   // Did the mapper produce a cost?
	IsSymbolic   bool   // Is the cost symbolic?
	Reason       string // Reason for symbolic/unsupported
}

// Analyze produces a coverage report
func (a *CoverageAnalyzer) Analyze(resources []ResourceInput) *CoverageReport {
	report := &CoverageReport{
		Details: make([]ResourceCoverage, 0, len(resources)),
	}

	for _, res := range resources {
		profile, exists := a.registry.Get(res.ResourceType)

		coverage := ResourceCoverage{
			Address:      res.Address,
			ResourceType: res.ResourceType,
		}

		if !exists {
			// Unknown resource type
			coverage.Behavior = CostUnsupported
			coverage.Status = "unsupported"
			coverage.Reason = "resource type not in registry"
			report.UnsupportedResources++
		} else {
			coverage.Behavior = profile.Behavior

			switch profile.Behavior {
			case CostFree:
				coverage.Status = "free"
				coverage.Reason = profile.Notes
				report.FreeResources++

			case CostIndirect:
				coverage.Status = "indirect"
				coverage.Reason = profile.Notes
				report.IndirectResources++

			case CostDirect, CostUsageBased:
				if !profile.MapperExists {
					coverage.Status = "unsupported"
					coverage.Reason = "mapper not implemented"
					report.UnsupportedResources++
				} else if res.IsSymbolic {
					coverage.Status = "symbolic"
					coverage.Reason = res.Reason
					report.SymbolicResources++
				} else if res.HasCost {
					coverage.Status = "numeric"
					report.NumericResources++
				} else {
					coverage.Status = "symbolic"
					coverage.Reason = "no cost produced"
					report.SymbolicResources++
				}

			case CostUnsupported:
				coverage.Status = "unsupported"
				coverage.Reason = profile.Notes
				report.UnsupportedResources++
			}
		}

		report.Details = append(report.Details, coverage)
	}

	report.TotalResources = len(resources)

	// Calculate percentages (weighted by estimated spend contribution)
	report.calculatePercentages(a.registry)

	// Generate warnings
	report.generateWarnings()

	return report
}

func (r *CoverageReport) calculatePercentages(registry *Registry) {
	// For now, use resource counts
	// In production, weight by estimated spend contribution
	if r.TotalResources == 0 {
		return
	}

	costBearing := r.NumericResources + r.SymbolicResources + r.UnsupportedResources
	if costBearing == 0 {
		r.NumericCostPercent = 100.0
		return
	}

	r.NumericCostPercent = float64(r.NumericResources) / float64(costBearing) * 100
	r.SymbolicCostPercent = float64(r.SymbolicResources) / float64(costBearing) * 100
	r.UnsupportedCostPercent = float64(r.UnsupportedResources) / float64(costBearing) * 100
}

func (r *CoverageReport) generateWarnings() {
	if r.UnsupportedCostPercent > 10 {
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("%.1f%% of cost-bearing resources are unsupported", r.UnsupportedCostPercent))
	}

	if r.SymbolicCostPercent > 20 {
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("%.1f%% of costs are symbolic (unknown)", r.SymbolicCostPercent))
	}
}

// ToJSON returns the report as JSON
func (r *CoverageReport) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ToCLI returns the report as CLI output
func (r *CoverageReport) ToCLI() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("╔════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                    COST COVERAGE REPORT                    ║\n")
	sb.WriteString("╠════════════════════════════════════════════════════════════╣\n")
	sb.WriteString(fmt.Sprintf("║  Numeric cost:      %5.1f%%  (%d resources)               ║\n",
		r.NumericCostPercent, r.NumericResources))
	sb.WriteString(fmt.Sprintf("║  Symbolic cost:     %5.1f%%  (%d resources)               ║\n",
		r.SymbolicCostPercent, r.SymbolicResources))
	sb.WriteString(fmt.Sprintf("║  Unsupported:       %5.1f%%  (%d resources)               ║\n",
		r.UnsupportedCostPercent, r.UnsupportedResources))
	sb.WriteString("╠════════════════════════════════════════════════════════════╣\n")
	sb.WriteString(fmt.Sprintf("║  Indirect (free):          %d resources                   ║\n",
		r.IndirectResources))
	sb.WriteString(fmt.Sprintf("║  Explicitly free:          %d resources                   ║\n",
		r.FreeResources))
	sb.WriteString(fmt.Sprintf("║  Total resources:          %d                             ║\n",
		r.TotalResources))
	sb.WriteString("╚════════════════════════════════════════════════════════════╝\n")

	if len(r.Warnings) > 0 {
		sb.WriteString("\n⚠️  WARNINGS:\n")
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("   • %s\n", w))
		}
	}

	// Details grouped by status
	sb.WriteString("\n📊 DETAILS:\n")

	// Sort by status priority
	sort.Slice(r.Details, func(i, j int) bool {
		return statusPriority(r.Details[i].Status) < statusPriority(r.Details[j].Status)
	})

	currentStatus := ""
	for _, d := range r.Details {
		if d.Status != currentStatus {
			currentStatus = d.Status
			sb.WriteString(fmt.Sprintf("\n  [%s]\n", strings.ToUpper(currentStatus)))
		}
		if d.Reason != "" {
			sb.WriteString(fmt.Sprintf("    • %s (%s)\n", d.Address, d.Reason))
		} else {
			sb.WriteString(fmt.Sprintf("    • %s\n", d.Address))
		}
	}

	return sb.String()
}

func statusPriority(status string) int {
	switch status {
	case "unsupported":
		return 0
	case "symbolic":
		return 1
	case "numeric":
		return 2
	case "indirect":
		return 3
	case "free":
		return 4
	default:
		return 5
	}
}
