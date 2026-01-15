// Package engine provides the API-primary estimation engine.
// CLI is a thin wrapper around this engine.
package engine

import (
	"context"
	"fmt"
	"time"

	"terraform-cost/core/determinism"
	"terraform-cost/core/model"
	"terraform-cost/core/pricing"
)

// Engine is the primary API for cost estimation.
// All other interfaces (CLI, HTTP, CI) are thin wrappers.
type Engine struct {
	// Required dependencies
	pricingResolver PricingResolver
	usageEstimator  UsageEstimator
	policyEvaluator PolicyEvaluator

	// Plugin registry
	cloudPlugins map[string]CloudPlugin

	// Configuration
	config EngineConfig
}

// EngineConfig configures the estimation engine
type EngineConfig struct {
	// Default region for pricing
	DefaultRegion string

	// Unknown handling
	UnknownCountDefault int
	UnknownBehavior     UnknownBehavior

	// Confidence thresholds
	MinConfidenceForEstimate float64
}

// UnknownBehavior defines how to handle unknown values
type UnknownBehavior int

const (
	// UnknownPropagate marks results as uncertain (correct behavior)
	UnknownPropagate UnknownBehavior = iota
	// UnknownFail returns an error on unknowns
	UnknownFail
)

// PricingResolver resolves pricing - MUST use snapshots
type PricingResolver interface {
	// GetSnapshot returns a pricing snapshot - never "latest" implicitly
	GetSnapshot(ctx context.Context, req SnapshotRequest) (*pricing.PricingSnapshot, error)

	// LookupRate finds a rate within a snapshot
	LookupRate(snapshot *pricing.PricingSnapshot, resourceType, component string, attrs map[string]string) (*pricing.RateEntry, error)
}

// SnapshotRequest specifies which snapshot to retrieve
type SnapshotRequest struct {
	// SnapshotID is preferred if known
	SnapshotID pricing.SnapshotID

	// Otherwise, specify provider and region
	Provider string
	Region   string

	// AsOf specifies point-in-time (nil = latest known)
	AsOf *time.Time
}

// UsageEstimator estimates usage for instances
type UsageEstimator interface {
	Estimate(ctx context.Context, instance *model.AssetInstance) (*UsageResult, error)
}

// UsageResult contains estimated usage with confidence
type UsageResult struct {
	Metrics    map[string]UsageMetric
	Source     pricing.UsageSource
	Confidence float64
}

// UsageMetric is a single usage estimate
type UsageMetric struct {
	Name       string
	Value      float64
	Unit       string
	Confidence float64
	IsUnknown  bool
}

// PolicyEvaluator evaluates cost policies
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, result *EstimationResult) (*PolicyResult, error)
}

// CloudPlugin provides cloud-specific cost mapping
type CloudPlugin interface {
	Provider() string
	MapInstance(instance *model.AssetInstance) ([]CostComponent, error)
}

// CostComponent is a billable component of an instance
type CostComponent struct {
	Name         string
	ResourceType string
	Unit         string
	Attributes   map[string]string
}

// NewEngine creates a new estimation engine
func NewEngine(
	pricingResolver PricingResolver,
	usageEstimator UsageEstimator,
	policyEvaluator PolicyEvaluator,
	config EngineConfig,
) *Engine {
	return &Engine{
		pricingResolver: pricingResolver,
		usageEstimator:  usageEstimator,
		policyEvaluator: policyEvaluator,
		cloudPlugins:    make(map[string]CloudPlugin),
		config:          config,
	}
}

// RegisterPlugin registers a cloud plugin
func (e *Engine) RegisterPlugin(plugin CloudPlugin) {
	e.cloudPlugins[plugin.Provider()] = plugin
}

// EstimateRequest is the input to estimation
type EstimateRequest struct {
	// REQUIRED: Instance graph to estimate
	Graph *model.InstanceGraph

	// REQUIRED: Pricing snapshot to use
	SnapshotRequest SnapshotRequest

	// Optional: Usage overrides per instance
	UsageOverrides map[model.InstanceID]map[string]float64

	// Optional: Policy configuration
	PolicyConfig map[string]any
}

// EstimationResult is the output of estimation
type EstimationResult struct {
	// Pricing snapshot used (for reproducibility)
	Snapshot *SnapshotReference

	// Costs per INSTANCE (not definition)
	InstanceCosts *determinism.StableMap[model.InstanceID, *InstanceCost]

	// Aggregated totals
	TotalMonthlyCost determinism.Money
	TotalHourlyCost  determinism.Money

	// Overall confidence
	Confidence CostConfidence

	// Warnings and degradations
	Warnings []string
	Degraded bool

	// Policy results (if evaluated)
	PolicyResult *PolicyResult

	// Timing
	EstimatedAt time.Time
	Duration    time.Duration
}

