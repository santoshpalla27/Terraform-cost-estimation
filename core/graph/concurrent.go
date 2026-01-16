// Package graph - Concurrent graph execution
// Resources are processed in parallel respecting dependency order.
package graph

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrentExecutor executes graph nodes in parallel with dependency respect
type ConcurrentExecutor struct {
	// Max concurrent workers
	maxWorkers int

	// Execution stats
	stats *ExecutionStats

	// Error handling
	stopOnError bool
	errors      []ExecutionError

	// Progress tracking
	progress *ExecutionProgress

	mu sync.Mutex
}

// ExecutionStats tracks execution statistics
type ExecutionStats struct {
	TotalNodes       int64
	CompletedNodes   int64
	FailedNodes      int64
	SkippedNodes     int64
	StartTime        time.Time
	EndTime          time.Time
	MaxConcurrency   int
	AverageDuration  time.Duration
	nodeDurations    []time.Duration
	mu               sync.Mutex
}

// ExecutionError records an execution error
type ExecutionError struct {
	NodeID   string
	Phase    string
	Message  string
	Cause    error
	Duration time.Duration
}

// ExecutionProgress tracks live progress
type ExecutionProgress struct {
	Total      int64
	Completed  int64
	InProgress int64
	Failed     int64
	Percent    float64
	ETA        time.Duration
	mu         sync.RWMutex
}

// NodeExecutor is a function that processes a single node
type NodeExecutor func(ctx context.Context, nodeID string) error

// NewConcurrentExecutor creates a new executor
func NewConcurrentExecutor(maxWorkers int) *ConcurrentExecutor {
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	return &ConcurrentExecutor{
		maxWorkers:  maxWorkers,
		stats:       &ExecutionStats{},
		stopOnError: false,
		errors:      []ExecutionError{},
		progress:    &ExecutionProgress{},
	}
}

// SetStopOnError configures error handling
func (e *ConcurrentExecutor) SetStopOnError(stop bool) {
	e.stopOnError = stop
}

// Execute runs the executor on a graph
func (e *ConcurrentExecutor) Execute(ctx context.Context, graph *InfrastructureGraph, executor NodeExecutor) error {
	// Get topological order
	order, err := graph.TopologicalSort()
	if err != nil {
		return fmt.Errorf("failed to sort graph: %w", err)
	}

	if len(order) == 0 {
		return nil
	}

	// Initialize stats
	e.stats.TotalNodes = int64(len(order))
	e.stats.StartTime = time.Now()
	e.progress.Total = int64(len(order))

	// Group nodes by level (nodes at same level can run in parallel)
	levels := e.groupByLevel(graph, order)

	// Execute level by level
	for levelNum, level := range levels {
		if err := e.executeLevel(ctx, level, levelNum, executor); err != nil {
			if e.stopOnError {
				return err
			}
		}
	}

	e.stats.EndTime = time.Now()
	e.calculateAverageDuration()

	return nil
}

// groupByLevel groups nodes into dependency levels
func (e *ConcurrentExecutor) groupByLevel(graph *InfrastructureGraph, order []string) [][]string {
	levels := [][]string{}
	processed := make(map[string]int) // node -> level

	for _, nodeID := range order {
		// Find the maximum level of dependencies
		maxDepLevel := -1
		deps := graph.GetDependencies(nodeID)
		for _, dep := range deps {
			if level, ok := processed[dep]; ok {
				if level > maxDepLevel {
					maxDepLevel = level
				}
			}
		}

		// This node goes in the next level
		nodeLevel := maxDepLevel + 1
		processed[nodeID] = nodeLevel

		// Ensure we have enough levels
		for len(levels) <= nodeLevel {
			levels = append(levels, []string{})
		}
		levels[nodeLevel] = append(levels[nodeLevel], nodeID)
	}

	return levels
}

