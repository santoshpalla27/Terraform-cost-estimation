// Package model - Enforced canonical identity
// A single canonical format used EVERYWHERE.
package model

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ErrInvalidAddress is returned when an address cannot be parsed
var ErrInvalidAddress = errors.New("invalid resource address")

// CanonicalFormat is the ONLY accepted identity format.
// Pattern: [module.name:]...<type>.<name>[<key_type>=<key>]
//
// Examples:
//   aws_instance.web                           - single resource
//   aws_instance.web[count=0]                  - count expansion
//   aws_instance.web[for_each=prod]            - for_each expansion
//   module.vpc:aws_subnet.main[count=0]        - in module
//   module.vpc:module.db:aws_rds.main          - nested modules
const canonicalPattern = `^((?:module\.[a-zA-Z_][a-zA-Z0-9_]*:)*)?([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z_][a-zA-Z0-9_\-]*)(\[(count|for_each)=([^\]]+)\])?$`

var canonicalRegex = regexp.MustCompile(canonicalPattern)

// EnforcedCanonicalAddress is a validated canonical address.
// Once created, it is guaranteed to be in the correct format.
type EnforcedCanonicalAddress struct {
	raw          string
	modulePath   []string
	resourceType string
	resourceName string
	keyType      KeyType
	keyValue     string
	intKey       int
}

// NewEnforcedAddress creates and validates a canonical address
func NewEnforcedAddress(addr string) (*EnforcedCanonicalAddress, error) {
	// First try to parse as canonical
	if match := canonicalRegex.FindStringSubmatch(addr); match != nil {
		return parseCanonicalMatch(addr, match)
	}

	// Try to normalize from Terraform format
	return normalizeFromTerraform(addr)
}

// MustNewEnforcedAddress creates an address or panics
func MustNewEnforcedAddress(addr string) *EnforcedCanonicalAddress {
	a, err := NewEnforcedAddress(addr)
	if err != nil {
		panic(fmt.Sprintf("invalid address %q: %v", addr, err))
	}
	return a
}

func parseCanonicalMatch(addr string, match []string) (*EnforcedCanonicalAddress, error) {
	a := &EnforcedCanonicalAddress{raw: addr}

	// Parse module path
	if match[1] != "" {
		modulePart := strings.TrimSuffix(match[1], ":")
		for _, part := range strings.Split(modulePart, ":") {
			if strings.HasPrefix(part, "module.") {
				a.modulePath = append(a.modulePath, strings.TrimPrefix(part, "module."))
			}
		}
	}

	a.resourceType = match[2]
	a.resourceName = match[3]

	// Parse key
	if match[4] != "" {
		keyTypeStr := match[5]
		keyVal := match[6]

		if keyTypeStr == "count" {
			a.keyType = KeyTypeInt
			n, err := strconv.Atoi(keyVal)
			if err != nil {
				return nil, fmt.Errorf("invalid count value: %s", keyVal)
			}
			a.intKey = n
			a.keyValue = keyVal
		} else if keyTypeStr == "for_each" {
			a.keyType = KeyTypeString
			a.keyValue = keyVal
		}
	} else {
		a.keyType = KeyTypeNone
	}

	return a, nil
}

func normalizeFromTerraform(addr string) (*EnforcedCanonicalAddress, error) {
	a := &EnforcedCanonicalAddress{}

	// Handle module path
	remaining := addr
	for strings.HasPrefix(remaining, "module.") {
		remaining = strings.TrimPrefix(remaining, "module.")
		dotIdx := strings.Index(remaining, ".")
		if dotIdx == -1 {
			return nil, ErrInvalidAddress
		}
		a.modulePath = append(a.modulePath, remaining[:dotIdx])
		remaining = remaining[dotIdx+1:]
	}

	// Parse resource type.name[key]
	bracketIdx := strings.Index(remaining, "[")
	resourcePart := remaining
	keyPart := ""
	if bracketIdx != -1 {
		resourcePart = remaining[:bracketIdx]
		keyPart = remaining[bracketIdx:]
	}

	// Split type.name
	dotIdx := strings.LastIndex(resourcePart, ".")
	if dotIdx == -1 {
		return nil, ErrInvalidAddress
	}
	a.resourceType = resourcePart[:dotIdx]
	a.resourceName = resourcePart[dotIdx+1:]

	// Parse key
	if keyPart != "" {
		// Remove brackets
		keyPart = strings.TrimPrefix(keyPart, "[")
		keyPart = strings.TrimSuffix(keyPart, "]")

		// Check if it's a number (count) or string (for_each)
		if n, err := strconv.Atoi(keyPart); err == nil {
			a.keyType = KeyTypeInt
			a.intKey = n
			a.keyValue = keyPart
		} else {
			a.keyType = KeyTypeString
			// Remove quotes if present
			a.keyValue = strings.Trim(keyPart, "\"'")
		}
	}

	// Build canonical form
	a.raw = a.String()
	return a, nil
}

