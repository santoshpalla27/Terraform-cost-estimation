// Package serverless - AWS Lambda Provisioned Concurrency mapper
package serverless

import (
	"terraform-cost/clouds"
)

// LambdaProvisionedConcurrencyMapper maps aws_lambda_provisioned_concurrency_config
type LambdaProvisionedConcurrencyMapper struct{}

func NewLambdaProvisionedConcurrencyMapper() *LambdaProvisionedConcurrencyMapper {
	return &LambdaProvisionedConcurrencyMapper{}
}

func (m *LambdaProvisionedConcurrencyMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *LambdaProvisionedConcurrencyMapper) ResourceType() string {
	return "aws_lambda_provisioned_concurrency_config"
}

func (m *LambdaProvisionedConcurrencyMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown concurrency config count")}, nil
	}
	return []clouds.UsageVector{clouds.NewUsageVector(clouds.MetricMonthlyHours, 730, 0.95)}, nil
}

func (m *LambdaProvisionedConcurrencyMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("provisioned_concurrency", "concurrency count unknown")}, nil
	}

	provisionedCount := asset.AttrInt("provisioned_concurrent_executions", 1)
	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	// Need memory size from referenced function (cross-resource)
	// For now, emit symbolic with count info
	return []clouds.CostUnit{
		clouds.SymbolicCost("provisioned_concurrency",
			"Provisioned concurrency cost requires function memory size. Provisioned: "+
				string(rune('0'+provisionedCount))),
		clouds.NewCostUnit("provisioned_hours", "concurrency-hours", float64(provisionedCount)*monthlyHours, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AWSLambda",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "Lambda-Provisioned-Concurrency",
			},
		}, 0.7),
	}, nil
}
