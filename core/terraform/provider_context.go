// Package terraform - Provider context propagation
// Each instance MUST know exactly which provider config applies.
package terraform

import (
	"fmt"
	"strings"

	"terraform-cost/core/types"
)

// ProviderContext represents the resolved provider configuration for a resource
type ProviderContext struct {
	// Provider type (aws, google, azurerm)
	ProviderType string

	// Alias (empty for default)
	Alias string

	// Resolved region
	Region string

	// Is this inherited from module?
	IsInherited bool

	// Module path where this was defined
	DefinedInModule string

	// Full provider address
	FullAddress string

	// Additional configuration
	Config map[string]interface{}
}

// ProviderKey returns a unique key for this provider
func (p *ProviderContext) ProviderKey() string {
	if p.Alias == "" {
		return p.ProviderType
	}
	return p.ProviderType + "." + p.Alias
}

// ProviderRegistry tracks all provider configurations
type ProviderRegistry struct {
	// Provider configs by module path + alias
	configs map[string]*ProviderConfig

	// Default providers per type
	defaults map[string]*ProviderConfig

	// Module provider mappings (providers = { aws = aws.west })
	moduleMappings map[string]map[string]string
}

// ProviderConfig is defined in pipeline.go - using that definition

// NewProviderRegistry creates a new registry
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		configs:        make(map[string]*ProviderConfig),
		defaults:       make(map[string]*ProviderConfig),
		moduleMappings: make(map[string]map[string]string),
	}
}

// RegisterProvider registers a provider configuration
func (r *ProviderRegistry) RegisterProvider(modulePath string, config *ProviderConfig) {
	key := r.makeKey(modulePath, config.Type, config.Alias)
	r.configs[key] = config

	// Track default if no alias
	if config.Alias == "" {
		defaultKey := r.makeKey(modulePath, config.Type, "")
		r.defaults[defaultKey] = config
	}
}

// RegisterModuleMapping registers a module's provider mapping
func (r *ProviderRegistry) RegisterModuleMapping(modulePath string, mappings map[string]string) {
	r.moduleMappings[modulePath] = mappings
}

// ResolveForResource resolves the provider context for a resource
func (r *ProviderRegistry) ResolveForResource(modulePath, resourceType, providerAttr string) *ProviderContext {
	// Determine provider type from resource type
	providerType := r.extractProviderType(resourceType)

	// Check if resource has explicit provider attribute
	if providerAttr != "" {
		return r.resolveExplicitProvider(modulePath, providerAttr)
	}

	// Check module mappings
	if mapping, ok := r.moduleMappings[modulePath]; ok {
		if mapped, exists := mapping[providerType]; exists {
			return r.resolveExplicitProvider(modulePath, mapped)
		}
	}

	// Look for default provider in current module chain
	return r.resolveDefaultProvider(modulePath, providerType)
}

func (r *ProviderRegistry) resolveExplicitProvider(modulePath, providerRef string) *ProviderContext {
	// Parse provider reference (e.g., "aws.west")
	parts := strings.SplitN(providerRef, ".", 2)
	providerType := parts[0]
	alias := ""
	if len(parts) > 1 {
		alias = parts[1]
	}

	// Search from current module up to root
	currentPath := modulePath
	for {
		key := r.makeKey(currentPath, providerType, alias)
		if config, ok := r.configs[key]; ok {
			return &ProviderContext{
				ProviderType:    providerType,
				Alias:           alias,
				Region:          config.Region,
				IsInherited:     currentPath != modulePath,
				DefinedInModule: currentPath,
				FullAddress:     providerRef,
				Config:          config.Config,
			}
		}

		// Move up one module level
		if currentPath == "" {
			break
		}
		lastDot := strings.LastIndex(currentPath, ".")
		if lastDot == -1 {
			currentPath = ""
		} else {
			currentPath = currentPath[:lastDot]
		}
	}

	// Provider not found - return unknown context
	return &ProviderContext{
		ProviderType: providerType,
		Alias:        alias,
		Region:       "", // UNKNOWN - will need to be resolved or flagged
		FullAddress:  providerRef,
	}
}

func (r *ProviderRegistry) resolveDefaultProvider(modulePath, providerType string) *ProviderContext {
	// Search from current module up to root for default provider
	currentPath := modulePath
	for {
		key := r.makeKey(currentPath, providerType, "")
		if config, ok := r.defaults[key]; ok {
			return &ProviderContext{
				ProviderType:    providerType,
				Alias:           "",
				Region:          config.Region,
				IsInherited:     currentPath != modulePath,
				DefinedInModule: currentPath,
				FullAddress:     providerType,
				Config:          config.Config,
			}
		}

		// Move up one module level
		if currentPath == "" {
			break
		}
		lastDot := strings.LastIndex(currentPath, ".")
		if lastDot == -1 {
			currentPath = ""
		} else {
			currentPath = currentPath[:lastDot]
		}
	}

	// No provider found - use defaults
	return r.createDefaultContext(providerType)
}

func (r *ProviderRegistry) createDefaultContext(providerType string) *ProviderContext {
	// Default regions per provider
	defaultRegions := map[string]string{
		"aws":      "us-east-1",
		"google":   "us-central1",
		"azurerm":  "eastus",
	}

	region := defaultRegions[providerType]
	if region == "" {
		region = "unknown"
	}

	return &ProviderContext{
		ProviderType: providerType,
		Alias:        "",
		Region:       region,
		IsInherited:  false,
		FullAddress:  providerType,
	}
}

func (r *ProviderRegistry) extractProviderType(resourceType string) string {
	// aws_instance -> aws
	// google_compute_instance -> google
	// azurerm_virtual_machine -> azurerm
	idx := strings.Index(resourceType, "_")
	if idx == -1 {
		return resourceType
	}
	return resourceType[:idx]
}

func (r *ProviderRegistry) makeKey(modulePath, providerType, alias string) string {
	if alias != "" {
		return fmt.Sprintf("%s::%s.%s", modulePath, providerType, alias)
	}
	return fmt.Sprintf("%s::%s", modulePath, providerType)
}

// InstanceWithProvider binds an instance to its provider context
type InstanceWithProvider struct {
	InstanceAddress string
	InstanceKey     interface{}
	ProviderCtx     *ProviderContext // Resolved provider context

	// For cost estimation
	PricingRegion   types.Region   // Region for pricing lookups
	PricingProvider types.Provider // Provider for pricing lookups
}

// ResolveRegionForPricing returns the region to use for pricing lookups
func (i *InstanceWithProvider) ResolveRegionForPricing() types.Region {
	if i.ProviderCtx == nil || i.ProviderCtx.Region == "" {
		return types.Region("us-east-1") // Fallback
	}
	return types.Region(i.ProviderCtx.Region)
}

// GetPricingProvider returns the provider for pricing lookups
func (i *InstanceWithProvider) GetPricingProvider() types.Provider {
	if i.ProviderCtx == nil {
		return types.ProviderUnknown
	}
	switch i.ProviderCtx.ProviderType {
	case "aws":
		return types.ProviderAWS
	case "google":
		return types.ProviderGCP
	case "azurerm":
		return types.ProviderAzure
	default:
		return types.ProviderUnknown
	}
}
