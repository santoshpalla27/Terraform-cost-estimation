// Package backup - AWS Backup Vault mapper
package backup

import (
	"terraform-cost/clouds"
)

// BackupVaultMapper maps aws_backup_vault to cost units
type BackupVaultMapper struct{}

func NewBackupVaultMapper() *BackupVaultMapper { return &BackupVaultMapper{} }

func (m *BackupVaultMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *BackupVaultMapper) ResourceType() string        { return "aws_backup_vault" }

func (m *BackupVaultMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage("vaults", "unknown vault count")}, nil
	}

	storageGB := ctx.ResolveOrDefault("storage_gb", -1)
	if storageGB < 0 {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricStorageGB, "backup storage size depends on backup schedule and retention"),
		}, nil
	}

	return []clouds.UsageVector{clouds.NewUsageVector(clouds.MetricStorageGB, storageGB, 0.5)}, nil
}

func (m *BackupVaultMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("backup", "Backup vault cost depends on stored backup data"),
		}, nil
	}

	storageGB, _ := usageVecs.Get(clouds.MetricStorageGB)

	return []clouds.CostUnit{
		clouds.NewCostUnit("storage", "GB-months", storageGB, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AWSBackup",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "StorageUsage",
			},
		}, 0.5),
	}, nil
}
