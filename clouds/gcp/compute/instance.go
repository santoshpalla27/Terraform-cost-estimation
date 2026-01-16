// Package compute - GCP Compute Instance cost mapper (placeholder)
// Clean-room implementation based on GCE pricing model:
// - Machine type hours (by machine type, region)
// - Preemptible/Spot VMs
// - Committed use discounts
// - Sustained use discounts
package compute

import (
	"terraform-cost/clouds"
)

// InstanceMapper maps google_compute_instance to cost units
type InstanceMapper struct{}

// NewInstanceMapper creates a GCE instance mapper
func NewInstanceMapper() *InstanceMapper {
	return &InstanceMapper{}
}

// Cloud returns the cloud provider
func (m *InstanceMapper) Cloud() clouds.CloudProvider {
	return clouds.GCP
}

// ResourceType returns the Terraform resource type
func (m *InstanceMapper) ResourceType() string {
	return "google_compute_instance"
}

// BuildUsage extracts usage vectors from a GCE instance
func (m *InstanceMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown instance count: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
	}, nil
}

// BuildCostUnits creates cost units for a GCE instance
func (m *InstanceMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("compute", "GCE cost unknown due to cardinality"),
		}, nil
	}

	machineType := asset.Attr("machine_type")
	if machineType == "" {
		machineType = "e2-micro"
	}

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	// GCP applies sustained use discounts automatically
	// For estimation, we use full hourly rate
	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"compute",
			"hours",
			monthlyHours,
			clouds.RateKey{
				Provider: asset.ProviderContext.ProviderID,
				Service:  "Compute Engine",
				Region:   asset.ProviderContext.Region,
				Attributes: map[string]string{
					"machineType": machineType,
				},
			},
			0.95,
		),
	}, nil
}
