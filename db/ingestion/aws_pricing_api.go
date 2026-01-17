// Package ingestion - Production AWS Pricing API client
// Fetches COMPLETE pricing catalogs - mapper-agnostic
package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"terraform-cost/db"
)

// AWSPricingAPIClient fetches pricing from real AWS Pricing API
// This is the PRODUCTION implementation - no stubs, no mocks
type AWSPricingAPIClient struct {
	httpClient      *http.Client
	bulkPricingURL  string
	region          string
	serviceCodeList []string
}

// AWSPricingConfig configures the AWS pricing client
type AWSPricingConfig struct {
	// Region for pricing API (always us-east-1 for pricing)
	Region string

	// HTTPTimeout for API calls
	HTTPTimeout time.Duration

	// Services to fetch (empty = ALL services)
	Services []string

	// UseBulkAPI uses bulk pricing files instead of GetProducts API
	UseBulkAPI bool
}

// DefaultAWSPricingConfig returns production defaults
func DefaultAWSPricingConfig() *AWSPricingConfig {
	return &AWSPricingConfig{
		Region:      "us-east-1",
		HTTPTimeout: 5 * time.Minute,
		UseBulkAPI:  true, // Bulk API is more reliable for full catalogs
		Services:    AllAWSServices(),
	}
}

// AllAWSServices returns all AWS services with pricing
func AllAWSServices() []string {
	return []string{
		// Compute
		"AmazonEC2",
		"AWSLambda",
		"AmazonECS",
		"AmazonEKS",
		"AWSFargate",
		"AmazonLightsail",

		// Storage
		"AmazonS3",
		"AmazonEFS",
		"AmazonFSx",
		"AmazonGlacier",

		// Database
		"AmazonRDS",
		"AmazonDynamoDB",
		"AmazonElastiCache",
		"AmazonRedshift",
		"AmazonDocDB",
		"AmazonNeptune",
		"AmazonMemoryDB",

		// Networking
		"AmazonVPC",
		"AWSDataTransfer",
		"AmazonCloudFront",
		"AmazonRoute53",
		"ElasticLoadBalancing",
		"AmazonApiGateway",
		"AWSDirectConnect",

		// Analytics
		"AmazonKinesis",
		"AmazonOpenSearchService",
		"AmazonAthena",
		"AWSGlue",
		"AmazonEMR",

		// Application Integration
		"AmazonSQS",
		"AmazonSNS",
		"AmazonEventBridge",
		"AWSStepFunctions",

		// Security & Identity
		"AWSSecretsManager",
		"AWSKeyManagementService",
		"AWSCertificateManager",
		"AWSWAF",
		"AWSShield",

		// Management & Monitoring
		"AmazonCloudWatch",
		"AWSCloudTrail",
		"AWSConfig",
		"AWSSystemsManager",

		// Containers
		"AmazonECR",

		// Machine Learning
		"AmazonSageMaker",
		"AmazonRekognition",
		"AmazonComprehend",
	}
}

// NewAWSPricingAPIClient creates a production AWS pricing client
func NewAWSPricingAPIClient(cfg *AWSPricingConfig) *AWSPricingAPIClient {
	if cfg == nil {
		cfg = DefaultAWSPricingConfig()
	}

	return &AWSPricingAPIClient{
		httpClient: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
		bulkPricingURL:  "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws",
		region:          cfg.Region,
		serviceCodeList: cfg.Services,
	}
}

// Cloud implements PriceFetcher
func (c *AWSPricingAPIClient) Cloud() db.CloudProvider {
	return db.AWS
}

// IsRealAPI implements RealAPIFetcher - THIS IS A REAL API
func (c *AWSPricingAPIClient) IsRealAPI() bool {
	return true
}

// SupportedRegions returns all AWS regions
func (c *AWSPricingAPIClient) SupportedRegions() []string {
	return []string{
		// US
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		// Europe
		"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1", "eu-south-1",
		// Asia Pacific
		"ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
		"ap-southeast-1", "ap-southeast-2", "ap-southeast-3",
		"ap-south-1", "ap-east-1",
		// South America
		"sa-east-1",
		// Canada
		"ca-central-1",
		// Middle East
		"me-south-1", "me-central-1",
		// Africa
		"af-south-1",
	}
}

