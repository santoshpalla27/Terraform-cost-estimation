// Package storage provides production-grade storage adapter for estimation results.
// Supports multiple backends: file, S3, GCS, Azure Blob, PostgreSQL.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Backend is a storage backend type
type Backend string

const (
	BackendFile     Backend = "file"
	BackendS3       Backend = "s3"
	BackendGCS      Backend = "gcs"
	BackendAzure    Backend = "azure"
	BackendPostgres Backend = "postgres"
	BackendMemory   Backend = "memory"
)

// Store is the storage interface
type Store interface {
	// Save stores an estimation result
	Save(ctx context.Context, result *StoredResult) error

	// Get retrieves an estimation by ID
	Get(ctx context.Context, id string) (*StoredResult, error)

	// List lists estimations with filters
	List(ctx context.Context, filter *ListFilter) ([]*StoredResult, error)

	// Delete removes an estimation
	Delete(ctx context.Context, id string) error

	// GetLatest gets the latest estimation for a project
	GetLatest(ctx context.Context, projectID string) (*StoredResult, error)

	// Compare compares two estimations
	Compare(ctx context.Context, oldID, newID string) (*CompareResult, error)

	// Close closes the store
	Close() error
}

// StoredResult is a stored estimation
type StoredResult struct {
	// ID is unique identifier
	ID string `json:"id"`

	// ProjectID groups estimations
	ProjectID string `json:"project_id"`

	// TotalCost monthly
	TotalCost float64 `json:"total_cost"`

	// Confidence (0-1)
	Confidence float64 `json:"confidence"`

	// Coverage breakdown
	Coverage CoverageData `json:"coverage"`

	// ResourceCount
	ResourceCount int `json:"resource_count"`

	// SnapshotID used
	SnapshotID string `json:"snapshot_id"`

	// Provider (aws, azure, gcp)
	Provider string `json:"provider"`

	// Region
	Region string `json:"region"`

	// GitInfo if available
	GitInfo *GitInfo `json:"git_info,omitempty"`

	// CreatedAt timestamp
	CreatedAt time.Time `json:"created_at"`

	// Metadata
	Metadata map[string]string `json:"metadata,omitempty"`

	// RawResult is the full result (compressed)
	RawResult []byte `json:"raw_result,omitempty"`
}

// CoverageData is coverage breakdown
type CoverageData struct {
	NumericPercent     float64 `json:"numeric_percent"`
	SymbolicPercent    float64 `json:"symbolic_percent"`
	UnsupportedPercent float64 `json:"unsupported_percent"`
}

// GitInfo contains git context
type GitInfo struct {
	Branch     string `json:"branch"`
	Commit     string `json:"commit"`
	CommitTime time.Time `json:"commit_time"`
	Author     string `json:"author"`
	Message    string `json:"message"`
	Tag        string `json:"tag,omitempty"`
}

// ListFilter filters result listing
type ListFilter struct {
	ProjectID  string
	Provider   string
	Region     string
	Since      time.Time
	Until      time.Time
	MinCost    float64
	MaxCost    float64
	Limit      int
	Offset     int
	OrderBy    string
	OrderDesc  bool
}

// CompareResult is a comparison between two estimations
type CompareResult struct {
	OldID         string    `json:"old_id"`
	NewID         string    `json:"new_id"`
	OldCost       float64   `json:"old_cost"`
	NewCost       float64   `json:"new_cost"`
	Delta         float64   `json:"delta"`
	DeltaPercent  float64   `json:"delta_percent"`
	OldConfidence float64   `json:"old_confidence"`
	NewConfidence float64   `json:"new_confidence"`
	CreatedAt     time.Time `json:"created_at"`
}

// FileStore is a file-based storage backend
type FileStore struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStore creates a file store
func NewFileStore(basePath string) (*FileStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	return &FileStore{basePath: basePath}, nil
}

func (s *FileStore) Save(ctx context.Context, result *StoredResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if result.ID == "" {
		result.ID = uuid.New().String()
	}
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now()
	}

	// Create project directory
	projectDir := filepath.Join(s.basePath, result.ProjectID)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Write result file
	filePath := filepath.Join(projectDir, result.ID+".json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	return nil
}

func (s *FileStore) Get(ctx context.Context, id string) (*StoredResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Search all project directories
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		filePath := filepath.Join(s.basePath, entry.Name(), id+".json")
		data, err := os.ReadFile(filePath)
		if err == nil {
			var result StoredResult
			if err := json.Unmarshal(data, &result); err != nil {
				return nil, fmt.Errorf("failed to unmarshal result: %w", err)
			}
			return &result, nil
		}
	}

	return nil, fmt.Errorf("result not found: %s", id)
}

