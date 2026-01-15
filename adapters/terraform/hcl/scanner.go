// Package hcl provides Terraform HCL parsing.
package hcl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"

	"terraform-cost/core/scanner"
	"terraform-cost/core/types"
)

// Scanner implements the scanner.Scanner interface for Terraform HCL
type Scanner struct {
	parser *hclparse.Parser
}

// NewScanner creates a new HCL scanner
func NewScanner() *Scanner {
	return &Scanner{
		parser: hclparse.NewParser(),
	}
}

// Name returns the scanner name
func (s *Scanner) Name() string {
	return "terraform-hcl"
}

// CanScan determines if this scanner can handle the input
func (s *Scanner) CanScan(ctx context.Context, input *types.ProjectInput) (bool, error) {
	// Look for .tf files
	hasTfFiles := false
	err := filepath.Walk(input.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".tf") {
			hasTfFiles = true
			return filepath.SkipAll // Found one, that's enough
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return false, err
	}

	return hasTfFiles, nil
}

// Scan parses HCL files and returns raw assets
func (s *Scanner) Scan(ctx context.Context, input *types.ProjectInput) (*scanner.ScanResult, error) {
	result := &scanner.ScanResult{
		Assets:    make([]types.RawAsset, 0),
		Modules:   make([]scanner.ModuleReference, 0),
		Variables: make(map[string]interface{}),
		Warnings:  make([]scanner.ScanWarning, 0),
		Errors:    make([]scanner.ScanError, 0),
	}

	// Find all .tf files
	var tfFiles []string
	err := filepath.Walk(input.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".tf") {
			tfFiles = append(tfFiles, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Parse each file
	for _, file := range tfFiles {
		assets, modules, warnings, errs := s.parseFile(ctx, file, input.Path)
		result.Assets = append(result.Assets, assets...)
		result.Modules = append(result.Modules, modules...)
		result.Warnings = append(result.Warnings, warnings...)
		result.Errors = append(result.Errors, errs...)
	}

	// Load tfvars if present
	result.Variables = s.loadVariables(input.Path)

	return result, nil
}

func (s *Scanner) parseFile(ctx context.Context, file, basePath string) ([]types.RawAsset, []scanner.ModuleReference, []scanner.ScanWarning, []scanner.ScanError) {
	var assets []types.RawAsset
	var modules []scanner.ModuleReference
	var warnings []scanner.ScanWarning
	var errors []scanner.ScanError

	src, err := os.ReadFile(file)
	if err != nil {
		errors = append(errors, scanner.ScanError{
			File:    file,
			Message: fmt.Sprintf("failed to read file: %v", err),
			Err:     err,
		})
		return assets, modules, warnings, errors
	}

	hclFile, diags := s.parser.ParseHCL(src, file)
	if diags.HasErrors() {
		for _, diag := range diags {
			if diag.Severity == hcl.DiagError {
				line := 0
				if diag.Subject != nil {
					line = diag.Subject.Start.Line
				}
				errors = append(errors, scanner.ScanError{
					File:    file,
					Line:    line,
					Message: diag.Summary + ": " + diag.Detail,
				})
			}
		}
		return assets, modules, warnings, errors
	}

	// Extract blocks from the file body
	body := hclFile.Body
	content, _, _ := body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "resource", LabelNames: []string{"type", "name"}},
			{Type: "data", LabelNames: []string{"type", "name"}},
			{Type: "module", LabelNames: []string{"name"}},
			{Type: "variable", LabelNames: []string{"name"}},
			{Type: "locals"},
			{Type: "provider", LabelNames: []string{"name"}},
		},
	})

	relPath, _ := filepath.Rel(basePath, file)

	for _, block := range content.Blocks {
		switch block.Type {
		case "resource":
			asset := s.parseResource(block, relPath, false)
			if asset != nil {
				assets = append(assets, *asset)
			}
		case "data":
			asset := s.parseResource(block, relPath, true)
			if asset != nil {
				assets = append(assets, *asset)
			}
		case "module":
			mod := s.parseModule(block)
			if mod != nil {
				modules = append(modules, *mod)
			}
		}
	}

	return assets, modules, warnings, errors
}

func (s *Scanner) parseResource(block *hcl.Block, file string, isDataSource bool) *types.RawAsset {
	if len(block.Labels) < 2 {
		return nil
	}

	resourceType := block.Labels[0]
	resourceName := block.Labels[1]

	// Determine provider from resource type
	provider := types.ProviderUnknown
	if strings.HasPrefix(resourceType, "aws_") {
		provider = types.ProviderAWS
	} else if strings.HasPrefix(resourceType, "azurerm_") {
		provider = types.ProviderAzure
	} else if strings.HasPrefix(resourceType, "google_") {
		provider = types.ProviderGCP
	}

	// Extract attributes from block body
	attrs := s.extractAttributes(block.Body)

	address := types.ResourceAddress(fmt.Sprintf("%s.%s", resourceType, resourceName))
	if isDataSource {
		address = types.ResourceAddress(fmt.Sprintf("data.%s.%s", resourceType, resourceName))
	}

	line := 0
	if block.DefRange.Start.Line > 0 {
		line = block.DefRange.Start.Line
	}

	return &types.RawAsset{
		Address:      address,
		Provider:     provider,
		Type:         resourceType,
		Name:         resourceName,
		Attributes:   attrs,
		IsDataSource: isDataSource,
		SourceFile:   file,
		SourceLine:   line,
	}
}

func (s *Scanner) parseModule(block *hcl.Block) *scanner.ModuleReference {
	if len(block.Labels) < 1 {
		return nil
	}

	name := block.Labels[0]
	attrs := s.extractAttributes(block.Body)

	source := ""
	if src := attrs.Get("source"); src != nil {
		if srcStr, ok := src.(string); ok {
			source = srcStr
		}
	}

	version := ""
	if ver := attrs.Get("version"); ver != nil {
		if verStr, ok := ver.(string); ok {
			version = verStr
		}
	}

	return &scanner.ModuleReference{
		Key:     name,
		Source:  source,
		Version: version,
	}
}

func (s *Scanner) extractAttributes(body hcl.Body) types.Attributes {
	attrs := make(types.Attributes)

	// Get all attributes from the body
	content, _, _ := body.PartialContent(&hcl.BodySchema{})
	
	for name, attr := range content.Attributes {
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			// Mark as computed if we can't evaluate
			attrs[name] = types.Attribute{
				Value:      nil,
				IsComputed: true,
			}
			continue
		}

		attrs[name] = types.Attribute{
			Value: s.ctyToGo(val),
		}
	}

	return attrs
}

func (s *Scanner) ctyToGo(val interface{}) interface{} {
	// This is a simplified conversion
	// In production, use cty.Value methods properly
	return val
}

func (s *Scanner) loadVariables(basePath string) map[string]interface{} {
	vars := make(map[string]interface{})

	// Look for terraform.tfvars
	tfvarsPath := filepath.Join(basePath, "terraform.tfvars")
	if _, err := os.Stat(tfvarsPath); err == nil {
		// Parse tfvars file
		// Simplified - in production, properly parse HCL
	}

	// Look for *.auto.tfvars
	matches, _ := filepath.Glob(filepath.Join(basePath, "*.auto.tfvars"))
	for range matches {
		// Parse each file
	}

	return vars
}

func init() {
	// Register this scanner
	scanner.Register(NewScanner())
}
