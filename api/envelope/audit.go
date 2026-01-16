// Package envelope - Envelope logging and audit
package envelope

import (
	"encoding/json"
	"fmt"
	"time"
)

// AuditEntry is a log entry for an envelope
type AuditEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	InputHash  string    `json:"input_hash"`
	Envelope   string    `json:"envelope_json"`
	RequestID  string    `json:"request_id,omitempty"`
	ClientIP   string    `json:"client_ip,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
}

// AuditLogger logs envelopes for audit and replay
type AuditLogger interface {
	Log(entry AuditEntry) error
}

// StdoutAuditLogger logs to stdout (for development)
type StdoutAuditLogger struct{}

// Log logs an audit entry to stdout
func (l *StdoutAuditLogger) Log(entry AuditEntry) error {
	data, _ := json.Marshal(entry)
	fmt.Printf("[AUDIT] %s\n", data)
	return nil
}

// CreateAuditEntry creates an audit entry from an envelope
func CreateAuditEntry(envelope *InputEnvelope, requestID, clientIP, userAgent string) AuditEntry {
	envJSON, _ := json.Marshal(envelope)
	return AuditEntry{
		Timestamp: time.Now().UTC(),
		InputHash: envelope.InputHash,
		Envelope:  string(envJSON),
		RequestID: requestID,
		ClientIP:  clientIP,
		UserAgent: userAgent,
		Success:   true,
	}
}

// MarkFailed marks the audit entry as failed
func (e *AuditEntry) MarkFailed(err error) {
	e.Success = false
	e.Error = err.Error()
}

// SetDuration sets the duration
func (e *AuditEntry) SetDuration(d time.Duration) {
	e.DurationMs = d.Milliseconds()
}
