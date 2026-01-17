// Package git provides production-grade Git adapter for the cost estimation engine.
// This adapter extracts git context for reproducible estimations and PR integration.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Adapter is the Git adapter
type Adapter struct {
	repoPath string
	gitPath  string
}

// Config configures the Git adapter
type Config struct {
	// RepoPath is the repository path
	RepoPath string `json:"repo_path"`

	// GitPath is the git executable path (defaults to "git")
	GitPath string `json:"git_path"`
}

// New creates a new Git adapter
func New(config *Config) (*Adapter, error) {
	if config == nil {
		config = &Config{}
	}

	repoPath := config.RepoPath
	if repoPath == "" {
		repoPath = "."
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve repo path: %w", err)
	}

	gitPath := config.GitPath
	if gitPath == "" {
		gitPath = "git"
	}

	return &Adapter{
		repoPath: absPath,
		gitPath:  gitPath,
	}, nil
}

// Context represents git context for an estimation
type Context struct {
	// IsRepo indicates if path is a git repository
	IsRepo bool `json:"is_repo"`

	// Branch is the current branch
	Branch string `json:"branch"`

	// Commit is the current commit SHA
	Commit string `json:"commit"`

	// ShortCommit is shortened SHA
	ShortCommit string `json:"short_commit"`

	// CommitTime is when the commit was made
	CommitTime time.Time `json:"commit_time"`

	// Author is the commit author
	Author string `json:"author"`

	// AuthorEmail is the author email
	AuthorEmail string `json:"author_email"`

	// Message is the commit message
	Message string `json:"message"`

	// Tag is the current tag if any
	Tag string `json:"tag,omitempty"`

	// IsDirty indicates uncommitted changes
	IsDirty bool `json:"is_dirty"`

	// RemoteURL is the origin URL
	RemoteURL string `json:"remote_url,omitempty"`

	// Repository is the parsed repository name
	Repository string `json:"repository,omitempty"`

	// Owner is the repository owner
	Owner string `json:"owner,omitempty"`

	// PRNumber if in a PR context
	PRNumber int `json:"pr_number,omitempty"`

	// BaseBranch for PR comparison
	BaseBranch string `json:"base_branch,omitempty"`
}

// GetContext retrieves the current git context
func (a *Adapter) GetContext(ctx context.Context) (*Context, error) {
	// Check if git repo
	isRepo, err := a.isGitRepo(ctx)
	if err != nil {
		return nil, err
	}

	gitCtx := &Context{IsRepo: isRepo}
	if !isRepo {
		return gitCtx, nil
	}

	// Get branch
	if branch, err := a.getCurrentBranch(ctx); err == nil {
		gitCtx.Branch = branch
	}

	// Get commit info
	if commit, err := a.getCommit(ctx, "HEAD"); err == nil {
		gitCtx.Commit = commit
		if len(commit) >= 7 {
			gitCtx.ShortCommit = commit[:7]
		}
	}

	// Get commit time
	if commitTime, err := a.getCommitTime(ctx, "HEAD"); err == nil {
		gitCtx.CommitTime = commitTime
	}

	// Get author
	if author, email, err := a.getAuthor(ctx, "HEAD"); err == nil {
		gitCtx.Author = author
		gitCtx.AuthorEmail = email
	}

	// Get message
	if message, err := a.getCommitMessage(ctx, "HEAD"); err == nil {
		gitCtx.Message = message
	}

	// Get tag
	if tag, err := a.getCurrentTag(ctx); err == nil {
		gitCtx.Tag = tag
	}

	// Check dirty
	if dirty, err := a.isDirty(ctx); err == nil {
		gitCtx.IsDirty = dirty
	}

	// Get remote
	if remoteURL, err := a.getRemoteURL(ctx, "origin"); err == nil {
		gitCtx.RemoteURL = remoteURL
		owner, repo := parseRemoteURL(remoteURL)
		gitCtx.Owner = owner
		gitCtx.Repository = repo
	}

	return gitCtx, nil
}

