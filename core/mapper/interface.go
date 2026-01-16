// Package mapper - Mapper interface definition
// All cloud mappers must implement this interface.
package mapper

// AssetNode represents a Terraform resource in the asset graph
type AssetNode struct {
	// Address is the Terraform address (e.g., "aws_instance.web")
	Address string

	// Type is the resource type (e.g., "aws_instance")
	Type string

	// Attributes are the resolved Terraform attributes
	Attributes map[string]interface{}

	// ProviderContext contains provider-level information
	ProviderContext ProviderContext

	// Cardinality indicates whether the instance count is known
	Cardinality Cardinality

	// InstanceKey for expanded resources (count/for_each)
	InstanceKey string
}

// Attr returns an attribute value as string
func (a AssetNode) Attr(key string) string {
	if v, ok := a.Attributes[key].(string); ok {
		return v
	}
	return ""
}

// AttrFloat returns an attribute value as float64
func (a AssetNode) AttrFloat(key string, defaultVal float64) float64 {
	if v, ok := a.Attributes[key].(float64); ok {
		return v
	}
	if v, ok := a.Attributes[key].(int); ok {
		return float64(v)
	}
	return defaultVal
}

// AttrInt returns an attribute value as int
func (a AssetNode) AttrInt(key string, defaultVal int) int {
	if v, ok := a.Attributes[key].(int); ok {
		return v
	}
	if v, ok := a.Attributes[key].(float64); ok {
		return int(v)
	}
	return defaultVal
}

// AttrBool returns an attribute value as bool
func (a AssetNode) AttrBool(key string, defaultVal bool) bool {
	if v, ok := a.Attributes[key].(bool); ok {
		return v
	}
	return defaultVal
}

// ProviderContext contains provider-level information
type ProviderContext struct {
	ProviderID string
	Alias      string
	Region     string
	AccountID  string
}

// Cardinality represents whether instance count is known
type Cardinality struct {
	IsKnown bool
	Count   int
	Reason  string
}

// IsUnknown returns true if cardinality is not deterministic
func (c Cardinality) IsUnknown() bool {
	return !c.IsKnown
}

// UsageContext provides context for usage resolution
type UsageContext struct {
	Profile    string
	Overrides  map[string]interface{}
	Confidence float64
}

// ResolveOrDefault returns an override value or default
func (ctx UsageContext) ResolveOrDefault(key string, defaultVal float64) float64 {
	if v, ok := ctx.Overrides[key].(float64); ok {
		return v
	}
	return defaultVal
}

// UsageVector represents a usage measurement
type UsageVector struct {
	Metric         string
	Value          *float64
	IsSymbolic     bool
	SymbolicReason string
	Confidence     float64
}

// UsageVectors is a collection of usage vectors
type UsageVectors []UsageVector

// IsSymbolic returns true if any usage is symbolic
func (vs UsageVectors) IsSymbolic() bool {
	for _, v := range vs {
		if v.IsSymbolic {
			return true
		}
	}
	return false
}

// Get returns the value for a metric
func (vs UsageVectors) Get(metric string) (float64, bool) {
	for _, v := range vs {
		if v.Metric == metric && v.Value != nil {
			return *v.Value, true
		}
	}
	return 0, false
}

// CostUnit represents a billable cost component
type CostUnit struct {
	Name           string
	Measure        string
	Quantity       *float64
	RateKey        RateKey
	IsSymbolic     bool
	SymbolicReason string
	Confidence     float64
}

// RateKey identifies a pricing rate
type RateKey struct {
	Provider   string
	Service    string
	Region     string
	Attributes map[string]string
}

// AssetCostMapper is the interface all cloud resource mappers must implement
type AssetCostMapper interface {
	// Metadata returns the mapper's metadata contract
	Metadata() MapperMetadata

	// BuildUsage extracts usage from an asset
	// Returns symbolic usage if cardinality is unknown
	BuildUsage(asset AssetNode, ctx UsageContext) (UsageVectors, error)

	// BuildCostUnits creates cost units from asset and usage
	// Returns symbolic cost if usage is symbolic
	BuildCostUnits(asset AssetNode, usage UsageVectors) ([]CostUnit, error)
}
