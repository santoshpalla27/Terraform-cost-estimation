// Package database - AWS Aurora (RDS Cluster) cost mapper
// Pricing model:
// - Aurora Provisioned: instance hours + storage + I/O
// - Aurora Serverless v1: ACU-hours
// - Aurora Serverless v2: ACU-hours (more granular)
// - Storage: per GB-month (grows automatically)
// - I/O: per million requests (unless I/O-Optimized)
// - Backtrack: per million change records
package database

import (
	"terraform-cost/clouds"
)

// AuroraMapper maps aws_rds_cluster to cost units
type AuroraMapper struct{}

// NewAuroraMapper creates an Aurora mapper
func NewAuroraMapper() *AuroraMapper {
	return &AuroraMapper{}
}

// Cloud returns the cloud provider
func (m *AuroraMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *AuroraMapper) ResourceType() string {
	return "aws_rds_cluster"
}

// BuildUsage extracts usage vectors
func (m *AuroraMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown Aurora cluster count: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)
	storageGB := ctx.ResolveOrDefault("storage_gb", 10)
	ioRequestsMillions := ctx.ResolveOrDefault("io_requests_millions", 10)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
		clouds.NewUsageVector(clouds.MetricStorageGB, storageGB, 0.5),
		clouds.NewUsageVector("io_requests_millions", ioRequestsMillions, 0.5),
	}, nil
}

// BuildCostUnits creates cost units
func (m *AuroraMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("aurora", "Aurora cost unknown due to cardinality"),
		}, nil
	}

	engine := asset.Attr("engine")
	if engine == "" {
		engine = "aurora-mysql"
	}

	engineMode := asset.Attr("engine_mode")
	if engineMode == "" {
		engineMode = "provisioned"
	}

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)
	storageGB, _ := usageVecs.Get(clouds.MetricStorageGB)
	ioRequests, _ := usageVecs.Get("io_requests_millions")

	var units []clouds.CostUnit

	// Engine mode determines compute pricing
	switch engineMode {
	case "serverless":
		// Aurora Serverless v1
		minCapacity := asset.AttrFloat("scaling_configuration.0.min_capacity", 2)
		units = append(units, clouds.NewCostUnit(
			"serverless_acu",
			"ACU-hours",
			minCapacity*monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonRDS",
				Region:   region,
				Attributes: map[string]string{
					"databaseEngine": normalizeAuroraEngine(engine),
					"usageType":      "Aurora:ServerlessUsage",
				},
			},
			0.7, // Lower confidence - actual ACU usage varies
		))

	case "serverlessv2":
		// Aurora Serverless v2
		minCapacity := asset.AttrFloat("serverlessv2_scaling_configuration.0.min_capacity", 0.5)
		units = append(units, clouds.NewCostUnit(
			"serverlessv2_acu",
			"ACU-hours",
			minCapacity*monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonRDS",
				Region:   region,
				Attributes: map[string]string{
					"databaseEngine": normalizeAuroraEngine(engine),
					"usageType":      "Aurora:ServerlessV2Usage",
				},
			},
			0.7,
		))

	default:
		// Provisioned - instances are separate aws_rds_cluster_instance resources
		// Cluster itself has no compute cost
	}

	// Storage (always charged)
	units = append(units, clouds.NewCostUnit(
		"storage",
		"GB-months",
		storageGB,
		clouds.RateKey{
			Provider: providerID,
			Service:  "AmazonRDS",
			Region:   region,
			Attributes: map[string]string{
				"databaseEngine": normalizeAuroraEngine(engine),
				"usageType":      "Aurora:StorageUsage",
			},
		},
		0.5,
	))

	// I/O (unless I/O-Optimized storage type)
	storageType := asset.Attr("storage_type")
	if storageType != "aurora-iopt1" && ioRequests > 0 {
		units = append(units, clouds.NewCostUnit(
			"io_requests",
			"million-requests",
			ioRequests,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonRDS",
				Region:   region,
				Attributes: map[string]string{
					"databaseEngine": normalizeAuroraEngine(engine),
					"usageType":      "Aurora:IOUsage",
				},
			},
			0.5,
		))
	}

	// Backtrack (if enabled)
	if backtrackWindow := asset.AttrInt("backtrack_window", 0); backtrackWindow > 0 {
		units = append(units, clouds.SymbolicCost(
			"backtrack",
			"backtrack cost depends on change rate",
		))
	}

	return units, nil
}

func normalizeAuroraEngine(engine string) string {
	switch engine {
	case "aurora", "aurora-mysql":
		return "Aurora MySQL"
	case "aurora-postgresql":
		return "Aurora PostgreSQL"
	default:
		return engine
	}
}
