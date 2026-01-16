// Package messaging - AWS SQS/SNS cost mapper
// SQS Pricing:
// - Requests: per million requests (Standard vs FIFO)
// - Data transfer: outbound
// SNS Pricing:
// - Publishes: per million
// - Deliveries: per protocol (HTTP, Email, SMS, Lambda)
// - Data transfer: outbound
package messaging

import (
	"terraform-cost/clouds"
)

// SQSMapper maps aws_sqs_queue to cost units
type SQSMapper struct{}

// NewSQSMapper creates an SQS mapper
func NewSQSMapper() *SQSMapper {
	return &SQSMapper{}
}

// Cloud returns the cloud provider
func (m *SQSMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *SQSMapper) ResourceType() string {
	return "aws_sqs_queue"
}

// BuildUsage extracts usage vectors
func (m *SQSMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyRequests, "unknown queue count: "+asset.Cardinality.Reason),
		}, nil
	}

	// SQS is HIGHLY usage-dependent
	monthlyRequests := ctx.ResolveOrDefault("monthly_requests", -1)

	if monthlyRequests < 0 {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyRequests, "monthly SQS requests not provided"),
		}, nil
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyRequests, monthlyRequests, 0.5),
	}, nil
}

// BuildCostUnits creates cost units
func (m *SQSMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("sqs_requests", "SQS cost requires request volume"),
		}, nil
	}

	monthlyRequests, _ := usageVecs.Get(clouds.MetricMonthlyRequests)

	// FIFO queues cost more
	isFIFO := asset.AttrBool("fifo_queue", false)
	queueType := "Standard"
	if isFIFO {
		queueType = "FIFO"
	}

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"requests",
			"million-requests",
			monthlyRequests/1000000,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonSQS",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "Requests",
					"queueType": queueType,
				},
			},
			0.5,
		),
	}, nil
}

// SNSMapper maps aws_sns_topic to cost units
type SNSMapper struct{}

// NewSNSMapper creates an SNS mapper
func NewSNSMapper() *SNSMapper {
	return &SNSMapper{}
}

// Cloud returns the cloud provider
func (m *SNSMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *SNSMapper) ResourceType() string {
	return "aws_sns_topic"
}

// BuildUsage extracts usage vectors
func (m *SNSMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage("publishes", "unknown topic count: "+asset.Cardinality.Reason),
		}, nil
	}

	// SNS is HIGHLY usage-dependent
	monthlyPublishes := ctx.ResolveOrDefault("monthly_publishes", -1)

	if monthlyPublishes < 0 {
		return []clouds.UsageVector{
			clouds.SymbolicUsage("publishes", "monthly SNS publishes not provided"),
		}, nil
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector("publishes", monthlyPublishes, 0.5),
	}, nil
}

// BuildCostUnits creates cost units
func (m *SNSMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("sns_publishes", "SNS cost requires publish volume"),
		}, nil
	}

	monthlyPublishes, _ := usageVecs.Get("publishes")

	// FIFO topics cost more
	isFIFO := asset.AttrBool("fifo_topic", false)
	topicType := "Standard"
	if isFIFO {
		topicType = "FIFO"
	}

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"publishes",
			"million-publishes",
			monthlyPublishes/1000000,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonSNS",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "PublishAPI-Requests",
					"topicType": topicType,
				},
			},
			0.5,
		),
		// Deliveries depend on subscriber types - symbolic
		clouds.SymbolicCost("deliveries", "delivery cost depends on subscriber protocols"),
	}, nil
}
