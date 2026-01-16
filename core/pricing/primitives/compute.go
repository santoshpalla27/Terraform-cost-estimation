// Package primitives - Compute pricing primitives
// Instance hours, vCPU hours, memory hours
package primitives

// InstanceHours creates a cost unit for compute instance hours
func InstanceHours(
	instanceType string,
	hours float64,
	provider CloudProvider,
	region string,
	os string,
	tenancy string,
	confidence float64,
) CostUnit {
	if instanceType == "" {
		return Symbolic("compute", "instance type not specified")
	}

	if hours <= 0 {
		return Symbolic("compute", "hours must be positive")
	}

	attrs := map[string]string{
		"instanceType": instanceType,
	}
	if os != "" {
		attrs["operatingSystem"] = os
	}
	if tenancy != "" {
		attrs["tenancy"] = tenancy
	}

	service := computeService(provider)

	return CostUnit{
		Name:       "compute",
		Measure:    "instance-hours",
		Quantity:   hours,
		Confidence: confidence,
		RateKey: RateKey{
			Provider:   string(provider),
			Service:    service,
			Region:     region,
			Attributes: attrs,
		},
	}
}

// NodeHours creates a cost unit for managed service node hours
// Used for: ElastiCache, OpenSearch, Redshift, etc.
func NodeHours(
	nodeType string,
	nodeCount int,
	hours float64,
	provider CloudProvider,
	service string,
	region string,
	confidence float64,
) CostUnit {
	if nodeType == "" {
		return Symbolic("nodes", "node type not specified")
	}

	return CostUnit{
		Name:       "nodes",
		Measure:    "node-hours",
		Quantity:   float64(nodeCount) * hours,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"nodeType": nodeType,
			},
		},
	}
}

// VCPUHours creates a cost unit for vCPU-based pricing
// Used for: Fargate, Lambda duration, etc.
func VCPUHours(
	vcpus float64,
	hours float64,
	provider CloudProvider,
	service string,
	region string,
	confidence float64,
) CostUnit {
	return CostUnit{
		Name:       "vcpu",
		Measure:    "vCPU-hours",
		Quantity:   vcpus * hours,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"usageType": "vCPU-Hours",
			},
		},
	}
}

// MemoryGBHours creates a cost unit for memory-based pricing
func MemoryGBHours(
	memoryGB float64,
	hours float64,
	provider CloudProvider,
	service string,
	region string,
	confidence float64,
) CostUnit {
	return CostUnit{
		Name:       "memory",
		Measure:    "GB-hours",
		Quantity:   memoryGB * hours,
		Confidence: confidence,
		RateKey: RateKey{
			Provider: string(provider),
			Service:  service,
			Region:   region,
			Attributes: map[string]string{
				"usageType": "Memory-Hours",
			},
		},
	}
}

func computeService(provider CloudProvider) string {
	switch provider {
	case AWS:
		return "AmazonEC2"
	case Azure:
		return "Virtual Machines"
	case GCP:
		return "Compute Engine"
	default:
		return "Compute"
	}
}
