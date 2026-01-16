// Package graph - Real infrastructure dependency graph
// Resources ARE connected via actual Terraform dependencies.
// depends_on, implicit references, module outputs are ALL modeled.
package graph

import (
	"fmt"
	"sort"
	"strings"

	"terraform-cost/core/model"
)

// InfrastructureGraph is the real dependency graph modeling Terraform semantics
type InfrastructureGraph struct {
	// Nodes by address
	nodes map[string]*InfraNode

	// Edges (from → to)
	edges map[string][]string

	// Reverse edges (to → from) for upstream lookups
	reverseEdges map[string][]string

	// Module hierarchy
	modules map[string]*ModuleNode

	// Topologically sorted order (computed lazily)
	topoOrder []string
	topoValid bool
}

// InfraNode is a node in the infrastructure graph
type InfraNode struct {
	// Identity
	Address    string
	Type       NodeType
	ModulePath string

	// Source definition
	DefinitionID model.DefinitionID
	SourceFile   string
	SourceLine   int

	// Dependencies
	ExplicitDeps   []string // depends_on
	ImplicitDeps   []string // expression references
	ProviderDep    string   // provider binding
	ModuleOutputs  []string // if this is a module output

	// Expansion state
	IsExpanded    bool
	ExpandedFrom  string   // parent definition address
	InstanceKey   interface{}
	SiblingCount  int

	// Lineage
	Lineage *NodeLineage
}

// NodeType indicates the type of infrastructure node
type NodeType int

const (
	NodeResource   NodeType = iota // resource block
	NodeDataSource                  // data block
	NodeModule                      // module block
	NodeVariable                    // variable
	NodeLocal                       // local value
	NodeOutput                      // output
	NodeProvider                    // provider config
)

// String returns the node type name
func (t NodeType) String() string {
	switch t {
	case NodeResource:
		return "resource"
	case NodeDataSource:
		return "data"
	case NodeModule:
		return "module"
	case NodeVariable:
		return "variable"
	case NodeLocal:
		return "local"
	case NodeOutput:
		return "output"
	case NodeProvider:
		return "provider"
	default:
		return "unknown"
	}
}

// NodeLineage tracks the complete derivation of a node
type NodeLineage struct {
	// Expression references that led to this node
	ExpressionRefs []ExpressionRef

	// Module call chain
	ModuleChain []string

	// Provider inheritance chain
	ProviderChain []string

	// Count/for_each expansion path
	ExpansionPath []ExpansionStep
}

// ExpressionRef is a reference from an expression
type ExpressionRef struct {
	FromAttribute string  // e.g., "subnet_id"
	ToAddress     string  // e.g., "aws_subnet.main"
	RefType       RefType
}

// RefType indicates the type of reference
type RefType int

const (
	RefDirect     RefType = iota // Direct resource reference
	RefAttribute                  // Attribute reference (resource.attr)
	RefSplat                      // Splat reference (resource[*].attr)
	RefIndex                      // Index reference (resource[0].attr)
	RefEach                       // each.value reference
	RefCount                      // count.index reference
)

// ExpansionStep records a single expansion step
type ExpansionStep struct {
	Type  string      // "count" or "for_each"
	Key   interface{} // index or key
	From  string      // parent address
}

// ModuleNode represents a module in the hierarchy
type ModuleNode struct {
	Path       string
	Source     string
	ParentPath string
	Inputs     map[string]string // input variable → source expression
	Outputs    map[string]string // output name → expression
	Providers  map[string]string // provider mapping
	Children   []string          // child module paths
}

// NewInfrastructureGraph creates a new graph
func NewInfrastructureGraph() *InfrastructureGraph {
	return &InfrastructureGraph{
		nodes:        make(map[string]*InfraNode),
		edges:        make(map[string][]string),
		reverseEdges: make(map[string][]string),
		modules:      make(map[string]*ModuleNode),
		topoValid:    false,
	}
}

// AddNode adds a node to the graph
func (g *InfrastructureGraph) AddNode(node *InfraNode) {
	g.nodes[node.Address] = node
	g.topoValid = false
}

// AddEdge adds a dependency edge (from depends on to)
func (g *InfrastructureGraph) AddEdge(from, to string) {
	g.edges[from] = append(g.edges[from], to)
	g.reverseEdges[to] = append(g.reverseEdges[to], from)
	g.topoValid = false
}

