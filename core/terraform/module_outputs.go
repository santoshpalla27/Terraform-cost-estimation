// Package terraform - Module output semantics
// Module outputs are NOT locals. They are:
// - Dependency edges
// - Potentially unknown
// - Cardinality-affecting
package terraform

import (
	"fmt"
	"strings"
)

// ModuleOutput represents a module output value
type ModuleOutput struct {
	// Path to the module
	ModulePath string

	// Output name
	Name string

	// Full address (module.name.output_name)
	Address string

	// Is this output known?
	IsKnown bool

	// Value if known
	Value interface{}

	// Expression if unknown
	Expression string

	// References in the expression
	References []string

	// Is this a dependency edge?
	IsDependencyEdge bool

	// Can this affect cardinality?
	AffectsCardinality bool

	// Confidence impact
	ConfidenceImpact float64
}

// ModuleOutputAnalyzer analyzes module outputs
type ModuleOutputAnalyzer struct {
	outputs map[string]*ModuleOutput
}

// NewModuleOutputAnalyzer creates an analyzer
func NewModuleOutputAnalyzer() *ModuleOutputAnalyzer {
	return &ModuleOutputAnalyzer{
		outputs: make(map[string]*ModuleOutput),
	}
}

// RegisterOutput registers a module output
func (a *ModuleOutputAnalyzer) RegisterOutput(modulePath, name string, expr *ExpressionValue) *ModuleOutput {
	address := modulePath + "." + name

	output := &ModuleOutput{
		ModulePath:         modulePath,
		Name:               name,
		Address:            address,
		IsKnown:            expr != nil && expr.IsKnown,
		References:         []string{},
		IsDependencyEdge:   true, // Module outputs are ALWAYS dependency edges
		AffectsCardinality: false,
		ConfidenceImpact:   0,
	}

	if expr != nil {
		output.Value = expr.Value
		output.Expression = expr.Expression
		output.References = expr.References

		// Check if this could affect cardinality
		output.AffectsCardinality = a.couldAffectCardinality(expr)

		// Calculate confidence impact
		output.ConfidenceImpact = a.calculateConfidenceImpact(output)
	}

	a.outputs[address] = output
	return output
}

// GetOutput returns an output by address
func (a *ModuleOutputAnalyzer) GetOutput(address string) (*ModuleOutput, bool) {
	output, ok := a.outputs[address]
	return output, ok
}

// IsModuleOutputRef checks if a reference is to a module output
func (a *ModuleOutputAnalyzer) IsModuleOutputRef(ref string) bool {
	// module.name.output_name
	return strings.HasPrefix(ref, "module.") && strings.Count(ref, ".") >= 2
}

// GetReferencedOutput gets the output for a reference
func (a *ModuleOutputAnalyzer) GetReferencedOutput(ref string) (*ModuleOutput, bool) {
	if !a.IsModuleOutputRef(ref) {
		return nil, false
	}
	return a.GetOutput(ref)
}

func (a *ModuleOutputAnalyzer) couldAffectCardinality(expr *ExpressionValue) bool {
	if expr == nil {
		return false
	}

	// Check if expression returns a collection
	if expr.Expression != "" {
		collectors := []string{"list", "map", "set", "tolist", "toset", "tomap"}
		for _, c := range collectors {
			if strings.Contains(expr.Expression, c) {
				return true
			}
		}
	}

	// Check if value is a collection
	switch expr.Value.(type) {
	case []interface{}, map[string]interface{}:
		return true
	}

	return false
}

func (a *ModuleOutputAnalyzer) calculateConfidenceImpact(output *ModuleOutput) float64 {
	if output.IsKnown {
		return 0.0
	}

	impact := 0.2 // Base impact for unknown output

	// Higher impact if it affects cardinality
	if output.AffectsCardinality {
		impact += 0.15
	}

	// Higher impact if references data sources
	for _, ref := range output.References {
		if strings.HasPrefix(ref, "data.") {
			impact += 0.1
			break
		}
	}

	return impact
}

// OutputDependencyEdge represents a dependency through module output
type OutputDependencyEdge struct {
	// Source: the module producing the output
	SourceModule string
	OutputName   string

	// Target: the consumer of the output
	TargetAddress string
	TargetAttr    string

	// Semantics
	IsKnown            bool
	AffectsCardinality bool
}

// ExtractOutputDependencies extracts dependency edges from references
func (a *ModuleOutputAnalyzer) ExtractOutputDependencies(address, attribute string, references []string) []*OutputDependencyEdge {
	var edges []*OutputDependencyEdge

	for _, ref := range references {
		if a.IsModuleOutputRef(ref) {
			output, _ := a.GetReferencedOutput(ref)

			edge := &OutputDependencyEdge{
				TargetAddress: address,
				TargetAttr:    attribute,
				IsKnown:       false,
			}

			// Parse module.name.output
			parts := strings.Split(ref, ".")
			if len(parts) >= 3 {
				edge.SourceModule = parts[0] + "." + parts[1]
				edge.OutputName = parts[2]
			}

			if output != nil {
				edge.IsKnown = output.IsKnown
				edge.AffectsCardinality = output.AffectsCardinality
			}

			edges = append(edges, edge)
		}
	}

	return edges
}

// ModuleOutputValidator validates module output usage
type ModuleOutputValidator struct {
	analyzer *ModuleOutputAnalyzer
	warnings []ModuleOutputWarning
}

// ModuleOutputWarning is a warning about module output usage
type ModuleOutputWarning struct {
	TargetAddress string
	OutputRef     string
	Type          OutputWarningType
	Message       string
}

// OutputWarningType classifies warnings
type OutputWarningType int

const (
	WarnUnknownOutput          OutputWarningType = iota // Output value unknown
	WarnCardinalityFromOutput                            // Using output for cardinality
	WarnDataSourceInOutput                               // Output depends on data source
)

// NewModuleOutputValidator creates a validator
func NewModuleOutputValidator(analyzer *ModuleOutputAnalyzer) *ModuleOutputValidator {
	return &ModuleOutputValidator{
		analyzer: analyzer,
		warnings: []ModuleOutputWarning{},
	}
}

// ValidateUsage validates usage of module outputs
func (v *ModuleOutputValidator) ValidateUsage(address, attribute string, references []string, isCardinalityContext bool) {
	for _, ref := range references {
		if v.analyzer.IsModuleOutputRef(ref) {
			output, ok := v.analyzer.GetReferencedOutput(ref)

			if !ok {
				// Unknown output
				v.warnings = append(v.warnings, ModuleOutputWarning{
					TargetAddress: address,
					OutputRef:     ref,
					Type:          WarnUnknownOutput,
					Message:       fmt.Sprintf("module output %s is not registered", ref),
				})
				continue
			}

			if !output.IsKnown {
				v.warnings = append(v.warnings, ModuleOutputWarning{
					TargetAddress: address,
					OutputRef:     ref,
					Type:          WarnUnknownOutput,
					Message:       fmt.Sprintf("module output %s has unknown value", ref),
				})
			}

			if isCardinalityContext && output.AffectsCardinality {
				v.warnings = append(v.warnings, ModuleOutputWarning{
					TargetAddress: address,
					OutputRef:     ref,
					Type:          WarnCardinalityFromOutput,
					Message:       fmt.Sprintf("cardinality depends on module output %s", ref),
				})
			}
		}
	}
}

// GetWarnings returns all warnings
func (v *ModuleOutputValidator) GetWarnings() []ModuleOutputWarning {
	return v.warnings
}
