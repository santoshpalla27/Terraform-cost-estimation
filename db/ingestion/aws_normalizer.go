// Package ingestion - Production AWS pricing normalizer
// Mapper-agnostic normalization of complete AWS pricing catalogs
package ingestion

import (
	"strings"

	"terraform-cost/db"

	"github.com/shopspring/decimal"
)

// AWSPricingNormalizer normalizes raw AWS pricing to canonical format
// This is MAPPER-AGNOSTIC - normalizes ALL pricing, not filtered by resources
type AWSPricingNormalizer struct{}

// NewAWSPricingNormalizer creates a production normalizer
func NewAWSPricingNormalizer() *AWSPricingNormalizer {
	return &AWSPricingNormalizer{}
}

// Cloud implements PriceNormalizer
func (n *AWSPricingNormalizer) Cloud() db.CloudProvider {
	return db.AWS
}

// Normalize converts raw AWS prices to normalized rates
// This normalizes the COMPLETE catalog, not filtered by mappers
func (n *AWSPricingNormalizer) Normalize(raw []RawPrice) ([]NormalizedRate, error) {
	var rates []NormalizedRate

	for _, r := range raw {
		// Skip zero prices (free tier entries)
		price, err := ParsePrice(r.PricePerUnit)
		if err != nil {
			continue
		}

		// Normalize attributes to canonical keys
		attrs := n.normalizeAttributes(r.Attributes)

		// Create rate key
		rateKey := db.RateKey{
			Cloud:         db.AWS,
			Service:       r.ServiceCode,
			ProductFamily: r.ProductFamily,
			Region:        n.normalizeRegion(r.Attributes, r.Region),
			Attributes:    attrs,
		}

		// Create normalized rate
		nr := NormalizedRate{
			RateKey:    rateKey,
			Unit:       n.normalizeUnit(r.Unit),
			Price:      price,
			Currency:   r.Currency,
			Confidence: 1.0, // Direct from AWS API
		}

		// Handle tiers
		if r.TierStart != nil {
			d := decimal.NewFromFloat(*r.TierStart)
			nr.TierMin = &d
		}
		if r.TierEnd != nil {
			d := decimal.NewFromFloat(*r.TierEnd)
			nr.TierMax = &d
		}

		rates = append(rates, nr)
	}

	return rates, nil
}

// normalizeAttributes converts AWS-specific attribute keys to canonical form
func (n *AWSPricingNormalizer) normalizeAttributes(raw map[string]string) map[string]string {
	result := make(map[string]string)

	for k, v := range raw {
		// Skip empty values
		if v == "" || v == "NA" {
			continue
		}

		// Normalize to canonical keys
		canonicalKey := n.canonicalizeKey(k)
		canonicalValue := strings.ToLower(strings.TrimSpace(v))

		result[canonicalKey] = canonicalValue
	}

	return result
}

// canonicalizeKey maps AWS attribute names to canonical names
func (n *AWSPricingNormalizer) canonicalizeKey(k string) string {
	// Standard AWS to canonical mappings
	mapping := map[string]string{
		// EC2
		"instanceType":            "instance_type",
		"operatingSystem":         "os",
		"tenancy":                 "tenancy",
		"preInstalledSw":          "preinstalled_sw",
		"licenseModel":            "license_model",
		"capacitystatus":          "capacity_status",
		"usagetype":               "usage_type",
		"operation":               "operation",
		"physicalProcessor":       "physical_processor",
		"processorArchitecture":   "processor_architecture",
		"vcpu":                    "vcpu",
		"memory":                  "memory",
		"storage":                 "storage",
		"networkPerformance":      "network_performance",
		"ecu":                     "ecu",
		"currentGeneration":       "current_generation",
		"instanceFamily":          "instance_family",

		// EBS
		"volumeApiName":           "volume_type",
		"volumeType":              "volume_type",
		"maxVolumeSize":           "max_volume_size",
		"maxIopsvolume":           "max_iops",
		"maxThroughputvolume":     "max_throughput",

		// RDS
		"databaseEngine":          "engine",
		"databaseEdition":         "edition",
		"deploymentOption":        "deployment_option",
		"instanceTypeFamily":      "instance_family",

		// Lambda
		"group":                   "group",
		"memorySize":              "memory_size",

		// S3
		"storageClass":            "storage_class",
		"volumeType":              "volume_type",

		// ElastiCache
		"cacheEngine":             "cache_engine",

		// Load Balancer
		"productFamily":           "product_family",

		// DynamoDB
		"termType":                "term_type",

		// Data Transfer
		"transferType":            "transfer_type",
		"fromLocation":            "from_location",
		"toLocation":              "to_location",
		"fromLocationType":        "from_location_type",
		"toLocationType":          "to_location_type",

		// General
		"servicecode":             "service_code",
		"location":                "location",
		"locationType":            "location_type",
		"regionCode":              "region_code",
	}

	if canonical, ok := mapping[k]; ok {
		return canonical
	}

	// Convert camelCase to snake_case for unknown keys
	return toSnakeCase(k)
}

// normalizeUnit converts AWS units to canonical form
func (n *AWSPricingNormalizer) normalizeUnit(unit string) string {
	mapping := map[string]string{
		"Hrs":                   "hours",
		"Hours":                 "hours",
		"GB":                    "GB",
		"GB-Mo":                 "GB-month",
		"GB-Month":              "GB-month",
		"Requests":              "requests",
		"Request":               "requests",
		"GB-Second":             "GB-seconds",
		"GB-Seconds":            "GB-seconds",
		"Lambda-GB-Second":      "GB-seconds",
		"vCPU-Hours":            "vCPU-hours",
		"ACU-Hr":                "ACU-hours",
		"LCU-Hrs":               "LCU-hours",
		"NLCU-Hrs":              "NLCU-hours",
		"IOPS-Mo":               "IOPS-month",
		"WriteCapacityUnit-Hrs": "WCU-hours",
		"ReadCapacityUnit-Hrs":  "RCU-hours",
		"WriteRequestUnits":     "WRU",
		"ReadRequestUnits":      "RRU",
		"Quantity":              "quantity",
		"Count":                 "count",
	}

	if normalized, ok := mapping[unit]; ok {
		return normalized
	}

	return strings.ToLower(unit)
}

// normalizeRegion extracts region from attributes if not set
func (n *AWSPricingNormalizer) normalizeRegion(attrs map[string]string, defaultRegion string) string {
	if regionCode, ok := attrs["regionCode"]; ok && regionCode != "" {
		return regionCode
	}
	return defaultRegion
}

// toSnakeCase converts camelCase to snake_case
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
