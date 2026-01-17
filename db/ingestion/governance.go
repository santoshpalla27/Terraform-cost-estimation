// Package ingestion - Ingestion governance and validation
package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"terraform-cost/db"

	"github.com/google/uuid"
)

// IngestionContract defines requirements for service ingestion
type IngestionContract struct {
	Cloud              db.CloudProvider
	Service            string
	RequiredDimensions []string
	MinRateCount       int
}

// DefaultContracts returns the default ingestion contracts
func DefaultContracts() []IngestionContract {
	return []IngestionContract{
		{db.AWS, "AmazonEC2", []string{"instance_type", "os", "tenancy"}, 100},
		{db.AWS, "AmazonRDS", []string{"instance_type", "engine"}, 50},
		{db.AWS, "AmazonS3", []string{"storage_class"}, 10},
		{db.AWS, "AWSLambda", []string{"memory_size"}, 5},
		{db.AWS, "ElasticLoadBalancing", []string{"product_family"}, 5},
		{db.AWS, "AmazonDynamoDB", []string{"read_capacity"}, 5},
		{db.Azure, "Virtual Machines", []string{"vm_size", "os"}, 100},
		{db.Azure, "Storage", []string{"redundancy", "tier"}, 20},
		{db.GCP, "Compute Engine", []string{"machine_type"}, 100},
		{db.GCP, "Cloud Storage", []string{"storage_class"}, 10},
	}
}

// IngestionState tracks ingestion progress
type IngestionState struct {
	ID            uuid.UUID
	SnapshotID    uuid.UUID
	Provider      string
	Status        IngestionStatus
	RecordCount   int
	DimensionCount int
	Checksum      string
	ErrorMessage  string
	StartedAt     time.Time
	CompletedAt   *time.Time
}

// IngestionStatus represents the state of an ingestion
type IngestionStatus string

const (
	IngestionStarted    IngestionStatus = "started"
	IngestionInProgress IngestionStatus = "in_progress"
	IngestionCompleted  IngestionStatus = "completed"
	IngestionFailed     IngestionStatus = "failed"
)

// IngestionValidator validates ingestion against contracts
type IngestionValidator struct {
	contracts map[string]IngestionContract
}

// NewIngestionValidator creates a new validator with default contracts
func NewIngestionValidator() *IngestionValidator {
	v := &IngestionValidator{
		contracts: make(map[string]IngestionContract),
	}
	for _, c := range DefaultContracts() {
		key := fmt.Sprintf("%s:%s", c.Cloud, c.Service)
		v.contracts[key] = c
	}
	return v
}

// AddContract adds a custom contract
func (v *IngestionValidator) AddContract(contract IngestionContract) {
	key := fmt.Sprintf("%s:%s", contract.Cloud, contract.Service)
	v.contracts[key] = contract
}

// ValidationResult contains validation outcome
type ValidationResult struct {
	IsValid           bool
	ServiceResults    map[string]ServiceValidation
	TotalRates        int
	TotalDimensions   int
	MissingServices   []string
	Errors            []string
}

// ServiceValidation contains per-service validation
type ServiceValidation struct {
	Service           string
	RateCount         int
	RequiredCount     int
	HasRequiredDims   bool
	MissingDimensions []string
	IsValid           bool
}

