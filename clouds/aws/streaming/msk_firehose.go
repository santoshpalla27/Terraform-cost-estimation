// Package streaming - AWS MSK and Kinesis Firehose mappers
package streaming

import (
	"terraform-cost/clouds"
)

// MSKClusterMapper maps aws_msk_cluster to cost units
type MSKClusterMapper struct{}

func NewMSKClusterMapper() *MSKClusterMapper { return &MSKClusterMapper{} }

func (m *MSKClusterMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *MSKClusterMapper) ResourceType() string        { return "aws_msk_cluster" }

func (m *MSKClusterMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown MSK cluster count")}, nil
	}
	return []clouds.UsageVector{clouds.NewUsageVector(clouds.MetricMonthlyHours, 730, 0.95)}, nil
}

func (m *MSKClusterMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("msk", "MSK cluster count unknown")}, nil
	}

	instanceType := asset.Attr("broker_node_group_info.0.instance_type")
	if instanceType == "" {
		instanceType = "kafka.m5.large"
	}

	numberOfBrokerNodes := asset.AttrInt("number_of_broker_nodes", 3)
	ebsVolumeSize := asset.AttrFloat("broker_node_group_info.0.ebs_volume_size", 100)
	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	return []clouds.CostUnit{
		clouds.NewCostUnit("broker_hours", "broker-hours", float64(numberOfBrokerNodes)*monthlyHours, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonMSK",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"instanceType": instanceType,
			},
		}, 0.95),
		clouds.NewCostUnit("storage", "GB-months", ebsVolumeSize*float64(numberOfBrokerNodes), clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonMSK",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "Storage",
			},
		}, 0.95),
	}, nil
}

// KinesisFirehoseMapper maps aws_kinesis_firehose_delivery_stream to cost units
type KinesisFirehoseMapper struct{}

func NewKinesisFirehoseMapper() *KinesisFirehoseMapper { return &KinesisFirehoseMapper{} }

func (m *KinesisFirehoseMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *KinesisFirehoseMapper) ResourceType() string        { return "aws_kinesis_firehose_delivery_stream" }

func (m *KinesisFirehoseMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage("streams", "unknown stream count")}, nil
	}

	dataIngestionGB := ctx.ResolveOrDefault("data_ingestion_gb", -1)
	if dataIngestionGB < 0 {
		return []clouds.UsageVector{clouds.SymbolicUsage("data_ingestion", "data ingestion volume not provided")}, nil
	}

	return []clouds.UsageVector{clouds.NewUsageVector("data_ingestion_gb", dataIngestionGB, 0.5)}, nil
}

func (m *KinesisFirehoseMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("firehose", "Firehose cost depends on data ingestion volume"),
		}, nil
	}

	dataIngestionGB, _ := usageVecs.Get("data_ingestion_gb")

	return []clouds.CostUnit{
		clouds.NewCostUnit("data_ingestion", "GB", dataIngestionGB, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonKinesisFirehose",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "DataIngested",
			},
		}, 0.5),
	}, nil
}
