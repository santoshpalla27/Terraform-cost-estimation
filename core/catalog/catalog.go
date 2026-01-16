// Package catalog - Authoritative cloud resource catalog
// Defines the canonical list of resources with tier classification.
// This is the source of truth for coverage.
package catalog

// CoverageTier classifies resources by cost behavior
type CoverageTier int

const (
	// Tier1Numeric - must have numeric cost mapper
	Tier1Numeric CoverageTier = iota
	// Tier2Symbolic - numeric only with usage data
	Tier2Symbolic
	// Tier3Indirect - never numeric, zero-cost graph node
	Tier3Indirect
)

// String returns string representation
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

// CostBehavior classifies how a resource affects cost
type CostBehavior int

const (
	// CostDirect - always billable
	CostDirect CostBehavior = iota
	// CostUsageBased - billable with usage
	CostUsageBased
	// CostIndirect - enables other costs, no direct billing
	CostIndirect
	// CostUnsupported - not yet modeled
	CostUnsupported
)

// String returns string representation
func (b CostBehavior) String() string {
	switch b {
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

// CloudProvider identifies a cloud
type CloudProvider string

const (
	AWS   CloudProvider = "aws"
	Azure CloudProvider = "azure"
	GCP   CloudProvider = "gcp"
)

// ResourceEntry is a catalog entry for a resource type
type ResourceEntry struct {
	Cloud         CloudProvider
	ResourceType  string
	Tier          CoverageTier
	Behavior      CostBehavior
	Category      string
	RequiresUsage bool
	MapperExists  bool
	Notes         string
}

// Catalog is the authoritative resource catalog
type Catalog struct {
	entries map[string]*ResourceEntry
}

// NewCatalog creates a new catalog
func NewCatalog() *Catalog {
	return &Catalog{
		entries: make(map[string]*ResourceEntry),
	}
}

// Register adds a resource to the catalog
func (c *Catalog) Register(entry ResourceEntry) {
	key := string(entry.Cloud) + ":" + entry.ResourceType
	c.entries[key] = &entry
}

// Get returns a resource entry
func (c *Catalog) Get(cloud CloudProvider, resourceType string) (*ResourceEntry, bool) {
	key := string(cloud) + ":" + resourceType
	entry, ok := c.entries[key]
	return entry, ok
}

// GetTier returns the tier for a resource
func (c *Catalog) GetTier(cloud CloudProvider, resourceType string) CoverageTier {
	if entry, ok := c.Get(cloud, resourceType); ok {
		return entry.Tier
	}
	return Tier2Symbolic // Default to symbolic for unknown
}

// ListByTier returns all resources in a tier
func (c *Catalog) ListByTier(cloud CloudProvider, tier CoverageTier) []string {
	var result []string
	for _, entry := range c.entries {
		if entry.Cloud == cloud && entry.Tier == tier {
			result = append(result, entry.ResourceType)
		}
	}
	return result
}

// Stats returns catalog statistics
func (c *Catalog) Stats() CatalogStats {
	stats := CatalogStats{
		ByCloud: make(map[CloudProvider]CloudStats),
	}
	
	for _, entry := range c.entries {
		stats.Total++
		
		cloudStats := stats.ByCloud[entry.Cloud]
		cloudStats.Total++
		
		switch entry.Tier {
		case Tier1Numeric:
			cloudStats.Tier1++
		case Tier2Symbolic:
			cloudStats.Tier2++
		case Tier3Indirect:
			cloudStats.Tier3++
		}
		
		if entry.MapperExists {
			cloudStats.WithMappers++
		}
		
		stats.ByCloud[entry.Cloud] = cloudStats
	}
	
	return stats
}

// CatalogStats holds catalog statistics
type CatalogStats struct {
	Total   int
	ByCloud map[CloudProvider]CloudStats
}

// CloudStats holds per-cloud statistics
type CloudStats struct {
	Total       int
	Tier1       int
	Tier2       int
	Tier3       int
	WithMappers int
}
