// Package expansion provides instance expansion for count and for_each.
// This is a clean-room implementation based on Terraform semantics.
package expansion

import (
	"fmt"
	"sort"

	"terraform-cost/core/expression"
	"terraform-cost/core/types"
)

// InstanceKey represents the index/key for an expanded instance
type InstanceKey struct {
	// Type indicates whether this is a numeric or string key
	Type KeyType
	// NumValue is set for count-based expansion
	NumValue int
	// StrValue is set for for_each-based expansion
	StrValue string
}

// KeyType indicates the type of instance key
type KeyType int

const (
	KeyTypeNone KeyType = iota
	KeyTypeInt
	KeyTypeString
)

// String returns the key as an address suffix
func (k InstanceKey) String() string {
	switch k.Type {
	case KeyTypeInt:
		return fmt.Sprintf("[%d]", k.NumValue)
	case KeyTypeString:
		return fmt.Sprintf("[%q]", k.StrValue)
	default:
		return ""
	}
}

// Value returns the key as an expression Value
func (k InstanceKey) Value() expression.Value {
	switch k.Type {
	case KeyTypeInt:
		return expression.NumberFromInt(int64(k.NumValue))
	case KeyTypeString:
		return expression.String(k.StrValue)
	default:
		return expression.Null()
	}
}

// AssetInstance represents an expanded instance of an asset
type AssetInstance struct {
	// Base is the original asset definition
	Base *types.Asset

	// Key is the instance index/key (from count or for_each)
	Key InstanceKey

	// Address is the full address including index
	Address types.ResourceAddress

	// EachValue is the element value for for_each (nil for count)
	EachValue expression.Value

	// Attributes are the resolved attributes for this instance
	Attributes types.Attributes

	// Metadata about the expansion
	Metadata InstanceMetadata
}

// InstanceMetadata contains information about how the instance was created
type InstanceMetadata struct {
	// ExpansionType indicates how this instance was created
	ExpansionType ExpansionType

	// OriginalAddress is the address before expansion
	OriginalAddress types.ResourceAddress

	// IsKnown indicates whether the expansion count was deterministic
	IsKnown bool

	// Warning is set if expansion produced a warning
	Warning string
}

// ExpansionType indicates the type of expansion
type ExpansionType int

const (
	ExpansionNone     ExpansionType = iota // No expansion (single instance)
	ExpansionCount                         // count meta-argument
	ExpansionForEach                       // for_each meta-argument
	ExpansionUnknown                       // Expansion couldn't be determined
)

// Expander expands assets with count/for_each into instances
type Expander struct {
	// DefaultCountOnUnknown is the count to assume when count is unknown
	DefaultCountOnUnknown int
}

// NewExpander creates a new instance expander
func NewExpander() *Expander {
	return &Expander{
		DefaultCountOnUnknown: 1,
	}
}

// Expand expands a single asset into instances
func (e *Expander) Expand(asset *types.Asset, ctx *expression.Context) ([]*AssetInstance, error) {
	// Check for count meta-argument
	if countAttr := asset.Attributes.Get("count"); countAttr != nil {
		return e.expandCount(asset, countAttr, ctx)
	}

	// Check for for_each meta-argument
	if forEachAttr := asset.Attributes.Get("for_each"); forEachAttr != nil {
		return e.expandForEach(asset, forEachAttr, ctx)
	}

	// No expansion - return single instance
	return []*AssetInstance{
		{
			Base:       asset,
			Key:        InstanceKey{Type: KeyTypeNone},
			Address:    asset.Address,
			Attributes: asset.Attributes,
			Metadata: InstanceMetadata{
				ExpansionType:   ExpansionNone,
				OriginalAddress: asset.Address,
				IsKnown:         true,
			},
		},
	}, nil
}

// expandCount handles count-based expansion
func (e *Expander) expandCount(asset *types.Asset, countVal interface{}, ctx *expression.Context) ([]*AssetInstance, error) {
	count, isKnown := e.resolveCount(countVal, ctx)

	if count == 0 {
		// count = 0 means no instances
		return []*AssetInstance{}, nil
	}

	instances := make([]*AssetInstance, count)
	for i := 0; i < count; i++ {
		key := InstanceKey{Type: KeyTypeInt, NumValue: i}
		addr := types.ResourceAddress(fmt.Sprintf("%s[%d]", asset.Address, i))

		// Create evaluation context for this instance
		instanceCtx := ctx.Clone()
		instanceCtx.SetCountIndex(i)

		instances[i] = &AssetInstance{
			Base:       asset,
			Key:        key,
			Address:    addr,
			Attributes: asset.Attributes,
			Metadata: InstanceMetadata{
				ExpansionType:   ExpansionCount,
				OriginalAddress: asset.Address,
				IsKnown:         isKnown,
			},
		}

		if !isKnown {
			instances[i].Metadata.Warning = "count could not be determined; assuming 1"
		}
	}

	return instances, nil
}