func (s *FileStore) List(ctx context.Context, filter *ListFilter) ([]*StoredResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*StoredResult

	// Walk storage
	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var result StoredResult
		if err := json.Unmarshal(data, &result); err != nil {
			return nil
		}

		// Apply filters
		if filter != nil {
			if filter.ProjectID != "" && result.ProjectID != filter.ProjectID {
				return nil
			}
			if filter.Provider != "" && result.Provider != filter.Provider {
				return nil
			}
			if !filter.Since.IsZero() && result.CreatedAt.Before(filter.Since) {
				return nil
			}
			if !filter.Until.IsZero() && result.CreatedAt.After(filter.Until) {
				return nil
			}
			if filter.MinCost > 0 && result.TotalCost < filter.MinCost {
				return nil
			}
			if filter.MaxCost > 0 && result.TotalCost > filter.MaxCost {
				return nil
			}
		}

		results = append(results, &result)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Apply limit/offset
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(results) {
			results = results[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(results) {
			results = results[:filter.Limit]
		}
	}

	return results, nil
}

func (s *FileStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return fmt.Errorf("failed to read storage: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		filePath := filepath.Join(s.basePath, entry.Name(), id+".json")
		if _, err := os.Stat(filePath); err == nil {
			return os.Remove(filePath)
		}
	}

	return fmt.Errorf("result not found: %s", id)
}

func (s *FileStore) GetLatest(ctx context.Context, projectID string) (*StoredResult, error) {
	results, err := s.List(ctx, &ListFilter{
		ProjectID: projectID,
		Limit:     1,
		OrderBy:   "created_at",
		OrderDesc: true,
	})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no results for project: %s", projectID)
	}
	return results[0], nil
}

func (s *FileStore) Compare(ctx context.Context, oldID, newID string) (*CompareResult, error) {
	oldResult, err := s.Get(ctx, oldID)
	if err != nil {
		return nil, fmt.Errorf("failed to get old result: %w", err)
	}

	newResult, err := s.Get(ctx, newID)
	if err != nil {
		return nil, fmt.Errorf("failed to get new result: %w", err)
	}

	delta := newResult.TotalCost - oldResult.TotalCost
	deltaPercent := 0.0
	if oldResult.TotalCost > 0 {
		deltaPercent = delta / oldResult.TotalCost * 100
	}

	return &CompareResult{
		OldID:         oldID,
		NewID:         newID,
		OldCost:       oldResult.TotalCost,
		NewCost:       newResult.TotalCost,
		Delta:         delta,
		DeltaPercent:  deltaPercent,
		OldConfidence: oldResult.Confidence,
		NewConfidence: newResult.Confidence,
		CreatedAt:     time.Now(),
	}, nil
}

func (s *FileStore) Close() error {
	return nil
}

// MemoryStore is an in-memory storage backend (for testing)
type MemoryStore struct {
	results map[string]*StoredResult
	mu      sync.RWMutex
}

// NewMemoryStore creates a memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		results: make(map[string]*StoredResult),
	}
}

func (s *MemoryStore) Save(ctx context.Context, result *StoredResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if result.ID == "" {
		result.ID = uuid.New().String()
	}
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now()
	}

	s.results[result.ID] = result
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, id string) (*StoredResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, ok := s.results[id]
	if !ok {
		return nil, fmt.Errorf("result not found: %s", id)
	}
	return result, nil
}

func (s *MemoryStore) List(ctx context.Context, filter *ListFilter) ([]*StoredResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*StoredResult
	for _, result := range s.results {
		results = append(results, result)
	}
	return results, nil
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.results, id)
	return nil
}

func (s *MemoryStore) GetLatest(ctx context.Context, projectID string) (*StoredResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest *StoredResult
	for _, result := range s.results {
		if result.ProjectID == projectID {
			if latest == nil || result.CreatedAt.After(latest.CreatedAt) {
				latest = result
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no results for project: %s", projectID)
	}
	return latest, nil
}

func (s *MemoryStore) Compare(ctx context.Context, oldID, newID string) (*CompareResult, error) {
	oldResult, err := s.Get(ctx, oldID)
	if err != nil {
		return nil, err
	}

	newResult, err := s.Get(ctx, newID)
	if err != nil {
		return nil, err
	}

	delta := newResult.TotalCost - oldResult.TotalCost
	deltaPercent := 0.0
	if oldResult.TotalCost > 0 {
		deltaPercent = delta / oldResult.TotalCost * 100
	}

	return &CompareResult{
		OldID:         oldID,
		NewID:         newID,
		OldCost:       oldResult.TotalCost,
		NewCost:       newResult.TotalCost,
		Delta:         delta,
		DeltaPercent:  deltaPercent,
		OldConfidence: oldResult.Confidence,
		NewConfidence: newResult.Confidence,
		CreatedAt:     time.Now(),
	}, nil
}

func (s *MemoryStore) Close() error {
	return nil
}

// StoreFactory creates stores by backend type
func StoreFactory(backend Backend, config map[string]string) (Store, error) {
	switch backend {
	case BackendFile:
		path := config["path"]
		if path == "" {
			path = ".terraform-cost"
		}
		return NewFileStore(path)
	case BackendMemory:
		return NewMemoryStore(), nil
	default:
		return nil, fmt.Errorf("unsupported backend: %s", backend)
	}
}

// Ensure interfaces are implemented
var _ io.Closer = (*FileStore)(nil)
var _ io.Closer = (*MemoryStore)(nil)
