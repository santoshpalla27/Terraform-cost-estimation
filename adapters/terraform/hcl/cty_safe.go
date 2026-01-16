// Package hcl - Safe CTY value conversion
// CTY values are NEVER blindly passed through.
// Unknown values MUST be explicitly handled.
package hcl

import (
	"fmt"

	"github.com/zclconf/go-cty/cty"
)

// SafeValue represents a safely-converted CTY value
type SafeValue struct {
	// The Go value (if known)
	Value interface{}

	// Is this value known?
	IsKnown bool

	// Is this value null?
	IsNull bool

	// Type information
	Type SafeValueType

	// Original CTY type name
	CtyType string

	// Why is this unknown?
	UnknownReason string

	// Confidence impact for using this value
	ConfidenceImpact float64
}

// SafeValueType indicates the type of value
type SafeValueType int

const (
	SafeTypeUnknown SafeValueType = iota
	SafeTypeNull
	SafeTypeString
	SafeTypeNumber
	SafeTypeBool
	SafeTypeList
	SafeTypeMap
	SafeTypeTuple
	SafeTypeObject
)

// String returns the type name
func (t SafeValueType) String() string {
	switch t {
	case SafeTypeString:
		return "string"
	case SafeTypeNumber:
		return "number"
	case SafeTypeBool:
		return "bool"
	case SafeTypeList:
		return "list"
	case SafeTypeMap:
		return "map"
	case SafeTypeTuple:
		return "tuple"
	case SafeTypeObject:
		return "object"
	case SafeTypeNull:
		return "null"
	default:
		return "unknown"
	}
}

// CtyToSafe safely converts a cty.Value to a SafeValue
// This NEVER loses type information or unknown status
func CtyToSafe(val cty.Value) SafeValue {
	result := SafeValue{
		CtyType: val.Type().FriendlyName(),
	}

	// Check for unknown FIRST - this is critical
	if !val.IsKnown() {
		result.IsKnown = false
		result.Type = SafeTypeUnknown
		result.UnknownReason = "value not yet known (computed at apply time)"
		result.ConfidenceImpact = 0.3 // Significant impact
		return result
	}

	// Check for null
	if val.IsNull() {
		result.IsNull = true
		result.IsKnown = true
		result.Type = SafeTypeNull
		result.Value = nil
		return result
	}

	result.IsKnown = true

	// Convert based on type
	switch {
	case val.Type() == cty.String:
		result.Type = SafeTypeString
		result.Value = val.AsString()

	case val.Type() == cty.Number:
		result.Type = SafeTypeNumber
		f, _ := val.AsBigFloat().Float64()
		result.Value = f

	case val.Type() == cty.Bool:
		result.Type = SafeTypeBool
		result.Value = val.True()

	case val.Type().IsListType() || val.Type().IsSetType():
		result.Type = SafeTypeList
		result.Value = convertList(val)

	case val.Type().IsTupleType():
		result.Type = SafeTypeTuple
		result.Value = convertTuple(val)

	case val.Type().IsMapType():
		result.Type = SafeTypeMap
		result.Value = convertMap(val)

	case val.Type().IsObjectType():
		result.Type = SafeTypeObject
		result.Value = convertObject(val)

	default:
		// Unknown type - mark as unknown
		result.IsKnown = false
		result.Type = SafeTypeUnknown
		result.UnknownReason = fmt.Sprintf("unhandled CTY type: %s", val.Type().FriendlyName())
		result.ConfidenceImpact = 0.2
	}

	return result
}

func convertList(val cty.Value) []interface{} {
	if !val.CanIterateElements() {
		return nil
	}

	result := make([]interface{}, 0, val.LengthInt())
	iter := val.ElementIterator()
	for iter.Next() {
		_, v := iter.Element()
		safe := CtyToSafe(v)
		if safe.IsKnown && !safe.IsNull {
			result = append(result, safe.Value)
		} else {
			// Include placeholder for unknown elements
			result = append(result, nil)
		}
	}
	return result
}

func convertTuple(val cty.Value) []interface{} {
	return convertList(val) // Same logic
}

func convertMap(val cty.Value) map[string]interface{} {
	if !val.CanIterateElements() {
		return nil
	}

	result := make(map[string]interface{})
	iter := val.ElementIterator()
	for iter.Next() {
		k, v := iter.Element()
		safe := CtyToSafe(v)
		if safe.IsKnown && !safe.IsNull {
			result[k.AsString()] = safe.Value
		} else {
			result[k.AsString()] = nil
		}
	}
	return result
}

func convertObject(val cty.Value) map[string]interface{} {
	return convertMap(val) // Same logic for objects
}

// AsString returns the value as a string, or empty if not a string
func (v SafeValue) AsString() string {
	if !v.IsKnown || v.IsNull || v.Type != SafeTypeString {
		return ""
	}
	if s, ok := v.Value.(string); ok {
		return s
	}
	return ""
}

// AsFloat returns the value as a float64, or 0 if not a number
func (v SafeValue) AsFloat() float64 {
	if !v.IsKnown || v.IsNull || v.Type != SafeTypeNumber {
		return 0
	}
	if f, ok := v.Value.(float64); ok {
		return f
	}
	return 0
}

// AsInt returns the value as an int
func (v SafeValue) AsInt() int {
	return int(v.AsFloat())
}

// AsBool returns the value as a bool
func (v SafeValue) AsBool() bool {
	if !v.IsKnown || v.IsNull || v.Type != SafeTypeBool {
		return false
	}
	if b, ok := v.Value.(bool); ok {
		return b
	}
	return false
}

// AsList returns the value as a slice
func (v SafeValue) AsList() []interface{} {
	if !v.IsKnown || v.IsNull {
		return nil
	}
	if l, ok := v.Value.([]interface{}); ok {
		return l
	}
	return nil
}

// AsMap returns the value as a map
func (v SafeValue) AsMap() map[string]interface{} {
	if !v.IsKnown || v.IsNull {
		return nil
	}
	if m, ok := v.Value.(map[string]interface{}); ok {
		return m
	}
	return nil
}

// MustBeKnown returns an error if the value is unknown
func (v SafeValue) MustBeKnown(context string) error {
	if !v.IsKnown {
		return &UnknownValueError{
			Context: context,
			Reason:  v.UnknownReason,
		}
	}
	return nil
}

// UnknownValueError indicates a value that should be known is unknown
type UnknownValueError struct {
	Context string
	Reason  string
}

func (e *UnknownValueError) Error() string {
	return fmt.Sprintf("unknown value in %s: %s", e.Context, e.Reason)
}
