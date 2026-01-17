// Package terraform provides production-grade Terraform adapter for the cost estimation engine.
// This adapter handles Terraform plan parsing, HCL scanning, and state extraction.
package terraform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Adapter is the Terraform adapter
type Adapter struct {
	terraformPath string
	workDir       string
	config        *Config
}

// Config configures the Terraform adapter
type Config struct {
	// TerraformPath is the terraform executable path
	TerraformPath string `json:"terraform_path"`

	// WorkDir is the working directory
	WorkDir string `json:"work_dir"`

	// Workspace is the Terraform workspace
	Workspace string `json:"workspace"`

	// VarFiles are additional var files
	VarFiles []string `json:"var_files"`

	// Vars are inline variables
	Vars map[string]string `json:"vars"`

	// BackendConfig for remote backends
	BackendConfig map[string]string `json:"backend_config"`

	// Timeout for Terraform commands
	Timeout time.Duration `json:"timeout"`

	// Parallelism for plan/apply
	Parallelism int `json:"parallelism"`

	// NoColor disables color output
	NoColor bool `json:"no_color"`

	// LockTimeout for state locking
	LockTimeout time.Duration `json:"lock_timeout"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		TerraformPath: "terraform",
		WorkDir:       ".",
		Workspace:     "default",
		Timeout:       30 * time.Minute,
		Parallelism:   10,
		NoColor:       true,
		LockTimeout:   5 * time.Minute,
	}
}

// New creates a new Terraform adapter
func New(config *Config) (*Adapter, error) {
	if config == nil {
		config = DefaultConfig()
	}

	workDir, err := filepath.Abs(config.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve work dir: %w", err)
	}

	return &Adapter{
		terraformPath: config.TerraformPath,
		workDir:       workDir,
		config:        config,
	}, nil
}

// PlanOutput represents parsed Terraform plan output
type PlanOutput struct {
	// FormatVersion is the plan format version
	FormatVersion string `json:"format_version"`

	// TerraformVersion is the Terraform version
	TerraformVersion string `json:"terraform_version"`

	// ResourceChanges contains resource changes
	ResourceChanges []ResourceChange `json:"resource_changes"`

	// Configuration contains the configuration
	Configuration *Configuration `json:"configuration,omitempty"`

	// PlannedValues contains planned values
	PlannedValues *PlannedValues `json:"planned_values,omitempty"`

	// PriorState is the prior state
	PriorState *State `json:"prior_state,omitempty"`

	// Variables are the input variables
	Variables map[string]Variable `json:"variables,omitempty"`
}

// ResourceChange represents a single resource change
type ResourceChange struct {
	// Address is the resource address
	Address string `json:"address"`

	// ModuleAddress is the module path
	ModuleAddress string `json:"module_address,omitempty"`

	// Mode is data or managed
	Mode string `json:"mode"`

	// Type is the resource type
	Type string `json:"type"`

	// Name is the resource name
	Name string `json:"name"`

	// Index for count/for_each
	Index interface{} `json:"index,omitempty"`

	// ProviderName is the provider
	ProviderName string `json:"provider_name"`

	// Change contains the change details
	Change Change `json:"change"`

	// ActionReason explains why action is needed
	ActionReason string `json:"action_reason,omitempty"`
}

// Change represents the change details
type Change struct {
	// Actions are the change actions
	Actions []string `json:"actions"`

	// Before is the state before
	Before map[string]interface{} `json:"before"`

	// After is the state after
	After map[string]interface{} `json:"after"`

	// AfterUnknown marks unknown values
	AfterUnknown map[string]interface{} `json:"after_unknown"`

	// BeforeSensitive marks sensitive values
	BeforeSensitive interface{} `json:"before_sensitive"`

	// AfterSensitive marks sensitive values
	AfterSensitive interface{} `json:"after_sensitive"`
}

// Configuration represents Terraform configuration
type Configuration struct {
	// ProviderConfig contains provider configs
	ProviderConfig map[string]ProviderConfig `json:"provider_config,omitempty"`

	// RootModule is the root module
	RootModule ModuleConfig `json:"root_module"`
}

// ProviderConfig is provider configuration
type ProviderConfig struct {
	Name        string                 `json:"name"`
	FullName    string                 `json:"full_name"`
	VersionConstraint string           `json:"version_constraint,omitempty"`
	Expressions map[string]interface{} `json:"expressions,omitempty"`
}

// ModuleConfig is module configuration
type ModuleConfig struct {
	Resources []ResourceConfig `json:"resources,omitempty"`
	ModuleCalls map[string]ModuleCall `json:"module_calls,omitempty"`
	Variables map[string]VariableConfig `json:"variables,omitempty"`
	Outputs map[string]OutputConfig `json:"outputs,omitempty"`
}

// ResourceConfig is resource configuration
type ResourceConfig struct {
	Address           string                 `json:"address"`
	Mode              string                 `json:"mode"`
	Type              string                 `json:"type"`
	Name              string                 `json:"name"`
	ProviderConfigKey string                 `json:"provider_config_key"`
	Expressions       map[string]interface{} `json:"expressions,omitempty"`
	SchemaVersion     int                    `json:"schema_version"`
	CountExpression   interface{}            `json:"count_expression,omitempty"`
	ForEachExpression interface{}            `json:"for_each_expression,omitempty"`
}

// ModuleCall is a module call
type ModuleCall struct {
	Source            string                 `json:"source"`
	VersionConstraint string                 `json:"version_constraint,omitempty"`
	Expressions       map[string]interface{} `json:"expressions,omitempty"`
	CountExpression   interface{}            `json:"count_expression,omitempty"`
	ForEachExpression interface{}            `json:"for_each_expression,omitempty"`
	Module            ModuleConfig           `json:"module"`
}

// VariableConfig is variable configuration
type VariableConfig struct {
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description,omitempty"`
	Sensitive   bool        `json:"sensitive,omitempty"`
}

