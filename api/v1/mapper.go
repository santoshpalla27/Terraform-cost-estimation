// Package v1 - Mapping layer from engine to API types
// This is the ONLY place where engine types are converted to API types
package v1

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Mapper converts engine results to API responses
type Mapper struct {
	engineVersion   string
	pricingSnapshot string
}

// NewMapper creates a mapper
func NewMapper(engineVersion, pricingSnapshot string) *Mapper {
	return &Mapper{
		engineVersion:   engineVersion,
		pricingSnapshot: pricingSnapshot,
	}
}

// NormalizedInput is the normalized form of an estimate request
type NormalizedInput struct {
	// ResolvedPath is the absolute path to Terraform files
	ResolvedPath string

	// InputHash is the SHA256 of the normalized request
	InputHash string

	// Mode
	Mode Mode

	// UsageProfile
	UsageProfile string

	// Options
	Options EstimateOptions

	// ResolvedAt is when normalization occurred
	ResolvedAt time.Time
}

// NormalizeInput normalizes an estimate request
func NormalizeInput(req *EstimateRequest) (*NormalizedInput, error) {
	// Resolve source to path
	resolvedPath, err := resolveSource(&req.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source: %w", err)
	}

	// Compute input hash
	hashData, _ := json.Marshal(struct {
		Path    string
		Mode    Mode
		Profile string
	}{
		Path:    resolvedPath,
		Mode:    req.Mode,
		Profile: req.UsageProfile,
	})
	hash := sha256.Sum256(hashData)

	return &NormalizedInput{
		ResolvedPath: resolvedPath,
		InputHash:    hex.EncodeToString(hash[:]),
		Mode:         req.Mode,
		UsageProfile: req.UsageProfile,
		Options:      req.Options,
		ResolvedAt:   time.Now().UTC(),
	}, nil
}

func resolveSource(src *SourceConfig) (string, error) {
	switch src.Type {
	case "local":
		return src.Path, nil
	case "git":
		// Would clone/checkout here
		return fmt.Sprintf("git:%s@%s:%s", src.Repo, src.Ref, src.Path), nil
	case "upload":
		return fmt.Sprintf("upload:%s", src.UploadID), nil
	default:
		return "", fmt.Errorf("unknown source type: %s", src.Type)
	}
}

// EngineResult is a placeholder for engine output
// In real implementation, this would import from core/engine
type EngineResult struct {
	TotalMonthlyCost string
	TotalHourlyCost  string
	Currency         string
	Confidence       float64
	ConfidenceReason string
	Resources        []EngineResourceCost
	SymbolicCosts    []EngineSymbolicCost
	Warnings         []string
}

// EngineResourceCost is a placeholder for engine resource cost
type EngineResourceCost struct {
	Address        string
	ResourceType   string
	ProviderAlias  string
	MonthlyCost    string
	HourlyCost     string
	Confidence     float64
	IsSymbolic     bool
	DependencyPath []string
}

// EngineSymbolicCost is a placeholder for engine symbolic cost
type EngineSymbolicCost struct {
	Address     string
	Reason      string
	Expression  string
	IsUnbounded bool
}

// MapEstimateResponse maps engine result to API response
func (m *Mapper) MapEstimateResponse(
	input *NormalizedInput,
	result *EngineResult,
	durationMs int64,
) *EstimateResponse {
	resp := &EstimateResponse{
		Metadata: ResponseMetadata{
			InputHash:       input.InputHash,
			EngineVersion:   m.engineVersion,
			PricingSnapshot: m.pricingSnapshot,
			Mode:            string(input.Mode),
			Timestamp:       time.Now().UTC(),
			DurationMs:      durationMs,
		},
		Summary: CostSummary{
			TotalMonthlyCost: result.TotalMonthlyCost,
			TotalHourlyCost:  result.TotalHourlyCost,
			Currency:         result.Currency,
			Confidence:       result.Confidence,
			ConfidenceLevel:  getConfidenceLevel(result.Confidence),
			ConfidenceReason: result.ConfidenceReason,
			ResourceCount:    len(result.Resources),
			SymbolicCount:    len(result.SymbolicCosts),
		},
	}

	// Map cost graph if requested
	if input.Options.IncludeDependencyGraph {
		resp.CostGraph = m.mapCostGraph(result.Resources)
	}

	// Map symbolic costs
	for _, sc := range result.SymbolicCosts {
		resp.SymbolicCosts = append(resp.SymbolicCosts, SymbolicCost{
			Address:     sc.Address,
			Reason:      sc.Reason,
			Expression:  sc.Expression,
			IsUnbounded: sc.IsUnbounded,
		})
	}

	// Map warnings
	for _, w := range result.Warnings {
		resp.Warnings = append(resp.Warnings, Warning{
			Code:    "WARN",
			Message: w,
		})
	}

	return resp
}

func (m *Mapper) mapCostGraph(resources []EngineResourceCost) *CostGraph {
	// Group by module
	moduleMap := make(map[string][]EngineResourceCost)
	for _, r := range resources {
		module := extractModule(r.Address)
		moduleMap[module] = append(moduleMap[module], r)
	}

	graph := &CostGraph{Modules: []ModuleCost{}}
	for modAddr, modResources := range moduleMap {
		modCost := ModuleCost{
			Address:   modAddr,
			Resources: []ResourceCost{},
		}

		totalCost := 0.0
		minConf := 1.0
		for _, r := range modResources {
			var cost float64
			fmt.Sscanf(r.MonthlyCost, "%f", &cost)
			totalCost += cost
			if r.Confidence < minConf {
				minConf = r.Confidence
			}

			rc := ResourceCost{
				Address:       r.Address,
				ResourceType:  r.ResourceType,
				ProviderAlias: r.ProviderAlias,
				MonthlyCost:   r.MonthlyCost,
				HourlyCost:    r.HourlyCost,
				Confidence:    r.Confidence,
				IsSymbolic:    r.IsSymbolic,
			}

			if len(r.DependencyPath) > 0 {
				rc.Lineage = &CostLineage{
					DependencyPath: r.DependencyPath,
					Explanation:    fmt.Sprintf("This cost exists because of: %v", r.DependencyPath),
				}
			}

			modCost.Resources = append(modCost.Resources, rc)
		}

		modCost.MonthlyCost = fmt.Sprintf("%.2f", totalCost)
		modCost.Confidence = minConf
		graph.Modules = append(graph.Modules, modCost)
	}

	return graph
}

func extractModule(address string) string {
	// Simple extraction - would be more sophisticated in real impl
	parts := splitAddress(address)
	if len(parts) > 2 {
		return parts[0] + "." + parts[1]
	}
	return "root"
}

func splitAddress(addr string) []string {
	var parts []string
	current := ""
	for _, c := range addr {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func getConfidenceLevel(confidence float64) string {
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
