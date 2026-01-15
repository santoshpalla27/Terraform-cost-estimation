// Package terraform - Unknown value propagation
// Implements correct Terraform unknown semantics: unknowns MUST propagate, never collapse.
package terraform

import (
	"fmt"

	"terraform-cost/core/model"
)

// UnknownValue represents a value that cannot be determined at plan time.
// This is a FIRST-CLASS type, not a nil or empty value.
type UnknownValue struct {
	// Type hint for the expected type
	ExpectedType ValueType

	// Why this value is unknown
	Reason UnknownReason

	// Source of the unknown (for debugging)
	Source string

	// Depth tracks how many levels of unknowns we've propagated through
	Depth int
}

// ValueType indicates the expected type of an unknown value
type ValueType int

const (
	TypeUnknown ValueType = iota
	TypeString
	TypeNumber
	TypeBool
	TypeList
	TypeMap
	TypeObject
)

// UnknownReason explains WHY a value is unknown
type UnknownReason int

const (
	// ReasonComputedAtApply - value computed during terraform apply
	ReasonComputedAtApply UnknownReason = iota

	// ReasonDataSourcePending - data source not yet evaluated
	ReasonDataSourcePending

	// ReasonVariableNotProvided - required variable with no default
	ReasonVariableNotProvided

	// ReasonDependsOnUnknown - depends on another unknown value
	ReasonDependsOnUnknown

	// ReasonExpressionError - expression couldn't be evaluated
	ReasonExpressionError

	// ReasonResourceNotCreated - resource doesn't exist yet
	ReasonResourceNotCreated
)

// String returns human-readable reason
func (r UnknownReason) String() string {
	switch r {
	case ReasonComputedAtApply:
		return "computed at apply time"
	case ReasonDataSourcePending:
		return "data source not yet evaluated"
	case ReasonVariableNotProvided:
		return "required variable not provided"
	case ReasonDependsOnUnknown:
		return "depends on unknown value"
	case ReasonExpressionError:
		return "expression evaluation failed"
	case ReasonResourceNotCreated:
		return "resource not yet created"
	default:
		return "unknown reason"
	}
}

// Value is a wrapper that can hold either a known value or an unknown
type Value struct {
	known   interface{}
	unknown *UnknownValue
}

// Known creates a known value
func Known(v interface{}) Value {
	return Value{known: v}
}

// Unknown creates an unknown value
func Unknown(reason UnknownReason, source string) Value {
	return Value{
		unknown: &UnknownValue{
			Reason: reason,
			Source: source,
			Depth:  0,
		},
	}
}

// UnknownWithType creates a typed unknown
func UnknownWithType(t ValueType, reason UnknownReason, source string) Value {
	return Value{
		unknown: &UnknownValue{
			ExpectedType: t,
			Reason:       reason,
			Source:       source,
			Depth:        0,
		},
	}
}

// IsKnown returns true if value is known
func (v Value) IsKnown() bool {
	return v.unknown == nil
}

// IsUnknown returns true if value is unknown
func (v Value) IsUnknown() bool {
	return v.unknown != nil
}

// Get returns the known value or nil
func (v Value) Get() interface{} {
	if v.IsUnknown() {
		return nil
	}
	return v.known
}

// GetUnknown returns the unknown info
func (v Value) GetUnknown() *UnknownValue {
	return v.unknown
}

// PropagateUnknown creates a new unknown that depends on this one
func (v Value) PropagateUnknown(newSource string) Value {
	if v.IsKnown() {
		return v // Nothing to propagate
	}

	return Value{
		unknown: &UnknownValue{
			ExpectedType: v.unknown.ExpectedType,
			Reason:       ReasonDependsOnUnknown,
			Source:       fmt.Sprintf("%s (from %s)", newSource, v.unknown.Source),
			Depth:        v.unknown.Depth + 1,
		},
	}
}

// UnknownAwareAttribute wraps an attribute that may be unknown
type UnknownAwareAttribute struct {
	Value     Value
	Sensitive bool
	Source    model.SourceLocation
}

// UnknownTracker tracks unknowns throughout the evaluation pipeline
type UnknownTracker struct {
	unknowns map[string]*UnknownValue
}

// NewUnknownTracker creates a new tracker
func NewUnknownTracker() *UnknownTracker {
	return &UnknownTracker{
		unknowns: make(map[string]*UnknownValue),
	}
}

// Track records an unknown value
func (t *UnknownTracker) Track(address string, u *UnknownValue) {
	t.unknowns[address] = u
}

// IsUnknown checks if an address is unknown
func (t *UnknownTracker) IsUnknown(address string) bool {
	_, ok := t.unknowns[address]
	return ok
}

// Get returns unknown info for an address
func (t *UnknownTracker) Get(address string) *UnknownValue {
	return t.unknowns[address]
}

// All returns all tracked unknowns
func (t *UnknownTracker) All() map[string]*UnknownValue {
	// Return copy
	result := make(map[string]*UnknownValue)
	for k, v := range t.unknowns {
		result[k] = v
	}
	return result
}

