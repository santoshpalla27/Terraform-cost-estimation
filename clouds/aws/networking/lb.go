// Package networking - AWS Load Balancer cost mapper
// Pricing model:
// - Hourly charge
// - LCU charge (ALB/NLB) or data processed (Classic)
// Types: ALB, NLB, GLB, Classic ELB
package networking

import (
	"terraform-cost/clouds"
)

// LBMapper maps aws_lb to cost units
type LBMapper struct{}

// NewLBMapper creates a Load Balancer mapper
func NewLBMapper() *LBMapper {
	return &LBMapper{}
}

// Cloud returns the cloud provider
func (m *LBMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *LBMapper) ResourceType() string {
	return "aws_lb"
}

// BuildUsage extracts usage vectors
func (m *LBMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown LB count: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)

	// LCU usage is highly variable
	lcuCount := ctx.ResolveOrDefault("lcu_count", 1)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
		clouds.NewUsageVector("lcu_count", lcuCount, 0.5),
	}, nil
}

// BuildCostUnits creates cost units
func (m *LBMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("load_balancer", "LB cost unknown due to cardinality"),
		}, nil
	}

	lbType := asset.Attr("load_balancer_type")
	if lbType == "" {
		lbType = "application"
	}

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)
	lcuCount, _ := usageVecs.Get("lcu_count")

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	// Determine rate key based on LB type
	var usageTypeHourly, usageTypeLCU string
	switch lbType {
	case "network":
		usageTypeHourly = "LoadBalancerUsage"
		usageTypeLCU = "LCUUsage"
	case "gateway":
		usageTypeHourly = "LoadBalancerUsage"
		usageTypeLCU = "LCUUsage"
	default: // application
		usageTypeHourly = "LoadBalancerUsage"
		usageTypeLCU = "LCUUsage"
	}

	return []clouds.CostUnit{
		// Hourly charge
		clouds.NewCostUnit(
			"hourly",
			"hours",
			monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AWSELB",
				Region:   region,
				Attributes: map[string]string{
					"usageType":        usageTypeHourly,
					"loadBalancerType": lbType,
				},
			},
			0.95,
		),
		// LCU charge
		clouds.NewCostUnit(
			"lcu",
			"LCU-hours",
			lcuCount*monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AWSELB",
				Region:   region,
				Attributes: map[string]string{
					"usageType":        usageTypeLCU,
					"loadBalancerType": lbType,
				},
			},
			0.5, // LCU is usage-dependent
		),
	}, nil
}
