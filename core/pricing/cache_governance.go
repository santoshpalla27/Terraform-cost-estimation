// Package pricing - Cache governance with TTL and schema versioning
// Stale pricing is silent failure. Governance is mandatory.
package pricing

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// CacheGovernance manages pricing cache lifecycle
type CacheGovernance struct {
	// TTL policy
	defaultTTL time.Duration
	perSourceTTL map[string]time.Duration

	// Schema versioning
	schemaVersions map[string]string

	// Provider metadata hashes
	providerHashes map[string]string

	// Cache entries with metadata
	entries map[string]*CacheEntry

	// Lock
	mu sync.RWMutex
}

// CacheEntry is a cached pricing entry with governance metadata
type CacheEntry struct {
	Key           string
	Value         interface{}
	SnapshotID    string
	SchemaVersion string
	ProviderHash  string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	AccessCount   int
	LastAccessed  time.Time
}

// IsExpired checks if the entry has expired
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// IsStale checks if the entry is stale (schema/provider changed)
func (e *CacheEntry) IsStale(currentSchema, currentProviderHash string) bool {
	if e.SchemaVersion != currentSchema {
		return true
	}
	if e.ProviderHash != currentProviderHash {
		return true
	}
	return false
}

// CachePolicy defines cache behavior
type CachePolicy struct {
	// TTL for entries
	TTL time.Duration

	// Max entries
	MaxEntries int

	// Schema version
	SchemaVersion string

	// Force refresh on schema change
	RefreshOnSchemaChange bool

	// Force refresh on provider update
	RefreshOnProviderUpdate bool
}

// DefaultCachePolicy returns the default policy
func DefaultCachePolicy() *CachePolicy {
	return &CachePolicy{
		TTL:                     24 * time.Hour,
		MaxEntries:              10000,
		SchemaVersion:           "1.0",
		RefreshOnSchemaChange:   true,
		RefreshOnProviderUpdate: true,
	}
}

// NewCacheGovernance creates a new governance instance
func NewCacheGovernance(policy *CachePolicy) *CacheGovernance {
	if policy == nil {
		policy = DefaultCachePolicy()
	}
	return &CacheGovernance{
		defaultTTL:     policy.TTL,
		perSourceTTL:   make(map[string]time.Duration),
		schemaVersions: make(map[string]string),
		providerHashes: make(map[string]string),
		entries:        make(map[string]*CacheEntry),
	}
}

// SetSourceTTL sets TTL for a specific source
func (g *CacheGovernance) SetSourceTTL(source string, ttl time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.perSourceTTL[source] = ttl
}

// SetSchemaVersion sets the schema version for a provider
func (g *CacheGovernance) SetSchemaVersion(provider, version string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.schemaVersions[provider] = version
}

// SetProviderHash sets the metadata hash for a provider
func (g *CacheGovernance) SetProviderHash(provider, hash string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.providerHashes[provider] = hash
}

// Get retrieves an entry if valid
func (g *CacheGovernance) Get(key, provider string) (interface{}, bool) {
	g.mu.RLock()
	entry, exists := g.entries[key]
	g.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Check expiration
	if entry.IsExpired() {
		g.Invalidate(key)
		return nil, false
	}

	// Check staleness
	currentSchema := g.schemaVersions[provider]
	currentHash := g.providerHashes[provider]
	if entry.IsStale(currentSchema, currentHash) {
		g.Invalidate(key)
		return nil, false
	}

	// Update access stats
	g.mu.Lock()
	entry.AccessCount++
	entry.LastAccessed = time.Now()
	g.mu.Unlock()

	return entry.Value, true
}

// Put stores an entry with metadata
func (g *CacheGovernance) Put(key string, value interface{}, provider, snapshotID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	ttl := g.defaultTTL
	if sourceTTL, ok := g.perSourceTTL[provider]; ok {
		ttl = sourceTTL
	}

	now := time.Now()
	g.entries[key] = &CacheEntry{
		Key:           key,
		Value:         value,
		SnapshotID:    snapshotID,
		SchemaVersion: g.schemaVersions[provider],
		ProviderHash:  g.providerHashes[provider],
		CreatedAt:     now,
		ExpiresAt:     now.Add(ttl),
		AccessCount:   0,
		LastAccessed:  now,
	}
}

// Invalidate removes an entry
func (g *CacheGovernance) Invalidate(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, key)
}

// InvalidateProvider invalidates all entries for a provider
func (g *CacheGovernance) InvalidateProvider(provider string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Build list of keys to remove
	toRemove := []string{}
	currentSchema := g.schemaVersions[provider]
	currentHash := g.providerHashes[provider]

	for key, entry := range g.entries {
		if entry.IsStale(currentSchema, currentHash) {
			toRemove = append(toRemove, key)
		}
	}

	for _, key := range toRemove {
		delete(g.entries, key)
	}
}

// InvalidateExpired removes all expired entries
func (g *CacheGovernance) InvalidateExpired() int {
	g.mu.Lock()
	defer g.mu.Unlock()

	count := 0
	for key, entry := range g.entries {
		if entry.IsExpired() {
			delete(g.entries, key)
			count++
		}
	}
	return count
}

// Stats returns cache statistics
func (g *CacheGovernance) Stats() *CacheStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := &CacheStats{
		TotalEntries:   len(g.entries),
		ExpiredEntries: 0,
		StaleEntries:   0,
	}

	for _, entry := range g.entries {
		if entry.IsExpired() {
			stats.ExpiredEntries++
		}
	}

	return stats
}

// CacheStats contains cache statistics
type CacheStats struct {
	TotalEntries   int
	ExpiredEntries int
	StaleEntries   int
}

// ComputeProviderHash computes a hash of provider metadata
func ComputeProviderHash(provider string, version string, lastUpdated time.Time) string {
	h := sha256.New()
	h.Write([]byte(provider))
	h.Write([]byte(version))
	h.Write([]byte(lastUpdated.Format(time.RFC3339)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// SchemaRegistry tracks schema versions
type SchemaRegistry struct {
	versions map[string]SchemaVersion
	mu       sync.RWMutex
}

// SchemaVersion describes a schema version
type SchemaVersion struct {
	Provider    string
	Version     string
	Hash        string
	FieldCount  int
	LastUpdated time.Time
}

// NewSchemaRegistry creates a new registry
func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{
		versions: make(map[string]SchemaVersion),
	}
}

// Register registers a schema version
func (r *SchemaRegistry) Register(provider, version string, fieldCount int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hash := computeSchemaHash(provider, version, fieldCount)
	r.versions[provider] = SchemaVersion{
		Provider:    provider,
		Version:     version,
		Hash:        hash,
		FieldCount:  fieldCount,
		LastUpdated: time.Now(),
	}
}

// Get returns the schema version for a provider
func (r *SchemaRegistry) Get(provider string) (SchemaVersion, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.versions[provider]
	return v, ok
}

// HasChanged checks if schema has changed
func (r *SchemaRegistry) HasChanged(provider, previousHash string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if v, ok := r.versions[provider]; ok {
		return v.Hash != previousHash
	}
	return true
}

func computeSchemaHash(provider, version string, fieldCount int) string {
	h := sha256.New()
	h.Write([]byte(provider))
	h.Write([]byte(version))
	h.Write([]byte(fmt.Sprintf("%d", fieldCount)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
