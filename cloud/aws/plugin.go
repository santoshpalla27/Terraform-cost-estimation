// Package aws - AWS cloud plugin
// Provides resource mappers, usage defaults, and pricing adapters.
// Core engine remains untouched.
package aws

import (
	"terraform-cost/core/determinism"
)

// ResourceMapper maps Terraform resources to cost components
type ResourceMapper struct {
	resourceType string
}

// NewResourceMapper creates a mapper for a resource type
func NewResourceMapper(resourceType string) *ResourceMapper {
	return &ResourceMapper{resourceType: resourceType}
}

// Map maps resource attributes to cost components
func (m *ResourceMapper) Map(attrs map[string]interface{}) []CostComponent {
	mapper, ok := resourceMappers[m.resourceType]
	if !ok {
		return nil
	}
	return mapper(attrs)
}

// CostComponent represents a cost component
type CostComponent struct {
	Name     string
	Unit     string
	Quantity float64
	RateKey  string
}

// UsageDefaults provides default usage values
var UsageDefaults = map[string]map[string]interface{}{
	"aws_instance": {
		"monthly_hours": 730,
	},
	"aws_lambda_function": {
		"monthly_requests":   1000000,
		"average_duration_ms": 100,
	},
	"aws_s3_bucket": {
		"storage_gb":          100,
		"put_requests":        10000,
		"get_requests":        100000,
	},
	"aws_db_instance": {
		"monthly_hours": 730,
		"storage_gb":    20,
	},
	"aws_nat_gateway": {
		"data_processed_gb": 100,
	},
	"aws_ebs_volume": {
		"iops": 3000,
	},
}

// GetUsageDefault returns the default usage value
func GetUsageDefault(resourceType, attribute string) (interface{}, bool) {
	if defaults, ok := UsageDefaults[resourceType]; ok {
		if val, ok := defaults[attribute]; ok {
			return val, true
		}
	}
	return nil, false
}

// Resource mappers by type
var resourceMappers = map[string]func(map[string]interface{}) []CostComponent{
	"aws_instance":         mapInstance,
	"aws_lambda_function":  mapLambda,
	"aws_s3_bucket":        mapS3,
	"aws_db_instance":      mapRDS,
	"aws_nat_gateway":      mapNATGateway,
	"aws_ebs_volume":       mapEBS,
	"aws_lb":               mapALB,
}

func mapInstance(attrs map[string]interface{}) []CostComponent {
	instanceType, _ := attrs["instance_type"].(string)
	if instanceType == "" {
		instanceType = "t3.micro"
	}

	components := []CostComponent{
		{
			Name:     "compute",
			Unit:     "hours",
			Quantity: 730,
			RateKey:  "aws_instance:" + instanceType,
		},
	}

	// EBS root volume
	if rootSize, ok := attrs["root_block_device.volume_size"].(float64); ok && rootSize > 0 {
		components = append(components, CostComponent{
			Name:     "storage",
			Unit:     "GB-months",
			Quantity: rootSize,
			RateKey:  "aws_ebs_volume:gp3",
		})
	}

	return components
}

func mapLambda(attrs map[string]interface{}) []CostComponent {
	memoryMB := 128.0
	if m, ok := attrs["memory_size"].(float64); ok {
		memoryMB = m
	}

	return []CostComponent{
		{
			Name:     "requests",
			Unit:     "requests",
			Quantity: 1000000,
			RateKey:  "aws_lambda:requests",
		},
		{
			Name:     "duration",
			Unit:     "GB-seconds",
			Quantity: 1000000 * 0.1 * (memoryMB / 1024),
			RateKey:  "aws_lambda:duration",
		},
	}
}

func mapS3(attrs map[string]interface{}) []CostComponent {
	return []CostComponent{
		{
			Name:     "storage",
			Unit:     "GB-months",
			Quantity: 100,
			RateKey:  "aws_s3:storage:standard",
		},
		{
			Name:     "put_requests",
			Unit:     "requests",
			Quantity: 10000,
			RateKey:  "aws_s3:put_requests",
		},
		{
			Name:     "get_requests",
			Unit:     "requests",
			Quantity: 100000,
			RateKey:  "aws_s3:get_requests",
		},
	}
}

func mapRDS(attrs map[string]interface{}) []CostComponent {
	instanceClass, _ := attrs["instance_class"].(string)
	if instanceClass == "" {
		instanceClass = "db.t3.micro"
	}

	engine, _ := attrs["engine"].(string)
	if engine == "" {
		engine = "mysql"
	}

	return []CostComponent{
		{
			Name:     "instance",
			Unit:     "hours",
			Quantity: 730,
			RateKey:  "aws_db_instance:" + engine + ":" + instanceClass,
		},
		{
			Name:     "storage",
			Unit:     "GB-months",
			Quantity: 20,
			RateKey:  "aws_db_instance:storage:gp2",
		},
	}
}

func mapNATGateway(attrs map[string]interface{}) []CostComponent {
	return []CostComponent{
		{
			Name:     "hourly",
			Unit:     "hours",
			Quantity: 730,
			RateKey:  "aws_nat_gateway:hourly",
		},
		{
			Name:     "data_processed",
			Unit:     "GB",
			Quantity: 100,
			RateKey:  "aws_nat_gateway:data",
		},
	}
}

func mapEBS(attrs map[string]interface{}) []CostComponent {
	volumeType, _ := attrs["type"].(string)
	if volumeType == "" {
		volumeType = "gp3"
	}

	size := 8.0
	if s, ok := attrs["size"].(float64); ok {
		size = s
	}

	return []CostComponent{
		{
			Name:     "storage",
			Unit:     "GB-months",
			Quantity: size,
			RateKey:  "aws_ebs_volume:" + volumeType,
		},
	}
}

func mapALB(attrs map[string]interface{}) []CostComponent {
	return []CostComponent{
		{
			Name:     "hourly",
			Unit:     "hours",
			Quantity: 730,
			RateKey:  "aws_lb:hourly",
		},
		{
			Name:     "lcu",
			Unit:     "LCU-hours",
			Quantity: 730, // Assumes 1 LCU average
			RateKey:  "aws_lb:lcu",
		},
	}
}

// PricingAdapter adapts AWS pricing responses
type PricingAdapter struct{}

// NewPricingAdapter creates a pricing adapter
func NewPricingAdapter() *PricingAdapter {
	return &PricingAdapter{}
}

// GetRate returns the rate for a rate key
func (a *PricingAdapter) GetRate(rateKey string, region string) (determinism.Money, error) {
	// This would call AWS Pricing API
	// For now, return placeholder
	return determinism.Zero("USD"), nil
}

// SupportedResources returns all supported resource types
func SupportedResources() []string {
	resources := make([]string, 0, len(resourceMappers))
	for k := range resourceMappers {
		resources = append(resources, k)
	}
	return resources
}
