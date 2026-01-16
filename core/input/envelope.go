// Package input - Normalized input envelope
// EVERYTHING downstream consumes this only.
// Decouples CLI, Git, API semantics from Terraform parsing.
package input

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Envelope is the normalized input to the estimation engine
// All sources (CLI, Git, API) produce this format
type Envelope struct {
	// Source information
	Source SourceInfo

	// Normalized filesystem
	FileSystem *NormalizedFS

	// Context
	Workspace  string
	Variables  map[string]interface{}
	VarFiles   []string

	// Metadata
	Metadata EnvelopeMetadata
}

// SourceInfo describes where the input came from
type SourceInfo struct {
	Type      SourceType
	Path      string            // For CLI: local path
	RepoURL   string            // For Git: repository URL
	CommitSHA string            // For Git: commit hash
	Branch    string            // For Git: branch name
	PRNumber  int               // For Git: PR number if applicable
	APISource string            // For API: source identifier
	Tags      map[string]string // Additional tags
}

// SourceType indicates the source of input
type SourceType int

const (
	SourceCLI SourceType = iota // Local CLI invocation
	SourceGit                    // Git repository
	SourceAPI                    // API request
	SourceCI                     // CI/CD pipeline
)

// String returns the source type name
func (t SourceType) String() string {
	switch t {
	case SourceCLI:
		return "cli"
	case SourceGit:
		return "git"
	case SourceAPI:
		return "api"
	case SourceCI:
		return "ci"
	default:
		return "unknown"
	}
}

// NormalizedFS is a normalized representation of the filesystem
type NormalizedFS struct {
	// Root path (virtual)
	Root string

	// Files by path
	Files map[string]*NormalizedFile

	// Content hash of entire FS
	ContentHash string

	// Stats
	TotalFiles int
	TotalBytes int64
}

// NormalizedFile is a normalized file
type NormalizedFile struct {
	Path        string
	Content     []byte
	ContentHash string
	ModTime     time.Time
	Size        int64
}

// EnvelopeMetadata contains metadata about the envelope
type EnvelopeMetadata struct {
	CreatedAt   time.Time
	Version     string
	EngineID    string
	ReplayToken string // For CI reproducibility
}

// NewEnvelope creates a new envelope from a source
func NewEnvelope(source SourceInfo) *Envelope {
	return &Envelope{
		Source:    source,
		Workspace: "default",
		Variables: make(map[string]interface{}),
		VarFiles:  []string{},
		Metadata: EnvelopeMetadata{
			CreatedAt: time.Now().UTC(),
			Version:   "1.0",
		},
	}
}

// NewEnvelopeFromCLI creates an envelope from CLI input
func NewEnvelopeFromCLI(path string, workspace string, vars map[string]interface{}) (*Envelope, error) {
	env := NewEnvelope(SourceInfo{
		Type: SourceCLI,
		Path: path,
	})
	env.Workspace = workspace
	env.Variables = vars

	// Normalize filesystem
	nfs, err := NormalizeDirectory(path)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize filesystem: %w", err)
	}
	env.FileSystem = nfs

	// Generate replay token
	env.Metadata.ReplayToken = generateReplayToken(env)

	return env, nil
}

// NewEnvelopeFromGit creates an envelope from Git input
func NewEnvelopeFromGit(repoURL, commitSHA, branch string) *Envelope {
	return NewEnvelope(SourceInfo{
		Type:      SourceGit,
		RepoURL:   repoURL,
		CommitSHA: commitSHA,
		Branch:    branch,
	})
}

