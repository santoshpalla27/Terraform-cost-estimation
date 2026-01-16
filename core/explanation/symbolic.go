// Package explanation - Symbolic cost explanations
// Explains why costs cannot be computed
package explanation

import "strings"

// SymbolicExplanation explains why a cost is symbolic
type SymbolicExplanation struct {
	Resource    string   `json:"resource"`
	Reason      string   `json:"reason"`
	Chain       []string `json:"chain,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// NewSymbolicExplanation creates a symbolic explanation
func NewSymbolicExplanation(resource, reason string) *SymbolicExplanation {
	return &SymbolicExplanation{
		Resource: resource,
		Reason:   reason,
	}
}

// WithChain adds the dependency chain that led to symbolic
func (s *SymbolicExplanation) WithChain(chain ...string) *SymbolicExplanation {
	s.Chain = chain
	return s
}

// WithSuggestion adds a suggestion for resolution
func (s *SymbolicExplanation) WithSuggestion(suggestion string) *SymbolicExplanation {
	s.Suggestions = append(s.Suggestions, suggestion)
	return s
}

// ToMarkdown returns a markdown explanation
func (s *SymbolicExplanation) ToMarkdown() string {
	var sb strings.Builder
	
	sb.WriteString("### Cost is symbolic\n\n")
	sb.WriteString("**Resource**: `" + s.Resource + "`\n\n")
	sb.WriteString("**Reason**: " + s.Reason + "\n\n")
	
	if len(s.Chain) > 0 {
		sb.WriteString("**Dependency chain**:\n")
		for i, step := range s.Chain {
			sb.WriteString("  " + strings.Repeat("  ", i) + "→ " + step + "\n")
		}
		sb.WriteString("\n")
	}
	
	if len(s.Suggestions) > 0 {
		sb.WriteString("**To resolve**:\n")
		for _, suggestion := range s.Suggestions {
			sb.WriteString("- " + suggestion + "\n")
		}
	}
	
	return sb.String()
}

// Common symbolic reasons
const (
	ReasonUnknownCardinality = "instance count depends on module output that cannot be resolved at plan time"
	ReasonUnknownUsage       = "usage data not provided (e.g., requests/month, storage size)"
	ReasonDynamicValue       = "value is computed dynamically and cannot be determined statically"
	ReasonUnsupportedResource = "resource type is not yet supported"
	ReasonMissingAttribute   = "required attribute is not set or uses a dynamic reference"
	ReasonForEachUnknown     = "for_each expression depends on unknown value"
	ReasonCountUnknown       = "count expression depends on unknown value"
)

// StandardExplanation creates a standard symbolic explanation
func StandardExplanation(resource, reasonType string) *SymbolicExplanation {
	exp := NewSymbolicExplanation(resource, reasonType)
	
	switch reasonType {
	case ReasonUnknownCardinality:
		exp.WithSuggestion("Use a usage profile to provide expected instance counts")
		exp.WithSuggestion("Refactor to use static count or for_each with known keys")
		
	case ReasonUnknownUsage:
		exp.WithSuggestion("Provide a usage profile with expected usage metrics")
		exp.WithSuggestion("Add usage annotations to your Terraform configuration")
		
	case ReasonUnsupportedResource:
		exp.WithSuggestion("Check SUPPORTED_SERVICES.md for supported resource types")
		exp.WithSuggestion("Request support for this resource type")
	}
	
	return exp
}
