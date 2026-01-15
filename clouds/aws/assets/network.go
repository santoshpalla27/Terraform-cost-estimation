// Package assets - AWS network asset builders
package assets

import (
	"context"
	"fmt"

	"terraform-cost/core/asset"
	"terraform-cost/core/types"
)

// NATGatewayBuilder builds assets for aws_nat_gateway
type NATGatewayBuilder struct {
	baseBuilder
}

// NewNATGatewayBuilder creates a new NAT Gateway builder
func NewNATGatewayBuilder() asset.Builder {
	return &NATGatewayBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_nat_gateway",
			category:     types.CategoryNetwork,
		},
	}
}

// Build converts a raw NAT Gateway into an asset
func (b *NATGatewayBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	return &types.Asset{
		ID:       fmt.Sprintf("aws_nat_gateway.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryNetwork,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"connectivity_type": raw.Attributes["connectivity_type"],
			"subnet_id":         raw.Attributes["subnet_id"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}

// VPCEndpointBuilder builds assets for aws_vpc_endpoint
type VPCEndpointBuilder struct {
	baseBuilder
}

// NewVPCEndpointBuilder creates a new VPC Endpoint builder
func NewVPCEndpointBuilder() asset.Builder {
	return &VPCEndpointBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_vpc_endpoint",
			category:     types.CategoryNetwork,
		},
	}
}

// Build converts a raw VPC Endpoint into an asset
func (b *VPCEndpointBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	vpcEndpointType := raw.Attributes.GetString("vpc_endpoint_type")
	if vpcEndpointType == "" {
		vpcEndpointType = "Gateway"
	}

	return &types.Asset{
		ID:       fmt.Sprintf("aws_vpc_endpoint.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryNetwork,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"vpc_endpoint_type": {Value: vpcEndpointType},
			"service_name":      raw.Attributes["service_name"],
			"vpc_id":            raw.Attributes["vpc_id"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}

// ELBBuilder builds assets for aws_elb (Classic)
type ELBBuilder struct {
	baseBuilder
}

// NewELBBuilder creates a new ELB builder
func NewELBBuilder() asset.Builder {
	return &ELBBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_elb",
			category:     types.CategoryNetwork,
		},
	}
}

// Build converts a raw ELB into an asset
func (b *ELBBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	return &types.Asset{
		ID:       fmt.Sprintf("aws_elb.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryNetwork,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"internal": raw.Attributes["internal"],
			"listener": raw.Attributes["listener"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}

// ALBBuilder builds assets for aws_lb (Application)
type ALBBuilder struct {
	baseBuilder
}

// NewALBBuilder creates a new ALB builder
func NewALBBuilder() asset.Builder {
	return &ALBBuilder{
		baseBuilder: baseBuilder{
			resourceType: "aws_lb",
			category:     types.CategoryNetwork,
		},
	}
}

// Build converts a raw ALB/NLB into an asset
func (b *ALBBuilder) Build(ctx context.Context, raw *types.RawAsset) (*types.Asset, error) {
	loadBalancerType := raw.Attributes.GetString("load_balancer_type")
	if loadBalancerType == "" {
		loadBalancerType = "application"
	}

	return &types.Asset{
		ID:       fmt.Sprintf("aws_lb.%s", raw.Name),
		Address:  raw.Address,
		Provider: types.ProviderAWS,
		Category: types.CategoryNetwork,
		Type:     raw.Type,
		Name:     raw.Name,
		Attributes: types.Attributes{
			"load_balancer_type": {Value: loadBalancerType},
			"internal":           raw.Attributes["internal"],
		},
		Metadata: types.AssetMetadata{
			Source: raw.SourceFile,
			Line:   raw.SourceLine,
		},
		Tags: extractTags(raw.Attributes),
	}, nil
}

// NLBBuilder builds assets for aws_lb (Network) - alias to ALBBuilder
func NewNLBBuilder() asset.Builder {
	return NewALBBuilder()
}
