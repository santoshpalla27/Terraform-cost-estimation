// Package model provides the core domain model with strict separation
// between definitions (static) and instances (expanded).
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// DefinitionID uniquely identifies an asset definition (hash-based, stable)
type DefinitionID string

// InstanceID uniquely identifies an expanded instance
type InstanceID string

// InstanceAddress is the full Terraform address with index
// Examples: "aws_instance.web[0]", "module.app.aws_s3_bucket.data[\"logs\"]"
type InstanceAddress string

// DefinitionAddress is the address without index
// Examples: "aws_instance.web", "module.app.aws_s3_bucket.data"
type DefinitionAddress string

// ProviderKey identifies a provider configuration
// Examples: "aws", "aws.west", "google.europe"
type ProviderKey string

// ResourceType is the Terraform resource type
// Examples: "aws_instance", "google_compute_instance"
type ResourceType string

// InstanceKey represents the index for count/for_each
type InstanceKey struct {
	Type     KeyType
	IntValue int
	StrValue string
}

// KeyType indicates how an instance was indexed
type KeyType int

const (
	KeyTypeNone   KeyType = iota // No expansion (single instance)
	KeyTypeInt                   // count expansion: [0], [1], ...
	KeyTypeString                // for_each expansion: ["key"], ...
)

// String returns the key as an address suffix
func (k InstanceKey) String() string {
	switch k.Type {
	case KeyTypeInt:
		return fmt.Sprintf("[%d]", k.IntValue)
	case KeyTypeString:
		return fmt.Sprintf("[%q]", k.StrValue)
	default:
		return ""
	}
}

// SourceLocation tracks where in the source files something was defined
type SourceLocation struct {
	File      string
	StartLine int
	EndLine   int
	Module    string // Module path, empty for root
}

// Expression represents an unevaluated HCL expression
type Expression struct {
	Raw         string   // Original HCL text
	References  []string // Extracted references
	IsLiteral   bool     // True if no references
	LiteralVal  any      // Value if literal
}

// IsStatic returns true if the expression has no dependencies
func (e Expression) IsStatic() bool {
	return e.IsLiteral || len(e.References) == 0
}

// DynamicBlock represents a Terraform dynamic block
type DynamicBlock struct {
	Name      string     // Block type being generated
	ForEach   Expression // Iterator expression
	Iterator  string     // Iterator variable name (default: Name)
	Content   map[string]Expression
	Labels    []Expression
}

// LifecycleConfig holds lifecycle meta-argument values
type LifecycleConfig struct {
	CreateBeforeDestroy bool
	PreventDestroy      bool
	IgnoreChanges       []string
	ReplaceTriggeredBy  []string
}

// AssetDefinition is the STATIC Terraform resource/data block.
// This is what's written in .tf files, before any expansion.
type AssetDefinition struct {
	// Identity
	ID       DefinitionID      // Hash of address + provider + source location
	Address  DefinitionAddress // aws_instance.web
	Provider ProviderKey       // aws, aws.west
	Type     ResourceType      // aws_instance
	Name     string            // web
	Mode     ResourceMode      // managed, data

	// Meta-arguments (unevaluated)
	Count     *Expression   // count meta-argument
	ForEach   *Expression   // for_each meta-argument
	DependsOn []string      // Explicit dependencies
	Lifecycle LifecycleConfig

	// Attributes (may contain expressions)
	Attributes map[string]Expression

	// Dynamic blocks (must be expanded)
	DynamicBlocks []DynamicBlock

	// Provisioners (for cost implications like null_resource)
	Provisioners []Provisioner

	// Source tracking
	Location SourceLocation
}

// ResourceMode indicates managed resource vs data source
type ResourceMode int

const (
	ModeManaged ResourceMode = iota
	ModeData
)

// Provisioner represents a provisioner block
type Provisioner struct {
	Type       string // local-exec, remote-exec, file
	When       string // create, destroy
	OnFailure  string // continue, fail
	Attributes map[string]Expression
}

// ComputeID generates a stable ID for the definition
func (d *AssetDefinition) ComputeID() DefinitionID {
	h := sha256.New()
	h.Write([]byte(d.Address))
	h.Write([]byte(d.Provider))
	h.Write([]byte(fmt.Sprintf("%s:%d", d.Location.File, d.Location.StartLine)))
	return DefinitionID(hex.EncodeToString(h.Sum(nil))[:16])
}

// HasExpansion returns true if count or for_each is set
func (d *AssetDefinition) HasExpansion() bool {
	return d.Count != nil || d.ForEach != nil
}

// ResolvedAttribute is a fully evaluated attribute value
type ResolvedAttribute struct {
	Value     any           // Concrete value
	IsUnknown bool          // True if value couldn't be determined
	Reason    UnknownReason // Why it's unknown
	Sensitive bool          // Marked as sensitive
}

// UnknownReason explains why a value couldn't be determined
type UnknownReason int

const (
	ReasonKnown             UnknownReason = iota // Value is known
	ReasonComputedAtApply                        // Depends on infrastructure state
	ReasonDataSourcePending                      // Data source not yet evaluated
	ReasonCyclicReference                        // Circular dependency
	ReasonMissingVariable                        // Variable not provided
	ReasonExpressionError                        // Evaluation failed
)

// ResolvedProvider is a fully resolved provider configuration
type ResolvedProvider struct {
	Type       string            // aws, google, azurerm
	Alias      string            // Optional alias
	Region     string            // Resolved region
	Attributes map[string]any    // Other provider config
}