// Validate validates ingested rates against contracts
func (v *IngestionValidator) Validate(cloud db.CloudProvider, rates []NormalizedRate) *ValidationResult {
	result := &ValidationResult{
		IsValid:        true,
		ServiceResults: make(map[string]ServiceValidation),
	}

	// Group rates by service
	byService := make(map[string][]NormalizedRate)
	for _, r := range rates {
		byService[r.RateKey.Service] = append(byService[r.RateKey.Service], r)
	}

	result.TotalRates = len(rates)

	// Collect unique dimensions
	dims := make(map[string]bool)
	for _, r := range rates {
		for k := range r.RateKey.Attributes {
			dims[k] = true
		}
	}
	result.TotalDimensions = len(dims)

	// Validate each contracted service
	for key, contract := range v.contracts {
		if contract.Cloud != cloud {
			continue
		}

		serviceRates := byService[contract.Service]
		sv := ServiceValidation{
			Service:       contract.Service,
			RateCount:     len(serviceRates),
			RequiredCount: contract.MinRateCount,
			IsValid:       true,
		}

		// Check minimum rate count
		if len(serviceRates) < contract.MinRateCount {
			sv.IsValid = false
			result.Errors = append(result.Errors, 
				fmt.Sprintf("%s: only %d rates, need %d", contract.Service, len(serviceRates), contract.MinRateCount))
		}

		// Check required dimensions
		presentDims := make(map[string]bool)
		for _, r := range serviceRates {
			for k := range r.RateKey.Attributes {
				presentDims[k] = true
			}
		}

		for _, reqDim := range contract.RequiredDimensions {
			if !presentDims[reqDim] {
				sv.MissingDimensions = append(sv.MissingDimensions, reqDim)
				sv.HasRequiredDims = false
				sv.IsValid = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: missing required dimension '%s'", contract.Service, reqDim))
			}
		}

		if len(sv.MissingDimensions) == 0 {
			sv.HasRequiredDims = true
		}

		if !sv.IsValid {
			result.IsValid = false
		}

		result.ServiceResults[key] = sv
	}

	// Check for missing services
	for _, contract := range v.contracts {
		if contract.Cloud != cloud {
			continue
		}
		if _, ok := byService[contract.Service]; !ok {
			result.MissingServices = append(result.MissingServices, contract.Service)
			result.IsValid = false
		}
	}

	return result
}

// GovernedPipeline wraps Pipeline with governance
type GovernedPipeline struct {
	*Pipeline
	validator *IngestionValidator
}

// NewGovernedPipeline creates a governed ingestion pipeline
func NewGovernedPipeline(fetcher PriceFetcher, normalizer PriceNormalizer, store db.PricingStore) *GovernedPipeline {
	return &GovernedPipeline{
		Pipeline:  NewPipeline(fetcher, normalizer, store),
		validator: NewIngestionValidator(),
	}
}

// IngestWithValidation runs ingestion with contract validation
func (p *GovernedPipeline) IngestWithValidation(ctx context.Context, region, alias string) (*db.PricingSnapshot, *ValidationResult, error) {
	// Fetch
	raw, err := p.fetcher.FetchRegion(ctx, region)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch failed: %w", err)
	}

	// Normalize
	normalized, err := p.normalizer.Normalize(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("normalization failed: %w", err)
	}

	// Validate against contracts
	validation := p.validator.Validate(p.fetcher.Cloud(), normalized)
	if !validation.IsValid {
		// Still build snapshot but mark as partial
		// In strict mode, this would fail
	}

	// Build snapshot
	snapshot, err := p.builder.BuildSnapshot(
		ctx,
		p.fetcher.Cloud(),
		region,
		alias,
		fmt.Sprintf("ingestion_pipeline_%s", p.fetcher.Cloud()),
		normalized,
	)
	if err != nil {
		return nil, validation, fmt.Errorf("snapshot build failed: %w", err)
	}

	// Only activate if validation passed
	if validation.IsValid {
		if err := p.builder.ActivateSnapshot(ctx, snapshot.ID); err != nil {
			return snapshot, validation, fmt.Errorf("activation failed: %w", err)
		}
	}

	return snapshot, validation, nil
}

// CalculateChecksum computes checksum for rates
func CalculateChecksum(rates []NormalizedRate) string {
	hasher := sha256.New()
	for _, r := range rates {
		hasher.Write([]byte(r.RateKey.Service))
		hasher.Write([]byte(r.RateKey.ProductFamily))
		hasher.Write([]byte(r.RateKey.Region))
		hasher.Write([]byte(r.Unit))
		hasher.Write([]byte(r.Price.String()))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
