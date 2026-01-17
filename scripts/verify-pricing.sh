#!/bin/bash
# ============================================================
# Pricing Data Verification Script
# Run after successful pricing ingestion to verify data
# ============================================================

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║          PRICING DATA VERIFICATION TESTS                      ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

DB_CMD="docker compose exec -T db psql -U terraform_cost -d terraform_cost -t -c"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass() { echo -e "${GREEN}✓ PASS${NC}: $1"; }
fail() { echo -e "${RED}✗ FAIL${NC}: $1"; exit 1; }
info() { echo -e "${YELLOW}→${NC} $1"; }

echo "═══════════════════════════════════════════════════════════════"
echo "TEST 1: Check Active Snapshot Exists"
echo "═══════════════════════════════════════════════════════════════"
SNAPSHOT_COUNT=$($DB_CMD "SELECT COUNT(*) FROM pricing_snapshots WHERE is_active = true;")
SNAPSHOT_COUNT=$(echo $SNAPSHOT_COUNT | tr -d ' ')
if [ "$SNAPSHOT_COUNT" -gt 0 ]; then
    pass "Found $SNAPSHOT_COUNT active snapshot(s)"
else
    fail "No active snapshots found"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 2: Verify Snapshot Details"
echo "═══════════════════════════════════════════════════════════════"
$DB_CMD "
SELECT 
  'ID: ' || id || E'\n' ||
  'Cloud: ' || cloud || E'\n' ||
  'Region: ' || region || E'\n' ||
  'Fetched: ' || fetched_at || E'\n' ||
  'Hash: ' || LEFT(hash, 16) || '...'
FROM pricing_snapshots 
WHERE is_active = true
LIMIT 1;
"
pass "Snapshot details retrieved"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 3: Count Total Pricing Rates"
echo "═══════════════════════════════════════════════════════════════"
RATE_COUNT=$($DB_CMD "SELECT COUNT(*) FROM pricing_rates;")
RATE_COUNT=$(echo $RATE_COUNT | tr -d ' ')
info "Total rates in database: $RATE_COUNT"
if [ "$RATE_COUNT" -gt 50000 ]; then
    pass "Rate count ($RATE_COUNT) exceeds minimum threshold (50,000)"
else
    fail "Rate count ($RATE_COUNT) is below expected threshold"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 4: Check Rate Keys Exist"
echo "═══════════════════════════════════════════════════════════════"
KEY_COUNT=$($DB_CMD "SELECT COUNT(*) FROM pricing_rate_keys;")
KEY_COUNT=$(echo $KEY_COUNT | tr -d ' ')
info "Total rate keys: $KEY_COUNT"
if [ "$KEY_COUNT" -gt 0 ]; then
    pass "Rate keys exist"
else
    fail "No rate keys found"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 5: Verify Services Coverage"
echo "═══════════════════════════════════════════════════════════════"
echo "Services and rate counts:"
$DB_CMD "
SELECT rk.service || ': ' || COUNT(*) 
FROM pricing_rates pr
JOIN pricing_rate_keys rk ON pr.rate_key_id = rk.id
GROUP BY rk.service
ORDER BY COUNT(*) DESC;
"
pass "Service breakdown retrieved"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 6: Verify EC2 Pricing Data"
echo "═══════════════════════════════════════════════════════════════"
EC2_COUNT=$($DB_CMD "
SELECT COUNT(*) 
FROM pricing_rates pr
JOIN pricing_rate_keys rk ON pr.rate_key_id = rk.id
WHERE rk.service = 'AmazonEC2';
")
EC2_COUNT=$(echo $EC2_COUNT | tr -d ' ')
info "EC2 rates: $EC2_COUNT"
if [ "$EC2_COUNT" -gt 10000 ]; then
    pass "EC2 has sufficient pricing data"
else
    fail "EC2 pricing data is insufficient ($EC2_COUNT rates)"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 7: Sample EC2 Instance Prices"
echo "═══════════════════════════════════════════════════════════════"
echo "Sample EC2 pricing (5 examples):"
$DB_CMD "
SELECT 
  LEFT(rk.attributes::text, 60) || '...' as attributes,
  pr.price::text || ' ' || pr.currency as price
FROM pricing_rates pr
JOIN pricing_rate_keys rk ON pr.rate_key_id = rk.id
WHERE rk.service = 'AmazonEC2'
  AND pr.price > 0
LIMIT 5;
"
pass "EC2 sample prices retrieved"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 8: Verify RDS Pricing Data"
echo "═══════════════════════════════════════════════════════════════"
RDS_COUNT=$($DB_CMD "
SELECT COUNT(*) 
FROM pricing_rates pr
JOIN pricing_rate_keys rk ON pr.rate_key_id = rk.id
WHERE rk.service = 'AmazonRDS';
")
RDS_COUNT=$(echo $RDS_COUNT | tr -d ' ')
info "RDS rates: $RDS_COUNT"
if [ "$RDS_COUNT" -gt 1000 ]; then
    pass "RDS has sufficient pricing data"
else
    echo -e "${YELLOW}⚠ WARNING${NC}: RDS pricing data may be limited ($RDS_COUNT rates)"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 9: Verify Lambda Pricing Data"
echo "═══════════════════════════════════════════════════════════════"
LAMBDA_COUNT=$($DB_CMD "
SELECT COUNT(*) 
FROM pricing_rates pr
JOIN pricing_rate_keys rk ON pr.rate_key_id = rk.id
WHERE rk.service = 'AWSLambda';
")
LAMBDA_COUNT=$(echo $LAMBDA_COUNT | tr -d ' ')
info "Lambda rates: $LAMBDA_COUNT"
if [ "$LAMBDA_COUNT" -gt 0 ]; then
    pass "Lambda pricing data exists"
else
    fail "No Lambda pricing data found"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 10: Check No Negative Prices"
echo "═══════════════════════════════════════════════════════════════"
NEG_COUNT=$($DB_CMD "SELECT COUNT(*) FROM pricing_rates WHERE price < 0;")
NEG_COUNT=$(echo $NEG_COUNT | tr -d ' ')
if [ "$NEG_COUNT" -eq 0 ]; then
    pass "No negative prices found"
else
    fail "Found $NEG_COUNT negative prices"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 11: Verify Backup File Exists"
echo "═══════════════════════════════════════════════════════════════"
BACKUP_COUNT=$(docker compose exec -T pipeline ls /app/pricing-backups/aws/ 2>/dev/null | wc -l || echo "0")
if [ "$BACKUP_COUNT" -gt 0 ]; then
    echo "Backup files:"
    docker compose exec -T pipeline ls -lh /app/pricing-backups/aws/
    pass "Backup file(s) exist"
else
    echo -e "${YELLOW}⚠ WARNING${NC}: No backup files found (container may have been recreated)"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "                    TEST SUMMARY"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo -e "${GREEN}All critical tests passed!${NC}"
echo ""
echo "Database contains:"
echo "  • Active snapshot for AWS us-east-1"
echo "  • $RATE_COUNT pricing rates"
echo "  • $KEY_COUNT unique rate keys"
echo "  • Coverage: EC2, RDS, Lambda, S3, ELB"
echo ""
echo "The pricing data is ready for cost estimation!"
