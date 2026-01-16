// Package terraform - Function classification
// Pure vs impure, deterministic vs environment-dependent
package terraform

// FunctionClass classifies Terraform functions
type FunctionClass int

const (
	FunctionPure          FunctionClass = iota // Pure, deterministic (length, concat)
	FunctionImpure                              // Side effects (file, timestamp)
	FunctionEnvironment                         // Depends on environment (pathexpand)
	FunctionRandom                              // Non-deterministic (uuid)
	FunctionExternal                            // External data (data sources)
)

// String returns the class name
func (c FunctionClass) String() string {
	switch c {
	case FunctionPure:
		return "pure"
	case FunctionImpure:
		return "impure"
	case FunctionEnvironment:
		return "environment"
	case FunctionRandom:
		return "random"
	case FunctionExternal:
		return "external"
	default:
		return "unknown"
	}
}

// IsDeterministic returns true if the function is deterministic
func (c FunctionClass) IsDeterministic() bool {
	return c == FunctionPure
}

// FunctionInfo contains metadata about a function
type FunctionInfo struct {
	Name             string
	Class            FunctionClass
	AffectsCardinality bool
	ConfidenceImpact float64
	Description      string
}

// FunctionClassifier classifies Terraform functions
type FunctionClassifier struct {
	functions map[string]*FunctionInfo
}

// NewFunctionClassifier creates a classifier with known functions
func NewFunctionClassifier() *FunctionClassifier {
	fc := &FunctionClassifier{
		functions: make(map[string]*FunctionInfo),
	}
	fc.registerBuiltins()
	return fc
}

func (fc *FunctionClassifier) registerBuiltins() {
	// Pure functions - deterministic, no side effects
	pureFunctions := []string{
		"abs", "ceil", "floor", "log", "max", "min", "pow", "signum",
		"chomp", "format", "formatlist", "indent", "join", "lower", "upper",
		"regex", "regexall", "replace", "split", "strrev", "substr", "title", "trim",
		"trimprefix", "trimsuffix", "trimspace",
		"chunklist", "coalesce", "coalescelist", "compact", "concat", "contains",
		"distinct", "element", "flatten", "index", "keys", "length", "list",
		"lookup", "map", "matchkeys", "merge", "range", "reverse", "setintersection",
		"setproduct", "setsubtract", "setunion", "slice", "sort", "sum", "transpose",
		"values", "zipmap",
		"alltrue", "anytrue", "can", "try",
		"tobool", "tolist", "tomap", "tonumber", "toset", "tostring",
		"base64decode", "base64encode", "base64gzip", "csvdecode", "jsondecode",
		"jsonencode", "textdecodebase64", "textencodebase64", "urlencode", "yamldecode",
		"yamlencode",
		"cidrhost", "cidrnetmask", "cidrsubnet", "cidrsubnets",
		"md5", "sha1", "sha256", "sha512", "bcrypt",
	}

	for _, name := range pureFunctions {
		fc.functions[name] = &FunctionInfo{
			Name:             name,
			Class:            FunctionPure,
			AffectsCardinality: false,
			ConfidenceImpact: 0,
			Description:      "Pure function",
		}
	}

	// Cardinality-affecting pure functions
	cardinalityFunctions := []string{"range", "setproduct", "flatten", "concat"}
	for _, name := range cardinalityFunctions {
		if f, ok := fc.functions[name]; ok {
			f.AffectsCardinality = true
		}
	}

	// Impure functions - file system access
	impureFunctions := map[string]string{
		"file":          "reads file from disk",
		"fileexists":    "checks file existence",
		"fileset":       "globs files on disk",
		"filebase64":    "reads file as base64",
		"templatefile":  "reads and renders template",
		"abspath":       "resolves absolute path",
	}

	for name, desc := range impureFunctions {
		fc.functions[name] = &FunctionInfo{
			Name:             name,
			Class:            FunctionImpure,
			AffectsCardinality: name == "fileset",
			ConfidenceImpact: 0.2,
			Description:      desc,
		}
	}

	// Environment-dependent functions
	envFunctions := map[string]string{
		"pathexpand": "expands ~ to home dir",
		"dirname":    "depends on path separator",
		"basename":   "depends on path separator",
	}

	for name, desc := range envFunctions {
		fc.functions[name] = &FunctionInfo{
			Name:             name,
			Class:            FunctionEnvironment,
			AffectsCardinality: false,
			ConfidenceImpact: 0.1,
			Description:      desc,
		}
	}

	// Random/non-deterministic functions
	randomFunctions := map[string]string{
		"uuid":     "generates random UUID",
		"bcrypt":   "generates random salt",
		"timestamp":"returns current time",
	}

	for name, desc := range randomFunctions {
		fc.functions[name] = &FunctionInfo{
			Name:             name,
			Class:            FunctionRandom,
			AffectsCardinality: false,
			ConfidenceImpact: 0.3,
			Description:      desc,
		}
	}
}

// Classify returns the classification for a function
func (fc *FunctionClassifier) Classify(name string) *FunctionInfo {
	if info, ok := fc.functions[name]; ok {
		return info
	}
	// Unknown function - assume impure
	return &FunctionInfo{
		Name:             name,
		Class:            FunctionImpure,
		AffectsCardinality: false,
		ConfidenceImpact: 0.15,
		Description:      "Unknown function",
	}
}

// IsDeterministic checks if a function is deterministic
func (fc *FunctionClassifier) IsDeterministic(name string) bool {
	info := fc.Classify(name)
	return info.Class.IsDeterministic()
}

// GetConfidenceImpact returns the confidence impact of using a function
func (fc *FunctionClassifier) GetConfidenceImpact(name string) float64 {
	info := fc.Classify(name)
	return info.ConfidenceImpact
}

// ClassifyExpression classifies an expression based on functions used
func (fc *FunctionClassifier) ClassifyExpression(expr string, functions []string) *ExpressionClassification {
	result := &ExpressionClassification{
		Expression:         expr,
		IsDeterministic:    true,
		Functions:          []string{},
		NonDeterministic:   []string{},
		ConfidenceImpact:   0,
		AffectsCardinality: false,
	}

	for _, fn := range functions {
		info := fc.Classify(fn)
		result.Functions = append(result.Functions, fn)

		if !info.Class.IsDeterministic() {
			result.IsDeterministic = false
			result.NonDeterministic = append(result.NonDeterministic, fn)
		}

		if info.ConfidenceImpact > result.ConfidenceImpact {
			result.ConfidenceImpact = info.ConfidenceImpact
		}

		if info.AffectsCardinality {
			result.AffectsCardinality = true
		}
	}

	return result
}

// ExpressionClassification is the result of classifying an expression
type ExpressionClassification struct {
	Expression         string
	IsDeterministic    bool
	Functions          []string
	NonDeterministic   []string
	ConfidenceImpact   float64
	AffectsCardinality bool
}

// Warning returns a warning message if non-deterministic
func (ec *ExpressionClassification) Warning() string {
	if ec.IsDeterministic {
		return ""
	}
	return "expression uses non-deterministic functions: " + joinStrings(ec.NonDeterministic)
}

func joinStrings(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += ", " + strs[i]
	}
	return result
}
