// Package catalog - Azure authoritative catalog
// This is the source of truth for Azure resource coverage.
package catalog

// RegisterAzure populates the catalog with all Azure resources
func RegisterAzure(c *Catalog) {
	// ============================================
	// TIER 1 - NUMERIC COST DRIVERS
	// ============================================

	// Compute
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_linux_virtual_machine", Tier: Tier1Numeric, Behavior: CostDirect, Category: "compute", MapperExists: true})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_windows_virtual_machine", Tier: Tier1Numeric, Behavior: CostDirect, Category: "compute", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_virtual_machine_scale_set", Tier: Tier1Numeric, Behavior: CostDirect, Category: "compute", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_managed_disk", Tier: Tier1Numeric, Behavior: CostDirect, Category: "storage", MapperExists: false})

	// Kubernetes
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_kubernetes_cluster", Tier: Tier1Numeric, Behavior: CostDirect, Category: "containers", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_kubernetes_cluster_node_pool", Tier: Tier1Numeric, Behavior: CostDirect, Category: "containers", MapperExists: false})

	// Storage
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_storage_account", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "storage", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_storage_share", Tier: Tier1Numeric, Behavior: CostDirect, Category: "storage", MapperExists: false})

	// Database
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_sql_database", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_sql_managed_instance", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_postgresql_flexible_server", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_mysql_flexible_server", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_cosmosdb_account", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "database", RequiresUsage: true, MapperExists: false})

	// Networking
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_lb", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_application_gateway", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_vpn_gateway", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_express_route_gateway", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_frontdoor", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "cdn", RequiresUsage: true, MapperExists: false})

	// App / Serverless
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_app_service_plan", Tier: Tier1Numeric, Behavior: CostDirect, Category: "app", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_function_app", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "serverless", RequiresUsage: true, MapperExists: false})

	// Monitoring
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_log_analytics_workspace", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "monitoring", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_application_insights", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "monitoring", RequiresUsage: true, MapperExists: false})

	// ============================================
	// TIER 2 - SYMBOLIC/USAGE-DEPENDENT
	// ============================================

	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_api_management", Tier: Tier2Symbolic, Behavior: CostDirect, Category: "apigateway", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_servicebus_namespace", Tier: Tier2Symbolic, Behavior: CostDirect, Category: "messaging", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_eventgrid_topic", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "messaging", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_data_factory", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "data", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_signalr_service", Tier: Tier2Symbolic, Behavior: CostDirect, Category: "app", MapperExists: false})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_powerbi_embedded", Tier: Tier2Symbolic, Behavior: CostDirect, Category: "analytics", MapperExists: false})

	// ============================================
	// TIER 3 - INDIRECT / ZERO-COST
	// ============================================

	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_virtual_network", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "networking", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_subnet", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "networking", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_network_security_group", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "networking", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_route_table", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "networking", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_dns_zone", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "dns", Notes: "Minimal cost"})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_dns_record_set", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "dns", Notes: "Query-based"})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_private_dns_zone", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "dns", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_user_assigned_identity", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "iam", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: Azure, ResourceType: "azurerm_role_assignment", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "iam", Notes: "No direct cost"})
}
