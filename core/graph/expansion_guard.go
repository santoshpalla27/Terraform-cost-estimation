// Package graph - Hard expansion blocking
// Unknown cardinality NEVER expands. This is non-negotiable.
package graph

import (
	"fmt"
)

// ErrUnknownCardinality is returned when expansion is blocked
var ErrUnknownCardinality = fmt.Errorf("BLOCKED: unknown cardinality - cannot expand")

// CardinalityKind indicates cardinality knowledge state
type CardinalityKind int

const (
	CardinalityKnownKind   CardinalityKind = iota
	CardinalityUnknownKind
	CardinalityRangeKind
)

// String returns the kind name
func (k CardinalityKind) String() string {
	names := []string{"known", "unknown", "range"}
	if int(k) < len(names) {
		return names[k]
	}
	return "invalid"
}

// ExpansionGuard prevents expansion of unknown cardinality
type ExpansionGuard struct {
	strictMode bool
	blocked    []BlockedExpansion
}

// BlockedExpansion records a blocked expansion
type BlockedExpansion struct {
	Address     string
	Reason      string
	Cardinality CardinalityKind
}

// NewExpansionGuard creates a guard
func NewExpansionGuard(strictMode bool) *ExpansionGuard {
	return &ExpansionGuard{
		strictMode: strictMode,
		blocked:    []BlockedExpansion{},
	}
}

// MustExpand expands if cardinality is known, blocks otherwise
// In strict mode: panics
// In permissive mode: returns error
func (g *ExpansionGuard) MustExpand(address string, cardinality CardinalityKind, count int) ([]string, error) {
	if cardinality != CardinalityKnownKind {
		blocked := BlockedExpansion{
			Address:     address,
			Reason:      fmt.Sprintf("cardinality is %s", cardinality),
			Cardinality: cardinality,
		}
		g.blocked = append(g.blocked, blocked)

		if g.strictMode {
			panic(fmt.Sprintf("STRICT MODE: cannot expand %s - %s", address, blocked.Reason))
		}
		return nil, ErrUnknownCardinality
	}

	// Known cardinality - expand
	instances := make([]string, count)
	for i := 0; i < count; i++ {
		instances[i] = fmt.Sprintf("%s[%d]", address, i)
	}
	return instances, nil
}

// MustExpandForEach expands for_each if keys are known, blocks otherwise
func (g *ExpansionGuard) MustExpandForEach(address string, cardinality CardinalityKind, keys []string) ([]string, error) {
	if cardinality != CardinalityKnownKind {
		blocked := BlockedExpansion{
			Address:     address,
			Reason:      fmt.Sprintf("for_each cardinality is %s", cardinality),
			Cardinality: cardinality,
		}
		g.blocked = append(g.blocked, blocked)

		if g.strictMode {
			panic(fmt.Sprintf("STRICT MODE: cannot expand %s - %s", address, blocked.Reason))
		}
		return nil, ErrUnknownCardinality
	}

	// Known keys - expand
	instances := make([]string, len(keys))
	for i, key := range keys {
		instances[i] = fmt.Sprintf("%s[%q]", address, key)
	}
	return instances, nil
}

// GetBlocked returns all blocked expansions
func (g *ExpansionGuard) GetBlocked() []BlockedExpansion {
	return g.blocked
}

// HasBlocked returns true if any expansions were blocked
func (g *ExpansionGuard) HasBlocked() bool {
	return len(g.blocked) > 0
}

// IsStrictMode returns whether strict mode is enabled
func (g *ExpansionGuard) IsStrictMode() bool {
	return g.strictMode
}
