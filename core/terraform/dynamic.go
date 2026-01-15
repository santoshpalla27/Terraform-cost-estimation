// Package terraform - Dynamic block expansion
package terraform

import (
	"fmt"
	"sort"

	"terraform-cost/core/model"
)

// DynamicBlockExpander handles Terraform dynamic blocks
type DynamicBlockExpander struct{}

// NewDynamicBlockExpander creates a new expander
func NewDynamicBlockExpander() *DynamicBlockExpander {
	return &DynamicBlockExpander{}
}

// ExpandDynamicBlocks expands all dynamic blocks in a definition
func (e *DynamicBlockExpander) ExpandDynamicBlocks(
	def *model.AssetDefinition,
	evalCtx *EvalContext,
) (map[string][]map[string]model.ResolvedAttribute, []string) {
	result := make(map[string][]map[string]model.ResolvedAttribute)
	var warnings []string

	for _, dyn := range def.DynamicBlocks {
		blocks, warn := e.expandSingle(dyn, evalCtx)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		result[dyn.Name] = blocks
	}

	return result, warnings
}

func (e *DynamicBlockExpander) expandSingle(
	dyn model.DynamicBlock,
	evalCtx *EvalContext,
) ([]map[string]model.ResolvedAttribute, string) {
	// Resolve the for_each expression
	forEachValue, ok := e.resolveForEach(dyn.ForEach, evalCtx)
	if !ok {
		return nil, fmt.Sprintf("dynamic %q: for_each could not be resolved", dyn.Name)
	}

	// Determine iterator name (default to block name)
	iterator := dyn.Iterator
	if iterator == "" {
		iterator = dyn.Name
	}

	var blocks []map[string]model.ResolvedAttribute

	// Iterate based on type
	switch v := forEachValue.(type) {
	case []any:
		for i, item := range v {
			block := e.expandContent(dyn.Content, iterator, i, item, evalCtx)
			blocks = append(blocks, block)
		}
	case map[string]any:
		// Sort keys for determinism
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			block := e.expandContent(dyn.Content, iterator, key, v[key], evalCtx)
			blocks = append(blocks, block)
		}
	default:
		return nil, fmt.Sprintf("dynamic %q: for_each must be list or map", dyn.Name)
	}

	return blocks, ""
}

func (e *DynamicBlockExpander) resolveForEach(expr model.Expression, ctx *EvalContext) (any, bool) {
	if expr.IsLiteral {
		return expr.LiteralVal, true
	}

	// Evaluate expression
	if ctx != nil {
		val, err := ctx.Evaluate(expr)
		if err != nil {
			return nil, false
		}
		return val, true
	}

	return nil, false
}

func (e *DynamicBlockExpander) expandContent(
	content map[string]model.Expression,
	iterator string,
	key any,
	value any,
	parentCtx *EvalContext,
) map[string]model.ResolvedAttribute {
	// Create child context with iterator variables
	ctx := parentCtx.Clone()
	ctx.SetLocal(iterator+".key", key)
	ctx.SetLocal(iterator+".value", value)

	result := make(map[string]model.ResolvedAttribute)

	for name, expr := range content {
		if expr.IsLiteral {
			result[name] = model.ResolvedAttribute{
				Value:     expr.LiteralVal,
				IsUnknown: false,
			}
		} else {
			// Try to evaluate with iterator context
			val, err := ctx.Evaluate(expr)
			if err != nil {
				result[name] = model.ResolvedAttribute{
					IsUnknown: true,
					Reason:    model.ReasonExpressionError,
				}
			} else {
				result[name] = model.ResolvedAttribute{
					Value:     val,
					IsUnknown: false,
				}
			}
		}
	}

	return result
}

// EvalContext provides evaluation context for expressions
type EvalContext struct {
	variables map[string]any
	locals    map[string]any
	resources map[string]any
	data      map[string]any
	parent    *EvalContext
}

// NewEvalContext creates a new evaluation context
func NewEvalContext() *EvalContext {
	return &EvalContext{
		variables: make(map[string]any),
		locals:    make(map[string]any),
		resources: make(map[string]any),
		data:      make(map[string]any),
	}
}

// Clone creates a child context
func (c *EvalContext) Clone() *EvalContext {
	return &EvalContext{
		variables: make(map[string]any),
		locals:    make(map[string]any),
		resources: make(map[string]any),
		data:      make(map[string]any),
		parent:    c,
	}
}

// SetLocal sets a local value
func (c *EvalContext) SetLocal(name string, value any) {
	c.locals[name] = value
}

// GetLocal gets a local value (searching parent chain)
func (c *EvalContext) GetLocal(name string) (any, bool) {
	if val, ok := c.locals[name]; ok {
		return val, true
	}
	if c.parent != nil {
		return c.parent.GetLocal(name)
	}
	return nil, false
}

// Evaluate evaluates an expression in this context
func (c *EvalContext) Evaluate(expr model.Expression) (any, error) {
	if expr.IsLiteral {
		return expr.LiteralVal, nil
	}

	// Parse references and resolve
	for _, ref := range expr.References {
		// Try to resolve reference
		// This is simplified - real implementation would parse the reference
		if val, ok := c.GetLocal(ref); ok {
			return val, nil
		}
	}

	return nil, fmt.Errorf("cannot evaluate expression: %s", expr.Raw)
}

// NestedDynamicBlock handles nested dynamic blocks (dynamic within dynamic)
type NestedDynamicBlock struct {
	Parent  string
	Block   model.DynamicBlock
}

// ExpandNested expands nested dynamic blocks
func (e *DynamicBlockExpander) ExpandNested(
	nested []NestedDynamicBlock,
	parentBlocks map[string][]map[string]model.ResolvedAttribute,
	evalCtx *EvalContext,
) map[string][]map[string]model.ResolvedAttribute {
	// For each parent block instance, expand child dynamics
	result := make(map[string][]map[string]model.ResolvedAttribute)

	for _, n := range nested {
		parentInstances := parentBlocks[n.Parent]
		for _, parentAttrs := range parentInstances {
			// Create context with parent block values
			childCtx := evalCtx.Clone()
			for k, v := range parentAttrs {
				childCtx.SetLocal(n.Parent+"."+k, v.Value)
			}

			// Expand the nested block
			blocks, _ := e.expandSingle(n.Block, childCtx)
			result[n.Block.Name] = append(result[n.Block.Name], blocks...)
		}
	}

	return result
}
