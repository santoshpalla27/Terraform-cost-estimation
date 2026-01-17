// Package http provides a production-grade HTTP adapter for the cost estimation engine.
// This adapter exposes the engine functionality via a RESTful API.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"terraform-cost/core/engine"
	"terraform-cost/core/model"
	"terraform-cost/core/pricing"
	"terraform-cost/core/terraform"
)

// Config holds HTTP adapter configuration
type Config struct {
	// Address to listen on
	Address string `json:"address"`
	
	// ReadTimeout for requests
	ReadTimeout time.Duration `json:"read_timeout"`
	
	// WriteTimeout for responses
	WriteTimeout time.Duration `json:"write_timeout"`
	
	// MaxBodySize limits request body size
	MaxBodySize int64 `json:"max_body_size"`
	
	// EnableCORS enables CORS headers
	EnableCORS bool `json:"enable_cors"`
	
	// AllowedOrigins for CORS
	AllowedOrigins []string `json:"allowed_origins"`
	
	// RateLimit per IP (requests per second)
	RateLimit float64 `json:"rate_limit"`
	
	// EnableMetrics enables Prometheus metrics
	EnableMetrics bool `json:"enable_metrics"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Address:        ":8080",
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxBodySize:    10 * 1024 * 1024, // 10MB
		EnableCORS:     true,
		AllowedOrigins: []string{"*"},
		RateLimit:      10,
		EnableMetrics:  true,
	}
}

// Adapter is the HTTP adapter
type Adapter struct {
	engine   *engine.Engine
	pipeline *terraform.Pipeline
	config   *Config
	server   *http.Server
	
	// Metrics
	requestCount   int64
	errorCount     int64
	totalLatencyMs int64
	mu             sync.RWMutex
}

// New creates a new HTTP adapter
func New(eng *engine.Engine, pipeline *terraform.Pipeline, config *Config) *Adapter {
	if config == nil {
		config = DefaultConfig()
	}
	
	return &Adapter{
		engine:   eng,
		pipeline: pipeline,
		config:   config,
	}
}

// Router returns the HTTP handler
func (a *Adapter) Router() http.Handler {
	mux := http.NewServeMux()
	
	// Health endpoints
	mux.HandleFunc("GET /health", a.handleHealth)
	mux.HandleFunc("GET /ready", a.handleReady)
	
	// API v1 endpoints
	mux.HandleFunc("POST /api/v1/estimate", a.handleEstimate)
	mux.HandleFunc("POST /api/v1/diff", a.handleDiff)
	mux.HandleFunc("GET /api/v1/snapshots", a.handleListSnapshots)
	mux.HandleFunc("GET /api/v1/snapshots/{id}", a.handleGetSnapshot)
	mux.HandleFunc("GET /api/v1/coverage", a.handleCoverage)
	
	// Metrics
	if a.config.EnableMetrics {
		mux.HandleFunc("GET /metrics", a.handleMetrics)
	}
	
	// Apply middleware
	handler := a.corsMiddleware(mux)
	handler = a.loggingMiddleware(handler)
	handler = a.recoveryMiddleware(handler)
	
	return handler
}

// Start starts the HTTP server
func (a *Adapter) Start() error {
	a.server = &http.Server{
		Addr:         a.config.Address,
		Handler:      a.Router(),
		ReadTimeout:  a.config.ReadTimeout,
		WriteTimeout: a.config.WriteTimeout,
	}
	
	return a.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (a *Adapter) Shutdown(ctx context.Context) error {
	if a.server != nil {
		return a.server.Shutdown(ctx)
	}
	return nil
}

// EstimateRequest is the API request body
type EstimateRequest struct {
	// TerraformPlan is the JSON plan output
	TerraformPlan json.RawMessage `json:"terraform_plan,omitempty"`
	
	// HCLPath is the path to HCL files (for local)
	HCLPath string `json:"hcl_path,omitempty"`
	
	// HCLContent is inline HCL content
	HCLContent string `json:"hcl_content,omitempty"`
	
	// Variables for Terraform
	Variables map[string]interface{} `json:"variables,omitempty"`
	
	// SnapshotID to use (optional, uses latest if empty)
	SnapshotID string `json:"snapshot_id,omitempty"`
	
	// Provider (aws, azure, gcp)
	Provider string `json:"provider"`
	
	// Region
	Region string `json:"region"`
	
	// Alias for multi-account
	Alias string `json:"alias,omitempty"`
	
	// UsageOverrides for symbolic costs
	UsageOverrides map[string]map[string]float64 `json:"usage_overrides,omitempty"`
	
	// StrictMode fails on symbolic costs
	StrictMode bool `json:"strict_mode,omitempty"`
	
	// IncludeLineage includes pricing lineage
	IncludeLineage bool `json:"include_lineage,omitempty"`
}

// EstimateResponse is the API response
type EstimateResponse struct {
	// Success indicates if estimation succeeded
	Success bool `json:"success"`
	
	// Error message if failed
	Error string `json:"error,omitempty"`
	
	// TotalMonthlyCost is the total cost
	TotalMonthlyCost string `json:"total_monthly_cost"`
	
	// TotalHourlyCost is hourly cost
	TotalHourlyCost string `json:"total_hourly_cost"`
	
	// Confidence (0-1)
	Confidence float64 `json:"confidence"`
	
	// Coverage breakdown
	Coverage CoverageResponse `json:"coverage"`
	
	// Resources with costs
	Resources []ResourceCostResponse `json:"resources"`
	
	// SymbolicReasons for symbolic costs
	SymbolicReasons map[string][]string `json:"symbolic_reasons,omitempty"`
	
	// UnsupportedTypes that couldn't be estimated
	UnsupportedTypes []string `json:"unsupported_types,omitempty"`
	
	// Snapshot used for pricing
	Snapshot SnapshotResponse `json:"snapshot"`
	
	// Lineage for auditability
	Lineage []LineageEntry `json:"lineage,omitempty"`
	
	// Warnings during estimation
	Warnings []string `json:"warnings,omitempty"`
	
	// Metadata
	Metadata ResponseMetadata `json:"metadata"`
}

// CoverageResponse is coverage breakdown
type CoverageResponse struct {
	NumericPercent     float64 `json:"numeric_percent"`
	SymbolicPercent    float64 `json:"symbolic_percent"`
	IndirectPercent    float64 `json:"indirect_percent"`
	UnsupportedPercent float64 `json:"unsupported_percent"`
	TotalResources     int     `json:"total_resources"`
	CoveredResources   int     `json:"covered_resources"`
}

// ResourceCostResponse is per-resource cost
type ResourceCostResponse struct {
	Address      string                    `json:"address"`
	Type         string                    `json:"type"`
	MonthlyCost  string                    `json:"monthly_cost"`
	HourlyCost   string                    `json:"hourly_cost"`
	Confidence   float64                   `json:"confidence"`
	CoverageType string                    `json:"coverage_type"`
	Components   []ComponentCostResponse   `json:"components,omitempty"`
}

// ComponentCostResponse is per-component cost
type ComponentCostResponse struct {
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	MonthlyCost string  `json:"monthly_cost"`
	UsageValue  float64 `json:"usage_value"`
	UsageUnit   string  `json:"usage_unit"`
	IsSymbolic  bool    `json:"is_symbolic"`
	Reason      string  `json:"reason,omitempty"`
}

// SnapshotResponse is snapshot info
type SnapshotResponse struct {
	ID           string    `json:"id"`
	Provider     string    `json:"provider"`
	Region       string    `json:"region"`
	Alias        string    `json:"alias"`
	ContentHash  string    `json:"content_hash"`
	EffectiveAt  time.Time `json:"effective_at"`
	RateCount    int       `json:"rate_count"`
}

// LineageEntry traces a rate lookup
type LineageEntry struct {
	Resource    string            `json:"resource"`
	Component   string            `json:"component"`
	RateKey     map[string]string `json:"rate_key"`
	SnapshotID  string            `json:"snapshot_id"`
	Price       string            `json:"price"`
	Unit        string            `json:"unit"`
	ResolvedAt  time.Time         `json:"resolved_at"`
}

// ResponseMetadata is response context
type ResponseMetadata struct {
	RequestID   string        `json:"request_id"`
	Duration    time.Duration `json:"duration_ms"`
	Version     string        `json:"version"`
	Timestamp   time.Time     `json:"timestamp"`
}

// Handler implementations

func (a *Adapter) handleHealth(w http.ResponseWriter, r *http.Request) {
	a.writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (a *Adapter) handleReady(w http.ResponseWriter, r *http.Request) {
	// Check engine health
	a.writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *Adapter) handleEstimate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()
	
	// Parse request
	var req EstimateRequest
	if err := a.parseJSON(r, &req); err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	
	// Validate
	if req.Provider == "" {
		a.writeError(w, http.StatusBadRequest, "provider is required")
		return
	}
	if req.Region == "" {
		a.writeError(w, http.StatusBadRequest, "region is required")
		return
	}
	
	// Build snapshot request
	snapshotReq := engine.SnapshotRequest{
		Provider: req.Provider,
		Region:   req.Region,
	}
	if req.SnapshotID != "" {
		snapshotReq.SnapshotID = pricing.SnapshotID(req.SnapshotID)
	}
	
	// Convert usage overrides
	overrides := make(map[model.InstanceID]map[string]float64)
	for k, v := range req.UsageOverrides {
		overrides[model.InstanceID(k)] = v
	}
	
	// Execute estimation
	engineReq := &engine.EstimateRequest{
		SnapshotRequest: snapshotReq,
		UsageOverrides:  overrides,
	}
	
	result, err := a.engine.Estimate(ctx, engineReq)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, "estimation failed: "+err.Error())
		return
	}
	
	// Build response
	resp := a.buildEstimateResponse(result, r.Header.Get("X-Request-ID"), start)
	a.writeJSON(w, http.StatusOK, resp)
}

func (a *Adapter) handleDiff(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement diff endpoint
	a.writeJSON(w, http.StatusOK, map[string]string{"status": "not implemented"})
}

func (a *Adapter) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement snapshot listing
	a.writeJSON(w, http.StatusOK, map[string]string{"status": "not implemented"})
}

func (a *Adapter) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement snapshot retrieval
	a.writeJSON(w, http.StatusOK, map[string]string{"status": "not implemented"})
}

func (a *Adapter) handleCoverage(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement coverage endpoint
	a.writeJSON(w, http.StatusOK, map[string]string{"status": "not implemented"})
}

func (a *Adapter) handleMetrics(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	avgLatency := float64(0)
	if a.requestCount > 0 {
		avgLatency = float64(a.totalLatencyMs) / float64(a.requestCount)
	}
	
	metrics := fmt.Sprintf(`# HELP terraform_cost_requests_total Total requests
# TYPE terraform_cost_requests_total counter
terraform_cost_requests_total %d

# HELP terraform_cost_errors_total Total errors
# TYPE terraform_cost_errors_total counter
terraform_cost_errors_total %d

# HELP terraform_cost_latency_avg_ms Average latency
# TYPE terraform_cost_latency_avg_ms gauge
terraform_cost_latency_avg_ms %.2f
`, a.requestCount, a.errorCount, avgLatency)
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(metrics))
}

func (a *Adapter) buildEstimateResponse(result *engine.EstimationResult, requestID string, start time.Time) *EstimateResponse {
	resp := &EstimateResponse{
		Success:          true,
		TotalMonthlyCost: result.TotalMonthlyCost.String(),
		TotalHourlyCost:  result.TotalHourlyCost.String(),
		Confidence:       result.Confidence.Score,
		Warnings:         result.Warnings,
		Metadata: ResponseMetadata{
			RequestID: requestID,
			Duration:  time.Since(start),
			Version:   "1.0.0",
			Timestamp: time.Now(),
		},
	}
	
	// Snapshot
	if result.Snapshot != nil {
		resp.Snapshot = SnapshotResponse{
			ID:          string(result.Snapshot.ID),
			Provider:    result.Snapshot.Provider,
			Region:      result.Snapshot.Region,
			ContentHash: result.Snapshot.ContentHash.Hex(),
			EffectiveAt: result.Snapshot.EffectiveAt,
		}
	}
	
	// Resources
	resp.Resources = make([]ResourceCostResponse, 0)
	result.InstanceCosts.Range(func(id model.InstanceID, cost *engine.InstanceCost) bool {
		rc := ResourceCostResponse{
			Address:     string(cost.Address),
			Type:        string(cost.ResourceType),
			MonthlyCost: cost.MonthlyCost.String(),
			HourlyCost:  cost.HourlyCost.String(),
			Confidence:  cost.Confidence.Score,
		}
		
		// Components
		for _, comp := range cost.Components {
			cc := ComponentCostResponse{
				Name:        comp.Name,
				MonthlyCost: comp.MonthlyCost.String(),
				UsageValue:  comp.UsageValue,
				UsageUnit:   comp.UsageUnit,
				IsSymbolic:  comp.Confidence < 0.7,
			}
			rc.Components = append(rc.Components, cc)
		}
		
		resp.Resources = append(resp.Resources, rc)
		return true
	})
	
	return resp
}

// Middleware

func (a *Adapter) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.config.EnableCORS {
			origin := "*"
			if len(a.config.AllowedOrigins) > 0 && a.config.AllowedOrigins[0] != "*" {
				origin = a.config.AllowedOrigins[0]
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
		}
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func (a *Adapter) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		
		a.mu.Lock()
		a.requestCount++
		a.totalLatencyMs += time.Since(start).Milliseconds()
		a.mu.Unlock()
	})
}

func (a *Adapter) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				a.mu.Lock()
				a.errorCount++
				a.mu.Unlock()
				
				a.writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Helpers

func (a *Adapter) parseJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, a.config.MaxBodySize))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func (a *Adapter) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (a *Adapter) writeError(w http.ResponseWriter, status int, message string) {
	a.writeJSON(w, status, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