// Count returns the number of unknowns
func (t *UnknownTracker) Count() int {
	return len(t.unknowns)
}

// UnknownAwareExpander expands resources with proper unknown handling
type UnknownAwareExpander struct {
	tracker *UnknownTracker

	// What to do when count/for_each is unknown
	behavior UnknownExpansionBehavior
}

// UnknownExpansionBehavior defines how to handle unknown count/for_each
type UnknownExpansionBehavior int

const (
	// BehaviorPlaceholder creates a single placeholder instance
	BehaviorPlaceholder UnknownExpansionBehavior = iota

	// BehaviorSkip skips the resource entirely
	BehaviorSkip

	// BehaviorError returns an error
	BehaviorError
)

// NewUnknownAwareExpander creates a new expander
func NewUnknownAwareExpander(behavior UnknownExpansionBehavior) *UnknownAwareExpander {
	return &UnknownAwareExpander{
		tracker:  NewUnknownTracker(),
		behavior: behavior,
	}
}

// ExpandWithUnknowns expands a definition, properly handling unknowns
func (e *UnknownAwareExpander) ExpandWithUnknowns(
	def *model.AssetDefinition,
	ctx *EvalContext,
) ([]*model.AssetInstance, *ExpansionResult) {
	result := &ExpansionResult{
		Warnings: []string{},
		Unknowns: []*UnknownValue{},
	}

	// Check for count
	if def.Count != nil {
		countVal := e.evaluateExpression(*def.Count, ctx)

		if countVal.IsUnknown() {
			// UNKNOWN COUNT: do not guess!
			e.tracker.Track(string(def.Address)+".count", countVal.GetUnknown())
			result.Unknowns = append(result.Unknowns, countVal.GetUnknown())
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("count is unknown: %s", countVal.GetUnknown().Reason))

			switch e.behavior {
			case BehaviorPlaceholder:
				return e.createPlaceholderInstance(def, countVal.GetUnknown()), result
			case BehaviorSkip:
				return []*model.AssetInstance{}, result
			case BehaviorError:
				result.Error = fmt.Errorf("unknown count not allowed")
				return nil, result
			}
		}

		// Known count
		count, ok := countVal.Get().(int)
		if !ok {
			if f, ok := countVal.Get().(float64); ok {
				count = int(f)
			}
		}
		return e.expandCount(def, count, ctx), result
	}

	// Check for for_each
	if def.ForEach != nil {
		forEachVal := e.evaluateExpression(*def.ForEach, ctx)

		if forEachVal.IsUnknown() {
			// UNKNOWN FOR_EACH: do not guess!
			e.tracker.Track(string(def.Address)+".for_each", forEachVal.GetUnknown())
			result.Unknowns = append(result.Unknowns, forEachVal.GetUnknown())
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("for_each is unknown: %s", forEachVal.GetUnknown().Reason))

			switch e.behavior {
			case BehaviorPlaceholder:
				return e.createPlaceholderInstance(def, forEachVal.GetUnknown()), result
			case BehaviorSkip:
				return []*model.AssetInstance{}, result
			case BehaviorError:
				result.Error = fmt.Errorf("unknown for_each not allowed")
				return nil, result
			}
		}

		return e.expandForEach(def, forEachVal.Get(), ctx), result
	}

	// No expansion
	return []*model.AssetInstance{e.createSingleInstance(def, ctx)}, result
}

// ExpansionResult contains the result of expansion
type ExpansionResult struct {
	Warnings []string
	Unknowns []*UnknownValue
	Error    error
}

func (e *UnknownAwareExpander) evaluateExpression(expr model.Expression, ctx *EvalContext) Value {
	if expr.IsLiteral {
		return Known(expr.LiteralVal)
	}

	// Check if any references are unknown
	for _, ref := range expr.References {
		if u := e.tracker.Get(ref); u != nil {
			return Unknown(ReasonDependsOnUnknown, ref)
		}
	}

	// Try to evaluate
	if ctx != nil {
		val, err := ctx.Evaluate(expr)
		if err != nil {
			return Unknown(ReasonExpressionError, expr.Raw)
		}
		return Known(val)
	}

	return Unknown(ReasonExpressionError, expr.Raw)
}

func (e *UnknownAwareExpander) createPlaceholderInstance(
	def *model.AssetDefinition,
	u *UnknownValue,
) []*model.AssetInstance {
	inst := &model.AssetInstance{
		ID:           model.InstanceID(fmt.Sprintf("%s:placeholder", def.ID)),
		DefinitionID: def.ID,
		Address:      model.InstanceAddress(fmt.Sprintf("%s[?]", def.Address)),
		Key:          model.InstanceKey{Type: model.KeyTypeNone},
		Attributes:   make(map[string]model.ResolvedAttribute),
		Metadata: model.InstanceMetadata{
			IsPlaceholder: true,
			Warning:       fmt.Sprintf("placeholder for unknown expansion: %s", u.Reason),
		},
	}

	// Mark all attributes as unknown
	for name := range def.Attributes {
		inst.Attributes[name] = model.ResolvedAttribute{
			IsUnknown: true,
			Reason:    model.ReasonComputedAtApply,
		}
	}

	return []*model.AssetInstance{inst}
}

