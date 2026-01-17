// Package cmd - Canonical CLI for pricing operations
// THIS IS THE ONLY WAY to ingest pricing data
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"terraform-cost/db"
	"terraform-cost/db/ingestion"
	"terraform-cost/db/regions"

	"github.com/spf13/cobra"
)

var pricingCmd = &cobra.Command{
	Use:   "pricing",
	Short: "Pricing data management (operator only)",
	Long: `Pricing data management commands.

IMPORTANT: These commands are for operators only.
Never run automatically or in CI/CD pipelines.`,
}

var pricingUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Ingest pricing from cloud API",
	Long: `Manually trigger pricing data ingestion.

This command runs a strict 5-phase lifecycle:
  1. FETCH     - Download from cloud API (NO DB writes)
  2. NORMALIZE - Transform to canonical format (NO DB writes)
  3. VALIDATE  - Governance checks (NO DB writes)
  4. BACKUP    - Write verified local backup (MANDATORY)
  5. COMMIT    - Single atomic DB transaction

CRITICAL: Backup must succeed before any DB write.
CRITICAL: All phases are in-memory until commit.`,
	RunE: runStrictIngestion,
}

var pricingRestoreCmd = &cobra.Command{
	Use:   "restore <backup-file>",
	Short: "Restore pricing from backup",
	Long: `Restore pricing data from a local backup file.

Creates a NEW snapshot (never overwrites existing).
Same validation rules apply as ingestion.`,
	Args: cobra.ExactArgs(1),
	RunE: runStrictRestore,
}

var (
	pricingProvider      string
	pricingRegion        string
	pricingAlias         string
	pricingDryRun        bool
	pricingOutputDir     string
	pricingEnvironment   string
	pricingConfirm       bool
	pricingForce         bool
	pricingTimeout       time.Duration
	pricingMemoryProfile string
	pricingStreaming     bool
)

func init() {
	rootCmd.AddCommand(pricingCmd)
	pricingCmd.AddCommand(pricingUpdateCmd)
	pricingCmd.AddCommand(pricingRestoreCmd)

	// Update command flags - REQUIRED for safety
	pricingUpdateCmd.Flags().StringVarP(&pricingProvider, "provider", "p", "", "Cloud provider (aws, azure, gcp) [REQUIRED]")
	pricingUpdateCmd.Flags().StringVarP(&pricingRegion, "region", "r", "all", "Region to ingest (e.g., us-east-1, or 'all' for all regions)")
	pricingUpdateCmd.Flags().StringVar(&pricingAlias, "alias", "default", "Provider alias for multi-account")
	pricingUpdateCmd.Flags().BoolVar(&pricingDryRun, "dry-run", false, "Validate only, no database writes")
	pricingUpdateCmd.Flags().StringVarP(&pricingOutputDir, "output-dir", "o", "./pricing-backups", "Directory for backup files")
	pricingUpdateCmd.Flags().StringVar(&pricingEnvironment, "environment", "production", "Environment (production, staging, development)")
	pricingUpdateCmd.Flags().BoolVar(&pricingConfirm, "confirm", false, "Confirm you want to modify production pricing [REQUIRED]")
	pricingUpdateCmd.Flags().DurationVar(&pricingTimeout, "timeout", 30*time.Minute, "Timeout for the pipeline")

	// Memory optimization flags
	pricingUpdateCmd.Flags().StringVar(&pricingMemoryProfile, "memory-profile", "auto", "Memory profile: low (4GB), default (8GB), high (16GB+), auto")
	pricingUpdateCmd.Flags().BoolVar(&pricingStreaming, "streaming", true, "Use streaming mode for large datasets (recommended for low-memory)")

	pricingUpdateCmd.MarkFlagRequired("provider")
	// region defaults to 'all' - not required

	// Restore command flags
	pricingRestoreCmd.Flags().BoolVar(&pricingDryRun, "dry-run", false, "Validate only, no database writes")
	pricingRestoreCmd.Flags().BoolVar(&pricingForce, "force", false, "Skip coverage decrease check")
	pricingRestoreCmd.Flags().BoolVar(&pricingConfirm, "confirm", false, "Confirm you want to modify production pricing")
}

