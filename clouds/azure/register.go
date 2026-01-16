// Package azure - Azure cloud provider registration
package azure

// SupportedResourceTypes returns all supported Azure resource types
func SupportedResourceTypes() []string {
	return []string{
		// Compute
		"azurerm_virtual_machine",
		"azurerm_linux_virtual_machine",
		"azurerm_windows_virtual_machine",
		"azurerm_virtual_machine_scale_set",

		// Storage
		"azurerm_storage_account",
		"azurerm_managed_disk",

		// Database
		"azurerm_sql_database",
		"azurerm_cosmosdb_account",
		"azurerm_mysql_flexible_server",
		"azurerm_postgresql_flexible_server",

		// Serverless
		"azurerm_function_app",
	}
}
