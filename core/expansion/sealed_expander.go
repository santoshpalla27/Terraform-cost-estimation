// Package expansion - Sealed expansion that NEVER creates instances for unknown cardinality
// This file replaces the permissive behavior with strict epistemic honesty.
package expansion

import (
	"fmt"
)

// ExpansionOutcome represents what happened during expansion
type ExpansionOutcome int

const (
	OutcomeExpanded ExpansionOutcome = iota // Known cardinality, instances created
	OutcomeSymbolic                         // Unknown cardinality, symbolic bucket created
	OutcomeBlocked                          // Strict mode, expansion blocked
	OutcomeEmpty                            // count = 0 or empty for_each
)

// SealedExpansionResult is the result of sealed expansion
type SealedExpansionResult struct {
	Outcome ExpansionOutcome

	// Only set for OutcomeExpanded
	Instances []*AssetInstance

	// Only set for OutcomeSymbolic
	SymbolicReason     string
	SymbolicExpression string

	// Only set for OutcomeBlocked
	BlockedError error

	// Warnings
	Warnings []string
}

// SealedExpander is an expander that NEVER creates instances for unknown cardinality
type SealedExpander struct {
	strictMode bool
	// DefaultCountOnUnknown is REMOVED - unknown means unknown
}

// NewSealedExpander creates a sealed expander
func NewSealedExpander(strictMode bool) *SealedExpander {
	return &SealedExpander{
		strictMode: strictMode,
	}
}

// ExpandSealed expands an asset with sealed semantics
// Unknown cardinality NEVER produces instances
func (e *SealedExpander) ExpandSealed(asset *AssetInstance) *SealedExpansionResult {
	// This is already an instance - just return it
	return &SealedExpansionResult{
		Outcome:   OutcomeExpanded,
		Instances: []*AssetInstance{asset},
	}
}

// TryExpandCount attempts count expansion with strict semantics
func (e *SealedExpander) TryExpandCount(address string, count interface{}, isKnown bool) *SealedExpansionResult {
	if !isKnown {
		// UNKNOWN CARDINALITY - NO EXPANSION
		if e.strictMode {
			return &SealedExpansionResult{
				Outcome:      OutcomeBlocked,
				BlockedError: fmt.Errorf("STRICT: cannot expand %s - count is unknown", address),
			}
		}

		// Permissive: symbolic bucket, NOT instances
		return &SealedExpansionResult{
			Outcome:            OutcomeSymbolic,
			SymbolicReason:     "count is unknown",
			SymbolicExpression: "count = <unknown>",
			Warnings:           []string{fmt.Sprintf("%s: count cannot be determined pre-apply", address)},
		}
	}

	// Known cardinality
	countVal, ok := count.(int)
	if !ok {
		if f, fok := count.(float64); fok {
			countVal = int(f)
		} else {
			// Cannot convert - treat as unknown
			if e.strictMode {
				return &SealedExpansionResult{
					Outcome:      OutcomeBlocked,
					BlockedError: fmt.Errorf("STRICT: cannot expand %s - count type invalid", address),
				}
			}
			return &SealedExpansionResult{
				Outcome:        OutcomeSymbolic,
				SymbolicReason: "count type cannot be converted",
			}
		}
	}

	if countVal == 0 {
		return &SealedExpansionResult{
			Outcome: OutcomeEmpty,
		}
	}

	// Create instances
	instances := make([]*AssetInstance, countVal)
	for i := 0; i < countVal; i++ {
		instances[i] = &AssetInstance{
			Key: InstanceKey{Type: KeyTypeInt, NumValue: i},
			Metadata: InstanceMetadata{
				ExpansionType: ExpansionCount,
				IsKnown:       true,
			},
		}
	}

	return &SealedExpansionResult{
		Outcome:   OutcomeExpanded,
		Instances: instances,
	}
}

// TryExpandForEach attempts for_each expansion with strict semantics
func (e *SealedExpander) TryExpandForEach(address string, keys []string, isKnown bool) *SealedExpansionResult {
	if !isKnown {
		// UNKNOWN CARDINALITY - NO EXPANSION
		if e.strictMode {
			return &SealedExpansionResult{
				Outcome:      OutcomeBlocked,
				BlockedError: fmt.Errorf("STRICT: cannot expand %s - for_each is unknown", address),
			}
		}

		// Permissive: symbolic bucket, NOT instances
		return &SealedExpansionResult{
			Outcome:            OutcomeSymbolic,
			SymbolicReason:     "for_each is unknown",
			SymbolicExpression: "for_each = <unknown>",
			Warnings:           []string{fmt.Sprintf("%s: for_each cannot be determined pre-apply", address)},
		}
	}

	if len(keys) == 0 {
		return &SealedExpansionResult{
			Outcome: OutcomeEmpty,
		}
	}

	// Create instances
	instances := make([]*AssetInstance, len(keys))
	for i, key := range keys {
		instances[i] = &AssetInstance{
			Key: InstanceKey{Type: KeyTypeString, StrValue: key},
			Metadata: InstanceMetadata{
				ExpansionType: ExpansionForEach,
				IsKnown:       true,
			},
		}
	}

	return &SealedExpansionResult{
		Outcome:   OutcomeExpanded,
		Instances: instances,
	}
}

// HasInstances returns true if expansion produced instances
func (r *SealedExpansionResult) HasInstances() bool {
	return r.Outcome == OutcomeExpanded && len(r.Instances) > 0
}

// IsSymbolic returns true if expansion resulted in symbolic bucket
func (r *SealedExpansionResult) IsSymbolic() bool {
	return r.Outcome == OutcomeSymbolic
}

// IsBlocked returns true if expansion was blocked
func (r *SealedExpansionResult) IsBlocked() bool {
	return r.Outcome == OutcomeBlocked
}

// BLOCKED: Legacy permissive functions that created instances for unknown

// DefaultCountOnUnknown is REMOVED - this was epistemically dishonest
var DefaultCountOnUnknown = func() {
	panic("REMOVED: DefaultCountOnUnknown - unknown cardinality cannot have a default")
}

// AssumeOneInstance is REMOVED - assuming 1 for unknown is a lie
var AssumeOneInstance = func() {
	panic("REMOVED: AssumeOneInstance - unknown cardinality must use SymbolicCost")
}

// ExpandWithWarning is REMOVED - warnings don't make lies true
var ExpandWithWarning = func() {
	panic("REMOVED: ExpandWithWarning - unknown cardinality must block or emit SymbolicCost")
}
