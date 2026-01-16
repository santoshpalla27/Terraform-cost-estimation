// Package streaming - AWS Kinesis cost mapper
// Pricing model:
// - Shard hours (per shard per hour)
// - PUT payload units (per 25KB chunk)
// - Extended retention (beyond 24 hours)
// - Enhanced fan-out (per consumer shard hour + data retrieval)
package streaming

import (
	"terraform-cost/clouds"
)

// KinesisStreamMapper maps aws_kinesis_stream to cost units
type KinesisStreamMapper struct{}

// NewKinesisStreamMapper creates a Kinesis Stream mapper
func NewKinesisStreamMapper() *KinesisStreamMapper {
	return &KinesisStreamMapper{}
}

// Cloud returns the cloud provider
func (m *KinesisStreamMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *KinesisStreamMapper) ResourceType() string {
	return "aws_kinesis_stream"
}

// BuildUsage extracts usage vectors
func (m *KinesisStreamMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown Kinesis stream count: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyHours := ctx.ResolveOrDefault("monthly_hours", 730)
	// PUT units are usage-dependent
	putUnitsMillions := ctx.ResolveOrDefault("put_units_millions", -1)

	vectors := []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, monthlyHours, 0.95),
	}

	if putUnitsMillions >= 0 {
		vectors = append(vectors, clouds.NewUsageVector("put_units_millions", putUnitsMillions, 0.5))
	} else {
		vectors = append(vectors, clouds.SymbolicUsage("put_units_millions", "PUT units depend on ingestion volume"))
	}

	return vectors, nil
}

// BuildCostUnits creates cost units
func (m *KinesisStreamMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("kinesis", "Kinesis cost unknown"),
		}, nil
	}

	// Get stream mode
	streamMode := asset.Attr("stream_mode_details.0.stream_mode")
	if streamMode == "" {
		streamMode = "PROVISIONED"
	}

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region
	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	var units []clouds.CostUnit

	if streamMode == "ON_DEMAND" {
		// On-demand pricing
		units = append(units, clouds.SymbolicCost(
			"on_demand",
			"On-demand Kinesis cost depends on throughput",
		))
	} else {
		// Provisioned shards
		shardCount := asset.AttrInt("shard_count", 1)
		units = append(units, clouds.NewCostUnit(
			"shards",
			"shard-hours",
			float64(shardCount)*monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonKinesis",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "Kinesis:shardHour",
				},
			},
			0.95,
		))
	}

	// PUT payload units
	if putUnits, ok := usageVecs.Get("put_units_millions"); ok {
		units = append(units, clouds.NewCostUnit(
			"put_payload_units",
			"million-units",
			putUnits,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonKinesis",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "Kinesis:PutRecordPayloadBytes",
				},
			},
			0.5,
		))
	}

	// Extended retention
	retentionPeriod := asset.AttrInt("retention_period", 24)
	if retentionPeriod > 24 {
		// Extended retention per shard-hour
		shardCount := asset.AttrInt("shard_count", 1)
		units = append(units, clouds.NewCostUnit(
			"extended_retention",
			"shard-hours",
			float64(shardCount)*monthlyHours,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonKinesis",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "Kinesis:ExtendedRetention",
				},
			},
			0.95,
		))
	}

	return units, nil
}
