// Package engine - Authoritative estimation orchestrator
// ENFORCES the correct execution flow:
// 1. Terraform Dependency Graph (authoritative)
// 2. Provider Binding (frozen)
// 3. Expanded Asset Graph (instances)
// 4. Cost Graph (derived from asset graph)
// 5. Policy Evaluation (on full context)
package engine

import (
	"context"
	"fmt"
	"time"

	"terraform-cost/core/cost"
	"terraform-cost/core/graph"
	"terraform-cost/core/model"
	"terraform-cost/core/policy"
	"terraform-cost/core/terraform"
)

// AuthoritativeOrchestrator enforces the correct execution order
type AuthoritativeOrchestrator struct {
	// Current phase - can only move forward
	phase OrchestrationPhase

	// Mode
	mode terraform.EvaluationMode

	// Components - each is nil until its phase
	infraGraph       *graph.InfrastructureGraph
	providerFinal    *terraform.ProviderFinalizer
	bindingRegistry  *terraform.BindingRegistry
	assetGraph       *AssetGraph
	costGraph        *graph.DependencyAwareCostGraph
	policyEngine     *policy.DiffPolicyEngine

	// Cardinality warnings
	cardinalityWarns *terraform.CardinalityWarnings

	// Errors
	errors []OrchestrationError
}

// OrchestrationPhase represents execution phases
type OrchestrationPhase int

const (
	PhaseUninitialized OrchestrationPhase = iota
	PhaseParsed                            // Terraform parsed
	PhaseGraphBuilt                        // Dependency graph built
	PhaseProvidersFrozen                   // Providers finalized
	PhaseExpanded                          // Assets expanded
	PhaseCosted                            // Costs calculated
	PhasePolicyEvaluated                   // Policies run
	PhaseComplete
)

// String returns the phase name
func (p OrchestrationPhase) String() string {
	names := []string{
		"uninitialized", "parsed", "graph_built", "providers_frozen",
		"expanded", "costed", "policy_evaluated", "complete",
	}
	if int(p) < len(names) {
		return names[p]
	}
	return "unknown"
}

// OrchestrationError is an error during orchestration
type OrchestrationError struct {
	Phase   OrchestrationPhase
	Message string
	Cause   error
	Fatal   bool
}

// NewAuthoritativeOrchestrator creates an orchestrator
func NewAuthoritativeOrchestrator(mode terraform.EvaluationMode) *AuthoritativeOrchestrator {
	return &AuthoritativeOrchestrator{
		phase:            PhaseUninitialized,
		mode:             mode,
		providerFinal:    terraform.NewProviderFinalizer(),
		bindingRegistry:  terraform.NewBindingRegistry(),
		cardinalityWarns: terraform.NewCardinalityWarnings(),
		policyEngine:     policy.NewDiffPolicyEngine(),
		errors:           []OrchestrationError{},
	}
}

// PhaseGuard ensures a phase has been completed
func (o *AuthoritativeOrchestrator) PhaseGuard(required OrchestrationPhase) error {
	if o.phase < required {
		return &PhaseOrderError{
			Required: required,
			Current:  o.phase,
		}
	}
	return nil
}

// PhaseOrderError indicates phases executed out of order
type PhaseOrderError struct {
	Required OrchestrationPhase
	Current  OrchestrationPhase
}

func (e *PhaseOrderError) Error() string {
	return fmt.Sprintf("phase %s required, but current phase is %s", e.Required, e.Current)
}

// BuildDependencyGraph builds the authoritative dependency graph
func (o *AuthoritativeOrchestrator) BuildDependencyGraph(ctx context.Context, parsed *graph.ParsedInfra) error {
	if o.phase >= PhaseGraphBuilt {
		return fmt.Errorf("dependency graph already built")
	}

	builder := graph.NewInfraGraphBuilder()
	infraGraph, err := builder.Build(parsed)
	if err != nil {
		o.recordError(PhaseGraphBuilt, "failed to build dependency graph", err, true)
		return err
	}

	o.infraGraph = infraGraph
	o.phase = PhaseGraphBuilt
	return nil
}

