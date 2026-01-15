// Package usage provides AWS usage estimators.
package usage

import (
	"context"

	"terraform-cost/core/types"
	coreUsage "terraform-cost/core/usage"
)

// baseEstimator provides common functionality for AWS estimators
type baseEstimator struct {
	resourceType string
}

func (e *baseEstimator) Provider() types.Provider {
	return types.ProviderAWS
}

func (e *baseEstimator) ResourceType() string {
	return e.resourceType
}

// EC2InstanceEstimator estimates usage for aws_instance
type EC2InstanceEstimator struct {
	baseEstimator
}

// NewEC2InstanceEstimator creates a new EC2 instance estimator
func NewEC2InstanceEstimator() coreUsage.Estimator {
	return &EC2InstanceEstimator{
		baseEstimator: baseEstimator{resourceType: "aws_instance"},
	}
}

// Estimate produces usage vectors for an EC2 instance
func (e *EC2InstanceEstimator) Estimate(ctx context.Context, asset *types.Asset, uctx *coreUsage.Context) ([]types.UsageVector, error) {
	// Default: 24/7 operation = 730 hours/month
	monthlyHours := 730.0
	confidence := 0.8

	// Check for overrides
	if uctx != nil && uctx.Profile != nil {
		if overrides := uctx.Profile.GetOverrides(asset.Address); len(overrides) > 0 {
			return overrides, nil
		}
	}

	// Adjust based on environment
	if uctx != nil && uctx.Environment == "development" {
		// Dev environments typically run 8 hours/day, 5 days/week
		monthlyHours = 8 * 5 * 4 // ~160 hours
		confidence = 0.6
	}

	return []types.UsageVector{
		{
			Metric:      types.MetricMonthlyHours,
			Value:       monthlyHours,
			Confidence:  confidence,
			Source:      types.SourceDefault,
			Description: "Estimated monthly runtime hours",
		},
	}, nil
}

// S3BucketEstimator estimates usage for aws_s3_bucket
type S3BucketEstimator struct {
	baseEstimator
}

// NewS3BucketEstimator creates a new S3 bucket estimator
func NewS3BucketEstimator() coreUsage.Estimator {
	return &S3BucketEstimator{
		baseEstimator: baseEstimator{resourceType: "aws_s3_bucket"},
	}
}

// Estimate produces usage vectors for an S3 bucket
func (e *S3BucketEstimator) Estimate(ctx context.Context, asset *types.Asset, uctx *coreUsage.Context) ([]types.UsageVector, error) {
	// Check for overrides
	if uctx != nil && uctx.Profile != nil {
		if overrides := uctx.Profile.GetOverrides(asset.Address); len(overrides) > 0 {
			return overrides, nil
		}
	}

	// Default estimates - these should be overridden in production
	return []types.UsageVector{
		{
			Metric:      types.MetricMonthlyGBStorage,
			Value:       100, // 100 GB default
			Confidence:  0.3, // Low confidence - highly variable
			Source:      types.SourceDefault,
			Description: "Estimated monthly storage in GB",
		},
		{
			Metric:      types.MetricMonthlyGetOperations,
			Value:       100000, // 100k GET requests
			Confidence:  0.3,
			Source:      types.SourceDefault,
			Description: "Estimated monthly GET requests",
		},
		{
			Metric:      types.MetricMonthlyPutOperations,
			Value:       10000, // 10k PUT requests
			Confidence:  0.3,
			Source:      types.SourceDefault,
			Description: "Estimated monthly PUT requests",
		},
		{
			Metric:      types.MetricMonthlyGBTransferOut,
			Value:       10, // 10 GB transfer out
			Confidence:  0.3,
			Source:      types.SourceDefault,
			Description: "Estimated monthly data transfer out in GB",
		},
	}, nil
}

// RDSInstanceEstimator estimates usage for aws_db_instance
type RDSInstanceEstimator struct {
	baseEstimator
}

