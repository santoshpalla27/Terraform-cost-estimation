// Package expression provides HCL expression evaluation.
// This is a clean-room implementation based on Terraform semantics.
package expression

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
)

// ValueKind represents the type of a value
type ValueKind int

const (
	KindNull ValueKind = iota
	KindBool
	KindNumber
	KindString
	KindList
	KindMap
	KindObject
	KindUnknown   // Value exists but is not yet known
	KindSensitive // Value is marked sensitive
)

// Value represents a Terraform/HCL value with type information
type Value struct {
	kind      ValueKind
	boolVal   bool
	numberVal *big.Float
	stringVal string
	listVal   []Value
	mapVal    map[string]Value
	marks     []ValueMark
}

// ValueMark represents metadata about a value
type ValueMark int

const (
	MarkNone ValueMark = iota
	MarkSensitive
	MarkUnknown
	MarkDynamic
)

// Null creates a null value
func Null() Value {
	return Value{kind: KindNull}
}

// Bool creates a boolean value
func Bool(v bool) Value {
	return Value{kind: KindBool, boolVal: v}
}

// Number creates a numeric value
func Number(v float64) Value {
	return Value{kind: KindNumber, numberVal: big.NewFloat(v)}
}

// NumberFromInt creates a numeric value from an integer
func NumberFromInt(v int64) Value {
	return Value{kind: KindNumber, numberVal: big.NewFloat(float64(v))}
}

// String creates a string value
func String(v string) Value {
	return Value{kind: KindString, stringVal: v}
}

// List creates a list value
func List(elements ...Value) Value {
	return Value{kind: KindList, listVal: elements}
}

// Map creates a map value
func Map(elements map[string]Value) Value {
	return Value{kind: KindMap, mapVal: elements}
}

// Unknown creates an unknown value (computed at runtime)
func Unknown() Value {
	return Value{kind: KindUnknown}
}

// UnknownWithType creates an unknown value with a type hint
func UnknownWithType(kind ValueKind) Value {
	v := Value{kind: kind, marks: []ValueMark{MarkUnknown}}
	return v
}

// FromGo converts a Go value to a Value
func FromGo(v interface{}) Value {
	if v == nil {
		return Null()
	}

	switch val := v.(type) {
	case bool:
		return Bool(val)
	case int:
		return NumberFromInt(int64(val))
	case int64:
		return NumberFromInt(val)
	case float64:
		return Number(val)
	case string:
		return String(val)
	case []interface{}:
		elements := make([]Value, len(val))
		for i, e := range val {
			elements[i] = FromGo(e)
		}
		return List(elements...)
	case map[string]interface{}:
		elements := make(map[string]Value)
		for k, e := range val {
			elements[k] = FromGo(e)
		}
		return Map(elements)
	default:
		// Try reflection for other types
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			elements := make([]Value, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				elements[i] = FromGo(rv.Index(i).Interface())
			}
			return List(elements...)
		case reflect.Map:
			elements := make(map[string]Value)
			iter := rv.MapRange()
			for iter.Next() {
				k := fmt.Sprintf("%v", iter.Key().Interface())
				elements[k] = FromGo(iter.Value().Interface())
			}
			return Map(elements)
		}
		// Fallback to string representation
		return String(fmt.Sprintf("%v", v))
	}
}

// Kind returns the value kind
func (v Value) Kind() ValueKind {
	return v.kind
}

// IsNull returns true if value is null
func (v Value) IsNull() bool {
	return v.kind == KindNull
}

// IsUnknown returns true if value is unknown
func (v Value) IsUnknown() bool {
	if v.kind == KindUnknown {
		return true
	}
	for _, m := range v.marks {
		if m == MarkUnknown {
			return true
		}
	}
	return false
}

// IsSensitive returns true if value is sensitive
func (v Value) IsSensitive() bool {
	for _, m := range v.marks {
		if m == MarkSensitive {
			return true
		}
	}
	return false
}

// IsKnown returns true if value is not unknown and not null
func (v Value) IsKnown() bool {
	return !v.IsUnknown() && !v.IsNull()
}

// AsBool returns the boolean value
func (v Value) AsBool() (bool, error) {
	if v.kind != KindBool {
		return false, fmt.Errorf("value is %v, not bool", v.kind)
	}
	return v.boolVal, nil
}

// AsNumber returns the numeric value as float64
func (v Value) AsNumber() (float64, error) {
	if v.kind != KindNumber {
		return 0, fmt.Errorf("value is %v, not number", v.kind)
	}
	f, _ := v.numberVal.Float64()
	return f, nil
}

// AsInt returns the numeric value as int64
func (v Value) AsInt() (int64, error) {
	if v.kind != KindNumber {
		return 0, fmt.Errorf("value is %v, not number", v.kind)
	}
	f, _ := v.numberVal.Float64()
	return int64(f), nil
}

