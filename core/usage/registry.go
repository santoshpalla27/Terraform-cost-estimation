// Package usage - Usage estimator registry
package usage

import (
	"fmt"
	"sync"

	"terraform-cost/core/types"
)

// DefaultEstimatorRegistry is the default implementation
type DefaultEstimatorRegistry struct {
	mu         sync.RWMutex
	estimators map[string]Estimator // key: provider/resource_type
}

// NewEstimatorRegistry creates a new estimator registry
func NewEstimatorRegistry() *DefaultEstimatorRegistry {
	return &DefaultEstimatorRegistry{
		estimators: make(map[string]Estimator),
	}
}

// Register adds an estimator to the registry
func (r *DefaultEstimatorRegistry) Register(estimator Estimator) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeEstimatorKey(estimator.Provider(), estimator.ResourceType())
	if _, exists := r.estimators[key]; exists {
		return fmt.Errorf("estimator already registered: %s", key)
	}

	r.estimators[key] = estimator
	return nil
}

// GetEstimator returns an estimator for a provider and resource type
func (r *DefaultEstimatorRegistry) GetEstimator(provider types.Provider, resourceType string) (Estimator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeEstimatorKey(provider, resourceType)
	estimator, ok := r.estimators[key]
	return estimator, ok
}

// GetProviderEstimators returns all estimators for a provider
func (r *DefaultEstimatorRegistry) GetProviderEstimators(provider types.Provider) []Estimator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var estimators []Estimator
	prefix := string(provider) + "/"
	for key, estimator := range r.estimators {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			estimators = append(estimators, estimator)
		}
	}
	return estimators
}

func makeEstimatorKey(provider types.Provider, resourceType string) string {
	return string(provider) + "/" + resourceType
}

// Global default registry
var defaultEstimatorRegistry = NewEstimatorRegistry()

// RegisterEstimator adds an estimator to the default registry
func RegisterEstimator(estimator Estimator) error {
	return defaultEstimatorRegistry.Register(estimator)
}

// GetDefaultEstimatorRegistry returns the default registry
func GetDefaultEstimatorRegistry() *DefaultEstimatorRegistry {
	return defaultEstimatorRegistry
}
