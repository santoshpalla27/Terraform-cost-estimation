// Package dns - AWS Route53 cost mapper
// Pricing model:
// - Hosted zones: per zone per month
// - Queries: per million queries (varies by type)
// - Health checks: per endpoint per month
// - Traffic policies: per policy record
// - Resolver endpoints: per ENI per hour
package dns

import (
	"terraform-cost/clouds"
)

// HostedZoneMapper maps aws_route53_zone to cost units
type HostedZoneMapper struct{}

// NewHostedZoneMapper creates a Route53 Hosted Zone mapper
func NewHostedZoneMapper() *HostedZoneMapper {
	return &HostedZoneMapper{}
}

// Cloud returns the cloud provider
func (m *HostedZoneMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *HostedZoneMapper) ResourceType() string {
	return "aws_route53_zone"
}

// BuildUsage extracts usage vectors
func (m *HostedZoneMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage("zones", "unknown zone count: "+asset.Cardinality.Reason),
		}, nil
	}

	// Queries are usage-dependent
	queriesMillions := ctx.ResolveOrDefault("monthly_queries_millions", -1)

	vectors := []clouds.UsageVector{
		clouds.NewUsageVector("zones", 1, 1.0),
	}

	if queriesMillions >= 0 {
		vectors = append(vectors, clouds.NewUsageVector("queries_millions", queriesMillions, 0.5))
	} else {
		vectors = append(vectors, clouds.SymbolicUsage("queries_millions", "query volume not provided"))
	}

	return vectors, nil
}

// BuildCostUnits creates cost units
func (m *HostedZoneMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	providerID := asset.ProviderContext.ProviderID
	region := "global" // Route53 is global

	units := []clouds.CostUnit{
		// Hosted zone - $0.50/month (first 25 free)
		clouds.NewCostUnit(
			"hosted_zone",
			"zones",
			1,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonRoute53",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "HostedZone",
				},
			},
			0.95,
		),
	}

	// Queries
	if queriesMillions, ok := usageVecs.Get("queries_millions"); ok {
		units = append(units, clouds.NewCostUnit(
			"queries",
			"million-queries",
			queriesMillions,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonRoute53",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "DNS-Queries",
				},
			},
			0.5,
		))
	}

	return units, nil
}

// HealthCheckMapper maps aws_route53_health_check to cost units
type HealthCheckMapper struct{}

// NewHealthCheckMapper creates a Route53 Health Check mapper
func NewHealthCheckMapper() *HealthCheckMapper {
	return &HealthCheckMapper{}
}

// Cloud returns the cloud provider
func (m *HealthCheckMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *HealthCheckMapper) ResourceType() string {
	return "aws_route53_health_check"
}

// BuildUsage extracts usage vectors
func (m *HealthCheckMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage("health_checks", "unknown health check count"),
		}, nil
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector("health_checks", 1, 1.0),
	}, nil
}

// BuildCostUnits creates cost units
func (m *HealthCheckMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("health_check", "health check cost unknown"),
		}, nil
	}

	providerID := asset.ProviderContext.ProviderID

	// Determine health check type
	healthCheckType := asset.Attr("type")
	if healthCheckType == "" {
		healthCheckType = "HTTP"
	}

	// HTTPS and string matching cost more
	usageType := "Health-Check-AWS"
	if healthCheckType == "HTTPS" || healthCheckType == "HTTPS_STR_MATCH" {
		usageType = "Health-Check-AWS-HTTPS"
	}

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"health_check",
			"endpoints",
			1,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonRoute53",
				Region:   "global",
				Attributes: map[string]string{
					"usageType":       usageType,
					"healthCheckType": healthCheckType,
				},
			},
			0.95,
		),
	}, nil
}