// AsString returns the string value
func (v Value) AsString() (string, error) {
	if v.kind != KindString {
		return "", fmt.Errorf("value is %v, not string", v.kind)
	}
	return v.stringVal, nil
}

// AsList returns the list elements
func (v Value) AsList() ([]Value, error) {
	if v.kind != KindList {
		return nil, fmt.Errorf("value is %v, not list", v.kind)
	}
	return v.listVal, nil
}

// AsMap returns the map elements
func (v Value) AsMap() (map[string]Value, error) {
	if v.kind != KindMap && v.kind != KindObject {
		return nil, fmt.Errorf("value is %v, not map", v.kind)
	}
	return v.mapVal, nil
}

// Length returns the length for lists/maps/strings
func (v Value) Length() (int, error) {
	switch v.kind {
	case KindString:
		return len(v.stringVal), nil
	case KindList:
		return len(v.listVal), nil
	case KindMap, KindObject:
		return len(v.mapVal), nil
	default:
		return 0, fmt.Errorf("cannot get length of %v", v.kind)
	}
}

// Index gets an element by index (for lists)
func (v Value) Index(i int) (Value, error) {
	if v.kind != KindList {
		return Null(), fmt.Errorf("cannot index %v", v.kind)
	}
	if i < 0 || i >= len(v.listVal) {
		return Null(), fmt.Errorf("index %d out of range [0, %d)", i, len(v.listVal))
	}
	return v.listVal[i], nil
}

// GetAttr gets an attribute by name (for maps/objects)
func (v Value) GetAttr(name string) (Value, error) {
	if v.kind != KindMap && v.kind != KindObject {
		return Null(), fmt.Errorf("cannot get attribute of %v", v.kind)
	}
	val, ok := v.mapVal[name]
	if !ok {
		return Null(), nil // Missing attributes are null
	}
	return val, nil
}

// Equals compares values for equality
func (v Value) Equals(other Value) bool {
	if v.kind != other.kind {
		return false
	}

	switch v.kind {
	case KindNull:
		return true
	case KindBool:
		return v.boolVal == other.boolVal
	case KindNumber:
		return v.numberVal.Cmp(other.numberVal) == 0
	case KindString:
		return v.stringVal == other.stringVal
	case KindList:
		if len(v.listVal) != len(other.listVal) {
			return false
		}
		for i := range v.listVal {
			if !v.listVal[i].Equals(other.listVal[i]) {
				return false
			}
		}
		return true
	case KindMap, KindObject:
		if len(v.mapVal) != len(other.mapVal) {
			return false
		}
		for k, val := range v.mapVal {
			otherVal, ok := other.mapVal[k]
			if !ok || !val.Equals(otherVal) {
				return false
			}
		}
		return true
	case KindUnknown:
		return false // Unknowns are never equal
	default:
		return false
	}
}

// ToGo converts the value to a Go interface{}
func (v Value) ToGo() interface{} {
	switch v.kind {
	case KindNull:
		return nil
	case KindBool:
		return v.boolVal
	case KindNumber:
		f, _ := v.numberVal.Float64()
		return f
	case KindString:
		return v.stringVal
	case KindList:
		result := make([]interface{}, len(v.listVal))
		for i, e := range v.listVal {
			result[i] = e.ToGo()
		}
		return result
	case KindMap, KindObject:
		result := make(map[string]interface{})
		for k, e := range v.mapVal {
			result[k] = e.ToGo()
		}
		return result
	case KindUnknown:
		return nil // Unknown converts to nil
	default:
		return nil
	}
}

// String returns a string representation
func (v Value) String() string {
	switch v.kind {
	case KindNull:
		return "null"
	case KindBool:
		if v.boolVal {
			return "true"
		}
		return "false"
	case KindNumber:
		return v.numberVal.Text('f', -1)
	case KindString:
		return fmt.Sprintf("%q", v.stringVal)
	case KindList:
		parts := make([]string, len(v.listVal))
		for i, e := range v.listVal {
			parts[i] = e.String()
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case KindMap, KindObject:
		parts := make([]string, 0, len(v.mapVal))
		for k, e := range v.mapVal {
			parts = append(parts, fmt.Sprintf("%q = %s", k, e.String()))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case KindUnknown:
		return "(unknown)"
	default:
		return "(invalid)"
	}
}

// MarkAsSensitive returns a copy of the value marked as sensitive
func (v Value) MarkAsSensitive() Value {
	newVal := v
	newVal.marks = append(newVal.marks, MarkSensitive)
	return newVal
}

// MarkAsUnknown returns a copy of the value marked as unknown
func (v Value) MarkAsUnknown() Value {
	newVal := v
	newVal.marks = append(newVal.marks, MarkUnknown)
	return newVal
}
