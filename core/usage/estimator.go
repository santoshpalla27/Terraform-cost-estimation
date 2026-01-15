// Package usage provides usage estimation interfaces.
// This package estimates how infrastructure will be used.
// Usage is decoupled from resource definitions to enable scenario modeling.
package usage

import (
	"context"

	"terraform-cost/core/types"
)

// Estimator estimates usage for a specific resource type
type Estimator interface {
	// Provider returns the cloud provider
	Provider() types.Provider

	// ResourceType returns the resource type this estimator handles
	ResourceType() string

	// Estimate produces usage vectors for an asset
	Estimate(ctx context.Context, asset *types.Asset, uctx *Context) ([]types.UsageVector, error)
}

// Context provides context for usage estimation
type Context struct {
	// Profile is the active usage profile
	Profile *types.UsageProfile

	// Environment is the target environment (dev, staging, prod)
	Environment string

	// Region is the deployment region
	Region types.Region

	// Scenario is the estimation scenario
	Scenario types.UsageScenario

	// Now is the current time (for time-based calculations)
	Now string

	// CustomDefaults are additional default values
	CustomDefaults map[types.UsageMetric]float64
}

// DefaultContext creates a new usage context with defaults
func DefaultContext() *Context {
	return &Context{
		Environment: "production",
		Scenario:    types.ScenarioTypical,
	}
}

// EstimatorRegistry manages usage estimator registration
type EstimatorRegistry interface {
	// Register adds an estimator to the registry
	Register(estimator Estimator) error

	// GetEstimator returns an estimator for a provider and resource type
	GetEstimator(provider types.Provider, resourceType string) (Estimator, bool)

	// GetProviderEstimators returns all estimators for a provider
	GetProviderEstimators(provider types.Provider) []Estimator
}

// EstimationResult contains usage estimation output
type EstimationResult struct {
	// AssetID is the asset this estimation is for
	AssetID string

	// Vectors are the usage estimates
	Vectors []types.UsageVector

	// Confidence is the overall confidence level
	Confidence float64

	// Assumptions lists assumptions made
	Assumptions []string
}

// Manager orchestrates usage estimation across all assets
type Manager interface {
	// EstimateAll estimates usage for all assets in a graph
	EstimateAll(ctx context.Context, graph *types.AssetGraph, uctx *Context) (map[string]*EstimationResult, error)

	// EstimateAsset estimates usage for a single asset
	EstimateAsset(ctx context.Context, asset *types.Asset, uctx *Context) (*EstimationResult, error)
}
