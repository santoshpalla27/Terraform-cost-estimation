// Package serverless - AWS Lambda cost mapper
// Clean-room implementation based on Lambda pricing model:
// - Requests (per 1M requests)
// - Duration (per GB-second)
// - Provisioned concurrency (per GB-hour)
// - Ephemeral storage (above 512MB baseline)
// - Architecture affects pricing (x86 vs ARM)
package serverless

import (
	"terraform-cost/clouds"
)

// LambdaMapper maps aws_lambda_function to cost units
type LambdaMapper struct{}

// NewLambdaMapper creates a Lambda mapper
func NewLambdaMapper() *LambdaMapper {
	return &LambdaMapper{}
}

// Cloud returns the cloud provider
func (m *LambdaMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *LambdaMapper) ResourceType() string {
	return "aws_lambda_function"
}

// BuildUsage extracts usage vectors from a Lambda function
func (m *LambdaMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	// Check for unknown cardinality
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyRequests, "unknown function count: "+asset.Cardinality.Reason),
		}, nil
	}

	// Lambda costs are highly usage-dependent
	// Request count and duration are not in Terraform config
	monthlyRequests := ctx.ResolveOrDefault("monthly_requests", 1000000)
	avgDurationMs := ctx.ResolveOrDefault("average_duration_ms", 100)

	// Lower confidence because these are estimates
	confidence := ctx.Confidence
	if confidence == 0 {
		confidence = 0.5
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyRequests, monthlyRequests, confidence),
		clouds.NewUsageVector("average_duration_ms", avgDurationMs, confidence),
	}, nil
}

// BuildCostUnits creates cost units for a Lambda function
func (m *LambdaMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	// Check for symbolic usage
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("requests", "Lambda cost unknown due to cardinality"),
		}, nil
	}

	// Extract attributes
	memorySize := asset.AttrFloat("memory_size", 128)

	// Architecture affects pricing (ARM is ~20% cheaper)
	architecture := asset.Attr("architectures.0")
	if architecture == "" {
		architecture = "x86_64"
	}

	// Get usage values
	monthlyRequests, _ := usageVecs.Get(clouds.MetricMonthlyRequests)
	avgDurationMs, _ := usageVecs.Get("average_duration_ms")

	// Calculate GB-seconds
	// GB-seconds = requests * (duration in seconds) * (memory in GB)
	gbSeconds := monthlyRequests * (avgDurationMs / 1000) * (memorySize / 1024)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	// Determine pricing tier based on architecture
	durationUsageType := "Lambda-GB-Second"
	if architecture == "arm64" {
		durationUsageType = "Lambda-GB-Second-ARM"
	}

	units := []clouds.CostUnit{
		// Requests (first 1M free, then $0.20 per 1M)
		clouds.NewCostUnit(
			"requests",
			"requests",
			monthlyRequests,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AWSLambda",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "Request",
				},
			},
			0.5,
		),

		// Duration (first 400K GB-seconds free)
		clouds.NewCostUnit(
			"duration",
			"GB-seconds",
			gbSeconds,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AWSLambda",
				Region:   region,
				Attributes: map[string]string{
					"usageType":    durationUsageType,
					"architecture": architecture,
				},
			},
			0.5,
		),
	}

	// Ephemeral storage (above 512MB baseline)
	ephemeralStorage := asset.AttrFloat("ephemeral_storage.0.size", 512)
	if ephemeralStorage > 512 {
		extraStorageMB := ephemeralStorage - 512
		// Storage-duration in GB-seconds
		storageGBSeconds := monthlyRequests * (avgDurationMs / 1000) * (extraStorageMB / 1024)

		units = append(units, clouds.NewCostUnit(
			"ephemeral_storage",
			"GB-seconds",
			storageGBSeconds,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AWSLambda",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "Lambda-Provisioned-GB-Second",
				},
			},
			0.5,
		))
	}

	return units, nil
}
