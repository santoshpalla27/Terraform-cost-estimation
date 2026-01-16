// Package storage - AWS EBS Snapshot mapper
package storage

import (
	"terraform-cost/clouds"
)

// EBSSnapshotMapper maps aws_ebs_snapshot to cost units
type EBSSnapshotMapper struct{}

func NewEBSSnapshotMapper() *EBSSnapshotMapper { return &EBSSnapshotMapper{} }

func (m *EBSSnapshotMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *EBSSnapshotMapper) ResourceType() string        { return "aws_ebs_snapshot" }

func (m *EBSSnapshotMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricStorageGB, "unknown snapshot count")}, nil
	}

	// Snapshot size is typically a fraction of volume size due to incremental
	storageGB := ctx.ResolveOrDefault("storage_gb", -1)
	if storageGB < 0 {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricStorageGB, "snapshot size unknown")}, nil
	}

	return []clouds.UsageVector{clouds.NewUsageVector(clouds.MetricStorageGB, storageGB, 0.5)}, nil
}

func (m *EBSSnapshotMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("snapshot", "snapshot size unknown - depends on changed data")}, nil
	}

	storageGB, _ := usageVecs.Get(clouds.MetricStorageGB)

	return []clouds.CostUnit{
		clouds.NewCostUnit("snapshot_storage", "GB-months", storageGB, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonEC2",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "EBS:SnapshotUsage",
			},
		}, 0.5),
	}, nil
}
