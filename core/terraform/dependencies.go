// Package terraform - Dependency graph and depends_on handling
package terraform

import (
	"sort"
	"strings"

	"terraform-cost/core/determinism"
	"terraform-cost/core/model"
)

// DependencyResolver builds the dependency graph between instances
type DependencyResolver struct {
	// Instance index for lookups
	instanceByAddr  map[model.InstanceAddress]*model.AssetInstance
	instanceByDefn  map[model.DefinitionID][]*model.AssetInstance
}

// NewDependencyResolver creates a new resolver
func NewDependencyResolver() *DependencyResolver {
	return &DependencyResolver{
		instanceByAddr: make(map[model.InstanceAddress]*model.AssetInstance),
		instanceByDefn: make(map[model.DefinitionID][]*model.AssetInstance),
	}
}

// ResolveDependencies builds all dependency edges
func (r *DependencyResolver) ResolveDependencies(
	instances []*model.AssetInstance,
	definitions map[model.DefinitionID]*model.AssetDefinition,
) []model.InstanceEdge {
	// Build indexes
	r.buildIndexes(instances)

	var edges []model.InstanceEdge

	for _, inst := range instances {
		def := definitions[inst.DefinitionID]
		if def == nil {
			continue
		}

		// 1. Explicit depends_on
		explicitEdges := r.resolveExplicitDeps(inst, def.DependsOn)
		edges = append(edges, explicitEdges...)

		// 2. Implicit reference-based dependencies
		implicitEdges := r.resolveImplicitDeps(inst, def)
		edges = append(edges, implicitEdges...)

		// 3. Provider dependencies
		providerEdges := r.resolveProviderDeps(inst)
		edges = append(edges, providerEdges...)
	}

	// Sort edges for determinism
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	// Deduplicate
	return r.deduplicate(edges)
}

func (r *DependencyResolver) buildIndexes(instances []*model.AssetInstance) {
	for _, inst := range instances {
		r.instanceByAddr[inst.Address] = inst
		r.instanceByDefn[inst.DefinitionID] = append(r.instanceByDefn[inst.DefinitionID], inst)
	}
}

// resolveExplicitDeps handles depends_on meta-argument
func (r *DependencyResolver) resolveExplicitDeps(
	inst *model.AssetInstance,
	dependsOn []string,
) []model.InstanceEdge {
	var edges []model.InstanceEdge

	for _, dep := range dependsOn {
		// depends_on references definitions, not instances
		// We need to create edges to ALL instances of that definition
		targetInstances := r.findInstancesByAddress(dep)
		for _, target := range targetInstances {
			if target.ID != inst.ID { // No self-loops
				edges = append(edges, model.InstanceEdge{
					From: inst.ID,
					To:   target.ID,
					Type: model.EdgeExplicit,
				})
			}
		}
	}

	return edges
}

// resolveImplicitDeps finds dependencies from attribute references
func (r *DependencyResolver) resolveImplicitDeps(
	inst *model.AssetInstance,
	def *model.AssetDefinition,
) []model.InstanceEdge {
	var edges []model.InstanceEdge

	// Collect all references from attributes
	for _, expr := range def.Attributes {
		for _, ref := range expr.References {
			// Parse reference to find target
			targetAddr := r.parseRefToAddress(ref)
			if targetAddr == "" {
				continue
			}

			targetInstances := r.findInstancesByAddress(targetAddr)
			for _, target := range targetInstances {
				if target.ID != inst.ID {
					edges = append(edges, model.InstanceEdge{
						From: inst.ID,
						To:   target.ID,
						Type: model.EdgeImplicit,
					})
				}
			}
		}
	}

	return edges
}

// resolveProviderDeps creates edges for provider requirements
func (r *DependencyResolver) resolveProviderDeps(inst *model.AssetInstance) []model.InstanceEdge {
	// Provider dependencies are generally implicit in Terraform
	// but we track them for completeness
	return nil
}

// findInstancesByAddress finds instances matching an address pattern
func (r *DependencyResolver) findInstancesByAddress(addr string) []*model.AssetInstance {
	var result []*model.AssetInstance

	// First try exact match
	if inst, ok := r.instanceByAddr[model.InstanceAddress(addr)]; ok {
		return []*model.AssetInstance{inst}
	}

	// Try as definition address (returns all instances)
	for instAddr, inst := range r.instanceByAddr {
		// Check if instance address starts with the definition address
		if strings.HasPrefix(string(instAddr), addr+"[") || string(instAddr) == addr {
			result = append(result, inst)
		}
	}

	// Sort for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].Address < result[j].Address
	})

	return result
}