// SnapshotReference is an immutable reference to the pricing snapshot used
type SnapshotReference struct {
	ID          pricing.SnapshotID
	ContentHash determinism.ContentHash
	EffectiveAt time.Time
	Provider    string
	Region      string
}

// InstanceCost is the cost for a SINGLE INSTANCE (not definition)
type InstanceCost struct {
	// Instance identity
	InstanceID model.InstanceID
	Address    model.InstanceAddress

	// Link to definition (for grouping)
	DefinitionID model.DefinitionID

	// Cost components
	Components []*ComponentCost

	// Roll-ups
	MonthlyCost determinism.Money
	HourlyCost  determinism.Money

	// Confidence for THIS instance
	Confidence CostConfidence

	// Full lineage for explainability
	Lineage []*pricing.CostLineage
}

// ComponentCost is a single cost component
type ComponentCost struct {
	Name        string
	MonthlyCost determinism.Money
	HourlyCost  determinism.Money

	// Rate used
	RateID  pricing.RateID
	RateKey pricing.RateKey

	// Usage applied
	UsageValue float64
	UsageUnit  string

	// Formula
	Formula pricing.FormulaApplication

	// Confidence
	Confidence float64
}

// CostConfidence tracks estimation confidence
type CostConfidence struct {
	Score   float64 // 0.0 - 1.0
	Factors []ConfidenceFactor
}

// ConfidenceFactor explains why confidence is reduced
type ConfidenceFactor struct {
	Reason      string
	Impact      float64 // How much this reduces confidence
	Component   string  // Which component affected
	IsUnknown   bool    // Is this due to an unknown value?
}

// PolicyResult is the output of policy evaluation
type PolicyResult struct {
	Passed   bool
	Policies []PolicyOutcome
}

// PolicyOutcome is the result of a single policy
type PolicyOutcome struct {
	Name    string
	Passed  bool
	Message string

	// Deep context for explainability
	AffectedInstances []model.InstanceID
	CostImpact        determinism.Money
	LineageRefs       []*pricing.CostLineage
}

