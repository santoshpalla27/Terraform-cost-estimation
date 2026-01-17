// Package mapper - Mapper metadata governance
// Enforces explicit contracts, metadata, and safety guarantees.
// Prevents mapper chaos as the system scales to 50–250+ mappers.
package mapper

import (
	"fmt"
)

// CoverageTier classifies mappers by cost coverage capability
type CoverageTier int

const (
	// Tier1Numeric - Must produce numeric cost (EC2, RDS, etc.)
	Tier1Numeric CoverageTier = iota

	// Tier2Symbolic - Numeric only with usage data, otherwise symbolic
	Tier2Symbolic

	// Tier3Indirect - Never numeric, zero-cost graph node (VPC, IAM)
	Tier3Indirect
)

// String returns the string representation
func (t CoverageTier) String() string {
	switch t {
	case Tier1Numeric:
		return "tier1_numeric"
	case Tier2Symbolic:
		return "tier2_symbolic"
	case Tier3Indirect:
		return "tier3_indirect"
	default:
		return "unknown"
	}
}

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
// Every field is required - no defaults, no optionals
type MapperMetadata struct {
	// ResourceType is the Terraform resource type (e.g., "aws_instance")
	ResourceType string

	// Cloud is the cloud provider
	Cloud CloudProvider

	// Tier classifies the mapper's coverage capability
	Tier CoverageTier

	// CostBehavior classifies cost impact
	CostBehavior CostBehaviorType

	// RequiresUsage indicates if usage data is needed for numeric cost
	RequiresUsage bool

	// CanBeSymbolic indicates if this mapper can produce symbolic costs
	// Tier1 mappers: should be false (but can be true if cardinality is unknown)
	// Tier2 mappers: must be true
	// Tier3 mappers: always true (they're always symbolic)
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
// Returns an error describing all validation failures
func (m MapperMetadata) Validate() error {
	// Required fields - FAIL FAST
	if m.ResourceType == "" {
		return fmt.Errorf("mapper missing resource type")
	}

	if m.Cloud == "" {
		return fmt.Errorf("mapper %s missing cloud provider", m.ResourceType)
	}

	if m.Category == "" {
		return fmt.Errorf("mapper %s missing category", m.ResourceType)
	}

	// Confidence ceiling validation
	if m.ConfidenceCeiling <= 0 || m.ConfidenceCeiling > 1.0 {
		return fmt.Errorf("mapper %s has invalid confidence ceiling: %f (must be 0.0-1.0)",
			m.ResourceType, m.ConfidenceCeiling)
	}

	// TIER-BASED INVARIANTS (NON-NEGOTIABLE)

	// Tier1 rules: must produce numeric costs
	if m.Tier == Tier1Numeric {
		// Tier1 can be symbolic ONLY due to unknown cardinality
		// But should not be marked as CanBeSymbolic by default
		if m.CostBehavior == CostIndirect {
			return fmt.Errorf("mapper %s: Tier1 cannot have CostIndirect behavior", m.ResourceType)
		}
	}

	// Tier2 rules: symbolic by default, numeric with usage
	if m.Tier == Tier2Symbolic {
		if !m.CanBeSymbolic {
			return fmt.Errorf("mapper %s: Tier2 must have CanBeSymbolic=true", m.ResourceType)
		}
	}

	// Tier3 rules: NEVER numeric
	if m.Tier == Tier3Indirect {
		if m.CostBehavior != CostIndirect {
			return fmt.Errorf("mapper %s: Tier3 must have CostIndirect behavior", m.ResourceType)
		}
		if !m.CanBeSymbolic {
			return fmt.Errorf("mapper %s: Tier3 must have CanBeSymbolic=true", m.ResourceType)
		}
	}

	// Usage-based consistency
	if m.CostBehavior == CostUsageBased && !m.RequiresUsage {
		return fmt.Errorf("mapper %s is usage-based but RequiresUsage is false", m.ResourceType)
	}

	return nil
}

// IsNumeric returns true if this mapper produces numeric costs
func (m MapperMetadata) IsNumeric() bool {
	return m.Tier == Tier1Numeric && !m.CanBeSymbolic
}

// CanProduceNumeric returns true if this mapper CAN produce numeric costs
func (m MapperMetadata) CanProduceNumeric() bool {
	return m.Tier == Tier1Numeric || (m.Tier == Tier2Symbolic && m.RequiresUsage)
}

// MustValidate panics if metadata is invalid
// Call this at mapper registration time to fail fast
func (m MapperMetadata) MustValidate() {
	if err := m.Validate(); err != nil {
		panic(fmt.Sprintf("FATAL: invalid mapper metadata: %v", err))
	}
}

// TierFromCatalog creates metadata tier from catalog tier
func TierFromCatalog(tier int) CoverageTier {
	switch tier {
	case 0:
		return Tier1Numeric
	case 1:
		return Tier2Symbolic
	case 2:
		return Tier3Indirect
	default:
		return Tier2Symbolic // Default to symbolic for safety
	}
}
