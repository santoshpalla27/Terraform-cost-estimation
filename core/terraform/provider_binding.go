// Package terraform - Module provider binding resolution
// Provider aliases MUST propagate correctly through module boundaries.
package terraform

import (
	"fmt"
	"strings"
)

// ModuleProviderBinding represents how providers are passed to modules
type ModuleProviderBinding struct {
	// Module path (e.g., "module.vpc.module.subnet")
	ModulePath string

	// Provider mappings from parent to child
	// Key: local provider name in module (e.g., "aws")
	// Value: parent provider reference (e.g., "aws.us_east")
	Bindings map[string]string

	// Explicit providers block in module call
	ExplicitProviders map[string]string

	// Inherited from parent (not explicitly set)
	InheritedProviders map[string]string
}

// ModuleProviderResolver resolves provider bindings through module chains
type ModuleProviderResolver struct {
	// Root provider configs
	rootProviders map[string]*ProviderConfig

	// Per-module bindings
	moduleBindings map[string]*ModuleProviderBinding

	// Resolved cache
	resolvedCache map[string]*ProviderContext
}

// NewModuleProviderResolver creates a resolver
func NewModuleProviderResolver() *ModuleProviderResolver {
	return &ModuleProviderResolver{
		rootProviders:  make(map[string]*ProviderConfig),
		moduleBindings: make(map[string]*ModuleProviderBinding),
		resolvedCache:  make(map[string]*ProviderContext),
	}
}

// RegisterRootProvider registers a provider at root level
func (r *ModuleProviderResolver) RegisterRootProvider(config *ProviderConfig) {
	key := config.Type
	if config.Alias != "" {
		key = config.Type + "." + config.Alias
	}
	r.rootProviders[key] = config
}

// RegisterModuleCall registers a module call with its provider mappings
func (r *ModuleProviderResolver) RegisterModuleCall(parentPath, moduleName string, providers map[string]string) {
	childPath := moduleName
	if parentPath != "" {
		childPath = parentPath + "." + moduleName
	}

	binding := &ModuleProviderBinding{
		ModulePath:         childPath,
		Bindings:           make(map[string]string),
		ExplicitProviders:  providers,
		InheritedProviders: make(map[string]string),
	}

	// Process explicit providers
	for localName, parentRef := range providers {
		binding.Bindings[localName] = parentRef
	}

	r.moduleBindings[childPath] = binding
}

// ResolveProvider resolves the exact provider config for a resource
// Following Terraform's provider inheritance rules exactly
func (r *ModuleProviderResolver) ResolveProvider(modulePath, resourceType, explicitProvider string) (*ProviderContext, error) {
	// Cache key
	cacheKey := fmt.Sprintf("%s::%s::%s", modulePath, resourceType, explicitProvider)
	if cached, ok := r.resolvedCache[cacheKey]; ok {
		return cached, nil
	}

	// Extract provider type from resource
	providerType := extractProviderTypeFromResource(resourceType)

	// Determine which provider reference to resolve
	providerRef := providerType // default
	if explicitProvider != "" {
		providerRef = explicitProvider
	}

	// Resolve through module chain
	ctx, err := r.resolveProviderThroughChain(modulePath, providerRef, providerType)
	if err != nil {
		return nil, err
	}

	r.resolvedCache[cacheKey] = ctx
	return ctx, nil
}

// resolveProviderThroughChain walks the module chain to find the actual provider
func (r *ModuleProviderResolver) resolveProviderThroughChain(modulePath, providerRef, providerType string) (*ProviderContext, error) {
	// If at root, resolve directly
	if modulePath == "" {
		return r.resolveAtRoot(providerRef)
	}

	// Check if this module has explicit binding for this provider
	binding := r.moduleBindings[modulePath]

	// Extract just the provider name without alias for matching
	baseProviderName := providerType
	if idx := strings.Index(providerRef, "."); idx != -1 {
		baseProviderName = providerRef[:idx]
	}

	if binding != nil {
		// Check explicit providers first
		if parentRef, ok := binding.ExplicitProviders[baseProviderName]; ok {
			// This module maps this provider to a parent provider
			// Recurse to parent to resolve the actual config
			parentPath := getParentPath(modulePath)
			resolved, err := r.resolveProviderThroughChain(parentPath, parentRef, providerType)
			if err != nil {
				return nil, err
			}
			// Mark as inherited
			resolved.IsInherited = true
			resolved.DefinedInModule = parentPath
			return resolved, nil
		}

		// Check all bindings
		if parentRef, ok := binding.Bindings[providerRef]; ok {
			parentPath := getParentPath(modulePath)
			resolved, err := r.resolveProviderThroughChain(parentPath, parentRef, providerType)
			if err != nil {
				return nil, err
			}
			resolved.IsInherited = true
			return resolved, nil
		}
	}

	// No explicit binding - inherit from parent
	parentPath := getParentPath(modulePath)
	resolved, err := r.resolveProviderThroughChain(parentPath, providerRef, providerType)
	if err != nil {
		return nil, err
	}
	resolved.IsInherited = true
	return resolved, nil
}

