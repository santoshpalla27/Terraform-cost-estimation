// Package ingestion - AWS pricing fetcher and normalizer
package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"terraform-cost/db"

	"github.com/shopspring/decimal"
)

// AWSFetcher fetches pricing from AWS
type AWSFetcher struct {
	httpClient *http.Client
	regions    []string
}

// NewAWSFetcher creates a new AWS pricing fetcher
func NewAWSFetcher() *AWSFetcher {
	return &AWSFetcher{
		httpClient: &http.Client{},
		regions: []string{
			"us-east-1", "us-east-2", "us-west-1", "us-west-2",
			"eu-west-1", "eu-west-2", "eu-central-1",
			"ap-southeast-1", "ap-southeast-2", "ap-northeast-1",
		},
	}
}

func (f *AWSFetcher) Cloud() db.CloudProvider {
	return db.AWS
}

func (f *AWSFetcher) SupportedRegions() []string {
	return f.regions
}

// FetchRegion fetches AWS pricing for a region
// Note: In production, use AWS Pricing API or bulk price list files
func (f *AWSFetcher) FetchRegion(ctx context.Context, region string) ([]RawPrice, error) {
	// AWS Pricing API endpoint (simplified)
	// In production, use: https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/index.json
	
	// For now, return stub data for development
	return f.getStubPrices(region), nil
}

// getStubPrices returns development stub prices
func (f *AWSFetcher) getStubPrices(region string) []RawPrice {
	return []RawPrice{
		// EC2 Instances
		{SKU: "ec2-t3-micro", ServiceCode: "AmazonEC2", ProductFamily: "Compute Instance", Region: region,
			Unit: "Hrs", PricePerUnit: "0.0104", Currency: "USD",
			Attributes: map[string]string{"instanceType": "t3.micro", "operatingSystem": "Linux", "tenancy": "Shared"}},
		{SKU: "ec2-t3-small", ServiceCode: "AmazonEC2", ProductFamily: "Compute Instance", Region: region,
			Unit: "Hrs", PricePerUnit: "0.0208", Currency: "USD",
			Attributes: map[string]string{"instanceType": "t3.small", "operatingSystem": "Linux", "tenancy": "Shared"}},
		{SKU: "ec2-t3-medium", ServiceCode: "AmazonEC2", ProductFamily: "Compute Instance", Region: region,
			Unit: "Hrs", PricePerUnit: "0.0416", Currency: "USD",
			Attributes: map[string]string{"instanceType": "t3.medium", "operatingSystem": "Linux", "tenancy": "Shared"}},
		{SKU: "ec2-t3-large", ServiceCode: "AmazonEC2", ProductFamily: "Compute Instance", Region: region,
			Unit: "Hrs", PricePerUnit: "0.0832", Currency: "USD",
			Attributes: map[string]string{"instanceType": "t3.large", "operatingSystem": "Linux", "tenancy": "Shared"}},
		{SKU: "ec2-m5-large", ServiceCode: "AmazonEC2", ProductFamily: "Compute Instance", Region: region,
			Unit: "Hrs", PricePerUnit: "0.096", Currency: "USD",
			Attributes: map[string]string{"instanceType": "m5.large", "operatingSystem": "Linux", "tenancy": "Shared"}},
		
		// EBS Volumes
		{SKU: "ebs-gp3", ServiceCode: "AmazonEC2", ProductFamily: "Storage", Region: region,
			Unit: "GB-Mo", PricePerUnit: "0.08", Currency: "USD",
			Attributes: map[string]string{"volumeApiName": "gp3"}},
		{SKU: "ebs-gp2", ServiceCode: "AmazonEC2", ProductFamily: "Storage", Region: region,
			Unit: "GB-Mo", PricePerUnit: "0.10", Currency: "USD",
			Attributes: map[string]string{"volumeApiName": "gp2"}},
		{SKU: "ebs-io1", ServiceCode: "AmazonEC2", ProductFamily: "Storage", Region: region,
			Unit: "GB-Mo", PricePerUnit: "0.125", Currency: "USD",
			Attributes: map[string]string{"volumeApiName": "io1"}},
		
		// RDS
		{SKU: "rds-mysql-db.t3.micro", ServiceCode: "AmazonRDS", ProductFamily: "Database Instance", Region: region,
			Unit: "Hrs", PricePerUnit: "0.017", Currency: "USD",
			Attributes: map[string]string{"instanceType": "db.t3.micro", "databaseEngine": "MySQL"}},
		{SKU: "rds-mysql-db.t3.small", ServiceCode: "AmazonRDS", ProductFamily: "Database Instance", Region: region,
			Unit: "Hrs", PricePerUnit: "0.034", Currency: "USD",
			Attributes: map[string]string{"instanceType": "db.t3.small", "databaseEngine": "MySQL"}},
		{SKU: "rds-postgres-db.t3.small", ServiceCode: "AmazonRDS", ProductFamily: "Database Instance", Region: region,
			Unit: "Hrs", PricePerUnit: "0.036", Currency: "USD",
			Attributes: map[string]string{"instanceType": "db.t3.small", "databaseEngine": "PostgreSQL"}},
		
		// NAT Gateway
		{SKU: "nat-gateway-hour", ServiceCode: "AmazonEC2", ProductFamily: "NAT Gateway", Region: region,
			Unit: "Hrs", PricePerUnit: "0.045", Currency: "USD",
			Attributes: map[string]string{"usagetype": "NatGateway-Hours"}},
		{SKU: "nat-gateway-data", ServiceCode: "AmazonEC2", ProductFamily: "NAT Gateway", Region: region,
			Unit: "GB", PricePerUnit: "0.045", Currency: "USD",
			Attributes: map[string]string{"usagetype": "NatGateway-Bytes"}},
		
		// Lambda
		{SKU: "lambda-requests", ServiceCode: "AWSLambda", ProductFamily: "Serverless", Region: region,
			Unit: "Requests", PricePerUnit: "0.0000002", Currency: "USD",
			Attributes: map[string]string{"group": "AWS-Lambda-Requests"}},
		{SKU: "lambda-duration", ServiceCode: "AWSLambda", ProductFamily: "Serverless", Region: region,
			Unit: "GB-Second", PricePerUnit: "0.0000166667", Currency: "USD",
			Attributes: map[string]string{"group": "AWS-Lambda-Duration"}},
		
		// Load Balancer
		{SKU: "alb-hour", ServiceCode: "ElasticLoadBalancing", ProductFamily: "Load Balancer", Region: region,
			Unit: "Hrs", PricePerUnit: "0.0225", Currency: "USD",
			Attributes: map[string]string{"productFamily": "Load Balancer-Application"}},
		{SKU: "nlb-hour", ServiceCode: "ElasticLoadBalancing", ProductFamily: "Load Balancer", Region: region,
			Unit: "Hrs", PricePerUnit: "0.0225", Currency: "USD",
			Attributes: map[string]string{"productFamily": "Load Balancer-Network"}},
	}
}

