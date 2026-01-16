// Package catalog - GCP authoritative catalog
// This is the source of truth for GCP resource coverage.
package catalog

// RegisterGCP populates the catalog with all GCP resources
func RegisterGCP(c *Catalog) {
	// ============================================
	// TIER 1 - NUMERIC COST DRIVERS
	// ============================================

	// Compute
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_compute_instance", Tier: Tier1Numeric, Behavior: CostDirect, Category: "compute", MapperExists: true})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_compute_disk", Tier: Tier1Numeric, Behavior: CostDirect, Category: "storage", MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_compute_snapshot", Tier: Tier1Numeric, Behavior: CostDirect, Category: "storage", MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_compute_image", Tier: Tier1Numeric, Behavior: CostDirect, Category: "storage", MapperExists: false})

	// Kubernetes
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_container_cluster", Tier: Tier1Numeric, Behavior: CostDirect, Category: "containers", MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_container_node_pool", Tier: Tier1Numeric, Behavior: CostDirect, Category: "containers", MapperExists: false})

	// Storage
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_storage_bucket", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "storage", RequiresUsage: true, MapperExists: false})

	// Database
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_sql_database_instance", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_spanner_instance", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_bigtable_instance", Tier: Tier1Numeric, Behavior: CostDirect, Category: "database", MapperExists: false})

	// Big Data
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_bigquery_dataset", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "analytics", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_bigquery_table", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "analytics", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_dataflow_job", Tier: Tier1Numeric, Behavior: CostDirect, Category: "analytics", MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_dataproc_cluster", Tier: Tier1Numeric, Behavior: CostDirect, Category: "analytics", MapperExists: false})

	// Serverless
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_cloudfunctions_function", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "serverless", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_cloud_run_service", Tier: Tier1Numeric, Behavior: CostUsageBased, Category: "serverless", RequiresUsage: true, MapperExists: false})

	// Networking
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_compute_router_nat", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_compute_forwarding_rule", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_compute_vpn_gateway", Tier: Tier1Numeric, Behavior: CostDirect, Category: "networking", MapperExists: false})

	// ============================================
	// TIER 2 - SYMBOLIC/USAGE-DEPENDENT
	// ============================================

	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_pubsub_topic", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "messaging", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_pubsub_subscription", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "messaging", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_monitoring_metric_descriptor", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "monitoring", RequiresUsage: true, MapperExists: false})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_logging_project_sink", Tier: Tier2Symbolic, Behavior: CostUsageBased, Category: "monitoring", RequiresUsage: true, MapperExists: false})

	// ============================================
	// TIER 3 - INDIRECT / ZERO-COST
	// ============================================

	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_compute_network", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "networking", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_compute_subnetwork", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "networking", Notes: "No direct cost"})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_dns_managed_zone", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "dns", Notes: "Query-based"})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_dns_record_set", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "dns", Notes: "Query-based"})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_kms_crypto_key", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "security", Notes: "Minimal cost"})
	c.Register(ResourceEntry{Cloud: GCP, ResourceType: "google_secret_manager_secret", Tier: Tier3Indirect, Behavior: CostIndirect, Category: "security", Notes: "Minimal cost"})
}
