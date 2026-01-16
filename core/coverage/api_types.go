// Package coverage - API response types for coverage
// These types are safe for JSON serialization and API responses.
package coverage

// CoverageResponse is the API-safe coverage response
type CoverageResponse struct {
	Coverage              CoverageSummary   `json:"coverage"`
	UnsupportedResources  []string          `json:"unsupported_resources,omitempty"`
	SymbolicResources     []SymbolicDetail  `json:"symbolic_resources,omitempty"`
	PolicyResult          *PolicyResult     `json:"policy_result,omitempty"`
}

// CoverageSummary is the cost-weighted coverage summary
type CoverageSummary struct {
	NumericCostPercent     float64 `json:"numeric_cost_percent"`
	SymbolicCostPercent    float64 `json:"symbolic_cost_percent"`
	UnsupportedCostPercent float64 `json:"unsupported_cost_percent"`
	TotalEstimatedCost     float64 `json:"total_estimated_cost"`
}

// SymbolicDetail provides detail on a symbolic resource
type SymbolicDetail struct {
	Address      string  `json:"address"`
	ResourceType string  `json:"resource_type"`
	Reason       string  `json:"reason"`
	EstimatedMax float64 `json:"estimated_max,omitempty"`
}

// PolicyResult is the strict mode policy result
type PolicyResult struct {
	Passed     bool              `json:"passed"`
	PolicyName string            `json:"policy_name"`
	Violations []PolicyViolation `json:"violations,omitempty"`
}

// PolicyViolation is an API-safe violation
type PolicyViolation struct {
	Rule    string  `json:"rule"`
	Actual  float64 `json:"actual"`
	Limit   float64 `json:"limit"`
	Message string  `json:"message"`
}

// ToAPIResponse converts a WeightedCoverageReport to API response
func (r *WeightedCoverageReport) ToAPIResponse() CoverageResponse {
	response := CoverageResponse{
		Coverage: CoverageSummary{
			NumericCostPercent:     r.NumericCostPercent,
			SymbolicCostPercent:    r.SymbolicCostPercent,
			UnsupportedCostPercent: r.UnsupportedCostPercent,
			TotalEstimatedCost:     r.TotalEstimatedCost,
		},
		UnsupportedResources: r.UnsupportedTypes,
	}

	// Add symbolic details
	for resourceType, reasons := range r.SymbolicReasons {
		for _, reason := range uniqueStrings(reasons) {
			response.SymbolicResources = append(response.SymbolicResources, SymbolicDetail{
				ResourceType: resourceType,
				Reason:       reason,
			})
		}
	}

	return response
}

// WithPolicyResult attaches policy enforcement result
func (r CoverageResponse) WithPolicyResult(policyName string, result EnforceResult) CoverageResponse {
	r.PolicyResult = &PolicyResult{
		Passed:     result.Passed,
		PolicyName: policyName,
	}

	for _, v := range result.Violations {
		r.PolicyResult.Violations = append(r.PolicyResult.Violations, PolicyViolation{
			Rule:    v.Rule,
			Actual:  v.Actual,
			Limit:   v.Limit,
			Message: v.Message,
		})
	}

	return r
}
