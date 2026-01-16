// Package primitives - Data transfer pricing primitives
// Bandwidth, egress, inter-region transfer
package primitives

// DataTransferGB creates a cost unit for data transfer
func DataTransferGB(
	gb float64,
	direction TransferDirection,
	provider CloudProvider,
	region string,
	confidence float64,
) CostUnit {
	if gb <= 0 {
		return CostUnit{}
	}

	service := dataTransferService(provider)

	return CostUnit{
		Name:       "data_transfer",
		Measure:    "GB",
		Quantity:   gb,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"direction": string(direction),
			},
		},
	}
}

// DataProcessedGB creates a cost unit for data processing (NAT Gateway, etc.)
func DataProcessedGB(
	gb float64,
	processingType string,
	provider CloudProvider,
	service string,
	region string,
	confidence float64,
) CostUnit {
	if gb <= 0 {
		return CostUnit{}
	}

	return CostUnit{
		Name:       "data_processed",
		Measure:    "GB",
		Quantity:   gb,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"processingType": processingType,
			},
		},
	}
}

// InterRegionTransferGB creates a cost unit for inter-region data transfer
func InterRegionTransferGB(
	gb float64,
	sourceRegion string,
	destRegion string,
	provider CloudProvider,
	confidence float64,
) CostUnit {
	if gb <= 0 {
		return CostUnit{}
	}

	service := dataTransferService(provider)

	return CostUnit{
		Name:       "inter_region_transfer",
		Measure:    "GB",
		Quantity:   gb,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   sourceRegion,
			Attributes: map[string]string{
				"direction":  "inter_region",
				"destRegion": destRegion,
			},
		},
	}
}

func dataTransferService(provider CloudProvider) string {
	switch provider {
	case AWS:
		return "AWSDataTransfer"
	case Azure:
		return "Bandwidth"
	case GCP:
		return "Network Egress"
	default:
		return "DataTransfer"
	}
}