// SupportedServices returns all supported services
func (c *AWSPricingAPIClient) SupportedServices() []string {
	return c.serviceCodeList
}

// FetchRegion fetches ALL pricing for a region from AWS APIs
// This is mapper-agnostic - fetches complete catalogs
func (c *AWSPricingAPIClient) FetchRegion(ctx context.Context, region string) ([]RawPrice, error) {
	var allPrices []RawPrice

	for _, serviceCode := range c.serviceCodeList {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		prices, err := c.fetchServicePricing(ctx, serviceCode, region)
		if err != nil {
			// Log but continue - don't fail on single service
			fmt.Printf("Warning: failed to fetch %s pricing: %v\n", serviceCode, err)
			continue
		}

		allPrices = append(allPrices, prices...)
	}

	if len(allPrices) == 0 {
		return nil, fmt.Errorf("failed to fetch any pricing for region %s", region)
	}

	return allPrices, nil
}

// fetchServicePricing fetches pricing for a single service
func (c *AWSPricingAPIClient) fetchServicePricing(ctx context.Context, serviceCode, region string) ([]RawPrice, error) {
	// Use bulk pricing files for complete catalogs
	url := fmt.Sprintf("%s/%s/current/region_index.json", c.bulkPricingURL, serviceCode)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pricing index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pricing API returned status %d", resp.StatusCode)
	}

	// Parse region index to find regional pricing file
	var regionIndex AWSSRegionIndex
	if err := json.NewDecoder(resp.Body).Decode(&regionIndex); err != nil {
		return nil, fmt.Errorf("failed to decode region index: %w", err)
	}

	// Find the region-specific offer file
	regionOffer, ok := regionIndex.Regions[region]
	if !ok {
		// Try with AWS location name mapping
		regionOffer, ok = regionIndex.Regions[awsRegionToLocation(region)]
		if !ok {
			return nil, fmt.Errorf("region %s not found in %s pricing", region, serviceCode)
		}
	}

	// Fetch the actual pricing data
	return c.fetchOfferFile(ctx, regionOffer.CurrentVersionURL, serviceCode, region)
}

// fetchOfferFile fetches and parses a pricing offer file
func (c *AWSPricingAPIClient) fetchOfferFile(ctx context.Context, offerURL, serviceCode, region string) ([]RawPrice, error) {
	// Resolve relative URL
	if !strings.HasPrefix(offerURL, "http") {
		offerURL = "https://pricing.us-east-1.amazonaws.com" + offerURL
	}

	req, err := http.NewRequestWithContext(ctx, "GET", offerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create offer request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch offer file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("offer file returned status %d", resp.StatusCode)
	}

	// Parse the offer file
	return c.parseOfferFile(resp.Body, serviceCode, region)
}

// parseOfferFile parses AWS pricing offer JSON
func (c *AWSPricingAPIClient) parseOfferFile(reader io.Reader, serviceCode, region string) ([]RawPrice, error) {
	var offer AWSOfferFile
	if err := json.NewDecoder(reader).Decode(&offer); err != nil {
		return nil, fmt.Errorf("failed to decode offer file: %w", err)
	}

	var prices []RawPrice

	// Extract products and their pricing
	for sku, product := range offer.Products {
		// Get on-demand pricing terms
		onDemandTerms, ok := offer.Terms.OnDemand[sku]
		if !ok {
			continue
		}

		for _, term := range onDemandTerms {
			for _, priceDimension := range term.PriceDimensions {
				usdPrice, ok := priceDimension.PricePerUnit["USD"]
				if !ok {
					continue
				}

				price := RawPrice{
					SKU:           sku,
					ServiceCode:   serviceCode,
					ProductFamily: product.ProductFamily,
					Region:        region,
					Unit:          priceDimension.Unit,
					PricePerUnit:  usdPrice,
					Currency:      "USD",
					Attributes:    product.Attributes,
				}

				// Parse effective date
				if term.EffectiveDate != "" {
					if t, err := time.Parse(time.RFC3339, term.EffectiveDate); err == nil {
						price.EffectiveDate = &t
					}
				}

				// Handle tiered pricing
				if priceDimension.BeginRange != "" && priceDimension.BeginRange != "0" {
					if start, err := parseFloat(priceDimension.BeginRange); err == nil {
						price.TierStart = &start
					}
				}
				if priceDimension.EndRange != "" && priceDimension.EndRange != "Inf" {
					if end, err := parseFloat(priceDimension.EndRange); err == nil {
						price.TierEnd = &end
					}
				}

				prices = append(prices, price)
			}
		}
	}

	return prices, nil
}

