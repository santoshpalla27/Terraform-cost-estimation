// Package ingestion - Streaming pricing pipeline for low-memory environments
// Designed to run on servers with 4-8GB RAM
package ingestion

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"terraform-cost/db"

	"github.com/google/uuid"
)

// StreamingConfig configures the streaming pipeline for low-memory environments
type StreamingConfig struct {
	// BatchSize is the number of prices to process before writing to disk
	// Lower = less memory, slower. Higher = more memory, faster.
	// Default: 10000 (good for 4GB RAM)
	BatchSize int

	// MaxMemoryMB is the soft memory limit in megabytes
	// Pipeline will pause and flush when approaching this limit
	// Default: 2048 (2GB, safe for 4GB server)
	MaxMemoryMB int

	// WorkDir is where temporary files are stored during processing
	// Default: system temp directory
	WorkDir string

	// ConcurrentFetches is the number of parallel service fetches
	// Lower = less memory, slower. Default: 2 for low-memory
	ConcurrentFetches int

	// EnableCheckpointing allows resuming interrupted ingestion
	EnableCheckpointing bool

	// GCInterval is how often to force garbage collection (in batches)
	// Default: 5 (every 5 batches)
	GCInterval int
}

// DefaultStreamingConfig returns configuration safe for 4GB RAM servers
func DefaultStreamingConfig() *StreamingConfig {
	return &StreamingConfig{
		BatchSize:           10000,
		MaxMemoryMB:         2048,
		WorkDir:             os.TempDir(),
		ConcurrentFetches:   2,
		EnableCheckpointing: true,
		GCInterval:          5,
	}
}

// LowMemoryConfig returns configuration for minimal memory usage (4GB)
func LowMemoryConfig() *StreamingConfig {
	return &StreamingConfig{
		BatchSize:           5000,
		MaxMemoryMB:         1024,
		WorkDir:             os.TempDir(),
		ConcurrentFetches:   1,
		EnableCheckpointing: true,
		GCInterval:          3,
	}
}

// HighMemoryConfig returns configuration for 16GB+ servers
func HighMemoryConfig() *StreamingConfig {
	return &StreamingConfig{
		BatchSize:           50000,
		MaxMemoryMB:         8192,
		WorkDir:             os.TempDir(),
		ConcurrentFetches:   4,
		EnableCheckpointing: true,
		GCInterval:          10,
	}
}

// StreamingLifecycle manages memory-efficient ingestion
type StreamingLifecycle struct {
	mu          sync.Mutex
	config      *StreamingConfig
	lcConfig    *LifecycleConfig
	fetcher     PriceFetcher
	normalizer  PriceNormalizer
	store       db.PricingStore
	
	// Progress tracking
	totalFetched    int
	totalNormalized int
	totalWritten    int
	batchCount      int
	
	// Temporary storage
	tempFiles   []string
	checkpoint  *IngestionCheckpoint
}

// IngestionCheckpoint tracks progress for resumable ingestion
type IngestionCheckpoint struct {
	Provider       db.CloudProvider `json:"provider"`
	Region         string           `json:"region"`
	StartedAt      time.Time        `json:"started_at"`
	CompletedServices []string      `json:"completed_services"`
	TotalPrices    int              `json:"total_prices"`
	TempFiles      []string         `json:"temp_files"`
}

// NewStreamingLifecycle creates a memory-efficient lifecycle
func NewStreamingLifecycle(
	fetcher PriceFetcher,
	normalizer PriceNormalizer,
	store db.PricingStore,
	streamConfig *StreamingConfig,
) *StreamingLifecycle {
	if streamConfig == nil {
		streamConfig = DefaultStreamingConfig()
	}

	return &StreamingLifecycle{
		config:     streamConfig,
		fetcher:    fetcher,
		normalizer: normalizer,
		store:      store,
	}
}

