// Package coverage - Cost-weighted coverage accounting
// Coverage is weighted by spend, not by resource count.
// This enables enterprise-grade accuracy assessment and strict mode enforcement.
package coverage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SpendCoverageType classifies coverage of a cost unit
type SpendCoverageType int

const (
	// SpendNumeric - fully priced with concrete dollar amount
	SpendNumeric SpendCoverageType = iota

	// SpendSymbolic - known resource but cost cannot be computed
	SpendSymbolic

	// SpendIndirect - zero-cost resource (VPC, IAM, etc.)
	SpendIndirect

	// SpendUnsupported - resource type not modeled
	SpendUnsupported
)

// String returns string representation
func (c SpendCoverageType) String() string {
	switch c {
	case SpendNumeric:
		return "numeric"
	case SpendSymbolic:
		return "symbolic"
	case SpendIndirect:
		return "indirect"
	case SpendUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

// CostUnitCoverage represents coverage info for a cost unit
type CostUnitCoverage struct {
	// Identity
	ResourceAddress string
	ResourceType    string
	ComponentName   string

	// Coverage classification
	CoverageType SpendCoverageType

	// Cost amounts
	NumericCost    float64 // Actual computed cost (if numeric)
	EstimatedBound float64 // Upper bound estimate (if symbolic)

	// Explanation
	Reason string
}

// WeightedCoverageReport is the cost-weighted coverage analysis
type WeightedCoverageReport struct {
	// Cost-weighted percentages (based on spend, not resource count)
	NumericCostPercent     float64 `json:"numeric_cost_percent"`
	SymbolicCostPercent    float64 `json:"symbolic_cost_percent"`
	IndirectCostPercent    float64 `json:"indirect_cost_percent"`
	UnsupportedCostPercent float64 `json:"unsupported_cost_percent"`

	// Absolute amounts
	TotalNumericCost    float64 `json:"total_numeric_cost"`
	TotalSymbolicBound  float64 `json:"total_symbolic_bound"`
	TotalUnsupportedEst float64 `json:"total_unsupported_estimate"`
	TotalEstimatedCost  float64 `json:"total_estimated_cost"`

	// Resource counts (secondary metric)
	NumericResourceCount     int `json:"numeric_resource_count"`
	SymbolicResourceCount    int `json:"symbolic_resource_count"`
	IndirectResourceCount    int `json:"indirect_resource_count"`
	UnsupportedResourceCount int `json:"unsupported_resource_count"`
	TotalResourceCount       int `json:"total_resource_count"`

	// Details
	UnsupportedTypes []string            `json:"unsupported_types,omitempty"`
	SymbolicReasons  map[string][]string `json:"symbolic_reasons,omitempty"`
	Details          []CostUnitCoverage  `json:"-"` // Full details, not in JSON by default

	// Warnings
	Warnings []string `json:"warnings,omitempty"`
}

// CoverageAggregator computes cost-weighted coverage
type CoverageAggregator struct {
	units []CostUnitCoverage
}

// NewCoverageAggregator creates a coverage aggregator
func NewCoverageAggregator() *CoverageAggregator {
	return &CoverageAggregator{
		units: make([]CostUnitCoverage, 0),
	}
}

// AddNumeric adds a numeric cost unit
func (a *CoverageAggregator) AddNumeric(address, resourceType, component string, cost float64) {
	a.units = append(a.units, CostUnitCoverage{
		ResourceAddress: address,
		ResourceType:    resourceType,
		ComponentName:   component,
		CoverageType:    SpendNumeric,
		NumericCost:     cost,
	})
}

// AddSymbolic adds a symbolic cost unit with estimated bound
func (a *CoverageAggregator) AddSymbolic(address, resourceType, component string, estimatedBound float64, reason string) {
	a.units = append(a.units, CostUnitCoverage{
		ResourceAddress: address,
		ResourceType:    resourceType,
		ComponentName:   component,
		CoverageType:    SpendSymbolic,
		EstimatedBound:  estimatedBound,
		Reason:          reason,
	})
}

// AddUnsupported adds an unsupported resource
// AddIndirect adds a zero-cost indirect resource (VPC, IAM, etc.)
func (a *CoverageAggregator) AddIndirect(address, resourceType string, reason string) {
	a.units = append(a.units, CostUnitCoverage{
		ResourceAddress: address,
		ResourceType:    resourceType,
		CoverageType:    SpendIndirect,
		NumericCost:     0,
		Reason:          reason,
	})
}

// AddUnsupported adds an unsupported resource
func (a *CoverageAggregator) AddUnsupported(address, resourceType string, estimatedCost float64, reason string) {
	a.units = append(a.units, CostUnitCoverage{
		ResourceAddress: address,
		ResourceType:    resourceType,
		CoverageType:    SpendUnsupported,
		EstimatedBound:  estimatedCost,
		Reason:          reason,
	})
}

// Compute generates the weighted coverage report
func (a *CoverageAggregator) Compute() *WeightedCoverageReport {
	report := &WeightedCoverageReport{
		SymbolicReasons: make(map[string][]string),
		Details:         a.units,
	}

	seenUnsupported := make(map[string]bool)
	seenResources := make(map[string]SpendCoverageType)

	for _, unit := range a.units {
		switch unit.CoverageType {
		case SpendNumeric:
			report.TotalNumericCost += unit.NumericCost

		case SpendSymbolic:
			report.TotalSymbolicBound += unit.EstimatedBound
			if unit.Reason != "" {
				report.SymbolicReasons[unit.ResourceType] = append(
					report.SymbolicReasons[unit.ResourceType], unit.Reason)
			}

		case SpendIndirect:
			// Indirect resources have zero cost but are tracked
			// They don't contribute to cost percentages

		case SpendUnsupported:
			report.TotalUnsupportedEst += unit.EstimatedBound
			if !seenUnsupported[unit.ResourceType] {
				seenUnsupported[unit.ResourceType] = true
				report.UnsupportedTypes = append(report.UnsupportedTypes, unit.ResourceType)
			}
		}

		// Track unique resources by address
		key := unit.ResourceAddress
		if existing, ok := seenResources[key]; ok {
			// Upgrade: unsupported beats symbolic beats indirect beats numeric
			if unit.CoverageType > existing {
				seenResources[key] = unit.CoverageType
			}
		} else {
			seenResources[key] = unit.CoverageType
		}
	}

	// Count resources by type
	for _, coverType := range seenResources {
		report.TotalResourceCount++
		switch coverType {
		case SpendNumeric:
			report.NumericResourceCount++
		case SpendSymbolic:
			report.SymbolicResourceCount++
		case SpendIndirect:
			report.IndirectResourceCount++
		case SpendUnsupported:
			report.UnsupportedResourceCount++
		}
	}

	// Calculate total estimated cost
	report.TotalEstimatedCost = report.TotalNumericCost + report.TotalSymbolicBound + report.TotalUnsupportedEst

	// Calculate percentages (cost-weighted)
	if report.TotalEstimatedCost > 0 {
		report.NumericCostPercent = (report.TotalNumericCost / report.TotalEstimatedCost) * 100
		report.SymbolicCostPercent = (report.TotalSymbolicBound / report.TotalEstimatedCost) * 100
		report.UnsupportedCostPercent = (report.TotalUnsupportedEst / report.TotalEstimatedCost) * 100
	} else if report.TotalResourceCount > 0 {
		// Fall back to count-based if no costs
		report.NumericCostPercent = float64(report.NumericResourceCount) / float64(report.TotalResourceCount) * 100
		report.SymbolicCostPercent = float64(report.SymbolicResourceCount) / float64(report.TotalResourceCount) * 100
		report.UnsupportedCostPercent = float64(report.UnsupportedResourceCount) / float64(report.TotalResourceCount) * 100
	}

	// Sort unsupported types
	sort.Strings(report.UnsupportedTypes)

	// Generate warnings
	report.generateWarnings()

	return report
}

func (r *WeightedCoverageReport) generateWarnings() {
	if r.UnsupportedCostPercent > 10 {
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("%.1f%% of estimated cost is from unsupported resources", r.UnsupportedCostPercent))
	}

	if r.SymbolicCostPercent > 20 {
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("%.1f%% of estimated cost is symbolic (uncertain)", r.SymbolicCostPercent))
	}

	if len(r.UnsupportedTypes) > 5 {
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("%d unsupported resource types detected", len(r.UnsupportedTypes)))
	}
}