// resolveAtRoot resolves a provider at the root module
func (r *ModuleProviderResolver) resolveAtRoot(providerRef string) (*ProviderContext, error) {
	// Look for exact match
	if config, ok := r.rootProviders[providerRef]; ok {
		return &ProviderContext{
			ProviderType:    config.Type,
			Alias:           config.Alias,
			Region:          config.Region,
			IsInherited:     false,
			DefinedInModule: "",
			FullAddress:     providerRef,
			Config:          config.Config,
		}, nil
	}

	// Try without alias (default provider)
	providerType := providerRef
	if idx := strings.Index(providerRef, "."); idx != -1 {
		providerType = providerRef[:idx]
	}

	if config, ok := r.rootProviders[providerType]; ok {
		return &ProviderContext{
			ProviderType:    config.Type,
			Alias:           config.Alias,
			Region:          config.Region,
			IsInherited:     false,
			DefinedInModule: "",
			FullAddress:     providerRef,
			Config:          config.Config,
		}, nil
	}

	// No provider found - return error, not a guess
	return nil, &ProviderNotFoundError{
		ProviderRef: providerRef,
		ModulePath:  "",
	}
}

// ProviderNotFoundError indicates a provider could not be resolved
type ProviderNotFoundError struct {
	ProviderRef string
	ModulePath  string
}

func (e *ProviderNotFoundError) Error() string {
	if e.ModulePath == "" {
		return fmt.Sprintf("provider %q not found in root module", e.ProviderRef)
	}
	return fmt.Sprintf("provider %q not found for module %q", e.ProviderRef, e.ModulePath)
}

func extractProviderTypeFromResource(resourceType string) string {
	if idx := strings.Index(resourceType, "_"); idx != -1 {
		return resourceType[:idx]
	}
	return resourceType
}

func getParentPath(modulePath string) string {
	if idx := strings.LastIndex(modulePath, "."); idx != -1 {
		return modulePath[:idx]
	}
	return ""
}

// ProviderBindingValidator validates that all resources have valid provider bindings
type ProviderBindingValidator struct {
	resolver *ModuleProviderResolver
	errors   []ProviderBindingError
}

// ProviderBindingError is a provider resolution error
type ProviderBindingError struct {
	ResourceAddress string
	ModulePath      string
	ProviderRef     string
	Reason          string
}

// NewProviderBindingValidator creates a validator
func NewProviderBindingValidator(resolver *ModuleProviderResolver) *ProviderBindingValidator {
	return &ProviderBindingValidator{
		resolver: resolver,
		errors:   []ProviderBindingError{},
	}
}

// Validate ensures all resources in a module have valid provider bindings
func (v *ProviderBindingValidator) Validate(resources []ResourceWithProvider) []ProviderBindingError {
	v.errors = []ProviderBindingError{}

	for _, res := range resources {
		_, err := v.resolver.ResolveProvider(res.ModulePath, res.ResourceType, res.ExplicitProvider)
		if err != nil {
			v.errors = append(v.errors, ProviderBindingError{
				ResourceAddress: res.Address,
				ModulePath:      res.ModulePath,
				ProviderRef:     res.ExplicitProvider,
				Reason:          err.Error(),
			})
		}
	}

	return v.errors
}

// ResourceWithProvider is a resource that needs provider resolution
type ResourceWithProvider struct {
	Address          string
	ModulePath       string
	ResourceType     string
	ExplicitProvider string
}