// AWSOfferFile represents the structure of AWS pricing offer files
type AWSOfferFile struct {
	FormatVersion   string                 `json:"formatVersion"`
	Disclaimer      string                 `json:"disclaimer"`
	OfferCode       string                 `json:"offerCode"`
	Version         string                 `json:"version"`
	PublicationDate string                 `json:"publicationDate"`
	Products        map[string]AWSProduct  `json:"products"`
	Terms           AWSTerms               `json:"terms"`
}

// AWSProduct represents an AWS product
type AWSProduct struct {
	SKU           string            `json:"sku"`
	ProductFamily string            `json:"productFamily"`
	Attributes    map[string]string `json:"attributes"`
}

// AWSTerms contains pricing terms
type AWSTerms struct {
	OnDemand map[string]map[string]AWSTerm `json:"OnDemand"`
	Reserved map[string]map[string]AWSTerm `json:"Reserved"`
}

// AWSTerm represents a pricing term
type AWSTerm struct {
	OfferTermCode   string                     `json:"offerTermCode"`
	SKU             string                     `json:"sku"`
	EffectiveDate   string                     `json:"effectiveDate"`
	PriceDimensions map[string]AWSPriceDimension `json:"priceDimensions"`
	TermAttributes  map[string]string          `json:"termAttributes"`
}

// AWSPriceDimension represents a price dimension
type AWSPriceDimension struct {
	RateCode     string            `json:"rateCode"`
	Description  string            `json:"description"`
	BeginRange   string            `json:"beginRange"`
	EndRange     string            `json:"endRange"`
	Unit         string            `json:"unit"`
	PricePerUnit map[string]string `json:"pricePerUnit"`
	AppliesTo    []string          `json:"appliesTo"`
}

// AWSSRegionIndex represents the region index file
type AWSSRegionIndex struct {
	FormatVersion   string                  `json:"formatVersion"`
	Disclaimer      string                  `json:"disclaimer"`
	PublicationDate string                  `json:"publicationDate"`
	Regions         map[string]AWSRegionOffer `json:"regions"`
}

// AWSRegionOffer represents a region's offer
type AWSRegionOffer struct {
	RegionCode        string `json:"regionCode"`
	CurrentVersionURL string `json:"currentVersionUrl"`
}

// awsRegionToLocation maps AWS region codes to location names used in pricing
func awsRegionToLocation(region string) string {
	locations := map[string]string{
		"us-east-1":      "US East (N. Virginia)",
		"us-east-2":      "US East (Ohio)",
		"us-west-1":      "US West (N. California)",
		"us-west-2":      "US West (Oregon)",
		"eu-west-1":      "EU (Ireland)",
		"eu-west-2":      "EU (London)",
		"eu-west-3":      "EU (Paris)",
		"eu-central-1":   "EU (Frankfurt)",
		"eu-north-1":     "EU (Stockholm)",
		"eu-south-1":     "EU (Milan)",
		"ap-northeast-1": "Asia Pacific (Tokyo)",
		"ap-northeast-2": "Asia Pacific (Seoul)",
		"ap-northeast-3": "Asia Pacific (Osaka)",
		"ap-southeast-1": "Asia Pacific (Singapore)",
		"ap-southeast-2": "Asia Pacific (Sydney)",
		"ap-south-1":     "Asia Pacific (Mumbai)",
		"ap-east-1":      "Asia Pacific (Hong Kong)",
		"sa-east-1":      "South America (Sao Paulo)",
		"ca-central-1":   "Canada (Central)",
		"me-south-1":     "Middle East (Bahrain)",
		"af-south-1":     "Africa (Cape Town)",
	}

	if loc, ok := locations[region]; ok {
		return loc
	}
	return region
}

// parseFloat parses a string to float64
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
