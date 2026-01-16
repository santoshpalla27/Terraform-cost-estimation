// Package engine - Sealed cost graph construction
// This file SEALS the cost graph construction path.
// There is ONE way to build a cost graph - no exceptions.
package engine

import (
	"fmt"

	"terraform-cost/core/graph"
)

// SealedCostGraphBuilder is the ONLY way to build a cost graph
// All other paths are BLOCKED at compile time by making this the only export
type SealedCostGraphBuilder struct {
	depGraph   *graph.CanonicalDependencyGraph
	assetGraph *graph.EnforcedAssetGraph
	validated  bool
}

// NewSealedCostGraphBuilder creates a builder with REQUIRED dependency graph
// Panics immediately if dependency graph is not ready
func NewSealedCostGraphBuilder(
	depGraph *graph.CanonicalDependencyGraph,
	assetGraph *graph.EnforcedAssetGraph,
) *SealedCostGraphBuilder {
	// INVARIANT: depGraph is required
	if depGraph == nil {
		panic("SEALED: cannot build cost graph - depGraph is nil")
	}

	// INVARIANT: depGraph must be sealed
	if !depGraph.IsSealed() {
		panic("SEALED: cannot build cost graph - depGraph is not sealed")
	}

	// INVARIANT: depGraph must be transitively closed
	if !depGraph.IsTransitivelyClosed() {
		panic("SEALED: cannot build cost graph - dependency graph not transitively closed")
	}

	// INVARIANT: assetGraph is required
	if assetGraph == nil {
		panic("SEALED: cannot build cost graph - assetGraph is nil")
	}

	return &SealedCostGraphBuilder{
		depGraph:   depGraph,
		assetGraph: assetGraph,
		validated:  true,
	}
}

// Build builds the cost graph - only succeeds if all invariants hold
func (b *SealedCostGraphBuilder) Build() (*graph.AuthoritativeCostGraph, error) {
	if !b.validated {
		return nil, fmt.Errorf("SEALED: builder not properly initialized")
	}

	return graph.BuildAuthoritativeCostGraph(b.depGraph, b.assetGraph)
}

// BLOCKED PATHS - These exist only to provide clear error messages

// BuildCostGraphWithoutDependencies is BLOCKED
func BuildCostGraphWithoutDependencies() {
	panic("SEALED: BuildCostGraphWithoutDependencies - use NewSealedCostGraphBuilder")
}

// BuildCostGraphFromExpandedInstances is BLOCKED
func BuildCostGraphFromExpandedInstances() {
	panic("SEALED: BuildCostGraphFromExpandedInstances - use NewSealedCostGraphBuilder")
}

// BuildCostGraphFromAssetList is BLOCKED
func BuildCostGraphFromAssetList() {
	panic("SEALED: BuildCostGraphFromAssetList - use NewSealedCostGraphBuilder")
}

// AcceptInstancesWithoutDepGraph is BLOCKED
func AcceptInstancesWithoutDepGraph() {
	panic("SEALED: AcceptInstancesWithoutDepGraph - instances require dependency lineage")
}