// GetChangedFiles returns files changed between two refs
func (a *Adapter) GetChangedFiles(ctx context.Context, base, head string) ([]string, error) {
	if base == "" {
		base = "HEAD~1"
	}
	if head == "" {
		head = "HEAD"
	}

	output, err := a.run(ctx, "diff", "--name-only", base, head)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// GetTerraformChanges returns changed Terraform files
func (a *Adapter) GetTerraformChanges(ctx context.Context, base, head string) ([]string, error) {
	files, err := a.GetChangedFiles(ctx, base, head)
	if err != nil {
		return nil, err
	}

	var tfFiles []string
	for _, file := range files {
		ext := filepath.Ext(file)
		if ext == ".tf" || ext == ".tfvars" || ext == ".hcl" {
			tfFiles = append(tfFiles, file)
		}
	}
	return tfFiles, nil
}

// HasTerraformChanges checks if there are Terraform changes
func (a *Adapter) HasTerraformChanges(ctx context.Context, base, head string) (bool, error) {
	files, err := a.GetTerraformChanges(ctx, base, head)
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}

// GetBaseRef returns the base ref for comparison
func (a *Adapter) GetBaseRef(ctx context.Context, prNumber int) (string, error) {
	// Try to get merge base with main/master
	for _, branch := range []string{"main", "master", "develop"} {
		if _, err := a.getCommit(ctx, "origin/"+branch); err == nil {
			mergeBase, err := a.run(ctx, "merge-base", "HEAD", "origin/"+branch)
			if err == nil {
				return strings.TrimSpace(mergeBase), nil
			}
		}
	}

	// Fall back to parent commit
	return "HEAD~1", nil
}

// Checkout checks out a ref
func (a *Adapter) Checkout(ctx context.Context, ref string) error {
	_, err := a.run(ctx, "checkout", ref)
	return err
}

// Stash stashes changes
func (a *Adapter) Stash(ctx context.Context) error {
	_, err := a.run(ctx, "stash", "push", "-m", "terraform-cost-temp")
	return err
}

// StashPop pops the stash
func (a *Adapter) StashPop(ctx context.Context) error {
	_, err := a.run(ctx, "stash", "pop")
	return err
}

// Fetch fetches from remote
func (a *Adapter) Fetch(ctx context.Context, remote string) error {
	if remote == "" {
		remote = "origin"
	}
	_, err := a.run(ctx, "fetch", remote)
	return err
}

// Private methods

func (a *Adapter) isGitRepo(ctx context.Context) (bool, error) {
	_, err := a.run(ctx, "rev-parse", "--git-dir")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *Adapter) getCurrentBranch(ctx context.Context) (string, error) {
	output, err := a.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func (a *Adapter) getCommit(ctx context.Context, ref string) (string, error) {
	output, err := a.run(ctx, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func (a *Adapter) getCommitTime(ctx context.Context, ref string) (time.Time, error) {
	output, err := a.run(ctx, "show", "-s", "--format=%cI", ref)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(output))
}

func (a *Adapter) getAuthor(ctx context.Context, ref string) (string, string, error) {
	name, err := a.run(ctx, "show", "-s", "--format=%an", ref)
	if err != nil {
		return "", "", err
	}
	email, err := a.run(ctx, "show", "-s", "--format=%ae", ref)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(name), strings.TrimSpace(email), nil
}

func (a *Adapter) getCommitMessage(ctx context.Context, ref string) (string, error) {
	output, err := a.run(ctx, "show", "-s", "--format=%s", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func (a *Adapter) getCurrentTag(ctx context.Context) (string, error) {
	output, err := a.run(ctx, "describe", "--tags", "--exact-match", "HEAD")
	if err != nil {
		return "", nil // No tag is not an error
	}
	return strings.TrimSpace(output), nil
}

func (a *Adapter) isDirty(ctx context.Context) (bool, error) {
	output, err := a.run(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func (a *Adapter) getRemoteURL(ctx context.Context, remote string) (string, error) {
	output, err := a.run(ctx, "remote", "get-url", remote)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func (a *Adapter) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, a.gitPath, args...)
	cmd.Dir = a.repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

// parseRemoteURL parses owner/repo from git remote URL
func parseRemoteURL(url string) (owner, repo string) {
	// SSH format: git@github.com:owner/repo.git
	sshRegex := regexp.MustCompile(`git@[^:]+:([^/]+)/(.+?)(?:\.git)?$`)
	if matches := sshRegex.FindStringSubmatch(url); len(matches) == 3 {
		return matches[1], matches[2]
	}

	// HTTPS format: https://github.com/owner/repo.git
	httpsRegex := regexp.MustCompile(`https?://[^/]+/([^/]+)/(.+?)(?:\.git)?$`)
	if matches := httpsRegex.FindStringSubmatch(url); len(matches) == 3 {
		return matches[1], matches[2]
	}

	return "", ""
}

// PRInfo contains pull request information
type PRInfo struct {
	Number     int       `json:"number"`
	Title      string    `json:"title"`
	Author     string    `json:"author"`
	BaseBranch string    `json:"base_branch"`
	HeadBranch string    `json:"head_branch"`
	BaseSHA    string    `json:"base_sha"`
	HeadSHA    string    `json:"head_sha"`
	IsDraft    bool      `json:"is_draft"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// DiffStats contains diff statistics
type DiffStats struct {
	FilesChanged   int `json:"files_changed"`
	Insertions     int `json:"insertions"`
	Deletions      int `json:"deletions"`
	TerraformFiles int `json:"terraform_files"`
}

// GetDiffStats returns diff statistics between refs
func (a *Adapter) GetDiffStats(ctx context.Context, base, head string) (*DiffStats, error) {
	if base == "" {
		base = "HEAD~1"
	}
	if head == "" {
		head = "HEAD"
	}

	output, err := a.run(ctx, "diff", "--stat", "--numstat", base, head)
	if err != nil {
		return nil, err
	}

	stats := &DiffStats{}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			stats.FilesChanged++
			// numstat format: insertions deletions filename
			if parts[0] != "-" {
				var ins int
				fmt.Sscanf(parts[0], "%d", &ins)
				stats.Insertions += ins
			}
			if parts[1] != "-" {
				var del int
				fmt.Sscanf(parts[1], "%d", &del)
				stats.Deletions += del
			}
			filename := parts[2]
			ext := filepath.Ext(filename)
			if ext == ".tf" || ext == ".tfvars" || ext == ".hcl" {
				stats.TerraformFiles++
			}
		}
	}

	return stats, nil
}