// resolveCount attempts to resolve a count value to an integer
func (e *Expander) resolveCount(countVal interface{}, ctx *expression.Context) (int, bool) {
	// If it's already an int, use it
	if n, ok := countVal.(int); ok {
		return n, true
	}

	// If it's a float, convert
	if f, ok := countVal.(float64); ok {
		return int(f), true
	}

	// If it's an expression.Value, extract
	if v, ok := countVal.(expression.Value); ok {
		if v.IsUnknown() {
			return e.DefaultCountOnUnknown, false
		}
		if n, err := v.AsInt(); err == nil {
			return int(n), true
		}
	}

	// If it's a string reference, try to resolve
	if s, ok := countVal.(string); ok {
		ref, err := expression.ParseReference(s)
		if err == nil && ctx != nil {
			resolved, err := ctx.Resolve(ref)
			if err == nil && !resolved.IsUnknown() {
				if n, err := resolved.AsInt(); err == nil {
					return int(n), true
				}
			}
		}
	}

	// Cannot determine count
	return e.DefaultCountOnUnknown, false
}

// expandForEach handles for_each-based expansion
func (e *Expander) expandForEach(asset *types.Asset, forEachVal interface{}, ctx *expression.Context) ([]*AssetInstance, error) {
	keys, values, isKnown := e.resolveForEach(forEachVal, ctx)

	if len(keys) == 0 {
		return []*AssetInstance{}, nil
	}

	instances := make([]*AssetInstance, len(keys))
	for i, key := range keys {
		instanceKey := InstanceKey{Type: KeyTypeString, StrValue: key}
		addr := types.ResourceAddress(fmt.Sprintf("%s[%q]", asset.Address, key))

		// Create evaluation context for this instance
		instanceCtx := ctx.Clone()
		instanceCtx.SetEach(key, values[key])

		instances[i] = &AssetInstance{
			Base:       asset,
			Key:        instanceKey,
			Address:    addr,
			EachValue:  values[key],
			Attributes: asset.Attributes,
			Metadata: InstanceMetadata{
				ExpansionType:   ExpansionForEach,
				OriginalAddress: asset.Address,
				IsKnown:         isKnown,
			},
		}

		if !isKnown {
			instances[i].Metadata.Warning = "for_each could not be determined"
		}
	}

	return instances, nil
}

// resolveForEach attempts to resolve a for_each value to keys and values
func (e *Expander) resolveForEach(forEachVal interface{}, ctx *expression.Context) ([]string, map[string]expression.Value, bool) {
	values := make(map[string]expression.Value)

	// If it's already a map
	if m, ok := forEachVal.(map[string]interface{}); ok {
		keys := make([]string, 0, len(m))
		for k, v := range m {
			keys = append(keys, k)
			values[k] = expression.FromGo(v)
		}
		sort.Strings(keys)
		return keys, values, true
	}

	// If it's a set/list of strings
	if list, ok := forEachVal.([]interface{}); ok {
		keys := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				keys = append(keys, s)
				values[s] = expression.String(s)
			}
		}
		sort.Strings(keys)
		return keys, values, true
	}

	// If it's an expression.Value
	if v, ok := forEachVal.(expression.Value); ok {
		if v.IsUnknown() {
			return nil, nil, false
		}

		// Try as map
		if m, err := v.AsMap(); err == nil {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
				values[k] = m[k]
			}
			sort.Strings(keys)
			return keys, values, true
		}

		// Try as list
		if list, err := v.AsList(); err == nil {
			keys := make([]string, 0, len(list))
			for _, item := range list {
				if s, err := item.AsString(); err == nil {
					keys = append(keys, s)
					values[s] = expression.String(s)
				}
			}
			sort.Strings(keys)
			return keys, values, true
		}
	}

	// Cannot determine for_each
	return nil, nil, false
}

// ExpandAll expands all assets in a graph
func (e *Expander) ExpandAll(assets []*types.Asset, ctx *expression.Context) ([]*AssetInstance, error) {
	var allInstances []*AssetInstance

	for _, asset := range assets {
		instances, err := e.Expand(asset, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to expand %s: %w", asset.Address, err)
		}
		allInstances = append(allInstances, instances...)
	}

	return allInstances, nil
}

// ExpandedGraph represents an asset graph with all instances expanded
type ExpandedGraph struct {
	// Instances is the list of all expanded instances
	Instances []*AssetInstance

	// ByAddress indexes instances by their full address
	ByAddress map[types.ResourceAddress]*AssetInstance

	// ByBaseAddress groups instances by their base address (before expansion)
	ByBaseAddress map[types.ResourceAddress][]*AssetInstance

	// Warnings collects expansion warnings
	Warnings []string
}

// NewExpandedGraph creates an expanded graph from instances
func NewExpandedGraph(instances []*AssetInstance) *ExpandedGraph {
	g := &ExpandedGraph{
		Instances:     instances,
		ByAddress:     make(map[types.ResourceAddress]*AssetInstance),
		ByBaseAddress: make(map[types.ResourceAddress][]*AssetInstance),
	}

	for _, inst := range instances {
		g.ByAddress[inst.Address] = inst
		g.ByBaseAddress[inst.Metadata.OriginalAddress] = append(
			g.ByBaseAddress[inst.Metadata.OriginalAddress], inst,
		)

		if inst.Metadata.Warning != "" {
			g.Warnings = append(g.Warnings, fmt.Sprintf("%s: %s", inst.Address, inst.Metadata.Warning))
		}
	}

	return g
}
