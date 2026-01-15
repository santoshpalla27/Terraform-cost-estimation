// Package pricing provides immutable pricing snapshots with content hashing.
package pricing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"terraform-cost/core/determinism"
)

// SnapshotID uniquely identifies a pricing snapshot
type SnapshotID string

// RateID uniquely identifies a rate within a snapshot
type RateID string

// PricingSnapshot is IMMUTABLE after creation.
// It represents a point-in-time capture of pricing data.
type PricingSnapshot struct {
	// Identity
	ID          SnapshotID              // UUID or hash-based
	ContentHash determinism.ContentHash // SHA-256 of all rates
	CreatedAt   time.Time               // When snapshot was created
	EffectiveAt time.Time               // When prices became effective
	ExpiresAt   *time.Time              // Optional expiry (for cached data)

	// Source information
	Source      PricingSource
	Region      string
	Provider    string // aws, azure, gcp

	// The actual rates (sorted for determinism)
	rates       []RateEntry
	rateIndex   map[RateKey]*RateEntry

	// Coverage information
	Coverage    SnapshotCoverage

	// Immutability flag
	sealed      bool
}

// PricingSource indicates where pricing data came from
type PricingSource int

const (
	SourceCloudAPI    PricingSource = iota // From cloud provider API
	SourceLocalCache                       // From local cache
	SourceDatabase                         // From pricing database
	SourceManual                           // Manually specified
	SourceDefault                          // Hardcoded defaults
)

// String returns the source name
func (s PricingSource) String() string {
	switch s {
	case SourceCloudAPI:
		return "cloud_api"
	case SourceLocalCache:
		return "local_cache"
	case SourceDatabase:
		return "database"
	case SourceManual:
		return "manual"
	case SourceDefault:
		return "default"
	default:
		return "unknown"
	}
}

// RateKey uniquely identifies a rate
type RateKey struct {
	ResourceType string // aws_instance
	Component    string // Compute, Storage
	UsageType    string // BoxUsage, DataTransfer
	Attributes   string // Serialized attributes (instance_type=t3.micro)
}

// String returns a deterministic string representation
func (k RateKey) String() string {
	return k.ResourceType + "/" + k.Component + "/" + k.UsageType + "/" + k.Attributes
}

// RateEntry is a single pricing rate
type RateEntry struct {
	ID          RateID
	Key         RateKey
	Price       decimal.Decimal // Price per unit
	Unit        string          // hour, GB, request
	Currency    string          // USD
	Description string
	Tiers       []RateTier // For tiered pricing
	Conditions  []RateCondition
}

// RateTier represents a tier in tiered pricing
type RateTier struct {
	StartUsage decimal.Decimal // Start of tier
	EndUsage   *decimal.Decimal // End of tier (nil = unlimited)
	Price      decimal.Decimal  // Price in this tier
}

// RateCondition is a condition that must match for the rate
type RateCondition struct {
	Attribute string
	Operator  string // eq, neq, contains, prefix
	Value     string
}

// Bytes returns deterministic bytes for hashing
func (r *RateEntry) Bytes() []byte {
	// Use JSON for deterministic serialization
	data, _ := json.Marshal(map[string]interface{}{
		"key":      r.Key.String(),
		"price":    r.Price.String(),
		"unit":     r.Unit,
		"currency": r.Currency,
	})
	return data
}

// SnapshotCoverage tracks what's included and what's missing
type SnapshotCoverage struct {
	IncludedServices []string
	ResourceTypes    int
	TotalRates       int
	MissingRates     []MissingRate
	CoveragePercent  float64 // Estimated coverage
}

// MissingRate documents an EXPLICIT missing rate
type MissingRate struct {
	ResourceType string
	Component    string
	Reason       MissingReason
	Message      string // Human-readable explanation
}

// MissingReason explains why a rate is missing
type MissingReason int

const (
	ReasonNotInAPI           MissingReason = iota // API doesn't provide this
	ReasonRegionNotSupported                       // Region not available
	ReasonServiceNotImpl                           // We haven't implemented this
	ReasonRateLimitHit                             // API rate limit
	ReasonParseError                               // Couldn't parse response
	ReasonNotApplicable                            // Resource is free
)

// String returns the reason name
func (r MissingReason) String() string {
	switch r {
	case ReasonNotInAPI:
		return "not_in_api"
	case ReasonRegionNotSupported:
		return "region_not_supported"
	case ReasonServiceNotImpl:
		return "not_implemented"
	case ReasonRateLimitHit:
		return "rate_limit"
	case ReasonParseError:
		return "parse_error"
	case ReasonNotApplicable:
		return "not_applicable"
	default:
		return "unknown"
	}
}

// SnapshotBuilder builds a pricing snapshot
type SnapshotBuilder struct {
	provider    string
	region      string
	source      PricingSource
	effectiveAt time.Time
	rates       []RateEntry
	missing     []MissingRate
	services    map[string]bool
}

// NewSnapshotBuilder creates a new builder
func NewSnapshotBuilder(provider, region string) *SnapshotBuilder {
	return &SnapshotBuilder{
		provider:    provider,
		region:      region,
		source:      SourceDefault,
		effectiveAt: time.Now().UTC(),
		services:    make(map[string]bool),
	}
}

// WithSource sets the pricing source
func (b *SnapshotBuilder) WithSource(source PricingSource) *SnapshotBuilder {
	b.source = source
	return b
}

// WithEffectiveAt sets the effective date
func (b *SnapshotBuilder) WithEffectiveAt(t time.Time) *SnapshotBuilder {
	b.effectiveAt = t
	return b
}

