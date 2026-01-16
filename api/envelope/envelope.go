// Package envelope - Input normalization and envelope creation
// The engine NEVER sees raw input - only normalized envelopes.
// This ensures determinism and auditability.
package envelope

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// InputEnvelope is the normalized, hashed representation of an API request.
// The engine receives ONLY this - never raw input.
type InputEnvelope struct {
	// Source identification
	SourceType string `json:"source_type"` // "git", "upload", "local"
	Repo       string `json:"repo,omitempty"`
	Ref        string `json:"ref,omitempty"`
	Path       string `json:"path"`

	// Normalized fields
	CanonicalPath string `json:"canonical_path"`
	ResolvedRef   string `json:"resolved_ref,omitempty"`

	// Identity
	InputHash string `json:"input_hash"`

	// Execution context
	Mode         string `json:"mode"`
	UsageProfile string `json:"usage_profile,omitempty"`

	// Timing
	NormalizedAt time.Time `json:"normalized_at"`

	// Options
	Options EnvelopeOptions `json:"options"`
}

// EnvelopeOptions are normalized options
type EnvelopeOptions struct {
	IncludeDependencyGraph bool `json:"include_dependency_graph"`
	IncludeCostLineage     bool `json:"include_cost_lineage"`
	IncludeComponents      bool `json:"include_components"`
}

// RawInput represents unnormalized API input
type RawInput struct {
	SourceType   string
	Repo         string
	Ref          string
	Path         string
	Mode         string
	UsageProfile string
	Options      RawOptions
}

// RawOptions are unnormalized options
type RawOptions struct {
	IncludeDependencyGraph bool
	IncludeCostLineage     bool
	IncludeComponents      bool
}

// Normalizer normalizes raw input into envelopes
type Normalizer struct {
	// RefResolver resolves git refs to commit hashes (optional)
	RefResolver RefResolver
}

// RefResolver resolves git refs
type RefResolver interface {
	Resolve(repo, ref string) (string, error)
}

// NewNormalizer creates a normalizer
func NewNormalizer() *Normalizer {
	return &Normalizer{}
}

// WithRefResolver sets the ref resolver
func (n *Normalizer) WithRefResolver(r RefResolver) *Normalizer {
	n.RefResolver = r
	return n
}

// Normalize transforms raw input to a deterministic envelope
func (n *Normalizer) Normalize(raw RawInput) (*InputEnvelope, error) {
	now := time.Now().UTC()

	envelope := &InputEnvelope{
		SourceType:   normalizeSourceType(raw.SourceType),
		Repo:         normalizeRepo(raw.Repo),
		Ref:          raw.Ref,
		Path:         raw.Path,
		Mode:         normalizeMode(raw.Mode),
		UsageProfile: raw.UsageProfile,
		NormalizedAt: now,
		Options: EnvelopeOptions{
			IncludeDependencyGraph: raw.Options.IncludeDependencyGraph,
			IncludeCostLineage:     raw.Options.IncludeCostLineage,
			IncludeComponents:      raw.Options.IncludeComponents,
		},
	}

	// Canonicalize path
	envelope.CanonicalPath = canonicalizePath(raw.SourceType, raw.Repo, raw.Ref, raw.Path)

	// Resolve ref if possible
	if n.RefResolver != nil && raw.Repo != "" && raw.Ref != "" {
		resolved, err := n.RefResolver.Resolve(raw.Repo, raw.Ref)
		if err == nil {
			envelope.ResolvedRef = resolved
		}
	}

	// Compute deterministic hash
	envelope.InputHash = computeInputHash(envelope)

	return envelope, nil
}

func normalizeSourceType(t string) string {
	switch strings.ToLower(t) {
	case "git", "github", "gitlab", "bitbucket":
		return "git"
	case "upload", "uploaded":
		return "upload"
	case "local", "file", "path":
		return "local"
	default:
		return "local"
	}
}

func normalizeRepo(repo string) string {
	// Remove trailing .git
	repo = strings.TrimSuffix(repo, ".git")
	// Normalize to HTTPS if SSH
	if strings.HasPrefix(repo, "git@") {
		repo = strings.Replace(repo, ":", "/", 1)
		repo = strings.Replace(repo, "git@", "https://", 1)
	}
	return repo
}

func normalizeMode(mode string) string {
	switch strings.ToLower(mode) {
	case "strict":
		return "strict"
	default:
		return "permissive"
	}
}

func canonicalizePath(sourceType, repo, ref, path string) string {
	switch sourceType {
	case "git":
		// git:repo@ref:path
		return fmt.Sprintf("git:%s@%s:%s", repo, ref, normalizePath(path))
	case "upload":
		return fmt.Sprintf("upload:%s", path)
	default:
		return normalizePath(path)
	}
}

func normalizePath(path string) string {
	// Clean the path
	path = filepath.Clean(path)
	// Convert to forward slashes
	path = filepath.ToSlash(path)
	// Remove leading ./
	path = strings.TrimPrefix(path, "./")
	return path
}

func computeInputHash(envelope *InputEnvelope) string {
	// Hash only the fields that affect estimation
	hashData := struct {
		CanonicalPath string
		ResolvedRef   string
		Mode          string
		UsageProfile  string
	}{
		CanonicalPath: envelope.CanonicalPath,
		ResolvedRef:   envelope.ResolvedRef,
		Mode:          envelope.Mode,
		UsageProfile:  envelope.UsageProfile,
	}

	data, _ := json.Marshal(hashData)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Validate validates the envelope
func (e *InputEnvelope) Validate() error {
	if e.CanonicalPath == "" {
		return fmt.Errorf("canonical path is required")
	}
	if e.InputHash == "" {
		return fmt.Errorf("input hash is required")
	}
	if e.Mode != "strict" && e.Mode != "permissive" {
		return fmt.Errorf("invalid mode: %s", e.Mode)
	}
	return nil
}

// ShortHash returns first 12 characters of hash
func (e *InputEnvelope) ShortHash() string {
	if len(e.InputHash) >= 12 {
		return e.InputHash[:12]
	}
	return e.InputHash
}

// IsGit returns true if source is git
func (e *InputEnvelope) IsGit() bool {
	return e.SourceType == "git"
}

// IsStrict returns true if strict mode
func (e *InputEnvelope) IsStrict() bool {
	return e.Mode == "strict"
}
