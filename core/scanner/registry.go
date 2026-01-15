// Package scanner - Registry for scanner implementations
package scanner

import (
	"context"
	"fmt"
	"sync"

	"terraform-cost/core/types"
)

// Registry manages scanner registration and lookup
type Registry interface {
	// Register adds a scanner to the registry
	Register(scanner Scanner) error

	// GetScanner returns a scanner by name
	GetScanner(name string) (Scanner, bool)

	// GetAll returns all registered scanners
	GetAll() []Scanner

	// DetectAndScan finds the appropriate scanner and scans the input
	DetectAndScan(ctx context.Context, input *types.ProjectInput) (*ScanResult, error)
}

// DefaultRegistry is the default scanner registry implementation
type DefaultRegistry struct {
	mu       sync.RWMutex
	scanners map[string]Scanner
	order    []string // maintains registration order for priority
}

// NewRegistry creates a new scanner registry
func NewRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		scanners: make(map[string]Scanner),
		order:    make([]string, 0),
	}
}

// Register adds a scanner to the registry
func (r *DefaultRegistry) Register(scanner Scanner) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := scanner.Name()
	if _, exists := r.scanners[name]; exists {
		return fmt.Errorf("scanner already registered: %s", name)
	}

	r.scanners[name] = scanner
	r.order = append(r.order, name)
	return nil
}

// GetScanner returns a scanner by name
func (r *DefaultRegistry) GetScanner(name string) (Scanner, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	scanner, ok := r.scanners[name]
	return scanner, ok
}

// GetAll returns all registered scanners in registration order
func (r *DefaultRegistry) GetAll() []Scanner {
	r.mu.RLock()
	defer r.mu.RUnlock()

	scanners := make([]Scanner, 0, len(r.order))
	for _, name := range r.order {
		if scanner, ok := r.scanners[name]; ok {
			scanners = append(scanners, scanner)
		}
	}
	return scanners
}

// DetectAndScan finds the first scanner that can handle the input and runs it
func (r *DefaultRegistry) DetectAndScan(ctx context.Context, input *types.ProjectInput) (*ScanResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, name := range r.order {
		scanner := r.scanners[name]

		canScan, err := scanner.CanScan(ctx, input)
		if err != nil {
			continue // Skip scanners that error on detection
		}

		if canScan {
			return scanner.Scan(ctx, input)
		}
	}

	return nil, fmt.Errorf("no scanner found for input: %s", input.Path)
}

// Global default registry
var defaultRegistry = NewRegistry()

// Register adds a scanner to the default registry
func Register(scanner Scanner) error {
	return defaultRegistry.Register(scanner)
}

// GetDefault returns the default registry
func GetDefault() *DefaultRegistry {
	return defaultRegistry
}