// Estimate performs the estimation
func (e *Engine) Estimate(ctx context.Context, req *EstimateRequest) (*EstimationResult, error) {
	start := time.Now()

	// REQUIRED: Validate inputs
	if req.Graph == nil {
		return nil, fmt.Errorf("instance graph is required")
	}

	// REQUIRED: Get pricing snapshot
	snapshot, err := e.pricingResolver.GetSnapshot(ctx, req.SnapshotRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to get pricing snapshot: %w", err)
	}

	// Verify snapshot integrity
	if !snapshot.Verify() {
		return nil, fmt.Errorf("pricing snapshot failed integrity check")
	}

	result := &EstimationResult{
		Snapshot: &SnapshotReference{
			ID:          snapshot.ID,
			ContentHash: snapshot.ContentHash,
			EffectiveAt: snapshot.EffectiveAt,
			Provider:    snapshot.Provider,
			Region:      snapshot.Region,
		},
		InstanceCosts:    determinism.NewStableMap[model.InstanceID, *InstanceCost](),
		TotalMonthlyCost: determinism.Zero("USD"),
		TotalHourlyCost:  determinism.Zero("USD"),
		Confidence:       CostConfidence{Score: 1.0},
		EstimatedAt:      time.Now().UTC(),
	}

	// Process each INSTANCE (not definition)
	for _, inst := range req.Graph.Instances() {
		instanceCost, err := e.estimateInstance(ctx, inst, snapshot, req.UsageOverrides)
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("%s: %v", inst.Address, err))
			result.Degraded = true
			continue
		}

		result.InstanceCosts.Set(inst.ID, instanceCost)
		result.TotalMonthlyCost = result.TotalMonthlyCost.Add(instanceCost.MonthlyCost)
		result.TotalHourlyCost = result.TotalHourlyCost.Add(instanceCost.HourlyCost)

		// Compound confidence
		result.Confidence.Score *= instanceCost.Confidence.Score
	}

	// Evaluate policies with full context
	if e.policyEvaluator != nil {
		policyResult, err := e.policyEvaluator.Evaluate(ctx, result)
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("policy evaluation failed: %v", err))
		} else {
			result.PolicyResult = policyResult
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

func (e *Engine) estimateInstance(
	ctx context.Context,
	inst *model.AssetInstance,
	snapshot *pricing.PricingSnapshot,
	overrides map[model.InstanceID]map[string]float64,
) (*InstanceCost, error) {
	result := &InstanceCost{
		InstanceID:   inst.ID,
		Address:      inst.Address,
		DefinitionID: inst.DefinitionID,
		Components:   []*ComponentCost{},
		MonthlyCost:  determinism.Zero("USD"),
		HourlyCost:   determinism.Zero("USD"),
		Confidence:   CostConfidence{Score: 1.0},
		Lineage:      []*pricing.CostLineage{},
	}

	// Get cloud plugin
	plugin, ok := e.cloudPlugins[inst.Provider.Type]
	if !ok {
		return result, fmt.Errorf("no plugin for provider %s", inst.Provider.Type)
	}

	// Map instance to cost components
	components, err := plugin.MapInstance(inst)
	if err != nil {
		return result, err
	}

	// Get usage estimates
	usage, err := e.usageEstimator.Estimate(ctx, inst)
	if err != nil {
		result.Confidence.Factors = append(result.Confidence.Factors, ConfidenceFactor{
			Reason: "usage estimation failed",
			Impact: 0.3,
		})
		usage = &UsageResult{Confidence: 0.5}
	}

	// Apply overrides if present
	instanceOverrides := overrides[inst.ID]

	// Price each component
	for _, comp := range components {
		compCost, lineage := e.priceComponent(comp, inst, snapshot, usage, instanceOverrides)
		result.Components = append(result.Components, compCost)
		result.MonthlyCost = result.MonthlyCost.Add(compCost.MonthlyCost)
		result.HourlyCost = result.HourlyCost.Add(compCost.HourlyCost)
		result.Lineage = append(result.Lineage, lineage)

		// Track confidence factors
		if compCost.Confidence < 1.0 {
			result.Confidence.Factors = append(result.Confidence.Factors, ConfidenceFactor{
				Reason:    "reduced component confidence",
				Impact:    1.0 - compCost.Confidence,
				Component: comp.Name,
			})
		}
	}

	// Calculate overall confidence
	result.Confidence.Score = e.calculateConfidence(result)

	return result, nil
}

func (e *Engine) priceComponent(
	comp CostComponent,
	inst *model.AssetInstance,
	snapshot *pricing.PricingSnapshot,
	usage *UsageResult,
	overrides map[string]float64,
) (*ComponentCost, *pricing.CostLineage) {
	result := &ComponentCost{
		Name:       comp.Name,
		Confidence: 1.0,
	}

	lineage := &pricing.CostLineage{
		InstanceID: string(inst.ID),
		Component:  comp.Name,
		SnapshotID: snapshot.ID,
		Timestamp:  time.Now().UTC(),
	}

	// Look up rate
	rate, ok := snapshot.LookupRate(comp.ResourceType, comp.Name, comp.Attributes)
	if !ok {
		// Rate not found - degraded estimation
		result.Confidence = 0.0
		lineage.Confidence = 0.0
		return result, lineage
	}

	result.RateID = rate.ID
	result.RateKey = rate.Key
	lineage.RateID = rate.ID
	lineage.RateKey = rate.Key

	// Get usage value
	usageValue := 730.0 // Default monthly hours
	usageUnit := "hours"
	usageConfidence := 1.0

	if metric, ok := usage.Metrics[comp.Name]; ok {
		if metric.IsUnknown {
			// UNKNOWN: propagate, don't guess
			result.Confidence *= 0.5
			usageConfidence = 0.5
			lineage.Confidence = 0.5
		} else {
			usageValue = metric.Value
			usageUnit = metric.Unit
			usageConfidence = metric.Confidence
		}
	}

	// Apply override if present
	if override, ok := overrides[comp.Name]; ok {
		usageValue = override
		usageConfidence = 1.0 // User-provided is trusted
	}

	result.UsageValue = usageValue
	result.UsageUnit = usageUnit

	// Calculate cost
	monthlyCost := determinism.NewMoneyFromDecimal(
		rate.Price.Mul(determinism.NewMoneyFromFloat(usageValue, "USD").Amount()),
		rate.Currency,
	)
	hourlyCost := monthlyCost.Div(determinism.NewMoneyFromFloat(730.0, "USD").Amount())

	result.MonthlyCost = monthlyCost
	result.HourlyCost = hourlyCost
	result.Confidence *= usageConfidence

	// Record formula
	result.Formula = pricing.FormulaApplication{
		Name:       "usage_based",
		Expression: fmt.Sprintf("%s * %s", rate.Price.String(), usageUnit),
		Inputs: map[string]string{
			"rate":  rate.Price.String(),
			"usage": fmt.Sprintf("%.2f", usageValue),
			"unit":  usageUnit,
		},
		Output: monthlyCost.StringRaw(),
	}
	lineage.Formula = result.Formula
	lineage.Usage = pricing.UsageLineage{
		Source:     usage.Source,
		Confidence: usageConfidence,
	}
	lineage.Confidence = result.Confidence

	return result, lineage
}

func (e *Engine) calculateConfidence(ic *InstanceCost) float64 {
	if len(ic.Components) == 0 {
		return 0.0
	}

	total := 0.0
	for _, c := range ic.Components {
		total += c.Confidence
	}
	return total / float64(len(ic.Components))
}
