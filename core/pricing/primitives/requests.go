// Package primitives - Request and usage-based pricing primitives
// Requests, API calls, data processing
package primitives

// Requests creates a cost unit for request-based pricing
func Requests(
	count float64,
	requestType string,
	provider CloudProvider,
	service string,
	region string,
	confidence float64,
) CostUnit {
	if count <= 0 {
		return CostUnit{} // No cost for zero requests
	}

	// Normalize to millions for pricing
	countMillions := count / 1_000_000

	return CostUnit{
		Name:       "requests",
		Measure:    "million-requests",
		Quantity:   countMillions,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"requestType": requestType,
			},
		},
	}
}

// APIOperations creates a cost unit for API operations (S3 PUT/GET, SQS, etc.)
func APIOperations(
	count float64,
	operationType string,
	tier string,
	provider CloudProvider,
	service string,
	region string,
	confidence float64,
) CostUnit {
	if count <= 0 {
		return CostUnit{}
	}

	// Different APIs use different units (per 1K, per 10K, per 1M)
	countThousands := count / 1_000

	return CostUnit{
		Name:       "operations",
		Measure:    "thousand-operations",
		Quantity:   countThousands,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"operationType": operationType,
				"tier":          tier,
			},
		},
	}
}

// Invocations creates a cost unit for function invocations (Lambda, Cloud Functions)
func Invocations(
	count float64,
	provider CloudProvider,
	service string,
	region string,
	confidence float64,
) CostUnit {
	if count <= 0 {
		return CostUnit{}
	}

	countMillions := count / 1_000_000

	return CostUnit{
		Name:       "invocations",
		Measure:    "million-invocations",
		Quantity:   countMillions,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"usageType": "Invocations",
			},
		},
	}
}

// Duration creates a cost unit for duration-based pricing (Lambda GB-seconds)
func Duration(
	gbSeconds float64,
	architecture string,
	provider CloudProvider,
	service string,
	region string,
	confidence float64,
) CostUnit {
	if gbSeconds <= 0 {
		return CostUnit{}
	}

	return CostUnit{
		Name:       "duration",
		Measure:    "GB-seconds",
		Quantity:   gbSeconds,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"architecture": architecture,
				"usageType":    "Duration",
			},
		},
	}
}
