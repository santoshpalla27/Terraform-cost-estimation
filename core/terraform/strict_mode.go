// Package terraform - Strict evaluation modes
// STRICT: block on unsafe unknowns
// PERMISSIVE: allow but degrade confidence
// ESTIMATE: best-effort (current behavior)
package terraform

import (
	"errors"
	"fmt"
)

// EvaluationMode controls how strictly unknowns are handled
type EvaluationMode int

const (
	// ModeEstimate - best effort, may guess (current default)
	ModeEstimate EvaluationMode = iota

	// ModePermissive - allows unknowns but degrades confidence
	ModePermissive

	// ModeStrict - blocks estimation on unsafe unknowns
	ModeStrict
)

// String returns the mode name
func (m EvaluationMode) String() string {
	switch m {
	case ModeEstimate:
		return "estimate"
	case ModePermissive:
		return "permissive"
	case ModeStrict:
		return "strict"
	default:
		return "unknown"
	}
}

// EvaluationModeFromString parses a mode string
func EvaluationModeFromString(s string) (EvaluationMode, error) {
	switch s {
	case "estimate", "":
		return ModeEstimate, nil
	case "permissive":
		return ModePermissive, nil
	case "strict":
		return ModeStrict, nil
	default:
		return ModeEstimate, fmt.Errorf("unknown evaluation mode: %s", s)
	}
}

// StrictModeEnforcer enforces strict mode rules
type StrictModeEnforcer struct {
	mode    EvaluationMode
	errors  []StrictModeError
	blocked bool
}

// StrictModeError is an error from strict mode enforcement
type StrictModeError struct {
	Address   string
	Attribute string
	Reason    string
	Category  ErrorCategory
	Blocking  bool // Does this error block estimation?
}

// ErrorCategory classifies the type of error
type ErrorCategory int

const (
	ErrorUnknownCount              ErrorCategory = iota // count is unknown
	ErrorUnknownForEach                                  // for_each is unknown
	ErrorUnknownProvider                                 // provider config unknown
	ErrorUnknownUsage                                    // usage value unknown
	ErrorDataSourceReference                             // data source in blocking position
	ErrorCircularDependency                              // cycle detected
	ErrorMissingRate                                     // no pricing rate found
	ErrorInvalidType                                     // type mismatch
)

// String returns the category name
func (c ErrorCategory) String() string {
	switch c {
	case ErrorUnknownCount:
		return "unknown_count"
	case ErrorUnknownForEach:
		return "unknown_for_each"
	case ErrorUnknownProvider:
		return "unknown_provider"
	case ErrorUnknownUsage:
		return "unknown_usage"
	case ErrorDataSourceReference:
		return "data_source_reference"
	case ErrorCircularDependency:
		return "circular_dependency"
	case ErrorMissingRate:
		return "missing_rate"
	case ErrorInvalidType:
		return "invalid_type"
	default:
		return "unknown"
	}
}

// NewStrictModeEnforcer creates an enforcer for the given mode
func NewStrictModeEnforcer(mode EvaluationMode) *StrictModeEnforcer {
	return &StrictModeEnforcer{
		mode:    mode,
		errors:  []StrictModeError{},
		blocked: false,
	}
}

// CheckUnknownCount checks if unknown count should block
func (e *StrictModeEnforcer) CheckUnknownCount(address string, reason string) error {
	err := StrictModeError{
		Address:  address,
		Reason:   reason,
		Category: ErrorUnknownCount,
		Blocking: e.mode == ModeStrict,
	}
	e.errors = append(e.errors, err)

	if e.mode == ModeStrict {
		e.blocked = true
		return &BlockedEstimationError{
			Address: address,
			Reason:  "unknown count blocks estimation in strict mode",
		}
	}
	return nil
}

// CheckUnknownForEach checks if unknown for_each should block
func (e *StrictModeEnforcer) CheckUnknownForEach(address string, reason string) error {
	err := StrictModeError{
		Address:  address,
		Reason:   reason,
		Category: ErrorUnknownForEach,
		Blocking: e.mode == ModeStrict,
	}
	e.errors = append(e.errors, err)

	if e.mode == ModeStrict {
		e.blocked = true
		return &BlockedEstimationError{
			Address: address,
			Reason:  "unknown for_each blocks estimation in strict mode",
		}
	}
	return nil
}

// CheckUnknownProvider checks if unknown provider should block
func (e *StrictModeEnforcer) CheckUnknownProvider(address, providerRef string) error {
	err := StrictModeError{
		Address:  address,
		Reason:   fmt.Sprintf("provider %s could not be resolved", providerRef),
		Category: ErrorUnknownProvider,
		Blocking: e.mode == ModeStrict,
	}
	e.errors = append(e.errors, err)

	if e.mode == ModeStrict {
		e.blocked = true
		return &BlockedEstimationError{
			Address: address,
			Reason:  "unknown provider blocks estimation in strict mode",
		}
	}
	return nil
}