// AWSNormalizer normalizes AWS pricing data
type AWSNormalizer struct{}

func NewAWSNormalizer() *AWSNormalizer {
	return &AWSNormalizer{}
}

func (n *AWSNormalizer) Cloud() db.CloudProvider {
	return db.AWS
}

// Normalize converts raw AWS prices to normalized rates
func (n *AWSNormalizer) Normalize(raw []RawPrice) ([]NormalizedRate, error) {
	var rates []NormalizedRate
	
	for _, r := range raw {
		price, err := ParsePrice(r.PricePerUnit)
		if err != nil {
			continue // Skip unparseable prices
		}
		
		// Normalize attributes
		attrs := n.normalizeAttributes(r.Attributes)
		
		// Create rate key
		rateKey := db.RateKey{
			Cloud:         db.AWS,
			Service:       r.ServiceCode,
			ProductFamily: r.ProductFamily,
			Region:        r.Region,
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

func (n *AWSNormalizer) normalizeAttributes(raw map[string]string) map[string]string {
	result := make(map[string]string)
	
	for k, v := range raw {
		// Normalize key names
		key := n.normalizeKey(k)
		// Normalize values
		val := strings.ToLower(strings.TrimSpace(v))
		result[key] = val
	}
	
	return result
}

func (n *AWSNormalizer) normalizeKey(k string) string {
	// Map AWS attribute names to canonical names
	mapping := map[string]string{
		"instanceType":     "instance_type",
		"operatingSystem":  "os",
		"tenancy":          "tenancy",
		"volumeApiName":    "volume_type",
		"databaseEngine":   "engine",
		"usagetype":        "usage_type",
		"productFamily":    "product_family",
		"group":            "group",
	}
	
	if canonical, ok := mapping[k]; ok {
		return canonical
	}
	return strings.ToLower(strings.ReplaceAll(k, " ", "_"))
}

func (n *AWSNormalizer) normalizeUnit(unit string) string {
	// Normalize unit names
	mapping := map[string]string{
		"Hrs":        "hours",
		"GB-Mo":      "GB-month",
		"GB":         "GB",
		"Requests":   "requests",
		"GB-Second":  "GB-seconds",
	}
	
	if normalized, ok := mapping[unit]; ok {
		return normalized
	}
	return strings.ToLower(unit)
}

// fetchFromAWSAPI is the production implementation (placeholder)
func fetchFromAWSAPI(ctx context.Context, region, service string) ([]byte, error) {
	// In production:
	// 1. Use AWS Pricing API: pricing.GetProducts()
	// 2. Or download bulk price list: https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/{service}/current/index.json
	return nil, fmt.Errorf("not implemented: use AWS SDK in production")
}

// parseAWSPriceList parses AWS bulk price list JSON
func parseAWSPriceList(data []byte) ([]RawPrice, error) {
	var result struct {
		Products map[string]struct {
			SKU           string            `json:"sku"`
			ProductFamily string            `json:"productFamily"`
			Attributes    map[string]string `json:"attributes"`
		} `json:"products"`
		Terms struct {
			OnDemand map[string]map[string]struct {
				PriceDimensions map[string]struct {
					Unit         string `json:"unit"`
					PricePerUnit struct {
						USD string `json:"USD"`
					} `json:"pricePerUnit"`
				} `json:"priceDimensions"`
			} `json:"OnDemand"`
		} `json:"terms"`
	}
	
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	
	// Parse would continue here...
	return nil, nil
}
