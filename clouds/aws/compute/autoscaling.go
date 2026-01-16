// Package compute - AWS Auto Scaling cost mapper
// Clean-room implementation based on ASG pricing model:
// - No direct ASG cost (it's free)
// - Costs come from launched instances (tracked separately)
// - This mapper handles capacity estimation for cost projection
package compute

import (
	"terraform-cost/clouds"
)

// AutoscalingMapper maps aws_autoscaling_group to cost units
type AutoscalingMapper struct{}

// NewAutoscalingMapper creates an ASG mapper
func NewAutoscalingMapper() *AutoscalingMapper {
	return &AutoscalingMapper{}
}

// Cloud returns the cloud provider
func (m *AutoscalingMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *AutoscalingMapper) ResourceType() string {
	return "aws_autoscaling_group"
}

// BuildUsage extracts usage vectors from an ASG
func (m *AutoscalingMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	// Check for unknown cardinality
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown ASG capacity: "+asset.Cardinality.Reason),
		}, nil
	}

	// Get capacity configuration
	minSize := asset.AttrInt("min_size", 0)
	maxSize := asset.AttrInt("max_size", 0)
	desiredCapacity := asset.AttrInt("desired_capacity", minSize)

	// If desired capacity depends on dynamic scaling, it's unknown
	if desiredCapacity == 0 && minSize == 0 {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "dynamic scaling with min_size=0"),
		}, nil
	}

	// Use desired capacity or min_size for estimation
	instanceCount := float64(desiredCapacity)
	if instanceCount == 0 {
		instanceCount = float64(minSize)
	}

	// Calculate instance hours (instances * hours/month)
	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)
	totalInstanceHours := instanceCount * monthlyHours

	// Confidence is lower for ASG because actual capacity varies
	confidence := 0.7
	if minSize == maxSize {
		// Fixed capacity = higher confidence
		confidence = 0.9
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, totalInstanceHours, confidence),
	}, nil
}

// BuildCostUnits creates cost units for an ASG
func (m *AutoscalingMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	// Check for symbolic usage
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("instances", "ASG cost unknown: capacity is dynamic"),
		}, nil
	}

	// ASG itself is free - costs come from instances
	// We estimate the aggregate instance cost here

	// Get launch template or launch configuration
	instanceType := m.getInstanceType(asset)

	// Get total instance hours
	totalHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	// Infer OS from launch template/config
	os := "Linux" // Default

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"instances",
			"instance-hours",
			totalHours,
			clouds.RateKey{
				Provider: asset.ProviderContext.ProviderID,
				Service:  "AmazonEC2",
				Region:   asset.ProviderContext.Region,
				Attributes: map[string]string{
					"instanceType":    instanceType,
					"operatingSystem": os,
					"tenancy":         "default",
					"capacityStatus":  "Used",
				},
			},
			0.7, // Lower confidence for ASG
		),
	}, nil
}

// getInstanceType extracts instance type from launch template or config
func (m *AutoscalingMapper) getInstanceType(asset clouds.AssetNode) string {
	// Check launch template
	if lt := asset.Attr("launch_template.0.instance_type"); lt != "" {
		return lt
	}

	// Check launch configuration (legacy)
	if lc := asset.Attr("launch_configuration"); lc != "" {
		// Would need to resolve launch configuration
		// For now, use a default
		return "t3.medium"
	}

	// Check mixed instances policy
	if override := asset.Attr("mixed_instances_policy.0.launch_template.0.override.0.instance_type"); override != "" {
		return override
	}

	return "t3.medium" // Default fallback
}
