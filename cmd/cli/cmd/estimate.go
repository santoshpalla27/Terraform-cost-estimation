// Package cmd - estimate command
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"terraform-cost/clouds"
	"terraform-cost/clouds/aws"
	"terraform-cost/core/asset"
	"terraform-cost/core/output"
	"terraform-cost/core/scanner"
	"terraform-cost/core/types"
	"terraform-cost/internal/logging"
)

var (
	outputFormat string
	usageFile    string
	showDetails  bool
	region       string
)

// estimateCmd represents the estimate command
var estimateCmd = &cobra.Command{
	Use:   "estimate [path]",
	Short: "Estimate costs for a Terraform project",
	Long: `Analyze Terraform configurations and produce cost estimates.

The path can be a directory containing .tf files or a Terraform plan JSON file.

Examples:
  terraform-cost estimate .
  terraform-cost estimate ./infrastructure
  terraform-cost estimate --format json ./my-project
  terraform-cost estimate --usage usage.yml ./my-project`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEstimate,
}

func init() {
	estimateCmd.Flags().StringVarP(&outputFormat, "format", "f", "cli", "output format (cli, json, html, markdown)")
	estimateCmd.Flags().StringVarP(&usageFile, "usage", "u", "", "usage file for custom usage estimates")
	estimateCmd.Flags().BoolVarP(&showDetails, "details", "d", true, "show detailed cost breakdown")
	estimateCmd.Flags().StringVarP(&region, "region", "r", "", "default AWS region")
}

func runEstimate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	startTime := time.Now()

	// Determine path
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Validate path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}

	logging.Info("Starting cost estimation")

	// Initialize cloud plugins
	if err := initializePlugins(); err != nil {
		return fmt.Errorf("failed to initialize plugins: %w", err)
	}

	// Create project input
	input := &types.ProjectInput{
		ID:     fmt.Sprintf("estimate-%d", time.Now().Unix()),
		Path:   path,
		Source: types.SourceCLI,
		Metadata: types.InputMetadata{
			Timestamp: time.Now(),
		},
	}

	// Scan the project
	fmt.Println("Scanning Terraform files...")
	scanResult, err := scanner.GetDefault().DetectAndScan(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to scan project: %w", err)
	}

	if scanResult.HasErrors() {
		fmt.Printf("Warning: %d errors during scanning\n", len(scanResult.Errors))
		for _, e := range scanResult.Errors {
			fmt.Printf("  %s:%d: %s\n", e.File, e.Line, e.Message)
		}
	}

	if len(scanResult.Assets) == 0 {
		fmt.Println("No resources found in the project.")
		return nil
	}

	fmt.Printf("Found %d resources\n\n", len(scanResult.Assets))

	// Build asset graph
	graph := buildAssetGraph(ctx, scanResult.Assets)

	// Calculate costs (simplified)
	costGraph := calculateCosts(graph)

	// Create estimation result
	result := &output.EstimationResult{
		CostGraph:  costGraph,
		AssetGraph: graph,
		Confidence: 0.7,
		Metadata: output.EstimationMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			Duration:  time.Since(startTime).String(),
			Version:   "0.1.0",
			Source:    types.SourceCLI,
		},
	}

	// Output results
	printResults(result)

	return nil
}

func initializePlugins() error {
	// Register AWS plugin
	awsPlugin := aws.New()
	if region != "" {
		awsPlugin = aws.NewWithRegion(region)
	}
	
	if err := clouds.RegisterPlugin(awsPlugin); err != nil {
		return err
	}

	return nil
}

func buildAssetGraph(ctx context.Context, rawAssets []types.RawAsset) *types.AssetGraph {
	graph := types.NewAssetGraph()
	builderRegistry := asset.GetDefaultBuilderRegistry()

	// Register AWS builders
	awsPlugin := aws.New()
	for _, builder := range awsPlugin.AssetBuilders() {
		builderRegistry.Register(builder)
	}

	for _, raw := range rawAssets {
		builder, ok := builderRegistry.GetBuilder(raw.Provider, raw.Type)
		if !ok {
			// No builder for this resource type - create a generic asset
			asset := &types.Asset{
				ID:         fmt.Sprintf("%s.%s", raw.Type, raw.Name),
				Address:    raw.Address,
				Provider:   raw.Provider,
				Category:   types.CategoryOther,
				Type:       raw.Type,
				Name:       raw.Name,
				Attributes: raw.Attributes,
				Metadata: types.AssetMetadata{
					Source: raw.SourceFile,
					Line:   raw.SourceLine,
				},
			}
			graph.Add(asset)
			continue
		}

		asset, err := builder.Build(ctx, &raw)
		if err != nil {
			fmt.Printf("Warning: failed to build asset %s: %v\n", raw.Address, err)
			continue
		}

		graph.Add(asset)
	}

	return graph
}

