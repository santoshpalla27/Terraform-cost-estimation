// Package terraform provides a formal evaluation pipeline with strict phase separation.
// Phases: PARSE → EVALUATE → RESOLVE → EXPAND → BUILD
package terraform

import (
	"context"
	"fmt"
	"sort"

	"terraform-cost/core/determinism"
	"terraform-cost/core/model"
)

// Phase represents a distinct stage in evaluation
type Phase int

const (
	PhaseParse    Phase = iota // Parse HCL into raw blocks
	PhaseEvaluate              // Evaluate expressions
	PhaseResolve               // Resolve variables, locals, data sources
	PhaseExpand                // Expand count/for_each
	PhaseBuild                 // Build instance graph
)

// String returns the phase name
func (p Phase) String() string {
	switch p {
	case PhaseParse:
		return "parse"
	case PhaseEvaluate:
		return "evaluate"
	case PhaseResolve:
		return "resolve"
	case PhaseExpand:
		return "expand"
	case PhaseBuild:
		return "build"
	default:
		return "unknown"
	}
}

// Pipeline orchestrates all evaluation phases in strict order
type Pipeline struct {
	parser    *Parser
	evaluator *Evaluator
	resolver  *Resolver
	expander  *Expander
	builder   *GraphBuilder

	// Options
	opts PipelineOptions
}

// PipelineOptions configures pipeline behavior
type PipelineOptions struct {
	// Workspace name (default: "default")
	Workspace string

	// Variables from CLI/environment
	Variables map[string]any

	// Target resources (partial apply)
	Targets []string

	// Continue on errors
	ContinueOnError bool

	// Provider defaults
	DefaultProviders map[string]ProviderConfig

	// Unknown handling
	UnknownCountDefault int
}

// NewPipeline creates a new evaluation pipeline
func NewPipeline(opts PipelineOptions) *Pipeline {
	if opts.Workspace == "" {
		opts.Workspace = "default"
	}
	if opts.UnknownCountDefault == 0 {
		opts.UnknownCountDefault = 1
	}

	return &Pipeline{
		parser:    NewParser(),
		evaluator: NewEvaluator(),
		resolver:  NewResolver(opts.Variables),
		expander:  NewExpander(opts.UnknownCountDefault),
		builder:   NewGraphBuilder(),
		opts:      opts,
	}
}

// PipelineResult is the output of the pipeline
type PipelineResult struct {
	Graph    *model.InstanceGraph
	Warnings []Warning
	Errors   []Error
	Stats    PipelineStats
}

// PipelineStats tracks statistics from the pipeline run
type PipelineStats struct {
	DefinitionsFound int
	InstancesCreated int
	EdgesCreated     int
	UnknownValues    int
	ParseDuration    int64 // milliseconds
	TotalDuration    int64
}

// Warning is a non-fatal issue
type Warning struct {
	Phase   Phase
	Address string
	Message string
}

// Error is a fatal issue (may be recoverable if ContinueOnError)
type Error struct {
	Phase   Phase
	Address string
	Message string
	Cause   error
}

// Execute runs all phases in strict order
func (p *Pipeline) Execute(ctx context.Context, input *ScanInput) (*PipelineResult, error) {
	result := &PipelineResult{
		Warnings: []Warning{},
		Errors:   []Error{},
	}

	// Phase 1: Parse
	parsed, err := p.runParse(ctx, input, result)
	if err != nil && !p.opts.ContinueOnError {
		return result, fmt.Errorf("parse phase failed: %w", err)
	}

	// Phase 2: Evaluate expressions
	evaluated, err := p.runEvaluate(ctx, parsed, result)
	if err != nil && !p.opts.ContinueOnError {
		return result, fmt.Errorf("evaluate phase failed: %w", err)
	}

	// Phase 3: Resolve variables, locals, data sources
	resolved, err := p.runResolve(ctx, evaluated, result)
	if err != nil && !p.opts.ContinueOnError {
		return result, fmt.Errorf("resolve phase failed: %w", err)
	}

	// Phase 4: Expand instances
	expanded, err := p.runExpand(ctx, resolved, result)
	if err != nil && !p.opts.ContinueOnError {
		return result, fmt.Errorf("expand phase failed: %w", err)
	}

	// Phase 5: Build instance graph
	graph, err := p.runBuild(ctx, expanded, result)
	if err != nil {
		return result, fmt.Errorf("build phase failed: %w", err)
	}

	result.Graph = graph
	result.Stats.DefinitionsFound = len(parsed.Definitions)
	result.Stats.InstancesCreated = graph.Size()
	return result, nil
}

