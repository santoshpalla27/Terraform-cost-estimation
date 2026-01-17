// Package webhook provides production-grade webhook adapter for integrations.
// Supports GitHub, GitLab, Bitbucket, and custom webhook targets.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Provider is a webhook provider type
type Provider string

const (
	ProviderGitHub    Provider = "github"
	ProviderGitLab    Provider = "gitlab"
	ProviderBitbucket Provider = "bitbucket"
	ProviderSlack     Provider = "slack"
	ProviderTeams     Provider = "teams"
	ProviderCustom    Provider = "custom"
)

// Config configures webhook behavior
type Config struct {
	// Provider type
	Provider Provider `json:"provider"`

	// Endpoint URL
	Endpoint string `json:"endpoint"`

	// Secret for signature verification
	Secret string `json:"secret"`

	// Headers to include
	Headers map[string]string `json:"headers"`

	// Timeout for requests
	Timeout time.Duration `json:"timeout"`

	// RetryCount for failed requests
	RetryCount int `json:"retry_count"`

	// RetryDelay between retries
	RetryDelay time.Duration `json:"retry_delay"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig(provider Provider) *Config {
	return &Config{
		Provider:   provider,
		Timeout:    30 * time.Second,
		RetryCount: 3,
		RetryDelay: 1 * time.Second,
		Headers:    make(map[string]string),
	}
}

// Adapter is the webhook adapter
type Adapter struct {
	config     *Config
	httpClient *http.Client
}

// New creates a new webhook adapter
func New(config *Config) *Adapter {
	return &Adapter{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Payload is the webhook payload
type Payload struct {
	// Event type
	Event string `json:"event"`

	// TotalCost monthly
	TotalCost float64 `json:"total_cost"`

	// Confidence (0-1)
	Confidence float64 `json:"confidence"`

	// Coverage breakdown
	Coverage CoveragePayload `json:"coverage"`

	// Delta if diff
	Delta *float64 `json:"delta,omitempty"`

	// Resources count
	ResourceCount int `json:"resource_count"`

	// PolicyViolations
	PolicyViolations []PolicyViolationPayload `json:"policy_violations,omitempty"`

	// Repository info
	Repository RepositoryPayload `json:"repository"`

	// Pull request info
	PullRequest *PullRequestPayload `json:"pull_request,omitempty"`

	// Snapshot
	Snapshot SnapshotPayload `json:"snapshot"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// CoveragePayload is coverage info
type CoveragePayload struct {
	NumericPercent     float64 `json:"numeric_percent"`
	SymbolicPercent    float64 `json:"symbolic_percent"`
	UnsupportedPercent float64 `json:"unsupported_percent"`
}

// PolicyViolationPayload is policy violation
type PolicyViolationPayload struct {
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// RepositoryPayload is repo info
type RepositoryPayload struct {
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	URL      string `json:"url"`
}

// PullRequestPayload is PR info
type PullRequestPayload struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	SHA    string `json:"sha"`
	Branch string `json:"branch"`
	Author string `json:"author"`
}

// SnapshotPayload is snapshot info
type SnapshotPayload struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Region   string `json:"region"`
}

// Send sends the webhook
func (a *Adapter) Send(ctx context.Context, payload *Payload) error {
	var lastErr error

	for attempt := 0; attempt <= a.config.RetryCount; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(a.config.RetryDelay):
			}
		}

		if err := a.sendOnce(ctx, payload); err != nil {
			lastErr = err
			continue
		}
		return nil
	}

	return fmt.Errorf("webhook failed after %d attempts: %w", a.config.RetryCount+1, lastErr)
}

func (a *Adapter) sendOnce(ctx context.Context, payload *Payload) error {
	// Format payload for provider
	body, err := a.formatPayload(payload)
	if err != nil {
		return fmt.Errorf("failed to format payload: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", a.config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range a.config.Headers {
		req.Header.Set(k, v)
	}

	// Sign if secret provided
	if a.config.Secret != "" {
		sig := a.sign(body)
		switch a.config.Provider {
		case ProviderGitHub:
			req.Header.Set("X-Hub-Signature-256", "sha256="+sig)
		case ProviderGitLab:
			req.Header.Set("X-Gitlab-Token", a.config.Secret)
		default:
			req.Header.Set("X-Signature", sig)
		}
	}

	// Send
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (a *Adapter) formatPayload(payload *Payload) ([]byte, error) {
	switch a.config.Provider {
	case ProviderSlack:
		return a.formatSlack(payload)
	case ProviderTeams:
		return a.formatTeams(payload)
	default:
		return json.Marshal(payload)
	}
}

func (a *Adapter) formatSlack(payload *Payload) ([]byte, error) {
	// Slack block kit format
	color := "good"
	if len(payload.PolicyViolations) > 0 {
		color = "danger"
	}

	slack := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": fmt.Sprintf("💰 Cost Estimate: $%.2f/month", payload.TotalCost),
				"fields": []map[string]interface{}{
					{
						"title": "Confidence",
						"value": fmt.Sprintf("%.0f%%", payload.Confidence*100),
						"short": true,
					},
					{
						"title": "Resources",
						"value": fmt.Sprintf("%d", payload.ResourceCount),
						"short": true,
					},
					{
						"title": "Coverage",
						"value": fmt.Sprintf("%.0f%% numeric", payload.Coverage.NumericPercent),
						"short": true,
					},
				},
				"footer": fmt.Sprintf("Snapshot: %s/%s", payload.Snapshot.Provider, payload.Snapshot.Region),
				"ts":     payload.Timestamp.Unix(),
			},
		},
	}

	return json.Marshal(slack)
}

func (a *Adapter) formatTeams(payload *Payload) ([]byte, error) {
	// Microsoft Teams adaptive card format
	themeColor := "00FF00"
	if len(payload.PolicyViolations) > 0 {
		themeColor = "FF0000"
	}

	teams := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"themeColor": themeColor,
		"summary":    fmt.Sprintf("Cost Estimate: $%.2f/month", payload.TotalCost),
		"sections": []map[string]interface{}{
			{
				"activityTitle": "💰 Terraform Cost Estimate",
				"facts": []map[string]interface{}{
					{"name": "Total Cost", "value": fmt.Sprintf("$%.2f/month", payload.TotalCost)},
					{"name": "Confidence", "value": fmt.Sprintf("%.0f%%", payload.Confidence*100)},
					{"name": "Resources", "value": fmt.Sprintf("%d", payload.ResourceCount)},
				},
			},
		},
	}

	return json.Marshal(teams)
}

func (a *Adapter) sign(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(a.config.Secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature verifies an incoming webhook signature
func VerifySignature(payload []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}
