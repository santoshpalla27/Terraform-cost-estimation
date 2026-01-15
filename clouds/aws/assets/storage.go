// Package assets - AWS storage asset builders
package assets

import (
	"context"
	"fmt"

	"terraform-cost/core/asset"
	"terraform-cost/core/types"
)

// S3BucketBuilder builds assets for aws_s3_bucket
type S3BucketBuilder struct {
	baseBuilder
}

// NewS3BucketBuilder creates a new S3 bucket builder
func NewS3BucketBuilder() asset.Builder {
	return &S3BucketBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_s3_bucket",
			category:     types.CategoryStorage,
		},
	}
}

// Build converts a raw S3 bucket into an asset
func (b *S3BucketBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	return &types.Asset{
		ID:       fmt.Sprintf("aws_s3_bucket.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryStorage,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"bucket": raw.Attributes["bucket"],
			"acl":    raw.Attributes["acl"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}

// EBSVolumeBuilder builds assets for aws_ebs_volume
type EBSVolumeBuilder struct {
	baseBuilder
}

// NewEBSVolumeBuilder creates a new EBS volume builder
func NewEBSVolumeBuilder() asset.Builder {
	return &EBSVolumeBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_ebs_volume",
			category:     types.CategoryStorage,
		},
	}
}

// Build converts a raw EBS volume into an asset
func (b *EBSVolumeBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	volumeType := raw.Attributes.GetString("type")
	if volumeType == "" {
		volumeType = "gp3"
	}

	size := raw.Attributes.GetInt("size")
	if size == 0 {
		size = 8
	}

	az := raw.Attributes.GetString("availability_zone")
	region := ""
	if len(az) > 0 {
		region = az[:len(az)-1]
	}

	return &types.Asset{
		ID:       fmt.Sprintf("aws_ebs_volume.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryStorage,
		Type:     raw.Type,
		Name:     raw.Name,
		Region:   types.Region(region),
		Attributes: types.Attributes{
			"type":              {Value: volumeType},
			"size":              {Value: size},
			"iops":              raw.Attributes["iops"],
			"throughput":        raw.Attributes["throughput"],
			"encrypted":         raw.Attributes["encrypted"],
			"availability_zone": {Value: az},
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}

// EFSFileSystemBuilder builds assets for aws_efs_file_system
type EFSFileSystemBuilder struct {
	baseBuilder
}

// NewEFSFileSystemBuilder creates a new EFS builder
func NewEFSFileSystemBuilder() asset.Builder {
	return &EFSFileSystemBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_efs_file_system",
			category:     types.CategoryStorage,
		},
	}
}

// Build converts a raw EFS file system into an asset
func (b *EFSFileSystemBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	throughputMode := raw.Attributes.GetString("throughput_mode")
	if throughputMode == "" {
		throughputMode = "bursting"
	}

	performanceMode := raw.Attributes.GetString("performance_mode")
	if performanceMode == "" {
		performanceMode = "generalPurpose"
	}

	return &types.Asset{
		ID:       fmt.Sprintf("aws_efs_file_system.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryStorage,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"throughput_mode":                  {Value: throughputMode},
			"performance_mode":                 {Value: performanceMode},
			"provisioned_throughput_in_mibps":  raw.Attributes["provisioned_throughput_in_mibps"],
			"encrypted":                        raw.Attributes["encrypted"],
			"lifecycle_policy":                 raw.Attributes["lifecycle_policy"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}