// NewRDSInstanceEstimator creates a new RDS instance estimator
func NewRDSInstanceEstimator() coreUsage.Estimator {
	return &RDSInstanceEstimator{
		baseEstimator: baseEstimator{resourceType: "aws_db_instance"},
	}
}

// Estimate produces usage vectors for an RDS instance
func (e *RDSInstanceEstimator) Estimate(ctx context.Context, asset *types.Asset, uctx *coreUsage.Context) ([]types.UsageVector, error) {
	// Check for overrides
	if uctx != nil && uctx.Profile != nil {
		if overrides := uctx.Profile.GetOverrides(asset.Address); len(overrides) > 0 {
			return overrides, nil
		}
	}

	// Get storage from attributes
	storage := asset.Attributes.GetInt("allocated_storage")
	if storage == 0 {
		storage = 20
	}

	return []types.UsageVector{
		{
			Metric:      types.MetricMonthlyHours,
			Value:       730, // 24/7
			Confidence:  0.9,
			Source:      types.SourceDefault,
			Description: "Database typically runs 24/7",
		},
		{
			Metric:      types.MetricMonthlyGBStorage,
			Value:       float64(storage),
			Confidence:  0.95,
			Source:      types.SourceTerraform,
			Description: "Storage from Terraform configuration",
		},
		{
			Metric:      types.MetricMonthlyBackupStorageGB,
			Value:       float64(storage), // Assume backup equals storage
			Confidence:  0.5,
			Source:      types.SourceDefault,
			Description: "Estimated backup storage",
		},
	}, nil
}

// LambdaFunctionEstimator estimates usage for aws_lambda_function
type LambdaFunctionEstimator struct {
	baseEstimator
}

// NewLambdaFunctionEstimator creates a new Lambda function estimator
func NewLambdaFunctionEstimator() coreUsage.Estimator {
	return &LambdaFunctionEstimator{
		baseEstimator: baseEstimator{resourceType: "aws_lambda_function"},
	}
}

// Estimate produces usage vectors for a Lambda function
func (e *LambdaFunctionEstimator) Estimate(ctx context.Context, asset *types.Asset, uctx *coreUsage.Context) ([]types.UsageVector, error) {
	// Check for overrides
	if uctx != nil && uctx.Profile != nil {
		if overrides := uctx.Profile.GetOverrides(asset.Address); len(overrides) > 0 {
			return overrides, nil
		}
	}

	memorySize := asset.Attributes.GetInt("memory_size")
	if memorySize == 0 {
		memorySize = 128
	}

	timeout := asset.Attributes.GetInt("timeout")
	if timeout == 0 {
		timeout = 3
	}

	// Default: 1 million invocations, average 500ms duration
	invocations := 1000000.0
	avgDurationMs := float64(timeout) * 1000 * 0.5 // 50% of timeout

	return []types.UsageVector{
		{
			Metric:      types.MetricMonthlyInvocations,
			Value:       invocations,
			Confidence:  0.3, // Very variable
			Source:      types.SourceDefault,
			Description: "Estimated monthly invocations",
		},
		{
			Metric:      types.MetricMonthlyDurationMs,
			Value:       avgDurationMs,
			Confidence:  0.3,
			Source:      types.SourceDefault,
			Description: "Average execution duration in ms",
		},
		{
			Metric:      types.MetricMonthlyGBSeconds,
			Value:       (float64(memorySize) / 1024) * (avgDurationMs / 1000) * invocations,
			Confidence:  0.3,
			Source:      types.SourceDefault,
			Description: "Compute GB-seconds",
		},
	}, nil
}

// DynamoDBTableEstimator estimates usage for aws_dynamodb_table
type DynamoDBTableEstimator struct {
	baseEstimator
}

// NewDynamoDBTableEstimator creates a new DynamoDB table estimator
func NewDynamoDBTableEstimator() coreUsage.Estimator {
	return &DynamoDBTableEstimator{
		baseEstimator: baseEstimator{resourceType: "aws_dynamodb_table"},
	}
}

