// Package detection provides project detection interfaces.
// This package determines what type of IaC project is being analyzed.
package detection

import (
	"context"

	"terraform-cost/core/types"
)

// Detector identifies project types
type Detector interface {
	// Name returns the detector identifier
	Name() string

	// Detect determines if the input matches this detector's project type
	Detect(ctx context.Context, path string) (*types.DetectedProject, error)

	// ProjectType returns the project type this detector handles
	ProjectType() types.ProjectType

	// Priority returns the detection priority (higher = checked first)
	Priority() int
}

// Registry manages detector registration
type Registry interface {
	// Register adds a detector to the registry
	Register(detector Detector) error

	// GetDetector returns a detector by name
	GetDetector(name string) (Detector, bool)

	// GetAll returns all registered detectors
	GetAll() []Detector

	// Detect finds the first matching detector and returns its result
	Detect(ctx context.Context, path string) (*types.DetectedProject, error)

	// DetectAll runs all detectors and returns all matches
	DetectAll(ctx context.Context, path string) ([]*types.DetectedProject, error)
}

// DetectionResult contains detection output
type DetectionResult struct {
	// Project is the detected project
	Project *types.DetectedProject

	// Detector is the detector that matched
	Detector string

	// Alternatives are other possible matches
	Alternatives []*types.DetectedProject
}
