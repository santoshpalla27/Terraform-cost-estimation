// Package containers - AWS ECR Repository mapper
package containers

import (
	"terraform-cost/clouds"
)

// ECRRepositoryMapper maps aws_ecr_repository to cost units
type ECRRepositoryMapper struct{}

func NewECRRepositoryMapper() *ECRRepositoryMapper { return &ECRRepositoryMapper{} }

func (m *ECRRepositoryMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *ECRRepositoryMapper) ResourceType() string        { return "aws_ecr_repository" }

func (m *ECRRepositoryMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage("repositories", "unknown repository count")}, nil
	}

	storageGB := ctx.ResolveOrDefault("storage_gb", -1)
	if storageGB < 0 {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricStorageGB, "ECR storage size unknown")}, nil
	}

	return []clouds.UsageVector{clouds.NewUsageVector(clouds.MetricStorageGB, storageGB, 0.5)}, nil
}

func (m *ECRRepositoryMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("ecr", "ECR cost depends on stored image size"),
		}, nil
	}

	storageGB, _ := usageVecs.Get(clouds.MetricStorageGB)

	return []clouds.CostUnit{
		clouds.NewCostUnit("storage", "GB-months", storageGB, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonECR",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "StorageUsage",
			},
		}, 0.5),
	}, nil
}