// FreezeProviders freezes all provider configurations
func (o *AuthoritativeOrchestrator) FreezeProviders(ctx context.Context, providers []*terraform.ProviderContext) error {
	if err := o.PhaseGuard(PhaseGraphBuilt); err != nil {
		return err
	}
	if o.phase >= PhaseProvidersFrozen {
		return fmt.Errorf("providers already frozen")
	}

	for _, p := range providers {
		if _, err := o.providerFinal.Freeze(p); err != nil {
			o.recordError(PhaseProvidersFrozen, "failed to freeze provider", err, true)
			return err
		}
	}

	o.providerFinal.Finalize()
	o.phase = PhaseProvidersFrozen
	return nil
}

// ExpandAssets expands all assets with frozen providers
func (o *AuthoritativeOrchestrator) ExpandAssets(ctx context.Context, definitions []*terraform.ResourceDefinition) error {
	if err := o.PhaseGuard(PhaseProvidersFrozen); err != nil {
		return err
	}
	if o.phase >= PhaseExpanded {
		return fmt.Errorf("assets already expanded")
	}

	// Ensure providers are finalized
	if !o.providerFinal.IsFinalized() {
		return fmt.Errorf("cannot expand assets: providers not finalized")
	}

	o.assetGraph = NewAssetGraph()
	forEachEval := terraform.NewSafeForEachEvaluator(o.mode)
	countEval := terraform.NewSafeCountEvaluator(o.mode)

	for _, def := range definitions {
		// Get frozen provider
		providerKey := def.Provider
		if providerKey == "" {
			providerKey = extractProviderType(def.Type)
		}

		frozenProvider, ok := o.providerFinal.Get(providerKey)
		if !ok {
			if o.mode == terraform.ModeStrict {
				o.recordError(PhaseExpanded, "no frozen provider for "+def.Address, nil, true)
				return fmt.Errorf("no frozen provider for %s", def.Address)
			}
			// Use default in permissive mode
		}

		// Handle for_each
		if def.ForEach != nil {
			result := forEachEval.Evaluate(def.Address, def.ForEach)
			o.cardinalityWarns.AddForEach(def.Address, result)

			if result.BlocksEstimation {
				o.recordError(PhaseExpanded, result.Warning, nil, true)
				return fmt.Errorf("for_each blocked: %s", result.Warning)
			}

			if result.IsKnown {
				// Expand with known keys
				for _, key := range result.Keys {
					o.addAssetInstance(def, key, frozenProvider)
				}
			} else {
				// DO NOT EXPAND - add symbolic placeholder
				o.addSymbolicAsset(def, result.SymbolicRange, frozenProvider)
			}
			continue
		}

		// Handle count
		if def.Count != nil {
			result := countEval.Evaluate(def.Address, def.Count)
			o.cardinalityWarns.AddCount(def.Address, result)

			if result.BlocksEstimation {
				o.recordError(PhaseExpanded, result.Warning, nil, true)
				return fmt.Errorf("count blocked: %s", result.Warning)
			}

			if result.IsKnown {
				for i := 0; i < result.Value; i++ {
					o.addAssetInstance(def, i, frozenProvider)
				}
			} else {
				// DO NOT EXPAND - add symbolic placeholder
				o.addSymbolicAsset(def, result.SymbolicRange, frozenProvider)
			}
			continue
		}

		// Single instance
		o.addAssetInstance(def, nil, frozenProvider)
	}

	o.phase = PhaseExpanded
	return nil
}

