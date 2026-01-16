// Package hcl provides Terraform HCL parsing with DEFERRED evaluation.
// Expressions are NOT evaluated immediately - they are captured as unevaluated
// and resolved only after variables, locals, and references are available.
package hcl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"terraform-cost/core/scanner"
	"terraform-cost/core/types"
)

// Scanner implements the scanner.Scanner interface for Terraform HCL
// CRITICAL: This scanner does NOT evaluate expressions.
// It captures expressions as unevaluated for later resolution.
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

// Scan parses HCL files and returns raw assets with UNEVALUATED expressions
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

	// Parse each file - DO NOT EVALUATE
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
			asset := s.parseResourceDeferred(block, relPath, false)
			if asset != nil {
				assets = append(assets, *asset)
			}
		case "data":
			asset := s.parseResourceDeferred(block, relPath, true)
			if asset != nil {
				assets = append(assets, *asset)
			}
		case "module":
			mod := s.parseModuleDeferred(block)
			if mod != nil {
				modules = append(modules, *mod)
			}
		}
	}

	return assets, modules, warnings, errors
}

// parseResourceDeferred captures expressions WITHOUT evaluating them
func (s *Scanner) parseResourceDeferred(block *hcl.Block, file string, isDataSource bool) *types.RawAsset {
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

	// Extract attributes as UNEVALUATED expressions
	attrs := s.extractAttributesDeferred(block.Body)

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

func (s *Scanner) parseModuleDeferred(block *hcl.Block) *scanner.ModuleReference {
	if len(block.Labels) < 1 {
		return nil
	}

	name := block.Labels[0]
	attrs := s.extractAttributesDeferred(block.Body)

	source := ""
	if src := attrs.Get("source"); src != nil {
		if attr, ok := src.(types.Attribute); ok && !attr.IsComputed && !attr.IsUnknown {
			if srcStr, ok := attr.Value.(string); ok {
				source = srcStr
			}
		}
	}

	version := ""
	if ver := attrs.Get("version"); ver != nil {
		if attr, ok := ver.(types.Attribute); ok && !attr.IsComputed && !attr.IsUnknown {
			if verStr, ok := attr.Value.(string); ok {
				version = verStr
			}
		}
	}

	return &scanner.ModuleReference{
		Key:     name,
		Source:  source,
		Version: version,
	}
}

// extractAttributesDeferred captures expressions WITHOUT evaluating them
// This is the CRITICAL fix - we do NOT call attr.Expr.Value(nil)
func (s *Scanner) extractAttributesDeferred(body hcl.Body) types.Attributes {
	attrs := make(types.Attributes)

	// Get all attributes from the body
	content, _, _ := body.PartialContent(&hcl.BodySchema{})

	for name, attr := range content.Attributes {
		// Analyze the expression to determine if it needs context
		exprInfo := s.analyzeExpression(attr.Expr)

		if exprInfo.IsLiteral {
			// Safe to evaluate literals immediately
			val, diags := attr.Expr.Value(nil)
			if !diags.HasErrors() {
				attrs[name] = types.Attribute{
					Value:      s.ctyToGo(val),
					IsComputed: false,
					IsUnknown:  false,
				}
				continue
			}
		}

		// Expression requires context - mark as unevaluated
		attrs[name] = types.Attribute{
			Value:             nil,
			IsComputed:        exprInfo.RequiresContext,
			IsUnknown:         exprInfo.HasUnknownRefs,
			Expression:        s.expressionToString(attr.Expr),
			ExpressionType:    exprInfo.Type,
			References:        exprInfo.References,
			ConfidenceImpact:  exprInfo.ConfidenceImpact,
		}
	}

	return attrs
}

// ExpressionInfo describes an unevaluated expression
type ExpressionInfo struct {
	IsLiteral        bool
	RequiresContext  bool
	HasUnknownRefs   bool
	Type             string   // "literal", "variable", "local", "reference", "function", "conditional"
	References       []string // Referenced addresses
	ConfidenceImpact float64  // How much this reduces confidence (0.0 - 1.0)
}

// analyzeExpression determines what kind of expression this is
func (s *Scanner) analyzeExpression(expr hcl.Expression) ExpressionInfo {
	info := ExpressionInfo{
		IsLiteral:        true,
		RequiresContext:  false,
		HasUnknownRefs:   false,
		Type:             "literal",
		References:       []string{},
		ConfidenceImpact: 0.0,
	}

	// Get all variable references
	refs := expr.Variables()
	if len(refs) > 0 {
		info.IsLiteral = false
		info.RequiresContext = true
		info.ConfidenceImpact = 0.1 // Base impact for having references

		for _, ref := range refs {
			refStr := formatTraversal(ref)
			info.References = append(info.References, refStr)

			// Classify reference type
			if len(ref) > 0 {
				root := ref.RootName()
				switch root {
				case "var":
					info.Type = "variable"
					info.ConfidenceImpact += 0.1 // Variable adds uncertainty
				case "local":
					info.Type = "local"
					// Locals are resolvable
				case "count":
					info.Type = "count_reference"
					info.ConfidenceImpact += 0.2 // Count adds more uncertainty
				case "each":
					info.Type = "for_each_reference"
					info.ConfidenceImpact += 0.2
				case "data":
					info.Type = "data_source"
					info.HasUnknownRefs = true // Data sources are runtime
					info.ConfidenceImpact += 0.3
				default:
					// Resource reference
					info.Type = "resource_reference"
					info.HasUnknownRefs = true
					info.ConfidenceImpact += 0.3
				}
			}
		}
	}

	// Check for function calls
	if synExpr, ok := expr.(*hclsyntax.FunctionCallExpr); ok {
		info.IsLiteral = false
		info.RequiresContext = true
		info.Type = "function:" + synExpr.Name
		info.ConfidenceImpact += 0.1
	}

	// Check for conditional
	if _, ok := expr.(*hclsyntax.ConditionalExpr); ok {
		info.IsLiteral = false
		info.RequiresContext = true
		info.Type = "conditional"
		info.ConfidenceImpact += 0.15
	}

	// Cap confidence impact
	if info.ConfidenceImpact > 0.5 {
		info.ConfidenceImpact = 0.5
	}

	return info
}

func formatTraversal(traversal hcl.Traversal) string {
	parts := make([]string, 0, len(traversal))
	for _, t := range traversal {
		switch tt := t.(type) {
		case hcl.TraverseRoot:
			parts = append(parts, tt.Name)
		case hcl.TraverseAttr:
			parts = append(parts, "."+tt.Name)
		case hcl.TraverseIndex:
			parts = append(parts, "[...]")
		}
	}
	return strings.Join(parts, "")
}

func (s *Scanner) expressionToString(expr hcl.Expression) string {
	// Get the source range and extract the text
	rng := expr.Range()
	return fmt.Sprintf("<%s:%d-%d>", rng.Filename, rng.Start.Line, rng.End.Line)
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
