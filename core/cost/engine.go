// Package cost provides the cost graph engine interface.
// This package transforms assets + usage into billable cost units.
package cost

import (
	"context"

	"terraform-cost/core/types"
	"terraform-cost/core/usage"
)

// Engine transforms assets and usage into cost graphs
type Engine interface {
	// Calculate produces a cost graph from an asset graph and usage data
	Calculate(ctx context.Context, assets *types.AssetGraph, usage map[string]*usage.EstimationResult, pricing *types.PricingResult) (*types.CostGraph, error)
}

// Calculator calculates costs for a specific resource type
type Calculator interface {
	// Provider returns the cloud provider
	Provider() types.Provider

	// ResourceType returns the resource type this calculator handles
	ResourceType() string

	// Calculate produces cost units for an asset
	Calculate(ctx context.Context, asset *types.Asset, usage []types.UsageVector, pricing *types.PricingResult) ([]*types.CostUnit, error)
}

// CalculatorRegistry manages cost calculator registration
type CalculatorRegistry interface {
	// Register adds a calculator to the registry
	Register(calculator Calculator) error

	// GetCalculator returns a calculator for a provider and resource type
	GetCalculator(provider types.Provider, resourceType string) (Calculator, bool)

	// GetProviderCalculators returns all calculators for a provider
	GetProviderCalculators(provider types.Provider) []Calculator
}

// FormulaContext provides context for cost formula evaluation
type FormulaContext struct {
	// Asset is the asset being priced
	Asset *types.Asset

	// Usage contains usage vectors for the asset
	Usage []types.UsageVector

	// Pricing contains resolved prices
	Pricing *types.PricingResult

	// Region is the deployment region
	Region types.Region
}

// Formula represents a cost calculation formula
type Formula interface {
	// Name returns the formula name
	Name() string

	// Calculate evaluates the formula
	Calculate(ctx *FormulaContext) ([]*types.CostUnit, error)

	// RateKeys returns the rate keys needed for this formula
	RateKeys(ctx *FormulaContext) []types.RateKey
}
