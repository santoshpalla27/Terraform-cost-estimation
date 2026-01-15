// Package errors provides error handling utilities.
package errors

import (
	"fmt"
)

// Type identifies the category of error
type Type string

const (
	// TypeInput indicates an input validation error
	TypeInput Type = "INPUT_ERROR"

	// TypeParsing indicates a parsing error
	TypeParsing Type = "PARSING_ERROR"

	// TypePricing indicates a pricing resolution error
	TypePricing Type = "PRICING_ERROR"

	// TypePolicy indicates a policy evaluation error
	TypePolicy Type = "POLICY_ERROR"

	// TypeConfig indicates a configuration error
	TypeConfig Type = "CONFIG_ERROR"

	// TypeNetwork indicates a network error
	TypeNetwork Type = "NETWORK_ERROR"

	// TypeInternal indicates an internal error
	TypeInternal Type = "INTERNAL_ERROR"

	// TypeNotFound indicates a resource not found error
	TypeNotFound Type = "NOT_FOUND"

	// TypeNotSupported indicates an unsupported operation
	TypeNotSupported Type = "NOT_SUPPORTED"
)

// Error represents a domain error with context
type Error struct {
	Type    Type                   `json:"type"`
	Message string                 `json:"message"`
	Cause   error                  `json:"-"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is checks if the error is of a specific type
func (e *Error) Is(t Type) bool {
	return e.Type == t
}

// WithContext adds context to the error
func (e *Error) WithContext(key string, value interface{}) *Error {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// New creates a new error
func New(errType Type, message string) *Error {
	return &Error{
		Type:    errType,
		Message: message,
	}
}

// Newf creates a new formatted error
func Newf(errType Type, format string, args ...interface{}) *Error {
	return &Error{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
	}
}

// Wrap wraps an error with context
func Wrap(errType Type, message string, cause error) *Error {
	return &Error{
		Type:    errType,
		Message: message,
		Cause:   cause,
	}
}

// Wrapf wraps an error with formatted context
func Wrapf(errType Type, cause error, format string, args ...interface{}) *Error {
	return &Error{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
		Cause:   cause,
	}
}

// IsType checks if an error is of a specific type
func IsType(err error, t Type) bool {
	if e, ok := err.(*Error); ok {
		return e.Type == t
	}
	return false
}

// Input creates an input error
func Input(message string) *Error {
	return New(TypeInput, message)
}

// Parsing creates a parsing error
func Parsing(message string, cause error) *Error {
	return Wrap(TypeParsing, message, cause)
}

// Pricing creates a pricing error
func Pricing(message string, cause error) *Error {
	return Wrap(TypePricing, message, cause)
}

// Policy creates a policy error
func Policy(message string) *Error {
	return New(TypePolicy, message)
}

// NotFound creates a not found error
func NotFound(resourceType, identifier string) *Error {
	return Newf(TypeNotFound, "%s not found: %s", resourceType, identifier)
}

// NotSupported creates a not supported error
func NotSupported(operation string) *Error {
	return Newf(TypeNotSupported, "operation not supported: %s", operation)
}

// Internal creates an internal error
func Internal(message string, cause error) *Error {
	return Wrap(TypeInternal, message, cause)
}
