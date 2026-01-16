// Package mapping - Explicit mapping from engine to API DTOs
// This is the ONLY place where engine types touch API types.
// Rules:
// - One-way mapping only (engine → API)
// - No mutation of engine results
// - No inference or aggregation
// - No business logic
package mapping

import (
	"fmt"
	"time"

	"terraform-cost/api/v1/types"
)

// EngineEstimateResult is an interface for engine results
// We use an interface to avoid importing engine package directly
type EngineEstimateResult interface {
	GetTotalMonthlyCost() string
	GetTotalHourlyCost() string
	GetCurrency() string
	GetConfidence() float64
	GetConfidenceReason() string
	GetResources() []EngineResource
	GetSymbolicCosts() []EngineSymbolicCost
	GetWarnings() []string
}

// EngineResource is an interface for engine resource
type EngineResource interface {
	GetAddress() string
	GetType() string
	GetProviderAlias() string
	GetMonthlyCost() string
	GetHourlyCost() string
	GetConfidence() float64
	IsSymbolic() bool
	GetDependencyPath() []string
}

// EngineSymbolicCost is an interface for symbolic costs
type EngineSymbolicCost interface {
	GetAddress() string
	GetReason() string
	GetExpression() string
	IsUnbounded() bool
}

// MapperConfig configures the mapping
type MapperConfig struct {
	EngineVersion   string
	PricingSnapshot string
	APIVersion      string
}

// MapEstimateResponse maps engine result to API response
// This is a PURE function - no side effects, no state
func MapEstimateResponse(
	result EngineEstimateResult,
	inputHash string,
	mode string,
	startTime time.Time,
	config MapperConfig,
) types.EstimateResponse {
	durationMs := time.Since(startTime).Milliseconds()

	resp := types.EstimateResponse{
		Metadata: types.MetadataDTO{
			InputHash:       inputHash,
			EngineVersion:   config.EngineVersion,
			PricingSnapshot: config.PricingSnapshot,
			Mode:            mode,
			Timestamp:       time.Now().UTC(),
			DurationMs:      durationMs,
			APIVersion:      config.APIVersion,
		},
		Summary: types.SummaryDTO{
			TotalMonthlyCost: result.GetTotalMonthlyCost(),
			TotalHourlyCost:  result.GetTotalHourlyCost(),
			Currency:         result.GetCurrency(),
			Confidence:       result.GetConfidence(),
			ConfidenceLevel:  mapConfidenceLevel(result.GetConfidence()),
			ConfidenceReason: result.GetConfidenceReason(),
			ResourceCount:    len(result.GetResources()),
			SymbolicCount:    len(result.GetSymbolicCosts()),
		},
		Costs:    mapCosts(result.GetResources()),
		Symbolic: mapSymbolicCosts(result.GetSymbolicCosts()),
		Warnings: result.GetWarnings(),
	}

	return resp
}

func mapCosts(resources []EngineResource) []types.CostNodeDTO {
	costs := make([]types.CostNodeDTO, len(resources))
	for i, r := range resources {
		costs[i] = types.CostNodeDTO{
			Address:       r.GetAddress(),
			Type:          r.GetType(),
			ProviderAlias: r.GetProviderAlias(),
			MonthlyCost:   r.GetMonthlyCost(),
			HourlyCost:    r.GetHourlyCost(),
			Confidence:    r.GetConfidence(),
			IsSymbolic:    r.IsSymbolic(),
		}

		if len(r.GetDependencyPath()) > 0 {
			costs[i].Lineage = &types.LineageDTO{
				DependencyPath: r.GetDependencyPath(),
				Explanation:    fmt.Sprintf("Cost derived from dependency: %v", r.GetDependencyPath()),
			}
		}
	}
	return costs
}

func mapSymbolicCosts(symbolics []EngineSymbolicCost) []types.SymbolicCostDTO {
	costs := make([]types.SymbolicCostDTO, len(symbolics))
	for i, s := range symbolics {
		costs[i] = types.SymbolicCostDTO{
			Address:     s.GetAddress(),
			Reason:      s.GetReason(),
			Expression:  s.GetExpression(),
			IsUnbounded: s.IsUnbounded(),
		}
	}
	return costs
}

func mapConfidenceLevel(confidence float64) string {
	switch {
	case confidence >= 0.9:
		return "high"
	case confidence >= 0.7:
		return "medium"
	case confidence >= 0.5:
		return "low"
	default:
		return "unknown"
	}
}
