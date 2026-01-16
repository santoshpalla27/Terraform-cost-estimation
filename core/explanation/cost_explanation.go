// Package explanation - Cost explanation tree
// Exposes WHY costs exist, not just totals.
// Enables enterprise-grade explainability.
package explanation

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CostExplanation provides full transparency for a cost unit
type CostExplanation struct {
	// Identity
	Resource     string `json:"resource"`
	CostUnit     string `json:"cost_unit"`
	
	// Formula breakdown
	Formula      string   `json:"formula"`
	Inputs       []Input  `json:"inputs"`
	
	// Provenance
	UsageSource  string   `json:"usage_source"`
	PricingSource string  `json:"pricing_source,omitempty"`
	
	// Confidence
	Confidence   float64  `json:"confidence"`
	ConfidenceReason string `json:"confidence_reason,omitempty"`
	
	// Dependencies
	Dependencies []string `json:"dependencies,omitempty"`
	
	// Symbolic explanation (if applicable)
	IsSymbolic   bool     `json:"is_symbolic,omitempty"`
	SymbolicReason string `json:"symbolic_reason,omitempty"`
	SymbolicChain []string `json:"symbolic_chain,omitempty"`
}

// Input represents an input to the cost formula
type Input struct {
	Name   string  `json:"name"`
	Value  string  `json:"value"`
	Source string  `json:"source"` // "terraform", "usage_profile", "default", "calculated"
}

// NewExplanation creates a new cost explanation
func NewExplanation(resource, costUnit string) *CostExplanation {
	return &CostExplanation{
		Resource:   resource,
		CostUnit:   costUnit,
		Inputs:     make([]Input, 0),
	}
}

// WithFormula sets the formula description
func (e *CostExplanation) WithFormula(formula string) *CostExplanation {
	e.Formula = formula
	return e
}

// AddInput adds an input to the explanation
func (e *CostExplanation) AddInput(name, value, source string) *CostExplanation {
	e.Inputs = append(e.Inputs, Input{
		Name:   name,
		Value:  value,
		Source: source,
	})
	return e
}

// WithUsageSource sets the usage data source
func (e *CostExplanation) WithUsageSource(source string) *CostExplanation {
	e.UsageSource = source
	return e
}

// WithConfidence sets the confidence with reason
func (e *CostExplanation) WithConfidence(confidence float64, reason string) *CostExplanation {
	e.Confidence = confidence
	e.ConfidenceReason = reason
	return e
}

// AddDependency adds a dependency resource address
func (e *CostExplanation) AddDependency(dep string) *CostExplanation {
	e.Dependencies = append(e.Dependencies, dep)
	return e
}

// AsSymbolic marks this as a symbolic cost with explanation
func (e *CostExplanation) AsSymbolic(reason string, chain []string) *CostExplanation {
	e.IsSymbolic = true
	e.SymbolicReason = reason
	e.SymbolicChain = chain
	e.Confidence = 0
	return e
}

// ToJSON returns JSON representation
func (e *CostExplanation) ToJSON() ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}

// ToHover returns a compact hover/tooltip format
func (e *CostExplanation) ToHover() string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("**%s** → %s\n", e.Resource, e.CostUnit))
	
	if e.IsSymbolic {
		sb.WriteString(fmt.Sprintf("⚠️ *Symbolic*: %s\n", e.SymbolicReason))
		if len(e.SymbolicChain) > 0 {
			sb.WriteString("Chain: " + strings.Join(e.SymbolicChain, " → ") + "\n")
		}
		return sb.String()
	}
	
	if e.Formula != "" {
		sb.WriteString(fmt.Sprintf("Formula: `%s`\n", e.Formula))
	}
	
	if len(e.Inputs) > 0 {
		sb.WriteString("Inputs:\n")
		for _, input := range e.Inputs {
			sb.WriteString(fmt.Sprintf("  • %s = %s (%s)\n", input.Name, input.Value, input.Source))
		}
	}
	
	sb.WriteString(fmt.Sprintf("Confidence: %.0f%%\n", e.Confidence*100))
	
	return sb.String()
}

// ToNarrative returns a human-readable narrative
func (e *CostExplanation) ToNarrative() string {
	if e.IsSymbolic {
		return fmt.Sprintf("Cost for %s is symbolic because: %s", e.Resource, e.SymbolicReason)
	}
	
	if e.Formula == "" {
		return fmt.Sprintf("%s costs are based on %s", e.Resource, e.CostUnit)
	}
	
	return fmt.Sprintf("%s %s cost calculated as: %s", e.Resource, e.CostUnit, e.Formula)
}
