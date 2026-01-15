// Package clouds provides the cloud plugin system.
// Cloud providers are modular plugins that can be added without modifying core.
package clouds

import (
	"fmt"
	"sync"

	"terraform-cost/core/asset"
	"terraform-cost/core/pricing"
	"terraform-cost/core/types"
	"terraform-cost/core/usage"
)

// Plugin defines the interface for a cloud provider plugin
type Plugin interface {
	// Provider returns the cloud provider identifier
	Provider() types.Provider

	// Name returns a human-readable name
	Name() string

	// Description returns a description of the plugin
	Description() string

	// Initialize sets up the plugin
	Initialize() error

	// AssetBuilders returns asset builders for this provider
	AssetBuilders() []asset.Builder

	// UsageEstimators returns usage estimators for this provider
	UsageEstimators() []usage.Estimator

	// PricingSource returns the pricing source for this provider
	PricingSource() pricing.Source

	// SupportedResourceTypes returns all supported resource types
	SupportedResourceTypes() []string

	// SupportedRegions returns all supported regions
	SupportedRegions() []string
}

// Registry manages cloud plugin registration
type Registry struct {
	mu      sync.RWMutex
	plugins map[types.Provider]Plugin
}

// NewRegistry creates a new plugin registry
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[types.Provider]Plugin),
	}
}

// Register adds a plugin to the registry
func (r *Registry) Register(plugin Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[plugin.Provider()]; exists {
		return fmt.Errorf("plugin already registered: %s", plugin.Provider())
	}

	// Initialize the plugin
	if err := plugin.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", plugin.Provider(), err)
	}

	r.plugins[plugin.Provider()] = plugin
	return nil
}

// GetPlugin returns a plugin by provider
func (r *Registry) GetPlugin(provider types.Provider) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, ok := r.plugins[provider]
	return plugin, ok
}

// GetAll returns all registered plugins
func (r *Registry) GetAll() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]Plugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// Providers returns all registered provider IDs
func (r *Registry) Providers() []types.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]types.Provider, 0, len(r.plugins))
	for p := range r.plugins {
		providers = append(providers, p)
	}
	return providers
}

// RegisterAll registers all components from a plugin to the core registries
func (r *Registry) RegisterAll(
	assetRegistry asset.BuilderRegistry,
	usageRegistry usage.EstimatorRegistry,
	pricingRegistry pricing.SourceRegistry,
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, plugin := range r.plugins {
		// Register asset builders
		for _, builder := range plugin.AssetBuilders() {
			if err := assetRegistry.Register(builder); err != nil {
				return fmt.Errorf("failed to register asset builder from %s: %w", plugin.Provider(), err)
			}
		}

		// Register usage estimators
		for _, estimator := range plugin.UsageEstimators() {
			if err := usageRegistry.Register(estimator); err != nil {
				return fmt.Errorf("failed to register usage estimator from %s: %w", plugin.Provider(), err)
			}
		}

		// Register pricing source
		if source := plugin.PricingSource(); source != nil {
			if err := pricingRegistry.Register(source); err != nil {
				return fmt.Errorf("failed to register pricing source from %s: %w", plugin.Provider(), err)
			}
		}
	}

	return nil
}

// Global default registry
var defaultRegistry = NewRegistry()

// RegisterPlugin adds a plugin to the default registry
func RegisterPlugin(plugin Plugin) error {
	return defaultRegistry.Register(plugin)
}

// GetDefaultRegistry returns the default registry
func GetDefaultRegistry() *Registry {
	return defaultRegistry
}