func (o *AuthoritativeOrchestrator) addAssetInstance(def *terraform.ResourceDefinition, key interface{}, provider *terraform.FrozenProviderContext) {
	address := def.Address
	if key != nil {
		switch k := key.(type) {
		case int:
			address = fmt.Sprintf("%s[%d]", def.Address, k)
		case string:
			address = fmt.Sprintf("%s[%q]", def.Address, k)
		}
	}

	asset := &AssetInstance{
		ID:           model.InstanceID(address),
		Address:      model.InstanceAddress(address),
		DefinitionID: def.Address,
		ResourceType: def.Type,
		Provider:     provider,
		InstanceKey:  key,
		IsSymbolic:   false,
	}

	o.assetGraph.AddInstance(asset)

	// Bind to provider
	if provider != nil {
		o.bindingRegistry.Bind(address, key, provider)
	}
}

func (o *AuthoritativeOrchestrator) addSymbolicAsset(def *terraform.ResourceDefinition, symRange *terraform.SymbolicRange, provider *terraform.FrozenProviderContext) {
	asset := &AssetInstance{
		ID:            model.InstanceID(def.Address + "[*]"),
		Address:       model.InstanceAddress(def.Address + "[*]"),
		DefinitionID:  def.Address,
		ResourceType:  def.Type,
		Provider:      provider,
		IsSymbolic:    true,
		SymbolicRange: symRange,
	}

	o.assetGraph.AddInstance(asset)
}

// CalculateCosts calculates costs from expanded assets
func (o *AuthoritativeOrchestrator) CalculateCosts(ctx context.Context, calculator CostCalculator) error {
	if err := o.PhaseGuard(PhaseExpanded); err != nil {
		return err
	}
	if o.phase >= PhaseCosted {
		return fmt.Errorf("costs already calculated")
	}

	// Pricing gate enforces provider binding
	gate := terraform.NewProviderPricingGate(o.providerFinal, o.bindingRegistry)

	// Create cost graph
	rawCostGraph := cost.NewCostGraph("project")

	for _, asset := range o.assetGraph.AllInstances() {
		// Skip symbolic assets - they represent unknown cardinality
		if asset.IsSymbolic {
			continue
		}

		// Verify pricing is allowed
		if err := gate.CanPrice(string(asset.Address)); err != nil {
			if o.mode == terraform.ModeStrict {
				return err
			}
			o.recordError(PhaseCosted, err.Error(), err, false)
			continue
		}

		// Calculate cost
		costNode, err := calculator.Calculate(asset)
		if err != nil {
			o.recordError(PhaseCosted, "failed to calculate cost for "+string(asset.Address), err, false)
			continue
		}

		rawCostGraph.AddNode(costNode)
	}

	// Build service aggregates
	rawCostGraph.BuildServiceAggregates()

	// Create dependency-aware cost graph
	o.costGraph = graph.NewDependencyAwareCostGraph(o.infraGraph, rawCostGraph)

	o.phase = PhaseCosted
	return nil
}

// EvaluatePolicies evaluates policies on the cost graph
func (o *AuthoritativeOrchestrator) EvaluatePolicies(ctx context.Context, diffCtx *policy.DiffPolicyContext) (*policy.DiffPolicyEngineResult, error) {
	if err := o.PhaseGuard(PhaseCosted); err != nil {
		return nil, err
	}

	// Policies operate on full context
	result := o.policyEngine.Evaluate(diffCtx)
	o.phase = PhasePolicyEvaluated

	return result, nil
}

// AddPolicy adds a policy to evaluate
func (o *AuthoritativeOrchestrator) AddPolicy(p policy.DiffAwarePolicy) {
	o.policyEngine.AddPolicy(p)
}

func (o *AuthoritativeOrchestrator) recordError(phase OrchestrationPhase, msg string, cause error, fatal bool) {
	o.errors = append(o.errors, OrchestrationError{
		Phase:   phase,
		Message: msg,
		Cause:   cause,
		Fatal:   fatal,
	})
}

// GetErrors returns all errors
func (o *AuthoritativeOrchestrator) GetErrors() []OrchestrationError {
	return o.errors
}

