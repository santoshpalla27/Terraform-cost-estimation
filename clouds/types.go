// Package clouds - Common mapper interfaces and types
// This package defines the contract that all cloud mappers must implement.
// Mappers emit UsageVectors and CostUnits - they NEVER resolve prices.
package clouds

import (
	"fmt"
)

// CloudProvider identifies a cloud provider
type CloudProvider string

const (
	AWS   CloudProvider = "aws"
	Azure CloudProvider = "azure"
	GCP   CloudProvider = "gcp"
)

// AssetCostMapper is the interface all cloud resource mappers must implement
type AssetCostMapper interface {
	// Cloud returns the cloud provider
	Cloud() CloudProvider

	// ResourceType returns the Terraform resource type (e.g., "aws_instance")
	ResourceType() string

	// BuildUsage extracts usage from an asset
	// Returns symbolic usage if cardinality is unknown
	BuildUsage(asset AssetNode, ctx UsageContext) ([]UsageVector, error)

	// BuildCostUnits creates cost units from asset and usage
	// Returns symbolic cost if usage is symbolic
	BuildCostUnits(asset AssetNode, usage []UsageVector) ([]CostUnit, error)
}

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
	// ProviderID is the finalized provider identity
	ProviderID string

	// Alias is the provider alias (e.g., "aws.prod")
	Alias string

	// Region is the cloud region
	Region string

	// Account/Project/Subscription ID
	AccountID string
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
	// Profile is the usage profile (prod, staging, dev)
	Profile string

	// Overrides are explicit usage overrides
	Overrides map[string]interface{}

	// Confidence is the confidence in the usage values
	Confidence float64
}

// ResolveOrDefault returns an override value or default
func (ctx UsageContext) ResolveOrDefault(key string, defaultVal float64) float64 {
	if v, ok := ctx.Overrides[key].(float64); ok {
		return v
	}
	return defaultVal
}

// Metric is a usage metric type
type Metric string

const (
	MetricMonthlyHours    Metric = "monthly_hours"
	MetricMonthlyRequests Metric = "monthly_requests"
	MetricStorageGB       Metric = "storage_gb"
	MetricDataTransferGB  Metric = "data_transfer_gb"
	MetricIOPS            Metric = "iops"
	MetricThroughputMBps  Metric = "throughput_mbps"
)

// UsageVector represents a usage measurement
type UsageVector struct {
	// Metric is the type of usage
	Metric Metric

	// Value is the numeric value (nil if symbolic)
	Value *float64

	// IsSymbolic indicates this usage cannot be numerically resolved
	IsSymbolic bool

	// SymbolicReason explains why usage is symbolic
	SymbolicReason string

	// Confidence in this usage value (0.0 to 1.0)
	Confidence float64
}

// NewUsageVector creates a concrete usage vector
func NewUsageVector(metric Metric, value float64, confidence float64) UsageVector {
	return UsageVector{
		Metric:     metric,
		Value:      &value,
		Confidence: confidence,
	}
}

// SymbolicUsage creates a symbolic usage vector
func SymbolicUsage(metric Metric, reason string) UsageVector {
	return UsageVector{
		Metric:         metric,
		IsSymbolic:     true,
		SymbolicReason: reason,
		Confidence:     0,
	}
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
func (vs UsageVectors) Get(metric Metric) (float64, bool) {
	for _, v := range vs {
		if v.Metric == metric && v.Value != nil {
			return *v.Value, true
		}
	}
	return 0, false
}

// CostUnit represents a billable cost component
type CostUnit struct {
	// Name of the cost component (e.g., "compute", "storage")
	Name string

	// Measure is the billing unit (e.g., "hours", "GB-months")
	Measure string

	// Quantity is the usage quantity (nil if symbolic)
	Quantity *float64

	// RateKey identifies the pricing rate
	RateKey RateKey

	// IsSymbolic indicates this cost cannot be numerically resolved
	IsSymbolic bool

	// SymbolicReason explains why cost is symbolic
	SymbolicReason string

	// Confidence in this cost (0.0 to 1.0)
	Confidence float64
}

// NewCostUnit creates a concrete cost unit
func NewCostUnit(name, measure string, quantity float64, rateKey RateKey, confidence float64) CostUnit {
	return CostUnit{
		Name:       name,
		Measure:    measure,
		Quantity:   &quantity,
		RateKey:    rateKey,
		Confidence: confidence,
	}
}

// SymbolicCost creates a symbolic cost unit
func SymbolicCost(name, reason string) CostUnit {
	return CostUnit{
		Name:           name,
		IsSymbolic:     true,
		SymbolicReason: reason,
		Confidence:     0,
	}
}

// RateKey identifies a pricing rate
type RateKey struct {
	// Provider is the finalized provider ID
	Provider string

	// Service is the cloud service (e.g., "EC2", "S3")
	Service string

	// Region is the cloud region
	Region string

	// Attributes are pricing dimensions
	Attributes map[string]string
}

// String returns a unique key string
func (k RateKey) String() string {
	return fmt.Sprintf("%s/%s/%s/%v", k.Provider, k.Service, k.Region, k.Attributes)
}
