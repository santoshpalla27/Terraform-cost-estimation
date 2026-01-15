// Package cmd provides the CLI commands for terraform-cost.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"terraform-cost/internal/config"
	"terraform-cost/internal/logging"
)

var (
	cfgFile string
	verbose bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "terraform-cost",
	Short: "Estimate costs for Terraform infrastructure",
	Long: `terraform-cost is a cloud-agnostic infrastructure cost estimation tool.

It analyzes Terraform configurations and produces accurate, reproducible
cost estimates with full lineage tracking.

Examples:
  terraform-cost estimate ./my-terraform-project
  terraform-cost estimate --format json ./infrastructure
  terraform-cost diff main..feature-branch`,
}

// Execute runs the CLI
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.terraform-cost.json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(estimateCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
}

func initConfig() {
	if cfgFile != "" {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
		config.Set(cfg)
	}

	// Initialize logging
	cfg := config.Get()
	if verbose {
		cfg.Logging.Level = "debug"
	}
	if err := logging.Initialize(cfg.Logging); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logging: %v\n", err)
	}
}

// versionCmd prints version information
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("terraform-cost version 0.1.0")
	},
}

// configCmd manages configuration
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}
