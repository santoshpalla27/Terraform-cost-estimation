// Package compute - Azure VM cost mapper (placeholder)
// Clean-room implementation based on Azure VM pricing model:
// - Compute hours (by VM size, region, OS)
// - Managed disks (separate resource)
// - Spot VMs
// - Reserved instances
package compute

import (
	"terraform-cost/clouds"
)

// VMMapper maps azurerm_linux_virtual_machine to cost units
type VMMapper struct{}

// NewVMMapper creates a VM mapper
func NewVMMapper() *VMMapper {
	return &VMMapper{}
}

// Cloud returns the cloud provider
func (m *VMMapper) Cloud() clouds.CloudProvider {
	return clouds.Azure
}

// ResourceType returns the Terraform resource type
func (m *VMMapper) ResourceType() string {
	return "azurerm_linux_virtual_machine"
}

// BuildUsage extracts usage vectors from an Azure VM
func (m *VMMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown VM count: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
	}, nil
}

// BuildCostUnits creates cost units for an Azure VM
func (m *VMMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("compute", "Azure VM cost unknown due to cardinality"),
		}, nil
	}

	vmSize := asset.Attr("size")
	if vmSize == "" {
		vmSize = "Standard_B1s"
	}

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"compute",
			"hours",
			monthlyHours,
			clouds.RateKey{
				Provider: asset.ProviderContext.ProviderID,
				Service:  "Virtual Machines",
				Region:   asset.ProviderContext.Region,
				Attributes: map[string]string{
					"vmSize": vmSize,
					"os":     "Linux",
				},
			},
			0.95,
		),
	}, nil
}
