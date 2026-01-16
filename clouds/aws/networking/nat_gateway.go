// Package networking - AWS NAT Gateway cost mapper
// Pricing model:
// - Hourly charge per NAT Gateway
// - Data processing charge per GB
package networking

import (
	"terraform-cost/clouds"
)

// NATGatewayMapper maps aws_nat_gateway to cost units
type NATGatewayMapper struct{}

// NewNATGatewayMapper creates a NAT Gateway mapper
func NewNATGatewayMapper() *NATGatewayMapper {
	return &NATGatewayMapper{}
}

// Cloud returns the cloud provider
func (m *NATGatewayMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *NATGatewayMapper) ResourceType() string {
	return "aws_nat_gateway"
}

// BuildUsage extracts usage vectors
func (m *NATGatewayMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown NAT Gateway count: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)
	dataProcessedGB := ctx.ResolveOrDefault("data_processed_gb", 100)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
		clouds.NewUsageVector(clouds.MetricDataTransferGB, dataProcessedGB, 0.5),
	}, nil
}

// BuildCostUnits creates cost units
func (m *NATGatewayMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("nat_gateway", "NAT Gateway cost unknown due to cardinality"),
		}, nil
	}

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)
	dataProcessedGB, _ := usageVecs.Get(clouds.MetricDataTransferGB)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	return []clouds.CostUnit{
		// Hourly charge
		clouds.NewCostUnit(
			"hourly",
			"hours",
			monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonEC2",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "NatGateway-Hours",
				},
			},
			0.95,
		),
		// Data processing
		clouds.NewCostUnit(
			"data_processed",
			"GB",
			dataProcessedGB,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonEC2",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "NatGateway-Bytes",
				},
			},
			0.5,
		),
	}, nil
}
