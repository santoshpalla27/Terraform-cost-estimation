// Package graph - Bypass prevention
// These functions exist SOLELY to prevent incorrect usage patterns.
// They panic immediately to ensure bypass attempts are caught in development.
package graph

// DEPRECATED CONSTRUCTORS - DO NOT USE
// These exist only to provide clear error messages if someone tries to bypass

// NewCostUnitFromAssetDirect is BLOCKED - use NewEnforcedCostUnit
func NewCostUnitFromAssetDirect(assetID string) {
	BlockBypassAttempt("NewCostUnitFromAssetDirect - cost units must be created via NewEnforcedCostUnit with dependency path")
}

// NewCostGraphWithoutDepGraph is BLOCKED - use NewEnforcedCostGraph
func NewCostGraphWithoutDepGraph() {
	BlockBypassAttempt("NewCostGraphWithoutDepGraph - cost graphs must derive from EnforcedAssetGraph with canonical dependency graph")
}

// ExpandWithoutCardinalityCheck is BLOCKED - use ExpansionGuard.MustExpand
func ExpandWithoutCardinalityCheck(address string, count int) {
	BlockBypassAttempt("ExpandWithoutCardinalityCheck - expansion must verify cardinality via ExpansionGuard")
}

// PriceWithoutProviderFreeze is BLOCKED - use PricingGate.MustGetProvider
func PriceWithoutProviderFreeze(resourceType string) {
	BlockBypassAttempt("PriceWithoutProviderFreeze - pricing must occur after provider finalization via PricingGate")
}

// AggregateConfidenceByAverage is BLOCKED - use confidence.AggregateConfidence
func AggregateConfidenceByAverage(values []float64) {
	BlockBypassAttempt("AggregateConfidenceByAverage - confidence must propagate pessimistically (MIN)")
}

// CreateNumericCostForUnknown is BLOCKED - use NewSymbolicCostUnit
func CreateNumericCostForUnknown(address string, cost float64) {
	BlockBypassAttempt("CreateNumericCostForUnknown - unknown cardinality must use symbolic costs only")
}

// SkipDependencyClosureForDiff is BLOCKED - use DependencyClosureDiff
func SkipDependencyClosureForDiff(addresses []string) {
	BlockBypassAttempt("SkipDependencyClosureForDiff - diffs must use dependency closure, not address matching")
}

// AcceptAssetWithoutDepNode is BLOCKED - assets must have dependency nodes
func AcceptAssetWithoutDepNode(assetID string) {
	BlockBypassAttempt("AcceptAssetWithoutDepNode - all assets must reference a DependencyNodeID")
}
