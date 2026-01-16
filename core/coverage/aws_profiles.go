// Package coverage - AWS resource cost profiles
// Based on real-world spend distribution:
// - Top 30 services cover ~95% of spend
// - Remaining ~270 services cover ~5%
package coverage

// RegisterAWS registers all AWS resource cost profiles
func RegisterAWS(reg *Registry) {
	// ============================================================
	// TIER 1: Top Cost Drivers (~60-70% of spend)
	// These MUST have full mappers
	// ============================================================

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_instance",
		Behavior:                   CostDirect,
		MapperExists:               true,
		EstimatedSpendContribution: 25.0,
		Notes:                      "EC2 instances - largest single cost driver",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_ebs_volume",
		Behavior:                   CostDirect,
		MapperExists:               true,
		EstimatedSpendContribution: 8.0,
		Notes:                      "EBS volumes - storage for EC2",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_db_instance",
		Behavior:                   CostDirect,
		MapperExists:               true,
		EstimatedSpendContribution: 12.0,
		Notes:                      "RDS instances - managed databases",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_rds_cluster",
		Behavior:                   CostDirect,
		MapperExists:               false, // TODO: implement
		EstimatedSpendContribution: 5.0,
		Notes:                      "Aurora clusters",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_s3_bucket",
		Behavior:                   CostUsageBased,
		MapperExists:               true,
		EstimatedSpendContribution: 6.0,
		Notes:                      "S3 storage and requests",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_nat_gateway",
		Behavior:                   CostDirect,
		MapperExists:               true,
		EstimatedSpendContribution: 5.0,
		Notes:                      "NAT Gateway - often surprising cost",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_lb",
		Behavior:                   CostDirect,
		MapperExists:               true,
		EstimatedSpendContribution: 4.0,
		Notes:                      "Application/Network Load Balancer",
	})

	// ============================================================
	// TIER 2: Serverless & Containers (~10-15% of spend)
	// ============================================================

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_lambda_function",
		Behavior:                   CostUsageBased,
		MapperExists:               true,
		EstimatedSpendContribution: 4.0,
		Notes:                      "Lambda - pay per invocation",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_eks_cluster",
		Behavior:                   CostDirect,
		MapperExists:               true,
		EstimatedSpendContribution: 3.0,
		Notes:                      "EKS control plane ($0.10/hour)",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_ecs_service",
		Behavior:                   CostIndirect,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "ECS itself is free - cost is in EC2/Fargate",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_ecs_task_definition",
		Behavior:                   CostIndirect,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "Task definition is config - Fargate tasks cost",
	})

	// ============================================================
	// TIER 3: Data & Analytics (~5-10% of spend)
	// ============================================================

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_dynamodb_table",
		Behavior:                   CostUsageBased,
		MapperExists:               true,
		EstimatedSpendContribution: 3.0,
		Notes:                      "DynamoDB - capacity or on-demand",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_elasticache_cluster",
		Behavior:                   CostDirect,
		MapperExists:               true,
		EstimatedSpendContribution: 2.5,
		Notes:                      "ElastiCache - Redis/Memcached",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_kinesis_stream",
		Behavior:                   CostDirect,
		MapperExists:               false,
		EstimatedSpendContribution: 1.5,
		Notes:                      "Kinesis Data Streams",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_redshift_cluster",
		Behavior:                   CostDirect,
		MapperExists:               false,
		EstimatedSpendContribution: 2.0,
		Notes:                      "Redshift data warehouse",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_opensearch_domain",
		Behavior:                   CostDirect,
		MapperExists:               false,
		EstimatedSpendContribution: 2.0,
		Notes:                      "OpenSearch (Elasticsearch)",
	})

	// ============================================================
	// TIER 4: Observability (~3-5% of spend)
	// ============================================================

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_cloudwatch_log_group",
		Behavior:                   CostUsageBased,
		MapperExists:               true,
		EstimatedSpendContribution: 2.0,
		Notes:                      "CloudWatch Logs - ingestion + storage",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_cloudwatch_metric_alarm",
		Behavior:                   CostDirect,
		MapperExists:               true,
		EstimatedSpendContribution: 0.5,
		Notes:                      "CloudWatch alarms per metric",
	})

	// ============================================================
	// TIER 5: Networking (~2-4% of spend)
	// ============================================================

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_vpc_endpoint",
		Behavior:                   CostDirect,
		MapperExists:               false,
		EstimatedSpendContribution: 1.0,
		Notes:                      "Interface VPC endpoints cost",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_eip",
		Behavior:                   CostDirect,
		MapperExists:               false,
		EstimatedSpendContribution: 0.3,
		Notes:                      "Idle EIPs cost money",
	})

	// ============================================================
	// INDIRECT COST (Enable costs elsewhere but free themselves)
	// ============================================================

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_vpc",
		Behavior:                   CostIndirect,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "VPC itself is free",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_subnet",
		Behavior:                   CostIndirect,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "Subnets are free",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_security_group",
		Behavior:                   CostIndirect,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "Security groups are free",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_security_group_rule",
		Behavior:                   CostIndirect,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "SG rules are free",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_route_table",
		Behavior:                   CostIndirect,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "Route tables are free",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_route",
		Behavior:                   CostIndirect,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "Routes are free",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_internet_gateway",
		Behavior:                   CostIndirect,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "IGW is free (data transfer costs)",
	})

	// ============================================================
	// IAM (All free)
	// ============================================================

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_iam_role",
		Behavior:                   CostFree,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "IAM roles are free",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_iam_policy",
		Behavior:                   CostFree,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "IAM policies are free",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_iam_role_policy_attachment",
		Behavior:                   CostFree,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "IAM attachments are free",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_iam_instance_profile",
		Behavior:                   CostFree,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "Instance profiles are free",
	})

	// ============================================================
	// AUTOSCALING
	// ============================================================

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_autoscaling_group",
		Behavior:                   CostIndirect,
		MapperExists:               true,
		EstimatedSpendContribution: 0.0,
		Notes:                      "ASG is free - cost is in launched instances",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_launch_template",
		Behavior:                   CostIndirect,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "Launch template is config",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_appautoscaling_target",
		Behavior:                   CostFree,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "Application autoscaling is free",
	})

	reg.Register(ResourceCostProfile{
		ResourceType:               "aws_appautoscaling_policy",
		Behavior:                   CostFree,
		MapperExists:               false,
		EstimatedSpendContribution: 0.0,
		Notes:                      "Autoscaling policies are free",
	})
}

// DefaultAWSRegistry returns a registry with all AWS profiles
func DefaultAWSRegistry() *Registry {
	reg := NewRegistry()
	RegisterAWS(reg)
	return reg
}