// AddRate adds a rate to the snapshot
func (b *SnapshotBuilder) AddRate(key RateKey, price decimal.Decimal, unit, currency string) *SnapshotBuilder {
	b.rates = append(b.rates, RateEntry{
		Key:      key,
		Price:    price,
		Unit:     unit,
		Currency: currency,
	})
	b.services[key.ResourceType] = true
	return b
}

// AddMissing documents a missing rate
func (b *SnapshotBuilder) AddMissing(resourceType, component string, reason MissingReason, message string) *SnapshotBuilder {
	b.missing = append(b.missing, MissingRate{
		ResourceType: resourceType,
		Component:    component,
		Reason:       reason,
		Message:      message,
	})
	return b
}

// Build creates an immutable snapshot
func (b *SnapshotBuilder) Build() *PricingSnapshot {
	// Sort rates for deterministic ordering
	sort.Slice(b.rates, func(i, j int) bool {
		return b.rates[i].Key.String() < b.rates[j].Key.String()
	})

	// Generate IDs for each rate
	idGen := determinism.NewIDGenerator("rate")
	for i := range b.rates {
		b.rates[i].ID = RateID(idGen.Generate(b.rates[i].Key.String()))
	}

	// Build index
	index := make(map[RateKey]*RateEntry)
	for i := range b.rates {
		index[b.rates[i].Key] = &b.rates[i]
	}

	// Collect services
	services := make([]string, 0, len(b.services))
	for svc := range b.services {
		services = append(services, svc)
	}
	sort.Strings(services)

	// Create snapshot
	snap := &PricingSnapshot{
		CreatedAt:   time.Now().UTC(),
		EffectiveAt: b.effectiveAt,
		Source:      b.source,
		Region:      b.region,
		Provider:    b.provider,
		rates:       b.rates,
		rateIndex:   index,
		Coverage: SnapshotCoverage{
			IncludedServices: services,
			ResourceTypes:    len(b.services),
			TotalRates:       len(b.rates),
			MissingRates:     b.missing,
		},
	}

	// Compute content hash
	snap.ContentHash = snap.computeHash()

	// Generate ID from hash
	snap.ID = SnapshotID(hex.EncodeToString(snap.ContentHash[:8]))

	// Seal the snapshot
	snap.sealed = true

	return snap
}

// computeHash creates a content hash of all rates
func (s *PricingSnapshot) computeHash() determinism.ContentHash {
	h := sha256.New()
	h.Write([]byte(s.Provider))
	h.Write([]byte(s.Region))
	h.Write([]byte(s.EffectiveAt.Format(time.RFC3339)))
	for _, rate := range s.rates {
		h.Write(rate.Bytes())
	}
	var hash determinism.ContentHash
	copy(hash[:], h.Sum(nil))
	return hash
}

// GetRate looks up a rate by key
func (s *PricingSnapshot) GetRate(key RateKey) (*RateEntry, bool) {
	rate, ok := s.rateIndex[key]
	return rate, ok
}

// LookupRate finds a rate by resource type and component
func (s *PricingSnapshot) LookupRate(resourceType, component string, attrs map[string]string) (*RateEntry, bool) {
	// Serialize attributes deterministically
	attrKeys := determinism.SortedKeys(attrs)
	var attrStr string
	for _, k := range attrKeys {
		if attrStr != "" {
			attrStr += ","
		}
		attrStr += k + "=" + attrs[k]
	}

	key := RateKey{
		ResourceType: resourceType,
		Component:    component,
		Attributes:   attrStr,
	}
	return s.GetRate(key)
}

// Rates returns all rates in sorted order
func (s *PricingSnapshot) Rates() []RateEntry {
	result := make([]RateEntry, len(s.rates))
	copy(result, s.rates)
	return result
}

// Verify checks content hash integrity
func (s *PricingSnapshot) Verify() bool {
	computed := s.computeHash()
	return computed == s.ContentHash
}

// CostEstimate MUST reference a snapshot
type CostEstimate struct {
	// Snapshot reference (mandatory)
	SnapshotID   SnapshotID
	SnapshotHash determinism.ContentHash // For verification

	// The estimate
	InstanceID   string
	Component    string
	MonthlyCost  determinism.Money
	HourlyCost   determinism.Money

	// Lineage (see Gap 5)
	Lineage      *CostLineage
}

// CostLineage tracks the full derivation chain (Gap 5)
type CostLineage struct {
	// What was priced
	InstanceID  string
	Component   string

	// Pricing source
	SnapshotID  SnapshotID
	RateID      RateID
	RateKey     RateKey

	// Formula applied
	Formula     FormulaApplication

	// Usage input
	Usage       UsageLineage

	// Derived costs (for aggregated)
	DerivedFrom []*CostLineage

	// Confidence
	Confidence  float64 // 0.0 - 1.0

	// Timestamp
	Timestamp   time.Time
}

// FormulaApplication tracks how cost was calculated
type FormulaApplication struct {
	Name       string            // "hourly_compute"
	Expression string            // "rate * hours * quantity"
	Inputs     map[string]string // rate=0.10, hours=730, quantity=3
	Output     string            // 219.00
}

// UsageLineage tracks usage source
type UsageLineage struct {
	Source      UsageSource
	Profile     string   // "production", "dev"
	Confidence  float64  // 0.0 - 1.0
	Assumptions []string // ["assumed 75% utilization"]
}

// UsageSource indicates where usage data came from
type UsageSource int

const (
	UsageDefault   UsageSource = iota // Hardcoded defaults
	UsageOverride                     // User provided
	UsageEstimated                    // ML/heuristic estimated
	UsageActual                       // Historical actual
)

// String returns the source name
func (s UsageSource) String() string {
	switch s {
	case UsageDefault:
		return "default"
	case UsageOverride:
		return "override"
	case UsageEstimated:
		return "estimated"
	case UsageActual:
		return "actual"
	default:
		return "unknown"
	}
}
