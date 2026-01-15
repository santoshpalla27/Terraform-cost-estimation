// Package types - Project input types
package types

import "time"

// ProjectInput represents a normalized input for estimation
type ProjectInput struct {
	// ID uniquely identifies this input
	ID string `json:"id"`

	// Path is the filesystem path to the project
	Path string `json:"path"`

	// Type is the detected project type
	Type ProjectType `json:"type"`

	// Source indicates where the input came from
	Source InputSource `json:"source"`

	// Metadata contains additional context
	Metadata InputMetadata `json:"metadata"`
}

// ProjectType identifies the type of infrastructure project
type ProjectType string

const (
	ProjectTypeTerraformHCL    ProjectType = "terraform-hcl"
	ProjectTypeTerraformPlan   ProjectType = "terraform-plan"
	ProjectTypeTerragrunt      ProjectType = "terragrunt"
	ProjectTypeCloudFormation  ProjectType = "cloudformation"
	ProjectTypeUnknown         ProjectType = "unknown"
)

// String returns the string representation
func (t ProjectType) String() string {
	return string(t)
}

// InputSource indicates the origin of the input
type InputSource string

const (
	SourceCLI      InputSource = "cli"
	SourceWeb      InputSource = "web"
	SourceGit      InputSource = "git"
	SourceCICD     InputSource = "cicd"
	SourceAPI      InputSource = "api"
)

// String returns the string representation
func (s InputSource) String() string {
	return string(s)
}

// InputMetadata contains metadata about the input
type InputMetadata struct {
	// Repository is the Git repository URL
	Repository string `json:"repository,omitempty"`

	// Branch is the Git branch
	Branch string `json:"branch,omitempty"`

	// Commit is the Git commit SHA
	Commit string `json:"commit,omitempty"`

	// User is the authenticated user
	User string `json:"user,omitempty"`

	// Timestamp is when the input was received
	Timestamp time.Time `json:"timestamp"`

	// Files lists the input files
	Files []string `json:"files,omitempty"`

	// Hash is a content hash for caching
	Hash string `json:"hash,omitempty"`

	// Environment is the target environment
	Environment string `json:"environment,omitempty"`

	// Tags are user-provided tags
	Tags map[string]string `json:"tags,omitempty"`
}

// DetectedProject contains detection results
type DetectedProject struct {
	// Type is the project type
	Type ProjectType `json:"type"`

	// Roots are the Terraform root modules
	Roots []string `json:"roots"`

	// Confidence is the detection confidence (0.0 to 1.0)
	Confidence float64 `json:"confidence"`

	// Files are the detected IaC files
	Files []DetectedFile `json:"files"`
}

// DetectedFile represents a detected infrastructure file
type DetectedFile struct {
	// Path is the file path relative to project root
	Path string `json:"path"`

	// Type is the file type
	Type FileType `json:"type"`
}

// FileType identifies the type of infrastructure file
type FileType string

const (
	FileTypeTerraform      FileType = "terraform"
	FileTypeTerraformVars  FileType = "tfvars"
	FileTypeTerraformPlan  FileType = "tfplan"
	FileTypeTerragrunt     FileType = "terragrunt"
	FileTypeCloudFormation FileType = "cloudformation"
)