// Estimate produces usage vectors for a DynamoDB table
func (e *DynamoDBTableEstimator) Estimate(ctx context.Context, asset *types.Asset, uctx *coreUsage.Context) ([]types.UsageVector, error) {
	// Check for overrides
	if uctx != nil && uctx.Profile != nil {
		if overrides := uctx.Profile.GetOverrides(asset.Address); len(overrides) > 0 {
			return overrides, nil
		}
	}

	billingMode := asset.Attributes.GetString("billing_mode")

	vectors := []types.UsageVector{
		{
			Metric:      types.MetricMonthlyGBStorage,
			Value:       10, // 10 GB default
			Confidence:  0.3,
			Source:      types.SourceDefault,
			Description: "Estimated table storage",
		},
	}

	if billingMode == "PAY_PER_REQUEST" {
		vectors = append(vectors,
			types.UsageVector{
				Metric:      types.MetricMonthlyReadRequests,
				Value:       1000000,
				Confidence:  0.3,
				Source:      types.SourceDefault,
				Description: "Estimated read request units",
			},
			types.UsageVector{
				Metric:      types.MetricMonthlyWriteRequests,
				Value:       100000,
				Confidence:  0.3,
				Source:      types.SourceDefault,
				Description: "Estimated write request units",
			},
		)
	}

	return vectors, nil
}

// NATGatewayEstimator estimates usage for aws_nat_gateway
type NATGatewayEstimator struct {
	baseEstimator
}

// NewNATGatewayEstimator creates a new NAT Gateway estimator
func NewNATGatewayEstimator() coreUsage.Estimator {
	return &NATGatewayEstimator{
		baseEstimator: baseEstimator{resourceType: "aws_nat_gateway"},
	}
}

// Estimate produces usage vectors for a NAT Gateway
func (e *NATGatewayEstimator) Estimate(ctx context.Context, asset *types.Asset, uctx *coreUsage.Context) ([]types.UsageVector, error) {
	// Check for overrides
	if uctx != nil && uctx.Profile != nil {
		if overrides := uctx.Profile.GetOverrides(asset.Address); len(overrides) > 0 {
			return overrides, nil
		}
	}

	return []types.UsageVector{
		{
			Metric:      types.MetricMonthlyHours,
			Value:       730, // 24/7
			Confidence:  0.95,
			Source:      types.SourceDefault,
			Description: "NAT Gateway runs continuously",
		},
		{
			Metric:      types.MetricMonthlyGB,
			Value:       100, // 100 GB data processed
			Confidence:  0.3,
			Source:      types.SourceDefault,
			Description: "Estimated data processed through NAT",
		},
	}, nil
}

// EBSVolumeEstimator estimates usage for aws_ebs_volume
type EBSVolumeEstimator struct {
	baseEstimator
}

// NewEBSVolumeEstimator creates a new EBS volume estimator
func NewEBSVolumeEstimator() coreUsage.Estimator {
	return &EBSVolumeEstimator{
		baseEstimator: baseEstimator{resourceType: "aws_ebs_volume"},
	}
}

// Estimate produces usage vectors for an EBS volume
func (e *EBSVolumeEstimator) Estimate(ctx context.Context, asset *types.Asset, uctx *coreUsage.Context) ([]types.UsageVector, error) {
	// Check for overrides
	if uctx != nil && uctx.Profile != nil {
		if overrides := uctx.Profile.GetOverrides(asset.Address); len(overrides) > 0 {
			return overrides, nil
		}
	}

	size := asset.Attributes.GetInt("size")
	if size == 0 {
		size = 8
	}

	return []types.UsageVector{
		{
			Metric:      types.MetricMonthlyGBStorage,
			Value:       float64(size),
			Confidence:  0.95,
			Source:      types.SourceTerraform,
			Description: "Volume size from Terraform configuration",
		},
		{
			Metric:      types.MetricMonthlySnapshots,
			Value:       1, // One snapshot per month
			Confidence:  0.5,
			Source:      types.SourceDefault,
			Description: "Estimated monthly snapshots",
		},
	}, nil
}