// OutputConfig is output configuration
type OutputConfig struct {
	Expression  interface{} `json:"expression,omitempty"`
	Description string      `json:"description,omitempty"`
	Sensitive   bool        `json:"sensitive,omitempty"`
}

// PlannedValues contains planned values
type PlannedValues struct {
	RootModule PlannedModule `json:"root_module"`
	Outputs    map[string]OutputValue `json:"outputs,omitempty"`
}

// PlannedModule is a planned module
type PlannedModule struct {
	Resources    []PlannedResource `json:"resources,omitempty"`
	ChildModules []PlannedModule   `json:"child_modules,omitempty"`
	Address      string            `json:"address,omitempty"`
}

// PlannedResource is a planned resource
type PlannedResource struct {
	Address       string                 `json:"address"`
	Mode          string                 `json:"mode"`
	Type          string                 `json:"type"`
	Name          string                 `json:"name"`
	Index         interface{}            `json:"index,omitempty"`
	ProviderName  string                 `json:"provider_name"`
	SchemaVersion int                    `json:"schema_version"`
	Values        map[string]interface{} `json:"values"`
	SensitiveValues interface{}          `json:"sensitive_values"`
}

// OutputValue is an output value
type OutputValue struct {
	Sensitive bool        `json:"sensitive"`
	Value     interface{} `json:"value"`
	Type      interface{} `json:"type,omitempty"`
}

// State represents Terraform state
type State struct {
	FormatVersion   string       `json:"format_version"`
	TerraformVersion string      `json:"terraform_version"`
	Values          *PlannedValues `json:"values,omitempty"`
}

// Variable is an input variable
type Variable struct {
	Value interface{} `json:"value"`
}

// Init initializes the Terraform working directory
func (a *Adapter) Init(ctx context.Context) error {
	args := []string{"init", "-input=false"}

	if a.config.NoColor {
		args = append(args, "-no-color")
	}

	// Add backend config
	for k, v := range a.config.BackendConfig {
		args = append(args, fmt.Sprintf("-backend-config=%s=%s", k, v))
	}

	_, err := a.run(ctx, args...)
	return err
}

// Plan generates a Terraform plan
func (a *Adapter) Plan(ctx context.Context, outFile string) error {
	args := []string{"plan", "-input=false", "-out=" + outFile}

	if a.config.NoColor {
		args = append(args, "-no-color")
	}

	if a.config.Parallelism > 0 {
		args = append(args, fmt.Sprintf("-parallelism=%d", a.config.Parallelism))
	}

	if a.config.LockTimeout > 0 {
		args = append(args, fmt.Sprintf("-lock-timeout=%s", a.config.LockTimeout))
	}

	// Add var files
	for _, varFile := range a.config.VarFiles {
		args = append(args, "-var-file="+varFile)
	}

	// Add vars
	for k, v := range a.config.Vars {
		args = append(args, fmt.Sprintf("-var=%s=%s", k, v))
	}

	_, err := a.run(ctx, args...)
	return err
}

// ShowPlanJSON shows plan in JSON format
func (a *Adapter) ShowPlanJSON(ctx context.Context, planFile string) (*PlanOutput, error) {
	args := []string{"show", "-json", planFile}

	if a.config.NoColor {
		args = append(args, "-no-color")
	}

	output, err := a.run(ctx, args...)
	if err != nil {
		return nil, err
	}

	var plan PlanOutput
	if err := json.Unmarshal([]byte(output), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	return &plan, nil
}

// ParsePlanFile parses an existing plan file
func (a *Adapter) ParsePlanFile(ctx context.Context, planFile string) (*PlanOutput, error) {
	return a.ShowPlanJSON(ctx, planFile)
}

// ParsePlanJSON parses plan JSON directly
func (a *Adapter) ParsePlanJSON(data []byte) (*PlanOutput, error) {
	var plan PlanOutput
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}
	return &plan, nil
}

