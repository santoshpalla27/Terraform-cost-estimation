// Package graph - Enforced assets with dependency lineage
// Every Asset MUST reference a DependencyNodeID.
// This is not optional - assets without lineage cannot exist.
package graph

import (
	"fmt"
)

// EnforcedAsset is an asset that MUST have dependency lineage
type EnforcedAsset struct {
	// Identity
	AssetID string

	// REQUIRED: Link to canonical dependency graph
	DependencyNodeID DependencyNodeID

	// REQUIRED: Upstream dependencies (transitive closure)
	UpstreamDeps []DependencyNodeID

	// Asset metadata
	Address      string
	ResourceType string
	Provider     string
	Region       string

	// Expansion info
	IsExpanded   bool
	ExpandedFrom string
	InstanceKey  interface{}
}

// NewEnforcedAsset creates an asset with REQUIRED dependency linkage
// Panics if dependency graph doesn't contain the node
func NewEnforcedAsset(
	assetID string,
	depNodeID DependencyNodeID,
	graph *CanonicalDependencyGraph,
) *EnforcedAsset {
	// ASSERTION: Node MUST exist in canonical graph
	graph.ValidateNode(depNodeID)

	// Get dependency path
	upstreamDeps := graph.GetDependencyPath(depNodeID)

	return &EnforcedAsset{
		AssetID:          assetID,
		DependencyNodeID: depNodeID,
		UpstreamDeps:     upstreamDeps,
	}
}

// ValidateLineage ensures the asset has proper dependency lineage
func (a *EnforcedAsset) ValidateLineage() error {
	if a.DependencyNodeID == "" {
		return fmt.Errorf("INVARIANT VIOLATED: asset %s has no DependencyNodeID", a.AssetID)
	}
	return nil
}

// EnforcedAssetGraph is a graph of assets with enforced dependency lineage
type EnforcedAssetGraph struct {
	// Source canonical graph (REQUIRED)
	canonical *CanonicalDependencyGraph

	// Assets indexed by ID
	assets map[string]*EnforcedAsset

	// Mapping from DependencyNodeID to assets (one-to-many for expanded resources)
	nodeToAssets map[DependencyNodeID][]*EnforcedAsset
}

// NewEnforcedAssetGraph creates an asset graph from a canonical dependency graph
// The canonical graph is REQUIRED
func NewEnforcedAssetGraph(canonical *CanonicalDependencyGraph) (*EnforcedAssetGraph, error) {
	if canonical == nil {
		return nil, fmt.Errorf("INVARIANT VIOLATED: cannot create asset graph without canonical dependency graph")
	}
	if !canonical.IsSealed() {
		return nil, fmt.Errorf("INVARIANT VIOLATED: canonical graph must be sealed before creating asset graph")
	}

	return &EnforcedAssetGraph{
		canonical:    canonical,
		assets:       make(map[string]*EnforcedAsset),
		nodeToAssets: make(map[DependencyNodeID][]*EnforcedAsset),
	}, nil
}

// AddAsset adds an asset with dependency validation
func (g *EnforcedAssetGraph) AddAsset(asset *EnforcedAsset) error {
	// ASSERTION: Asset must have DependencyNodeID
	if asset.DependencyNodeID == "" {
		panic(fmt.Sprintf("INVARIANT VIOLATED: asset %s has no DependencyNodeID", asset.AssetID))
	}

	// ASSERTION: DependencyNodeID must exist in canonical graph
	g.canonical.ValidateNode(asset.DependencyNodeID)

	g.assets[asset.AssetID] = asset
	g.nodeToAssets[asset.DependencyNodeID] = append(g.nodeToAssets[asset.DependencyNodeID], asset)
	return nil
}

// GetAsset returns an asset by ID
func (g *EnforcedAssetGraph) GetAsset(assetID string) (*EnforcedAsset, bool) {
	asset, ok := g.assets[assetID]
	return asset, ok
}

// GetAssetsByNode returns all assets for a dependency node
func (g *EnforcedAssetGraph) GetAssetsByNode(nodeID DependencyNodeID) []*EnforcedAsset {
	return g.nodeToAssets[nodeID]
}

// GetAffectedAssets returns assets affected by changes to specified nodes
func (g *EnforcedAssetGraph) GetAffectedAssets(changedNodes []DependencyNodeID) []*EnforcedAsset {
	affected := make(map[string]*EnforcedAsset)

	for _, nodeID := range changedNodes {
		// Direct assets
		for _, asset := range g.nodeToAssets[nodeID] {
			affected[asset.AssetID] = asset
		}

		// Transitive dependents
		dependents := g.canonical.GetTransitiveDependents(nodeID)
		for _, depID := range dependents {
			for _, asset := range g.nodeToAssets[depID] {
				affected[asset.AssetID] = asset
			}
		}
	}

	result := make([]*EnforcedAsset, 0, len(affected))
	for _, asset := range affected {
		result = append(result, asset)
	}
	return result
}

// AllAssets returns all assets
func (g *EnforcedAssetGraph) AllAssets() []*EnforcedAsset {
	result := make([]*EnforcedAsset, 0, len(g.assets))
	for _, asset := range g.assets {
		result = append(result, asset)
	}
	return result
}

// GetCanonicalGraph returns the canonical dependency graph
func (g *EnforcedAssetGraph) GetCanonicalGraph() *CanonicalDependencyGraph {
	return g.canonical
}
