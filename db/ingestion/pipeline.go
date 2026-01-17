// Package ingestion - Pricing data ingestion pipeline
// Strictly separated from estimation: fetch → normalize → store
package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"terraform-cost/db"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// RawPrice represents a raw price record from a cloud API
type RawPrice struct {
	SKU            string
	ServiceCode    string
	ProductFamily  string
	Region         string
	Unit           string
	PricePerUnit   string
	Currency       string
	Attributes     map[string]string
	TierStart      *float64
	TierEnd        *float64
	EffectiveDate  *time.Time
}

// NormalizedRate is the output of normalization
type NormalizedRate struct {
	RateKey    db.RateKey
	Unit       string
	Price      decimal.Decimal
	Currency   string
	Confidence float64
	TierMin    *decimal.Decimal
	TierMax    *decimal.Decimal
}

// PriceFetcher fetches raw prices from a cloud API
type PriceFetcher interface {
	// Cloud returns the cloud provider
	Cloud() db.CloudProvider
	
	// FetchRegion fetches all prices for a region
	FetchRegion(ctx context.Context, region string) ([]RawPrice, error)
	
	// SupportedRegions returns supported regions
	SupportedRegions() []string
}

// PriceNormalizer converts raw prices to normalized rates
type PriceNormalizer interface {
	// Cloud returns the cloud provider
	Cloud() db.CloudProvider
	
	// Normalize converts raw prices to normalized rates
	Normalize(raw []RawPrice) ([]NormalizedRate, error)
}

// SnapshotBuilder builds and persists pricing snapshots
type SnapshotBuilder struct {
	store  db.PricingStore
}

// NewSnapshotBuilder creates a new snapshot builder
func NewSnapshotBuilder(store db.PricingStore) *SnapshotBuilder {
	return &SnapshotBuilder{store: store}
}

// BuildSnapshot creates a pricing snapshot from normalized rates
func (b *SnapshotBuilder) BuildSnapshot(
	ctx context.Context,
	cloud db.CloudProvider,
	region string,
	alias string,
	source string,
	rates []NormalizedRate,
) (*db.PricingSnapshot, error) {
	// Calculate content hash
	hash := b.calculateHash(rates)
	
	// Check if snapshot already exists
	existing, err := b.findExistingSnapshot(ctx, cloud, region, alias, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing snapshot: %w", err)
	}
	if existing != nil {
		return existing, nil // Already ingested
	}
	
	// Create new snapshot
	snapshot := &db.PricingSnapshot{
		ID:            uuid.New(),
		Cloud:         cloud,
		Region:        region,
		ProviderAlias: alias,
		Source:        source,
		FetchedAt:     time.Now(),
		ValidFrom:     time.Now(),
		Hash:          hash,
		Version:       "1.0",
		IsActive:      false,
	}
	
	// Insert snapshot
	if err := b.store.CreateSnapshot(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}
	
	// Insert rates
	for _, nr := range rates {
		// Upsert rate key
		nr.RateKey.ID = uuid.New()
		key, err := b.store.UpsertRateKey(ctx, &nr.RateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to upsert rate key: %w", err)
		}
		
		// Create rate
		rate := &db.PricingRate{
			ID:         uuid.New(),
			SnapshotID: snapshot.ID,
			RateKeyID:  key.ID,
			Unit:       nr.Unit,
			Price:      nr.Price,
			Currency:   nr.Currency,
			Confidence: nr.Confidence,
			TierMin:    nr.TierMin,
			TierMax:    nr.TierMax,
		}
		if err := b.store.CreateRate(ctx, rate); err != nil {
			return nil, fmt.Errorf("failed to create rate: %w", err)
		}
	}
	
	return snapshot, nil
}

// ActivateSnapshot activates a snapshot
func (b *SnapshotBuilder) ActivateSnapshot(ctx context.Context, snapshotID uuid.UUID) error {
	return b.store.ActivateSnapshot(ctx, snapshotID)
}