// Validate validates Terraform configuration
func (a *Adapter) Validate(ctx context.Context) error {
	args := []string{"validate", "-json"}

	if a.config.NoColor {
		args = append(args, "-no-color")
	}

	_, err := a.run(ctx, args...)
	return err
}

// SelectWorkspace selects a workspace
func (a *Adapter) SelectWorkspace(ctx context.Context, workspace string) error {
	// Try to select
	_, err := a.run(ctx, "workspace", "select", workspace)
	if err != nil {
		// Try to create
		_, err = a.run(ctx, "workspace", "new", workspace)
	}
	return err
}

// GetWorkspace returns the current workspace
func (a *Adapter) GetWorkspace(ctx context.Context) (string, error) {
	output, err := a.run(ctx, "workspace", "show")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// State returns current state
func (a *Adapter) State(ctx context.Context) (*State, error) {
	output, err := a.run(ctx, "show", "-json")
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal([]byte(output), &state); err != nil {
		return nil, fmt.Errorf("failed to parse state JSON: %w", err)
	}

	return &state, nil
}

// Providers returns the required providers
func (a *Adapter) Providers(ctx context.Context) ([]string, error) {
	output, err := a.run(ctx, "providers")
	if err != nil {
		return nil, err
	}

	var providers []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "provider[") {
			// Extract provider name
			start := strings.Index(line, "[") + 1
			end := strings.Index(line, "]")
			if start > 0 && end > start {
				providers = append(providers, line[start:end])
			}
		}
	}

	return providers, nil
}

// Version returns Terraform version
func (a *Adapter) Version(ctx context.Context) (string, error) {
	output, err := a.run(ctx, "version", "-json")
	if err != nil {
		// Try without -json for older versions
		output, err = a.run(ctx, "version")
		if err != nil {
			return "", err
		}
		lines := strings.Split(output, "\n")
		if len(lines) > 0 {
			return strings.TrimPrefix(lines[0], "Terraform v"), nil
		}
		return "", nil
	}

	var result struct {
		TerraformVersion string `json:"terraform_version"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return "", err
	}

	return result.TerraformVersion, nil
}

// FindTerraformFiles finds all Terraform files in directory
func (a *Adapter) FindTerraformFiles(ctx context.Context) ([]string, error) {
	var files []string

	err := filepath.Walk(a.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories and common non-terraform dirs
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext == ".tf" || ext == ".tf.json" {
			relPath, _ := filepath.Rel(a.workDir, path)
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

// run executes a Terraform command
func (a *Adapter) run(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.terraformPath, args...)
	cmd.Dir = a.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("terraform %s failed: %w: %s", strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

// ExtractResources extracts resources from plan for cost estimation
func (a *Adapter) ExtractResources(plan *PlanOutput) []ResourceInfo {
	var resources []ResourceInfo

	for _, change := range plan.ResourceChanges {
		// Skip data sources
		if change.Mode == "data" {
			continue
		}

		// Determine action
		action := "no_change"
		if len(change.Change.Actions) > 0 {
			if contains(change.Change.Actions, "create") {
				action = "create"
			} else if contains(change.Change.Actions, "delete") {
				action = "destroy"
			} else if contains(change.Change.Actions, "update") {
				action = "update"
			}
		}

		resources = append(resources, ResourceInfo{
			Address:       change.Address,
			Type:          change.Type,
			Name:          change.Name,
			Provider:      change.ProviderName,
			ModuleAddress: change.ModuleAddress,
			Index:         change.Index,
			Action:        action,
			Values:        change.Change.After,
			PriorValues:   change.Change.Before,
			Unknown:       change.Change.AfterUnknown,
		})
	}

	return resources
}

// ResourceInfo is extracted resource information
type ResourceInfo struct {
	Address       string                 `json:"address"`
	Type          string                 `json:"type"`
	Name          string                 `json:"name"`
	Provider      string                 `json:"provider"`
	ModuleAddress string                 `json:"module_address,omitempty"`
	Index         interface{}            `json:"index,omitempty"`
	Action        string                 `json:"action"`
	Values        map[string]interface{} `json:"values"`
	PriorValues   map[string]interface{} `json:"prior_values,omitempty"`
	Unknown       map[string]interface{} `json:"unknown,omitempty"`
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
