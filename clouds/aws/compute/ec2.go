// Package compute - AWS EC2 cost mapper
// Clean-room implementation based on EC2 pricing model:
// - Instance hours (by instance type, region, OS, tenancy)
// - EBS storage for root/additional volumes
// - Data transfer
// - EBS-optimized surcharge (some instance types)
package compute

import (
	"terraform-cost/clouds"
)

// EC2Mapper maps aws_instance to cost units
type EC2Mapper struct{}

// NewEC2Mapper creates an EC2 mapper
func NewEC2Mapper() *EC2Mapper {
	return &EC2Mapper{}
}

// Cloud returns the cloud provider
func (m *EC2Mapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *EC2Mapper) ResourceType() string {
	return "aws_instance"
}

// BuildUsage extracts usage vectors from an EC2 instance
func (m *EC2Mapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	// Check for unknown cardinality
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown instance count: "+asset.Cardinality.Reason),
		}, nil
	}

	// Default: 730 hours/month (24*365/12)
	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, ctx.Confidence),
	}, nil
}

// BuildCostUnits creates cost units for an EC2 instance
func (m *EC2Mapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	// Check for symbolic usage
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("compute", "EC2 cost unknown due to cardinality"),
		}, nil
	}

	// Extract attributes
	instanceType := asset.Attr("instance_type")
	if instanceType == "" {
		instanceType = "t3.micro"
	}

	// Determine OS (affects pricing)
	ami := asset.Attr("ami")
	os := inferOS(ami, asset.Attributes)

	// Tenancy
	tenancy := asset.Attr("tenancy")
	if tenancy == "" {
		tenancy = "default"
	}

	// Get monthly hours
	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	// Build rate key
	rateKey := clouds.RateKey{
		Provider: asset.ProviderContext.ProviderID,
		Service:  "AmazonEC2",
		Region:   asset.ProviderContext.Region,
		Attributes: map[string]string{
			"instanceType": instanceType,
			"operatingSystem": os,
			"tenancy":      tenancy,
			"capacityStatus": "Used",
		},
	}

	units := []clouds.CostUnit{
		clouds.NewCostUnit("compute", "hours", monthlyHours, rateKey, 0.95),
	}

	// Root block device storage
	if rootVolumeSize := asset.AttrFloat("root_block_device.0.volume_size", 0); rootVolumeSize > 0 {
		volumeType := asset.Attr("root_block_device.0.volume_type")
		if volumeType == "" {
			volumeType = "gp3"
		}

		units = append(units, clouds.NewCostUnit(
			"root_storage",
			"GB-months",
			rootVolumeSize,
			clouds.RateKey{
				Provider: asset.ProviderContext.ProviderID,
				Service:  "AmazonEC2",
				Region:   asset.ProviderContext.Region,
				Attributes: map[string]string{
					"volumeType": volumeType,
					"usageType":  "EBS:VolumeUsage." + volumeType,
				},
			},
			0.95,
		))
	}

	// EBS optimized (if applicable and not included)
	if asset.AttrBool("ebs_optimized", false) {
		// Some newer instance types include EBS optimization for free
		if !isEBSOptimizedFree(instanceType) {
			units = append(units, clouds.NewCostUnit(
				"ebs_optimized",
				"hours",
				monthlyHours,
				clouds.RateKey{
					Provider: asset.ProviderContext.ProviderID,
					Service:  "AmazonEC2",
					Region:   asset.ProviderContext.Region,
					Attributes: map[string]string{
						"instanceType": instanceType,
						"usageType":    "EBSOptimized:" + instanceType,
					},
				},
				0.90,
			))
		}
	}

	return units, nil
}

// inferOS infers the operating system from AMI or attributes
func inferOS(ami string, attrs map[string]interface{}) string {
	// Check for Windows in AMI name or tags
	if amiName, ok := attrs["ami_name"].(string); ok {
		if containsWindows(amiName) {
			return "Windows"
		}
	}

	// Default to Linux
	return "Linux"
}

func containsWindows(s string) bool {
	// Simple check - would be more sophisticated in production
	for i := 0; i < len(s)-6; i++ {
		if s[i:i+7] == "windows" || s[i:i+7] == "Windows" {
			return true
		}
	}
	return false
}

// isEBSOptimizedFree checks if EBS optimization is free for instance type
func isEBSOptimizedFree(instanceType string) bool {
	// Most newer instance types include EBS optimization
	// This is a simplified check
	freeTypes := map[string]bool{
		"t3": true, "t3a": true, "m5": true, "m5a": true, "m5d": true,
		"c5": true, "c5d": true, "c5n": true, "r5": true, "r5a": true,
	}

	// Extract family from instance type (e.g., "m5" from "m5.large")
	family := ""
	for i, c := range instanceType {
		if c == '.' {
			family = instanceType[:i]
			break
		}
	}

	return freeTypes[family]
}
