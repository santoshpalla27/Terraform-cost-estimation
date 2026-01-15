// Package types defines core domain types shared across all layers.
// This package contains NO business logic - only type definitions.
package types

import "time"

// Provider represents a cloud provider
type Provider string

const (
	ProviderAWS     Provider = "aws"
	ProviderAzure   Provider = "azure"
	ProviderGCP     Provider = "gcp"
	ProviderUnknown Provider = "unknown"
)

// String returns the string representation of the provider
func (p Provider) String() string {
	return string(p)
}

// IsValid checks if the provider is a known provider
func (p Provider) IsValid() bool {
	switch p {
	case ProviderAWS, ProviderAzure, ProviderGCP:
		return true
	default:
		return false
	}
}

// ResourceAddress uniquely identifies a resource in Terraform
// Format: module.name.resource_type.resource_name or resource_type.resource_name
type ResourceAddress string

// String returns the string representation
func (r ResourceAddress) String() string {
	return string(r)
}

// Attribute represents a resource attribute value
type Attribute struct {
	Value       interface{} `json:"value"`
	IsComputed  bool        `json:"is_computed"`
	IsSensitive bool        `json:"is_sensitive"`
	Type        string      `json:"type,omitempty"`
}

// Attributes is a map of attribute names to values
type Attributes map[string]Attribute

// Get retrieves an attribute value, returning nil if not found
func (a Attributes) Get(key string) interface{} {
	if attr, ok := a[key]; ok {
		return attr.Value
	}
	return nil
}

// GetString retrieves a string attribute value
func (a Attributes) GetString(key string) string {
	if v := a.Get(key); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt retrieves an integer attribute value
func (a Attributes) GetInt(key string) int {
	if v := a.Get(key); v != nil {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}

// GetBool retrieves a boolean attribute value
func (a Attributes) GetBool(key string) bool {
	if v := a.Get(key); v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetFloat retrieves a float64 attribute value
func (a Attributes) GetFloat(key string) float64 {
	if v := a.Get(key); v != nil {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case int64:
			return float64(n)
		}
	}
	return 0
}

// Metadata contains common metadata fields
type Metadata struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   string    `json:"version"`
}

// Region represents a cloud region
type Region string

// String returns the string representation
func (r Region) String() string {
	return string(r)
}
