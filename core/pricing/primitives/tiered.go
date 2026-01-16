// Package primitives - Tiered pricing primitives
// Handles AWS/Azure/GCP tiered pricing models
package primitives

// TieredUsage creates a cost unit for tiered pricing
// Calculates cost across multiple pricing tiers
func TieredUsage(
	quantity float64,
	tiers []PricingTier,
	measure string,
	provider CloudProvider,
	service string,
	region string,
	confidence float64,
) CostUnit {
	if quantity <= 0 || len(tiers) == 0 {
		return CostUnit{}
	}

	// For cost graph, we emit the total quantity
	// Pricing resolution handles tier calculation
	return CostUnit{
		Name:       "tiered_usage",
		Measure:    measure,
		Quantity:   quantity,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"pricingModel": "tiered",
			},
		},
	}
}

// CalculateTieredCost computes the actual cost for tiered pricing
// This is called at pricing resolution time, not in mappers
func CalculateTieredCost(quantity float64, tiers []PricingTier) float64 {
	if quantity <= 0 || len(tiers) == 0 {
		return 0
	}

	var totalCost float64
	remaining := quantity
	previousLimit := 0.0

	for _, tier := range tiers {
		if remaining <= 0 {
			break
		}

		tierLimit := tier.UpTo
		if tierLimit == 0 {
			// Unlimited tier - all remaining goes here
			totalCost += remaining * tier.UnitRate
			remaining = 0
		} else {
			tierSize := tierLimit - previousLimit
			usageInTier := min(remaining, tierSize)
			totalCost += usageInTier * tier.UnitRate
			remaining -= usageInTier
			previousLimit = tierLimit
		}
	}

	return totalCost
}

// FreeTier creates a cost unit that accounts for free tier
func FreeTier(
	quantity float64,
	freeAmount float64,
	measure string,
	provider CloudProvider,
	service string,
	region string,
	confidence float64,
) CostUnit {
	billableQuantity := quantity - freeAmount
	if billableQuantity <= 0 {
		return CostUnit{} // Entirely within free tier
	}

	return CostUnit{
		Name:       "usage",
		Measure:    measure,
		Quantity:   billableQuantity,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"pricingModel": "free_tier_applied",
			},
		},
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
