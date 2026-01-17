// Package api - Thin, deterministic API layer
// The API is ONLY responsible for: input ingestion, engine orchestration, output serialization.
// The API NEVER performs cost logic.
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"terraform-cost/db"
)

// Server is the API server
type Server struct {
	handler *Handler
	mux     *http.ServeMux
	version string
	store   db.PricingStore
}

// NewServer creates a new API server (without database)
func NewServer(version string) *Server {
	return NewServerWithStore(version, nil)
}

// NewServerWithStore creates a new API server with database connection
func NewServerWithStore(version string, store db.PricingStore) *Server {
	handler := NewHandler()
	mux := http.NewServeMux()

	s := &Server{
		handler: handler,
		mux:     mux,
		version: version,
		store:   store,
	}

	s.registerRoutes()
	return s
}

// registerRoutes registers all API routes
func (s *Server) registerRoutes() {
	// Core endpoints
	s.mux.HandleFunc("POST /estimate", s.handleEstimate)
	s.mux.HandleFunc("POST /diff", s.handleDiff)
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// Supporting endpoints
	s.mux.HandleFunc("GET /version", s.handleVersion)
	s.mux.HandleFunc("GET /pricing-snapshots", s.handleListSnapshots)
}

// handleEstimate handles POST /estimate
func (s *Server) handleEstimate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	// Parse request
	var req EstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "INVALID_JSON", err.Error(), http.StatusBadRequest)
		return
	}

	// Validate
	if err := validateEstimateRequest(&req); err != nil {
		s.writeError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}

	// Compute deterministic input hash
	inputHash := computeInputHash(&req)

	// Execute engine (NO COST LOGIC HERE)
	result, err := s.handler.execute(ctx, generateRequestID(), &req)
	if err != nil {
		s.writeError(w, "ENGINE_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

	// Add metadata
	result.Metadata = &ResponseMetadata{
		InputHash:       inputHash,
		EngineVersion:   s.version,
		PricingSnapshot: getCurrentPricingSnapshot(),
		Mode:            string(req.Mode),
		DurationMs:      time.Since(start).Milliseconds(),
	}

	s.writeJSON(w, result, http.StatusOK)
}

// handleDiff handles POST /diff
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	var req DiffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "INVALID_JSON", err.Error(), http.StatusBadRequest)
		return
	}

	// Execute diff (NO COST LOGIC HERE)
	result, err := s.executeDiff(ctx, &req)
	if err != nil {
		s.writeError(w, "DIFF_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

	result.DurationMs = time.Since(start).Milliseconds()
	s.writeJSON(w, result, http.StatusOK)
}

// handleHealth handles GET /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, map[string]interface{}{
		"status":  "healthy",
		"version": s.version,
		"time":    time.Now().UTC().Format(time.RFC3339),
	}, http.StatusOK)
}

// handleVersion handles GET /version
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, map[string]string{
		"version":    s.version,
		"engine":     "terraform-cost",
		"api_version": "v1",
	}, http.StatusOK)
}

// handleListSnapshots handles GET /pricing-snapshots
func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.writeJSON(w, map[string]interface{}{
			"error": "Database not connected",
			"snapshots": []map[string]string{},
		}, http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	
	// Query active snapshots for each cloud/region
	type snapshotInfo struct {
		ID        string `json:"id"`
		Cloud     string `json:"cloud"`
		Region    string `json:"region"`
		RateCount int    `json:"rate_count"`
		FetchedAt string `json:"fetched_at"`
	}
	
	var snapshots []snapshotInfo
	
	// Check AWS us-east-1
	if snap, err := s.store.GetActiveSnapshot(ctx, db.AWS, "us-east-1", "default"); err == nil && snap != nil {
		count, _ := s.store.CountRates(ctx, snap.ID)
		snapshots = append(snapshots, snapshotInfo{
			ID:        snap.ID.String(),
			Cloud:     "aws",
			Region:    snap.Region,
			RateCount: count,
			FetchedAt: snap.FetchedAt.Format(time.RFC3339),
		})
	}
	
	s.writeJSON(w, map[string]interface{}{
		"snapshots": snapshots,
		"count":     len(snapshots),
	}, http.StatusOK)
}

// executeDiff executes a diff between two estimates
func (s *Server) executeDiff(ctx context.Context, req *DiffRequest) (*DiffResponse, error) {
	// Get base estimate
	baseReq := &EstimateRequest{
		Source: SourceConfig{Type: "git", Ref: req.Base.Ref},
		Mode:   req.Mode,
	}
	baseResult, err := s.handler.execute(ctx, generateRequestID(), baseReq)
	if err != nil {
		return nil, fmt.Errorf("base estimate failed: %w", err)
	}

	// Get head estimate
	headReq := &EstimateRequest{
		Source: SourceConfig{Type: "git", Ref: req.Head.Ref},
		Mode:   req.Mode,
	}
	headResult, err := s.handler.execute(ctx, generateRequestID(), headReq)
	if err != nil {
		return nil, fmt.Errorf("head estimate failed: %w", err)
	}

	// Compute diff (using engine diff, NOT computing here)
	return computeDiff(baseResult, headResult), nil
}

func (s *Server) writeJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, code, message string, status int) {
	s.writeJSON(w, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	}, status)
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the server
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s)
}

// Helper functions

func computeInputHash(req *EstimateRequest) string {
	data, _ := json.Marshal(req)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func getCurrentPricingSnapshot() string {
	return fmt.Sprintf("aws-us-east-1-%s", time.Now().Format("2006-01-02"))
}

func validateEstimateRequest(req *EstimateRequest) error {
	if req.Source.Type == "" {
		return fmt.Errorf("source.type is required")
	}
	return nil
}

func computeDiff(base, head *EstimateResponse) *DiffResponse {
	// Parse costs
	baseCost := parseCost(base.TotalMonthlyCost)
	headCost := parseCost(head.TotalMonthlyCost)

	delta := headCost - baseCost
	var deltaStr string
	if delta >= 0 {
		deltaStr = fmt.Sprintf("+$%.2f", delta)
	} else {
		deltaStr = fmt.Sprintf("-$%.2f", -delta)
	}

	return &DiffResponse{
		Base: DiffSummary{
			Ref:         "",
			TotalCost:   base.TotalMonthlyCost,
			Confidence:  base.Confidence,
		},
		Head: DiffSummary{
			Ref:         "",
			TotalCost:   head.TotalMonthlyCost,
			Confidence:  head.Confidence,
		},
		Delta: DiffDelta{
			MonthlyCost:      deltaStr,
			ConfidenceDelta:  head.Confidence - base.Confidence,
		},
		Changes:       []DiffChange{},
		PolicyResults: []PolicyResult{},
	}
}

func parseCost(cost *CostValue) float64 {
	if cost == nil {
		return 0
	}
	var f float64
	fmt.Sscanf(cost.Amount, "%f", &f)
	return f
}
