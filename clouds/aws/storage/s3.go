// Package storage - AWS S3 cost mapper
// Clean-room implementation based on S3 pricing model:
// - Storage (per GB-month, varies by storage class)
// - Requests (PUT, GET, LIST, etc.)
// - Data transfer (out to internet, cross-region)
// - Management features (analytics, inventory, replication)
package storage

import (
	"terraform-cost/clouds"
)

// S3Mapper maps aws_s3_bucket to cost units
type S3Mapper struct{}

// NewS3Mapper creates an S3 mapper
func NewS3Mapper() *S3Mapper {
	return &S3Mapper{}
}

// Cloud returns the cloud provider
func (m *S3Mapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *S3Mapper) ResourceType() string {
	return "aws_s3_bucket"
}

// BuildUsage extracts usage vectors from an S3 bucket
func (m *S3Mapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	// S3 buckets always have known cardinality (1 bucket)
	// But storage size and request volume are usage-dependent

	// Storage (usage-based, needs override or default)
	storageGB := ctx.ResolveOrDefault("storage_gb", 100)

	// Requests (usage-based)
	putRequests := ctx.ResolveOrDefault("put_requests", 10000)
	getRequests := ctx.ResolveOrDefault("get_requests", 100000)

	// Data transfer out
	dataTransferGB := ctx.ResolveOrDefault("data_transfer_out_gb", 10)

	// Lower confidence because these are estimates
	confidence := ctx.Confidence
	if confidence == 0 {
		confidence = 0.5 // Usage-based = moderate confidence
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricStorageGB, storageGB, confidence),
		clouds.NewUsageVector("put_requests", putRequests, confidence),
		clouds.NewUsageVector("get_requests", getRequests, confidence),
		clouds.NewUsageVector(clouds.MetricDataTransferGB, dataTransferGB, confidence),
	}, nil
}

// BuildCostUnits creates cost units for an S3 bucket
func (m *S3Mapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	// Determine storage class
	storageClass := "STANDARD"
	if sc := asset.Attr("storage_class"); sc != "" {
		storageClass = sc
	}

	// Get usage values
	storageGB, _ := usageVecs.Get(clouds.MetricStorageGB)
	putRequests, _ := usageVecs.Get("put_requests")
	getRequests, _ := usageVecs.Get("get_requests")
	dataTransferGB, _ := usageVecs.Get(clouds.MetricDataTransferGB)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	units := []clouds.CostUnit{
		// Storage
		clouds.NewCostUnit(
			"storage",
			"GB-months",
			storageGB,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonS3",
				Region:   region,
				Attributes: map[string]string{
					"storageClass": storageClass,
					"usageType":    "TimedStorage-" + storageClass,
				},
			},
			0.5,
		),

		// PUT requests
		clouds.NewCostUnit(
			"put_requests",
			"requests",
			putRequests,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonS3",
				Region:   region,
				Attributes: map[string]string{
					"storageClass": storageClass,
					"usageType":    "Requests-Tier1",
				},
			},
			0.5,
		),

		// GET requests
		clouds.NewCostUnit(
			"get_requests",
			"requests",
			getRequests,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonS3",
				Region:   region,
				Attributes: map[string]string{
					"storageClass": storageClass,
					"usageType":    "Requests-Tier2",
				},
			},
			0.5,
		),
	}

	// Data transfer (first 1GB free, then tiered)
	if dataTransferGB > 0 {
		units = append(units, clouds.NewCostUnit(
			"data_transfer_out",
			"GB",
			dataTransferGB,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AWSDataTransfer",
				Region:   region,
				Attributes: map[string]string{
					"transferType": "AWS Outbound",
					"toLocation":   "External",
				},
			},
			0.5,
		))
	}

	return units, nil
}
