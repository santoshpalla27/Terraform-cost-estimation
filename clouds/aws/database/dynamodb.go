// Package database - AWS DynamoDB cost mapper
// Clean-room implementation based on DynamoDB pricing model:
// - On-demand: Read/Write Request Units
// - Provisioned: Read/Write Capacity Units
// - Storage (per GB-month)
// - Global tables (additional per-replicated write)
// - Backups (per GB-month)
// - Streams (per 100K read requests)
package database

import (
	"terraform-cost/clouds"
)

// DynamoDBMapper maps aws_dynamodb_table to cost units
type DynamoDBMapper struct{}

// NewDynamoDBMapper creates a DynamoDB mapper
func NewDynamoDBMapper() *DynamoDBMapper {
	return &DynamoDBMapper{}
}

// Cloud returns the cloud provider
func (m *DynamoDBMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *DynamoDBMapper) ResourceType() string {
	return "aws_dynamodb_table"
}

// BuildUsage extracts usage vectors from a DynamoDB table
func (m *DynamoDBMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	// Check for unknown cardinality
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricStorageGB, "unknown table count: "+asset.Cardinality.Reason),
		}, nil
	}

	// DynamoDB usage is highly dependent on application patterns
	// Storage grows with data
	storageGB := ctx.ResolveOrDefault("storage_gb", 10)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricStorageGB, storageGB, 0.5),
	}, nil
}

// BuildCostUnits creates cost units for a DynamoDB table
func (m *DynamoDBMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	// Check for symbolic usage
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("capacity", "DynamoDB cost unknown due to cardinality"),
		}, nil
	}

	// Determine billing mode
	billingMode := asset.Attr("billing_mode")
	if billingMode == "" {
		billingMode = "PROVISIONED" // Default
	}

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	var units []clouds.CostUnit

	if billingMode == "PAY_PER_REQUEST" {
		// On-demand mode - charged per request
		// Need usage data for accurate estimation
		units = []clouds.CostUnit{
			clouds.SymbolicCost("read_requests", "on-demand read usage unknown"),
			clouds.SymbolicCost("write_requests", "on-demand write usage unknown"),
		}
	} else {
		// Provisioned mode
		readCapacity := asset.AttrFloat("read_capacity", 5)
		writeCapacity := asset.AttrFloat("write_capacity", 5)

		units = []clouds.CostUnit{
			// Read Capacity Units
			clouds.NewCostUnit(
				"read_capacity",
				"RCU-hours",
				readCapacity*730, // RCUs * hours/month
				clouds.RateKey{
					Provider: providerID,
					Service:  "AmazonDynamoDB",
					Region:   region,
					Attributes: map[string]string{
						"usageType": "ReadCapacityUnit-Hrs",
					},
				},
				0.9,
			),

			// Write Capacity Units
			clouds.NewCostUnit(
				"write_capacity",
				"WCU-hours",
				writeCapacity*730,
				clouds.RateKey{
					Provider: providerID,
					Service:  "AmazonDynamoDB",
					Region:   region,
					Attributes: map[string]string{
						"usageType": "WriteCapacityUnit-Hrs",
					},
				},
				0.9,
			),
		}
	}

	// Storage (always charged)
	storageGB, _ := usageVecs.Get(clouds.MetricStorageGB)
	units = append(units, clouds.NewCostUnit(
		"storage",
		"GB-months",
		storageGB,
		clouds.RateKey{
			Provider: providerID,
			Service:  "AmazonDynamoDB",
			Region:   region,
			Attributes: map[string]string{
				"usageType": "TimedStorage-ByteHrs",
			},
		},
		0.5, // Lower confidence - storage is usage-dependent
	))

	// Global tables (if replicas exist)
	if replicas := asset.AttrInt("replica.#", 0); replicas > 0 {
		// Each replica incurs additional write costs
		if billingMode != "PAY_PER_REQUEST" {
			writeCapacity := asset.AttrFloat("write_capacity", 5)
			for i := 0; i < replicas; i++ {
				units = append(units, clouds.NewCostUnit(
					"replica_write_capacity",
					"rWCU-hours",
					writeCapacity*730,
					clouds.RateKey{
						Provider: providerID,
						Service:  "AmazonDynamoDB",
						Region:   region, // Would be replica region
						Attributes: map[string]string{
							"usageType": "ReplicatedWriteCapacityUnit-Hrs",
						},
					},
					0.8,
				))
			}
		}
	}

	// Streams (if enabled)
	if streamEnabled := asset.Attr("stream_enabled"); streamEnabled == "true" {
		units = append(units, clouds.SymbolicCost(
			"streams",
			"stream read requests depend on consumer patterns",
		))
	}

	return units, nil
}