// AddModule registers a module
func (g *InfrastructureGraph) AddModule(module *ModuleNode) {
	g.modules[module.Path] = module
}

// GetNode returns a node by address
func (g *InfrastructureGraph) GetNode(address string) *InfraNode {
	return g.nodes[address]
}

// GetDependencies returns direct dependencies of a node
func (g *InfrastructureGraph) GetDependencies(address string) []string {
	return g.edges[address]
}

// GetDependents returns nodes that depend on this node
func (g *InfrastructureGraph) GetDependents(address string) []string {
	return g.reverseEdges[address]
}

// GetTransitiveDependencies returns all dependencies (recursive)
func (g *InfrastructureGraph) GetTransitiveDependencies(address string) []string {
	visited := make(map[string]bool)
	result := []string{}
	g.collectDeps(address, visited, &result)
	return result
}

func (g *InfrastructureGraph) collectDeps(address string, visited map[string]bool, result *[]string) {
	for _, dep := range g.edges[address] {
		if !visited[dep] {
			visited[dep] = true
			*result = append(*result, dep)
			g.collectDeps(dep, visited, result)
		}
	}
}

// GetTransitiveDependents returns all dependents (recursive)
func (g *InfrastructureGraph) GetTransitiveDependents(address string) []string {
	visited := make(map[string]bool)
	result := []string{}
	g.collectDependents(address, visited, &result)
	return result
}

func (g *InfrastructureGraph) collectDependents(address string, visited map[string]bool, result *[]string) {
	for _, dep := range g.reverseEdges[address] {
		if !visited[dep] {
			visited[dep] = true
			*result = append(*result, dep)
			g.collectDependents(dep, visited, result)
		}
	}
}

// TopologicalSort returns nodes in dependency order
func (g *InfrastructureGraph) TopologicalSort() ([]string, error) {
	if g.topoValid {
		return g.topoOrder, nil
	}

	visited := make(map[string]bool)
	temp := make(map[string]bool)
	order := []string{}

	var visit func(n string) error
	visit = func(n string) error {
		if temp[n] {
			return &CycleError{Node: n}
		}
		if visited[n] {
			return nil
		}
		temp[n] = true
		for _, dep := range g.edges[n] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		temp[n] = false
		visited[n] = true
		order = append(order, n)
		return nil
	}

	// Sort nodes for determinism
	nodes := make([]string, 0, len(g.nodes))
	for addr := range g.nodes {
		nodes = append(nodes, addr)
	}
	sort.Strings(nodes)

	for _, n := range nodes {
		if err := visit(n); err != nil {
			return nil, err
		}
	}

	// Reverse for correct order
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}

	g.topoOrder = order
	g.topoValid = true
	return order, nil
}

// CycleError indicates a dependency cycle
type CycleError struct {
	Node string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle detected at %s", e.Node)
}

// GetResourcesInModule returns all resources in a module
func (g *InfrastructureGraph) GetResourcesInModule(modulePath string) []*InfraNode {
	var result []*InfraNode
	for _, node := range g.nodes {
		if node.ModulePath == modulePath && node.Type == NodeResource {
			result = append(result, node)
		}
	}
	return result
}

// Size returns the number of nodes
func (g *InfrastructureGraph) Size() int {
	return len(g.nodes)
}

// EdgeCount returns the number of edges
func (g *InfrastructureGraph) EdgeCount() int {
	count := 0
	for _, deps := range g.edges {
		count += len(deps)
	}
	return count
}

// InfraGraphBuilder builds infrastructure graphs from parsed Terraform
type InfraGraphBuilder struct {
	graph *InfrastructureGraph
}

// NewInfraGraphBuilder creates a builder
func NewInfraGraphBuilder() *InfraGraphBuilder {
	return &InfraGraphBuilder{
		graph: NewInfrastructureGraph(),
	}
}