// GetCardinalityWarnings returns cardinality warnings
func (o *AuthoritativeOrchestrator) GetCardinalityWarnings() []terraform.CardinalityWarning {
	return o.cardinalityWarns.All()
}

// GetCostGraph returns the dependency-aware cost graph
func (o *AuthoritativeOrchestrator) GetCostGraph() *graph.DependencyAwareCostGraph {
	return o.costGraph
}

// GetAssetGraph returns the asset graph
func (o *AuthoritativeOrchestrator) GetAssetGraph() *AssetGraph {
	return o.assetGraph
}

// GetPhase returns current phase
func (o *AuthoritativeOrchestrator) GetPhase() OrchestrationPhase {
	return o.phase
}

func extractProviderType(resourceType string) string {
	// aws_instance → aws
	for i, c := range resourceType {
		if c == '_' {
			return resourceType[:i]
		}
	}
	return resourceType
}

// AssetGraph holds expanded asset instances
type AssetGraph struct {
	instances map[model.InstanceID]*AssetInstance
	order     []model.InstanceID
}

// NewAssetGraph creates an asset graph
func NewAssetGraph() *AssetGraph {
	return &AssetGraph{
		instances: make(map[model.InstanceID]*AssetInstance),
		order:     []model.InstanceID{},
	}
}

// AddInstance adds an instance
func (g *AssetGraph) AddInstance(inst *AssetInstance) {
	g.instances[inst.ID] = inst
	g.order = append(g.order, inst.ID)
}

// GetInstance returns an instance
func (g *AssetGraph) GetInstance(id model.InstanceID) *AssetInstance {
	return g.instances[id]
}

// AllInstances returns all instances in order
func (g *AssetGraph) AllInstances() []*AssetInstance {
	result := make([]*AssetInstance, 0, len(g.order))
	for _, id := range g.order {
		result = append(result, g.instances[id])
	}
	return result
}

// Count returns the number of instances
func (g *AssetGraph) Count() int {
	return len(g.instances)
}

// SymbolicCount returns count of symbolic (unknown cardinality) assets
func (g *AssetGraph) SymbolicCount() int {
	count := 0
	for _, inst := range g.instances {
		if inst.IsSymbolic {
			count++
		}
	}
	return count
}

// AssetInstance is an expanded asset
type AssetInstance struct {
	ID            model.InstanceID
	Address       model.InstanceAddress
	DefinitionID  string
	ResourceType  string
	Provider      *terraform.FrozenProviderContext
	InstanceKey   interface{}
	IsSymbolic    bool
	SymbolicRange *terraform.SymbolicRange
}

// CostCalculator calculates cost for an asset
type CostCalculator interface {
	Calculate(asset *AssetInstance) (*cost.CostNode, error)
}

// OrchestrationResult is the final result
type OrchestrationResult struct {
	Phase             OrchestrationPhase
	CostGraph         *graph.DependencyAwareCostGraph
	TotalMonthly      float64
	TotalHourly       float64
	Confidence        float64
	ResourceCount     int
	SymbolicCount     int
	CardinalityWarns  []terraform.CardinalityWarning
	PolicyResult      *policy.DiffPolicyEngineResult
	Errors            []OrchestrationError
	Duration          time.Duration
}

// NewOrchestrationResult creates a result from orchestrator
func NewOrchestrationResult(o *AuthoritativeOrchestrator, duration time.Duration) *OrchestrationResult {
	result := &OrchestrationResult{
		Phase:            o.GetPhase(),
		CostGraph:        o.GetCostGraph(),
		CardinalityWarns: o.GetCardinalityWarnings(),
		Errors:           o.GetErrors(),
		Duration:         duration,
	}

	if o.assetGraph != nil {
		result.ResourceCount = o.assetGraph.Count()
		result.SymbolicCount = o.assetGraph.SymbolicCount()
	}

	// Calculate aggregates from cost graph
	// No field access needed here

	return result
}