// Execute runs the streaming ingestion pipeline
func (s *StreamingLifecycle) Execute(ctx context.Context, config *LifecycleConfig) (*LifecycleResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config == nil {
		config = DefaultLifecycleConfig()
	}
	s.lcConfig = config

	startTime := time.Now()

	// Check for existing checkpoint
	if s.config.EnableCheckpointing {
		if err := s.loadCheckpoint(); err == nil {
			fmt.Println("Resuming from checkpoint...")
		}
	}

	// Apply timeout
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	fmt.Printf("Streaming config: batch=%d, maxMem=%dMB, concurrency=%d\n",
		s.config.BatchSize, s.config.MaxMemoryMB, s.config.ConcurrentFetches)

	// Phase 1: Stream fetch and normalize to temp files
	if err := s.streamFetchAndNormalize(ctx); err != nil {
		s.cleanup()
		return s.fail(err, startTime)
	}

	// Phase 2: Merge and validate
	allRates, err := s.mergeAndValidate(ctx)
	if err != nil {
		s.cleanup()
		return s.fail(err, startTime)
	}

	// Phase 3: Backup
	backupPath, err := s.writeBackup(allRates)
	if err != nil {
		s.cleanup()
		return s.fail(fmt.Errorf("backup failed: %w", err), startTime)
	}

	// Phase 4: Commit (if not dry-run)
	var snapshotID *uuid.UUID
	if !config.DryRun {
		sid, err := s.streamCommit(ctx, allRates)
		if err != nil {
			s.cleanup()
			return s.fail(fmt.Errorf("commit failed: %w", err), startTime)
		}
		snapshotID = &sid
	}

	// Cleanup
	s.cleanup()
	s.deleteCheckpoint()

	return &LifecycleResult{
		Success:         true,
		Phase:           PhaseActive,
		Message:         "streaming ingestion complete",
		Duration:        time.Since(startTime),
		SnapshotID:      snapshotID,
		BackupPath:      backupPath,
		ContentHash:     calculateHash(allRates),
		RawCount:        s.totalFetched,
		NormalizedCount: len(allRates),
	}, nil
}

// streamFetchAndNormalize fetches pricing in batches and writes to temp files
func (s *StreamingLifecycle) streamFetchAndNormalize(ctx context.Context) error {
	fmt.Println("\nPhase 1: Streaming fetch and normalize...")

	// Get services to fetch
	services := s.fetcher.SupportedServices()

	// Process services with limited concurrency
	sem := make(chan struct{}, s.config.ConcurrentFetches)
	errChan := make(chan error, len(services))
	var wg sync.WaitGroup

	for _, service := range services {
		// Skip if already completed (checkpoint)
		if s.isServiceCompleted(service) {
			fmt.Printf("  Skipping %s (already completed)\n", service)
			continue
		}

		wg.Add(1)
		go func(svc string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if err := s.streamService(ctx, svc); err != nil {
				errChan <- fmt.Errorf("service %s: %w", svc, err)
			}
		}(service)
	}

	wg.Wait()
	close(errChan)

	// Collect errors (allow partial success)
	for err := range errChan {
		fmt.Printf("Warning: %v\n", err)
	}

	return nil
}

// streamService fetches and normalizes a single service in batches
func (s *StreamingLifecycle) streamService(ctx context.Context, service string) error {
	// Create temp file for this service
	tempFile := filepath.Join(s.config.WorkDir, fmt.Sprintf("pricing_%s_%s_%d.jsonl.gz",
		s.lcConfig.Provider, service, time.Now().UnixNano()))

	f, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()

	writer := bufio.NewWriter(gzw)

	// Fetch raw prices
	rawPrices, err := s.fetchServicePricing(ctx, service)
	if err != nil {
		return err
	}

	// Process in batches
	batchNum := 0
	for i := 0; i < len(rawPrices); i += s.config.BatchSize {
		end := i + s.config.BatchSize
		if end > len(rawPrices) {
			end = len(rawPrices)
		}

		batch := rawPrices[i:end]

		// Normalize batch
		normalized, err := s.normalizer.Normalize(batch)
		if err != nil {
			continue
		}

		// Write to temp file (JSON Lines format)
		for _, rate := range normalized {
			data, err := json.Marshal(rate)
			if err != nil {
				continue
			}
			writer.Write(data)
			writer.WriteString("\n")
			s.totalNormalized++
		}

		s.totalFetched += len(batch)
		batchNum++

		// Memory management
		if batchNum%s.config.GCInterval == 0 {
			s.checkMemoryAndGC()
		}
	}

	writer.Flush()

	s.mu.Lock()
	s.tempFiles = append(s.tempFiles, tempFile)
	s.markServiceCompleted(service)
	s.mu.Unlock()

	fmt.Printf("  ✓ %s: %d prices\n", service, s.totalFetched)

	return nil
}

