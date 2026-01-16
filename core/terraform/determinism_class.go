// Package terraform - Function classification by determinism
// Functions are classified as PureDeterministic, PureContextual, or Impure.
// Impure functions block estimation in strict mode.
package terraform

// DeterminismClass indicates function determinism
type DeterminismClass int

const (
	// PureDeterministic - same inputs always produce same outputs
	PureDeterministic DeterminismClass = iota

	// PureContextual - deterministic given context (e.g., provider region)
	PureContextual

	// Impure - not deterministic (timestamp, uuid, external calls)
	Impure
)

// String returns the class name
func (c DeterminismClass) String() string {
	names := []string{"pure_deterministic", "pure_contextual", "impure"}
	if int(c) < len(names) {
		return names[c]
	}
	return "unknown"
}

// FunctionDeterminism maps Terraform functions to their determinism class
var FunctionDeterminism = map[string]DeterminismClass{
	// Pure Deterministic - safe
	"abs":           PureDeterministic,
	"ceil":          PureDeterministic,
	"floor":         PureDeterministic,
	"max":           PureDeterministic,
	"min":           PureDeterministic,
	"pow":           PureDeterministic,
	"signum":        PureDeterministic,
	"length":        PureDeterministic,
	"concat":        PureDeterministic,
	"contains":      PureDeterministic,
	"distinct":      PureDeterministic,
	"flatten":       PureDeterministic,
	"keys":          PureDeterministic,
	"values":        PureDeterministic,
	"lookup":        PureDeterministic,
	"merge":         PureDeterministic,
	"reverse":       PureDeterministic,
	"sort":          PureDeterministic,
	"zipmap":        PureDeterministic,
	"coalesce":      PureDeterministic,
	"compact":       PureDeterministic,
	"element":       PureDeterministic,
	"index":         PureDeterministic,
	"list":          PureDeterministic,
	"map":           PureDeterministic,
	"range":         PureDeterministic,
	"setintersection": PureDeterministic,
	"setproduct":    PureDeterministic,
	"setunion":      PureDeterministic,
	"slice":         PureDeterministic,
	"chomp":         PureDeterministic,
	"format":        PureDeterministic,
	"formatlist":    PureDeterministic,
	"indent":        PureDeterministic,
	"join":          PureDeterministic,
	"lower":         PureDeterministic,
	"upper":         PureDeterministic,
	"regex":         PureDeterministic,
	"regexall":      PureDeterministic,
	"replace":       PureDeterministic,
	"split":         PureDeterministic,
	"strrev":        PureDeterministic,
	"substr":        PureDeterministic,
	"title":         PureDeterministic,
	"trim":          PureDeterministic,
	"trimprefix":    PureDeterministic,
	"trimsuffix":    PureDeterministic,
	"trimspace":     PureDeterministic,
	"base64decode":  PureDeterministic,
	"base64encode":  PureDeterministic,
	"jsonencode":    PureDeterministic,
	"jsondecode":    PureDeterministic,
	"yamlencode":    PureDeterministic,
	"yamldecode":    PureDeterministic,
	"csvdecode":     PureDeterministic,
	"urlencode":     PureDeterministic,
	"md5":           PureDeterministic,
	"sha1":          PureDeterministic,
	"sha256":        PureDeterministic,
	"sha512":        PureDeterministic,
	"tolist":        PureDeterministic,
	"toset":         PureDeterministic,
	"tomap":         PureDeterministic,
	"tonumber":      PureDeterministic,
	"tostring":      PureDeterministic,
	"try":           PureDeterministic,
	"can":           PureDeterministic,

	// Pure Contextual - depends on provider/environment context
	"cidrhost":      PureContextual,
	"cidrnetmask":   PureContextual,
	"cidrsubnet":    PureContextual,
	"cidrsubnets":   PureContextual,
	"pathexpand":    PureContextual, // Depends on home directory

	// Impure - not deterministic, blocks in strict mode
	"timestamp":     Impure,
	"uuid":          Impure,
	"file":          Impure, // Reads from filesystem
	"fileexists":    Impure,
	"fileset":       Impure, // Directory listing
	"filebase64":    Impure,
	"templatefile":  Impure,
	"filemd5":       Impure,
	"filesha1":      Impure,
	"filesha256":    Impure,
	"filesha512":    Impure,
	"filebase64sha256": Impure,
	"filebase64sha512": Impure,
	"bcrypt":        Impure, // Random salt
	"rsadecrypt":    Impure, // External key
	"textencodebase64": PureDeterministic,
	"textdecodebase64": PureDeterministic,
}

// GetDeterminismClass returns the determinism class for a function
func GetDeterminismClass(function string) DeterminismClass {
	if class, ok := FunctionDeterminism[function]; ok {
		return class
	}
	// Unknown functions are treated as impure for safety
	return Impure
}

// IsPure returns true if function is deterministic
func IsPure(function string) bool {
	class := GetDeterminismClass(function)
	return class == PureDeterministic || class == PureContextual
}

// BlocksStrictMode returns true if function blocks strict mode
func BlocksStrictMode(function string) bool {
	return GetDeterminismClass(function) == Impure
}

// GetConfidenceImpact returns confidence impact for a function
func GetConfidenceImpact(function string) float64 {
	switch GetDeterminismClass(function) {
	case PureDeterministic:
		return 0.0 // No impact
	case PureContextual:
		return 0.1 // Small impact
	case Impure:
		return 0.5 // Large impact
	default:
		return 0.3 // Unknown
	}
}

// DeterminismAnalysis analyzes an expression for determinism
type DeterminismAnalysis struct {
	IsDeterministic bool
	ImpureFunctions []string
	ContextualFunctions []string
	TotalImpact     float64
}

// AnalyzeFunctions analyzes a list of function calls
func AnalyzeFunctions(functions []string) *DeterminismAnalysis {
	analysis := &DeterminismAnalysis{
		IsDeterministic: true,
		ImpureFunctions: []string{},
		ContextualFunctions: []string{},
		TotalImpact: 0.0,
	}

	for _, fn := range functions {
		class := GetDeterminismClass(fn)
		impact := GetConfidenceImpact(fn)
		analysis.TotalImpact += impact

		switch class {
		case Impure:
			analysis.IsDeterministic = false
			analysis.ImpureFunctions = append(analysis.ImpureFunctions, fn)
		case PureContextual:
			analysis.ContextualFunctions = append(analysis.ContextualFunctions, fn)
		}
	}

	return analysis
}

// ShouldBlock returns true if analysis should block estimation in strict mode
func (a *DeterminismAnalysis) ShouldBlock(strictMode bool) bool {
	return strictMode && len(a.ImpureFunctions) > 0
}