func calculateCosts(graph *types.AssetGraph) *types.CostGraph {
	costGraph := types.NewCostGraph(types.CurrencyUSD)

	graph.Walk(func(asset *types.Asset) error {
		// Calculate cost for this asset
		units := calculateAssetCost(asset)
		for _, unit := range units {
			costGraph.AddCostUnit(unit, asset)
		}
		return nil
	})

	costGraph.Summarize()
	return costGraph
}

func calculateAssetCost(asset *types.Asset) []*types.CostUnit {
	var units []*types.CostUnit

	// Simplified cost calculation based on resource type
	switch asset.Type {
	case "aws_instance":
		instanceType := asset.Attributes.GetString("instance_type")
		if instanceType == "" {
			instanceType = "t3.micro"
		}
		hourlyRate := getEC2HourlyRate(instanceType)
		monthlyHours := decimal.NewFromInt(730)
		monthlyCost := hourlyRate.Mul(monthlyHours)

		units = append(units, &types.CostUnit{
			ID:       fmt.Sprintf("%s-compute", asset.ID),
			Label:    fmt.Sprintf("EC2 Instance (%s)", instanceType),
			Measure:  "hours",
			Quantity: monthlyHours,
			Rate:     hourlyRate,
			Amount:   monthlyCost,
			Currency: types.CurrencyUSD,
			Lineage: types.CostLineage{
				AssetID:      asset.ID,
				AssetAddress: asset.Address,
				Formula:      "hourly_rate * 730 hours/month",
			},
		})

	case "aws_db_instance":
		instanceClass := asset.Attributes.GetString("instance_class")
		if instanceClass == "" {
			instanceClass = "db.t3.micro"
		}
		hourlyRate := getRDSHourlyRate(instanceClass)
		monthlyHours := decimal.NewFromInt(730)
		monthlyCost := hourlyRate.Mul(monthlyHours)

		units = append(units, &types.CostUnit{
			ID:       fmt.Sprintf("%s-compute", asset.ID),
			Label:    fmt.Sprintf("RDS Instance (%s)", instanceClass),
			Measure:  "hours",
			Quantity: monthlyHours,
			Rate:     hourlyRate,
			Amount:   monthlyCost,
			Currency: types.CurrencyUSD,
			Lineage: types.CostLineage{
				AssetID:      asset.ID,
				AssetAddress: asset.Address,
				Formula:      "hourly_rate * 730 hours/month",
			},
		})

		// Add storage cost
		storage := asset.Attributes.GetInt("allocated_storage")
		if storage > 0 {
			storageRate := decimal.NewFromFloat(0.115) // gp2 per GB-month
			storageCost := storageRate.Mul(decimal.NewFromInt(int64(storage)))
			units = append(units, &types.CostUnit{
				ID:       fmt.Sprintf("%s-storage", asset.ID),
				Label:    "RDS Storage (gp2)",
				Measure:  "GB-month",
				Quantity: decimal.NewFromInt(int64(storage)),
				Rate:     storageRate,
				Amount:   storageCost,
				Currency: types.CurrencyUSD,
				Lineage: types.CostLineage{
					AssetID:      asset.ID,
					AssetAddress: asset.Address,
					Formula:      "storage_gb * $0.115/GB-month",
				},
			})
		}

	case "aws_nat_gateway":
		hourlyRate := decimal.NewFromFloat(0.045)
		monthlyHours := decimal.NewFromInt(730)
		monthlyCost := hourlyRate.Mul(monthlyHours)

		units = append(units, &types.CostUnit{
			ID:       fmt.Sprintf("%s-hourly", asset.ID),
			Label:    "NAT Gateway",
			Measure:  "hours",
			Quantity: monthlyHours,
			Rate:     hourlyRate,
			Amount:   monthlyCost,
			Currency: types.CurrencyUSD,
			Lineage: types.CostLineage{
				AssetID:      asset.ID,
				AssetAddress: asset.Address,
				Formula:      "$0.045/hour * 730 hours/month",
			},
		})

	case "aws_ebs_volume":
		volumeType := asset.Attributes.GetString("type")
		if volumeType == "" {
			volumeType = "gp3"
		}
		size := asset.Attributes.GetInt("size")
		if size == 0 {
			size = 8
		}
		rate := getEBSRate(volumeType)
		amount := rate.Mul(decimal.NewFromInt(int64(size)))

		units = append(units, &types.CostUnit{
			ID:       fmt.Sprintf("%s-storage", asset.ID),
			Label:    fmt.Sprintf("EBS Volume (%s)", volumeType),
			Measure:  "GB-month",
			Quantity: decimal.NewFromInt(int64(size)),
			Rate:     rate,
			Amount:   amount,
			Currency: types.CurrencyUSD,
			Lineage: types.CostLineage{
				AssetID:      asset.ID,
				AssetAddress: asset.Address,
				Formula:      fmt.Sprintf("$%.3f/GB-month * %d GB", rate.InexactFloat64(), size),
			},
		})

	case "aws_lambda_function":
		// Lambda free tier: 1M requests, 400K GB-seconds
		// Just show a minimal cost for estimation
		units = append(units, &types.CostUnit{
			ID:       fmt.Sprintf("%s-compute", asset.ID),
			Label:    "Lambda Function (usage-based)",
			Measure:  "invocations",
			Quantity: decimal.NewFromInt(1000000),
			Rate:     decimal.NewFromFloat(0.0000002),
			Amount:   decimal.NewFromFloat(0.20),
			Currency: types.CurrencyUSD,
			Lineage: types.CostLineage{
				AssetID:      asset.ID,
				AssetAddress: asset.Address,
				Formula:      "Usage-based pricing (1M requests estimate)",
				Assumptions:  []string{"Estimated 1M invocations/month"},
			},
		})
	}

	return units
}

