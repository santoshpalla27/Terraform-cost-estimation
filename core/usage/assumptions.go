// Package usage - Tracked assumptions
// Every default MUST be recorded as an assumption and reduce confidence.
package usage

import (
	"fmt"
	"terraform-cost/core/confidence"
)

// Assumption represents a usage assumption made during estimation
type Assumption struct {
	// What was assumed
	Component string
	Attribute string

	// What value was used
	Value     interface{}
	Unit      string

	// Why this was assumed
	Source    AssumptionSource
	Reason    string

	// Confidence impact
	ConfidenceImpact float64

	// Is this overrideable?
	Overrideable     bool
	OverrideKey      string // Key to use in usage file
}

// AssumptionSource indicates where the assumption came from
type AssumptionSource int

const (
	AssumptionFromDefault    AssumptionSource = iota // Hardcoded default
	AssumptionFromHeuristic                           // Calculated from other attributes
	AssumptionFromProfile                             // Usage profile
	AssumptionFromHistorical                          // Historical data
)

// String returns the source name
func (s AssumptionSource) String() string {
	switch s {
	case AssumptionFromDefault:
		return "default"
	case AssumptionFromHeuristic:
		return "heuristic"
	case AssumptionFromProfile:
		return "profile"
	case AssumptionFromHistorical:
		return "historical"
	default:
		return "unknown"
	}
}

// AssumptionTracker tracks all assumptions made during estimation
type AssumptionTracker struct {
	assumptions []*Assumption
	byComponent map[string][]*Assumption
}

// NewAssumptionTracker creates a new tracker
func NewAssumptionTracker() *AssumptionTracker {
	return &AssumptionTracker{
		assumptions: []*Assumption{},
		byComponent: make(map[string][]*Assumption),
	}
}

// RecordDefault records a default value assumption
// This ALWAYS reduces confidence
func (t *AssumptionTracker) RecordDefault(component, attribute string, value interface{}, unit string, impact float64) *Assumption {
	a := &Assumption{
		Component:        component,
		Attribute:        attribute,
		Value:            value,
		Unit:             unit,
		Source:           AssumptionFromDefault,
		Reason:           "using default value",
		ConfidenceImpact: impact,
		Overrideable:     true,
		OverrideKey:      fmt.Sprintf("%s.%s", component, attribute),
	}

	t.assumptions = append(t.assumptions, a)
	t.byComponent[component] = append(t.byComponent[component], a)
	return a
}

// RecordHeuristic records a heuristic-based assumption
func (t *AssumptionTracker) RecordHeuristic(component, attribute string, value interface{}, unit, reason string, impact float64) *Assumption {
	a := &Assumption{
		Component:        component,
		Attribute:        attribute,
		Value:            value,
		Unit:             unit,
		Source:           AssumptionFromHeuristic,
		Reason:           reason,
		ConfidenceImpact: impact,
		Overrideable:     true,
		OverrideKey:      fmt.Sprintf("%s.%s", component, attribute),
	}

	t.assumptions = append(t.assumptions, a)
	t.byComponent[component] = append(t.byComponent[component], a)
	return a
}

// All returns all assumptions
func (t *AssumptionTracker) All() []*Assumption {
	return t.assumptions
}

// ForComponent returns assumptions for a component
func (t *AssumptionTracker) ForComponent(component string) []*Assumption {
	return t.byComponent[component]
}

// TotalConfidenceImpact returns the total confidence impact
func (t *AssumptionTracker) TotalConfidenceImpact() float64 {
	total := 0.0
	for _, a := range t.assumptions {
		total += a.ConfidenceImpact
	}
	return total
}

// ApplyToTracker applies all assumptions to a confidence tracker
func (t *AssumptionTracker) ApplyToTracker(ct *confidence.ConfidenceTracker) {
	for _, a := range t.assumptions {
		ct.Apply("default_usage", fmt.Sprintf("%s: %v %s", a.OverrideKey, a.Value, a.Unit))
	}
}

// Count returns the number of assumptions
func (t *AssumptionTracker) Count() int {
	return len(t.assumptions)
}

// DefaultAssumptionImpacts defines standard confidence impacts
var DefaultAssumptionImpacts = map[string]float64{
	// Compute
	"lambda.memory_size":         0.05,
	"lambda.invocations":         0.20, // High impact - very variable
	"lambda.duration":            0.15,
	"ec2.cpu_utilization":        0.10,
	"ec2.data_transfer_gb":       0.20,
	"ecs.desired_count":          0.10,
	
	// Storage
	"s3.storage_gb":              0.25, // High impact - very variable
	"s3.requests":                0.20,
	"ebs.iops":                   0.10,
	"rds.storage_gb":             0.15,
	
	// Network
	"nat.data_processed_gb":      0.25, // High impact
	"lb.processed_bytes":         0.20,
	"data_transfer.egress_gb":    0.25,
	
	// Cache
	"elasticache.data_tiering":   0.05,
	
	// Default for unknown
	"default":                    0.15,
}

// GetDefaultImpact returns the standard impact for an assumption
func GetDefaultImpact(key string) float64 {
	if impact, ok := DefaultAssumptionImpacts[key]; ok {
		return impact
	}
	return DefaultAssumptionImpacts["default"]
}

// CommonDefaults defines common default values
var CommonDefaults = map[string]interface{}{
	// Lambda
	"aws_lambda_function.memory_size":  128,
	"aws_lambda_function.timeout":      3,
	"aws_lambda_function.invocations":  1000000, // 1M/month
	"aws_lambda_function.duration_ms":  100,

	// EC2
	"aws_instance.cpu_utilization":     50.0,  // %
	"aws_instance.data_transfer_gb":    100.0, // GB/month

	// ECS
	"aws_ecs_service.desired_count":    1,
	"aws_ecs_task.cpu_utilization":     50.0,

	// S3
	"aws_s3_bucket.storage_gb":         100.0, // GB
	"aws_s3_bucket.put_requests":       10000,
	"aws_s3_bucket.get_requests":       100000,

	// RDS
	"aws_db_instance.storage_gb":       20.0,
	"aws_db_instance.iops":             1000,

	// NAT
	"aws_nat_gateway.data_processed_gb": 100.0,

	// ALB
	"aws_lb.processed_bytes":           1000000000, // 1 GB
	"aws_lb.new_connections":           1000,
	"aws_lb.active_connections":        100,
}

// GetDefault returns a default value for a resource attribute
func GetDefault(resourceType, attribute string) (interface{}, bool) {
	key := resourceType + "." + attribute
	if val, ok := CommonDefaults[key]; ok {
		return val, true
	}
	return nil, false
}
