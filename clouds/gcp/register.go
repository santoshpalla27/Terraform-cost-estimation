// Package gcp - GCP cloud provider registration
package gcp

// SupportedResourceTypes returns all supported GCP resource types
func SupportedResourceTypes() []string {
	return []string{
		// Compute
		"google_compute_instance",
		"google_compute_instance_template",
		"google_compute_instance_group_manager",

		// Storage
		"google_storage_bucket",
		"google_compute_disk",

		// Database
		"google_sql_database_instance",
		"google_bigtable_instance",
		"google_spanner_instance",

		// Serverless
		"google_cloudfunctions_function",
		"google_cloud_run_service",
	}
}
