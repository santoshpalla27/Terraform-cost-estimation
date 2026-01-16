// Package security - AWS WAF mappers
package security

import (
	"terraform-cost/clouds"
)

// WAFWebACLMapper maps aws_waf_web_acl to cost units
type WAFWebACLMapper struct{}

func NewWAFWebACLMapper() *WAFWebACLMapper { return &WAFWebACLMapper{} }

func (m *WAFWebACLMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *WAFWebACLMapper) ResourceType() string        { return "aws_waf_web_acl" }

func (m *WAFWebACLMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage("web_acls", "unknown ACL count")}, nil
	}

	requestsMillions := ctx.ResolveOrDefault("requests_millions", -1)
	if requestsMillions < 0 {
		return []clouds.UsageVector{
			clouds.NewUsageVector("web_acls", 1, 1.0),
			clouds.SymbolicUsage(clouds.MetricMonthlyRequests, "WAF request volume not provided"),
		}, nil
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector("web_acls", 1, 1.0),
		clouds.NewUsageVector("requests_millions", requestsMillions, 0.5),
	}, nil
}

func (m *WAFWebACLMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	units := []clouds.CostUnit{
		// Web ACL monthly cost
		clouds.NewCostUnit("web_acl", "ACLs", 1, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AWSWAF",
			Region:   "global",
			Attributes: map[string]string{
				"usageType": "WebACL",
			},
		}, 0.95),
	}

	// Request cost if available
	if requestsMillions, ok := usageVecs.Get("requests_millions"); ok {
		units = append(units, clouds.NewCostUnit("requests", "million-requests", requestsMillions, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AWSWAF",
			Region:   "global",
			Attributes: map[string]string{
				"usageType": "Request",
			},
		}, 0.5))
	} else {
		units = append(units, clouds.SymbolicCost("requests", "WAF request cost depends on traffic volume"))
	}

	return units, nil
}

// WAFv2WebACLMapper maps aws_wafv2_web_acl to cost units
type WAFv2WebACLMapper struct{}

func NewWAFv2WebACLMapper() *WAFv2WebACLMapper { return &WAFv2WebACLMapper{} }

func (m *WAFv2WebACLMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *WAFv2WebACLMapper) ResourceType() string        { return "aws_wafv2_web_acl" }

func (m *WAFv2WebACLMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	return (&WAFWebACLMapper{}).BuildUsage(asset, ctx)
}

func (m *WAFv2WebACLMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	return (&WAFWebACLMapper{}).BuildCostUnits(asset, usage)
}