func (e *UnknownAwareExpander) createSingleInstance(
	def *model.AssetDefinition,
	ctx *EvalContext,
) *model.AssetInstance {
	inst := &model.AssetInstance{
		ID:           model.InstanceID(def.ID),
		DefinitionID: def.ID,
		Address:      model.InstanceAddress(def.Address),
		Key:          model.InstanceKey{Type: model.KeyTypeNone},
		Attributes:   e.resolveAttributes(def, ctx),
	}
	return inst
}

func (e *UnknownAwareExpander) expandCount(
	def *model.AssetDefinition,
	count int,
	ctx *EvalContext,
) []*model.AssetInstance {
	instances := make([]*model.AssetInstance, count)
	for i := 0; i < count; i++ {
		instances[i] = &model.AssetInstance{
			ID:           model.InstanceID(fmt.Sprintf("%s:%d", def.ID, i)),
			DefinitionID: def.ID,
			Address:      model.InstanceAddress(fmt.Sprintf("%s[%d]", def.Address, i)),
			Key:          model.InstanceKey{Type: model.KeyTypeInt, IntValue: i},
			Attributes:   e.resolveAttributesWithCount(def, i, ctx),
		}
	}
	return instances
}

func (e *UnknownAwareExpander) expandForEach(
	def *model.AssetDefinition,
	forEach interface{},
	ctx *EvalContext,
) []*model.AssetInstance {
	var instances []*model.AssetInstance

	switch v := forEach.(type) {
	case map[string]interface{}:
		for key, value := range v {
			instances = append(instances, &model.AssetInstance{
				ID:           model.InstanceID(fmt.Sprintf("%s:%s", def.ID, key)),
				DefinitionID: def.ID,
				Address:      model.InstanceAddress(fmt.Sprintf("%s[%q]", def.Address, key)),
				Key:          model.InstanceKey{Type: model.KeyTypeString, StrValue: key},
				Attributes:   e.resolveAttributesWithEach(def, key, value, ctx),
			})
		}
	case []interface{}:
		for i, item := range v {
			if key, ok := item.(string); ok {
				instances = append(instances, &model.AssetInstance{
					ID:           model.InstanceID(fmt.Sprintf("%s:%s", def.ID, key)),
					DefinitionID: def.ID,
					Address:      model.InstanceAddress(fmt.Sprintf("%s[%q]", def.Address, key)),
					Key:          model.InstanceKey{Type: model.KeyTypeString, StrValue: key},
					Attributes:   e.resolveAttributesWithEach(def, key, item, ctx),
				})
			} else {
				// Use index as fallback
				instances = append(instances, &model.AssetInstance{
					ID:           model.InstanceID(fmt.Sprintf("%s:%d", def.ID, i)),
					DefinitionID: def.ID,
					Address:      model.InstanceAddress(fmt.Sprintf("%s[%d]", def.Address, i)),
					Key:          model.InstanceKey{Type: model.KeyTypeInt, IntValue: i},
					Attributes:   e.resolveAttributesWithEach(def, i, item, ctx),
				})
			}
		}
	}

	return instances
}

func (e *UnknownAwareExpander) resolveAttributes(
	def *model.AssetDefinition,
	ctx *EvalContext,
) map[string]model.ResolvedAttribute {
	result := make(map[string]model.ResolvedAttribute)

	for name, expr := range def.Attributes {
		if name == "count" || name == "for_each" || name == "depends_on" || name == "lifecycle" || name == "provider" {
			continue
		}

		val := e.evaluateExpression(expr, ctx)
		if val.IsUnknown() {
			result[name] = model.ResolvedAttribute{
				IsUnknown: true,
				Reason:    model.UnknownReason(val.GetUnknown().Reason),
			}
		} else {
			result[name] = model.ResolvedAttribute{
				Value:     val.Get(),
				IsUnknown: false,
			}
		}
	}

	return result
}

func (e *UnknownAwareExpander) resolveAttributesWithCount(
	def *model.AssetDefinition,
	index int,
	ctx *EvalContext,
) map[string]model.ResolvedAttribute {
	// Clone context and add count.index
	childCtx := ctx.Clone()
	childCtx.SetLocal("count.index", index)
	return e.resolveAttributes(def, childCtx)
}

func (e *UnknownAwareExpander) resolveAttributesWithEach(
	def *model.AssetDefinition,
	key interface{},
	value interface{},
	ctx *EvalContext,
) map[string]model.ResolvedAttribute {
	childCtx := ctx.Clone()
	childCtx.SetLocal("each.key", key)
	childCtx.SetLocal("each.value", value)
	return e.resolveAttributes(def, childCtx)
}

// Tracker returns the unknown tracker
func (e *UnknownAwareExpander) Tracker() *UnknownTracker {
	return e.tracker
}