// CheckDataSourceInBlockingPosition checks if data source ref is blocking
func (e *StrictModeEnforcer) CheckDataSourceInBlockingPosition(address, attribute, dataRef string) error {
	blocking := e.mode == ModeStrict

	err := StrictModeError{
		Address:   address,
		Attribute: attribute,
		Reason:    fmt.Sprintf("data source reference %s cannot be estimated", dataRef),
		Category:  ErrorDataSourceReference,
		Blocking:  blocking,
	}
	e.errors = append(e.errors, err)

	if blocking {
		e.blocked = true
		return &BlockedEstimationError{
			Address: address,
			Reason:  fmt.Sprintf("data source %s in blocking position", dataRef),
		}
	}
	return nil
}

// CheckMissingRate checks if missing rate should block
func (e *StrictModeEnforcer) CheckMissingRate(address, rateKey string) error {
	// Missing rates always block in strict mode
	blocking := e.mode == ModeStrict

	err := StrictModeError{
		Address:  address,
		Reason:   fmt.Sprintf("no pricing rate found for %s", rateKey),
		Category: ErrorMissingRate,
		Blocking: blocking,
	}
	e.errors = append(e.errors, err)

	if blocking {
		e.blocked = true
		return &BlockedEstimationError{
			Address: address,
			Reason:  fmt.Sprintf("missing rate %s blocks estimation", rateKey),
		}
	}
	return nil
}

// IsBlocked returns true if estimation is blocked
func (e *StrictModeEnforcer) IsBlocked() bool {
	return e.blocked
}

// GetErrors returns all errors
func (e *StrictModeEnforcer) GetErrors() []StrictModeError {
	return e.errors
}

// GetBlockingErrors returns only blocking errors
func (e *StrictModeEnforcer) GetBlockingErrors() []StrictModeError {
	var result []StrictModeError
	for _, err := range e.errors {
		if err.Blocking {
			result = append(result, err)
		}
	}
	return result
}

// BlockedEstimationError indicates estimation was blocked
type BlockedEstimationError struct {
	Address string
	Reason  string
}

func (e *BlockedEstimationError) Error() string {
	return fmt.Sprintf("estimation blocked for %s: %s", e.Address, e.Reason)
}

// IsBlockedEstimationError checks if an error is a blocked estimation error
func IsBlockedEstimationError(err error) bool {
	var blocked *BlockedEstimationError
	return errors.As(err, &blocked)
}

// ModeConfig configures behavior per mode
type ModeConfig struct {
	Mode EvaluationMode

	// What to do with unknown counts
	UnknownCountBehavior UnknownBehavior

	// What to do with unknown for_each
	UnknownForEachBehavior UnknownBehavior

	// What to do with data source references
	DataSourceBehavior DataSourceBehavior

	// Default count when unknown
	DefaultUnknownCount int
}

// UnknownBehavior defines how to handle unknowns
type UnknownBehavior int

const (
	UnknownBlock    UnknownBehavior = iota // Block estimation
	UnknownDegrade                          // Continue but degrade confidence
	UnknownDefault                          // Use default value
)

// DataSourceBehavior defines how to handle data sources
type DataSourceBehavior int

const (
	DataSourceBlock   DataSourceBehavior = iota // Block on data source refs
	DataSourceDegrade                            // Degrade confidence
	DataSourceIgnore                             // Treat as unknown value
)

// GetModeConfig returns the configuration for a mode
func GetModeConfig(mode EvaluationMode) ModeConfig {
	switch mode {
	case ModeStrict:
		return ModeConfig{
			Mode:                   ModeStrict,
			UnknownCountBehavior:   UnknownBlock,
			UnknownForEachBehavior: UnknownBlock,
			DataSourceBehavior:     DataSourceBlock,
			DefaultUnknownCount:    0,
		}
	case ModePermissive:
		return ModeConfig{
			Mode:                   ModePermissive,
			UnknownCountBehavior:   UnknownDegrade,
			UnknownForEachBehavior: UnknownDegrade,
			DataSourceBehavior:     DataSourceDegrade,
			DefaultUnknownCount:    1,
		}
	default: // ModeEstimate
		return ModeConfig{
			Mode:                   ModeEstimate,
			UnknownCountBehavior:   UnknownDefault,
			UnknownForEachBehavior: UnknownDefault,
			DataSourceBehavior:     DataSourceIgnore,
			DefaultUnknownCount:    1,
		}
	}
}