// AssetInstance is a CONCRETE, EXPANDED instance.
// This is what we actually cost - after count/for_each expansion.
type AssetInstance struct {
	// Identity
	ID           InstanceID        // Globally unique, hash-based
	DefinitionID DefinitionID      // Links back to definition
	Address      InstanceAddress   // aws_instance.web[0]

	// Instance-specific
	Key          InstanceKey       // The expansion key (0, "prod", etc.)

	// Fully resolved values (no expressions)
	Attributes   map[string]ResolvedAttribute

	// Provider after alias resolution
	Provider     ResolvedProvider

	// Dependencies after resolution (instance-level)
	Dependencies []InstanceID

	// Derived from dynamic blocks
	DynamicData  map[string][]map[string]ResolvedAttribute

	// Metadata
	Metadata     InstanceMetadata
}

// InstanceMetadata contains instance-level metadata
type InstanceMetadata struct {
	CreatedAt     time.Time
	Source        InstanceSource
	IsPlaceholder bool   // True if created for unknown expansion
	Warning       string // Any warning during expansion
}

// InstanceSource tracks how the instance was created
type InstanceSource int

const (
	SourceHCL         InstanceSource = iota // From .tf files
	SourcePlanJSON                          // From terraform plan JSON
	SourceState                             // From terraform state
	SourcePlaceholder                       // Synthetic for unknown count
)

// ComputeID generates a stable ID for the instance
func (i *AssetInstance) ComputeID() InstanceID {
	h := sha256.New()
	h.Write([]byte(i.DefinitionID))
	h.Write([]byte(i.Key.String()))
	return InstanceID(hex.EncodeToString(h.Sum(nil))[:16])
}

// GetAttribute returns an attribute value, handling unknowns
func (i *AssetInstance) GetAttribute(name string) (any, bool, UnknownReason) {
	attr, ok := i.Attributes[name]
	if !ok {
		return nil, false, ReasonKnown
	}
	return attr.Value, !attr.IsUnknown, attr.Reason
}

// InstanceEdge represents a dependency between instances
type InstanceEdge struct {
	From   InstanceID
	To     InstanceID
	Type   EdgeType
}

// EdgeType indicates the type of dependency
type EdgeType int

const (
	EdgeExplicit  EdgeType = iota // depends_on
	EdgeImplicit                  // Reference-based
	EdgeProvider                  // Provider dependency
)

// InstanceGraph is a DAG of AssetInstances.
// All operations happen on instances, NOT definitions.
type InstanceGraph struct {
	// Core data (use sorted access only)
	instances map[InstanceID]*AssetInstance
	edges     []InstanceEdge

	// Indexes (maintained automatically)
	byAddress    map[InstanceAddress]*AssetInstance
	byDefinition map[DefinitionID][]*AssetInstance

	// Computed on demand
	topologicalOrder []InstanceID
	orderValid       bool
}

// NewInstanceGraph creates an empty instance graph
func NewInstanceGraph() *InstanceGraph {
	return &InstanceGraph{
		instances:    make(map[InstanceID]*AssetInstance),
		byAddress:    make(map[InstanceAddress]*AssetInstance),
		byDefinition: make(map[DefinitionID][]*AssetInstance),
	}
}

// AddInstance adds an instance to the graph
func (g *InstanceGraph) AddInstance(inst *AssetInstance) {
	g.instances[inst.ID] = inst
	g.byAddress[inst.Address] = inst
	g.byDefinition[inst.DefinitionID] = append(g.byDefinition[inst.DefinitionID], inst)
	g.orderValid = false
}

// AddEdge adds a dependency edge
func (g *InstanceGraph) AddEdge(from, to InstanceID, edgeType EdgeType) {
	g.edges = append(g.edges, InstanceEdge{From: from, To: to, Type: edgeType})
	g.orderValid = false
}

// Instances returns all instances in stable, sorted order
func (g *InstanceGraph) Instances() []*AssetInstance {
	ids := make([]InstanceID, 0, len(g.instances))
	for id := range g.instances {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	result := make([]*AssetInstance, len(ids))
	for i, id := range ids {
		result[i] = g.instances[id]
	}
	return result
}

// ByAddress looks up an instance by its full address
func (g *InstanceGraph) ByAddress(addr InstanceAddress) (*AssetInstance, bool) {
	inst, ok := g.byAddress[addr]
	return inst, ok
}

// ByDefinition returns all instances expanded from a definition
func (g *InstanceGraph) ByDefinition(defID DefinitionID) []*AssetInstance {
	instances := g.byDefinition[defID]
	// Return sorted copy
	sorted := make([]*AssetInstance, len(instances))
	copy(sorted, instances)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Address < sorted[j].Address
	})
	return sorted
}

// TopologicalOrder returns instances in dependency order
func (g *InstanceGraph) TopologicalOrder() []InstanceID {
	if g.orderValid {
		return g.topologicalOrder
	}

	// Kahn's algorithm for topological sort
	inDegree := make(map[InstanceID]int)
	for id := range g.instances {
		inDegree[id] = 0
	}
	for _, edge := range g.edges {
		inDegree[edge.To]++
	}

	// Find all nodes with no incoming edges
	queue := make([]InstanceID, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })

	result := make([]InstanceID, 0, len(g.instances))
	for len(queue) > 0 {
		// Pop (stable: always take first)
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// Reduce in-degree for neighbors
		for _, edge := range g.edges {
			if edge.From == node {
				inDegree[edge.To]--
				if inDegree[edge.To] == 0 {
					queue = append(queue, edge.To)
					sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })
				}
			}
		}
	}

	g.topologicalOrder = result
	g.orderValid = true
	return result
}

// Size returns the number of instances
func (g *InstanceGraph) Size() int {
	return len(g.instances)
}
