// Package pricing - Mandatory snapshot enforcement
// No cost can be computed without a valid, verified snapshot.
package pricing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"terraform-cost/core/determinism"
)

// ErrNoSnapshot is returned when no pricing snapshot is available
var ErrNoSnapshot = errors.New("pricing snapshot required but not provided")

// ErrSnapshotInvalid is returned when snapshot fails verification
var ErrSnapshotInvalid = errors.New("pricing snapshot failed integrity verification")

// ErrSnapshotExpired is returned when snapshot is too old
var ErrSnapshotExpired = errors.New("pricing snapshot has expired")

// ErrRateNotFound is returned when a rate is not in the snapshot
var ErrRateNotFound = errors.New("rate not found in pricing snapshot")

// EnforcedResolver wraps a pricing resolver with mandatory snapshot enforcement.
// It is IMPOSSIBLE to get pricing without a valid snapshot.
type EnforcedResolver struct {
	store         SnapshotStore
	maxAge        time.Duration
	strictMode    bool
	mu            sync.RWMutex
	activeSnapshot *PricingSnapshot
}

// SnapshotStore provides snapshot storage
type SnapshotStore interface {
	// Get retrieves a specific snapshot by ID
	Get(ctx context.Context, id SnapshotID) (*PricingSnapshot, error)

	// GetLatest retrieves the latest snapshot for a provider/region
	// Returns error if no snapshot exists
	GetLatest(ctx context.Context, provider, region string) (*PricingSnapshot, error)

	// Store saves a snapshot
	Store(ctx context.Context, snapshot *PricingSnapshot) error
}

// EnforcedResolverConfig configures the enforced resolver
type EnforcedResolverConfig struct {
	// MaxAge is the maximum age of a snapshot before it's considered expired
	MaxAge time.Duration

	// StrictMode fails on any missing rate (vs degraded estimation)
	StrictMode bool
}

// NewEnforcedResolver creates a resolver that REQUIRES snapshots
func NewEnforcedResolver(store SnapshotStore, config EnforcedResolverConfig) *EnforcedResolver {
	maxAge := config.MaxAge
	if maxAge == 0 {
		maxAge = 24 * time.Hour // Default: 24 hours
	}
	return &EnforcedResolver{
		store:      store,
		maxAge:     maxAge,
		strictMode: config.StrictMode,
	}
}

// SnapshotRequest specifies which snapshot to use
type SnapshotRequest struct {
	// SnapshotID - if provided, use this specific snapshot
	SnapshotID SnapshotID

	// Otherwise, find latest for provider/region
	Provider string
	Region   string

	// AllowExpired allows using expired snapshots (with warning)
	AllowExpired bool
}

// ResolveResult contains the resolution result
type ResolveResult struct {
	// Snapshot used (ALWAYS set on success)
	Snapshot *PricingSnapshot

	// Rate found (nil if not found)
	Rate *RateEntry

	// Status
	Found   bool
	Reason  string
	Warning string
}

// GetSnapshot retrieves a snapshot - NEVER returns nil snapshot on success
func (r *EnforcedResolver) GetSnapshot(ctx context.Context, req SnapshotRequest) (*PricingSnapshot, error) {
	var snapshot *PricingSnapshot
	var err error

	// Try specific ID first
	if req.SnapshotID != "" {
		snapshot, err = r.store.Get(ctx, req.SnapshotID)
		if err != nil {
			return nil, fmt.Errorf("failed to get snapshot %s: %w", req.SnapshotID, err)
		}
	} else if req.Provider != "" && req.Region != "" {
		snapshot, err = r.store.GetLatest(ctx, req.Provider, req.Region)
		if err != nil {
			return nil, fmt.Errorf("no snapshot for %s/%s: %w", req.Provider, req.Region, ErrNoSnapshot)
		}
	} else {
		return nil, ErrNoSnapshot
	}

	if snapshot == nil {
		return nil, ErrNoSnapshot
	}

	// Verify integrity
	if !snapshot.Verify() {
		return nil, ErrSnapshotInvalid
	}

	// Check expiry
	if time.Since(snapshot.CreatedAt) > r.maxAge {
		if !req.AllowExpired {
			return nil, fmt.Errorf("%w: snapshot is %v old (max: %v)",
				ErrSnapshotExpired, time.Since(snapshot.CreatedAt), r.maxAge)
		}
	}

	return snapshot, nil
}

