// Package graph - Canonical dependency graph
// This is THE authoritative source of truth for all dependencies.
// Everything downstream MUST reference this graph.
package graph

// DependencyNodeID uniquely identifies a node in the dependency graph
type DependencyNodeID string

// EdgeType indicates the type of dependency edge
type EdgeType int

const (
	EdgeReference    EdgeType = iota // Expression reference (e.g., aws_instance.web.id)
	EdgeDependsOn                    // Explicit depends_on
	EdgeModuleInput                  // Module input variable
	EdgeModuleOutput                 // Module output reference
	EdgeProviderBinding              // Provider configuration binding
	EdgeDataSource                   // Data source dependency
)

// String returns edge type name
func (t EdgeType) String() string {
	names := []string{"reference", "depends_on", "module_input", "module_output", "provider_binding", "data_source"}
	if int(t) < len(names) {
		return names[t]
	}
	return "unknown"
}

// DependencyEdge represents a directed edge in the dependency graph
type DependencyEdge struct {
	From      DependencyNodeID
	To        DependencyNodeID
	Type      EdgeType
	Attribute string // Which attribute caused this edge (for reference edges)
}

// NodeType indicates what kind of node this is
type CanonicalNodeType int

const (
	CanonicalResource   CanonicalNodeType = iota // resource block
	CanonicalData                                 // data source
	CanonicalModule                               // module call
	CanonicalProvider                             // provider configuration
	CanonicalVariable                             // input variable
	CanonicalLocal                                // local value
	CanonicalOutput                               // output value
)

// NodeMeta contains metadata about a dependency node
type NodeMeta struct {
	ID           DependencyNodeID
	Type         CanonicalNodeType
	Address      string // Terraform address (e.g., aws_instance.web)
	ModulePath   string // Module path (empty for root)
	ResourceType string // For resources/data: aws_instance
	Provider     string // Provider key
	SourceFile   string
	SourceLine   int
}

// CanonicalDependencyGraph is THE authoritative dependency graph
// All downstream systems MUST derive from this graph.
type CanonicalDependencyGraph struct {
	// Nodes indexed by ID
	nodes map[DependencyNodeID]*NodeMeta

	// Forward edges (from → to)
	edges map[DependencyNodeID][]DependencyEdge

	// Reverse edges (to → from) for upstream lookups
	reverseEdges map[DependencyNodeID][]DependencyEdge

	// Root nodes (no incoming edges)
	roots []DependencyNodeID

	// Sealed flag - no modifications after sealing
	sealed bool
}

// NewCanonicalDependencyGraph creates a new graph
func NewCanonicalDependencyGraph() *CanonicalDependencyGraph {
	return &CanonicalDependencyGraph{
		nodes:        make(map[DependencyNodeID]*NodeMeta),
		edges:        make(map[DependencyNodeID][]DependencyEdge),
		reverseEdges: make(map[DependencyNodeID][]DependencyEdge),
		roots:        []DependencyNodeID{},
		sealed:       false,
	}
}

// AddNode adds a node to the graph
func (g *CanonicalDependencyGraph) AddNode(meta *NodeMeta) {
	if g.sealed {
		panic("INVARIANT VIOLATED: cannot modify sealed dependency graph")
	}
	g.nodes[meta.ID] = meta
}

// AddEdge adds an edge to the graph
func (g *CanonicalDependencyGraph) AddEdge(edge DependencyEdge) {
	if g.sealed {
		panic("INVARIANT VIOLATED: cannot modify sealed dependency graph")
	}
	
	// Validate nodes exist
	if _, ok := g.nodes[edge.From]; !ok {
		panic("INVARIANT VIOLATED: edge from non-existent node: " + string(edge.From))
	}
	if _, ok := g.nodes[edge.To]; !ok {
		panic("INVARIANT VIOLATED: edge to non-existent node: " + string(edge.To))
	}

	g.edges[edge.From] = append(g.edges[edge.From], edge)
	g.reverseEdges[edge.To] = append(g.reverseEdges[edge.To], edge)
}

// Seal seals the graph - no more modifications allowed
func (g *CanonicalDependencyGraph) Seal() {
	// Compute roots
	g.roots = []DependencyNodeID{}
	for id := range g.nodes {
		if len(g.reverseEdges[id]) == 0 {
			g.roots = append(g.roots, id)
		}
	}
	g.sealed = true
}

// IsSealed returns whether the graph is sealed
func (g *CanonicalDependencyGraph) IsSealed() bool {
	return g.sealed
}

