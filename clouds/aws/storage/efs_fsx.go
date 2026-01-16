// Package storage - AWS EFS/FSx cost mapper
// EFS Pricing:
// - Storage: per GB-month (Standard, IA, One Zone)
// - Throughput: provisioned or bursting
// - Data transfer: infrequent access reads
// FSx Pricing:
// - Capacity: per GB-month
// - Throughput: per MBps-month (FSx for Lustre, Windows)
// - Backups: per GB-month
package storage

import (
	"terraform-cost/clouds"
)

// EFSMapper maps aws_efs_file_system to cost units
type EFSMapper struct{}

// NewEFSMapper creates an EFS mapper
func NewEFSMapper() *EFSMapper {
	return &EFSMapper{}
}

// Cloud returns the cloud provider
func (m *EFSMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *EFSMapper) ResourceType() string {
	return "aws_efs_file_system"
}

// BuildUsage extracts usage vectors
func (m *EFSMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricStorageGB, "unknown EFS count: "+asset.Cardinality.Reason),
		}, nil
	}

	// EFS storage is usage-dependent
	storageGB := ctx.ResolveOrDefault("storage_gb", -1)

	if storageGB < 0 {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricStorageGB, "EFS storage size unknown (grows dynamically)"),
		}, nil
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricStorageGB, storageGB, 0.5),
	}, nil
}

// BuildCostUnits creates cost units
func (m *EFSMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("efs_storage", "EFS cost depends on actual storage used"),
		}, nil
	}

	storageGB, _ := usageVecs.Get(clouds.MetricStorageGB)

	// Determine storage class
	performanceMode := asset.Attr("performance_mode")
	if performanceMode == "" {
		performanceMode = "generalPurpose"
	}

	// One Zone vs Standard
	availabilityZoneName := asset.Attr("availability_zone_name")
	storageClass := "Standard"
	if availabilityZoneName != "" {
		storageClass = "One Zone"
	}

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	units := []clouds.CostUnit{
		clouds.NewCostUnit(
			"storage",
			"GB-months",
			storageGB,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonEFS",
				Region:   region,
				Attributes: map[string]string{
					"storageClass":    storageClass,
					"performanceMode": performanceMode,
					"usageType":       "TimedStorage-ByteHrs",
				},
			},
			0.5,
		),
	}

	// Provisioned throughput (if specified)
	provisionedThroughput := asset.AttrFloat("provisioned_throughput_in_mibps", 0)
	if provisionedThroughput > 0 {
		units = append(units, clouds.NewCostUnit(
			"provisioned_throughput",
			"MiBps-months",
			provisionedThroughput,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonEFS",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "ProvisionedTP-MiBpsHrs",
				},
			},
			0.95,
		))
	}

	return units, nil
}

// FSxMapper maps aws_fsx_lustre_file_system to cost units
type FSxLustreMapper struct{}

// NewFSxLustreMapper creates an FSx for Lustre mapper
func NewFSxLustreMapper() *FSxLustreMapper {
	return &FSxLustreMapper{}
}

// Cloud returns the cloud provider
func (m *FSxLustreMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *FSxLustreMapper) ResourceType() string {
	return "aws_fsx_lustre_file_system"
}

// BuildUsage extracts usage vectors
func (m *FSxLustreMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricStorageGB, "unknown FSx count: "+asset.Cardinality.Reason),
		}, nil
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricStorageGB, 1, 1.0), // Placeholder
	}, nil
}

// BuildCostUnits creates cost units
func (m *FSxLustreMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("fsx_lustre", "FSx cost unknown"),
		}, nil
	}

	storageCapacity := asset.AttrFloat("storage_capacity", 1200) // Minimum 1.2 TB
	deploymentType := asset.Attr("deployment_type")
	if deploymentType == "" {
		deploymentType = "SCRATCH_1"
	}

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"storage",
			"GB-months",
			storageCapacity,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonFSx",
				Region:   region,
				Attributes: map[string]string{
					"fileSystemType": "Lustre",
					"deploymentType": deploymentType,
					"usageType":      "Storage-SDD",
				},
			},
			0.95,
		),
	}, nil
}

// FSxWindowsMapper maps aws_fsx_windows_file_system to cost units
type FSxWindowsMapper struct{}

// NewFSxWindowsMapper creates an FSx for Windows mapper
func NewFSxWindowsMapper() *FSxWindowsMapper {
	return &FSxWindowsMapper{}
}

// Cloud returns the cloud provider
func (m *FSxWindowsMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *FSxWindowsMapper) ResourceType() string {
	return "aws_fsx_windows_file_system"
}

// BuildUsage extracts usage vectors
func (m *FSxWindowsMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricStorageGB, "unknown FSx count: "+asset.Cardinality.Reason),
		}, nil
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricStorageGB, 1, 1.0),
	}, nil
}

// BuildCostUnits creates cost units
func (m *FSxWindowsMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("fsx_windows", "FSx cost unknown"),
		}, nil
	}

	storageCapacity := asset.AttrFloat("storage_capacity", 32) // Minimum 32 GB
	storageType := asset.Attr("storage_type")
	if storageType == "" {
		storageType = "SSD"
	}
	throughputCapacity := asset.AttrFloat("throughput_capacity", 8) // MBps
	deploymentType := asset.Attr("deployment_type")
	if deploymentType == "" {
		deploymentType = "SINGLE_AZ_1"
	}

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"storage",
			"GB-months",
			storageCapacity,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonFSx",
				Region:   region,
				Attributes: map[string]string{
					"fileSystemType": "Windows",
					"storageType":    storageType,
					"deploymentType": deploymentType,
				},
			},
			0.95,
		),
		clouds.NewCostUnit(
			"throughput",
			"MBps-months",
			throughputCapacity,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonFSx",
				Region:   region,
				Attributes: map[string]string{
					"fileSystemType": "Windows",
					"usageType":      "Throughput-MBps",
				},
			},
			0.95,
		),
	}, nil
}
