// Package compute - AWS EC2 dedicated host mapper
package compute

import (
	"terraform-cost/clouds"
)

// EC2HostMapper maps aws_ec2_host to cost units
type EC2HostMapper struct{}

func NewEC2HostMapper() *EC2HostMapper { return &EC2HostMapper{} }

func (m *EC2HostMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *EC2HostMapper) ResourceType() string        { return "aws_ec2_host" }

func (m *EC2HostMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown dedicated host count")}, nil
	}
	return []clouds.UsageVector{clouds.NewUsageVector(clouds.MetricMonthlyHours, ctx.ResolveOrDefault("monthly_hours", 730), 0.95)}, nil
}

func (m *EC2HostMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("dedicated_host", "host count unknown")}, nil
	}

	instanceFamily := asset.Attr("instance_family")
	if instanceFamily == "" {
		instanceFamily = "m5"
	}
	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	return []clouds.CostUnit{
		clouds.NewCostUnit("dedicated_host", "host-hours", monthlyHours, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonEC2",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"instanceFamily": instanceFamily,
				"usageType":      "DedicatedHostUsage",
			},
		}, 0.95),
	}, nil
}
