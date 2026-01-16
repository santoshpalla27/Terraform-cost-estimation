// Package pricing - Immutable snapshot storage
// Snapshots are write-once, content-hashed, and versioned.
// No silent updates. Ever.
package pricing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ImmutabilityError is returned when attempting to violate immutability
var ErrImmutabilityViolation = errors.New("immutability violation: snapshot cannot be modified")

// ErrHashMismatch is returned when stored hash doesn't match computed hash
var ErrHashMismatch = errors.New("snapshot hash mismatch: data may be corrupted")

// ImmutableSnapshotStore is a storage layer that ENFORCES immutability.
// Once a snapshot is written, it can NEVER be overwritten.
type ImmutableSnapshotStore struct {
	mu       sync.RWMutex
	basePath string

	// In-memory index
	index map[SnapshotID]*SnapshotMetadata

	// Latest per provider:region
	latest map[string]SnapshotID
}

// SnapshotMetadata is stored alongside each snapshot
type SnapshotMetadata struct {
	ID          SnapshotID `json:"id"`
	ContentHash string     `json:"content_hash"`
	CreatedAt   time.Time  `json:"created_at"`
	EffectiveAt time.Time  `json:"effective_at"`
	Provider    string     `json:"provider"`
	Region      string     `json:"region"`
	Version     int        `json:"version"`
	Size        int64      `json:"size"`
	FilePath    string     `json:"file_path"`
}

// NewImmutableSnapshotStore creates a new immutable store
func NewImmutableSnapshotStore(basePath string) (*ImmutableSnapshotStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	store := &ImmutableSnapshotStore{
		basePath: basePath,
		index:    make(map[SnapshotID]*SnapshotMetadata),
		latest:   make(map[string]SnapshotID),
	}

	// Load existing index
	if err := store.loadIndex(); err != nil {
		// Index doesn't exist yet - that's OK
	}

	return store, nil
}

// Store writes a snapshot - FAILS if already exists
func (s *ImmutableSnapshotStore) Store(ctx context.Context, snapshot *PricingSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	if _, exists := s.index[snapshot.ID]; exists {
		return ErrImmutabilityViolation
	}

	// Compute content hash
	data, err := s.serialize(snapshot)
	if err != nil {
		return fmt.Errorf("failed to serialize snapshot: %w", err)
	}

	computedHash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(computedHash[:])

	// Verify against snapshot's hash
	if hashStr != snapshot.ContentHash.Hex() {
		return ErrHashMismatch
	}

	// Write to file
	filename := fmt.Sprintf("%s_%s.json", snapshot.ID, hashStr[:8])
	filePath := filepath.Join(s.basePath, filename)

	// Check file doesn't already exist (belt and suspenders)
	if _, err := os.Stat(filePath); err == nil {
		return ErrImmutabilityViolation
	}

	// Write file
	if err := os.WriteFile(filePath, data, 0444); err != nil { // Read-only!
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	// Update index
	meta := &SnapshotMetadata{
		ID:          snapshot.ID,
		ContentHash: hashStr,
		CreatedAt:   snapshot.CreatedAt,
		EffectiveAt: snapshot.EffectiveAt,
		Provider:    snapshot.Provider,
		Region:      snapshot.Region,
		Version:     1, // First version
		Size:        int64(len(data)),
		FilePath:    filePath,
	}

	s.index[snapshot.ID] = meta

	// Update latest
	key := snapshot.Provider + ":" + snapshot.Region
	if current, ok := s.latest[key]; !ok {
		s.latest[key] = snapshot.ID
	} else {
		// Only update if newer
		if currentMeta := s.index[current]; currentMeta.CreatedAt.Before(snapshot.CreatedAt) {
			s.latest[key] = snapshot.ID
		}
	}

	// Persist index
	return s.saveIndex()
}

// Get retrieves a snapshot by ID - verifies hash
func (s *ImmutableSnapshotStore) Get(ctx context.Context, id SnapshotID) (*PricingSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, ok := s.index[id]
	if !ok {
		return nil, ErrNoSnapshot
	}

	// Read file
	data, err := os.ReadFile(meta.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot: %w", err)
	}

	// Verify hash
	computedHash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(computedHash[:])
	if hashStr != meta.ContentHash {
		return nil, ErrHashMismatch
	}

	// Deserialize
	return s.deserialize(data)
}

// GetLatest retrieves the latest snapshot for provider/region
func (s *ImmutableSnapshotStore) GetLatest(ctx context.Context, provider, region string) (*PricingSnapshot, error) {
	s.mu.RLock()
	key := provider + ":" + region
	id, ok := s.latest[key]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrNoSnapshot
	}

	return s.Get(ctx, id)
}