// ToJSON returns the report as JSON
func (r *WeightedCoverageReport) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ToCLI returns the report formatted for CLI
func (r *WeightedCoverageReport) ToCLI() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("╔══════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║              COST COVERAGE REPORT (Spend-Weighted)               ║\n")
	sb.WriteString("╠══════════════════════════════════════════════════════════════════╣\n")
	sb.WriteString(fmt.Sprintf("║  Numeric cost:      %6.1f%%   ($%.2f)                          \n",
		r.NumericCostPercent, r.TotalNumericCost))
	sb.WriteString(fmt.Sprintf("║  Symbolic cost:     %6.1f%%   ($%.2f estimated)                \n",
		r.SymbolicCostPercent, r.TotalSymbolicBound))
	sb.WriteString(fmt.Sprintf("║  Unsupported:       %6.1f%%   ($%.2f estimated)                \n",
		r.UnsupportedCostPercent, r.TotalUnsupportedEst))
	sb.WriteString("╠══════════════════════════════════════════════════════════════════╣\n")
	sb.WriteString(fmt.Sprintf("║  Total estimated:             $%.2f/month                       \n",
		r.TotalEstimatedCost))
	sb.WriteString(fmt.Sprintf("║  Resources: %d numeric, %d symbolic, %d unsupported             \n",
		r.NumericResourceCount, r.SymbolicResourceCount, r.UnsupportedResourceCount))
	sb.WriteString("╚══════════════════════════════════════════════════════════════════╝\n")

	if len(r.Warnings) > 0 {
		sb.WriteString("\n⚠️  WARNINGS:\n")
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("   • %s\n", w))
		}
	}

	if len(r.UnsupportedTypes) > 0 {
		sb.WriteString("\n❌ UNSUPPORTED RESOURCES:\n")
		for _, t := range r.UnsupportedTypes {
			sb.WriteString(fmt.Sprintf("   • %s\n", t))
		}
	}

	if len(r.SymbolicReasons) > 0 {
		sb.WriteString("\n⚠️  SYMBOLIC COSTS:\n")
		for resourceType, reasons := range r.SymbolicReasons {
			// Deduplicate reasons
			uniqueReasons := uniqueStrings(reasons)
			sb.WriteString(fmt.Sprintf("   • %s: %s\n", resourceType, strings.Join(uniqueReasons, "; ")))
		}
	}

	return sb.String()
}

func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range input {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