// String returns the canonical string representation
func (a *EnforcedCanonicalAddress) String() string {
	if a.raw != "" {
		return a.raw
	}

	var sb strings.Builder

	// Module path
	for _, mod := range a.modulePath {
		sb.WriteString("module.")
		sb.WriteString(mod)
		sb.WriteString(":")
	}

	// Resource
	sb.WriteString(a.resourceType)
	sb.WriteString(".")
	sb.WriteString(a.resourceName)

	// Key
	switch a.keyType {
	case KeyTypeInt:
		sb.WriteString("[count=")
		sb.WriteString(strconv.Itoa(a.intKey))
		sb.WriteString("]")
	case KeyTypeString:
		sb.WriteString("[for_each=")
		sb.WriteString(a.keyValue)
		sb.WriteString("]")
	}

	return sb.String()
}

// ModulePath returns the module path components
func (a *EnforcedCanonicalAddress) ModulePath() []string {
	return a.modulePath
}

// ResourceType returns the resource type
func (a *EnforcedCanonicalAddress) ResourceType() string {
	return a.resourceType
}

// ResourceName returns the resource name
func (a *EnforcedCanonicalAddress) ResourceName() string {
	return a.resourceName
}

// Key returns the expansion key
func (a *EnforcedCanonicalAddress) Key() InstanceKey {
	return InstanceKey{
		Type:     a.keyType,
		IntValue: a.intKey,
		StrValue: a.keyValue,
	}
}

// IsInModule returns true if the resource is inside a module
func (a *EnforcedCanonicalAddress) IsInModule() bool {
	return len(a.modulePath) > 0
}

// BaseAddress returns the address without the expansion key
func (a *EnforcedCanonicalAddress) BaseAddress() string {
	var sb strings.Builder
	for _, mod := range a.modulePath {
		sb.WriteString("module.")
		sb.WriteString(mod)
		sb.WriteString(":")
	}
	sb.WriteString(a.resourceType)
	sb.WriteString(".")
	sb.WriteString(a.resourceName)
	return sb.String()
}

// WithKey returns a new address with a different key
func (a *EnforcedCanonicalAddress) WithKey(keyType KeyType, value interface{}) *EnforcedCanonicalAddress {
	newAddr := &EnforcedCanonicalAddress{
		modulePath:   a.modulePath,
		resourceType: a.resourceType,
		resourceName: a.resourceName,
		keyType:      keyType,
	}

	switch keyType {
	case KeyTypeInt:
		newAddr.intKey = value.(int)
		newAddr.keyValue = strconv.Itoa(value.(int))
	case KeyTypeString:
		newAddr.keyValue = value.(string)
	}

	newAddr.raw = newAddr.String()
	return newAddr
}

// Equals compares two addresses
func (a *EnforcedCanonicalAddress) Equals(other *EnforcedCanonicalAddress) bool {
	return a.String() == other.String()
}

// CanonicalAddressRegistry ensures unique addresses
type CanonicalAddressRegistry struct {
	addresses map[string]*EnforcedCanonicalAddress
}

// NewCanonicalAddressRegistry creates a new registry
func NewCanonicalAddressRegistry() *CanonicalAddressRegistry {
	return &CanonicalAddressRegistry{
		addresses: make(map[string]*EnforcedCanonicalAddress),
	}
}

// Register adds an address, returning error if duplicate
func (r *CanonicalAddressRegistry) Register(addr *EnforcedCanonicalAddress) error {
	key := addr.String()
	if _, exists := r.addresses[key]; exists {
		return fmt.Errorf("duplicate address: %s", key)
	}
	r.addresses[key] = addr
	return nil
}

// Get retrieves an address by string
func (r *CanonicalAddressRegistry) Get(addr string) *EnforcedCanonicalAddress {
	return r.addresses[addr]
}

// All returns all registered addresses
func (r *CanonicalAddressRegistry) All() []*EnforcedCanonicalAddress {
	result := make([]*EnforcedCanonicalAddress, 0, len(r.addresses))
	for _, a := range r.addresses {
		result = append(result, a)
	}
	return result
}
