// Package assets - AWS database asset builders
package assets

import (
	"context"
	"fmt"

	"terraform-cost/core/asset"
	"terraform-cost/core/types"
)

// RDSInstanceBuilder builds assets for aws_db_instance
type RDSInstanceBuilder struct {
	baseBuilder
}

// NewRDSInstanceBuilder creates a new RDS instance builder
func NewRDSInstanceBuilder() asset.Builder {
	return &RDSInstanceBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_db_instance",
			category:     types.CategoryDatabase,
		},
	}
}

// Build converts a raw RDS instance into an asset
func (b *RDSInstanceBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	instanceClass := raw.Attributes.GetString("instance_class")
	if instanceClass == "" {
		instanceClass = "db.t3.micro"
	}

	engine := raw.Attributes.GetString("engine")
	storageType := raw.Attributes.GetString("storage_type")
	if storageType == "" {
		storageType = "gp2"
	}

	allocatedStorage := raw.Attributes.GetInt("allocated_storage")
	if allocatedStorage == 0 {
		allocatedStorage = 20
	}

	return &types.Asset{
		ID:       fmt.Sprintf("aws_db_instance.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryDatabase,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"instance_class":              {Value: instanceClass},
			"engine":                      {Value: engine},
			"engine_version":              raw.Attributes["engine_version"],
			"storage_type":                {Value: storageType},
			"allocated_storage":           {Value: allocatedStorage},
			"max_allocated_storage":       raw.Attributes["max_allocated_storage"],
			"iops":                        raw.Attributes["iops"],
			"storage_throughput":          raw.Attributes["storage_throughput"],
			"multi_az":                    raw.Attributes["multi_az"],
			"storage_encrypted":           raw.Attributes["storage_encrypted"],
			"performance_insights_enabled": raw.Attributes["performance_insights_enabled"],
			"backup_retention_period":     raw.Attributes["backup_retention_period"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}

// RDSClusterBuilder builds assets for aws_rds_cluster
type RDSClusterBuilder struct {
	baseBuilder
}

// NewRDSClusterBuilder creates a new RDS cluster builder
func NewRDSClusterBuilder() asset.Builder {
	return &RDSClusterBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_rds_cluster",
			category:     types.CategoryDatabase,
		},
	}
}

// Build converts a raw RDS cluster into an asset
func (b *RDSClusterBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	return &types.Asset{
		ID:       fmt.Sprintf("aws_rds_cluster.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryDatabase,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"engine":                  raw.Attributes["engine"],
			"engine_mode":             raw.Attributes["engine_mode"],
			"engine_version":          raw.Attributes["engine_version"],
			"serverlessv2_scaling_configuration": raw.Attributes["serverlessv2_scaling_configuration"],
			"backup_retention_period": raw.Attributes["backup_retention_period"],
			"storage_encrypted":       raw.Attributes["storage_encrypted"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}

// DynamoDBTableBuilder builds assets for aws_dynamodb_table
type DynamoDBTableBuilder struct {
	baseBuilder
}

// NewDynamoDBTableBuilder creates a new DynamoDB builder
func NewDynamoDBTableBuilder() asset.Builder {
	return &DynamoDBTableBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_dynamodb_table",
			category:     types.CategoryDatabase,
		},
	}
}

// Build converts a raw DynamoDB table into an asset
func (b *DynamoDBTableBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	billingMode := raw.Attributes.GetString("billing_mode")
	if billingMode == "" {
		billingMode = "PROVISIONED"
	}

	return &types.Asset{
		ID:       fmt.Sprintf("aws_dynamodb_table.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryDatabase,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"billing_mode":           {Value: billingMode},
			"read_capacity":          raw.Attributes["read_capacity"],
			"write_capacity":         raw.Attributes["write_capacity"],
			"global_secondary_index": raw.Attributes["global_secondary_index"],
			"stream_enabled":         raw.Attributes["stream_enabled"],
			"table_class":            raw.Attributes["table_class"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}

// ElastiCacheClusterBuilder builds assets for aws_elasticache_cluster
type ElastiCacheClusterBuilder struct {
	baseBuilder
}

// NewElastiCacheClusterBuilder creates a new ElastiCache builder
func NewElastiCacheClusterBuilder() asset.Builder {
	return &ElastiCacheClusterBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_elasticache_cluster",
			category:     types.CategoryDatabase,
		},
	}
}

// Build converts a raw ElastiCache cluster into an asset
func (b *ElastiCacheClusterBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	nodeType := raw.Attributes.GetString("node_type")
	if nodeType == "" {
		nodeType = "cache.t3.micro"
	}

	numCacheNodes := raw.Attributes.GetInt("num_cache_nodes")
	if numCacheNodes == 0 {
		numCacheNodes = 1
	}

	return &types.Asset{
		ID:       fmt.Sprintf("aws_elasticache_cluster.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryDatabase,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"node_type":       {Value: nodeType},
			"num_cache_nodes": {Value: numCacheNodes},
			"engine":          raw.Attributes["engine"],
			"engine_version":  raw.Attributes["engine_version"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}
