// Package apigateway - AWS API Gateway cost mapper
// Pricing model:
// - REST API: per million requests + cache (if enabled)
// - HTTP API: per million requests (cheaper)
// - WebSocket API: per million messages + connection minutes
// - Data transfer: standard AWS rates
package apigateway

import (
	"terraform-cost/clouds"
)

// RESTAPIMapper maps aws_api_gateway_rest_api to cost units
type RESTAPIMapper struct{}

// NewRESTAPIMapper creates an API Gateway REST API mapper
func NewRESTAPIMapper() *RESTAPIMapper {
	return &RESTAPIMapper{}
}

// Cloud returns the cloud provider
func (m *RESTAPIMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *RESTAPIMapper) ResourceType() string {
	return "aws_api_gateway_rest_api"
}

// BuildUsage extracts usage vectors
func (m *RESTAPIMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyRequests, "unknown API count: "+asset.Cardinality.Reason),
		}, nil
	}

	// API Gateway is HIGHLY usage-dependent
	monthlyRequests := ctx.ResolveOrDefault("monthly_requests", -1)

	if monthlyRequests < 0 {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyRequests, "monthly API requests not provided"),
		}, nil
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyRequests, monthlyRequests, 0.5),
	}, nil
}

// BuildCostUnits creates cost units
func (m *RESTAPIMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("api_requests", "API Gateway cost requires request volume"),
		}, nil
	}

	monthlyRequests, _ := usageVecs.Get(clouds.MetricMonthlyRequests)

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"requests",
			"million-requests",
			monthlyRequests/1000000,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonApiGateway",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "ApiGatewayRequest",
					"apiType":   "REST",
				},
			},
			0.5,
		),
	}, nil
}

// HTTPAPIMapper maps aws_apigatewayv2_api (HTTP) to cost units
type HTTPAPIMapper struct{}

// NewHTTPAPIMapper creates an HTTP API mapper
func NewHTTPAPIMapper() *HTTPAPIMapper {
	return &HTTPAPIMapper{}
}

// Cloud returns the cloud provider
func (m *HTTPAPIMapper) Cloud() clouds.CloudProvider {
	return clouds.AWS
}

// ResourceType returns the Terraform resource type
func (m *HTTPAPIMapper) ResourceType() string {
	return "aws_apigatewayv2_api"
}

// BuildUsage extracts usage vectors
func (m *HTTPAPIMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyRequests, "unknown API count: "+asset.Cardinality.Reason),
		}, nil
	}

	monthlyRequests := ctx.ResolveOrDefault("monthly_requests", -1)
	monthlyMessages := ctx.ResolveOrDefault("monthly_messages", -1)

	protocolType := asset.Attr("protocol_type")

	if protocolType == "WEBSOCKET" {
		if monthlyMessages < 0 {
			return []clouds.UsageVector{
				clouds.SymbolicUsage("monthly_messages", "WebSocket messages not provided"),
			}, nil
		}
		return []clouds.UsageVector{
			clouds.NewUsageVector("monthly_messages", monthlyMessages, 0.5),
		}, nil
	}

	// HTTP API
	if monthlyRequests < 0 {
		return []clouds.UsageVector{
			clouds.SymbolicUsage(clouds.MetricMonthlyRequests, "monthly API requests not provided"),
		}, nil
	}

	return []clouds.UsageVector{
		clouds.NewUsageVector(clouds.MetricMonthlyRequests, monthlyRequests, 0.5),
	}, nil
}

// BuildCostUnits creates cost units
func (m *HTTPAPIMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)

	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{
			clouds.SymbolicCost("api", "API Gateway v2 cost requires usage data"),
		}, nil
	}

	protocolType := asset.Attr("protocol_type")
	if protocolType == "" {
		protocolType = "HTTP"
	}

	providerID := asset.ProviderContext.ProviderID
	region := asset.ProviderContext.Region

	if protocolType == "WEBSOCKET" {
		messages, _ := usageVecs.Get("monthly_messages")
		return []clouds.CostUnit{
			clouds.NewCostUnit(
				"messages",
				"million-messages",
				messages/1000000,
				clouds.RateKey{
					Provider: providerID,
					Service:  "AmazonApiGateway",
					Region:   region,
					Attributes: map[string]string{
						"usageType": "WebSocketMessage",
						"apiType":   "WEBSOCKET",
					},
				},
				0.5,
			),
		}, nil
	}

	// HTTP API (cheaper than REST)
	monthlyRequests, _ := usageVecs.Get(clouds.MetricMonthlyRequests)
	return []clouds.CostUnit{
		clouds.NewCostUnit(
			"requests",
			"million-requests",
			monthlyRequests/1000000,
			clouds.RateKey{
				Provider: providerID,
				Service:  "AmazonApiGateway",
				Region:   region,
				Attributes: map[string]string{
					"usageType": "ApiGatewayHttpRequest",
					"apiType":   "HTTP",
				},
			},
			0.5,
		),
	}, nil
}
