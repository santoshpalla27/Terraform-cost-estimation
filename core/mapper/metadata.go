// Package mapper - Mapper metadata governance
// Enforces explicit contracts, metadata, and safety guarantees.
// Prevents mapper chaos as the system scales to 50–250+ mappers.
package mapper

import (
	"fmt"
)

// CostBehaviorType defines how a resource affects cost
type CostBehaviorType int

const (
	// CostDirect - Always billable (EC2, RDS, etc.)
	CostDirect CostBehaviorType = iota

	// CostUsageBased - Billable with usage data (Lambda, S3 requests)
	CostUsageBased

	// CostIndirect - Enables costs elsewhere but has no direct cost (VPC, IAM)
	CostIndirect

	// CostUnsupported - Not yet modeled, cost unknown
	CostUnsupported
)

// String returns the string representation
func (c CostBehaviorType) String() string {
	switch c {
	case CostDirect:
		return "direct"
	case CostUsageBased:
		return "usage_based"
	case CostIndirect:
		return "indirect"
	case CostUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

// CloudProvider identifies a cloud provider
type CloudProvider string

const (
	AWS   CloudProvider = "aws"
	Azure CloudProvider = "azure"
	GCP   CloudProvider = "gcp"
)

// MapperMetadata defines the contract for a mapper
type MapperMetadata struct {
	// ResourceType is the Terraform resource type (e.g., "aws_instance")
	ResourceType string

	// Cloud is the cloud provider
	Cloud CloudProvider

	// CostBehavior classifies cost impact
	CostBehavior CostBehaviorType

	// RequiresUsage indicates if usage data is needed for numeric cost
	RequiresUsage bool

	// CanBeSymbolic indicates if this mapper can produce symbolic costs
	// (e.g., due to unknown cardinality or missing usage)
	CanBeSymbolic bool

	// ConfidenceCeiling is the maximum confidence this mapper can produce (0.0-1.0)
	ConfidenceCeiling float64

	// HighImpact indicates if this resource is a significant cost driver
	HighImpact bool

	// Category is the service category (compute, storage, database, etc.)
	Category string

	// CostComponents lists the cost components this mapper produces
	CostComponents []string

	// Notes provides additional context
	Notes string
}

// Validate checks that metadata is complete and consistent
func (m MapperMetadata) Validate() error {
	// Required fields
	if m.ResourceType == "" {
		return fmt.Errorf("mapper missing resource type")
	}

	if m.Cloud == "" {
		return fmt.Errorf("mapper %s missing cloud provider", m.ResourceType)
	}

	// Confidence ceiling validation
	if m.ConfidenceCeiling <= 0 || m.ConfidenceCeiling > 1.0 {
		return fmt.Errorf("mapper %s has invalid confidence ceiling: %f (must be 0.0-1.0)",
			m.ResourceType, m.ConfidenceCeiling)
	}

	// Consistency rules
	if m.CostBehavior == CostDirect && !m.CanBeSymbolic {
		// Direct cost mappers CAN be symbolic if cardinality is unknown
		// This is actually valid - don't reject
	}

	if m.CostBehavior == CostIndirect && m.ConfidenceCeiling > 0.5 {
		// Indirect costs should have low confidence since they're $0
		// This is a warning, not an error
	}

	if m.CostBehavior == CostUsageBased && !m.RequiresUsage {
		return fmt.Errorf("mapper %s is usage-based but RequiresUsage is false",
			m.ResourceType)
	}

	if m.Category == "" {
		return fmt.Errorf("mapper %s missing category", m.ResourceType)
	}

	return nil
}

// IsNumeric returns true if this mapper produces numeric costs
func (m MapperMetadata) IsNumeric() bool {
	return m.CostBehavior == CostDirect || m.CostBehavior == CostUsageBased
}

// MustValidate panics if metadata is invalid
func (m MapperMetadata) MustValidate() {
	if err := m.Validate(); err != nil {
		panic(fmt.Sprintf("invalid mapper metadata: %v", err))
	}
}
