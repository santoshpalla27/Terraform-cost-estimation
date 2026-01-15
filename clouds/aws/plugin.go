// Package aws provides the AWS cloud plugin.
package aws

import (
	"terraform-cost/clouds/aws/assets"
	"terraform-cost/clouds/aws/pricing"
	"terraform-cost/clouds/aws/usage"
	"terraform-cost/core/asset"
	corePricing "terraform-cost/core/pricing"
	coreUsage "terraform-cost/core/usage"
	"terraform-cost/core/types"
)

// Plugin implements the AWS cloud plugin
type Plugin struct {
	initialized bool
	region      string
}

// New creates a new AWS plugin
func New() *Plugin {
	return &Plugin{
		region: "us-east-1",
	}
}

// NewWithRegion creates a new AWS plugin with a specific default region
func NewWithRegion(region string) *Plugin {
	return &Plugin{
		region: region,
	}
}

// Provider returns the cloud provider identifier
func (p *Plugin) Provider() types.Provider {
	return types.ProviderAWS
}

// Name returns a human-readable name
func (p *Plugin) Name() string {
	return "Amazon Web Services"
}

// Description returns a description of the plugin
func (p *Plugin) Description() string {
	return "Cost estimation for AWS infrastructure including EC2, RDS, S3, Lambda, and more"
}

// Initialize sets up the plugin
func (p *Plugin) Initialize() error {
	p.initialized = true
	return nil
}

// AssetBuilders returns asset builders for AWS resources
func (p *Plugin) AssetBuilders() []asset.Builder {
	return []asset.Builder{
		// Compute
		assets.NewEC2InstanceBuilder(),
		assets.NewEC2AutoScalingGroupBuilder(),
		assets.NewLambdaFunctionBuilder(),
		assets.NewECSServiceBuilder(),
		assets.NewEKSClusterBuilder(),

		// Storage
		assets.NewS3BucketBuilder(),
		assets.NewEBSVolumeBuilder(),
		assets.NewEFSFileSystemBuilder(),

		// Database
		assets.NewRDSInstanceBuilder(),
		assets.NewRDSClusterBuilder(),
		assets.NewDynamoDBTableBuilder(),
		assets.NewElastiCacheClusterBuilder(),

		// Network
		assets.NewNATGatewayBuilder(),
		assets.NewVPCEndpointBuilder(),
		assets.NewELBBuilder(),
		assets.NewALBBuilder(),
		assets.NewNLBBuilder(),

		// Other
		assets.NewCloudWatchLogGroupBuilder(),
		assets.NewKMSKeyBuilder(),
		assets.NewSecretsManagerSecretBuilder(),
	}
}

// UsageEstimators returns usage estimators for AWS resources
func (p *Plugin) UsageEstimators() []coreUsage.Estimator {
	return []coreUsage.Estimator{
		usage.NewEC2InstanceEstimator(),
		usage.NewS3BucketEstimator(),
		usage.NewRDSInstanceEstimator(),
		usage.NewLambdaFunctionEstimator(),
		usage.NewDynamoDBTableEstimator(),
		usage.NewNATGatewayEstimator(),
		usage.NewEBSVolumeEstimator(),
	}
}

// PricingSource returns the pricing source for AWS
func (p *Plugin) PricingSource() corePricing.Source {
	return pricing.NewAWSPricingSource(p.region)
}

// SupportedResourceTypes returns all supported AWS resource types
func (p *Plugin) SupportedResourceTypes() []string {
	return []string{
		// Compute
		"aws_instance",
		"aws_autoscaling_group",
		"aws_lambda_function",
		"aws_ecs_service",
		"aws_eks_cluster",
		"aws_eks_node_group",

		// Storage
		"aws_s3_bucket",
		"aws_ebs_volume",
		"aws_efs_file_system",

		// Database
		"aws_db_instance",
		"aws_rds_cluster",
		"aws_dynamodb_table",
		"aws_elasticache_cluster",
		"aws_elasticache_replication_group",

		// Network
		"aws_nat_gateway",
		"aws_vpc_endpoint",
		"aws_lb", // ALB/NLB
		"aws_elb",

		// Other
		"aws_cloudwatch_log_group",
		"aws_kms_key",
		"aws_secretsmanager_secret",
	}
}

// SupportedRegions returns all supported AWS regions
func (p *Plugin) SupportedRegions() []string {
	return []string{
		"us-east-1",
		"us-east-2",
		"us-west-1",
		"us-west-2",
		"eu-west-1",
		"eu-west-2",
		"eu-west-3",
		"eu-central-1",
		"eu-north-1",
		"ap-northeast-1",
		"ap-northeast-2",
		"ap-northeast-3",
		"ap-southeast-1",
		"ap-southeast-2",
		"ap-south-1",
		"sa-east-1",
		"ca-central-1",
	}
}
