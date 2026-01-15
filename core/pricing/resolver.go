// Package pricing provides the pricing resolution interface.
// This package resolves prices from cache, database, and cloud APIs.
package pricing

import (
	"context"

	"terraform-cost/core/types"
)

// Resolver resolves pricing for rate keys
type Resolver interface {
	// Resolve fetches rates for the given keys
	Resolve(ctx context.Context, keys []types.RateKey, snapshot *types.PricingSnapshot) (*types.PricingResult, error)

	// GetSnapshot returns the current pricing snapshot for a provider/region
	GetSnapshot(ctx context.Context, provider types.Provider, region string) (*types.PricingSnapshot, error)

	// RefreshSnapshot updates pricing data from cloud APIs
	RefreshSnapshot(ctx context.Context, provider types.Provider, region string) (*types.PricingSnapshot, error)
}

// Cache provides in-memory pricing caching
type Cache interface {
	// Get retrieves a rate from cache
	Get(key types.RateKey) (*types.Rate, bool)

	// Set stores a rate in cache
	Set(key types.RateKey, rate *types.Rate)

	// GetMulti retrieves multiple rates
	GetMulti(keys []types.RateKey) map[string]*types.Rate

	// SetMulti stores multiple rates
	SetMulti(rates map[string]*types.Rate)

	// Invalidate removes a rate from cache
	Invalidate(key types.RateKey)

	// Clear removes all cached rates
	Clear()

	// Size returns the number of cached entries
	Size() int
}

// Store provides persistent pricing storage
type Store interface {
	// GetRates retrieves rates from storage
	GetRates(ctx context.Context, keys []types.RateKey, snapshotID string) ([]types.Rate, error)

	// SaveRates stores rates in storage
	SaveRates(ctx context.Context, rates []types.Rate) error

	// GetSnapshot retrieves a pricing snapshot
	GetSnapshot(ctx context.Context, id string) (*types.PricingSnapshot, error)

	// SaveSnapshot stores a pricing snapshot
	SaveSnapshot(ctx context.Context, snapshot *types.PricingSnapshot) error

	// GetLatestSnapshot returns the most recent snapshot for a provider/region
	GetLatestSnapshot(ctx context.Context, provider types.Provider, region string) (*types.PricingSnapshot, error)

	// ListSnapshots returns all snapshots for a provider/region
	ListSnapshots(ctx context.Context, provider types.Provider, region string) ([]types.PricingSnapshot, error)
}

// Source fetches pricing from external cloud APIs
type Source interface {
	// Provider returns the cloud provider
	Provider() types.Provider

	// FetchRates retrieves rates for the given keys
	FetchRates(ctx context.Context, keys []types.RateKey) ([]types.Rate, error)

	// FetchAll retrieves all rates for a region
	FetchAll(ctx context.Context, region string) ([]types.Rate, error)

	// SupportedRegions returns the list of supported regions
	SupportedRegions() []string
}

// SourceRegistry manages pricing source registration
type SourceRegistry interface {
	// Register adds a source to the registry
	Register(source Source) error

	// GetSource returns a source for a provider
	GetSource(provider types.Provider) (Source, bool)

	// GetAll returns all registered sources
	GetAll() []Source
}

// CompositeResolver implements Resolver using cache, store, and sources
type CompositeResolver struct {
	Cache   Cache
	Store   Store
	Sources SourceRegistry
}
