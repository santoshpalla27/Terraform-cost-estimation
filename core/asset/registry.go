// Package asset - Asset builder registry
package asset

import (
	"fmt"
	"sync"

	"terraform-cost/core/types"
)

// DefaultBuilderRegistry is the default implementation of BuilderRegistry
type DefaultBuilderRegistry struct {
	mu       sync.RWMutex
	builders map[string]Builder // key: provider/resource_type
	byType   map[string][]Builder
}

// NewBuilderRegistry creates a new builder registry
func NewBuilderRegistry() *DefaultBuilderRegistry {
	return &DefaultBuilderRegistry{
		builders: make(map[string]Builder),
		byType:   make(map[string][]Builder),
	}
}

// Register adds a builder to the registry
func (r *DefaultBuilderRegistry) Register(builder Builder) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeKey(builder.Provider(), builder.ResourceType())
	if _, exists := r.builders[key]; exists {
		return fmt.Errorf("builder already registered: %s", key)
	}

	r.builders[key] = builder
	r.byType[builder.ResourceType()] = append(r.byType[builder.ResourceType()], builder)
	return nil
}

// GetBuilder returns a builder for a specific provider and resource type
func (r *DefaultBuilderRegistry) GetBuilder(provider types.Provider, resourceType string) (Builder, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeKey(provider, resourceType)
	builder, ok := r.builders[key]
	return builder, ok
}

// GetProviderBuilders returns all builders for a provider
func (r *DefaultBuilderRegistry) GetProviderBuilders(provider types.Provider) []Builder {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var builders []Builder
	prefix := string(provider) + "/"
	for key, builder := range r.builders {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			builders = append(builders, builder)
		}
	}
	return builders
}

// GetAllResourceTypes returns all registered resource types
func (r *DefaultBuilderRegistry) GetAllResourceTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.byType))
	for t := range r.byType {
		types = append(types, t)
	}
	return types
}

func makeKey(provider types.Provider, resourceType string) string {
	return string(provider) + "/" + resourceType
}

// Global default registry
var defaultBuilderRegistry = NewBuilderRegistry()

// RegisterBuilder adds a builder to the default registry
func RegisterBuilder(builder Builder) error {
	return defaultBuilderRegistry.Register(builder)
}

// GetDefaultBuilderRegistry returns the default builder registry
func GetDefaultBuilderRegistry() *DefaultBuilderRegistry {
	return defaultBuilderRegistry
}
