// Package terraform - Data source estimation barriers
// Data sources create HARD uncertainty. They cannot be estimated deterministically.
package terraform

import (
	"fmt"
	"strings"

	"terraform-cost/core/confidence"
)

// DataSourceBarrier enforces estimation rules for data sources
type DataSourceBarrier struct {
	// Data sources that are inherently unpriceable
	unpriceable map[string]bool

	// Attributes that when sourced from data, block pricing
	blockingAttributes map[string][]string

	// Confidence impacts per data source type
	confidenceImpacts map[string]float64
}

// NewDataSourceBarrier creates a new barrier
func NewDataSourceBarrier() *DataSourceBarrier {
	return &DataSourceBarrier{
		unpriceable: map[string]bool{
			"aws_ami":                true, // Runtime lookup
			"aws_availability_zones": true, // Runtime lookup
			"aws_caller_identity":    true, // Runtime lookup
			"aws_region":             true, // Runtime lookup
			"aws_vpc":                true, // External resource
			"aws_subnet":             true, // External resource
			"aws_security_group":     true, // External resource
			"aws_iam_role":           true, // External resource
		},
		blockingAttributes: map[string][]string{
			"aws_instance": {"ami", "subnet_id"},
			"aws_db_instance": {"db_subnet_group_name"},
			"aws_lambda_function": {"role"},
			"aws_ecs_service": {"cluster"},
		},
		confidenceImpacts: map[string]float64{
			"aws_ami":                0.25, // AMI affects pricing
			"aws_availability_zones": 0.10, // AZ is minor
			"aws_caller_identity":    0.05, // No pricing impact
			"aws_vpc":                0.15, // VPC affects networking
			"aws_subnet":             0.15,
			"aws_security_group":     0.05,
			"aws_iam_role":           0.05,
			"default":                0.20,
		},
	}
}

// DataSourceReference represents a reference to a data source
type DataSourceReference struct {
	DataType      string // e.g., "aws_ami"
	DataName      string // e.g., "latest"
	Attribute     string // e.g., "id"
	FullReference string // e.g., "data.aws_ami.latest.id"
}

// ParseDataSourceReference parses a data source reference
func ParseDataSourceReference(ref string) (*DataSourceReference, bool) {
	if !strings.HasPrefix(ref, "data.") {
		return nil, false
	}

	parts := strings.Split(ref, ".")
	if len(parts) < 3 {
		return nil, false
	}

	result := &DataSourceReference{
		DataType:      parts[1],
		DataName:      parts[2],
		FullReference: ref,
	}

	if len(parts) >= 4 {
		result.Attribute = strings.Join(parts[3:], ".")
	}

	return result, true
}

// CanEstimate checks if an attribute can be estimated when sourced from data
func (b *DataSourceBarrier) CanEstimate(resourceType, attribute string, dataRef *DataSourceReference) EstimationDecision {
	decision := EstimationDecision{
		CanEstimate:      true,
		ConfidenceImpact: 0,
		Reason:           "",
		Recommendations:  []string{},
	}

	// Check if data source is inherently unpriceable
	if b.unpriceable[dataRef.DataType] {
		decision.ConfidenceImpact = b.getConfidenceImpact(dataRef.DataType)
		decision.Reason = fmt.Sprintf("data source %s is resolved at runtime", dataRef.DataType)
		decision.Recommendations = append(decision.Recommendations,
			fmt.Sprintf("Consider providing explicit value for %s.%s", resourceType, attribute))
	}

	// Check if this attribute blocks pricing when from data
	blockingAttrs := b.blockingAttributes[resourceType]
	for _, blocked := range blockingAttrs {
		if blocked == attribute {
			decision.CanEstimate = false
			decision.ConfidenceImpact = 0.4 // Severe
			decision.Reason = fmt.Sprintf("attribute %s cannot be estimated from data source", attribute)
			decision.Recommendations = append(decision.Recommendations,
				fmt.Sprintf("Provide explicit %s value or use usage override", attribute))
			return decision
		}
	}

	return decision
}