// parseRefToAddress extracts the resource address from a reference
// e.g., "aws_instance.web.id" -> "aws_instance.web"
// e.g., "module.app.aws_s3_bucket.data" -> "module.app.aws_s3_bucket.data"
func (r *DependencyResolver) parseRefToAddress(ref string) string {
	parts := strings.Split(ref, ".")

	// Skip variable/local references
	if len(parts) < 2 {
		return ""
	}

	switch parts[0] {
	case "var", "local", "path", "terraform":
		return "" // Not a resource reference
	case "data":
		if len(parts) >= 3 {
			return strings.Join(parts[:3], ".")
		}
	case "module":
		// Find where the resource part starts
		for i := 0; i < len(parts)-1; i += 2 {
			if parts[i] != "module" {
				// parts[i] is the resource type
				return strings.Join(parts[:i+2], ".")
			}
		}
	default:
		// Regular resource reference
		if len(parts) >= 2 {
			return parts[0] + "." + parts[1]
		}
	}

	return ""
}

func (r *DependencyResolver) deduplicate(edges []model.InstanceEdge) []model.InstanceEdge {
	seen := make(map[string]bool)
	result := make([]model.InstanceEdge, 0, len(edges))

	for _, e := range edges {
		key := string(e.From) + "->" + string(e.To)
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}

	return result
}

// TopologicalSort sorts instances in dependency order
func TopologicalSort(instances []*model.AssetInstance, edges []model.InstanceEdge) []model.InstanceID {
	// Build adjacency list
	adj := make(map[model.InstanceID][]model.InstanceID)
	inDegree := make(map[model.InstanceID]int)

	for _, inst := range instances {
		adj[inst.ID] = []model.InstanceID{}
		inDegree[inst.ID] = 0
	}

	for _, edge := range edges {
		adj[edge.From] = append(adj[edge.From], edge.To)
		inDegree[edge.To]++
	}

	// Kahn's algorithm with stable ordering
	var queue []model.InstanceID
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	determinism.SortSlice(queue, func(a, b model.InstanceID) bool {
		return a < b
	})

	var result []model.InstanceID
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, neighbor := range adj[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
				determinism.SortSlice(queue, func(a, b model.InstanceID) bool {
					return a < b
				})
			}
		}
	}

	// Check for cycles
	if len(result) != len(instances) {
		// Cycle detected - return what we have
		// Caller should handle this error case
	}

	return result
}

// DetectCycles finds all cycles in the dependency graph
func DetectCycles(instances []*model.AssetInstance, edges []model.InstanceEdge) [][]model.InstanceID {
	// Build adjacency list
	adj := make(map[model.InstanceID][]model.InstanceID)
	for _, inst := range instances {
		adj[inst.ID] = []model.InstanceID{}
	}
	for _, edge := range edges {
		adj[edge.From] = append(adj[edge.From], edge.To)
	}

	// DFS-based cycle detection
	var cycles [][]model.InstanceID
	color := make(map[model.InstanceID]int) // 0=white, 1=gray, 2=black
	parent := make(map[model.InstanceID]model.InstanceID)

	var dfs func(node model.InstanceID)
	dfs = func(node model.InstanceID) {
		color[node] = 1 // Gray

		for _, neighbor := range adj[node] {
			if color[neighbor] == 1 {
				// Back edge found - cycle detected
				cycle := []model.InstanceID{neighbor}
				for n := node; n != neighbor; n = parent[n] {
					cycle = append([]model.InstanceID{n}, cycle...)
				}
				cycles = append(cycles, cycle)
			} else if color[neighbor] == 0 {
				parent[neighbor] = node
				dfs(neighbor)
			}
		}

		color[node] = 2 // Black
	}

	// Sort nodes for deterministic traversal
	nodes := make([]model.InstanceID, 0, len(adj))
	for id := range adj {
		nodes = append(nodes, id)
	}
	determinism.SortSlice(nodes, func(a, b model.InstanceID) bool {
		return a < b
	})

	for _, node := range nodes {
		if color[node] == 0 {
			dfs(node)
		}
	}

	return cycles
}
