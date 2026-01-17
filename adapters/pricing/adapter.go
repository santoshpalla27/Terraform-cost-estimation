// Package pricing provides production-grade pricing adapter for multiple cloud providers.
// This adapter abstracts cloud-specific pricing API details and provides a unified interface.
package pricing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"terraform-cost/db"

	"github.com/shopspring/decimal"
)

// CloudAdapter is the unified pricing adapter interface
type CloudAdapter interface {
	// Cloud returns the cloud provider
	Cloud() db.CloudProvider
	
	// FetchPricing fetches pricing for a region
	FetchPricing(ctx context.Context, region string) (*FetchResult, error)
	
	// SupportedRegions returns supported regions
	SupportedRegions() []string
	
	// SupportedServices returns supported services
	SupportedServices() []string
	
	// Healthcheck verifies connectivity
	Healthcheck(ctx context.Context) error
}

// FetchResult contains fetched pricing data
type FetchResult struct {
	Cloud       db.CloudProvider
	Region      string
	Rates       []Rate
	FetchedAt   time.Time
	Source      string
	APIVersion  string
	RateCounts  map[string]int // by service
}

// Rate is a normalized pricing rate
type Rate struct {
	Service       string
	ProductFamily string
	SKU           string
	Attributes    map[string]string
	Price         decimal.Decimal
	Unit          string
	Currency      string
	TierMin       *decimal.Decimal
	TierMax       *decimal.Decimal
	EffectiveFrom *time.Time
	EffectiveTo   *time.Time
}

// AdapterRegistry manages cloud adapters
type AdapterRegistry struct {
	adapters map[db.CloudProvider]CloudAdapter
	mu       sync.RWMutex
}

// NewAdapterRegistry creates a new registry
func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{
		adapters: make(map[db.CloudProvider]CloudAdapter),
	}
}

// Register registers an adapter
func (r *AdapterRegistry) Register(adapter CloudAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Cloud()] = adapter
}

// Get returns an adapter for a cloud
func (r *AdapterRegistry) Get(cloud db.CloudProvider) (CloudAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[cloud]
	return adapter, ok
}

// List returns all registered clouds
func (r *AdapterRegistry) List() []db.CloudProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	clouds := make([]db.CloudProvider, 0, len(r.adapters))
	for cloud := range r.adapters {
		clouds = append(clouds, cloud)
	}
	return clouds
}

// FetchConfig configures fetch behavior
type FetchConfig struct {
	// Services to fetch (empty = all)
	Services []string
	
	// Concurrency for parallel fetching
	Concurrency int
	
	// Timeout per service
	Timeout time.Duration
	
	// RetryCount for failed requests
	RetryCount int
	
	// RetryDelay between retries
	RetryDelay time.Duration
	
	// FilterFunc filters rates during fetch
	FilterFunc func(Rate) bool
}

// DefaultFetchConfig returns sensible defaults
func DefaultFetchConfig() *FetchConfig {
	return &FetchConfig{
		Concurrency: 5,
		Timeout:     60 * time.Second,
		RetryCount:  3,
		RetryDelay:  1 * time.Second,
	}
}

// RateNormalizer normalizes cloud-specific rates
type RateNormalizer interface {
	// Normalize converts raw API response to normalized rates
	Normalize(raw interface{}) ([]Rate, error)
	
	// NormalizeUnit standardizes unit names
	NormalizeUnit(unit string) string
	
	// NormalizeAttributes standardizes attribute keys
	NormalizeAttributes(attrs map[string]string) map[string]string
}

// BaseNormalizer provides common normalization logic
type BaseNormalizer struct {
	unitMapping map[string]string
	attrMapping map[string]string
}

// NewBaseNormalizer creates a base normalizer
func NewBaseNormalizer() *BaseNormalizer {
	return &BaseNormalizer{
		unitMapping: map[string]string{
			"Hrs":          "hours",
			"hrs":          "hours",
			"GB-Mo":        "GB-month",
			"GB-month":     "GB-month",
			"GB":           "GB",
			"Requests":     "requests",
			"requests":     "requests",
			"GB-Second":    "GB-seconds",
			"GB-Seconds":   "GB-seconds",
			"Quantity":     "units",
			"LCU-Hrs":      "LCU-hours",
		},
		attrMapping: map[string]string{
			"instanceType":      "instance_type",
			"instanceFamily":    "instance_family",
			"operatingSystem":   "os",
			"tenancy":           "tenancy",
			"storageClass":      "storage_class",
			"databaseEngine":    "engine",
			"productFamily":     "product_family",
		},
	}
}

