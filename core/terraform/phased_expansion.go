// Package terraform - Phased module expansion
// Expansion happens in strict order to avoid phantom resources:
// 1. Parse → 2. Bind Providers → 3. Resolve Variables → 4. Expand Modules → 5. Expand Resources
package terraform

import (
	"context"
	"fmt"
	"sort"
)

// ExpansionPhase represents a distinct expansion stage
type ExpansionPhase int

const (
	ExpansionPhaseParse           ExpansionPhase = iota // Parse HCL
	ExpansionPhaseBindProviders                          // Bind provider configurations
	ExpansionPhaseResolveVars                            // Resolve variables and locals
	ExpansionPhaseExpandModules                          // Expand module calls
	ExpansionPhaseExpandResources                        // Expand count/for_each
	ExpansionPhaseBuildGraph                             // Build final graph
)

// String returns the phase name
func (p ExpansionPhase) String() string {
	switch p {
	case ExpansionPhaseParse:
		return "parse"
	case ExpansionPhaseBindProviders:
		return "bind_providers"
	case ExpansionPhaseResolveVars:
		return "resolve_vars"
	case ExpansionPhaseExpandModules:
		return "expand_modules"
	case ExpansionPhaseExpandResources:
		return "expand_resources"
	case ExpansionPhaseBuildGraph:
		return "build_graph"
	default:
		return "unknown"
	}
}

// PhasedExpander expands infrastructure in strict phases
type PhasedExpander struct {
	// Current phase
	currentPhase ExpansionPhase

	// Mode for handling unknowns
	mode EvaluationMode

	// Enforcer for strict mode
	enforcer *StrictModeEnforcer

	// Phase results
	phaseResults map[ExpansionPhase]*PhaseResult

	// Provider resolver
	providerResolver *ModuleProviderResolver

	// Expansion stats
	stats ExpansionStats
}

// PhaseResult is the result of a single phase
type PhaseResult struct {
	Phase       ExpansionPhase
	Success     bool
	Errors      []PhaseError
	Warnings    []PhaseWarning
	ItemCount   int
	DurationMs  int64
}

// PhaseError is an error from a phase
type PhaseError struct {
	Phase   ExpansionPhase
	Address string
	Message string
	Cause   error
}

// PhaseWarning is a warning from a phase
type PhaseWarning struct {
	Phase   ExpansionPhase
	Address string
	Message string
}

// ExpansionStats tracks expansion statistics
type ExpansionStats struct {
	ModulesFound      int
	ModulesExpanded   int
	ResourcesFound    int
	ResourcesExpanded int
	InstancesCreated  int
	UnknownsDeferred  int
	ProvidersResolved int
	VariablesResolved int
}

// NewPhasedExpander creates a new phased expander
func NewPhasedExpander(mode EvaluationMode) *PhasedExpander {
	return &PhasedExpander{
		currentPhase:     ExpansionPhaseParse,
		mode:             mode,
		enforcer:         NewStrictModeEnforcer(mode),
		phaseResults:     make(map[ExpansionPhase]*PhaseResult),
		providerResolver: NewModuleProviderResolver(),
	}
}

// ExpansionInput is the input to phased expansion
type ExpansionInput struct {
	// Parsed modules
	Modules []*ModuleDefinition

	// Root variables
	Variables map[string]interface{}

	// Provider configurations
	Providers []*ProviderConfig

	// Workspace
	Workspace string
}

// ModuleDefinition is a parsed module
type ModuleDefinition struct {
	Path      string
	Source    string
	Parent    string
	Count     *ExpressionValue
	ForEach   *ExpressionValue
	Providers map[string]string
	Inputs    map[string]*ExpressionValue
	Resources []*ResourceDefinition
	Children  []string
}

// ResourceDefinition is a parsed resource
type ResourceDefinition struct {
	Address      string
	ModulePath   string
	Type         string
	Name         string
	Provider     string
	Count        *ExpressionValue
	ForEach      *ExpressionValue
	Attributes   map[string]*ExpressionValue
	DependsOn    []string
}

// ExpressionValue is an expression that may or may not be evaluated
type ExpressionValue struct {
	IsKnown    bool
	Value      interface{}
	Expression string
	References []string
}

// ExpansionOutput is the output of phased expansion
type ExpansionOutput struct {
	// Final instances
	Instances []*ExpandedInstance

	// Phase results
	Phases []*PhaseResult

	// Stats
	Stats ExpansionStats

	// Is expansion complete?
	Complete bool

	// Blocked on unknowns?
	Blocked bool
	BlockedReasons []string
}

// ExpandedInstance is an expanded resource instance
type ExpandedInstance struct {
	Address       string
	ModulePath    string
	DefinitionID  string
	ResourceType  string
	InstanceKey   interface{}
	Provider      *ProviderContext
	Attributes    map[string]interface{}
	DependsOn     []string
	References    []string
}

