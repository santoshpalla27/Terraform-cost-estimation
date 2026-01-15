// Package scanner defines the interface for infrastructure scanners.
// Scanners parse IaC configurations into RawAsset descriptors.
// NO pricing or cost logic belongs here.
package scanner

import (
	"context"

	"terraform-cost/core/types"
)

// Scanner parses infrastructure code into raw assets
type Scanner interface {
	// Name returns the scanner identifier
	Name() string

	// CanScan determines if this scanner can handle the input
	CanScan(ctx context.Context, input *types.ProjectInput) (bool, error)

	// Scan parses the input and returns raw assets
	Scan(ctx context.Context, input *types.ProjectInput) (*ScanResult, error)
}

// ScanResult contains the output of a scan operation
type ScanResult struct {
	// Assets are the parsed infrastructure resources
	Assets []types.RawAsset `json:"assets"`

	// Modules are referenced modules
	Modules []ModuleReference `json:"modules,omitempty"`

	// Variables are the resolved variables
	Variables map[string]interface{} `json:"variables,omitempty"`

	// Warnings are non-fatal issues encountered
	Warnings []ScanWarning `json:"warnings,omitempty"`

	// Errors are parsing errors
	Errors []ScanError `json:"errors,omitempty"`
}

// ModuleReference tracks module dependencies
type ModuleReference struct {
	// Source is the module source (registry, git, local)
	Source string `json:"source"`

	// Version is the module version constraint
	Version string `json:"version,omitempty"`

	// Path is the local path to the module
	Path string `json:"path"`

	// Key is the module key in the configuration
	Key string `json:"key"`
}

// ScanWarning represents a non-fatal scanning issue
type ScanWarning struct {
	// File is the file where the warning occurred
	File string `json:"file"`

	// Line is the line number
	Line int `json:"line,omitempty"`

	// Message describes the warning
	Message string `json:"message"`

	// Code is a warning code for programmatic handling
	Code string `json:"code,omitempty"`
}

// ScanError represents a scanning error
type ScanError struct {
	// File is the file where the error occurred
	File string `json:"file"`

	// Line is the line number
	Line int `json:"line,omitempty"`

	// Message describes the error
	Message string `json:"message"`

	// Code is an error code for programmatic handling
	Code string `json:"code,omitempty"`

	// Err is the underlying error
	Err error `json:"-"`
}

// Error implements the error interface
func (e ScanError) Error() string {
	return e.Message
}

// Unwrap returns the underlying error
func (e ScanError) Unwrap() error {
	return e.Err
}

// HasErrors returns true if there are any errors
func (r *ScanResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if there are any warnings
func (r *ScanResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}
