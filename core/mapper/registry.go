// Package mapper - Mapper registry with validation
// Enforces metadata validation at registration time.
// Fails fast if any mapper is invalid.
package mapper

import (
	"fmt"
	"sync"
)

// Registry holds all registered mappers with validation
type Registry struct {
	mu       sync.RWMutex
	mappers  map[string]AssetCostMapper
	metadata map[string]MapperMetadata
}

// NewRegistry creates a new validated mapper registry
func NewRegistry() *Registry {
	return &Registry{
		mappers:  make(map[string]AssetCostMapper),
		metadata: make(map[string]MapperMetadata),
	}
}

// Register adds a mapper to the registry with validation
// Panics if metadata is invalid (fail fast)
func (r *Registry) Register(mapper AssetCostMapper) {
	md := mapper.Metadata()

	// Validate metadata - panic on failure
	md.MustValidate()

	r.mu.Lock()
	defer r.mu.Unlock()

	key := string(md.Cloud) + ":" + md.ResourceType
	if _, exists := r.mappers[key]; exists {
		panic(fmt.Sprintf("mapper already registered: %s", key))
	}

	r.mappers[key] = mapper
	r.metadata[key] = md
}

// RegisterSafe adds a mapper returning error instead of panic
func (r *Registry) RegisterSafe(mapper AssetCostMapper) error {
	md := mapper.Metadata()

	if err := md.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := string(md.Cloud) + ":" + md.ResourceType
	if _, exists := r.mappers[key]; exists {
		return fmt.Errorf("mapper already registered: %s", key)
	}

	r.mappers[key] = mapper
	r.metadata[key] = md
	return nil
}

// Get returns a mapper by cloud and resource type
func (r *Registry) Get(cloud CloudProvider, resourceType string) (AssetCostMapper, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := string(cloud) + ":" + resourceType
	mapper, ok := r.mappers[key]
	return mapper, ok
}

// GetMetadata returns metadata by cloud and resource type
func (r *Registry) GetMetadata(cloud CloudProvider, resourceType string) (MapperMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := string(cloud) + ":" + resourceType
	md, ok := r.metadata[key]
	return md, ok
}

// ListByCloud returns all mappers for a cloud
func (r *Registry) ListByCloud(cloud CloudProvider) []MapperMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []MapperMetadata
	for _, md := range r.metadata {
		if md.Cloud == cloud {
			result = append(result, md)
		}
	}
	return result
}

// ListByCategory returns all mappers in a category
func (r *Registry) ListByCategory(category string) []MapperMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []MapperMetadata
	for _, md := range r.metadata {
		if md.Category == category {
			result = append(result, md)
		}
	}
	return result
}

// ListHighImpact returns all high-impact mappers
func (r *Registry) ListHighImpact() []MapperMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []MapperMetadata
	for _, md := range r.metadata {
		if md.HighImpact {
			result = append(result, md)
		}
	}
	return result
}

// ListByTier returns all mappers in a coverage tier
func (r *Registry) ListByTier(tier CoverageTier) []MapperMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []MapperMetadata
	for _, md := range r.metadata {
		if md.Tier == tier {
			result = append(result, md)
		}
	}
	return result
}

// Stats returns registry statistics
func (r *Registry) Stats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RegistryStats{
		ByCloud:    make(map[CloudProvider]int),
		ByCategory: make(map[string]int),
		ByBehavior: make(map[CostBehaviorType]int),
		ByTier:     make(map[CoverageTier]int),
	}

	for _, md := range r.metadata {
		stats.Total++
		stats.ByCloud[md.Cloud]++
		stats.ByCategory[md.Category]++
		stats.ByBehavior[md.CostBehavior]++
		stats.ByTier[md.Tier]++

		if md.HighImpact {
			stats.HighImpact++
		}
		if md.RequiresUsage {
			stats.RequiresUsage++
		}
	}

	return stats
}

// RegistryStats holds registry statistics
type RegistryStats struct {
	Total         int
	HighImpact    int
	RequiresUsage int
	ByCloud       map[CloudProvider]int
	ByCategory    map[string]int
	ByBehavior    map[CostBehaviorType]int
	ByTier        map[CoverageTier]int
}

// ValidateAllMappers validates all registered mappers
func (r *Registry) ValidateAllMappers() []error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errors []error
	for key, md := range r.metadata {
		if err := md.Validate(); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", key, err))
		}
	}
	return errors
}

// GlobalRegistry is the default global registry
var GlobalRegistry = NewRegistry()

// Register registers a mapper in the global registry
func Register(mapper AssetCostMapper) {
	GlobalRegistry.Register(mapper)
}

// Get gets a mapper from the global registry
func Get(cloud CloudProvider, resourceType string) (AssetCostMapper, bool) {
	return GlobalRegistry.Get(cloud, resourceType)
}