// LookupRate finds a rate in a snapshot - snapshot is REQUIRED
func (r *EnforcedResolver) LookupRate(
	snapshot *PricingSnapshot,
	resourceType, component string,
	attrs map[string]string,
) (*ResolveResult, error) {
	if snapshot == nil {
		return nil, ErrNoSnapshot
	}

	result := &ResolveResult{
		Snapshot: snapshot,
	}

	rate, found := snapshot.LookupRate(resourceType, component, attrs)
	if !found {
		result.Found = false
		result.Reason = fmt.Sprintf("no rate for %s/%s in snapshot %s", resourceType, component, snapshot.ID)

		if r.strictMode {
			return result, ErrRateNotFound
		}
		return result, nil
	}

	result.Found = true
	result.Rate = rate
	return result, nil
}

// MustHaveSnapshot ensures a snapshot exists or panics - for critical paths
func MustHaveSnapshot(snapshot *PricingSnapshot) {
	if snapshot == nil {
		panic("BUG: pricing snapshot is nil - this should never happen")
	}
}

// CostCalculation tracks a cost calculation with mandatory snapshot reference
type CostCalculation struct {
	// MANDATORY: Snapshot reference
	SnapshotID   SnapshotID
	SnapshotHash determinism.ContentHash

	// The calculation
	InstanceID  string
	Component   string
	MonthlyCost determinism.Money
	HourlyCost  determinism.Money

	// Rate used
	RateID  RateID
	RateKey RateKey

	// Formula applied
	Formula FormulaApplication

	// Confidence (reduced if rate was missing or usage unknown)
	Confidence float64

	// Degradation info
	IsDegraded    bool
	DegradedParts []DegradedPart
}

// DegradedPart describes why a calculation was degraded
type DegradedPart struct {
	Component string
	Reason    DegradationReason
	Message   string
}

// DegradationReason explains why estimation is degraded
type DegradationReason int

const (
	ReasonRateMissing DegradationReason = iota
	ReasonUsageUnknown
	ReasonResourceUnknown
	ReasonSnapshotStale
)

// String returns the reason name
func (r DegradationReason) String() string {
	switch r {
	case ReasonRateMissing:
		return "rate_missing"
	case ReasonUsageUnknown:
		return "usage_unknown"
	case ReasonResourceUnknown:
		return "resource_unknown"
	case ReasonSnapshotStale:
		return "snapshot_stale"
	default:
		return "unknown"
	}
}

// InMemorySnapshotStore is a simple in-memory store for testing
type InMemorySnapshotStore struct {
	mu        sync.RWMutex
	snapshots map[SnapshotID]*PricingSnapshot
	latest    map[string]*PricingSnapshot // key: provider:region
}

// NewInMemorySnapshotStore creates an in-memory store
func NewInMemorySnapshotStore() *InMemorySnapshotStore {
	return &InMemorySnapshotStore{
		snapshots: make(map[SnapshotID]*PricingSnapshot),
		latest:    make(map[string]*PricingSnapshot),
	}
}

// Get retrieves a snapshot by ID
func (s *InMemorySnapshotStore) Get(ctx context.Context, id SnapshotID) (*PricingSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.snapshots[id]
	if !ok {
		return nil, ErrNoSnapshot
	}
	return snap, nil
}

// GetLatest retrieves the latest snapshot for provider/region
func (s *InMemorySnapshotStore) GetLatest(ctx context.Context, provider, region string) (*PricingSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := provider + ":" + region
	snap, ok := s.latest[key]
	if !ok {
		return nil, ErrNoSnapshot
	}
	return snap, nil
}

// Store saves a snapshot
func (s *InMemorySnapshotStore) Store(ctx context.Context, snapshot *PricingSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.ID] = snapshot
	key := snapshot.Provider + ":" + snapshot.Region
	// Update latest if this is newer
	if existing, ok := s.latest[key]; !ok || snapshot.CreatedAt.After(existing.CreatedAt) {
		s.latest[key] = snapshot
	}
	return nil
}
