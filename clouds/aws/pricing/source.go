// Package pricing provides AWS pricing source.
package pricing

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	corePricing "terraform-cost/core/pricing"
	"terraform-cost/core/types"
)

// AWSPricingSource fetches pricing from AWS Pricing API
type AWSPricingSource struct {
	defaultRegion string
	// In production, this would use the AWS SDK
	// client *pricing.Client
}

// NewAWSPricingSource creates a new AWS pricing source
func NewAWSPricingSource(region string) corePricing.Source {
	return &AWSPricingSource{
		defaultRegion: region,
	}
}

// Provider returns AWS
func (s *AWSPricingSource) Provider() types.Provider {
	return types.ProviderAWS
}

// FetchRates retrieves rates for the given keys
func (s *AWSPricingSource) FetchRates(ctx context.Context, keys []types.RateKey) ([]types.Rate, error) {
	rates := make([]types.Rate, 0, len(keys))

	for _, key := range keys {
		rate, err := s.fetchRate(ctx, key)
		if err != nil {
			continue // Skip missing rates
		}
		rates = append(rates, rate)
	}

	return rates, nil
}

// FetchAll retrieves all rates for a region
func (s *AWSPricingSource) FetchAll(ctx context.Context, region string) ([]types.Rate, error) {
	// In production, this would call the AWS Pricing API
	// For now, return common defaults
	return s.getDefaultRates(region), nil
}

// SupportedRegions returns the list of supported AWS regions
func (s *AWSPricingSource) SupportedRegions() []string {
	return []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1",
		"ap-northeast-1", "ap-northeast-2", "ap-southeast-1", "ap-southeast-2",
		"ap-south-1", "sa-east-1", "ca-central-1",
	}
}

func (s *AWSPricingSource) fetchRate(ctx context.Context, key types.RateKey) (types.Rate, error) {
	// In production, this would call the AWS Pricing API
	// For now, return hardcoded defaults for common resources

	switch key.Service {
	case "EC2":
		return s.getEC2Rate(key)
	case "RDS":
		return s.getRDSRate(key)
	case "S3":
		return s.getS3Rate(key)
	case "Lambda":
		return s.getLambdaRate(key)
	case "NAT Gateway":
		return s.getNATGatewayRate(key)
	case "EBS":
		return s.getEBSRate(key)
	default:
		return types.Rate{}, fmt.Errorf("unknown service: %s", key.Service)
	}
}

func (s *AWSPricingSource) getEC2Rate(key types.RateKey) (types.Rate, error) {
	// Instance type pricing (On-Demand, us-east-1)
	instancePricing := map[string]float64{
		"t3.micro":    0.0104,
		"t3.small":    0.0208,
		"t3.medium":   0.0416,
		"t3.large":    0.0832,
		"t3.xlarge":   0.1664,
		"t3.2xlarge":  0.3328,
		"m5.large":    0.096,
		"m5.xlarge":   0.192,
		"m5.2xlarge":  0.384,
		"m5.4xlarge":  0.768,
		"c5.large":    0.085,
		"c5.xlarge":   0.17,
		"c5.2xlarge":  0.34,
		"r5.large":    0.126,
		"r5.xlarge":   0.252,
		"r5.2xlarge":  0.504,
	}

	instanceType := key.Attributes["instance_type"]
	price, ok := instancePricing[instanceType]
	if !ok {
		price = 0.10 // Default
	}

	return types.Rate{
		Key:           key,
		Price:         decimal.NewFromFloat(price),
		Unit:          "hour",
		Currency:      types.CurrencyUSD,
		EffectiveFrom: time.Now().AddDate(0, -1, 0),
	}, nil
}