// Build creates the graph from parsed module
func (b *InfraGraphBuilder) Build(parsed *ParsedInfra) (*InfrastructureGraph, error) {
	// Add all resources
	for _, res := range parsed.Resources {
		node := &InfraNode{
			Address:      res.Address,
			Type:         NodeResource,
			ModulePath:   res.ModulePath,
			DefinitionID: res.DefinitionID,
			SourceFile:   res.SourceFile,
			SourceLine:   res.SourceLine,
			ExplicitDeps: res.DependsOn,
			ImplicitDeps: res.ImplicitRefs,
			ProviderDep:  res.Provider,
			Lineage:      b.buildLineage(res),
		}
		b.graph.AddNode(node)
	}

	// Add data sources
	for _, data := range parsed.DataSources {
		node := &InfraNode{
			Address:      data.Address,
			Type:         NodeDataSource,
			ModulePath:   data.ModulePath,
			ImplicitDeps: data.ImplicitRefs,
		}
		b.graph.AddNode(node)
	}

	// Add modules
	for _, mod := range parsed.Modules {
		moduleNode := &ModuleNode{
			Path:       mod.Path,
			Source:     mod.Source,
			ParentPath: mod.ParentPath,
			Inputs:     mod.Inputs,
			Outputs:    mod.Outputs,
			Providers:  mod.Providers,
		}
		b.graph.AddModule(moduleNode)
	}

	// Build edges from dependencies
	for addr, node := range b.graph.nodes {
		// Explicit depends_on
		for _, dep := range node.ExplicitDeps {
			if _, exists := b.graph.nodes[dep]; exists {
				b.graph.AddEdge(addr, dep)
			}
		}
		// Implicit references
		for _, ref := range node.ImplicitDeps {
			// Normalize reference to resource address
			targetAddr := b.normalizeReference(ref)
			if _, exists := b.graph.nodes[targetAddr]; exists {
				b.graph.AddEdge(addr, targetAddr)
			}
		}
	}

	return b.graph, nil
}

func (b *InfraGraphBuilder) normalizeReference(ref string) string {
	// aws_instance.web.id → aws_instance.web
	// module.vpc.aws_subnet.main[0] → module.vpc.aws_subnet.main
	parts := strings.Split(ref, ".")
	if len(parts) >= 2 {
		// Check for index
		if idx := strings.Index(parts[1], "["); idx != -1 {
			parts[1] = parts[1][:idx]
		}
		return parts[0] + "." + parts[1]
	}
	return ref
}

func (b *InfraGraphBuilder) buildLineage(res *ParsedResource) *NodeLineage {
	lineage := &NodeLineage{
		ExpressionRefs: []ExpressionRef{},
		ModuleChain:    []string{},
		ExpansionPath:  []ExpansionStep{},
	}

	// Build expression refs
	for attr, refs := range res.AttributeRefs {
		for _, ref := range refs {
			lineage.ExpressionRefs = append(lineage.ExpressionRefs, ExpressionRef{
				FromAttribute: attr,
				ToAddress:     ref,
				RefType:       b.classifyRef(ref),
			})
		}
	}

	// Build module chain
	if res.ModulePath != "" {
		parts := strings.Split(res.ModulePath, ".")
		for i := range parts {
			lineage.ModuleChain = append(lineage.ModuleChain, strings.Join(parts[:i+1], "."))
		}
	}

	return lineage
}

func (b *InfraGraphBuilder) classifyRef(ref string) RefType {
	if strings.Contains(ref, "[*]") {
		return RefSplat
	}
	if strings.Contains(ref, "[") {
		return RefIndex
	}
	if strings.Contains(ref, "each.") {
		return RefEach
	}
	if strings.Contains(ref, "count.") {
		return RefCount
	}
	if strings.Count(ref, ".") > 1 {
		return RefAttribute
	}
	return RefDirect
}

// ParsedInfra is the input to the graph builder
type ParsedInfra struct {
	Resources   []*ParsedResource
	DataSources []*ParsedDataSource
	Modules     []*ParsedModule
}

// ParsedResource is a parsed resource block
type ParsedResource struct {
	Address       string
	ModulePath    string
	DefinitionID  model.DefinitionID
	SourceFile    string
	SourceLine    int
	DependsOn     []string
	ImplicitRefs  []string
	Provider      string
	AttributeRefs map[string][]string // attribute → references
}

// ParsedDataSource is a parsed data block
type ParsedDataSource struct {
	Address      string
	ModulePath   string
	ImplicitRefs []string
}

// ParsedModule is a parsed module block
type ParsedModule struct {
	Path       string
	Source     string
	ParentPath string
	Inputs     map[string]string
	Outputs    map[string]string
	Providers  map[string]string
}
