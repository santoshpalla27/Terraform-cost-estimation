// Package terraform - Module output strict handling
// Module outputs are NEVER like locals.
// They are: dependency edges, potentially unknown, cardinality-affecting.
package terraform

import (
	"fmt"
	"strings"
)

// StrictModuleOutput enforces correct module output handling
type StrictModuleOutput struct {
	// Always a dependency edge
	IsDependencyEdge bool // ALWAYS true

	// Output identity
	ModulePath string
	OutputName string
	Address    string

	// State
	IsKnown     bool
	IsComputed  bool // Runtime value
	IsSensitive bool

	// Value (only if known and not sensitive)
	Value interface{}

	// Expression (always captured)
	Expression string
	References []string

	// Cardinality impact
	AffectsCardinality    bool
	CardinalityExpression string

	// Confidence
	ConfidenceImpact float64
}

// NewStrictModuleOutput creates a strict module output
func NewStrictModuleOutput(modulePath, outputName string) *StrictModuleOutput {
	return &StrictModuleOutput{
		IsDependencyEdge: true, // ALWAYS
		ModulePath:       modulePath,
		OutputName:       outputName,
		Address:          modulePath + "." + outputName,
		IsKnown:          false, // Default to unknown
		ConfidenceImpact: 0.2,  // Default impact for module outputs
	}
}

// MarkKnown marks the output as known with a value
func (o *StrictModuleOutput) MarkKnown(value interface{}) {
	o.IsKnown = true
	o.Value = value

	// Check if value affects cardinality
	switch v := value.(type) {
	case []interface{}:
		o.AffectsCardinality = true
		o.CardinalityExpression = fmt.Sprintf("list with %d elements", len(v))
	case map[string]interface{}:
		o.AffectsCardinality = true
		o.CardinalityExpression = fmt.Sprintf("map with %d keys", len(v))
	}
}

// MarkUnknown marks the output as unknown
func (o *StrictModuleOutput) MarkUnknown(expression string, references []string) {
	o.IsKnown = false
	o.Expression = expression
	o.References = references
	o.ConfidenceImpact = 0.3 // Higher impact for unknown

	// Check if expression affects cardinality
	if o.expressionAffectsCardinality(expression) {
		o.AffectsCardinality = true
		o.ConfidenceImpact = 0.4
	}
}

func (o *StrictModuleOutput) expressionAffectsCardinality(expr string) bool {
	if expr == "" {
		return false
	}
	cardinalityFns := []string{"tolist", "toset", "tomap", "list", "set", "map", "keys", "values"}
	for _, fn := range cardinalityFns {
		if strings.Contains(expr, fn+"(") {
			return true
		}
	}
	return false
}

// StrictModuleOutputRegistry tracks all module outputs
type StrictModuleOutputRegistry struct {
	outputs map[string]*StrictModuleOutput
}

// NewStrictModuleOutputRegistry creates a registry
func NewStrictModuleOutputRegistry() *StrictModuleOutputRegistry {
	return &StrictModuleOutputRegistry{
		outputs: make(map[string]*StrictModuleOutput),
	}
}

// Register registers a module output
func (r *StrictModuleOutputRegistry) Register(output *StrictModuleOutput) {
	r.outputs[output.Address] = output
}

// Get gets an output by address
func (r *StrictModuleOutputRegistry) Get(address string) (*StrictModuleOutput, bool) {
	output, ok := r.outputs[address]
	return output, ok
}

// IsModuleOutputReference checks if a reference is to a module output
func (r *StrictModuleOutputRegistry) IsModuleOutputReference(ref string) bool {
	// module.name.output_name pattern
	if !strings.HasPrefix(ref, "module.") {
		return false
	}
	parts := strings.Split(ref, ".")
	return len(parts) >= 3
}

// ResolveReference resolves a module output reference
func (r *StrictModuleOutputRegistry) ResolveReference(ref string) (*StrictModuleOutput, error) {
	if !r.IsModuleOutputReference(ref) {
		return nil, fmt.Errorf("not a module output reference: %s", ref)
	}

	output, ok := r.Get(ref)
	if !ok {
		// Unknown module output - create a strict unknown entry
		parts := strings.Split(ref, ".")
		if len(parts) >= 3 {
			modulePath := strings.Join(parts[:2], ".")
			outputName := strings.Join(parts[2:], ".")
			output = NewStrictModuleOutput(modulePath, outputName)
			output.MarkUnknown("unknown_reference", []string{ref})
			r.Register(output)
		}
		return output, nil
	}

	return output, nil
}

// ValidateUsageInCardinality validates using module output for cardinality
func (r *StrictModuleOutputRegistry) ValidateUsageInCardinality(ref string, context string) *StrictModuleOutputWarning {
	output, _ := r.ResolveReference(ref)
	if output == nil {
		return nil
	}

	// If output is unknown or affects cardinality, this is dangerous
	if !output.IsKnown || output.AffectsCardinality {
		return &StrictModuleOutputWarning{
			Address:  ref,
			Context:  context,
			Warning:  fmt.Sprintf("module output %s used in cardinality context but is %s", ref, output.state()),
			Severity: WarningSevere,
			BlocksEstimation: !output.IsKnown,
		}
	}

	return nil
}

func (o *StrictModuleOutput) state() string {
	if !o.IsKnown {
		return "unknown"
	}
	if o.AffectsCardinality {
		return "cardinality-affecting"
	}
	return "known"
}

// StrictModuleOutputWarning is a warning about module output usage
type StrictModuleOutputWarning struct {
	Address          string
	Context          string
	Warning          string
	Severity         WarningSeverity
	BlocksEstimation bool
}

// WarningSeverity indicates warning severity
type WarningSeverity int

const (
	WarningInfo WarningSeverity = iota
	WarningModerate
	WarningSevere
)

// AllUnknown returns all unknown outputs
func (r *StrictModuleOutputRegistry) AllUnknown() []*StrictModuleOutput {
	var result []*StrictModuleOutput
	for _, output := range r.outputs {
		if !output.IsKnown {
			result = append(result, output)
		}
	}
	return result
}

// AllCardinalityAffecting returns outputs that affect cardinality
func (r *StrictModuleOutputRegistry) AllCardinalityAffecting() []*StrictModuleOutput {
	var result []*StrictModuleOutput
	for _, output := range r.outputs {
		if output.AffectsCardinality {
			result = append(result, output)
		}
	}
	return result
}
