// Package containers - AWS ECS Service and Task Definition mappers
package containers

import (
	"terraform-cost/clouds"
)

// ECSServiceMapper maps aws_ecs_service to cost units
type ECSServiceMapper struct{}

func NewECSServiceMapper() *ECSServiceMapper { return &ECSServiceMapper{} }

func (m *ECSServiceMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *ECSServiceMapper) ResourceType() string        { return "aws_ecs_service" }

func (m *ECSServiceMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	if asset.Cardinality.IsUnknown() {
		return []clouds.UsageVector{clouds.SymbolicUsage(clouds.MetricMonthlyHours, "unknown service count")}, nil
	}
	return []clouds.UsageVector{clouds.NewUsageVector(clouds.MetricMonthlyHours, ctx.ResolveOrDefault("monthly_hours", 730), 0.95)}, nil
}

func (m *ECSServiceMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	usageVecs := clouds.UsageVectors(usage)
	if usageVecs.IsSymbolic() {
		return []clouds.CostUnit{clouds.SymbolicCost("ecs_service", "service count unknown")}, nil
	}

	launchType := asset.Attr("launch_type")
	if launchType == "" {
		launchType = "EC2"
	}

	desiredCount := asset.AttrInt("desired_count", 1)
	monthlyHours, _ := usageVecs.Get(clouds.MetricMonthlyHours)

	// For Fargate, cost is vCPU + memory hours
	if launchType == "FARGATE" {
		return []clouds.CostUnit{
			clouds.SymbolicCost("fargate_tasks", "Fargate cost depends on task definition CPU/memory"),
		}, nil
	}

	// For EC2 launch type, tasks run on EC2 instances (no additional service cost)
	return []clouds.CostUnit{
		clouds.NewCostUnit("service_tasks", "task-hours", float64(desiredCount)*monthlyHours, clouds.RateKey{
			Provider: asset.ProviderContext.ProviderID,
			Service:  "AmazonECS",
			Region:   asset.ProviderContext.Region,
			Attributes: map[string]string{
				"launchType": launchType,
			},
		}, 0.7),
	}, nil
}

// ECSTaskDefinitionMapper maps aws_ecs_task_definition to cost units
type ECSTaskDefinitionMapper struct{}

func NewECSTaskDefinitionMapper() *ECSTaskDefinitionMapper { return &ECSTaskDefinitionMapper{} }

func (m *ECSTaskDefinitionMapper) Cloud() clouds.CloudProvider { return clouds.AWS }
func (m *ECSTaskDefinitionMapper) ResourceType() string        { return "aws_ecs_task_definition" }

func (m *ECSTaskDefinitionMapper) BuildUsage(asset clouds.AssetNode, ctx clouds.UsageContext) ([]clouds.UsageVector, error) {
	// Task definitions don't have direct cost - cost is per running task
	return []clouds.UsageVector{clouds.NewUsageVector("task_definitions", 1, 1.0)}, nil
}

func (m *ECSTaskDefinitionMapper) BuildCostUnits(asset clouds.AssetNode, usage []clouds.UsageVector) ([]clouds.CostUnit, error) {
	requiresCompatibilities := asset.Attr("requires_compatibilities.0")

	if requiresCompatibilities == "FARGATE" {
		cpu := asset.AttrFloat("cpu", 256)
		memory := asset.AttrFloat("memory", 512)

		// Convert to vCPU (256 CPU units = 0.25 vCPU)
		vcpu := cpu / 1024
		memoryGB := memory / 1024

		return []clouds.CostUnit{
			clouds.SymbolicCost("fargate", 
				"Fargate task definition: "+
				"vCPU="+formatFloat(vcpu)+
				", Memory="+formatFloat(memoryGB)+"GB. "+
				"Cost depends on number of running tasks"),
		}, nil
	}

	// EC2 launch type - no additional cost from task definition
	return []clouds.CostUnit{
		clouds.SymbolicCost("ec2_task", "Task runs on EC2 instances, no additional ECS cost"),
	}, nil
}

func formatFloat(f float64) string {
	if f == float64(int(f)) {
		return string(rune('0' + int(f)))
	}
	return "~"
}
