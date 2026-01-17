// Package terragrunt provides production-grade Terragrunt adapter for the cost estimation engine.
// This adapter handles Terragrunt configuration parsing and multi-module estimation.
package terragrunt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Adapter is the Terragrunt adapter
type Adapter struct {
	terragruntPath string
	terraformPath  string
	workDir        string
	config         *Config
}

// Config configures the Terragrunt adapter
type Config struct {
	// TerragruntPath is the terragrunt executable path
	TerragruntPath string `json:"terragrunt_path"`

	// TerraformPath is the terraform executable path
	TerraformPath string `json:"terraform_path"`

	// WorkDir is the working directory
	WorkDir string `json:"work_dir"`

	// Timeout for commands
	Timeout time.Duration `json:"timeout"`

	// Parallelism for run-all commands
	Parallelism int `json:"parallelism"`

	// IgnoreDependencyErrors continues on dependency errors
	IgnoreDependencyErrors bool `json:"ignore_dependency_errors"`

	// IncludeDirs filters modules to include
	IncludeDirs []string `json:"include_dirs"`

	// ExcludeDirs filters modules to exclude
	ExcludeDirs []string `json:"exclude_dirs"`

	// NoColor disables color output
	NoColor bool `json:"no_color"`

	// NonInteractive disables prompts
	NonInteractive bool `json:"non_interactive"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		TerragruntPath:         "terragrunt",
		TerraformPath:          "terraform",
		WorkDir:                ".",
		Timeout:                60 * time.Minute,
		Parallelism:            5,
		IgnoreDependencyErrors: false,
		NoColor:                true,
		NonInteractive:         true,
	}
}

// New creates a new Terragrunt adapter
func New(config *Config) (*Adapter, error) {
	if config == nil {
		config = DefaultConfig()
	}

	workDir, err := filepath.Abs(config.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve work dir: %w", err)
	}

	return &Adapter{
		terragruntPath: config.TerragruntPath,
		terraformPath:  config.TerraformPath,
		workDir:        workDir,
		config:         config,
	}, nil
}

// Module represents a Terragrunt module
type Module struct {
	// Path is the module directory path
	Path string `json:"path"`

	// RelativePath from workDir
	RelativePath string `json:"relative_path"`

	// Dependencies are module dependencies
	Dependencies []string `json:"dependencies,omitempty"`

	// Inputs are the module inputs
	Inputs map[string]interface{} `json:"inputs,omitempty"`

	// TerraformSource is the terraform source
	TerraformSource string `json:"terraform_source,omitempty"`

	// IncludeConfigs are included configs
	IncludeConfigs []string `json:"include_configs,omitempty"`
}

// ModuleOutput is the output from a module estimation
type ModuleOutput struct {
	// Module is the module info
	Module *Module `json:"module"`

	// Success indicates if estimation succeeded
	Success bool `json:"success"`

	// Error message if failed
	Error string `json:"error,omitempty"`

	// TotalCost is the module cost
	TotalCost float64 `json:"total_cost"`

	// Resources in the module
	ResourceCount int `json:"resource_count"`

	// PlanJSON is the Terraform plan JSON
	PlanJSON json.RawMessage `json:"plan_json,omitempty"`

	// Duration of the estimation
	Duration time.Duration `json:"duration"`
}

// RunAllOutput is the output from run-all command
type RunAllOutput struct {
	// Modules are the module outputs
	Modules []*ModuleOutput `json:"modules"`

	// TotalCost across all modules
	TotalCost float64 `json:"total_cost"`

	// TotalResources across all modules
	TotalResources int `json:"total_resources"`

	// SuccessCount is number of successful modules
	SuccessCount int `json:"success_count"`

	// FailureCount is number of failed modules
	FailureCount int `json:"failure_count"`

	// Duration of the entire run
	Duration time.Duration `json:"duration"`
}

// FindModules finds all Terragrunt modules
func (a *Adapter) FindModules(ctx context.Context) ([]*Module, error) {
	// Use graph-dependencies to find modules
	output, err := a.run(ctx, "graph-dependencies")
	if err != nil {
		// Fall back to finding terragrunt.hcl files
		return a.findModulesByFile(ctx)
	}

	// Parse dependencies
	modules, err := a.parseGraphOutput(output)
	if err != nil {
		return a.findModulesByFile(ctx)
	}

	return modules, nil
}

// findModulesByFile finds modules by searching for terragrunt.hcl
func (a *Adapter) findModulesByFile(ctx context.Context) ([]*Module, error) {
	var modules []*Module

	err := filepath.Walk(a.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}

		// Check for terragrunt.hcl
		if info.Name() == "terragrunt.hcl" {
			dir := filepath.Dir(path)
			relPath, _ := filepath.Rel(a.workDir, dir)

			// Apply filters
			if a.shouldInclude(relPath) {
				modules = append(modules, &Module{
					Path:         dir,
					RelativePath: relPath,
				})
			}
		}

		return nil
	})

	return modules, err
}

// parseGraphOutput parses terragrunt graph-dependencies output
func (a *Adapter) parseGraphOutput(output string) ([]*Module, error) {
	moduleMap := make(map[string]*Module)

	// Parse DOT format
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		// Extract module paths from edges and nodes
		if strings.Contains(line, "->") {
			// Edge: "moduleA" -> "moduleB"
			parts := strings.Split(line, "->")
			for _, part := range parts {
				part = strings.Trim(strings.TrimSpace(part), "\"")
				if part != "" && !strings.HasPrefix(part, "digraph") {
					if _, ok := moduleMap[part]; !ok {
						moduleMap[part] = &Module{
							Path:         filepath.Join(a.workDir, part),
							RelativePath: part,
						}
					}
				}
			}
		} else if strings.Contains(line, "\"") && !strings.Contains(line, "{") && !strings.Contains(line, "}") {
			// Node: "modulePath"
			part := strings.Trim(line, "\" \t;")
			if part != "" && !strings.HasPrefix(part, "digraph") {
				if _, ok := moduleMap[part]; !ok {
					moduleMap[part] = &Module{
						Path:         filepath.Join(a.workDir, part),
						RelativePath: part,
					}
				}
			}
		}
	}

	var modules []*Module
	for _, m := range moduleMap {
		if a.shouldInclude(m.RelativePath) {
			modules = append(modules, m)
		}
	}

	return modules, nil
}

// shouldInclude checks if a module should be included
func (a *Adapter) shouldInclude(path string) bool {
	// Check excludes
	for _, exclude := range a.config.ExcludeDirs {
		if strings.Contains(path, exclude) {
			return false
		}
	}

	// Check includes (if specified)
	if len(a.config.IncludeDirs) > 0 {
		for _, include := range a.config.IncludeDirs {
			if strings.Contains(path, include) {
				return true
			}
		}
		return false
	}

	return true
}

// GetInputs gets the inputs for a module
func (a *Adapter) GetInputs(ctx context.Context, modulePath string) (map[string]interface{}, error) {
	output, err := a.runInDir(ctx, modulePath, "terragrunt-info")
	if err != nil {
		return nil, err
	}

	var info struct {
		Inputs map[string]interface{} `json:"Inputs"`
	}
	if err := json.Unmarshal([]byte(output), &info); err != nil {
		return nil, fmt.Errorf("failed to parse terragrunt-info: %w", err)
	}

	return info.Inputs, nil
}

// PlanModule generates Terraform plan for a single module
func (a *Adapter) PlanModule(ctx context.Context, module *Module, outFile string) error {
	args := []string{"plan", "-out=" + outFile, "-input=false"}

	if a.config.NoColor {
		args = append(args, "-no-color")
	}

	if a.config.NonInteractive {
		args = append(args, "--terragrunt-non-interactive")
	}

	_, err := a.runInDir(ctx, module.Path, args...)
	return err
}

// ShowPlanJSON shows plan in JSON format for a module
func (a *Adapter) ShowPlanJSON(ctx context.Context, module *Module, planFile string) (json.RawMessage, error) {
	args := []string{"show", "-json", planFile}

	if a.config.NoColor {
		args = append(args, "-no-color")
	}

	output, err := a.runInDir(ctx, module.Path, args...)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(output), nil
}

// RunAll runs estimation on all modules
func (a *Adapter) RunAll(ctx context.Context) (*RunAllOutput, error) {
	start := time.Now()

	modules, err := a.FindModules(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find modules: %w", err)
	}

	output := &RunAllOutput{
		Modules: make([]*ModuleOutput, 0, len(modules)),
	}

	// Process modules in parallel
	var wg sync.WaitGroup
	results := make(chan *ModuleOutput, len(modules))
	semaphore := make(chan struct{}, a.config.Parallelism)

	for _, module := range modules {
		wg.Add(1)
		go func(m *Module) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := a.processModule(ctx, m)
			results <- result
		}(module)
	}

	// Close results when done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for result := range results {
		output.Modules = append(output.Modules, result)
		if result.Success {
			output.SuccessCount++
			output.TotalCost += result.TotalCost
			output.TotalResources += result.ResourceCount
		} else {
			output.FailureCount++
		}
	}

	output.Duration = time.Since(start)
	return output, nil
}

// processModule processes a single module
func (a *Adapter) processModule(ctx context.Context, module *Module) *ModuleOutput {
	start := time.Now()

	result := &ModuleOutput{
		Module: module,
	}

	// Create temp plan file
	planFile := filepath.Join(module.Path, ".terraform-cost-plan")
	defer os.Remove(planFile)

	// Generate plan
	if err := a.PlanModule(ctx, module, planFile); err != nil {
		result.Error = fmt.Sprintf("plan failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	// Get plan JSON
	planJSON, err := a.ShowPlanJSON(ctx, module, planFile)
	if err != nil {
		result.Error = fmt.Sprintf("show plan failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Success = true
	result.PlanJSON = planJSON
	result.Duration = time.Since(start)

	// Count resources from plan JSON
	var plan struct {
		ResourceChanges []struct {
			Address string `json:"address"`
			Mode    string `json:"mode"`
		} `json:"resource_changes"`
	}
	if err := json.Unmarshal(planJSON, &plan); err == nil {
		for _, rc := range plan.ResourceChanges {
			if rc.Mode == "managed" {
				result.ResourceCount++
			}
		}
	}

	return result
}

// Init initializes all modules
func (a *Adapter) Init(ctx context.Context) error {
	args := []string{"run-all", "init", "--terragrunt-non-interactive"}

	if a.config.NoColor {
		args = append(args, "-no-color")
	}

	if a.config.Parallelism > 0 {
		args = append(args, fmt.Sprintf("--terragrunt-parallelism=%d", a.config.Parallelism))
	}

	if a.config.IgnoreDependencyErrors {
		args = append(args, "--terragrunt-ignore-dependency-errors")
	}

	_, err := a.run(ctx, args...)
	return err
}

// Validate validates all modules
func (a *Adapter) Validate(ctx context.Context) error {
	args := []string{"run-all", "validate", "--terragrunt-non-interactive"}

	if a.config.NoColor {
		args = append(args, "-no-color")
	}

	_, err := a.run(ctx, args...)
	return err
}

// Version returns Terragrunt version
func (a *Adapter) Version(ctx context.Context) (string, error) {
	output, err := a.run(ctx, "--version")
	if err != nil {
		return "", err
	}

	// Parse "terragrunt version v0.xx.xx"
	parts := strings.Fields(output)
	if len(parts) >= 3 {
		return strings.TrimPrefix(parts[2], "v"), nil
	}

	return strings.TrimSpace(output), nil
}

// run executes a Terragrunt command
func (a *Adapter) run(ctx context.Context, args ...string) (string, error) {
	return a.runInDir(ctx, a.workDir, args...)
}

// runInDir executes a Terragrunt command in a specific directory
func (a *Adapter) runInDir(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.terragruntPath, args...)
	cmd.Dir = dir

	// Set terraform path if specified
	if a.terraformPath != "" {
		cmd.Env = append(os.Environ(), "TERRAGRUNT_TFPATH="+a.terraformPath)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("terragrunt %s failed: %w: %s", strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

// GetDependencyGraph returns the dependency graph
func (a *Adapter) GetDependencyGraph(ctx context.Context) (map[string][]string, error) {
	output, err := a.run(ctx, "graph-dependencies")
	if err != nil {
		return nil, err
	}

	graph := make(map[string][]string)

	// Parse DOT format for dependencies
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "->") {
			parts := strings.Split(line, "->")
			if len(parts) == 2 {
				from := strings.Trim(strings.TrimSpace(parts[0]), "\"")
				to := strings.Trim(strings.TrimSpace(parts[1]), "\" ;")
				graph[from] = append(graph[from], to)
			}
		}
	}

	return graph, nil
}

// GetOrderedModules returns modules in dependency order
func (a *Adapter) GetOrderedModules(ctx context.Context) ([]*Module, error) {
	graph, err := a.GetDependencyGraph(ctx)
	if err != nil {
		return a.FindModules(ctx)
	}

	// Topological sort
	var ordered []string
	visited := make(map[string]bool)
	temp := make(map[string]bool)

	var visit func(node string) error
	visit = func(node string) error {
		if temp[node] {
			return fmt.Errorf("circular dependency detected at %s", node)
		}
		if visited[node] {
			return nil
		}

		temp[node] = true
		for _, dep := range graph[node] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		temp[node] = false
		visited[node] = true
		ordered = append(ordered, node)
		return nil
	}

	for node := range graph {
		if !visited[node] {
			if err := visit(node); err != nil {
				return nil, err
			}
		}
	}

	// Convert to modules
	var modules []*Module
	for _, path := range ordered {
		modules = append(modules, &Module{
			Path:         filepath.Join(a.workDir, path),
			RelativePath: path,
			Dependencies: graph[path],
		})
	}

	return modules, nil
}
