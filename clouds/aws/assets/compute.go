// Package assets - AWS compute asset builders
package assets

import (
	"context"
	"fmt"

	"terraform-cost/core/asset"
	"terraform-cost/core/types"
)

// EC2AutoScalingGroupBuilder builds assets for aws_autoscaling_group
type EC2AutoScalingGroupBuilder struct {
	baseBuilder
}

// NewEC2AutoScalingGroupBuilder creates a new ASG builder
func NewEC2AutoScalingGroupBuilder() asset.Builder {
	return &EC2AutoScalingGroupBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_autoscaling_group",
			category:     types.CategoryCompute,
		},
	}
}

// Build converts a raw ASG into an asset
func (b *EC2AutoScalingGroupBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	return &types.Asset{
		ID:       fmt.Sprintf("aws_autoscaling_group.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryCompute,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"min_size":              raw.Attributes["min_size"],
			"max_size":              raw.Attributes["max_size"],
			"desired_capacity":      raw.Attributes["desired_capacity"],
			"launch_configuration":  raw.Attributes["launch_configuration"],
			"launch_template":       raw.Attributes["launch_template"],
			"mixed_instances_policy": raw.Attributes["mixed_instances_policy"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
	}, nil
}

// LambdaFunctionBuilder builds assets for aws_lambda_function
type LambdaFunctionBuilder struct {
	baseBuilder
}

// NewLambdaFunctionBuilder creates a new Lambda builder
func NewLambdaFunctionBuilder() asset.Builder {
	return &LambdaFunctionBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_lambda_function",
			category:     types.CategoryServerless,
		},
	}
}

// Build converts a raw Lambda function into an asset
func (b *LambdaFunctionBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	memorySize := raw.Attributes.GetInt("memory_size")
	if memorySize == 0 {
		memorySize = 128 // Default
	}

	timeout := raw.Attributes.GetInt("timeout")
	if timeout == 0 {
		timeout = 3 // Default
	}

	return &types.Asset{
		ID:       fmt.Sprintf("aws_lambda_function.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryServerless,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"memory_size":       {Value: memorySize},
			"timeout":           {Value: timeout},
			"runtime":           raw.Attributes["runtime"],
			"architectures":     raw.Attributes["architectures"],
			"ephemeral_storage": raw.Attributes["ephemeral_storage"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
	}, nil
}

// ECSServiceBuilder builds assets for aws_ecs_service
type ECSServiceBuilder struct {
	baseBuilder
}

// NewECSServiceBuilder creates a new ECS service builder
func NewECSServiceBuilder() asset.Builder {
	return &ECSServiceBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_ecs_service",
			category:     types.CategoryContainer,
		},
	}
}

// Build converts a raw ECS service into an asset
func (b *ECSServiceBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	desiredCount := raw.Attributes.GetInt("desired_count")
	if desiredCount == 0 {
		desiredCount = 1
	}

	return &types.Asset{
		ID:       fmt.Sprintf("aws_ecs_service.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryContainer,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"desired_count":   {Value: desiredCount},
			"task_definition": raw.Attributes["task_definition"],
			"launch_type":     raw.Attributes["launch_type"],
			"capacity_provider_strategy": raw.Attributes["capacity_provider_strategy"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
	}, nil
}

// EKSClusterBuilder builds assets for aws_eks_cluster
type EKSClusterBuilder struct {
	baseBuilder
}

// NewEKSClusterBuilder creates a new EKS cluster builder
func NewEKSClusterBuilder() asset.Builder {
	return &EKSClusterBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_eks_cluster",
			category:     types.CategoryContainer,
		},
	}
}

// Build converts a raw EKS cluster into an asset
func (b *EKSClusterBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	return &types.Asset{
		ID:       fmt.Sprintf("aws_eks_cluster.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryContainer,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"version": raw.Attributes["version"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
	}, nil
}