// ScanInput is the input to the pipeline
type ScanInput struct {
	RootPath    string
	Files       []string
	ModulePaths []string
	Workspace   string
}

// ParsedModule represents parsed HCL content
type ParsedModule struct {
	Path        string
	Definitions []*model.AssetDefinition
	Variables   []*VariableBlock
	Locals      []*LocalBlock
	Outputs     []*OutputBlock
	Providers   []*ProviderBlock
	Modules     []*ModuleCall
	DataSources []*model.AssetDefinition
}

// VariableBlock is a Terraform variable
type VariableBlock struct {
	Name        string
	Type        string
	Default     any
	Description string
	Sensitive   bool
	Validation  []ValidationRule
}

// LocalBlock is a Terraform local
type LocalBlock struct {
	Name       string
	Expression model.Expression
}

// OutputBlock is a Terraform output
type OutputBlock struct {
	Name        string
	Expression  model.Expression
	Description string
	Sensitive   bool
	DependsOn   []string
}

// ProviderBlock is a Terraform provider configuration
type ProviderBlock struct {
	Type       string
	Alias      string
	Attributes map[string]model.Expression
}

// ProviderConfig is a resolved provider
type ProviderConfig struct {
	Type   string
	Alias  string
	Region string
	Config map[string]any
}

// ModuleCall is a Terraform module block
type ModuleCall struct {
	Name       string
	Source     string
	Version    string
	Count      *model.Expression
	ForEach    *model.Expression
	Providers  map[string]string // Provider aliases
	Inputs     map[string]model.Expression
	DependsOn  []string
}

// ValidationRule is a variable validation
type ValidationRule struct {
	Condition    model.Expression
	ErrorMessage string
}

// runParse executes the parse phase
func (p *Pipeline) runParse(ctx context.Context, input *ScanInput, result *PipelineResult) (*ParsedModule, error) {
	return p.parser.Parse(ctx, input)
}

// EvaluatedModule has expressions partially evaluated
type EvaluatedModule struct {
	*ParsedModule
	// Local values computed
	ComputedLocals map[string]any
	// Provider configs resolved
	ResolvedProviders map[string]ProviderConfig
}

func (p *Pipeline) runEvaluate(ctx context.Context, parsed *ParsedModule, result *PipelineResult) (*EvaluatedModule, error) {
	return p.evaluator.Evaluate(ctx, parsed)
}

// ResolvedModule has all references resolved
type ResolvedModule struct {
	*EvaluatedModule
	// Variable values after resolution
	ResolvedVariables map[string]any
	// Data sources evaluated (some may be unknown)
	ResolvedData map[string]ResolvedData
}

// ResolvedData is a resolved data source
type ResolvedData struct {
	Address    string
	Attributes map[string]model.ResolvedAttribute
	IsKnown    bool
}

func (p *Pipeline) runResolve(ctx context.Context, evaluated *EvaluatedModule, result *PipelineResult) (*ResolvedModule, error) {
	return p.resolver.Resolve(ctx, evaluated)
}

// ExpandedModule has all count/for_each expanded
type ExpandedModule struct {
	*ResolvedModule
	// Instances after expansion
	Instances []*model.AssetInstance
}

func (p *Pipeline) runExpand(ctx context.Context, resolved *ResolvedModule, result *PipelineResult) (*ExpandedModule, error) {
	return p.expander.Expand(ctx, resolved, result)
}

