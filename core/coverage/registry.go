// Package coverage - Resource cost behavior classification
// This registry classifies every Terraform resource by cost behavior.
// Mappers exist ONLY for resources that materially affect spend.
// Unsupported resources are EXPLICIT, not hidden.
package coverage

// CostBehavior classifies how a resource affects cost
type CostBehavior string

const (
	// CostDirect - Always billable (EC2, RDS, etc.)
	CostDirect CostBehavior = "direct"

	// CostUsageBased - Billable with usage data (Lambda, S3 requests)
	CostUsageBased CostBehavior = "usage_based"

	// CostIndirect - Enables cost elsewhere but has no direct cost (VPC, IAM)
	CostIndirect CostBehavior = "indirect"

	// CostFree - Explicitly free resources (data sources, some configs)
	CostFree CostBehavior = "free"

	// CostUnsupported - Not yet modeled, cost unknown
	CostUnsupported CostBehavior = "unsupported"
)

// ResourceCostProfile defines the cost behavior of a resource type
type ResourceCostProfile struct {
	// ResourceType is the Terraform resource type
	ResourceType string

	// Behavior classifies cost impact
	Behavior CostBehavior

	// MapperExists indicates if a cost mapper is implemented
	MapperExists bool

	// EstimatedSpendContribution is % of typical cloud spend (0-100)
	EstimatedSpendContribution float64

	// Notes explains the classification
	Notes string
}

// CoverageLevel indicates mapper implementation status
type CoverageLevel string

const (
	CoverageFull     CoverageLevel = "full"     // Complete mapper with all components
	CoveragePartial  CoverageLevel = "partial"  // Mapper exists but incomplete
	CoverageSymbolic CoverageLevel = "symbolic" // Known but not priced
	CoverageNone     CoverageLevel = "none"     // Not implemented
)

// Registry holds all resource cost profiles
type Registry struct {
	profiles map[string]*ResourceCostProfile
}

// NewRegistry creates a new coverage registry
func NewRegistry() *Registry {
	return &Registry{
		profiles: make(map[string]*ResourceCostProfile),
	}
}

// Register adds a profile to the registry
func (r *Registry) Register(profile ResourceCostProfile) {
	r.profiles[profile.ResourceType] = &profile
}

// Get returns a profile for a resource type
func (r *Registry) Get(resourceType string) (*ResourceCostProfile, bool) {
	profile, ok := r.profiles[resourceType]
	return profile, ok
}

// GetBehavior returns the cost behavior for a resource type
func (r *Registry) GetBehavior(resourceType string) CostBehavior {
	if profile, ok := r.profiles[resourceType]; ok {
		return profile.Behavior
	}
	return CostUnsupported
}

// IsSupported returns true if a mapper exists for this resource
func (r *Registry) IsSupported(resourceType string) bool {
	if profile, ok := r.profiles[resourceType]; ok {
		return profile.MapperExists
	}
	return false
}

// ListByBehavior returns all resources with a given behavior
func (r *Registry) ListByBehavior(behavior CostBehavior) []string {
	var result []string
	for rt, profile := range r.profiles {
		if profile.Behavior == behavior {
			result = append(result, rt)
		}
	}
	return result
}

// ListSupported returns all resources with mappers
func (r *Registry) ListSupported() []string {
	var result []string
	for rt, profile := range r.profiles {
		if profile.MapperExists {
			result = append(result, rt)
		}
	}
	return result
}

// Stats returns registry statistics
func (r *Registry) Stats() RegistryStats {
	stats := RegistryStats{}
	for _, profile := range r.profiles {
		stats.Total++
		switch profile.Behavior {
		case CostDirect:
			stats.Direct++
		case CostUsageBased:
			stats.UsageBased++
		case CostIndirect:
			stats.Indirect++
		case CostFree:
			stats.Free++
		case CostUnsupported:
			stats.Unsupported++
		}
		if profile.MapperExists {
			stats.WithMappers++
			stats.EstimatedCoverage += profile.EstimatedSpendContribution
		}
	}
	return stats
}

// RegistryStats holds registry statistics
type RegistryStats struct {
	Total             int
	Direct            int
	UsageBased        int
	Indirect          int
	Free              int
	Unsupported       int
	WithMappers       int
	EstimatedCoverage float64 // % of spend covered
}
