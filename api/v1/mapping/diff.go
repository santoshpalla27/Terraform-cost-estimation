// Package mapping - Diff mapping
package mapping

import (
	"fmt"
	"time"

	"terraform-cost/api/v1/types"
)

// EngineDiffResult is an interface for engine diff results
type EngineDiffResult interface {
	GetBaseTotal() string
	GetHeadTotal() string
	GetBaseConfidence() float64
	GetHeadConfidence() float64
	GetBaseResourceCount() int
	GetHeadResourceCount() int
	GetBaseSymbolicCount() int
	GetHeadSymbolicCount() int
	GetChanges() []EngineChange
	GetExplanations() []EngineExplanation
}

// EngineChange is an interface for a single change
type EngineChange interface {
	GetType() string
	GetAddress() string
	GetResourceType() string
	GetCostBefore() string
	GetCostAfter() string
	GetDependencyPath() []string
}

// EngineExplanation is an interface for an explanation
type EngineExplanation interface {
	GetAddress() string
	GetExplanation() string
	GetCausalChain() []string
}

// MapDiffResponse maps engine diff result to API response
func MapDiffResponse(
	result EngineDiffResult,
	baseRef, headRef string,
	inputHash string,
	mode string,
	startTime time.Time,
	config MapperConfig,
) types.DiffResponse {
	durationMs := time.Since(startTime).Milliseconds()

	resp := types.DiffResponse{
		Metadata: types.MetadataDTO{
			InputHash:       inputHash,
			EngineVersion:   config.EngineVersion,
			PricingSnapshot: config.PricingSnapshot,
			Mode:            mode,
			Timestamp:       time.Now().UTC(),
			DurationMs:      durationMs,
			APIVersion:      config.APIVersion,
		},
		Base: types.DiffSideDTO{
			Ref:              baseRef,
			TotalMonthlyCost: result.GetBaseTotal(),
			Confidence:       result.GetBaseConfidence(),
			ResourceCount:    result.GetBaseResourceCount(),
			SymbolicCount:    result.GetBaseSymbolicCount(),
		},
		Head: types.DiffSideDTO{
			Ref:              headRef,
			TotalMonthlyCost: result.GetHeadTotal(),
			Confidence:       result.GetHeadConfidence(),
			ResourceCount:    result.GetHeadResourceCount(),
			SymbolicCount:    result.GetHeadSymbolicCount(),
		},
		Delta:        computeDelta(result),
		Changes:      mapChanges(result.GetChanges()),
		Explanations: mapExplanations(result.GetExplanations()),
	}

	return resp
}

func computeDelta(result EngineDiffResult) types.DeltaDTO {
	var baseVal, headVal float64
	fmt.Sscanf(result.GetBaseTotal(), "%f", &baseVal)
	fmt.Sscanf(result.GetHeadTotal(), "%f", &headVal)

	delta := headVal - baseVal
	var deltaStr string
	if delta >= 0 {
		deltaStr = fmt.Sprintf("+%.2f", delta)
	} else {
		deltaStr = fmt.Sprintf("%.2f", delta)
	}

	var percentStr string
	if baseVal > 0 {
		percent := (delta / baseVal) * 100
		if percent >= 0 {
			percentStr = fmt.Sprintf("+%.1f%%", percent)
		} else {
			percentStr = fmt.Sprintf("%.1f%%", percent)
		}
	}

	changes := result.GetChanges()
	var added, removed, changed int
	for _, c := range changes {
		switch c.GetType() {
		case "added":
			added++
		case "removed":
			removed++
		case "changed":
			changed++
		}
	}

	return types.DeltaDTO{
		MonthlyCostDelta: deltaStr,
		PercentChange:    percentStr,
		ConfidenceDelta:  result.GetHeadConfidence() - result.GetBaseConfidence(),
		AddedCount:       added,
		RemovedCount:     removed,
		ChangedCount:     changed,
	}
}

func mapChanges(changes []EngineChange) []types.ChangeDTO {
	result := make([]types.ChangeDTO, len(changes))
	for i, c := range changes {
		var delta string
		var before, after float64
		fmt.Sscanf(c.GetCostBefore(), "%f", &before)
		fmt.Sscanf(c.GetCostAfter(), "%f", &after)
		d := after - before
		if d >= 0 {
			delta = fmt.Sprintf("+%.2f", d)
		} else {
			delta = fmt.Sprintf("%.2f", d)
		}

		result[i] = types.ChangeDTO{
			Type:           c.GetType(),
			Address:        c.GetAddress(),
			ResourceType:   c.GetResourceType(),
			CostBefore:     c.GetCostBefore(),
			CostAfter:      c.GetCostAfter(),
			CostDelta:      delta,
			DependencyPath: c.GetDependencyPath(),
		}
	}
	return result
}

func mapExplanations(explanations []EngineExplanation) []types.ExplanationDTO {
	result := make([]types.ExplanationDTO, len(explanations))
	for i, e := range explanations {
		result[i] = types.ExplanationDTO{
			Address:     e.GetAddress(),
			Explanation: e.GetExplanation(),
			CausalChain: e.GetCausalChain(),
		}
	}
	return result
}