func (p *Pipeline) runBuild(ctx context.Context, expanded *ExpandedModule, result *PipelineResult) (*model.InstanceGraph, error) {
	return p.builder.Build(ctx, expanded)
}

// Parser handles Phase 1: Parse
type Parser struct{}

func NewParser() *Parser { return &Parser{} }

func (p *Parser) Parse(ctx context.Context, input *ScanInput) (*ParsedModule, error) {
	// Implementation would use HCL parser
	// For now, return skeleton
	return &ParsedModule{
		Path:        input.RootPath,
		Definitions: []*model.AssetDefinition{},
		Variables:   []*VariableBlock{},
		Locals:      []*LocalBlock{},
	}, nil
}

// Evaluator handles Phase 2: Evaluate
type Evaluator struct{}

func NewEvaluator() *Evaluator { return &Evaluator{} }

func (e *Evaluator) Evaluate(ctx context.Context, parsed *ParsedModule) (*EvaluatedModule, error) {
	result := &EvaluatedModule{
		ParsedModule:      parsed,
		ComputedLocals:    make(map[string]any),
		ResolvedProviders: make(map[string]ProviderConfig),
	}

	// Evaluate locals in dependency order
	// Resolve provider configurations

	return result, nil
}

// Resolver handles Phase 3: Resolve
type Resolver struct {
	inputVars map[string]any
}

func NewResolver(vars map[string]any) *Resolver {
	if vars == nil {
		vars = make(map[string]any)
	}
	return &Resolver{inputVars: vars}
}

func (r *Resolver) Resolve(ctx context.Context, evaluated *EvaluatedModule) (*ResolvedModule, error) {
	result := &ResolvedModule{
		EvaluatedModule:   evaluated,
		ResolvedVariables: make(map[string]any),
		ResolvedData:      make(map[string]ResolvedData),
	}

	// Resolve variables from inputs, defaults, environment
	for _, v := range evaluated.Variables {
		if val, ok := r.inputVars[v.Name]; ok {
			result.ResolvedVariables[v.Name] = val
		} else if v.Default != nil {
			result.ResolvedVariables[v.Name] = v.Default
		}
	}

	return result, nil
}

// Expander handles Phase 4: Expand
type Expander struct {
	defaultCount int
}

func NewExpander(defaultCount int) *Expander {
	return &Expander{defaultCount: defaultCount}
}

func (e *Expander) Expand(ctx context.Context, resolved *ResolvedModule, result *PipelineResult) (*ExpandedModule, error) {
	expanded := &ExpandedModule{
		ResolvedModule: resolved,
		Instances:      []*model.AssetInstance{},
	}

	idGen := determinism.NewIDGenerator("inst")

	for _, def := range resolved.Definitions {
		instances, warnings := e.expandDefinition(def, resolved, idGen)
		expanded.Instances = append(expanded.Instances, instances...)

		for _, w := range warnings {
			result.Warnings = append(result.Warnings, Warning{
				Phase:   PhaseExpand,
				Address: string(def.Address),
				Message: w,
			})
		}
	}

	// Sort instances for determinism
	sort.Slice(expanded.Instances, func(i, j int) bool {
		return expanded.Instances[i].Address < expanded.Instances[j].Address
	})

	return expanded, nil
}

