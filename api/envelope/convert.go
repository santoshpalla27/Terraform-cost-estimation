// Package envelope - API request to envelope conversion
package envelope

// FromRequest converts API request types to RawInput
// This is the bridge between API DTOs and normalization

import (
	"terraform-cost/api/v1/types"
)

// FromEstimateRequest converts EstimateRequest to RawInput
func FromEstimateRequest(req *types.EstimateRequest) RawInput {
	return RawInput{
		SourceType:   req.Source.Type,
		Repo:         req.Source.Repo,
		Ref:          req.Source.Ref,
		Path:         req.Source.Path,
		Mode:         req.Mode,
		UsageProfile: req.UsageProfile,
		Options: RawOptions{
			IncludeDependencyGraph: req.Options.IncludeDependencyGraph,
			IncludeCostLineage:     req.Options.IncludeCostLineage,
			IncludeComponents:      req.Options.IncludeComponents,
		},
	}
}

// FromDiffRequest converts DiffRequest to two RawInputs
func FromDiffRequest(req *types.DiffRequest) (base, head RawInput) {
	base = RawInput{
		SourceType: "git",
		Ref:        req.Base.Ref,
		Path:       req.Base.Path,
		Mode:       req.Mode,
	}
	head = RawInput{
		SourceType: "git",
		Ref:        req.Head.Ref,
		Path:       req.Head.Path,
		Mode:       req.Mode,
	}
	return
}
