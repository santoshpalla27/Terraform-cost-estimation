// Package cmd - CLI command: terraform-cost pricing restore
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"terraform-cost/db/ingestion"

	"github.com/spf13/cobra"
)

var pricingRestoreCmd = &cobra.Command{
	Use:   "restore <backup-file>",
	Short: "Restore pricing data from backup",
	Long: `Restore pricing data from a local backup file.

This command:
  1. Reads the backup file (gzipped JSON)
  2. Validates content hash and structure
  3. Runs all governance checks
  4. Creates a NEW snapshot (never overwrites existing)
  5. Atomically commits to database

IMPORTANT: This creates a new snapshot ID, not a rollback.`,
	Args: cobra.ExactArgs(1),
	RunE: runPricingRestore,
}

var (
	restoreDryRun bool
	restoreForce  bool
)

func init() {
	pricingCmd.AddCommand(pricingRestoreCmd)

	pricingRestoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "Validate only, no database writes")
	pricingRestoreCmd.Flags().BoolVar(&restoreForce, "force", false, "Skip coverage decrease check")
}

func runPricingRestore(cmd *cobra.Command, args []string) error {
	backupPath := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Print header
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           PRICING DATA RESTORE FROM BACKUP                   ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Printf("Backup file: %s\n", backupPath)
	fmt.Printf("Dry-run:     %t\n", restoreDryRun)
	fmt.Printf("Force:       %t\n", restoreForce)
	fmt.Println("")

	// Read backup
	fmt.Println("Reading backup file...")
	backupMgr := ingestion.NewBackupManager()
	backup, err := backupMgr.ReadBackup(backupPath)
	if err != nil {
		fmt.Printf("✗ Failed to read backup: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Backup read successfully\n")
	fmt.Println("")

	// Print backup info
	fmt.Println("Backup details:")
	fmt.Printf("  Provider:      %s\n", backup.Provider)
	fmt.Printf("  Region:        %s\n", backup.Region)
	fmt.Printf("  Alias:         %s\n", backup.Alias)
	fmt.Printf("  Timestamp:     %s\n", backup.Timestamp.Format(time.RFC3339))
	fmt.Printf("  Rate count:    %d\n", backup.RateCount)
	fmt.Printf("  Content hash:  %s\n", truncateHash(backup.ContentHash))
	fmt.Printf("  Schema:        %s\n", backup.SchemaVersion)
	fmt.Println("")

	// Run validation
	fmt.Println("Running governance validation...")
	validator := ingestion.NewIngestionValidator()
	if restoreForce {
		validator.SetMinCoveragePercent(0) // Skip coverage check
	}

	if err := validator.ValidateAll(backup.Rates, 0); err != nil {
		fmt.Printf("✗ Validation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Validation passed")
	fmt.Println("")

	// Dry-run check
	if restoreDryRun {
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("✓ DRY-RUN COMPLETED - No database changes made")
		fmt.Println("═══════════════════════════════════════════════════════════════")
		return nil
	}

	// Get database store
	store, err := getDBStore()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Check for existing snapshot with same hash
	existing, _ := store.FindSnapshotByHash(ctx, backup.Provider, backup.Region, backup.Alias, backup.ContentHash)
	if existing != nil {
		fmt.Printf("⚠ Snapshot with this content hash already exists: %s\n", existing.ID.String())
		fmt.Println("  No action needed - data is already current")
		return nil
	}

	// Create restore pipeline config
	fmt.Println("Creating new snapshot...")

	// For restore, we need to directly commit the rates
	// This would normally go through the pipeline, but we already have normalized rates
	fmt.Println("✗ Database commit not yet implemented")
	fmt.Println("  Backup validation successful - would create snapshot from %d rates", backup.RateCount)
	os.Exit(1)

	return nil
}