// executeLevel executes all nodes in a level concurrently
func (e *ConcurrentExecutor) executeLevel(ctx context.Context, nodes []string, levelNum int, executor NodeExecutor) error {
	if len(nodes) == 0 {
		return nil
	}

	// Create worker pool
	workers := e.maxWorkers
	if len(nodes) < workers {
		workers = len(nodes)
	}

	// Track concurrency
	if workers > e.stats.MaxConcurrency {
		e.stats.MaxConcurrency = workers
	}

	// Channel for work items
	work := make(chan string, len(nodes))
	for _, node := range nodes {
		work <- node
	}
	close(work)

	// Error channel
	errChan := make(chan ExecutionError, len(nodes))

	// WaitGroup for workers
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for nodeID := range work {
				select {
				case <-ctx.Done():
					return
				default:
					e.executeNode(ctx, nodeID, executor, errChan)
				}
			}
		}()
	}

	// Wait for completion
	wg.Wait()
	close(errChan)

	// Collect errors
	for err := range errChan {
		e.mu.Lock()
		e.errors = append(e.errors, err)
		e.mu.Unlock()
	}

	return nil
}

func (e *ConcurrentExecutor) executeNode(ctx context.Context, nodeID string, executor NodeExecutor, errChan chan<- ExecutionError) {
	// Update progress
	atomic.AddInt64(&e.progress.InProgress, 1)
	e.updateProgress()

	start := time.Now()
	err := executor(ctx, nodeID)
	duration := time.Since(start)

	// Record duration
	e.stats.mu.Lock()
	e.stats.nodeDurations = append(e.stats.nodeDurations, duration)
	e.stats.mu.Unlock()

	atomic.AddInt64(&e.progress.InProgress, -1)

	if err != nil {
		atomic.AddInt64(&e.stats.FailedNodes, 1)
		atomic.AddInt64(&e.progress.Failed, 1)
		errChan <- ExecutionError{
			NodeID:   nodeID,
			Message:  err.Error(),
			Cause:    err,
			Duration: duration,
		}
	} else {
		atomic.AddInt64(&e.stats.CompletedNodes, 1)
		atomic.AddInt64(&e.progress.Completed, 1)
	}

	e.updateProgress()
}

func (e *ConcurrentExecutor) updateProgress() {
	e.progress.mu.Lock()
	defer e.progress.mu.Unlock()

	if e.progress.Total > 0 {
		e.progress.Percent = float64(e.progress.Completed+e.progress.Failed) / float64(e.progress.Total) * 100
	}

	// Estimate ETA
	elapsed := time.Since(e.stats.StartTime)
	if e.progress.Completed > 0 {
		avgDuration := elapsed / time.Duration(e.progress.Completed)
		remaining := e.progress.Total - e.progress.Completed - e.progress.Failed
		e.progress.ETA = avgDuration * time.Duration(remaining)
	}
}

func (e *ConcurrentExecutor) calculateAverageDuration() {
	e.stats.mu.Lock()
	defer e.stats.mu.Unlock()

	if len(e.stats.nodeDurations) == 0 {
		return
	}

	var total time.Duration
	for _, d := range e.stats.nodeDurations {
		total += d
	}
	e.stats.AverageDuration = total / time.Duration(len(e.stats.nodeDurations))
}

// GetProgress returns current progress
func (e *ConcurrentExecutor) GetProgress() ExecutionProgress {
	e.progress.mu.RLock()
	defer e.progress.mu.RUnlock()
	return *e.progress
}

// GetStats returns execution stats
func (e *ConcurrentExecutor) GetStats() ExecutionStats {
	return *e.stats
}

// GetErrors returns all errors
func (e *ConcurrentExecutor) GetErrors() []ExecutionError {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.errors
}

// ProgressCallback is called with progress updates
type ProgressCallback func(progress ExecutionProgress)

// ExecuteWithProgress runs with progress callbacks
func (e *ConcurrentExecutor) ExecuteWithProgress(ctx context.Context, graph *InfrastructureGraph, executor NodeExecutor, callback ProgressCallback, interval time.Duration) error {
	// Start progress reporter
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				callback(e.GetProgress())
			}
		}
	}()

	// Execute
	err := e.Execute(ctx, graph, executor)
	close(done)

	// Final callback
	callback(e.GetProgress())

	return err
}
