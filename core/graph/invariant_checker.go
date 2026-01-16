// Package graph - Invariant assertions and tests
// These assertions verify correctness at boundaries.
// They intentionally try to violate rules to ensure enforcement works.
package graph

import (
	"fmt"
)

// InvariantViolation represents a detected invariant violation
type InvariantViolation struct {
	Invariant string
	Location  string
	Details   string
}

func (v *InvariantViolation) Error() string {
	return fmt.Sprintf("INVARIANT VIOLATED [%s] at %s: %s", v.Invariant, v.Location, v.Details)
}

// InvariantChecker verifies all architectural invariants
type InvariantChecker struct {
	violations []InvariantViolation
	strictMode bool
}

// NewInvariantChecker creates a checker
func NewInvariantChecker(strictMode bool) *InvariantChecker {
	return &InvariantChecker{
		violations: []InvariantViolation{},
		strictMode: strictMode,
	}
}

// AssertDepGraphClosed asserts dependency graph is closed
func (c *InvariantChecker) AssertDepGraphClosed(g *CanonicalDependencyGraph) error {
	if g == nil {
		return c.fail("DEP_GRAPH_EXISTS", "AssertDepGraphClosed", "dependency graph is nil")
	}
	if !g.IsSealed() {
		return c.fail("DEP_GRAPH_SEALED", "AssertDepGraphClosed", "dependency graph not sealed")
	}
	if !g.IsTransitivelyClosed() {
		return c.fail("DEP_GRAPH_CLOSED", "AssertDepGraphClosed", "dependency graph not transitively closed")
	}
	return nil
}

// AssertAssetHasDepNode asserts asset has a dependency node
func (c *InvariantChecker) AssertAssetHasDepNode(asset *EnforcedAsset) error {
	if asset == nil {
		return c.fail("ASSET_EXISTS", "AssertAssetHasDepNode", "asset is nil")
	}
	if asset.DependencyNodeID == "" {
		return c.fail("ASSET_HAS_DEP_NODE", "AssertAssetHasDepNode", 
			fmt.Sprintf("asset %s has no DependencyNodeID", asset.AssetID))
	}
	return nil
}

// AssertCostUnitHasPath asserts cost unit has dependency path
func (c *InvariantChecker) AssertCostUnitHasPath(unit *EnforcedCostUnit) error {
	if unit == nil {
		return c.fail("COST_UNIT_EXISTS", "AssertCostUnitHasPath", "cost unit is nil")
	}
	if len(unit.DependencyPath) == 0 {
		return c.fail("COST_UNIT_HAS_PATH", "AssertCostUnitHasPath",
			fmt.Sprintf("cost unit %s has no dependency path", unit.CostUnitID))
	}
	return nil
}

// AssertNoNumericCostForUnknown asserts unknown cardinality has no numeric cost
func (c *InvariantChecker) AssertNoNumericCostForUnknown(unit *EnforcedCostUnit) error {
	if unit == nil {
		return nil
	}
	if unit.IsSymbolic {
		// Symbolic costs must not have numeric values
		if !unit.MonthlyCost.IsZero() {
			return c.fail("NO_NUMERIC_FOR_UNKNOWN", "AssertNoNumericCostForUnknown",
				fmt.Sprintf("symbolic cost unit %s has non-zero monthly cost", unit.CostUnitID))
		}
	}
	return nil
}

// AssertCardinalityKnownForExpansion asserts cardinality is known before expansion
func (c *InvariantChecker) AssertCardinalityKnownForExpansion(address string, cardinality CardinalityKind) error {
	if cardinality != CardinalityKnownKind {
		return c.fail("CARDINALITY_KNOWN", "AssertCardinalityKnownForExpansion",
			fmt.Sprintf("cannot expand %s with %s cardinality", address, cardinality))
	}
	return nil
}

// AssertConfidencePessimistic asserts confidence is pessimistic (MIN)
func (c *InvariantChecker) AssertConfidencePessimistic(aggregate float64, components []float64) error {
	if len(components) == 0 {
		return nil
	}
	min := 1.0
	for _, v := range components {
		if v < min {
			min = v
		}
	}
	if aggregate > min {
		return c.fail("CONFIDENCE_PESSIMISTIC", "AssertConfidencePessimistic",
			fmt.Sprintf("aggregate confidence %.2f exceeds minimum component %.2f", aggregate, min))
	}
	return nil
}

func (c *InvariantChecker) fail(invariant, location, details string) error {
	v := InvariantViolation{
		Invariant: invariant,
		Location:  location,
		Details:   details,
	}
	c.violations = append(c.violations, v)
	
	if c.strictMode {
		panic(v.Error())
	}
	return &v
}

// GetViolations returns all recorded violations
func (c *InvariantChecker) GetViolations() []InvariantViolation {
	return c.violations
}

// HasViolations returns true if any violations occurred
func (c *InvariantChecker) HasViolations() bool {
	return len(c.violations) > 0
}

// RunFullCheck runs all invariant checks on a cost graph
func (c *InvariantChecker) RunFullCheck(costGraph *EnforcedCostGraph) error {
	if costGraph == nil {
		return c.fail("COST_GRAPH_EXISTS", "RunFullCheck", "cost graph is nil")
	}

	for _, unit := range costGraph.AllCostUnits() {
		if err := c.AssertCostUnitHasPath(unit); err != nil && c.strictMode {
			return err
		}
		if err := c.AssertNoNumericCostForUnknown(unit); err != nil && c.strictMode {
			return err
		}
	}

	// Check pessimistic confidence
	confidences := []float64{}
	for _, unit := range costGraph.AllCostUnits() {
		confidences = append(confidences, unit.Confidence)
	}
	aggregate := costGraph.GetMinConfidence()
	if err := c.AssertConfidencePessimistic(aggregate, confidences); err != nil && c.strictMode {
		return err
	}

	return nil
}

// BlockBypassAttempt blocks any attempt to bypass dependency semantics
func BlockBypassAttempt(description string) {
	panic(fmt.Sprintf("BYPASS BLOCKED: %s - cost logic must go through dependency graph", description))
}