// fetchServicePricing fetches pricing for a service (uses streaming internally)
func (s *StreamingLifecycle) fetchServicePricing(ctx context.Context, service string) ([]RawPrice, error) {
	// For now, delegate to fetcher
	// In future, implement streaming JSON parsing for very large responses
	return s.fetcher.FetchRegion(ctx, s.lcConfig.Region)
}

// mergeAndValidate reads temp files and validates
func (s *StreamingLifecycle) mergeAndValidate(ctx context.Context) ([]NormalizedRate, error) {
	fmt.Println("\nPhase 2: Merge and validate...")

	var allRates []NormalizedRate

	for _, tempFile := range s.tempFiles {
		rates, err := s.readTempFile(tempFile)
		if err != nil {
			fmt.Printf("Warning: failed to read %s: %v\n", tempFile, err)
			continue
		}
		allRates = append(allRates, rates...)
	}

	fmt.Printf("  Total rates: %d\n", len(allRates))

	// Validate
	validator := NewIngestionValidator()
	validator.SetMinCoveragePercent(s.lcConfig.MinCoverage)

	if err := validator.ValidateAll(allRates, 0); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	fmt.Println("  ✓ Validation passed")

	return allRates, nil
}

// readTempFile reads a gzipped JSON Lines temp file
func (s *StreamingLifecycle) readTempFile(path string) ([]NormalizedRate, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	var rates []NormalizedRate
	scanner := bufio.NewScanner(gzr)
	
	// Increase buffer size for large lines
	buf := make([]byte, 1024*1024) // 1MB buffer
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		var rate NormalizedRate
		if err := json.Unmarshal(scanner.Bytes(), &rate); err != nil {
			continue
		}
		rates = append(rates, rate)
	}

	return rates, scanner.Err()
}

// writeBackup writes the final backup
func (s *StreamingLifecycle) writeBackup(rates []NormalizedRate) (string, error) {
	fmt.Println("\nPhase 3: Write backup...")

	backup := &SnapshotBackup{
		Provider:      s.lcConfig.Provider,
		Region:        s.lcConfig.Region,
		Alias:         s.lcConfig.Alias,
		Timestamp:     time.Now(),
		ContentHash:   calculateHash(rates),
		RateCount:     len(rates),
		SchemaVersion: "1.0",
		Rates:         rates,
	}

	backupMgr := NewBackupManager()
	path, err := backupMgr.WriteBackup(s.lcConfig.BackupDir, backup)
	if err != nil {
		return "", err
	}

	fmt.Printf("  ✓ Backup written: %s\n", path)
	return path, nil
}

