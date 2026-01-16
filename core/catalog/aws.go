// Package catalog - AWS authoritative catalog
// This is the source of truth for AWS resource coverage.
package catalog

// RegisterAWS populates the catalog with all AWS resources
func RegisterAWS(c *Catalog) {
	// ============================================
	// TIER 1 - NUMERIC COST DRIVERS (~90-95% spend)
	// ============================================

	// Compute
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_instance", Tier: Tier1Numeric, Behavior: CostDirect, Category: "compute", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_launch_template", Tier: Tier1Numeric, Behavior: CostDirect, Category: "compute", MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_autoscaling_group", Tier: Tier1Numeric, Behavior: CostDirect, Category: "compute", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_ec2_host", Tier: Tier1Numeric, Behavior: CostDirect, Category: "compute", MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_eks_cluster", Tier: Tier1Numeric, Behavior: CostDirect, Category: "containers", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_eks_node_group", Tier: Tier1Numeric, Behavior: CostDirect, Category: "containers", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_ecs_service", Tier: Tier1Numeric, Behavior: CostDirect, Category: "containers", MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_ecs_task_definition", Tier: Tier1Numeric, Behavior: CostDirect, Category: "containers", MapperExists: false})

	// Storage
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_ebs_volume", Tier: Tier1Numeric, Behavior: CostDirect, Category: "storage", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_ebs_snapshot", Tier: Tier1Numeric, Behavior: CostDirect, Category: "storage", MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_s3_bucket", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "storage", RequiresUsage: true, MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_efs_file_system", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "storage", RequiresUsage: true, MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_fsx_windows_file_system", Tier: Tier1Numeric, Behavior: CostDirect, Category: "storage", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_fsx_openzfs_file_system", Tier: Tier1Numeric, Behavior: CostDirect, Category: "storage", MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_fsx_lustre_file_system", Tier: Tier1Numeric, Behavior: CostDirect, Category: "storage", MapperExists: true})

	// Database
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_db_instance", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_rds_cluster", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_rds_cluster_instance", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_dynamodb_table", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_elasticache_cluster", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_elasticache_replication_group", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_redshift_cluster", Tier: Tier1Numeric, Behavior: CostDirect, Category: "analytics", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_opensearch_domain", Tier: Tier1Numeric, Behavior: CostDirect, Category: "analytics", MapperExists: true})

	// Networking
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_nat_gateway", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_eip", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_vpc_endpoint", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_dx_connection", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_vpn_connection", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})

	// Load Balancing
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_lb", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_elb", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})

	// Serverless
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_lambda_function", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "serverless", RequiresUsage: true, MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_lambda_provisioned_concurrency_config", Tier: Tier1Numeric, Behavior: CostDirect, Category: "serverless", MapperExists: false})

	// Monitoring
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_cloudwatch_log_group", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "monitoring", RequiresUsage: true, MapperExists: true})

	// Data Transfer / CDN
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_global_accelerator", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_cloudfront_distribution", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "cdn", RequiresUsage: true, MapperExists: false})

	// ============================================
	// TIER 2 - SYMBOLIC/USAGE-DEPENDENT
	// ============================================

	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_api_gateway_rest_api", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "apigateway", RequiresUsage: true, MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_api_gateway_stage", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "apigateway", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_apigatewayv2_api", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "apigateway", RequiresUsage: true, MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_sfn_state_machine", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "serverless", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_kinesis_stream", Tier: Tier2Symbolic, Behavior: CostDirect, Category: "streaming", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_kinesis_firehose_delivery_stream", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "streaming", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_msk_cluster", Tier: Tier2Symbolic, Behavior: CostDirect, Category: "streaming", MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_sqs_queue", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "messaging", RequiresUsage: true, MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_sns_topic", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "messaging", RequiresUsage: true, MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_cloudtrail", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "monitoring", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_backup_vault", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "backup", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_ecr_repository", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "containers", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_route53_zone", Tier: Tier2Symbolic, Behavior: CostDirect, Category: "dns", MapperExists: true})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_route53_record", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "dns", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_waf_web_acl", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "security", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_wafv2_web_acl", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "security", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_secretsmanager_secret", Tier: Tier2Symbolic, Behavior: CostDirect, Category: "security", MapperExists: true})

	// ============================================
	// TIER 3 - INDIRECT / ZERO-COST
	// ============================================

	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_vpc", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "networking", Notes: "No direct cost, enables other resources"})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_subnet", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "networking", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_route_table", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "networking", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_security_group", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "networking", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_iam_role", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "iam", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_iam_policy", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "iam", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_network_interface", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "networking", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_appautoscaling_target", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "compute", Notes: "Affects capacity, no direct cost"})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_cloudformation_stack", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "infra", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_cloudformation_stack_set", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "infra", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: AWS, ResourceType: "aws_ssm_parameter", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "config", Notes: "Free for standard tier"})
}