// NormalizeDirectory creates a normalized FS from a directory
func NormalizeDirectory(root string) (*NormalizedFS, error) {
	nfs := &NormalizedFS{
		Root:  root,
		Files: make(map[string]*NormalizedFile),
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			// Skip hidden directories
			if d.Name()[0] == '.' && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Only include .tf and .tfvars files
		ext := filepath.Ext(path)
		if ext != ".tf" && ext != ".tfvars" {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		// Normalize path separators
		relPath = filepath.ToSlash(relPath)

		info, _ := d.Info()
		modTime := time.Time{}
		if info != nil {
			modTime = info.ModTime()
		}

		nfs.Files[relPath] = &NormalizedFile{
			Path:        relPath,
			Content:     content,
			ContentHash: hashContent(content),
			ModTime:     modTime,
			Size:        int64(len(content)),
		}

		nfs.TotalFiles++
		nfs.TotalBytes += int64(len(content))

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Compute overall hash
	nfs.ContentHash = nfs.computeHash()

	return nfs, nil
}

func (nfs *NormalizedFS) computeHash() string {
	h := sha256.New()

	// Sort paths for determinism
	paths := make([]string, 0, len(nfs.Files))
	for p := range nfs.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write([]byte(nfs.Files[p].ContentHash))
		h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil))
}

func hashContent(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func generateReplayToken(env *Envelope) string {
	h := sha256.New()
	h.Write([]byte(env.Source.Type.String()))
	h.Write([]byte(env.Source.Path))
	h.Write([]byte(env.Workspace))
	if env.FileSystem != nil {
		h.Write([]byte(env.FileSystem.ContentHash))
	}
	h.Write([]byte(env.Metadata.CreatedAt.Format(time.RFC3339)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// GetFile returns a file by path
func (nfs *NormalizedFS) GetFile(path string) *NormalizedFile {
	return nfs.Files[path]
}

// GetTerraformFiles returns all .tf files
func (nfs *NormalizedFS) GetTerraformFiles() []*NormalizedFile {
	var result []*NormalizedFile
	for path, f := range nfs.Files {
		if filepath.Ext(path) == ".tf" {
			result = append(result, f)
		}
	}
	// Sort for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	return result
}

// GetVarFiles returns all .tfvars files
func (nfs *NormalizedFS) GetVarFiles() []*NormalizedFile {
	var result []*NormalizedFile
	for path, f := range nfs.Files {
		if filepath.Ext(path) == ".tfvars" {
			result = append(result, f)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	return result
}

// EnvelopeValidator validates an envelope
type EnvelopeValidator struct {
	errors []string
}

// NewEnvelopeValidator creates a validator
func NewEnvelopeValidator() *EnvelopeValidator {
	return &EnvelopeValidator{
		errors: []string{},
	}
}

// Validate validates an envelope
func (v *EnvelopeValidator) Validate(env *Envelope) []string {
	v.errors = []string{}

	// Must have source type
	if env.Source.Type < SourceCLI || env.Source.Type > SourceCI {
		v.errors = append(v.errors, "invalid source type")
	}

	// CLI must have path
	if env.Source.Type == SourceCLI && env.Source.Path == "" {
		v.errors = append(v.errors, "CLI source must have path")
	}

	// Git must have repo and commit
	if env.Source.Type == SourceGit {
		if env.Source.RepoURL == "" {
			v.errors = append(v.errors, "Git source must have repo URL")
		}
		if env.Source.CommitSHA == "" {
			v.errors = append(v.errors, "Git source must have commit SHA")
		}
	}

	// Must have filesystem
	if env.FileSystem == nil {
		v.errors = append(v.errors, "envelope must have normalized filesystem")
	} else if len(env.FileSystem.Files) == 0 {
		v.errors = append(v.errors, "no Terraform files found")
	}

	return v.errors
}

// IsReplayable returns true if the envelope can be replayed
func (env *Envelope) IsReplayable() bool {
	return env.Metadata.ReplayToken != "" &&
		env.FileSystem != nil &&
		env.FileSystem.ContentHash != ""
}

// GetReplayInfo returns info needed to replay this envelope
func (env *Envelope) GetReplayInfo() map[string]string {
	info := map[string]string{
		"replay_token":  env.Metadata.ReplayToken,
		"content_hash":  "",
		"source_type":   env.Source.Type.String(),
		"workspace":     env.Workspace,
		"created_at":    env.Metadata.CreatedAt.Format(time.RFC3339),
	}
	if env.FileSystem != nil {
		info["content_hash"] = env.FileSystem.ContentHash
	}
	return info
}