// EstimationDecision is the result of checking if estimation is possible
type EstimationDecision struct {
	CanEstimate      bool
	ConfidenceImpact float64
	Reason           string
	Recommendations  []string
}

func (b *DataSourceBarrier) getConfidenceImpact(dataType string) float64 {
	if impact, ok := b.confidenceImpacts[dataType]; ok {
		return impact
	}
	return b.confidenceImpacts["default"]
}

// ApplyDataSourceImpacts applies confidence impacts for all data source references
func (b *DataSourceBarrier) ApplyDataSourceImpacts(
	tracker *confidence.ConfidenceTracker,
	resourceType string,
	attributes map[string]interface{},
	dataRefs map[string]*DataSourceReference,
) []EstimationDecision {
	var decisions []EstimationDecision

	for attrName, dataRef := range dataRefs {
		decision := b.CanEstimate(resourceType, attrName, dataRef)
		decisions = append(decisions, decision)

		if !decision.CanEstimate {
			tracker.Apply("data_source_barrier", decision.Reason)
		} else if decision.ConfidenceImpact > 0 {
			tracker.Apply("data_source_uncertainty", decision.Reason)
		}
	}

	return decisions
}

// AttributeSourceAnalyzer analyzes where attribute values come from
type AttributeSourceAnalyzer struct {
	barrier *DataSourceBarrier
}

// NewAttributeSourceAnalyzer creates an analyzer
func NewAttributeSourceAnalyzer() *AttributeSourceAnalyzer {
	return &AttributeSourceAnalyzer{
		barrier: NewDataSourceBarrier(),
	}
}

// AttributeSource describes where an attribute value comes from
type AttributeSource struct {
	// Source type
	Type AttributeSourceType

	// Reference (if from data or resource)
	Reference string

	// Is this estimable?
	IsEstimable bool

	// Confidence impact
	ConfidenceImpact float64

	// Reason if not estimable
	Reason string
}

// AttributeSourceType indicates the source of an attribute
type AttributeSourceType int

const (
	SourceLiteral     AttributeSourceType = iota // Hardcoded value
	SourceVariable                                // From variable
	SourceLocal                                   // From local
	SourceDataSource                              // From data source
	SourceResource                                // From another resource
	SourceUnknown                                 // Cannot determine
)

// String returns the source type name
func (t AttributeSourceType) String() string {
	switch t {
	case SourceLiteral:
		return "literal"
	case SourceVariable:
		return "variable"
	case SourceLocal:
		return "local"
	case SourceDataSource:
		return "data_source"
	case SourceResource:
		return "resource"
	default:
		return "unknown"
	}
}

// AnalyzeAttribute determines the source of an attribute value
func (a *AttributeSourceAnalyzer) AnalyzeAttribute(resourceType, attrName, expression string, references []string) AttributeSource {
	source := AttributeSource{
		Type:        SourceUnknown,
		IsEstimable: true,
	}

	// No references = literal
	if len(references) == 0 {
		source.Type = SourceLiteral
		return source
	}

	// Check first reference to determine type
	ref := references[0]

	if strings.HasPrefix(ref, "var.") {
		source.Type = SourceVariable
		source.Reference = ref
		source.ConfidenceImpact = 0.1 // Variables are usually known
		return source
	}

	if strings.HasPrefix(ref, "local.") {
		source.Type = SourceLocal
		source.Reference = ref
		source.ConfidenceImpact = 0.05 // Locals are resolvable
		return source
	}

	if strings.HasPrefix(ref, "data.") {
		source.Type = SourceDataSource
		source.Reference = ref

		dataRef, _ := ParseDataSourceReference(ref)
		if dataRef != nil {
			decision := a.barrier.CanEstimate(resourceType, attrName, dataRef)
			source.IsEstimable = decision.CanEstimate
			source.ConfidenceImpact = decision.ConfidenceImpact
			source.Reason = decision.Reason
		}
		return source
	}

	// Resource reference
	source.Type = SourceResource
	source.Reference = ref
	source.ConfidenceImpact = 0.15 // Resource refs are runtime
	return source
}