// streamCommit commits rates in batches to reduce memory
func (s *StreamingLifecycle) streamCommit(ctx context.Context, rates []NormalizedRate) (uuid.UUID, error) {
	fmt.Println("\nPhase 4: Stream commit...")

	snapshotID := uuid.New()
	snapshot := &db.PricingSnapshot{
		ID:            snapshotID,
		Cloud:         s.lcConfig.Provider,
		Region:        s.lcConfig.Region,
		ProviderAlias: s.lcConfig.Alias,
		Source:        "streaming_ingestion",
		FetchedAt:     time.Now(),
		ValidFrom:     time.Now(),
		Hash:          calculateHash(rates),
		Version:       "1.0",
		IsActive:      false,
	}

	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	if err = tx.CreateSnapshot(ctx, snapshot); err != nil {
		return uuid.Nil, err
	}

	// Commit in batches
	batchSize := s.config.BatchSize
	for i := 0; i < len(rates); i += batchSize {
		end := i + batchSize
		if end > len(rates) {
			end = len(rates)
		}

		for _, nr := range rates[i:end] {
			nr.RateKey.ID = uuid.New()
			key, err := tx.UpsertRateKey(ctx, &nr.RateKey)
			if err != nil {
				return uuid.Nil, err
			}

			rate := &db.PricingRate{
				ID:         uuid.New(),
				SnapshotID: snapshotID,
				RateKeyID:  key.ID,
				Unit:       nr.Unit,
				Price:      nr.Price,
				Currency:   nr.Currency,
				Confidence: nr.Confidence,
				TierMin:    nr.TierMin,
				TierMax:    nr.TierMax,
			}
			if err = tx.CreateRate(ctx, rate); err != nil {
				return uuid.Nil, err
			}
		}

		s.totalWritten += (end - i)
		fmt.Printf("  Committed %d/%d rates\n", s.totalWritten, len(rates))

		// GC between batches
		if (i/batchSize)%s.config.GCInterval == 0 {
			s.checkMemoryAndGC()
		}
	}

	if err = tx.ActivateSnapshot(ctx, snapshotID); err != nil {
		return uuid.Nil, err
	}

	if err = tx.Commit(); err != nil {
		return uuid.Nil, err
	}
	committed = true

	fmt.Printf("  ✓ Snapshot activated: %s\n", snapshotID)
	return snapshotID, nil
}

// checkMemoryAndGC checks memory usage and triggers GC if needed
func (s *StreamingLifecycle) checkMemoryAndGC() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	usedMB := m.Alloc / 1024 / 1024
	if int(usedMB) > s.config.MaxMemoryMB*80/100 { // 80% threshold
		fmt.Printf("  Memory: %dMB (triggering GC)\n", usedMB)
		runtime.GC()
	}
}

// cleanup removes temp files
func (s *StreamingLifecycle) cleanup() {
	for _, f := range s.tempFiles {
		os.Remove(f)
	}
	s.tempFiles = nil
}

// Checkpoint management
func (s *StreamingLifecycle) loadCheckpoint() error {
	path := s.checkpointPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.checkpoint)
}

func (s *StreamingLifecycle) saveCheckpoint() error {
	if s.checkpoint == nil {
		s.checkpoint = &IngestionCheckpoint{
			Provider:  s.lcConfig.Provider,
			Region:    s.lcConfig.Region,
			StartedAt: time.Now(),
		}
	}
	s.checkpoint.TempFiles = s.tempFiles
	s.checkpoint.TotalPrices = s.totalFetched

	data, err := json.Marshal(s.checkpoint)
	if err != nil {
		return err
	}
	return os.WriteFile(s.checkpointPath(), data, 0644)
}

func (s *StreamingLifecycle) deleteCheckpoint() {
	os.Remove(s.checkpointPath())
}

func (s *StreamingLifecycle) checkpointPath() string {
	return filepath.Join(s.config.WorkDir, fmt.Sprintf("checkpoint_%s_%s.json",
		s.lcConfig.Provider, s.lcConfig.Region))
}

func (s *StreamingLifecycle) isServiceCompleted(service string) bool {
	if s.checkpoint == nil {
		return false
	}
	for _, completed := range s.checkpoint.CompletedServices {
		if completed == service {
			return true
		}
	}
	return false
}

func (s *StreamingLifecycle) markServiceCompleted(service string) {
	if s.checkpoint == nil {
		s.checkpoint = &IngestionCheckpoint{}
	}
	s.checkpoint.CompletedServices = append(s.checkpoint.CompletedServices, service)
	s.saveCheckpoint()
}

func (s *StreamingLifecycle) fail(err error, startTime time.Time) (*LifecycleResult, error) {
	return &LifecycleResult{
		Success:         false,
		Phase:           PhaseFailed,
		Error:           err.Error(),
		Duration:        time.Since(startTime),
		RawCount:        s.totalFetched,
		NormalizedCount: s.totalNormalized,
	}, nil
}