func getEC2HourlyRate(instanceType string) decimal.Decimal {
	rates := map[string]float64{
		"t3.micro":   0.0104,
		"t3.small":   0.0208,
		"t3.medium":  0.0416,
		"t3.large":   0.0832,
		"t3.xlarge":  0.1664,
		"m5.large":   0.096,
		"m5.xlarge":  0.192,
		"c5.large":   0.085,
		"r5.large":   0.126,
	}
	if rate, ok := rates[instanceType]; ok {
		return decimal.NewFromFloat(rate)
	}
	return decimal.NewFromFloat(0.10) // Default
}

func getRDSHourlyRate(instanceClass string) decimal.Decimal {
	rates := map[string]float64{
		"db.t3.micro":   0.017,
		"db.t3.small":   0.034,
		"db.t3.medium":  0.068,
		"db.m5.large":   0.171,
		"db.r5.large":   0.24,
	}
	if rate, ok := rates[instanceClass]; ok {
		return decimal.NewFromFloat(rate)
	}
	return decimal.NewFromFloat(0.10) // Default
}

func getEBSRate(volumeType string) decimal.Decimal {
	rates := map[string]float64{
		"gp3": 0.08,
		"gp2": 0.10,
		"io1": 0.125,
		"st1": 0.045,
		"sc1": 0.015,
	}
	if rate, ok := rates[volumeType]; ok {
		return decimal.NewFromFloat(rate)
	}
	return decimal.NewFromFloat(0.10) // Default
}

func printResults(result *output.EstimationResult) {
	fmt.Println("┌─────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                        COST ESTIMATION SUMMARY                         │")
	fmt.Println("├─────────────────────────────────────────────────────────────────────────┤")

	// Print by resource
	for assetID, agg := range result.CostGraph.ByAsset {
		if len(agg.Units) == 0 {
			continue
		}
		fmt.Printf("│ %-50s %20s │\n", 
			truncate(assetID, 50), 
			fmt.Sprintf("$%.2f/month", agg.MonthlyCost.InexactFloat64()))
		
		if showDetails {
			for _, unit := range agg.Units {
				fmt.Printf("│   └─ %-46s %20s │\n",
					truncate(unit.Label, 46),
					fmt.Sprintf("$%.2f", unit.Amount.InexactFloat64()))
			}
		}
	}

	fmt.Println("├─────────────────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ %-50s %20s │\n", 
		"TOTAL MONTHLY ESTIMATE",
		fmt.Sprintf("$%.2f", result.CostGraph.TotalMonthlyCost.InexactFloat64()))
	fmt.Printf("│ %-50s %20s │\n",
		"TOTAL HOURLY ESTIMATE",
		fmt.Sprintf("$%.4f", result.CostGraph.TotalHourlyCost.InexactFloat64()))
	fmt.Println("└─────────────────────────────────────────────────────────────────────────┘")

	fmt.Printf("\nEstimation completed in %s\n", result.Metadata.Duration)
	fmt.Printf("Confidence: %.0f%%\n", result.Confidence*100)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
