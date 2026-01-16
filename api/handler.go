// Package api - HTTP handler for cost estimation
// This handler wraps the engine - it contains NO estimation logic.
// All logic is delegated to core packages.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"terraform-cost/core/confidence"
	"terraform-cost/core/graph"
	"terraform-cost/core/pricing"
)

// Handler handles estimation requests
type Handler struct {
	// Dependencies
	pricingGate *pricing.PricingGate

	// Configuration
	defaultMode EstimationMode
}

// NewHandler creates a new handler
func NewHandler() *Handler {
	return &Handler{
		pricingGate: pricing.NewPricingGate(),
		defaultMode: ModePermissive,
	}
}

// HandleEstimate handles POST /estimate
func (h *Handler) HandleEstimate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := generateRequestID()

	// Parse request
	var req EstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, requestID, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validateRequest(&req); err != nil {
		h.writeError(w, requestID, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}

	// Execute estimation
	resp, err := h.execute(ctx, requestID, &req)
	if err != nil {
		h.writeError(w, requestID, "EXECUTION_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

	// Write response
	h.writeJSON(w, resp, http.StatusOK)
}

func (h *Handler) execute(ctx context.Context, requestID string, req *EstimateRequest) (*EstimateResponse, error) {
	resp := &EstimateResponse{
		RequestID:   requestID,
		Timestamp:   time.Now().UTC(),
		Status:      "success",
		Resources:   []ResourceCost{},
		Unknowns:    []UnknownCost{},
		Assumptions: []Assumption{},
	}

	isStrict := req.Mode == ModeStrict

	// Step 1: Build canonical dependency graph
	depGraph := graph.NewCanonicalDependencyGraph()
	// ... parsing and building would go here
	depGraph.Seal()
	depGraph.MustBeClosed() // INVARIANT: graph must be closed

	// Step 2: Create expansion guard
	expansionGuard := graph.NewExpansionGuard(isStrict)

	// Step 3: Create asset graph
	assetGraph, err := graph.NewEnforcedAssetGraph(depGraph)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset graph: %w", err)
	}

	// Step 4: Create cost graph (ONLY from asset graph with dep graph)
	costGraph, err := graph.NewEnforcedCostGraph(assetGraph)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost graph: %w", err)
	}

	// Step 5: Run invariant checks
	checker := graph.NewInvariantChecker(isStrict)
	if err := checker.RunFullCheck(costGraph); err != nil {
		return nil, fmt.Errorf("invariant check failed: %w", err)
	}

	// Step 6: Collect results
	confidences := []float64{}
	for _, unit := range costGraph.AllCostUnits() {
		confidences = append(confidences, unit.Confidence)

		if unit.IsSymbolic {
			// Add to unknowns
			resp.Unknowns = append(resp.Unknowns, UnknownCost{
				Address:     unit.AssetID,
				Reason:      unit.SymbolicInfo.Reason,
				IsUnbounded: unit.SymbolicInfo.IsUnbounded,
			})
		} else {
			// Add to resources
			resp.Resources = append(resp.Resources, ResourceCost{
				Address:        unit.AssetID,
				MonthlyCost:    &CostValue{Amount: unit.MonthlyCost.String(), Currency: "USD"},
				Confidence:     unit.Confidence,
				DependencyPath: unit.DependencyPath,
			})
		}
	}

	// Step 7: Aggregate confidence (PESSIMISTIC)
	resp.Confidence = confidence.AggregateConfidence(confidences)
	resp.ConfidenceLevel = confidence.ConfidenceLevel(resp.Confidence)

	// Step 8: Add blocked expansions as unknowns
	for _, blocked := range expansionGuard.GetBlocked() {
		resp.Unknowns = append(resp.Unknowns, UnknownCost{
			Address:     blocked.Address,
			Reason:      blocked.Reason,
			IsUnbounded: true,
		})
	}

	// Set status based on unknowns
	if len(resp.Unknowns) > 0 {
		resp.Status = "partial"
		resp.Message = fmt.Sprintf("%d resources have unknown cardinality", len(resp.Unknowns))
	}

	return resp, nil
}

func validateRequest(req *EstimateRequest) error {
	if req.Source.Type == "" {
		return fmt.Errorf("source.type is required")
	}
	switch req.Source.Type {
	case "directory":
		if req.Source.Path == "" {
			return fmt.Errorf("source.path is required for directory type")
		}
	case "git":
		if req.Source.URL == "" {
			return fmt.Errorf("source.url is required for git type")
		}
	case "inline":
		if len(req.Source.InlineHCL) == 0 {
			return fmt.Errorf("source.inline_hcl is required for inline type")
		}
	default:
		return fmt.Errorf("invalid source.type: %s", req.Source.Type)
	}
	return nil
}

func (h *Handler) writeError(w http.ResponseWriter, requestID, code, message string, status int) {
	resp := &EstimateResponse{
		RequestID: requestID,
		Timestamp: time.Now().UTC(),
		Status:    "error",
		Message:   message,
		Errors: []ErrorDetail{
			{Code: code, Message: message},
		},
	}
	h.writeJSON(w, resp, status)
}

func (h *Handler) writeJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func generateRequestID() string {
	return fmt.Sprintf("est-%d", time.Now().UnixNano())
}

// RegisterRoutes registers API routes
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("POST /estimate", h.HandleEstimate)
	mux.HandleFunc("GET /health", handleHealth)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
