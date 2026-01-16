// Package enforcement - Unified architectural enforcement
// This is the ONLY entry point for estimation.
// All other paths are blocked by package visibility.
package enforcement

import (
	"context"
	"fmt"

	"terraform-cost/core/determinism"
	"terraform-cost/core/graph"
	"terraform-cost/core/guards"
	"terraform-cost/core/pricing"
	"terraform-cost/core/terraform"
)

// EstimationPipeline is the ONLY way to perform estimation.
// It enforces all architectural invariants at each step.
type EstimationPipeline struct {
	enforcer        *guards.InvariantEnforcer
	guardedExpand   *guards.GuardedExpansion
	mode            terraform.EvaluationMode
	
	// State - each becomes non-nil only after its phase
	depGraph        *graph.InfrastructureGraph
	providerFinal   *terraform.ProviderFinalizer
	costGraph       *graph.DerivedCostGraph
	pricingSnapshot *pricing.PricingSnapshot
	
	// Accumulated warnings and errors
	warnings        []string
	symbolicCosts   []*graph.SymbolicCost
}

// NewEstimationPipeline creates the pipeline
func NewEstimationPipeline(mode terraform.EvaluationMode) *EstimationPipeline {
	enforcer := guards.NewInvariantEnforcer()
	return &EstimationPipeline{
		enforcer:      enforcer,
		guardedExpand: guards.NewGuardedExpansion(enforcer),
		mode:          mode,
		warnings:      []string{},
		symbolicCosts: []*graph.SymbolicCost{},
	}
}

// Step1_BuildDependencyGraph builds the authoritative dependency graph
// This MUST be called first.
func (p *EstimationPipeline) Step1_BuildDependencyGraph(ctx context.Context, parsed *graph.ParsedInfra) error {
	builder := graph.NewInfraGraphBuilder()
	depGraph, err := builder.Build(parsed)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	p.depGraph = depGraph
	p.enforcer.MarkDependencyGraphBuilt(depGraph)
	return nil
}

// Step2_FreezeProviders freezes all provider configurations
// This MUST be called after Step1.
func (p *EstimationPipeline) Step2_FreezeProviders(ctx context.Context, providers []*terraform.ProviderContext) error {
	p.enforcer.AssertDependencyGraphBuilt()

	p.providerFinal = terraform.NewProviderFinalizer()
	for _, prov := range providers {
		if _, err := p.providerFinal.Freeze(prov); err != nil {
			return fmt.Errorf("failed to freeze provider %s: %w", prov.ProviderKey(), err)
		}
	}
	p.providerFinal.Finalize()
	p.enforcer.MarkProvidersFrozen(p.providerFinal)
	return nil
}

// Step3_ExpandAssets expands assets using guarded expansion
// This MUST be called after Step2.
// Unknown cardinality is NEVER expanded - it becomes a SymbolicCost.
func (p *EstimationPipeline) Step3_ExpandAssets(ctx context.Context, definitions []ResourceDefinition) (*ExpandedAssets, error) {
	p.enforcer.AssertProvidersFrozen()

	expanded := &ExpandedAssets{
		Instances:     []ExpandedInstance{},
		SymbolicCosts: []*graph.SymbolicCost{},
	}

	for _, def := range definitions {
		// Verify resource is in dependency graph
		p.enforcer.AssertNodeInDependencyGraph(def.Address)

		// Get frozen provider
		providerKey := def.ProviderKey
		if providerKey == "" {
			providerKey = extractProvider(def.ResourceType)
		}
		frozenProvider, ok := p.providerFinal.Get(providerKey)
		if !ok && p.mode == terraform.ModeStrict {
			return nil, fmt.Errorf("no provider for %s in strict mode", def.Address)
		}

		// Handle expansion
		if def.Count != nil {
			instances, err := p.guardedExpand.ExpandResource(def.Address, def.Count.Value, def.Count.IsKnown)
			if err != nil {
				// Unknown cardinality - create symbolic cost
				symbolic := &graph.SymbolicCost{
					Address:      def.Address,
					Expression:   def.Count.Expression,
					MinInstances: 0,
					MaxInstances: -1, // Unbounded
					IsUnbounded:  true,
					Confidence:   0.3,
					Warning:      fmt.Sprintf("count at %s is unknown - cost is unbounded", def.Address),
				}
				expanded.SymbolicCosts = append(expanded.SymbolicCosts, symbolic)
				p.symbolicCosts = append(p.symbolicCosts, symbolic)
				continue
			}
			for _, addr := range instances {
				expanded.Instances = append(expanded.Instances, ExpandedInstance{
					Address:        addr,
					DefinitionAddr: def.Address,
					Provider:       frozenProvider,
				})
			}
			continue
		}

		if def.ForEach != nil {
			instances, err := p.guardedExpand.ExpandForEach(def.Address, def.ForEach.Keys, def.ForEach.IsKnown)
			if err != nil {
				// Unknown cardinality - create symbolic cost
				symbolic := &graph.SymbolicCost{
					Address:      def.Address,
					Expression:   def.ForEach.Expression,
					MinInstances: 0,
					MaxInstances: -1,
					IsUnbounded:  true,
					Confidence:   0.3,
					Warning:      fmt.Sprintf("for_each at %s is unknown - cost is unbounded", def.Address),
				}
				expanded.SymbolicCosts = append(expanded.SymbolicCosts, symbolic)
				p.symbolicCosts = append(p.symbolicCosts, symbolic)
				continue
			}
			for _, addr := range instances {
				expanded.Instances = append(expanded.Instances, ExpandedInstance{
					Address:        addr,
					DefinitionAddr: def.Address,
					Provider:       frozenProvider,
				})
			}
			continue
		}

		// Single instance
		expanded.Instances = append(expanded.Instances, ExpandedInstance{
			Address:        def.Address,
			DefinitionAddr: def.Address,
			Provider:       frozenProvider,
		})
	}

	p.enforcer.MarkExpansionComplete()
	p.warnings = append(p.warnings, p.guardedExpand.GetWarnings()...)
	return expanded, nil
}