func (e *Expander) expandDefinition(def *model.AssetDefinition, resolved *ResolvedModule, idGen *determinism.IDGenerator) ([]*model.AssetInstance, []string) {
	var warnings []string

	// Handle count
	if def.Count != nil {
		count, known := e.resolveCount(def.Count, resolved)
		if !known {
			warnings = append(warnings, fmt.Sprintf("count could not be determined, assuming %d", e.defaultCount))
			count = e.defaultCount
		}

		instances := make([]*model.AssetInstance, count)
		for i := 0; i < count; i++ {
			addr := model.InstanceAddress(fmt.Sprintf("%s[%d]", def.Address, i))
			instances[i] = &model.AssetInstance{
				ID:           model.InstanceID(idGen.Generate(string(def.ID), fmt.Sprintf("%d", i))),
				DefinitionID: def.ID,
				Address:      addr,
				Key:          model.InstanceKey{Type: model.KeyTypeInt, IntValue: i},
				Attributes:   e.resolveAttributes(def, i, "", resolved),
			}
		}
		return instances, warnings
	}

	// Handle for_each
	if def.ForEach != nil {
		keys, known := e.resolveForEach(def.ForEach, resolved)
		if !known {
			warnings = append(warnings, "for_each could not be determined")
			return []*model.AssetInstance{}, warnings
		}

		// Sort keys for determinism
		sort.Strings(keys)

		instances := make([]*model.AssetInstance, len(keys))
		for i, key := range keys {
			addr := model.InstanceAddress(fmt.Sprintf("%s[%q]", def.Address, key))
			instances[i] = &model.AssetInstance{
				ID:           model.InstanceID(idGen.Generate(string(def.ID), key)),
				DefinitionID: def.ID,
				Address:      addr,
				Key:          model.InstanceKey{Type: model.KeyTypeString, StrValue: key},
				Attributes:   e.resolveAttributes(def, 0, key, resolved),
			}
		}
		return instances, warnings
	}

	// No expansion - single instance
	return []*model.AssetInstance{
		{
			ID:           model.InstanceID(idGen.Generate(string(def.ID))),
			DefinitionID: def.ID,
			Address:      model.InstanceAddress(def.Address),
			Key:          model.InstanceKey{Type: model.KeyTypeNone},
			Attributes:   e.resolveAttributes(def, 0, "", resolved),
		},
	}, warnings
}

func (e *Expander) resolveCount(expr *model.Expression, resolved *ResolvedModule) (int, bool) {
	if expr.IsLiteral {
		if n, ok := expr.LiteralVal.(int); ok {
			return n, true
		}
		if f, ok := expr.LiteralVal.(float64); ok {
			return int(f), true
		}
	}
	// Would need full expression evaluation
	return 0, false
}

func (e *Expander) resolveForEach(expr *model.Expression, resolved *ResolvedModule) ([]string, bool) {
	if expr.IsLiteral {
		switch v := expr.LiteralVal.(type) {
		case map[string]any:
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			return keys, true
		case []any:
			keys := make([]string, len(v))
			for i, item := range v {
				if s, ok := item.(string); ok {
					keys[i] = s
				}
			}
			return keys, true
		}
	}
	return nil, false
}

func (e *Expander) resolveAttributes(def *model.AssetDefinition, countIdx int, eachKey string, resolved *ResolvedModule) map[string]model.ResolvedAttribute {
	result := make(map[string]model.ResolvedAttribute)

	for name, expr := range def.Attributes {
		// Skip meta-arguments
		if name == "count" || name == "for_each" || name == "depends_on" || name == "lifecycle" || name == "provider" {
			continue
		}

		if expr.IsLiteral {
			result[name] = model.ResolvedAttribute{
				Value:     expr.LiteralVal,
				IsUnknown: false,
			}
		} else {
			// Mark as unknown for now
			result[name] = model.ResolvedAttribute{
				IsUnknown: true,
				Reason:    model.ReasonComputedAtApply,
			}
		}
	}

	return result
}

// GraphBuilder handles Phase 5: Build
type GraphBuilder struct{}

func NewGraphBuilder() *GraphBuilder { return &GraphBuilder{} }

func (b *GraphBuilder) Build(ctx context.Context, expanded *ExpandedModule) (*model.InstanceGraph, error) {
	graph := model.NewInstanceGraph()

	// Add all instances
	for _, inst := range expanded.Instances {
		graph.AddInstance(inst)
	}

	// Build dependency edges from depends_on and references
	// This requires analyzing instance dependencies

	return graph, nil
}