// calculateHash computes a deterministic hash of rates
func (b *SnapshotBuilder) calculateHash(rates []NormalizedRate) string {
	// Sort for determinism
	sort.Slice(rates, func(i, j int) bool {
		ki := rateKeyString(rates[i].RateKey)
		kj := rateKeyString(rates[j].RateKey)
		return ki < kj
	})
	
	hasher := sha256.New()
	for _, r := range rates {
		hasher.Write([]byte(rateKeyString(r.RateKey)))
		hasher.Write([]byte(r.Unit))
		hasher.Write([]byte(r.Price.String()))
	}
	
	return hex.EncodeToString(hasher.Sum(nil))
}

func rateKeyString(k db.RateKey) string {
	attrs := make([]string, 0, len(k.Attributes))
	for k, v := range k.Attributes {
		attrs = append(attrs, k+"="+v)
	}
	sort.Strings(attrs)
	return fmt.Sprintf("%s|%s|%s|%s|%s", k.Cloud, k.Service, k.ProductFamily, k.Region, strings.Join(attrs, ","))
}

func (b *SnapshotBuilder) findExistingSnapshot(ctx context.Context, cloud db.CloudProvider, region, alias, hash string) (*db.PricingSnapshot, error) {
	snapshots, err := b.store.ListSnapshots(ctx, cloud, region)
	if err != nil {
		return nil, err
	}
	for _, s := range snapshots {
		if s.ProviderAlias == alias && s.Hash == hash {
			return s, nil
		}
	}
	return nil, nil
}

// Pipeline orchestrates the full ingestion flow
type Pipeline struct {
	fetcher    PriceFetcher
	normalizer PriceNormalizer
	builder    *SnapshotBuilder
}

// NewPipeline creates a new ingestion pipeline
func NewPipeline(fetcher PriceFetcher, normalizer PriceNormalizer, store db.PricingStore) *Pipeline {
	return &Pipeline{
		fetcher:    fetcher,
		normalizer: normalizer,
		builder:    NewSnapshotBuilder(store),
	}
}

// IngestRegion runs the full ingestion for a region
func (p *Pipeline) IngestRegion(ctx context.Context, region, alias string) (*db.PricingSnapshot, error) {
	// Fetch
	raw, err := p.fetcher.FetchRegion(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	
	// Normalize
	normalized, err := p.normalizer.Normalize(raw)
	if err != nil {
		return nil, fmt.Errorf("normalization failed: %w", err)
	}
	
	// Build snapshot
	snapshot, err := p.builder.BuildSnapshot(
		ctx,
		p.fetcher.Cloud(),
		region,
		alias,
		"ingestion_pipeline",
		normalized,
	)
	if err != nil {
		return nil, fmt.Errorf("snapshot build failed: %w", err)
	}
	
	// Activate
	if err := p.builder.ActivateSnapshot(ctx, snapshot.ID); err != nil {
		return nil, fmt.Errorf("activation failed: %w", err)
	}
	
	return snapshot, nil
}

// IngestAllRegions ingests all supported regions
func (p *Pipeline) IngestAllRegions(ctx context.Context, alias string) ([]*db.PricingSnapshot, error) {
	var snapshots []*db.PricingSnapshot
	for _, region := range p.fetcher.SupportedRegions() {
		snapshot, err := p.IngestRegion(ctx, region, alias)
		if err != nil {
			return snapshots, fmt.Errorf("region %s: %w", region, err)
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

// NormalizeAttributes canonicalizes attribute keys and values
func NormalizeAttributes(raw map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range raw {
		// Normalize key
		key := strings.ToLower(strings.ReplaceAll(k, " ", "_"))
		// Normalize value
		val := strings.ToLower(strings.TrimSpace(v))
		result[key] = val
	}
	return result
}

// ParsePrice parses a price string to decimal
func ParsePrice(s string) (decimal.Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(s)
}

// MarshalRateKey converts a RateKey to JSON for storage
func MarshalRateKey(k db.RateKey) ([]byte, error) {
	return json.Marshal(k.Attributes)
}
