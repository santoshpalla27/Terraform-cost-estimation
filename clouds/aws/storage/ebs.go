// Package storage - AWS EBS cost mapper
// Clean-room implementation based on EBS pricing model:
// - Storage (per GB-month, varies by volume type)
// - Provisioned IOPS (for io1, io2)
// - Provisioned throughput (for gp3)
// - Snapshots
package storage

import (
	"terraform-cost/clouds"
)

// EBSMapper maps aws_ebs_volume to cost units
type EBSMapper struct{}

// NewEBSMapper creates an EBS mapper
func NewEBSMapper() *EBSMapper {
	return &EBSMapper{}
}

// Cloud returns the cloud provider
func (m *EBSMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *EBSMapper) ResourceType() string {
	return "aws_ebs_volume"
}

// BuildUsage extracts usage vectors from an EBS volume
func (m *EBSMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	// Check for unknown cardinality
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricStorageGB, "unknown volume count: "+asset.Cardinality.Reason),
		}, nil
	}

	// EBS volumes have deterministic attributes
	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricStorageGB, 1, 1.0), // Placeholder, actual from attrs
	}, nil
}

// BuildCostUnits creates cost units for an EBS volume
func (m *EBSMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	// Check for symbolic usage
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("storage", "EBS cost unknown due to cardinality"),
		}, nil
	}

	// Extract volume attributes
	volumeType := asset.Attr("type")
	if volumeType == "" {
		volumeType = "gp3" // Default in newer AWS
	}

	size := asset.AttrFloat("size", 8) // Default 8 GB
	iops := asset.AttrFloat("iops", 0)
	throughput := asset.AttrFloat("throughput", 0)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	units := []clouds.CostUnit{
		// Storage
		clouds.NewCostUnit(
			"storage",
			"GB-months",
			size,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonEC2",
				Region:   region,
				Attributes: map[string]string{
					"volumeType": volumeType,
					"usageType":  "EBS:VolumeUsage." + volumeType,
				},
			},
			0.95,
		),
	}

	// Provisioned IOPS for io1/io2
	if (volumeType == "io1" || volumeType == "io2") && iops > 0 {
		units = append(units, clouds.NewCostUnit(
			"provisioned_iops",
			"IOPS-months",
			iops,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonEC2",
				Region:   region,
				Attributes: map[string]string{
					"volumeType": volumeType,
					"usageType":  "EBS:VolumeP-IOPS." + volumeType,
				},
			},
			0.95,
		))
	}

	// Provisioned IOPS for gp3 (above 3000 baseline)
	if volumeType == "gp3" && iops > 3000 {
		extraIOPS := iops - 3000
		units = append(units, clouds.NewCostUnit(
			"provisioned_iops",
			"IOPS-months",
			extraIOPS,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonEC2",
				Region:   region,
				Attributes: map[string]string{
					"volumeType": "gp3",
					"usageType":  "EBS:VolumeP-IOPS.gp3",
				},
			},
			0.95,
		))
	}

	// Provisioned throughput for gp3 (above 125 MiB/s baseline)
	if volumeType == "gp3" && throughput > 125 {
		extraThroughput := throughput - 125
		units = append(units, clouds.NewCostUnit(
			"provisioned_throughput",
			"MiBps-months",
			extraThroughput,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonEC2",
				Region:   region,
				Attributes: map[string]string{
					"volumeType": "gp3",
					"usageType":  "EBS:VolumeP-Throughput.gp3",
				},
			},
			0.95,
		))
	}

	return units, nil
}