// Expand runs all phases in order
func (e *PhasedExpander) Expand(ctx context.Context, input *ExpansionInput) (*ExpansionOutput, error) {
	output := &ExpansionOutput{
		Instances: []*ExpandedInstance{},
		Phases:    []*PhaseResult{},
		Complete:  false,
	}

	// Phase 1: Parse (already done - input is parsed)
	e.recordPhase(ExpansionPhaseParse, true, 0, nil)

	// Phase 2: Bind Providers
	if err := e.executeBindProviders(ctx, input); err != nil {
		output.Phases = e.getPhaseResults()
		return output, err
	}

	// Phase 3: Resolve Variables
	resolvedVars, err := e.executeResolveVars(ctx, input)
	if err != nil && e.mode == ModeStrict {
		output.Phases = e.getPhaseResults()
		return output, err
	}

	// Phase 4: Expand Modules
	expandedModules, err := e.executeExpandModules(ctx, input, resolvedVars)
	if err != nil && e.mode == ModeStrict {
		output.Phases = e.getPhaseResults()
		return output, err
	}

	// Phase 5: Expand Resources
	instances, err := e.executeExpandResources(ctx, expandedModules, resolvedVars)
	if err != nil && e.mode == ModeStrict {
		output.Phases = e.getPhaseResults()
		return output, err
	}

	output.Instances = instances
	output.Phases = e.getPhaseResults()
	output.Stats = e.stats
	output.Complete = !e.enforcer.IsBlocked()
	output.Blocked = e.enforcer.IsBlocked()

	if output.Blocked {
		for _, err := range e.enforcer.GetBlockingErrors() {
			output.BlockedReasons = append(output.BlockedReasons, err.Reason)
		}
	}

	return output, nil
}

func (e *PhasedExpander) executeBindProviders(ctx context.Context, input *ExpansionInput) error {
	e.currentPhase = ExpansionPhaseBindProviders

	// Register root providers
	for _, p := range input.Providers {
		e.providerResolver.RegisterRootProvider(p)
		e.stats.ProvidersResolved++
	}

	// Register module provider mappings
	for _, mod := range input.Modules {
		if len(mod.Providers) > 0 {
			e.providerResolver.RegisterModuleCall(mod.Parent, mod.Path, mod.Providers)
		}
	}

	e.recordPhase(ExpansionPhaseBindProviders, true, len(input.Providers), nil)
	return nil
}

func (e *PhasedExpander) executeResolveVars(ctx context.Context, input *ExpansionInput) (map[string]interface{}, error) {
	e.currentPhase = ExpansionPhaseResolveVars

	resolved := make(map[string]interface{})
	var errors []PhaseError

	// Copy input variables
	for k, v := range input.Variables {
		resolved[k] = v
		e.stats.VariablesResolved++
	}

	// NOTE: In full implementation, this would:
	// - Resolve locals in dependency order
	// - Evaluate variable defaults
	// - Handle workspace-specific values

	hasErrors := len(errors) > 0
	e.recordPhase(ExpansionPhaseResolveVars, !hasErrors, e.stats.VariablesResolved, errors)

	if hasErrors && e.mode == ModeStrict {
		return resolved, fmt.Errorf("variable resolution failed in strict mode")
	}

	return resolved, nil
}

func (e *PhasedExpander) executeExpandModules(ctx context.Context, input *ExpansionInput, vars map[string]interface{}) ([]*ModuleDefinition, error) {
	e.currentPhase = ExpansionPhaseExpandModules

	var expanded []*ModuleDefinition
	var errors []PhaseError

	for _, mod := range input.Modules {
		e.stats.ModulesFound++

		// Check count/for_each
		if mod.Count != nil {
			if !mod.Count.IsKnown {
				if err := e.enforcer.CheckUnknownCount(mod.Path, "module count is unknown"); err != nil {
					errors = append(errors, PhaseError{
						Phase:   ExpansionPhaseExpandModules,
						Address: mod.Path,
						Message: "unknown module count",
						Cause:   err,
					})
					e.stats.UnknownsDeferred++
					continue
				}
			}
		}

		if mod.ForEach != nil {
			if !mod.ForEach.IsKnown {
				if err := e.enforcer.CheckUnknownForEach(mod.Path, "module for_each is unknown"); err != nil {
					errors = append(errors, PhaseError{
						Phase:   ExpansionPhaseExpandModules,
						Address: mod.Path,
						Message: "unknown module for_each",
						Cause:   err,
					})
					e.stats.UnknownsDeferred++
					continue
				}
			}
		}

		expanded = append(expanded, mod)
		e.stats.ModulesExpanded++
	}

	hasErrors := len(errors) > 0
	e.recordPhase(ExpansionPhaseExpandModules, !hasErrors, e.stats.ModulesExpanded, errors)

	return expanded, nil
}

