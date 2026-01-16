// Package observability - AWS CloudWatch cost mapper
// Pricing model:
// - Log ingestion per GB
// - Log storage per GB-month
// - Metrics (custom metrics, high-res)
// - Alarms per metric
// - Dashboards
package observability

import (
	"terraform-cost/clouds"
)

// CloudWatchLogGroupMapper maps aws_cloudwatch_log_group to cost units
type CloudWatchLogGroupMapper struct{}

// NewCloudWatchLogGroupMapper creates a CloudWatch Log Group mapper
func NewCloudWatchLogGroupMapper() *CloudWatchLogGroupMapper {
	return &CloudWatchLogGroupMapper{}
}

// Cloud returns the cloud provider
func (m *CloudWatchLogGroupMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *CloudWatchLogGroupMapper) ResourceType() string {
	return "aws_cloudwatch_log_group"
}

// BuildUsage extracts usage vectors
func (m *CloudWatchLogGroupMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	// Log groups are HIGHLY usage-dependent
	// Without explicit usage data, we return symbolic
	ingestionGB := ctx.ResolveOrDefault("monthly_ingestion_gb", -1)
	storageGB := ctx.ResolveOrDefault("storage_gb", -1)

	if ingestionGB < 0 || storageGB < 0 {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricStorageGB, "log ingestion/storage usage not provided"),
		}, nil
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector("ingestion_gb", ingestionGB, 0.5),
		clouds.NewUsageVector(clouds.MetricStorageGB, storageGB, 0.5),
	}, nil
}

// BuildCostUnits creates cost units
func (m *CloudWatchLogGroupMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("logs", "CloudWatch Logs cost requires usage data"),
		}, nil
	}

	ingestionGB, _ := usageVecs.Get("ingestion_gb")
	storageGB, _ := usageVecs.Get(clouds.MetricStorageGB)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	return []clouds.CostUnit{
		// Log ingestion
		clouds.NewCostUnit(
			"ingestion",
			"GB",
			ingestionGB,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonCloudWatch",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "DataProcessing-Bytes",
				},
			},
			0.5,
		),
		// Log storage
		clouds.NewCostUnit(
			"storage",
			"GB-months",
			storageGB,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonCloudWatch",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "TimedStorage-ByteHrs",
				},
			},
			0.5,
		),
	}, nil
}

// CloudWatchMetricAlarmMapper maps aws_cloudwatch_metric_alarm to cost units
type CloudWatchMetricAlarmMapper struct{}

// NewCloudWatchMetricAlarmMapper creates a CloudWatch Alarm mapper
func NewCloudWatchMetricAlarmMapper() *CloudWatchMetricAlarmMapper {
	return &CloudWatchMetricAlarmMapper{}
}

// Cloud returns the cloud provider
func (m *CloudWatchMetricAlarmMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *CloudWatchMetricAlarmMapper) ResourceType() string {
	return "aws_cloudwatch_metric_alarm"
}

// BuildUsage extracts usage vectors
func (m *CloudWatchMetricAlarmMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage("alarm_count", "unknown alarm count: "+asset.Cardinality.Reason),
		}, nil
	}

	// One alarm per resource
	return []clouds.UsageVector{
		clouds.NewUsageVector("alarm_count", 1, 1.0),
	}, nil
}

// BuildCostUnits creates cost units
func (m *CloudWatchMetricAlarmMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("alarm", "CloudWatch Alarm cost unknown"),
		}, nil
	}

	// Determine alarm type (standard vs high-res)
	period := asset.AttrInt("period", 60)
	alarmType := "standard"
	if period < 60 {
		alarmType = "high_resolution"
	}

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"alarm",
			"alarms",
			1,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonCloudWatch",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "AlarmMonitorUsage",
					"alarmType": alarmType,
				},
			},
			0.95,
		),
	}, nil
}
