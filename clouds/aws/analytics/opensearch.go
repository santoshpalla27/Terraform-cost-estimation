// Package analytics - AWS OpenSearch cost mapper
// Pricing model:
// - Instance hours (by instance type)
// - Storage (EBS or instance-based)
// - UltraWarm storage (per GB-month)
// - Cold storage (per GB-month)
// - Serverless (OCU-hours for compute + indexing)
package analytics

import (
	"terraform-cost/clouds"
)

// OpenSearchMapper maps aws_opensearch_domain to cost units
type OpenSearchMapper struct{}

// NewOpenSearchMapper creates an OpenSearch mapper
func NewOpenSearchMapper() *OpenSearchMapper {
	return &OpenSearchMapper{}
}

// Cloud returns the cloud provider
func (m *OpenSearchMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *OpenSearchMapper) ResourceType() string {
	return "aws_opensearch_domain"
}

// BuildUsage extracts usage vectors
func (m *OpenSearchMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown OpenSearch domain count: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
	}, nil
}

// BuildCostUnits creates cost units
func (m *OpenSearchMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("opensearch", "OpenSearch cost unknown due to cardinality"),
		}, nil
	}

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	// Get instance configuration
	instanceType := asset.Attr("cluster_config.0.instance_type")
	if instanceType == "" {
		instanceType = "t3.small.search"
	}
	instanceCount := asset.AttrInt("cluster_config.0.instance_count", 1)

	// Dedicated master nodes
	masterType := asset.Attr("cluster_config.0.dedicated_master_type")
	masterCount := asset.AttrInt("cluster_config.0.dedicated_master_count", 0)
	masterEnabled := asset.AttrBool("cluster_config.0.dedicated_master_enabled", false)

	// EBS storage
	ebsEnabled := asset.AttrBool("ebs_options.0.ebs_enabled", true)
	volumeSize := asset.AttrFloat("ebs_options.0.volume_size", 10)
	volumeType := asset.Attr("ebs_options.0.volume_type")
	if volumeType == "" {
		volumeType = "gp3"
	}

	var units []clouds.CostUnit

	// Data nodes
	units = append(units, clouds.NewCostUnit(
		"data_nodes",
		"instance-hours",
		float64(instanceCount)*monthlyHours,
		clouds.RateKey{
			Provider: providerID,
			Service:  "AmazonES",
			Region:   region,
			Attributes: map[string]string{
				"instanceType": instanceType,
				"usageType":    "ESInstance:" + instanceType,
			},
		},
		0.95,
	))

	// Master nodes (if enabled)
	if masterEnabled && masterCount > 0 && masterType != "" {
		units = append(units, clouds.NewCostUnit(
			"master_nodes",
			"instance-hours",
			float64(masterCount)*monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonES",
				Region:   region,
				Attributes: map[string]string{
					"instanceType": masterType,
					"usageType":    "ESInstance:" + masterType,
				},
			},
			0.95,
		))
	}

	// EBS storage
	if ebsEnabled && volumeSize > 0 {
		totalStorage := volumeSize * float64(instanceCount)
		units = append(units, clouds.NewCostUnit(
			"ebs_storage",
			"GB-months",
			totalStorage,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonES",
				Region:   region,
				Attributes: map[string]string{
					"volumeType": volumeType,
					"usageType":  "ES:VolumeUsage." + volumeType,
				},
			},
			0.95,
		))
	}

	// UltraWarm nodes (if configured)
	warmType := asset.Attr("cluster_config.0.warm_type")
	warmCount := asset.AttrInt("cluster_config.0.warm_count", 0)
	if warmCount > 0 && warmType != "" {
		units = append(units, clouds.NewCostUnit(
			"ultrawarm_nodes",
			"instance-hours",
			float64(warmCount)*monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonES",
				Region:   region,
				Attributes: map[string]string{
					"instanceType": warmType,
					"usageType":    "ESInstance:" + warmType,
				},
			},
			0.95,
		))
	}

	return units, nil
}