// Step4_BuildCostGraph builds the cost graph FROM the dependency graph
// This MUST be called after Step3.
// The cost graph MUST derive from the dependency graph.
func (p *EstimationPipeline) Step4_BuildCostGraph(ctx context.Context, expanded *ExpandedAssets) error {
	p.enforcer.AssertExpansionComplete()

	// Cost graph MUST be derived from dependency graph
	costGraph, err := graph.NewDerivedCostGraph(p.depGraph)
	if err != nil {
		return fmt.Errorf("failed to create cost graph: %w", err)
	}

	// Add symbolic costs for unknown cardinality
	for _, symbolic := range expanded.SymbolicCosts {
		costGraph.AddSymbolicCost(
			symbolic.Address,
			determinism.Zero("USD"), // Cost per unit unknown
			symbolic.MinInstances,
			symbolic.MaxInstances,
			symbolic.Expression,
		)
	}

	p.costGraph = costGraph
	p.enforcer.MarkCostGraphBuilt()
	return nil
}

// Step5_ApplyPricing applies pricing to the cost graph
// This MUST be called after Step4.
// Provider alias MUST be in every rate key.
func (p *EstimationPipeline) Step5_ApplyPricing(ctx context.Context, expanded *ExpandedAssets, calculator PricingCalculator) error {
	p.enforcer.AssertCostGraphBuilt()

	if p.pricingSnapshot == nil {
		return fmt.Errorf("pricing snapshot not set")
	}

	resolver := pricing.NewAliasAwareRateResolver()

	for _, inst := range expanded.Instances {
		// Provider alias MUST be in rate key
		rateKey := pricing.NewAliasAwareRateKey(
			inst.Provider.Type,
			inst.Provider.Alias,
			inst.Provider.Region,
			extractResourceType(inst.Address),
			"compute",
		)

		rate, err := resolver.ResolveRate(p.pricingSnapshot, rateKey)
		if err != nil {
			if p.mode == terraform.ModeStrict {
				return fmt.Errorf("rate not found for %s: %w", inst.Address, err)
			}
			p.warnings = append(p.warnings, fmt.Sprintf("rate not found for %s", inst.Address))
			continue
		}

		// Calculate cost
		monthly := determinism.NewMoneyFromDecimal(rate.Price, rate.Currency)
		hourly := monthly.Div(determinism.NewMoneyFromFloat(730, "USD").Amount())

		if err := p.costGraph.SetNodeCost(inst.DefinitionAddr, monthly, hourly, 1.0); err != nil {
			p.warnings = append(p.warnings, fmt.Sprintf("could not set cost for %s: %v", inst.Address, err))
		}
	}

	p.costGraph.CalculateTransitiveCosts()
	p.enforcer.MarkPricingComplete()
	return nil
}

// SetPricingSnapshot sets the pricing snapshot to use
func (p *EstimationPipeline) SetPricingSnapshot(snapshot *pricing.PricingSnapshot) {
	p.pricingSnapshot = snapshot
}

// GetResult returns the estimation result
func (p *EstimationPipeline) GetResult() *EstimationResult {
	costRange := p.costGraph.GetTotalCostRange()

	return &EstimationResult{
		CostGraph:       p.costGraph,
		TotalCostRange:  costRange,
		SymbolicCosts:   p.symbolicCosts,
		HasUnbounded:    p.costGraph.HasUnboundedCosts(),
		Warnings:        p.warnings,
	}
}

// ResourceDefinition is input to expansion
type ResourceDefinition struct {
	Address      string
	ResourceType string
	ProviderKey  string
	Count        *CountValue
	ForEach      *ForEachValue
}

// CountValue represents a count expression
type CountValue struct {
	Value      int
	IsKnown    bool
	Expression string
}

// ForEachValue represents a for_each expression
type ForEachValue struct {
	Keys       []string
	IsKnown    bool
	Expression string
}

// ExpandedAssets is the result of expansion
type ExpandedAssets struct {
	Instances     []ExpandedInstance
	SymbolicCosts []*graph.SymbolicCost
}

// ExpandedInstance is a single expanded instance
type ExpandedInstance struct {
	Address        string
	DefinitionAddr string
	Provider       *terraform.FrozenProviderContext
}

// PricingCalculator calculates pricing
type PricingCalculator interface {
	Calculate(resourceType string, attrs map[string]string) (determinism.Money, error)
}

// EstimationResult is the final result
type EstimationResult struct {
	CostGraph      *graph.DerivedCostGraph
	TotalCostRange *graph.CostBounds
	SymbolicCosts  []*graph.SymbolicCost
	HasUnbounded   bool
	Warnings       []string
}

func extractProvider(resourceType string) string {
	for i, c := range resourceType {
		if c == '_' {
			return resourceType[:i]
		}
	}
	return resourceType
}

func extractResourceType(address string) string {
	// aws_instance.foo[0] → aws_instance
	for i, c := range address {
		if c == '.' {
			return address[:i]
		}
	}
	return address
}
