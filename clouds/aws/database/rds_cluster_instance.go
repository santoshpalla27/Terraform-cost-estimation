// Package database - AWS RDS Cluster Instance mapper
package database

import (
	"terraform-cost/clouds"
)

// RDSClusterInstanceMapper maps aws_rds_cluster_instance to cost units
type RDSClusterInstanceMapper struct{}

func NewRDSClusterInstanceMapper() *RDSClusterInstanceMapper { return &RDSClusterInstanceMapper{} }

func (m *RDSClusterInstanceMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *RDSClusterInstanceMapper) ResourceType() string        { return "aws_rds_cluster_instance" }

func (m *RDSClusterInstanceMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown instance count")}, nil
	}
	return []clouds.UsageVector{clouds.NewUsageVector(clouds.MetricMonthlyHours, ctx.ResolveOrDefault("monthly_hours", 730), 0.95)}, nil
}

func (m *RDSClusterInstanceMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("aurora_instance", "instance count unknown")}, nil
	}

	instanceClass := asset.Attr("instance_class")
	if instanceClass == "" {
		instanceClass = "db.r5.large"
	}

	engine := asset.Attr("engine")
	if engine == "" {
		engine = "aurora-mysql"
	}

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	return []clouds.CostUnit{
		clouds.NewCostUnit("instance", "hours", monthlyHours, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonRDS",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"instanceType":   instanceClass,
				"databaseEngine": normalizeAuroraEngine(engine),
			},
		}, 0.95),
	}, nil
}