// NormalizeUnit normalizes a unit string
func (n *BaseNormalizer) NormalizeUnit(unit string) string {
	if normalized, ok := n.unitMapping[unit]; ok {
		return normalized
	}
	return unit
}

// NormalizeAttributes normalizes attribute keys
func (n *BaseNormalizer) NormalizeAttributes(attrs map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range attrs {
		key := k
		if normalized, ok := n.attrMapping[k]; ok {
			key = normalized
		}
		result[key] = v
	}
	return result
}

// CachingAdapter wraps an adapter with caching
type CachingAdapter struct {
	inner    CloudAdapter
	cache    map[string]*cachedResult
	ttl      time.Duration
	mu       sync.RWMutex
}

type cachedResult struct {
	result    *FetchResult
	expiresAt time.Time
}

// NewCachingAdapter creates a caching wrapper
func NewCachingAdapter(inner CloudAdapter, ttl time.Duration) *CachingAdapter {
	return &CachingAdapter{
		inner: inner,
		cache: make(map[string]*cachedResult),
		ttl:   ttl,
	}
}

func (a *CachingAdapter) Cloud() db.CloudProvider {
	return a.inner.Cloud()
}

func (a *CachingAdapter) FetchPricing(ctx context.Context, region string) (*FetchResult, error) {
	key := fmt.Sprintf("%s:%s", a.inner.Cloud(), region)
	
	// Check cache
	a.mu.RLock()
	if cached, ok := a.cache[key]; ok && time.Now().Before(cached.expiresAt) {
		a.mu.RUnlock()
		return cached.result, nil
	}
	a.mu.RUnlock()
	
	// Fetch
	result, err := a.inner.FetchPricing(ctx, region)
	if err != nil {
		return nil, err
	}
	
	// Cache
	a.mu.Lock()
	a.cache[key] = &cachedResult{
		result:    result,
		expiresAt: time.Now().Add(a.ttl),
	}
	a.mu.Unlock()
	
	return result, nil
}

func (a *CachingAdapter) SupportedRegions() []string {
	return a.inner.SupportedRegions()
}

func (a *CachingAdapter) SupportedServices() []string {
	return a.inner.SupportedServices()
}

func (a *CachingAdapter) Healthcheck(ctx context.Context) error {
	return a.inner.Healthcheck(ctx)
}

// MetricsAdapter wraps an adapter with metrics
type MetricsAdapter struct {
	inner        CloudAdapter
	fetchCount   int64
	fetchErrors  int64
	totalLatency int64
	mu           sync.RWMutex
}

// NewMetricsAdapter creates a metrics wrapper
func NewMetricsAdapter(inner CloudAdapter) *MetricsAdapter {
	return &MetricsAdapter{
		inner: inner,
	}
}

func (a *MetricsAdapter) Cloud() db.CloudProvider {
	return a.inner.Cloud()
}

func (a *MetricsAdapter) FetchPricing(ctx context.Context, region string) (*FetchResult, error) {
	start := time.Now()
	result, err := a.inner.FetchPricing(ctx, region)
	
	a.mu.Lock()
	a.fetchCount++
	a.totalLatency += time.Since(start).Milliseconds()
	if err != nil {
		a.fetchErrors++
	}
	a.mu.Unlock()
	
	return result, err
}

func (a *MetricsAdapter) SupportedRegions() []string {
	return a.inner.SupportedRegions()
}

func (a *MetricsAdapter) SupportedServices() []string {
	return a.inner.SupportedServices()
}

func (a *MetricsAdapter) Healthcheck(ctx context.Context) error {
	return a.inner.Healthcheck(ctx)
}

// Metrics returns adapter metrics
func (a *MetricsAdapter) Metrics() (fetchCount, fetchErrors, avgLatencyMs int64) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	avg := int64(0)
	if a.fetchCount > 0 {
		avg = a.totalLatency / a.fetchCount
	}
	return a.fetchCount, a.fetchErrors, avg
}