func (s *AWSPricingSource) getRDSRate(key types.RateKey) (types.Rate, error) {
	// RDS instance pricing (Single-AZ, us-east-1)
	instancePricing := map[string]float64{
		"db.t3.micro":   0.017,
		"db.t3.small":   0.034,
		"db.t3.medium":  0.068,
		"db.t3.large":   0.136,
		"db.m5.large":   0.171,
		"db.m5.xlarge":  0.342,
		"db.m5.2xlarge": 0.684,
		"db.r5.large":   0.24,
		"db.r5.xlarge":  0.48,
		"db.r5.2xlarge": 0.96,
	}

	instanceClass := key.Attributes["instance_class"]
	price, ok := instancePricing[instanceClass]
	if !ok {
		price = 0.10 // Default
	}

	return types.Rate{
		Key:           key,
		Price:         decimal.NewFromFloat(price),
		Unit:          "hour",
		Currency:      types.CurrencyUSD,
		EffectiveFrom: time.Now().AddDate(0, -1, 0),
	}, nil
}

func (s *AWSPricingSource) getS3Rate(key types.RateKey) (types.Rate, error) {
	// S3 pricing (Standard, us-east-1)
	storageClass := key.Attributes["storage_class"]
	if storageClass == "" {
		storageClass = "STANDARD"
	}

	prices := map[string]float64{
		"STANDARD":            0.023,  // per GB-month
		"STANDARD_IA":         0.0125,
		"ONEZONE_IA":          0.01,
		"GLACIER":             0.004,
		"GLACIER_DEEP_ARCHIVE": 0.00099,
	}

	price, ok := prices[storageClass]
	if !ok {
		price = 0.023
	}

	return types.Rate{
		Key:           key,
		Price:         decimal.NewFromFloat(price),
		Unit:          "GB-month",
		Currency:      types.CurrencyUSD,
		EffectiveFrom: time.Now().AddDate(0, -1, 0),
	}, nil
}

func (s *AWSPricingSource) getLambdaRate(key types.RateKey) (types.Rate, error) {
	// Lambda is priced per GB-second and per request
	rateType := key.Attributes["rate_type"]

	switch rateType {
	case "requests":
		return types.Rate{
			Key:           key,
			Price:         decimal.NewFromFloat(0.0000002), // $0.20 per 1M requests
			Unit:          "request",
			Currency:      types.CurrencyUSD,
			EffectiveFrom: time.Now().AddDate(0, -1, 0),
		}, nil
	case "duration":
		return types.Rate{
			Key:           key,
			Price:         decimal.NewFromFloat(0.0000166667), // per GB-second
			Unit:          "GB-second",
			Currency:      types.CurrencyUSD,
			EffectiveFrom: time.Now().AddDate(0, -1, 0),
		}, nil
	default:
		return types.Rate{}, fmt.Errorf("unknown Lambda rate type")
	}
}

func (s *AWSPricingSource) getNATGatewayRate(key types.RateKey) (types.Rate, error) {
	rateType := key.Attributes["rate_type"]

	switch rateType {
	case "hourly":
		return types.Rate{
			Key:           key,
			Price:         decimal.NewFromFloat(0.045), // per hour
			Unit:          "hour",
			Currency:      types.CurrencyUSD,
			EffectiveFrom: time.Now().AddDate(0, -1, 0),
		}, nil
	case "data":
		return types.Rate{
			Key:           key,
			Price:         decimal.NewFromFloat(0.045), // per GB processed
			Unit:          "GB",
			Currency:      types.CurrencyUSD,
			EffectiveFrom: time.Now().AddDate(0, -1, 0),
		}, nil
	default:
		return types.Rate{}, fmt.Errorf("unknown NAT Gateway rate type")
	}
}

func (s *AWSPricingSource) getEBSRate(key types.RateKey) (types.Rate, error) {
	// EBS pricing per GB-month
	volumeType := key.Attributes["volume_type"]

	prices := map[string]float64{
		"gp3":      0.08,
		"gp2":      0.10,
		"io1":      0.125,
		"io2":      0.125,
		"st1":      0.045,
		"sc1":      0.015,
		"standard": 0.05,
	}

	price, ok := prices[volumeType]
	if !ok {
		price = 0.10
	}

	return types.Rate{
		Key:           key,
		Price:         decimal.NewFromFloat(price),
		Unit:          "GB-month",
		Currency:      types.CurrencyUSD,
		EffectiveFrom: time.Now().AddDate(0, -1, 0),
	}, nil
}

func (s *AWSPricingSource) getDefaultRates(region string) []types.Rate {
	// Return a set of common default rates
	// In production, this would be fetched from AWS
	return []types.Rate{}
}
