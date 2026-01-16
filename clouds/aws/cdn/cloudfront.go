// Package cdn - AWS CloudFront and Global Accelerator mappers
package cdn

import (
	"terraform-cost/clouds"
)

// CloudFrontMapper maps aws_cloudfront_distribution to cost units
type CloudFrontMapper struct{}

func NewCloudFrontMapper() *CloudFrontMapper { return &CloudFrontMapper{} }

func (m *CloudFrontMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *CloudFrontMapper) ResourceType() string        { return "aws_cloudfront_distribution" }

func (m *CloudFrontMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage("distributions", "unknown distribution count")}, nil
	}

	// CloudFront is entirely usage-based
	dataTransferGB := ctx.ResolveOrDefault("data_transfer_gb", -1)
	requests := ctx.ResolveOrDefault("monthly_requests", -1)

	if dataTransferGB < 0 || requests < 0 {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricDataTransferGB, "CloudFront usage not provided"),
		}, nil
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricDataTransferGB, dataTransferGB, 0.5),
		clouds.NewUsageVector(clouds.MetricMonthlyRequests, requests, 0.5),
	}, nil
}

func (m *CloudFrontMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("cloudfront", "CloudFront cost depends on data transfer and requests"),
		}, nil
	}

	dataTransferGB, _ := usageVecs.Get(clouds.MetricDataTransferGB)
	requests, _ := usageVecs.Get(clouds.MetricMonthlyRequests)

	priceClass := asset.Attr("price_class")
	if priceClass == "" {
		priceClass = "PriceClass_All"
	}

	return []clouds.CostUnit{
		clouds.NewCostUnit("data_transfer", "GB", dataTransferGB, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonCloudFront",
			Region:   "global",
			Attributes: map[string]string{
				"priceClass": priceClass,
				"usageType":  "DataTransfer-Out-Bytes",
			},
		}, 0.5),
		clouds.NewCostUnit("requests", "10k-requests", requests/10000, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonCloudFront",
			Region:   "global",
			Attributes: map[string]string{
				"usageType": "Requests-Tier1",
			},
		}, 0.5),
	}, nil
}

// GlobalAcceleratorMapper maps aws_global_accelerator to cost units
type GlobalAcceleratorMapper struct{}

func NewGlobalAcceleratorMapper() *GlobalAcceleratorMapper { return &GlobalAcceleratorMapper{} }

func (m *GlobalAcceleratorMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *GlobalAcceleratorMapper) ResourceType() string        { return "aws_globalaccelerator_accelerator" }

func (m *GlobalAcceleratorMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown accelerator count")}, nil
	}
	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, 730, 0.95),
		clouds.NewUsageVector(clouds.MetricDataTransferGB, ctx.ResolveOrDefault("data_transfer_gb", 0), 0.5),
	}, nil
}

func (m *GlobalAcceleratorMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("global_accelerator", "accelerator count unknown")}, nil
	}

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)
	dataTransferGB, _ := usageVecs.Get(clouds.MetricDataTransferGB)

	units := []clouds.CostUnit{
		// Fixed hourly cost per accelerator
		clouds.NewCostUnit("accelerator_hours", "hours", monthlyHours, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AWSGlobalAccelerator",
			Region:   "global",
			Attributes: map[string]string{
				"usageType": "Accelerator-Hours",
			},
		}, 0.95),
	}

	// Data transfer (DT-Premium)
	if dataTransferGB > 0 {
		units = append(units, clouds.NewCostUnit("data_transfer", "GB", dataTransferGB, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AWSGlobalAccelerator",
			Region:   "global",
			Attributes: map[string]string{
				"usageType": "DataTransfer-Premium",
			},
		}, 0.5))
	}

	return units, nil
}
