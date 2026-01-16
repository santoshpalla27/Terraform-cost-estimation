// Package primitives - Storage pricing primitives
// GB-months, IOPS-months, throughput
package primitives

// StorageType for volume/storage classification
type StorageType string

const (
	StorageSSD      StorageType = "ssd"
	StorageHDD      StorageType = "hdd"
	StorageStandard StorageType = "standard"
	StorageArchive  StorageType = "archive"
)

// StorageGB creates a cost unit for storage by GB-month
func StorageGB(
	gb float64,
	storageType StorageType,
	provider CloudProvider,
	service string,
	region string,
	confidence float64,
) CostUnit {
	if gb <= 0 {
		return Symbolic("storage", "storage size must be positive")
	}

	return CostUnit{
		Name:       "storage",
		Measure:    "GB-months",
		Quantity:   gb,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"storageType": string(storageType),
			},
		},
	}
}

// VolumeStorage creates a cost unit for block storage (EBS, Persistent Disk)
func VolumeStorage(
	gb float64,
	volumeType string,
	provider CloudProvider,
	region string,
	confidence float64,
) CostUnit {
	if gb <= 0 {
		return Symbolic("storage", "volume size must be positive")
	}

	service := volumeService(provider)

	return CostUnit{
		Name:       "storage",
		Measure:    "GB-months",
		Quantity:   gb,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"volumeType": volumeType,
			},
		},
	}
}

// ProvisionedIOPS creates a cost unit for provisioned IOPS
func ProvisionedIOPS(
	iops float64,
	volumeType string,
	provider CloudProvider,
	region string,
	confidence float64,
) CostUnit {
	if iops <= 0 {
		return CostUnit{} // No cost for zero IOPS
	}

	service := volumeService(provider)

	return CostUnit{
		Name:       "iops",
		Measure:    "IOPS-months",
		Quantity:   iops,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"volumeType": volumeType,
				"usageType":  "ProvisionedIOPS",
			},
		},
	}
}

// ProvisionedThroughput creates a cost unit for provisioned throughput
func ProvisionedThroughput(
	mbps float64,
	volumeType string,
	provider CloudProvider,
	region string,
	confidence float64,
) CostUnit {
	if mbps <= 0 {
		return CostUnit{} // No cost for zero throughput
	}

	service := volumeService(provider)

	return CostUnit{
		Name:       "throughput",
		Measure:    "MBps-months",
		Quantity:   mbps,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"volumeType": volumeType,
				"usageType":  "ProvisionedThroughput",
			},
		},
	}
}

func volumeService(provider CloudProvider) string {
	switch provider {
	case AWS:
		return "AmazonEC2" // EBS is under EC2
	case Azure:
		return "Managed Disks"
	case GCP:
		return "Compute Engine" // Persistent Disk is under GCE
	default:
		return "Storage"
	}
}
