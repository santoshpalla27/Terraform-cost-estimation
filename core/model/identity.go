// Package model - Canonical instance identity
// Instance identity is NORMALIZED everywhere: cost lineage, policy, diffing, output.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// CanonicalAddress is a normalized, stable instance identity.
// Format: module.path:resource_type.name[key_type=key_value]
// Examples:
//   aws_instance.web                           (single instance)
//   aws_instance.web[count=0]                  (count expansion)
//   aws_instance.web[for_each=prod]            (for_each expansion)
//   module.app:aws_instance.web[count=0]      (in module)
//   module.app:module.db:aws_rds_instance.main (nested modules)
type CanonicalAddress string

// AddressBuilder constructs canonical addresses
type AddressBuilder struct {
	modulePath   []string
	resourceType string
	resourceName string
	keyType      string
	keyValue     string
}

// NewAddressBuilder creates a new builder
func NewAddressBuilder() *AddressBuilder {
	return &AddressBuilder{}
}

// InModule adds a module to the path
func (b *AddressBuilder) InModule(name string) *AddressBuilder {
	b.modulePath = append(b.modulePath, name)
	return b
}

// Resource sets the resource type and name
func (b *AddressBuilder) Resource(resourceType, name string) *AddressBuilder {
	b.resourceType = resourceType
	b.resourceName = name
	return b
}

// WithCount sets a count key
func (b *AddressBuilder) WithCount(index int) *AddressBuilder {
	b.keyType = "count"
	b.keyValue = fmt.Sprintf("%d", index)
	return b
}

// WithForEach sets a for_each key
func (b *AddressBuilder) WithForEach(key string) *AddressBuilder {
	b.keyType = "for_each"
	b.keyValue = key
	return b
}

// Build creates the canonical address
func (b *AddressBuilder) Build() CanonicalAddress {
	var sb strings.Builder

	// Module path
	for i, mod := range b.modulePath {
		if i > 0 {
			sb.WriteString(":")
		}
		sb.WriteString("module.")
		sb.WriteString(mod)
	}

	// Separator if we have modules
	if len(b.modulePath) > 0 {
		sb.WriteString(":")
	}

	// Resource
	sb.WriteString(b.resourceType)
	sb.WriteString(".")
	sb.WriteString(b.resourceName)

	// Key
	if b.keyType != "" {
		sb.WriteString("[")
		sb.WriteString(b.keyType)
		sb.WriteString("=")
		sb.WriteString(b.keyValue)
		sb.WriteString("]")
	}

	return CanonicalAddress(sb.String())
}

// ParseAddress parses any address format into canonical form
func ParseAddress(addr string) (CanonicalAddress, error) {
	// Already canonical?
	if isCanonical(addr) {
		return CanonicalAddress(addr), nil
	}

	// Parse Terraform-style address
	builder := NewAddressBuilder()

	// Split into parts
	parts := strings.Split(addr, ".")

	i := 0
	// Collect module path
	for i < len(parts)-2 {
		if parts[i] == "module" {
			builder.InModule(parts[i+1])
			i += 2
		} else {
			break
		}
	}

	// Resource type and name
	if i+1 >= len(parts) {
		return "", fmt.Errorf("invalid address: %s", addr)
	}

	resourceType := parts[i]
	resourceName := parts[i+1]

	// Check for index in resource name
	if idx := strings.Index(resourceName, "["); idx != -1 {
		keyPart := resourceName[idx+1 : len(resourceName)-1]
		resourceName = resourceName[:idx]

		// Parse key
		if num, err := fmt.Sscanf(keyPart, "%d", new(int)); err == nil && num == 1 {
			var index int
			fmt.Sscanf(keyPart, "%d", &index)
			builder.WithCount(index)
		} else {
			// Remove quotes if present
			keyPart = strings.Trim(keyPart, "\"")
			builder.WithForEach(keyPart)
		}
	}

	builder.Resource(resourceType, resourceName)
	return builder.Build(), nil
}

// isCanonical checks if address is already in canonical form
func isCanonical(addr string) bool {
	// Canonical format uses [key_type=value] not [index] or ["key"]
	return strings.Contains(addr, "[count=") || strings.Contains(addr, "[for_each=")
}

// StableID generates a hash-based ID from a canonical address
func (a CanonicalAddress) StableID() InstanceID {
	h := sha256.New()
	h.Write([]byte(a))
	return InstanceID(hex.EncodeToString(h.Sum(nil))[:16])
}

// ModulePath returns the module path component
func (a CanonicalAddress) ModulePath() string {
	s := string(a)
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return ""
	}
	return s[:idx]
}

// ResourceAddress returns just the resource part
func (a CanonicalAddress) ResourceAddress() string {
	s := string(a)
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s
	}
	return s[idx+1:]
}

