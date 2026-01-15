// Package types - Pricing types
package types

import (
	"time"

	"github.com/shopspring/decimal"
)

// PricingSnapshot represents a point-in-time pricing dataset
type PricingSnapshot struct {
	// ID uniquely identifies this snapshot
	ID string `json:"id"`

	// Provider is the cloud provider
	Provider Provider `json:"provider"`

	// Region is the pricing region
	Region string `json:"region"`

	// Timestamp is when the snapshot was taken
	Timestamp time.Time `json:"timestamp"`

	// Hash is a content hash for validation
	Hash string `json:"hash"`

	// Source indicates where the pricing came from
	Source string `json:"source"`

	// Version is the pricing data version
	Version string `json:"version,omitempty"`
}

// Rate represents a pricing rate for a specific SKU
type Rate struct {
	// Key uniquely identifies what this rate applies to
	Key RateKey `json:"key"`

	// Price is the unit price
	Price decimal.Decimal `json:"price"`

	// Unit is the billing unit (e.g., "hour", "GB-month")
	Unit string `json:"unit"`

	// Currency is the price currency
	Currency Currency `json:"currency"`

	// EffectiveFrom is when this rate became effective
	EffectiveFrom time.Time `json:"effective_from"`

	// EffectiveTo is when this rate expires (nil = current)
	EffectiveTo *time.Time `json:"effective_to,omitempty"`

	// SnapshotID links to the pricing snapshot
	SnapshotID string `json:"snapshot_id"`

	// Description provides additional context
	Description string `json:"description,omitempty"`

	// Tiers contains tiered pricing information
	Tiers []PricingTier `json:"tiers,omitempty"`
}

// PricingTier represents a tier in tiered pricing
type PricingTier struct {
	// StartQuantity is the tier start (inclusive)
	StartQuantity decimal.Decimal `json:"start_quantity"`

	// EndQuantity is the tier end (exclusive, nil = unlimited)
	EndQuantity *decimal.Decimal `json:"end_quantity,omitempty"`

	// Price is the unit price for this tier
	Price decimal.Decimal `json:"price"`

	// Unit is the billing unit
	Unit string `json:"unit"`
}

// CalculateTieredCost calculates cost for tiered pricing
func (r *Rate) CalculateTieredCost(quantity decimal.Decimal) decimal.Decimal {
	if len(r.Tiers) == 0 {
		return quantity.Mul(r.Price)
	}

	total := decimal.Zero
	remaining := quantity

	for _, tier := range r.Tiers {
		if remaining.LessThanOrEqual(decimal.Zero) {
			break
		}

		var tierQuantity decimal.Decimal
		if tier.EndQuantity == nil {
			tierQuantity = remaining
		} else {
			tierRange := tier.EndQuantity.Sub(tier.StartQuantity)
			if remaining.LessThan(tierRange) {
				tierQuantity = remaining
			} else {
				tierQuantity = tierRange
			}
		}

		total = total.Add(tierQuantity.Mul(tier.Price))
		remaining = remaining.Sub(tierQuantity)
	}

	return total
}

// PricingResult contains resolved pricing for a set of rate keys
type PricingResult struct {
	// Rates maps rate keys to their resolved rates
	Rates map[string]Rate `json:"rates"`

	// Snapshot is the pricing snapshot used
	Snapshot PricingSnapshot `json:"snapshot"`

	// Missing lists rate keys that couldn't be resolved
	Missing []RateKey `json:"missing,omitempty"`

	// FromCache indicates how many rates were from cache
	FromCache int `json:"from_cache"`

	// FromDB indicates how many rates were from database
	FromDB int `json:"from_db"`

	// FromAPI indicates how many rates were from API
	FromAPI int `json:"from_api"`
}

// GetRate retrieves a rate by its key
func (r *PricingResult) GetRate(key RateKey) (Rate, bool) {
	if r == nil || r.Rates == nil {
		return Rate{}, false
	}
	rate, ok := r.Rates[key.String()]
	return rate, ok
}

// PricingFilter specifies criteria for pricing lookups
type PricingFilter struct {
	// Provider filters by cloud provider
	Provider Provider `json:"provider,omitempty"`

	// Service filters by service name
	Service string `json:"service,omitempty"`

	// Region filters by region
	Region string `json:"region,omitempty"`

	// ProductFamily filters by product family
	ProductFamily string `json:"product_family,omitempty"`

	// Attributes filters by specific attributes
	Attributes map[string]string `json:"attributes,omitempty"`
}
