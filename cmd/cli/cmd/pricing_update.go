// Package cmd - CLI command: terraform-cost pricing update
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"terraform-cost/db"
	"terraform-cost/db/ingestion"

	"github.com/spf13/cobra"
)

var pricingCmd = &cobra.Command{
	Use:   "pricing",
	Short: "Pricing data management commands",
	Long:  "Commands for updating, restoring, and managing pricing data snapshots.",
}

var pricingUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update pricing data from cloud provider",
	Long: `Manually trigger pricing data ingestion from cloud provider APIs.

This command runs a 5-phase pipeline:
  1. FETCH     - Download raw pricing from cloud API (no DB writes)
  2. NORMALIZE - Transform to canonical dimensions
  3. VALIDATE  - Governance checks (abort on failure)
  4. BACKUP    - Write local snapshot dump
  5. COMMIT    - Atomic database transaction

IMPORTANT: This command is for operators only. Never run automatically.`,
	RunE: runPricingUpdate,
}

var (
	updateProvider  string
	updateRegion    string
	updateDryRun    bool
	updateOutputDir string
	updateAlias     string
	updateTimeout   time.Duration
)

func init() {
	// Add pricing command to root
	rootCmd.AddCommand(pricingCmd)
	pricingCmd.AddCommand(pricingUpdateCmd)

	// Flags for update command
	pricingUpdateCmd.Flags().StringVarP(&updateProvider, "provider", "p", "", "Cloud provider (aws, azure, gcp)")
	pricingUpdateCmd.Flags().StringVarP(&updateRegion, "region", "r", "", "Region to ingest (e.g., us-east-1)")
	pricingUpdateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "Validate only, no database writes")
	pricingUpdateCmd.Flags().StringVarP(&updateOutputDir, "output-dir", "o", "./pricing-backups", "Directory for backup files")
	pricingUpdateCmd.Flags().StringVar(&updateAlias, "alias", "default", "Provider alias for multi-account")
	pricingUpdateCmd.Flags().DurationVar(&updateTimeout, "timeout", 30*time.Minute, "Timeout for the pipeline")

	pricingUpdateCmd.MarkFlagRequired("provider")
	pricingUpdateCmd.MarkFlagRequired("region")
}

func runPricingUpdate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
	defer cancel()

	// Parse provider
	var provider db.CloudProvider
	switch updateProvider {
	case "aws":
		provider = db.AWS
	case "azure":
		provider = db.Azure
	case "gcp":
		provider = db.GCP
	default:
		return fmt.Errorf("unsupported provider: %s (use aws, azure, or gcp)", updateProvider)
	}

	// Print header
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           MANUAL PRICING DATA INGESTION PIPELINE             ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Printf("Provider:   %s\n", provider)
	fmt.Printf("Region:     %s\n", updateRegion)
	fmt.Printf("Alias:      %s\n", updateAlias)
	fmt.Printf("Dry-run:    %t\n", updateDryRun)
	fmt.Printf("Output:     %s\n", updateOutputDir)
	fmt.Println("")

	// Create fetcher and normalizer based on provider
	var fetcher ingestion.PriceFetcher
	var normalizer ingestion.PriceNormalizer

	switch provider {
	case db.AWS:
		fetcher = ingestion.NewAWSPriceFetcher()
		normalizer = ingestion.NewAWSPriceNormalizer()
	case db.Azure:
		fetcher = ingestion.NewAzurePriceFetcher()
		normalizer = ingestion.NewAzurePriceNormalizer()
	case db.GCP:
		fetcher = ingestion.NewGCPPriceFetcher()
		normalizer = ingestion.NewGCPPriceNormalizer()
	default:
		return fmt.Errorf("provider %s not implemented", provider)
	}

	// Get database store (placeholder - would come from config)
	store, err := getDBStore()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create pipeline
	pipeline := ingestion.NewPipeline(fetcher, normalizer, store)

	// Configure
	config := &ingestion.PipelineConfig{
		Provider:           provider,
		Region:             updateRegion,
		Alias:              updateAlias,
		DryRun:             updateDryRun,
		BackupDir:          updateOutputDir,
		MinCoveragePercent: 95.0,
		Timeout:            updateTimeout,
	}

	// Execute
	fmt.Println("Starting pipeline...")
	fmt.Println("")
	result, err := pipeline.Execute(ctx, config)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Print results
	printPipelineResult(result)

	// Exit with error if failed
	if !result.Success {
		os.Exit(1)
	}

	return nil
}

func printPipelineResult(result *ingestion.PipelineResult) {
	fmt.Println("═══════════════════════════════════════════════════════════════")
	
	if result.Success {
		fmt.Println("✓ PIPELINE COMPLETED SUCCESSFULLY")
	} else {
		fmt.Printf("✗ PIPELINE FAILED at phase: %s\n", result.FailedPhase)
		fmt.Printf("  Error: %s\n", result.Error)
	}
	
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("")

	// Phases
	fmt.Println("Phases completed:")
	phases := []ingestion.Phase{
		ingestion.PhaseFetch,
		ingestion.PhaseNormalize,
		ingestion.PhaseValidate,
		ingestion.PhaseBackup,
		ingestion.PhaseCommit,
	}
	for _, phase := range phases {
		status := "⬜"
		for _, completed := range result.PhasesCompleted {
			if completed == phase {
				status = "✓"
				break
			}
		}
		if phase == result.FailedPhase {
			status = "✗"
		}
		fmt.Printf("  %s %s\n", status, phase)
	}
	fmt.Println("")

	// Stats
	fmt.Println("Statistics:")
	fmt.Printf("  Raw prices fetched:     %d\n", result.Stats.RawPricesCount)
	fmt.Printf("  Normalized rates:       %d\n", result.Stats.NormalizedRatesCount)
	fmt.Printf("  Unique services:        %d\n", result.Stats.UniqueServicesCount)
	fmt.Printf("  Content hash:           %s\n", truncateHash(result.Stats.ContentHash))
	fmt.Println("")

	if result.BackupPath != "" {
		fmt.Printf("Backup written to: %s\n", result.BackupPath)
	}

	if result.SnapshotID != nil {
		fmt.Printf("Snapshot ID: %s\n", result.SnapshotID.String())
	}

	fmt.Printf("\nDuration: %s\n", result.Duration)
}

func truncateHash(hash string) string {
	if len(hash) > 16 {
		return hash[:16] + "..."
	}
	return hash
}

// getDBStore returns the database store (placeholder)
func getDBStore() (db.PricingStore, error) {
	// TODO: Load from config
	// For now, return nil to indicate not implemented
	return nil, fmt.Errorf("database connection not configured - set DATABASE_URL environment variable")
}
