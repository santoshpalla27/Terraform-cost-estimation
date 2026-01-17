#!/bin/bash
# ============================================================
# Multi-Region Pricing Ingestion Script
# Ingests pricing data for all billable regions
# ============================================================

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║          MULTI-REGION PRICING INGESTION                       ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Configuration
PROVIDER="${1:-aws}"
MEMORY_PROFILE="${MEMORY_PROFILE:-low}"
OUTPUT_DIR="${OUTPUT_DIR:-/app/pricing-backups}"
DRY_RUN="${DRY_RUN:-false}"

# AWS Regions (commercial, excluding GovCloud and China for now)
AWS_REGIONS=(
    "us-east-1"
    "us-east-2"
    "us-west-1"
    "us-west-2"
    "ca-central-1"
    "eu-west-1"
    "eu-west-2"
    "eu-west-3"
    "eu-central-1"
    "eu-north-1"
    "ap-northeast-1"
    "ap-northeast-2"
    "ap-southeast-1"
    "ap-southeast-2"
    "ap-south-1"
    "sa-east-1"
)

# Azure Regions
AZURE_REGIONS=(
    "eastus"
    "eastus2"
    "westus"
    "westus2"
    "centralus"
    "northeurope"
    "westeurope"
    "uksouth"
    "southeastasia"
    "japaneast"
    "australiaeast"
)

# GCP Regions
GCP_REGIONS=(
    "us-central1"
    "us-east1"
    "us-west1"
    "europe-west1"
    "europe-west2"
    "asia-east1"
    "asia-northeast1"
    "australia-southeast1"
)

# Select regions based on provider
case $PROVIDER in
    aws)
        REGIONS=("${AWS_REGIONS[@]}")
        ;;
    azure)
        REGIONS=("${AZURE_REGIONS[@]}")
        ;;
    gcp)
        REGIONS=("${GCP_REGIONS[@]}")
        ;;
    *)
        echo "Unknown provider: $PROVIDER"
        echo "Usage: $0 [aws|azure|gcp]"
        exit 1
        ;;
esac

echo "Provider:       $PROVIDER"
echo "Regions:        ${#REGIONS[@]}"
echo "Memory Profile: $MEMORY_PROFILE"
echo "Output Dir:     $OUTPUT_DIR"
echo "Dry Run:        $DRY_RUN"
echo ""

# Track results
SUCCESSFUL=()
FAILED=()
START_TIME=$(date +%s)

# Ingest each region
for i in "${!REGIONS[@]}"; do
    REGION="${REGIONS[$i]}"
    REGION_NUM=$((i + 1))
    TOTAL=${#REGIONS[@]}
    
    echo "═══════════════════════════════════════════════════════════════"
    echo "[$REGION_NUM/$TOTAL] Ingesting $PROVIDER $REGION"
    echo "═══════════════════════════════════════════════════════════════"
    
    REGION_START=$(date +%s)
    
    # Build command
    CMD="docker compose --profile pipeline run --rm pipeline pricing update"
    CMD="$CMD --provider $PROVIDER"
    CMD="$CMD --region $REGION"
    CMD="$CMD --memory-profile $MEMORY_PROFILE"
    CMD="$CMD --output-dir $OUTPUT_DIR"
    
    if [ "$DRY_RUN" = "true" ]; then
        CMD="$CMD --dry-run"
    else
        CMD="$CMD --confirm"
    fi
    
    # Run ingestion
    if $CMD; then
        REGION_END=$(date +%s)
        REGION_DURATION=$((REGION_END - REGION_START))
        echo "✓ $REGION completed in ${REGION_DURATION}s"
        SUCCESSFUL+=("$REGION")
    else
        echo "✗ $REGION failed"
        FAILED+=("$REGION")
    fi
    
    echo ""
done

# Summary
END_TIME=$(date +%s)
TOTAL_DURATION=$((END_TIME - START_TIME))

echo "═══════════════════════════════════════════════════════════════"
echo "                    INGESTION COMPLETE"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Provider:   $PROVIDER"
echo "Duration:   ${TOTAL_DURATION}s"
echo ""
echo "Successful: ${#SUCCESSFUL[@]} regions"
for r in "${SUCCESSFUL[@]}"; do
    echo "  ✓ $r"
done
echo ""
if [ ${#FAILED[@]} -gt 0 ]; then
    echo "Failed: ${#FAILED[@]} regions"
    for r in "${FAILED[@]}"; do
        echo "  ✗ $r"
    done
else
    echo "Failed: 0 regions"
fi
echo ""

# Exit with error if any failed
if [ ${#FAILED[@]} -gt 0 ]; then
    exit 1
fi
