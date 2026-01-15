// Package asset provides the asset graph builder interface and types.
// This package builds the provider-agnostic infrastructure DAG.
package asset

import (
	"context"

	"terraform-cost/core/types"
)

// Builder constructs Asset nodes from RawAssets
type Builder interface {
	// Provider returns the cloud provider this builder handles
	Provider() types.Provider

	// ResourceType returns the resource type (e.g., "aws_instance")
	ResourceType() string

	// Build converts a RawAsset into an Asset
	Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error)

	// Category returns the asset category for this resource type
	Category() types.AssetCategory
}

// GraphBuilder builds an AssetGraph from raw assets
type GraphBuilder interface {
	// Build constructs an asset graph from raw assets
	Build(ctx context.Context, raw []types.RawAsset) (*types.AssetGraph, error)
}

// BuilderRegistry manages asset builder registration
type BuilderRegistry interface {
	// Register adds a builder to the registry
	Register(builder Builder) error

	// GetBuilder returns a builder for a specific provider and resource type
	GetBuilder(provider types.Provider, resourceType string) (Builder, bool)

	// GetProviderBuilders returns all builders for a provider
	GetProviderBuilders(provider types.Provider) []Builder

	// GetAllResourceTypes returns all registered resource types
	GetAllResourceTypes() []string
}

// BuildOptions configures asset building behavior
type BuildOptions struct {
	// ExpandCount expands count meta-argument into separate assets
	ExpandCount bool

	// ExpandForEach expands for_each meta-argument into separate assets
	ExpandForEach bool

	// ResolveReferences attempts to resolve resource references
	ResolveReferences bool

	// IncludeDataSources includes data sources in the graph
	IncludeDataSources bool
}

// DefaultBuildOptions returns sensible default build options
func DefaultBuildOptions() BuildOptions {
	return BuildOptions{
		ExpandCount:        true,
		ExpandForEach:      true,
		ResolveReferences:  true,
		IncludeDataSources: false,
	}
}

// BuildContext provides context for asset building
type BuildContext struct {
	// Options are the build options
	Options BuildOptions

	// Variables are resolved Terraform variables
	Variables map[string]interface{}

	// Providers are provider configurations
	Providers map[string]ProviderConfig
}

// ProviderConfig contains provider configuration
type ProviderConfig struct {
	// Name is the provider name
	Name string

	// Alias is the provider alias
	Alias string

	// Region is the provider region
	Region string

	// Config contains provider-specific configuration
	Config map[string]interface{}
}
