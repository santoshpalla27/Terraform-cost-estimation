// Package terraform - Provider configuration and alias resolution
package terraform

import (
	"fmt"
	"sort"
	"strings"

	"terraform-cost/core/model"
)

// ProviderResolver handles provider configuration and alias inheritance
type ProviderResolver struct {
	// Resolved providers by key (type.alias)
	providers map[string]ProviderConfig

	// Provider inheritance chain
	inheritance map[string]string // child -> parent

	// Default regions per provider type
	defaultRegions map[string]string
}

// NewProviderResolver creates a new resolver
func NewProviderResolver() *ProviderResolver {
	return &ProviderResolver{
		providers:   make(map[string]ProviderConfig),
		inheritance: make(map[string]string),
		defaultRegions: map[string]string{
			"aws":     "us-east-1",
			"azurerm": "eastus",
			"google":  "us-central1",
		},
	}
}

// AddProvider registers a provider configuration
func (r *ProviderResolver) AddProvider(cfg ProviderConfig) {
	key := r.providerKey(cfg.Type, cfg.Alias)
	r.providers[key] = cfg
}

// SetInheritance sets up module provider inheritance
// modulePath is the child module, providerMap maps child aliases to parent aliases
func (r *ProviderResolver) SetInheritance(modulePath string, providerMap map[string]string) {
	for childAlias, parentAlias := range providerMap {
		childKey := modulePath + ":" + childAlias
		r.inheritance[childKey] = parentAlias
	}
}

// Resolve determines the provider configuration for a resource
func (r *ProviderResolver) Resolve(
	resourceType string,
	explicitProvider string,
	modulePath string,
) (model.ResolvedProvider, error) {
	// Determine provider type from resource type
	providerType := r.providerTypeFromResource(resourceType)

	// Determine provider key
	providerKey := r.determineProviderKey(providerType, explicitProvider, modulePath)

	// Look up provider
	cfg, ok := r.providers[providerKey]
	if !ok {
		// Fall back to default provider
		cfg = r.defaultProvider(providerType)
	}

	return model.ResolvedProvider{
		Type:       cfg.Type,
		Alias:      cfg.Alias,
		Region:     cfg.Region,
		Attributes: cfg.Config,
	}, nil
}

// providerTypeFromResource extracts provider type from resource type
// e.g., "aws_instance" -> "aws", "google_compute_instance" -> "google"
func (r *ProviderResolver) providerTypeFromResource(resourceType string) string {
	parts := strings.SplitN(resourceType, "_", 2)
	if len(parts) < 1 {
		return ""
	}
	return parts[0]
}

// providerKey creates a unique key for a provider
func (r *ProviderResolver) providerKey(providerType, alias string) string {
	if alias == "" {
		return providerType
	}
	return providerType + "." + alias
}

// determineProviderKey determines which provider to use
func (r *ProviderResolver) determineProviderKey(
	providerType string,
	explicitProvider string,
	modulePath string,
) string {
	// If explicit provider specified, use it
	if explicitProvider != "" {
		// Check if it needs inheritance lookup
		if modulePath != "" {
			inheritKey := modulePath + ":" + explicitProvider
			if inherited, ok := r.inheritance[inheritKey]; ok {
				return inherited
			}
		}
		return explicitProvider
	}

	// Check module-level provider inheritance
	if modulePath != "" {
		// Walk up the module tree looking for provider
		for path := modulePath; path != ""; path = r.parentModule(path) {
			inheritKey := path + ":" + providerType
			if inherited, ok := r.inheritance[inheritKey]; ok {
				return inherited
			}
		}
	}

	// Default to un-aliased provider
	return providerType
}

// parentModule returns the parent module path
func (r *ProviderResolver) parentModule(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], ".")
}

// defaultProvider returns a default provider config
func (r *ProviderResolver) defaultProvider(providerType string) ProviderConfig {
	region := r.defaultRegions[providerType]
	if region == "" {
		region = "us-east-1" // Fallback
	}
	return ProviderConfig{
		Type:   providerType,
		Alias:  "",
		Region: region,
		Config: map[string]any{},
	}
}

// ResolveAllProviders resolves providers for all instances
func (r *ProviderResolver) ResolveAllProviders(
	instances []*model.AssetInstance,
	definitions map[model.DefinitionID]*model.AssetDefinition,
) error {
	for _, inst := range instances {
		def := definitions[inst.DefinitionID]
		if def == nil {
			continue
		}

		// Get explicit provider from definition
		explicitProvider := ""
		if provAttr, ok := def.Attributes["provider"]; ok && provAttr.IsLiteral {
			if s, ok := provAttr.LiteralVal.(string); ok {
				explicitProvider = s
			}
		}

		// Extract module path from address
		modulePath := r.modulePathFromAddress(string(def.Address))

		// Resolve provider
		resolved, err := r.Resolve(string(def.Type), explicitProvider, modulePath)
		if err != nil {
			return fmt.Errorf("failed to resolve provider for %s: %w", inst.Address, err)
		}

		inst.Provider = resolved
	}

	return nil
}

// modulePathFromAddress extracts module path from resource address
// e.g., "module.foo.module.bar.aws_instance.web" -> "module.foo.module.bar"
func (r *ProviderResolver) modulePathFromAddress(addr string) string {
	parts := strings.Split(addr, ".")
	var moduleParts []string

	for i := 0; i < len(parts)-2; i += 2 {
		if parts[i] == "module" {
			moduleParts = append(moduleParts, "module."+parts[i+1])
		} else {
			break
		}
	}

	return strings.Join(moduleParts, ".")
}

// Providers returns all registered providers in sorted order
func (r *ProviderResolver) Providers() []ProviderConfig {
	keys := make([]string, 0, len(r.providers))
	for k := range r.providers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]ProviderConfig, len(keys))
	for i, k := range keys {
		result[i] = r.providers[k]
	}
	return result
}

// ExtractRegion extracts region from provider config or resource attributes
func (r *ProviderResolver) ExtractRegion(
	inst *model.AssetInstance,
	def *model.AssetDefinition,
) string {
	// First check instance provider
	if inst.Provider.Region != "" {
		return inst.Provider.Region
	}

	// Check resource attributes for region
	if regionAttr, ok := inst.Attributes["region"]; ok && !regionAttr.IsUnknown {
		if s, ok := regionAttr.Value.(string); ok {
			return s
		}
	}

	// Check availability_zone and derive region
	if azAttr, ok := inst.Attributes["availability_zone"]; ok && !azAttr.IsUnknown {
		if s, ok := azAttr.Value.(string); ok {
			// Remove the AZ suffix (e.g., "us-east-1a" -> "us-east-1")
			if len(s) > 1 {
				return s[:len(s)-1]
			}
		}
	}

	// Fall back to provider default
	return r.defaultRegions[inst.Provider.Type]
}
