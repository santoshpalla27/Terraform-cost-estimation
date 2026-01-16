// Package networking - AWS EIP, VPC Endpoint, VPN mappers
package networking

import (
	"terraform-cost/clouds"
)

// EIPMapper maps aws_eip to cost units
type EIPMapper struct{}

func NewEIPMapper() *EIPMapper { return &EIPMapper{} }

func (m *EIPMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *EIPMapper) ResourceType() string        { return "aws_eip" }

func (m *EIPMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown EIP count")}, nil
	}
	return []clouds.UsageVector{clouds.NewUsageVector(clouds.MetricMonthlyHours, 730, 0.95)}, nil
}

func (m *EIPMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("eip", "EIP count unknown")}, nil
	}

	// Unattached EIPs cost money, attached ones are free (pre-2024)
	// Post Feb 2024, all EIPs cost $0.005/hour
	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	return []clouds.CostUnit{
		clouds.NewCostUnit("eip", "hours", monthlyHours, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonEC2",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "ElasticIP:IdleAddress",
			},
		}, 0.95),
	}, nil
}

// VPCEndpointMapper maps aws_vpc_endpoint to cost units
type VPCEndpointMapper struct{}

func NewVPCEndpointMapper() *VPCEndpointMapper { return &VPCEndpointMapper{} }

func (m *VPCEndpointMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *VPCEndpointMapper) ResourceType() string        { return "aws_vpc_endpoint" }

func (m *VPCEndpointMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown endpoint count")}, nil
	}
	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, 730, 0.95),
		clouds.NewUsageVector("data_processed_gb", ctx.ResolveOrDefault("data_processed_gb", 0), 0.5),
	}, nil
}

func (m *VPCEndpointMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("vpc_endpoint", "endpoint count unknown")}, nil
	}

	vpcEndpointType := asset.Attr("vpc_endpoint_type")
	if vpcEndpointType == "" {
		vpcEndpointType = "Gateway"
	}

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)
	dataProcessed, _ := usageVecs.Get("data_processed_gb")

	var units []clouds.CostUnit

	// Gateway endpoints are free, Interface endpoints cost per hour + data
	if vpcEndpointType == "Interface" {
		units = append(units, clouds.NewCostUnit("endpoint_hours", "hours", monthlyHours, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonVPC",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "VpcEndpoint-Hours",
			},
		}, 0.95))

		if dataProcessed > 0 {
			units = append(units, clouds.NewCostUnit("data_processed", "GB", dataProcessed, clouds.RateKey{
				Provider: asset.ProviderContext.ProviderID,
				Service:  "AmazonVPC",
				Region:   asset.ProviderContext.Region,
				Attributes: map[string]string{
					"usageType": "VpcEndpoint-Bytes",
				},
			}, 0.5))
		}
	}

	if len(units) == 0 {
		return []clouds.CostUnit{clouds.SymbolicCost("gateway_endpoint", "Gateway endpoints are free")}, nil
	}

	return units, nil
}

// VPNConnectionMapper maps aws_vpn_connection to cost units
type VPNConnectionMapper struct{}

func NewVPNConnectionMapper() *VPNConnectionMapper { return &VPNConnectionMapper{} }

func (m *VPNConnectionMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *VPNConnectionMapper) ResourceType() string        { return "aws_vpn_connection" }

func (m *VPNConnectionMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown VPN count")}, nil
	}
	return []clouds.UsageVector{clouds.NewUsageVector(clouds.MetricMonthlyHours, 730, 0.95)}, nil
}

func (m *VPNConnectionMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("vpn", "VPN count unknown")}, nil
	}

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	return []clouds.CostUnit{
		clouds.NewCostUnit("vpn_connection", "hours", monthlyHours, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonVPC",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "VPN-Connection-Hours",
			},
		}, 0.95),
	}, nil
}

// DXConnectionMapper maps aws_dx_connection to cost units
type DXConnectionMapper struct{}

func NewDXConnectionMapper() *DXConnectionMapper { return &DXConnectionMapper{} }

func (m *DXConnectionMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *DXConnectionMapper) ResourceType() string        { return "aws_dx_connection" }

func (m *DXConnectionMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown DX count")}, nil
	}
	return []clouds.UsageVector{clouds.NewUsageVector(clouds.MetricMonthlyHours, 730, 0.95)}, nil
}

func (m *DXConnectionMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("direct_connect", "DX count unknown")}, nil
	}

	bandwidth := asset.Attr("bandwidth")
	if bandwidth == "" {
		bandwidth = "1Gbps"
	}
	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	return []clouds.CostUnit{
		clouds.NewCostUnit("port_hours", "hours", monthlyHours, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AWSDirectConnect",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"bandwidth": bandwidth,
				"usageType": "PortUsage:" + bandwidth,
			},
		}, 0.95),
	}, nil
}

// CLBMapper maps aws_elb (Classic Load Balancer) to cost units
type CLBMapper struct{}

func NewCLBMapper() *CLBMapper { return &CLBMapper{} }

func (m *CLBMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *CLBMapper) ResourceType() string        { return "aws_elb" }

func (m *CLBMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown ELB count")}, nil
	}
	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyHours, 730, 0.95),
		clouds.NewUsageVector("data_processed_gb", ctx.ResolveOrDefault("data_processed_gb", 0), 0.5),
	}, nil
}

func (m *CLBMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("clb", "ELB count unknown")}, nil
	}

	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)
	dataProcessed, _ := usageVecs.Get("data_processed_gb")

	units := []clouds.CostUnit{
		clouds.NewCostUnit("clb_hours", "hours", monthlyHours, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AWSELB",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "LoadBalancerUsage",
			},
		}, 0.95),
	}

	if dataProcessed > 0 {
		units = append(units, clouds.NewCostUnit("data_processed", "GB", dataProcessed, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AWSELB",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"usageType": "DataProcessing-Bytes",
			},
		}, 0.5))
	}

	return units, nil
}
