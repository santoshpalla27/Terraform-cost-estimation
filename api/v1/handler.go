// Package v1 - Versioned API handler
// Routes: POST /api/v1/estimate, POST /api/v1/diff
package v1

import (
	"encoding/json"
	"net/http"
	"time"
)

// Handler handles v1 API requests
type Handler struct {
	mapper *Mapper
}

// NewHandler creates a v1 handler
func NewHandler(engineVersion, pricingSnapshot string) *Handler {
	return &Handler{
		mapper: NewMapper(engineVersion, pricingSnapshot),
	}
}

// RegisterRoutes registers v1 routes on the mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/estimate", h.handleEstimate)
	mux.HandleFunc("POST /api/v1/diff", h.handleDiff)
	mux.HandleFunc("GET /api/v1/health", h.handleHealth)
}

// handleEstimate handles POST /api/v1/estimate
func (h *Handler) handleEstimate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Parse request
	var req EstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "INVALID_JSON", err.Error(), http.StatusBadRequest)
		return
	}

	// Normalize input
	input, err := NormalizeInput(&req)
	if err != nil {
		h.writeError(w, "NORMALIZATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}

	// Execute engine (placeholder - would call actual engine)
	result := h.executeEngine(input)

	// Map to response
	resp := h.mapper.MapEstimateResponse(input, result, time.Since(start).Milliseconds())

	h.writeJSON(w, resp, http.StatusOK)
}

// handleDiff handles POST /api/v1/diff
func (h *Handler) handleDiff(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req DiffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "INVALID_JSON", err.Error(), http.StatusBadRequest)
		return
	}

	// Execute base and head
	baseInput := &NormalizedInput{
		ResolvedPath: "git:@" + req.Base.Ref,
		Mode:         req.Mode,
	}
	headInput := &NormalizedInput{
		ResolvedPath: "git:@" + req.Head.Ref,
		Mode:         req.Mode,
	}

	baseResult := h.executeEngine(baseInput)
	headResult := h.executeEngine(headInput)

	// Build diff response
	resp := h.buildDiffResponse(req, baseResult, headResult, time.Since(start).Milliseconds())

	h.writeJSON(w, resp, http.StatusOK)
}

// handleHealth handles GET /api/v1/health
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, map[string]interface{}{
		"status":      "healthy",
		"api_version": "v1",
		"engine":      h.mapper.engineVersion,
		"time":        time.Now().UTC().Format(time.RFC3339),
	}, http.StatusOK)
}

// executeEngine is a placeholder for actual engine execution
func (h *Handler) executeEngine(input *NormalizedInput) *EngineResult {
	// This would call the actual engine
	// For now, return mock data
	return &EngineResult{
		TotalMonthlyCost: "1234.56",
		TotalHourlyCost:  "1.69",
		Currency:         "USD",
		Confidence:       0.72,
		ConfidenceReason: "Unknown usage for aws_lambda_function.api",
		Resources: []EngineResourceCost{
			{
				Address:        "module.compute.aws_instance.web",
				ResourceType:   "aws_instance",
				ProviderAlias:  "aws.prod",
				MonthlyCost:    "730.00",
				Confidence:     0.95,
				DependencyPath: []string{"module.compute", "aws_instance.web"},
			},
			{
				Address:        "module.database.aws_db_instance.main",
				ResourceType:   "aws_db_instance",
				ProviderAlias:  "aws.prod",
				MonthlyCost:    "504.56",
				Confidence:     0.72,
				DependencyPath: []string{"module.database", "aws_db_instance.main"},
			},
		},
		SymbolicCosts: []EngineSymbolicCost{
			{
				Address:     "module.workers.aws_instance.worker",
				Reason:      "for_each derived from module output",
				Expression:  "for_each = module.config.worker_names",
				IsUnbounded: true,
			},
		},
		Warnings: []string{
			"Unknown usage for aws_lambda_function.api - using default",
		},
	}
}

func (h *Handler) buildDiffResponse(req DiffRequest, base, head *EngineResult, durationMs int64) *DiffResponse {
	return &DiffResponse{
		Metadata: ResponseMetadata{
			InputHash:       "diff-" + req.Base.Ref + "-" + req.Head.Ref,
			EngineVersion:   h.mapper.engineVersion,
			PricingSnapshot: h.mapper.pricingSnapshot,
			Mode:            string(req.Mode),
			Timestamp:       time.Now().UTC(),
			DurationMs:      durationMs,
		},
		Base: DiffSummary{
			Ref:              req.Base.Ref,
			TotalMonthlyCost: base.TotalMonthlyCost,
			Confidence:       base.Confidence,
			ResourceCount:    len(base.Resources),
			SymbolicCount:    len(base.SymbolicCosts),
		},
		Head: DiffSummary{
			Ref:              req.Head.Ref,
			TotalMonthlyCost: head.TotalMonthlyCost,
			Confidence:       head.Confidence,
			ResourceCount:    len(head.Resources),
			SymbolicCount:    len(head.SymbolicCosts),
		},
		Delta: DiffDelta{
			MonthlyCostDelta: "+$0.00",
			ConfidenceDelta:  0,
			AddedCount:       0,
			RemovedCount:     0,
			ChangedCount:     0,
		},
		Changes: []DiffChange{},
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", "v1")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) writeError(w http.ResponseWriter, code, message string, status int) {
	h.writeJSON(w, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	}, status)
}
