// Package monitoring - AWS CloudTrail mapper
package monitoring

import (
	"terraform-cost/clouds"
)

// CloudTrailMapper maps aws_cloudtrail to cost units
type CloudTrailMapper struct{}

func NewCloudTrailMapper() *CloudTrailMapper { return &CloudTrailMapper{} }

func (m *CloudTrailMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *CloudTrailMapper) ResourceType() string        { return "aws_cloudtrail" }

func (m *CloudTrailMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage("trails", "unknown trail count")}, nil
	}

	eventsPerMonth := ctx.ResolveOrDefault("events_per_month", -1)
	if eventsPerMonth < 0 {
		return []clouds.UsageVector{clouds.SymbolicUsage("events", "event volume not provided")}, nil
	}

	return []clouds.UsageVector{clouds.NewUsageVector("events", eventsPerMonth, 0.5)}, nil
}

func (m *CloudTrailMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("cloudtrail", "CloudTrail cost depends on event volume"),
		}, nil
	}

	events, _ := usageVecs.Get("events")

	isMultiRegion := asset.AttrBool("is_multi_region_trail", false)
	includeDataEvents := asset.AttrBool("include_global_service_events", false)

	var notes string
	if isMultiRegion {
		notes = "multi-region"
	}
	if includeDataEvents {
		notes += " with data events"
	}

	return []clouds.CostUnit{
		clouds.NewCostUnit("events", "100k-events", events/100000, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AWSCloudTrail",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "DataEventsRecorded",
				"notes":     notes,
			},
		}, 0.5),
	}, nil
}
