// Package containers - AWS EKS cost mapper
// Pricing model:
// - Control plane: $0.10/hour per cluster
// - Nodes: EC2 instance costs (tracked separately)
// - Fargate: vCPU-hour + GB-hour
package containers

import (
	"terraform-cost/clouds"
)

// EKSClusterMapper maps aws_eks_cluster to cost units
type EKSClusterMapper struct{}

// NewEKSClusterMapper creates an EKS Cluster mapper
func NewEKSClusterMapper() *EKSClusterMapper {
	return &EKSClusterMapper{}
}

// Cloud returns the cloud provider
func (m *EKSClusterMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *EKSClusterMapper) ResourceType() string {
	return "aws_eks_cluster"
}

// BuildUsage extracts usage vectors
func (m *EKSClusterMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown EKS cluster count: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
	}, nil
}

// BuildCostUnits creates cost units for EKS control plane
func (m *EKSClusterMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("control_plane", "EKS cost unknown due to cardinality"),
		}, nil
	}

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	// EKS control plane is fixed at $0.10/hour
	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"control_plane",
			"hours",
			monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonEKS",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "AmazonEKS-Hours:perkubernetes",
				},
			},
			0.95,
		),
	}, nil
}

// EKSNodeGroupMapper maps aws_eks_node_group to cost units
type EKSNodeGroupMapper struct{}

// NewEKSNodeGroupMapper creates an EKS Node Group mapper
func NewEKSNodeGroupMapper() *EKSNodeGroupMapper {
	return &EKSNodeGroupMapper{}
}

// Cloud returns the cloud provider
func (m *EKSNodeGroupMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *EKSNodeGroupMapper) ResourceType() string {
	return "aws_eks_node_group"
}

// BuildUsage extracts usage vectors
func (m *EKSNodeGroupMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown node group: "+asset.Cardinality.Reason),
		}, nil
	}

	// Get scaling config
	desiredSize := asset.AttrInt("scaling_config.0.desired_size", 2)
	minSize := asset.AttrInt("scaling_config.0.min_size", 1)

	// Use desired or min for estimation
	nodeCount := float64(desiredSize)
	if nodeCount == 0 {
		nodeCount = float64(minSize)
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)

	// Confidence depends on whether it's fixed or auto-scaling
	confidence := 0.7
	maxSize := asset.AttrInt("scaling_config.0.max_size", desiredSize)
	if minSize == maxSize {
		confidence = 0.95
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, nodeCount*monthlyHours, confidence),
	}, nil
}

// BuildCostUnits creates cost units for EKS nodes
func (m *EKSNodeGroupMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("nodes", "EKS node cost unknown"),
		}, nil
	}

	// Get instance types
	instanceTypes := asset.Attr("instance_types.0")
	if instanceTypes == "" {
		instanceTypes = "t3.medium"
	}

	totalHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"nodes",
			"instance-hours",
			totalHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonEC2",
				Region:   region,
				Attributes: map[string]string{
					"instanceType":    instanceTypes,
					"operatingSystem": "Linux",
					"tenancy":         "default",
				},
			},
			0.7,
		),
	}, nil
}
