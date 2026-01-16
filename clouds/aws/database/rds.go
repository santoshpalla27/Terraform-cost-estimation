// Package database - AWS RDS cost mapper
// Clean-room implementation based on RDS pricing model:
// - Instance hours (by instance class, engine, deployment type)
// - Storage (by storage type and size)
// - Provisioned IOPS (for io1)
// - Backup storage (beyond allocated)
// - Data transfer
package database

import (
	"terraform-cost/clouds"
)

// RDSMapper maps aws_db_instance to cost units
type RDSMapper struct{}

// NewRDSMapper creates an RDS mapper
func NewRDSMapper() *RDSMapper {
	return &RDSMapper{}
}

// Cloud returns the cloud provider
func (m *RDSMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *RDSMapper) ResourceType() string {
	return "aws_db_instance"
}

// BuildUsage extracts usage vectors from an RDS instance
func (m *RDSMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	// Check for unknown cardinality
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown RDS instance count: "+asset.Cardinality.Reason),
		}, nil
	}

	// RDS instances run 24/7 by default
	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
	}, nil
}

// BuildCostUnits creates cost units for an RDS instance
func (m *RDSMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	// Check for symbolic usage
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("instance", "RDS cost unknown due to cardinality"),
		}, nil
	}

	// Extract attributes
	instanceClass := asset.Attr("instance_class")
	if instanceClass == "" {
		instanceClass = "db.t3.micro"
	}

	engine := asset.Attr("engine")
	if engine == "" {
		engine = "mysql"
	}

	storageType := asset.Attr("storage_type")
	if storageType == "" {
		storageType = "gp2"
	}

	allocatedStorage := asset.AttrFloat("allocated_storage", 20)
	iops := asset.AttrFloat("iops", 0)
	multiAZ := asset.AttrBool("multi_az", false)

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	// Normalize engine for pricing
	engineFamily := normalizeEngine(engine)

	// Deployment option affects pricing
	deploymentOption := "Single-AZ"
	if multiAZ {
		deploymentOption = "Multi-AZ"
	}

	units := []clouds.CostUnit{
		// Instance
		clouds.NewCostUnit(
			"instance",
			"hours",
			monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonRDS",
				Region:   region,
				Attributes: map[string]string{
					"instanceType":     instanceClass,
					"databaseEngine":   engineFamily,
					"deploymentOption": deploymentOption,
				},
			},
			0.95,
		),

		// Storage
		clouds.NewCostUnit(
			"storage",
			"GB-months",
			allocatedStorage,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonRDS",
				Region:   region,
				Attributes: map[string]string{
					"volumeType":       storageType,
					"deploymentOption": deploymentOption,
					"usageType":        "RDS:GP2-Storage",
				},
			},
			0.95,
		),
	}

	// Provisioned IOPS for io1
	if storageType == "io1" && iops > 0 {
		units = append(units, clouds.NewCostUnit(
			"provisioned_iops",
			"IOPS-months",
			iops,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonRDS",
				Region:   region,
				Attributes: map[string]string{
					"deploymentOption": deploymentOption,
					"usageType":        "RDS:PIOPS",
				},
			},
			0.95,
		))
	}

	// Backup storage (simplified: assume equal to allocated)
	backupRetention := asset.AttrInt("backup_retention_period", 1)
	if backupRetention > 0 {
		units = append(units, clouds.NewCostUnit(
			"backup_storage",
			"GB-months",
			allocatedStorage,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonRDS",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "RDS:ChargedBackupUsage",
				},
			},
			0.7, // Lower confidence - actual backup size varies
		))
	}

	return units, nil
}

// normalizeEngine normalizes engine name for pricing
func normalizeEngine(engine string) string {
	switch engine {
	case "mysql":
		return "MySQL"
	case "postgres":
		return "PostgreSQL"
	case "mariadb":
		return "MariaDB"
	case "oracle-se", "oracle-se1", "oracle-se2", "oracle-ee":
		return "Oracle"
	case "sqlserver-ee", "sqlserver-se", "sqlserver-ex", "sqlserver-web":
		return "SQL Server"
	case "aurora", "aurora-mysql":
		return "Aurora MySQL"
	case "aurora-postgresql":
		return "Aurora PostgreSQL"
	default:
		return engine
	}
}