// IsTransitivelyClosed checks if graph is transitively closed
// A graph is closed if all referenced nodes exist
func (g *CanonicalDependencyGraph) IsTransitivelyClosed() bool {
	for _, edges := range g.edges {
		for _, edge := range edges {
			if _, ok := g.nodes[edge.To]; !ok {
				return false
			}
		}
	}
	return true
}

// MustBeClosed panics if graph is not closed
// CALL THIS BEFORE ASSET EXPANSION
func (g *CanonicalDependencyGraph) MustBeClosed() {
	if !g.sealed {
		panic("INVARIANT VIOLATED: dependency graph must be sealed before use")
	}
	if !g.IsTransitivelyClosed() {
		panic("INVARIANT VIOLATED: dependency graph is not transitively closed")
	}
}

// GetNode returns a node by ID
func (g *CanonicalDependencyGraph) GetNode(id DependencyNodeID) (*NodeMeta, bool) {
	node, ok := g.nodes[id]
	return node, ok
}

// MustGetNode returns a node or panics
func (g *CanonicalDependencyGraph) MustGetNode(id DependencyNodeID) *NodeMeta {
	node, ok := g.nodes[id]
	if !ok {
		panic("INVARIANT VIOLATED: node not found: " + string(id))
	}
	return node
}

// GetDependencies returns direct dependencies of a node
func (g *CanonicalDependencyGraph) GetDependencies(id DependencyNodeID) []DependencyEdge {
	return g.edges[id]
}

// GetDependents returns nodes that depend on this node
func (g *CanonicalDependencyGraph) GetDependents(id DependencyNodeID) []DependencyEdge {
	return g.reverseEdges[id]
}

// GetDependencyPath returns the transitive closure from roots to this node
// This is REQUIRED for CostUnit lineage
func (g *CanonicalDependencyGraph) GetDependencyPath(id DependencyNodeID) []DependencyNodeID {
	if !g.sealed {
		panic("INVARIANT VIOLATED: cannot compute dependency path on unsealed graph")
	}

	visited := make(map[DependencyNodeID]bool)
	path := []DependencyNodeID{}
	g.collectUpstream(id, visited, &path)
	
	// Reverse to get root → target order
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	
	return path
}

func (g *CanonicalDependencyGraph) collectUpstream(id DependencyNodeID, visited map[DependencyNodeID]bool, path *[]DependencyNodeID) {
	if visited[id] {
		return
	}
	visited[id] = true

	// First collect upstream
	for _, edge := range g.reverseEdges[id] {
		g.collectUpstream(edge.From, visited, path)
	}

	// Then add self
	*path = append(*path, id)
}

// GetTransitiveDependents returns all nodes affected by changes to this node
func (g *CanonicalDependencyGraph) GetTransitiveDependents(id DependencyNodeID) []DependencyNodeID {
	if !g.sealed {
		panic("INVARIANT VIOLATED: cannot compute transitive dependents on unsealed graph")
	}

	visited := make(map[DependencyNodeID]bool)
	result := []DependencyNodeID{}
	g.collectDownstream(id, visited, &result)
	return result
}

func (g *CanonicalDependencyGraph) collectDownstream(id DependencyNodeID, visited map[DependencyNodeID]bool, result *[]DependencyNodeID) {
	for _, edge := range g.edges[id] {
		if !visited[edge.To] {
			visited[edge.To] = true
			*result = append(*result, edge.To)
			g.collectDownstream(edge.To, visited, result)
		}
	}
}

// GetRoots returns root nodes
func (g *CanonicalDependencyGraph) GetRoots() []DependencyNodeID {
	return g.roots
}

// Size returns node count
func (g *CanonicalDependencyGraph) Size() int {
	return len(g.nodes)
}

// EdgeCount returns edge count
func (g *CanonicalDependencyGraph) EdgeCount() int {
	count := 0
	for _, edges := range g.edges {
		count += len(edges)
	}
	return count
}

// AllNodes returns all nodes
func (g *CanonicalDependencyGraph) AllNodes() map[DependencyNodeID]*NodeMeta {
	return g.nodes
}

// ValidateNode asserts a node exists
func (g *CanonicalDependencyGraph) ValidateNode(id DependencyNodeID) {
	if _, ok := g.nodes[id]; !ok {
		panic("INVARIANT VIOLATED: node not in canonical dependency graph: " + string(id))
	}
}