// BaseAddress returns the address without the key
func (a CanonicalAddress) BaseAddress() string {
	s := string(a)
	idx := strings.Index(s, "[")
	if idx == -1 {
		return s
	}
	return s[:idx]
}

// Key returns the key part, if any
func (a CanonicalAddress) Key() (keyType, keyValue string) {
	s := string(a)
	re := regexp.MustCompile(`\[(count|for_each)=([^\]]+)\]`)
	matches := re.FindStringSubmatch(s)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	return "", ""
}

// String implements Stringer
func (a CanonicalAddress) String() string {
	return string(a)
}

// InstanceIdentity provides complete identity information
type InstanceIdentity struct {
	// Canonical address (normalized)
	Canonical CanonicalAddress

	// Stable ID (hash-based)
	ID InstanceID

	// Components
	ModulePath   string
	ResourceType string
	ResourceName string
	KeyType      string // "count" or "for_each" or ""
	KeyValue     string

	// Parent definition
	DefinitionID DefinitionID

	// For nested expansion tracking
	ParentKey    *InstanceIdentity
}

// NewInstanceIdentity creates identity from an instance
func NewInstanceIdentity(inst *AssetInstance) *InstanceIdentity {
	canonical, _ := ParseAddress(string(inst.Address))

	keyType, keyValue := "", ""
	switch inst.Key.Type {
	case KeyTypeInt:
		keyType = "count"
		keyValue = fmt.Sprintf("%d", inst.Key.IntValue)
	case KeyTypeString:
		keyType = "for_each"
		keyValue = inst.Key.StrValue
	}

	parts := strings.Split(canonical.BaseAddress(), ".")
	resourceType, resourceName := "", ""
	if len(parts) >= 2 {
		// Find last two parts that aren't module names
		for i := len(parts) - 2; i >= 0; i-- {
			if parts[i] != "module" {
				resourceType = parts[i]
				resourceName = parts[i+1]
				break
			}
		}
	}

	return &InstanceIdentity{
		Canonical:    canonical,
		ID:           canonical.StableID(),
		ModulePath:   canonical.ModulePath(),
		ResourceType: resourceType,
		ResourceName: resourceName,
		KeyType:      keyType,
		KeyValue:     keyValue,
		DefinitionID: inst.DefinitionID,
	}
}

// IdentityIndex indexes instances by various identity components
type IdentityIndex struct {
	byCanonical    map[CanonicalAddress]*InstanceIdentity
	byID           map[InstanceID]*InstanceIdentity
	byModule       map[string][]*InstanceIdentity
	byResourceType map[string][]*InstanceIdentity
	byDefinition   map[DefinitionID][]*InstanceIdentity
}

// NewIdentityIndex creates an empty index
func NewIdentityIndex() *IdentityIndex {
	return &IdentityIndex{
		byCanonical:    make(map[CanonicalAddress]*InstanceIdentity),
		byID:           make(map[InstanceID]*InstanceIdentity),
		byModule:       make(map[string][]*InstanceIdentity),
		byResourceType: make(map[string][]*InstanceIdentity),
		byDefinition:   make(map[DefinitionID][]*InstanceIdentity),
	}
}

// Add adds an identity to the index
func (idx *IdentityIndex) Add(id *InstanceIdentity) {
	idx.byCanonical[id.Canonical] = id
	idx.byID[id.ID] = id
	idx.byModule[id.ModulePath] = append(idx.byModule[id.ModulePath], id)
	idx.byResourceType[id.ResourceType] = append(idx.byResourceType[id.ResourceType], id)
	idx.byDefinition[id.DefinitionID] = append(idx.byDefinition[id.DefinitionID], id)
}

// ByCanonical looks up by canonical address
func (idx *IdentityIndex) ByCanonical(addr CanonicalAddress) *InstanceIdentity {
	return idx.byCanonical[addr]
}

// ByID looks up by ID
func (idx *IdentityIndex) ByID(id InstanceID) *InstanceIdentity {
	return idx.byID[id]
}

// ByModule returns all instances in a module
func (idx *IdentityIndex) ByModule(path string) []*InstanceIdentity {
	result := idx.byModule[path]
	sort.Slice(result, func(i, j int) bool {
		return result[i].Canonical < result[j].Canonical
	})
	return result
}

// ByResourceType returns all instances of a type
func (idx *IdentityIndex) ByResourceType(t string) []*InstanceIdentity {
	result := idx.byResourceType[t]
	sort.Slice(result, func(i, j int) bool {
		return result[i].Canonical < result[j].Canonical
	})
	return result
}

// ByDefinition returns all instances from a definition
func (idx *IdentityIndex) ByDefinition(id DefinitionID) []*InstanceIdentity {
	result := idx.byDefinition[id]
	sort.Slice(result, func(i, j int) bool {
		return result[i].Canonical < result[j].Canonical
	})
	return result
}
