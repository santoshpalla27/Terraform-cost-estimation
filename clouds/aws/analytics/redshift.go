// Package analytics - AWS Redshift cost mapper
// Pricing model:
// - Node hours (by node type)
// - Managed storage (RA3 nodes only)
// - Spectrum queries (per TB scanned)
// - Concurrency scaling (per second)
// - Snapshots (beyond free storage)
package analytics

import (
	"terraform-cost/clouds"
)

// RedshiftMapper maps aws_redshift_cluster to cost units
type RedshiftMapper struct{}

// NewRedshiftMapper creates a Redshift mapper
func NewRedshiftMapper() *RedshiftMapper {
	return &RedshiftMapper{}
}

// Cloud returns the cloud provider
func (m *RedshiftMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *RedshiftMapper) ResourceType() string {
	return "aws_redshift_cluster"
}

// BuildUsage extracts usage vectors
func (m *RedshiftMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown Redshift cluster count: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
	}, nil
}

// BuildCostUnits creates cost units
func (m *RedshiftMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("redshift", "Redshift cost unknown due to cardinality"),
		}, nil
	}

	nodeType := asset.Attr("node_type")
	if nodeType == "" {
		nodeType = "dc2.large"
	}

	numberOfNodes := asset.AttrInt("number_of_nodes", 1)
	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	units := []clouds.CostUnit{
		// Node hours
		clouds.NewCostUnit(
			"nodes",
			"node-hours",
			float64(numberOfNodes)*monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonRedshift",
				Region:   region,
				Attributes: map[string]string{
					"nodeType":  nodeType,
					"usageType": "Node:" + nodeType,
				},
			},
			0.95,
		),
	}

	// RA3 nodes have separate managed storage cost
	if isRA3Node(nodeType) {
		// RA3 storage is usage-based
		units = append(units, clouds.SymbolicCost(
			"managed_storage",
			"RA3 managed storage depends on data volume",
		))
	}

	// Concurrency scaling (if enabled)
	// This is highly usage-dependent
	units = append(units, clouds.SymbolicCost(
		"concurrency_scaling",
		"concurrency scaling cost depends on query load",
	))

	return units, nil
}

func isRA3Node(nodeType string) bool {
	return len(nodeType) >= 3 && nodeType[:3] == "ra3"
}
