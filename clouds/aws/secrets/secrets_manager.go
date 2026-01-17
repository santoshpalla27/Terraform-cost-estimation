// Package secrets - AWS Secrets Manager cost mapper
// Pricing model:
// - Secret storage: per secret per month
// - API calls: per 10,000 API calls
package secrets

import (
	"terraform-cost/clouds"
)

// SecretsManagerMapper maps aws_secretsmanager_secret to cost units
type SecretsManagerMapper struct{}

// NewSecretsManagerMapper creates a Secrets Manager mapper
func NewSecretsManagerMapper() *SecretsManagerMapper {
	return &SecretsManagerMapper{}
}

// Cloud returns the cloud provider
func (m *SecretsManagerMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *SecretsManagerMapper) ResourceType() string {
	return "aws_secretsmanager_secret"
}

// BuildUsage extracts usage vectors
func (m *SecretsManagerMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage("secrets", "unknown secret count: "+asset.Cardinality.Reason),
		}, nil
	}

	// API calls are usage-dependent
	monthlyAPICalls := ctx.ResolveOrDefault("monthly_api_calls", -1)

	vectors := []clouds.UsageVector{
		clouds.NewUsageVector("secrets", 1, 1.0),
	}

	if monthlyAPICalls >= 0 {
		vectors = append(vectors, clouds.NewUsageVector("api_calls", monthlyAPICalls, 0.5))
	} else {
		vectors = append(vectors, clouds.SymbolicUsage("api_calls", "API call volume not provided"))
	}

	return vectors, nil
}

// BuildCostUnits creates cost units
func (m *SecretsManagerMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	units := []clouds.CostUnit{
		// Per secret per month ($0.40/secret/month)
		clouds.NewCostUnit(
			"secret_storage",
			"secrets",
			1,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AWSSecretsManager",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "SecretsManager-Secret",
				},
			},
			0.95,
		),
	}

	// API calls ($0.05 per 10,000)
	if apiCalls, ok := usageVecs.Get("api_calls"); ok {
		units = append(units, clouds.NewCostUnit(
			"api_calls",
			"10k-calls",
			apiCalls/10000,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AWSSecretsManager",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "SecretsManager-API",
				},
			},
			0.5,
		))
	}

	return units, nil
}
