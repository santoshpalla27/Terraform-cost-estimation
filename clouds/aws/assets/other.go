// Package assets - AWS other asset builders
package assets

import (
	"context"
	"fmt"

	"terraform-cost/core/asset"
	"terraform-cost/core/types"
)

// CloudWatchLogGroupBuilder builds assets for aws_cloudwatch_log_group
type CloudWatchLogGroupBuilder struct {
	baseBuilder
}

// NewCloudWatchLogGroupBuilder creates a new CloudWatch Log Group builder
func NewCloudWatchLogGroupBuilder() asset.Builder {
	return &CloudWatchLogGroupBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_cloudwatch_log_group",
			category:     types.CategoryMonitoring,
		},
	}
}

// Build converts a raw CloudWatch Log Group into an asset
func (b *CloudWatchLogGroupBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	retentionDays := raw.Attributes.GetInt("retention_in_days")
	// 0 means never expire

	return &types.Asset{
		ID:       fmt.Sprintf("aws_cloudwatch_log_group.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryMonitoring,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"retention_in_days": {Value: retentionDays},
			"kms_key_id":        raw.Attributes["kms_key_id"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}

// KMSKeyBuilder builds assets for aws_kms_key
type KMSKeyBuilder struct {
	baseBuilder
}

// NewKMSKeyBuilder creates a new KMS Key builder
func NewKMSKeyBuilder() asset.Builder {
	return &KMSKeyBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_kms_key",
			category:     types.CategorySecurity,
		},
	}
}

// Build converts a raw KMS Key into an asset
func (b *KMSKeyBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	keySpec := raw.Attributes.GetString("customer_master_key_spec")
	if keySpec == "" {
		keySpec = "SYMMETRIC_DEFAULT"
	}

	return &types.Asset{
		ID:       fmt.Sprintf("aws_kms_key.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategorySecurity,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"customer_master_key_spec": {Value: keySpec},
			"key_usage":                raw.Attributes["key_usage"],
			"is_enabled":               raw.Attributes["is_enabled"],
			"multi_region":              raw.Attributes["multi_region"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}

// SecretsManagerSecretBuilder builds assets for aws_secretsmanager_secret
type SecretsManagerSecretBuilder struct {
	baseBuilder
}

// NewSecretsManagerSecretBuilder creates a new Secrets Manager builder
func NewSecretsManagerSecretBuilder() asset.Builder {
	return &SecretsManagerSecretBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_secretsmanager_secret",
			category:     types.CategorySecurity,
		},
	}
}

// Build converts a raw Secrets Manager Secret into an asset
func (b *SecretsManagerSecretBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	return &types.Asset{
		ID:       fmt.Sprintf("aws_secretsmanager_secret.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategorySecurity,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"recovery_window_in_days": raw.Attributes["recovery_window_in_days"],
			"kms_key_id":              raw.Attributes["kms_key_id"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}
