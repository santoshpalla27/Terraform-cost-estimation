// Package config provides configuration management.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"terraform-cost/core/types"
	"terraform-cost/internal/logging"
)

// Config is the main application configuration
type Config struct {
	// Version is the configuration version
	Version string `json:"version"`

	// Pricing contains pricing configuration
	Pricing PricingConfig `json:"pricing"`

	// Output contains output configuration
	Output OutputConfig `json:"output"`

	// Cache contains cache configuration
	Cache CacheConfig `json:"cache"`

	// Logging contains logging configuration
	Logging logging.Config `json:"logging"`

	// AWS contains AWS-specific configuration
	AWS AWSConfig `json:"aws,omitempty"`

	// Azure contains Azure-specific configuration  
	Azure AzureConfig `json:"azure,omitempty"`

	// GCP contains GCP-specific configuration
	GCP GCPConfig `json:"gcp,omitempty"`
}

// PricingConfig contains pricing-related settings
type PricingConfig struct {
	// DefaultCurrency is the default currency
	DefaultCurrency types.Currency `json:"default_currency"`

	// CacheEnabled enables pricing caching
	CacheEnabled bool `json:"cache_enabled"`

	// CacheTTLSeconds is how long to cache prices
	CacheTTLSeconds int `json:"cache_ttl_seconds"`

	// DatabasePath is the path to the pricing database
	DatabasePath string `json:"database_path"`

	// RefreshOnStart refreshes pricing on startup
	RefreshOnStart bool `json:"refresh_on_start"`
}

// OutputConfig contains output-related settings
type OutputConfig struct {
	// DefaultFormat is the default output format
	DefaultFormat string `json:"default_format"`

	// ShowDetails shows detailed cost breakdown
	ShowDetails bool `json:"show_details"`

	// ShowConfidence shows confidence scores
	ShowConfidence bool `json:"show_confidence"`

	// GroupBy is the default grouping
	GroupBy string `json:"group_by"`
}

// CacheConfig contains cache-related settings
type CacheConfig struct {
	// Enabled enables caching
	Enabled bool `json:"enabled"`

	// Directory is the cache directory
	Directory string `json:"directory"`

	// MaxSizeMB is the maximum cache size in MB
	MaxSizeMB int `json:"max_size_mb"`
}

// AWSConfig contains AWS-specific settings
type AWSConfig struct {
	// DefaultRegion is the default AWS region
	DefaultRegion string `json:"default_region"`

	// Profile is the AWS profile to use
	Profile string `json:"profile,omitempty"`

	// Regions to include in pricing
	Regions []string `json:"regions,omitempty"`
}

// AzureConfig contains Azure-specific settings
type AzureConfig struct {
	// DefaultRegion is the default Azure region
	DefaultRegion string `json:"default_region"`

	// SubscriptionID is the Azure subscription
	SubscriptionID string `json:"subscription_id,omitempty"`
}

// GCPConfig contains GCP-specific settings
type GCPConfig struct {
	// DefaultRegion is the default GCP region
	DefaultRegion string `json:"default_region"`

	// Project is the GCP project
	Project string `json:"project,omitempty"`
}

// Default returns a default configuration
func Default() *Config {
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".terraform-cost", "cache")
	dbPath := filepath.Join(homeDir, ".terraform-cost", "pricing.db")

	return &Config{
		Version: "1.0",
		Pricing: PricingConfig{
			DefaultCurrency: types.CurrencyUSD,
			CacheEnabled:    true,
			CacheTTLSeconds: 86400, // 24 hours
			DatabasePath:    dbPath,
			RefreshOnStart:  false,
		},
		Output: OutputConfig{
			DefaultFormat:  "cli",
			ShowDetails:    true,
			ShowConfidence: false,
			GroupBy:        "resource",
		},
		Cache: CacheConfig{
			Enabled:   true,
			Directory: cacheDir,
			MaxSizeMB: 100,
		},
		Logging: logging.DefaultConfig(),
		AWS: AWSConfig{
			DefaultRegion: "us-east-1",
		},
		Azure: AzureConfig{
			DefaultRegion: "eastus",
		},
		GCP: GCPConfig{
			DefaultRegion: "us-central1",
		},
	}
}

// Load loads configuration from a file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}

	config := Default()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// Save saves configuration to a file
func (c *Config) Save(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Global configuration instance
var globalConfig = Default()

// Get returns the global configuration
func Get() *Config {
	return globalConfig
}

// Set sets the global configuration
func Set(config *Config) {
	globalConfig = config
}