// ListVersions returns all versions for a provider/region
func (s *ImmutableSnapshotStore) ListVersions(provider, region string) []*SnapshotMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SnapshotMetadata
	for _, meta := range s.index {
		if meta.Provider == provider && meta.Region == region {
			result = append(result, meta)
		}
	}
	return result
}

// VerifyIntegrity checks all stored snapshots
func (s *ImmutableSnapshotStore) VerifyIntegrity() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var corrupted []string

	for id, meta := range s.index {
		data, err := os.ReadFile(meta.FilePath)
		if err != nil {
			corrupted = append(corrupted, fmt.Sprintf("%s: file missing", id))
			continue
		}

		computedHash := sha256.Sum256(data)
		hashStr := hex.EncodeToString(computedHash[:])
		if hashStr != meta.ContentHash {
			corrupted = append(corrupted, fmt.Sprintf("%s: hash mismatch", id))
		}
	}

	return corrupted, nil
}

func (s *ImmutableSnapshotStore) serialize(snapshot *PricingSnapshot) ([]byte, error) {
	// Create a serializable version
	type serializableSnapshot struct {
		ID          SnapshotID         `json:"id"`
		ContentHash string             `json:"content_hash"`
		CreatedAt   time.Time          `json:"created_at"`
		EffectiveAt time.Time          `json:"effective_at"`
		Provider    string             `json:"provider"`
		Region      string             `json:"region"`
		Rates       []RateEntry        `json:"rates"`
		Coverage    SnapshotCoverage   `json:"coverage"`
	}

	ss := serializableSnapshot{
		ID:          snapshot.ID,
		ContentHash: snapshot.ContentHash.Hex(),
		CreatedAt:   snapshot.CreatedAt,
		EffectiveAt: snapshot.EffectiveAt,
		Provider:    snapshot.Provider,
		Region:      snapshot.Region,
		Rates:       snapshot.Rates(),
		Coverage:    snapshot.Coverage,
	}

	return json.MarshalIndent(ss, "", "  ")
}

func (s *ImmutableSnapshotStore) deserialize(data []byte) (*PricingSnapshot, error) {
	type serializableSnapshot struct {
		ID          SnapshotID         `json:"id"`
		ContentHash string             `json:"content_hash"`
		CreatedAt   time.Time          `json:"created_at"`
		EffectiveAt time.Time          `json:"effective_at"`
		Provider    string             `json:"provider"`
		Region      string             `json:"region"`
		Rates       []RateEntry        `json:"rates"`
		Coverage    SnapshotCoverage   `json:"coverage"`
	}

	var ss serializableSnapshot
	if err := json.Unmarshal(data, &ss); err != nil {
		return nil, err
	}

	// Rebuild snapshot using builder
	builder := NewSnapshotBuilder(ss.Provider, ss.Region).
		WithEffectiveAt(ss.EffectiveAt)

	for _, rate := range ss.Rates {
		builder.AddRate(rate.Key, rate.Price, rate.Unit, rate.Currency)
	}

	snapshot := builder.Build()
	return snapshot, nil
}

func (s *ImmutableSnapshotStore) loadIndex() error {
	indexPath := filepath.Join(s.basePath, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	type indexFile struct {
		Snapshots map[SnapshotID]*SnapshotMetadata `json:"snapshots"`
		Latest    map[string]SnapshotID            `json:"latest"`
	}

	var idx indexFile
	if err := json.Unmarshal(data, &idx); err != nil {
		return err
	}

	s.index = idx.Snapshots
	s.latest = idx.Latest
	return nil
}

func (s *ImmutableSnapshotStore) saveIndex() error {
	indexPath := filepath.Join(s.basePath, "index.json")

	type indexFile struct {
		Snapshots map[SnapshotID]*SnapshotMetadata `json:"snapshots"`
		Latest    map[string]SnapshotID            `json:"latest"`
		UpdatedAt time.Time                        `json:"updated_at"`
	}

	idx := indexFile{
		Snapshots: s.index,
		Latest:    s.latest,
		UpdatedAt: time.Now().UTC(),
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}

	// Write atomically using temp file
	tempPath := indexPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tempPath, indexPath)
}