func runStrictIngestion(cmd *cobra.Command, args []string) error {
	// SAFETY: Require explicit confirmation for production
	if pricingEnvironment == "production" && !pricingDryRun && !pricingConfirm {
		fmt.Println("")
		fmt.Println("╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("║                    ⚠️  PRODUCTION WARNING ⚠️                   ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════╝")
		fmt.Println("")
		fmt.Println("You are about to modify PRODUCTION pricing data.")
		fmt.Println("This will affect all cost estimations.")
		fmt.Println("")
		fmt.Print("Type 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), pricingTimeout)
	defer cancel()

	// Parse provider
	var provider db.CloudProvider
	switch pricingProvider {
	case "aws":
		provider = db.AWS
	case "azure":
		provider = db.Azure
	case "gcp":
		provider = db.GCP
	default:
		return fmt.Errorf("unsupported provider: %s (use aws, azure, or gcp)", pricingProvider)
	}

	// Handle --region=all case
	if pricingRegion == "all" {
		return runMultiRegionIngestion(ctx, provider)
	}

	// Single region ingestion
	return runSingleRegionIngestion(ctx, provider, pricingRegion)
}

// runMultiRegionIngestion ingests all billable regions for a provider
func runMultiRegionIngestion(ctx context.Context, provider db.CloudProvider) error {
	registry := regions.NewRegistry()
	billableRegions := registry.GetBillableRegions(provider)

	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          MULTI-REGION PRICING INGESTION                       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Printf("Provider: %s\n", provider)
	fmt.Printf("Regions:  %d billable regions\n", len(billableRegions))
	fmt.Printf("Dry-run:  %v\n", pricingDryRun)
	fmt.Println("")

	var successful, failed []string
	startTime := time.Now()

	for i, region := range billableRegions {
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Printf("[%d/%d] Ingesting %s %s\n", i+1, len(billableRegions), provider, region.Region)
		fmt.Println("═══════════════════════════════════════════════════════════════")

		regionStart := time.Now()
		err := runSingleRegionIngestion(ctx, provider, region.Region)
		
		if err != nil {
			fmt.Printf("✗ %s failed: %v\n", region.Region, err)
			failed = append(failed, region.Region)
		} else {
			fmt.Printf("✓ %s completed in %s\n", region.Region, time.Since(regionStart).Round(time.Second))
			successful = append(successful, region.Region)
		}
		fmt.Println("")
	}

	// Print summary
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("                    INGESTION COMPLETE")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("Duration:   %s\n", time.Since(startTime).Round(time.Second))
	fmt.Printf("Successful: %d regions\n", len(successful))
	fmt.Printf("Failed:     %d regions\n", len(failed))
	
	if len(failed) > 0 {
		fmt.Println("\nFailed regions:")
		for _, r := range failed {
			fmt.Printf("  ✗ %s\n", r)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d regions failed", len(failed))
	}
	return nil
}

// runSingleRegionIngestion ingests a single region
func runSingleRegionIngestion(ctx context.Context, provider db.CloudProvider, region string) error {
	// Print header
	printIngestionHeader(provider)

	// Get production fetcher and normalizer from registry
	fetcher, err := ingestion.GetProductionFetcher(provider)
	if err != nil {
		// In non-production, allow fallback to any registered fetcher
		if pricingEnvironment != "production" {
			fetcher, err = ingestion.GetRegistry().GetFetcher(provider)
		}
		if err != nil {
			return fmt.Errorf("no fetcher available for %s: %w", provider, err)
		}
	}

	normalizer, err := ingestion.GetProductionNormalizer(provider)
	if err != nil {
		return fmt.Errorf("no normalizer available for %s: %w", provider, err)
	}

	// Verify real API in production
	if pricingEnvironment == "production" {
		if realAPI, ok := fetcher.(ingestion.RealAPIFetcher); ok {
			if !realAPI.IsRealAPI() {
				return fmt.Errorf("FATAL: cannot use mock pricing in production environment")
			}
		}
	}

	// Get database store
	store, err := getDBStore()
	if err != nil && !pricingDryRun {
		return fmt.Errorf("database connection required: %w", err)
	}

	// Configure lifecycle
	config := &ingestion.LifecycleConfig{
		Provider:         provider,
		Region:           region,
		Alias:            pricingAlias,
		Environment:      pricingEnvironment,
		BackupDir:        pricingOutputDir,
		DryRun:           pricingDryRun,
		AllowMockPricing: pricingEnvironment != "production",
		MinCoverage:      95.0,
		Timeout:          pricingTimeout,
	}

	var result *ingestion.LifecycleResult

	// Use streaming mode for low-memory environments
	if pricingStreaming {
		streamConfig := getStreamingConfig()
		fmt.Printf("\nMemory profile: %s (batch=%d, maxMem=%dMB)\n",
			pricingMemoryProfile, streamConfig.BatchSize, streamConfig.MaxMemoryMB)

		streamLifecycle := ingestion.NewStreamingLifecycle(fetcher, normalizer, store, streamConfig)
		fmt.Println("Starting STREAMING ingestion lifecycle...")
		fmt.Println("")
		result, err = streamLifecycle.Execute(ctx, config)
	} else {
		// Standard in-memory lifecycle
		lifecycle := ingestion.NewLifecycle(fetcher, normalizer, store)
		fmt.Println("\nStarting strict ingestion lifecycle...")
		fmt.Println("")
		result, err = lifecycle.Execute(ctx, config)
	}

	if err != nil {
		return fmt.Errorf("lifecycle execution failed: %w", err)
	}

	// Print results
	printIngestionResult(result)

	if !result.Success {
		os.Exit(1)
	}

	return nil
}

func runStrictRestore(cmd *cobra.Command, args []string) error {
	backupPath := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// SAFETY: Require explicit confirmation
	if !pricingDryRun && !pricingConfirm {
		fmt.Println("")
		fmt.Println("⚠️  You are about to restore pricing data from backup.")
		fmt.Print("Type 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              PRICING DATA RESTORE FROM BACKUP                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Printf("Backup file: %s\n", backupPath)
	fmt.Printf("Dry-run:     %t\n", pricingDryRun)
	fmt.Println("")

	// Read and validate backup
	fmt.Println("Reading backup file...")
	backupMgr := ingestion.NewBackupManager()
	backup, err := backupMgr.ReadBackup(backupPath)
	if err != nil {
		fmt.Printf("✗ Failed to read backup: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Backup validated: %d rates, hash=%s\n", backup.RateCount, backup.ContentHash[:16])

	// Validate governance
	fmt.Println("\nRunning governance validation...")
	validator := ingestion.NewIngestionValidator()
	if pricingForce {
		validator.SetMinCoveragePercent(0)
	}

	if err := validator.ValidateAll(backup.Rates, 0); err != nil {
		fmt.Printf("✗ Validation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Validation passed")

	if pricingDryRun {
		fmt.Println("\n✓ DRY-RUN COMPLETED - No database changes made")
		return nil
	}

	// Get database and restore
	store, err := getDBStore()
	if err != nil {
		return fmt.Errorf("database connection required: %w", err)
	}

	// Check for existing snapshot
	existing, _ := store.FindSnapshotByHash(ctx, backup.Provider, backup.Region, backup.Alias, backup.ContentHash)
	if existing != nil {
		fmt.Printf("\n⚠ Snapshot with this hash already exists: %s\n", existing.ID.String())
		fmt.Println("  No action needed - data is already current")
		return nil
	}

	fmt.Println("\n✗ Full restore not yet implemented")
	fmt.Printf("  Backup validation successful - would create snapshot from %d rates\n", backup.RateCount)
	os.Exit(1)

	return nil
}

func printIngestionHeader(provider db.CloudProvider) {
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          STRICT PRICING INGESTION LIFECYCLE                  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Printf("Provider:    %s\n", provider)
	fmt.Printf("Region:      %s\n", pricingRegion)
	fmt.Printf("Alias:       %s\n", pricingAlias)
	fmt.Printf("Environment: %s\n", pricingEnvironment)
	fmt.Printf("Dry-run:     %t\n", pricingDryRun)
	fmt.Printf("Output:      %s\n", pricingOutputDir)
}

func printIngestionResult(result *ingestion.LifecycleResult) {
	fmt.Println("═══════════════════════════════════════════════════════════════")

	if result.Success {
		fmt.Println("✓ LIFECYCLE COMPLETED SUCCESSFULLY")
	} else {
		fmt.Printf("✗ LIFECYCLE FAILED at phase: %s\n", result.Phase)
		fmt.Printf("  Error: %s\n", result.Error)
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("")

	// Phase progression
	fmt.Println("Phase progression:")
	phases := []ingestion.IngestionPhase{
		ingestion.PhaseFetching,
		ingestion.PhaseNormalizing,
		ingestion.PhaseValidating,
		ingestion.PhaseStaging,
		ingestion.PhaseBackedUp,
		ingestion.PhaseCommitting,
		ingestion.PhaseActive,
	}

	for _, phase := range phases {
		status := "⬜"
		if result.Phase >= phase {
			status = "✓"
		}
		if result.Phase == ingestion.PhaseFailed && phase > result.Phase {
			status = "⬜"
		}
		fmt.Printf("  %s %s\n", status, phase)
	}
	fmt.Println("")

	// Stats
	fmt.Println("Statistics:")
	fmt.Printf("  Raw prices fetched: %d\n", result.RawCount)
	fmt.Printf("  Normalized rates:   %d\n", result.NormalizedCount)
	if result.ContentHash != "" {
		fmt.Printf("  Content hash:       %s...\n", result.ContentHash[:16])
	}
	fmt.Println("")

	if result.BackupPath != "" {
		fmt.Printf("Backup: %s\n", result.BackupPath)
	}

	if result.SnapshotID != nil {
		fmt.Printf("Snapshot ID: %s\n", result.SnapshotID.String())
	}

	fmt.Printf("\nDuration: %s\n", result.Duration)
}

// getStreamingConfig returns the streaming config based on memory profile
func getStreamingConfig() *ingestion.StreamingConfig {
	switch pricingMemoryProfile {
	case "low":
		return ingestion.LowMemoryConfig()
	case "high":
		return ingestion.HighMemoryConfig()
	case "default":
		return ingestion.DefaultStreamingConfig()
	case "auto":
		// Auto-detect based on available memory
		return autoDetectMemoryConfig()
	default:
		return ingestion.DefaultStreamingConfig()
	}
}

// autoDetectMemoryConfig determines config based on available system memory
func autoDetectMemoryConfig() *ingestion.StreamingConfig {
	// Simple heuristic: on Windows, assume conservative memory
	// In production, could read from /proc/meminfo on Linux
	// For now, default to "default" profile
	return ingestion.DefaultStreamingConfig()
}

// getDBStore returns the database store from environment configuration
func getDBStore() (db.PricingStore, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Try individual environment variables
		host := os.Getenv("DB_HOST")
		if host == "" {
			host = "localhost"
		}
		port := 5432
		user := os.Getenv("DB_USER")
		if user == "" {
			user = "terraform_cost"
		}
		password := os.Getenv("DB_PASSWORD")
		if password == "" {
			password = "terraform_cost_dev"
		}
		database := os.Getenv("DB_NAME")
		if database == "" {
			database = "terraform_cost"
		}
		sslmode := os.Getenv("DB_SSLMODE")
		if sslmode == "" {
			sslmode = "disable"
		}

		return db.NewPostgresStore(db.Config{
			Host:     host,
			Port:     port,
			User:     user,
			Password: password,
			Database: database,
			SSLMode:  sslmode,
		})
	}

	// Parse DATABASE_URL
	// Format: postgres://user:password@host:port/dbname?sslmode=disable
	return db.NewPostgresStoreFromURL(dbURL)
}