func (e *PhasedExpander) executeExpandResources(ctx context.Context, modules []*ModuleDefinition, vars map[string]interface{}) ([]*ExpandedInstance, error) {
	e.currentPhase = ExpansionPhaseExpandResources

	var instances []*ExpandedInstance
	var errors []PhaseError

	for _, mod := range modules {
		for _, res := range mod.Resources {
			e.stats.ResourcesFound++

			// Resolve provider for this resource
			providerCtx, err := e.providerResolver.ResolveProvider(res.ModulePath, res.Type, res.Provider)
			if err != nil {
				if strictErr := e.enforcer.CheckUnknownProvider(res.Address, res.Provider); strictErr != nil {
					errors = append(errors, PhaseError{
						Phase:   ExpansionPhaseExpandResources,
						Address: res.Address,
						Message: err.Error(),
						Cause:   strictErr,
					})
					continue
				}
			}

			// Expand count
			if res.Count != nil {
				if !res.Count.IsKnown {
					if err := e.enforcer.CheckUnknownCount(res.Address, "resource count is unknown"); err != nil {
						errors = append(errors, PhaseError{
							Phase:   ExpansionPhaseExpandResources,
							Address: res.Address,
							Message: "unknown count",
							Cause:   err,
						})
						e.stats.UnknownsDeferred++
						continue
					}
					// In permissive/estimate mode, expand with default
					res.Count.Value = GetModeConfig(e.mode).DefaultUnknownCount
				}

				count := e.toInt(res.Count.Value)
				for i := 0; i < count; i++ {
					inst := e.createInstance(res, i, nil, providerCtx)
					instances = append(instances, inst)
					e.stats.InstancesCreated++
				}
				e.stats.ResourcesExpanded++
				continue
			}

			// Expand for_each
			if res.ForEach != nil {
				if !res.ForEach.IsKnown {
					if err := e.enforcer.CheckUnknownForEach(res.Address, "resource for_each is unknown"); err != nil {
						errors = append(errors, PhaseError{
							Phase:   ExpansionPhaseExpandResources,
							Address: res.Address,
							Message: "unknown for_each",
							Cause:   err,
						})
						e.stats.UnknownsDeferred++
						continue
					}
				}

				keys := e.extractKeys(res.ForEach.Value)
				for _, key := range keys {
					inst := e.createInstance(res, 0, key, providerCtx)
					instances = append(instances, inst)
					e.stats.InstancesCreated++
				}
				e.stats.ResourcesExpanded++
				continue
			}

			// Single instance
			inst := e.createInstance(res, 0, nil, providerCtx)
			instances = append(instances, inst)
			e.stats.InstancesCreated++
			e.stats.ResourcesExpanded++
		}
	}

	// Sort for determinism
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].Address < instances[j].Address
	})

	hasErrors := len(errors) > 0
	e.recordPhase(ExpansionPhaseExpandResources, !hasErrors, e.stats.InstancesCreated, errors)

	return instances, nil
}

func (e *PhasedExpander) createInstance(res *ResourceDefinition, countIdx int, forEachKey interface{}, provider *ProviderContext) *ExpandedInstance {
	address := res.Address
	if countIdx > 0 || res.Count != nil {
		address = fmt.Sprintf("%s[%d]", res.Address, countIdx)
	}
	if forEachKey != nil {
		address = fmt.Sprintf("%s[%q]", res.Address, forEachKey)
	}

	return &ExpandedInstance{
		Address:      address,
		ModulePath:   res.ModulePath,
		DefinitionID: res.Address,
		ResourceType: res.Type,
		InstanceKey:  forEachKey,
		Provider:     provider,
		DependsOn:    res.DependsOn,
	}
}

func (e *PhasedExpander) recordPhase(phase ExpansionPhase, success bool, count int, errors []PhaseError) {
	result := &PhaseResult{
		Phase:     phase,
		Success:   success,
		ItemCount: count,
		Errors:    errors,
	}
	e.phaseResults[phase] = result
}

func (e *PhasedExpander) getPhaseResults() []*PhaseResult {
	results := make([]*PhaseResult, 0, len(e.phaseResults))
	for phase := ExpansionPhaseParse; phase <= ExpansionPhaseBuildGraph; phase++ {
		if r, ok := e.phaseResults[phase]; ok {
			results = append(results, r)
		}
	}
	return results
}

func (e *PhasedExpander) toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 1
	}
}

func (e *PhasedExpander) extractKeys(v interface{}) []string {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	case []interface{}:
		keys := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				keys = append(keys, s)
			}
		}
		return keys
	default:
		return []string{}
	}
}
