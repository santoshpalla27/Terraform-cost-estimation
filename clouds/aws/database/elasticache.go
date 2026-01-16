// Package database - AWS ElastiCache cost mapper
// Pricing model:
// - Node hours (by node type, engine)
// - Data transfer
// - Backup storage (beyond free tier)
package database

import (
	"terraform-cost/clouds"
)

// ElastiCacheMapper maps aws_elasticache_cluster to cost units
type ElastiCacheMapper struct{}

// NewElastiCacheMapper creates an ElastiCache mapper
func NewElastiCacheMapper() *ElastiCacheMapper {
	return &ElastiCacheMapper{}
}

// Cloud returns the cloud provider
func (m *ElastiCacheMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *ElastiCacheMapper) ResourceType() string {
	return "aws_elasticache_cluster"
}

// BuildUsage extracts usage vectors
func (m *ElastiCacheMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown cluster count: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
	}, nil
}

// BuildCostUnits creates cost units
func (m *ElastiCacheMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("cache_nodes", "ElastiCache cost unknown"),
		}, nil
	}

	nodeType := asset.Attr("node_type")
	if nodeType == "" {
		nodeType = "cache.t3.micro"
	}

	engine := asset.Attr("engine")
	if engine == "" {
		engine = "redis"
	}

	numCacheNodes := asset.AttrInt("num_cache_nodes", 1)
	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"cache_nodes",
			"node-hours",
			float64(numCacheNodes)*monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonElastiCache",
				Region:   region,
				Attributes: map[string]string{
					"nodeType":   nodeType,
					"cacheEngine": engine,
					"usageType":  "NodeUsage:" + nodeType,
				},
			},
			0.95,
		),
	}, nil
}

// ElastiCacheReplicationGroupMapper maps aws_elasticache_replication_group
type ElastiCacheReplicationGroupMapper struct{}

// NewElastiCacheReplicationGroupMapper creates mapper
func NewElastiCacheReplicationGroupMapper() *ElastiCacheReplicationGroupMapper {
	return &ElastiCacheReplicationGroupMapper{}
}

// Cloud returns the cloud provider
func (m *ElastiCacheReplicationGroupMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *ElastiCacheReplicationGroupMapper) ResourceType() string {
	return "aws_elasticache_replication_group"
}

// BuildUsage extracts usage vectors
func (m *ElastiCacheReplicationGroupMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown replication group: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
	}, nil
}

// BuildCostUnits creates cost units
func (m *ElastiCacheReplicationGroupMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("cache_nodes", "ElastiCache Replication Group cost unknown"),
		}, nil
	}

	nodeType := asset.Attr("node_type")
	if nodeType == "" {
		nodeType = "cache.t3.micro"
	}

	// Calculate total nodes
	numNodeGroups := asset.AttrInt("num_node_groups", 1)
	replicasPerGroup := asset.AttrInt("replicas_per_node_group", 1)
	totalNodes := numNodeGroups * (replicasPerGroup + 1) // +1 for primary

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"cache_nodes",
			"node-hours",
			float64(totalNodes)*monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonElastiCache",
				Region:   region,
				Attributes: map[string]string{
					"nodeType":   nodeType,
					"cacheEngine": "redis",
					"usageType":  "NodeUsage:" + nodeType,
				},
			},
			0.95,
		),
	}, nil
}
