// Package graph_test - Invariant violation tests
// These tests INTENTIONALLY try to violate rules to ensure enforcement works.
package graph_test

import (
	"testing"

	"terraform-cost/core/graph"
)

// TestBypassAttempts verifies that bypass attempts panic
func TestBypassAttempts(t *testing.T) {
	tests := []struct {
		name     string
		fn       func()
		expected string
	}{
		{
			name: "NewCostUnitFromAssetDirect",
			fn: func() {
				graph.NewCostUnitFromAssetDirect("test-asset")
			},
			expected: "BYPASS BLOCKED",
		},
		{
			name: "NewCostGraphWithoutDepGraph",
			fn: func() {
				graph.NewCostGraphWithoutDepGraph()
			},
			expected: "BYPASS BLOCKED",
		},
		{
			name: "ExpandWithoutCardinalityCheck",
			fn: func() {
				graph.ExpandWithoutCardinalityCheck("aws_instance.test", 5)
			},
			expected: "BYPASS BLOCKED",
		},
		{
			name: "CreateNumericCostForUnknown",
			fn: func() {
				graph.CreateNumericCostForUnknown("aws_instance.test", 100.0)
			},
			expected: "BYPASS BLOCKED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("%s did not panic", tt.name)
					return
				}
				msg, ok := r.(string)
				if !ok {
					t.Errorf("%s panicked with non-string: %v", tt.name, r)
					return
				}
				if len(msg) < len(tt.expected) || msg[:len(tt.expected)] != tt.expected {
					t.Errorf("%s panicked with wrong message: %s", tt.name, msg)
				}
			}()
			tt.fn()
		})
	}
}

// TestUnsealedGraphPanics verifies unsealed graph panics
func TestUnsealedGraphPanics(t *testing.T) {
	g := graph.NewCanonicalDependencyGraph()
	g.AddNode(&graph.NodeMeta{
		ID:      "test",
		Address: "aws_instance.test",
	})
	// Don't seal

	defer func() {
		r := recover()
		if r == nil {
			t.Error("MustBeClosed did not panic on unsealed graph")
		}
	}()

	g.MustBeClosed()
}

// TestExpansionGuardStrict verifies strict mode panics on unknown cardinality
func TestExpansionGuardStrict(t *testing.T) {
	guard := graph.NewExpansionGuard(true) // strict mode

	defer func() {
		r := recover()
		if r == nil {
			t.Error("MustExpand did not panic on unknown cardinality in strict mode")
		}
	}()

	_, _ = guard.MustExpand("aws_instance.test", graph.CardinalityUnknownKind, 5)
}

// TestExpansionGuardPermissive verifies permissive mode returns error
func TestExpansionGuardPermissive(t *testing.T) {
	guard := graph.NewExpansionGuard(false) // permissive mode

	instances, err := guard.MustExpand("aws_instance.test", graph.CardinalityUnknownKind, 5)
	if err == nil {
		t.Error("MustExpand should return error on unknown cardinality")
	}
	if instances != nil {
		t.Error("MustExpand should return nil instances on unknown cardinality")
	}
	if !guard.HasBlocked() {
		t.Error("MustExpand should record blocked expansion")
	}
}

// TestInvariantCheckerStrict verifies strict checker panics
func TestInvariantCheckerStrict(t *testing.T) {
	checker := graph.NewInvariantChecker(true) // strict mode

	defer func() {
		r := recover()
		if r == nil {
			t.Error("AssertAssetHasDepNode did not panic on nil asset")
		}
	}()

	_ = checker.AssertAssetHasDepNode(nil)
}

// TestInvariantCheckerPermissive verifies permissive checker records violations
func TestInvariantCheckerPermissive(t *testing.T) {
	checker := graph.NewInvariantChecker(false) // permissive mode

	// Should not panic
	err := checker.AssertAssetHasDepNode(nil)
	if err == nil {
		t.Error("AssertAssetHasDepNode should return error on nil asset")
	}

	if !checker.HasViolations() {
		t.Error("Checker should record violation")
	}
}

// TestConfidencePessimistic verifies confidence aggregation is pessimistic
func TestConfidencePessimistic(t *testing.T) {
	checker := graph.NewInvariantChecker(true)

	// Valid: aggregate equals minimum
	err := checker.AssertConfidencePessimistic(0.5, []float64{0.9, 0.7, 0.5})
	if err != nil {
		t.Errorf("Valid pessimistic confidence rejected: %v", err)
	}

	// Invalid: aggregate exceeds minimum
	defer func() {
		r := recover()
		if r == nil {
			t.Error("AssertConfidencePessimistic did not panic on optimistic aggregation")
		}
	}()

	_ = checker.AssertConfidencePessimistic(0.8, []float64{0.9, 0.7, 0.5})
}
