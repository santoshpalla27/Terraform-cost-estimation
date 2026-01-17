#!/bin/bash
# ============================================================
# Pricing Data Verification Script
# Run after successful pricing ingestion to verify data
# Tests all regions with active snapshots
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
CYAN='\033[0;36m'
NC='\033[0m' # No Color

pass() { echo -e "${GREEN}✓ PASS${NC}: $1"; }
fail() { echo -e "${RED}✗ FAIL${NC}: $1"; exit 1; }
warn() { echo -e "${YELLOW}⚠ WARNING${NC}: $1"; }
info() { echo -e "${CYAN}→${NC} $1"; }

# ============================================================
# TEST 1: Count All Active Snapshots
# ============================================================
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 1: Count All Active Snapshots"
echo "═══════════════════════════════════════════════════════════════"

SNAPSHOT_COUNT=$($DB_CMD "SELECT COUNT(*) FROM pricing_snapshots WHERE is_active = true;")
SNAPSHOT_COUNT=$(echo $SNAPSHOT_COUNT | tr -d ' ')

if [ "$SNAPSHOT_COUNT" -gt 0 ]; then
    pass "Found $SNAPSHOT_COUNT active snapshot(s)"
else
    fail "No active snapshots found"
fi

# ============================================================
# TEST 2: List All Active Snapshots with Rate Counts
# ============================================================
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 2: List All Active Snapshots"
echo "═══════════════════════════════════════════════════════════════"

echo ""
info "Active snapshots by region:"
echo ""
printf "%-10s %-20s %-15s %-25s\n" "CLOUD" "REGION" "RATES" "FETCHED AT"
printf "%-10s %-20s %-15s %-25s\n" "-----" "------" "-----" "----------"

$DB_CMD "
SELECT 
    ps.cloud || '|' || 
    ps.region || '|' ||
    COALESCE((SELECT COUNT(*)::text FROM pricing_rates WHERE snapshot_id = ps.id), '0') || '|' ||
    TO_CHAR(ps.fetched_at, 'YYYY-MM-DD HH24:MI')
FROM pricing_snapshots ps
WHERE ps.is_active = true
ORDER BY ps.cloud, ps.region;
" | while IFS='|' read -r cloud region rates fetched; do
    cloud=$(echo $cloud | tr -d ' ')
    region=$(echo $region | tr -d ' ')
    rates=$(echo $rates | tr -d ' ')
    fetched=$(echo $fetched | tr -d ' ')
    if [ -n "$cloud" ]; then
        printf "%-10s %-20s %-15s %-25s\n" "$cloud" "$region" "$rates" "$fetched"
    fi
done

pass "Snapshot listing complete"

# ============================================================
# TEST 3: Total Rates Across All Snapshots
# ============================================================
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 3: Total Pricing Rates (All Regions)"
echo "═══════════════════════════════════════════════════════════════"

TOTAL_RATES=$($DB_CMD "SELECT COUNT(*) FROM pricing_rates;")
TOTAL_RATES=$(echo $TOTAL_RATES | tr -d ' ')
info "Total rates in database: $TOTAL_RATES"

if [ "$TOTAL_RATES" -gt 50000 ]; then
    pass "Rate count ($TOTAL_RATES) exceeds minimum threshold (50,000)"
else
    warn "Rate count ($TOTAL_RATES) is below expected threshold"
fi

# ============================================================
# TEST 4: Rates by Cloud Provider
# ============================================================
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 4: Rates by Cloud Provider"
echo "═══════════════════════════════════════════════════════════════"

echo ""
$DB_CMD "
SELECT 
    ps.cloud || ': ' || COUNT(pr.id)::text || ' rates'
FROM pricing_rates pr
JOIN pricing_snapshots ps ON pr.snapshot_id = ps.id
WHERE ps.is_active = true
GROUP BY ps.cloud
ORDER BY COUNT(pr.id) DESC;
"
pass "Provider breakdown complete"

# ============================================================
# TEST 5: Rates by Region (Each Active Snapshot)
# ============================================================
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 5: Rates by Region"
echo "═══════════════════════════════════════════════════════════════"

echo ""
$DB_CMD "
SELECT 
    ps.cloud || ' ' || ps.region || ': ' || COUNT(pr.id)::text || ' rates'
FROM pricing_rates pr
JOIN pricing_snapshots ps ON pr.snapshot_id = ps.id
WHERE ps.is_active = true
GROUP BY ps.cloud, ps.region
ORDER BY ps.cloud, COUNT(pr.id) DESC
LIMIT 20;
"
info "(Showing top 20 regions by rate count)"
pass "Region breakdown complete"

# ============================================================
# TEST 6: Services Coverage (All Regions)
# ============================================================
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 6: Services Coverage"
echo "═══════════════════════════════════════════════════════════════"

echo ""
echo "Services and rate counts (top 15):"
$DB_CMD "
SELECT rk.service || ': ' || COUNT(*)::text
FROM pricing_rates pr
JOIN pricing_rate_keys rk ON pr.rate_key_id = rk.id
JOIN pricing_snapshots ps ON pr.snapshot_id = ps.id
WHERE ps.is_active = true
GROUP BY rk.service
ORDER BY COUNT(*) DESC
LIMIT 15;
"
pass "Service breakdown complete"

# ============================================================
# TEST 7: Verify No Negative Prices
# ============================================================
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 7: Data Integrity - No Negative Prices"
echo "═══════════════════════════════════════════════════════════════"

NEG_COUNT=$($DB_CMD "SELECT COUNT(*) FROM pricing_rates WHERE price < 0;")
NEG_COUNT=$(echo $NEG_COUNT | tr -d ' ')

if [ "$NEG_COUNT" -eq 0 ]; then
    pass "No negative prices found"
else
    fail "Found $NEG_COUNT negative prices"
fi

# ============================================================
# TEST 8: Check Region Aliases (if table exists)
# ============================================================
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 8: Region Aliases (Deduplication)"
echo "═══════════════════════════════════════════════════════════════"

ALIAS_COUNT=$($DB_CMD "SELECT COUNT(*) FROM pricing_region_aliases;" 2>/dev/null || echo "0")
ALIAS_COUNT=$(echo $ALIAS_COUNT | tr -d ' ')

if [ "$ALIAS_COUNT" != "0" ] && [ -n "$ALIAS_COUNT" ]; then
    info "Found $ALIAS_COUNT region aliases"
    $DB_CMD "
    SELECT canonical_region || ' ← ' || COUNT(*)::text || ' aliases'
    FROM pricing_region_aliases
    GROUP BY canonical_region
    ORDER BY COUNT(*) DESC
    LIMIT 5;
    " 2>/dev/null || true
    pass "Region aliases checked"
else
    info "No region aliases configured yet (table may not exist)"
fi

# ============================================================
# TEST 9: Sample Prices by Region
# ============================================================
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 9: Sample Prices (Random Regions)"
echo "═══════════════════════════════════════════════════════════════"

echo ""
$DB_CMD "
SELECT 
    ps.region || ' | ' || 
    rk.service || ' | ' ||
    ROUND(pr.price::numeric, 4)::text || ' ' || pr.currency
FROM pricing_rates pr
JOIN pricing_rate_keys rk ON pr.rate_key_id = rk.id
JOIN pricing_snapshots ps ON pr.snapshot_id = ps.id
WHERE ps.is_active = true AND pr.price > 0
ORDER BY RANDOM()
LIMIT 10;
"
pass "Sample prices retrieved"

# ============================================================
# SUMMARY
# ============================================================
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "                    TEST SUMMARY"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo -e "${GREEN}All tests passed!${NC}"
echo ""
echo "Database contains:"
echo "  • $SNAPSHOT_COUNT active snapshot(s)"
echo "  • $TOTAL_RATES total pricing rates"
echo ""

# List unique regions
UNIQUE_REGIONS=$($DB_CMD "SELECT COUNT(DISTINCT region) FROM pricing_snapshots WHERE is_active = true;")
UNIQUE_REGIONS=$(echo $UNIQUE_REGIONS | tr -d ' ')
echo "  • $UNIQUE_REGIONS unique regions with pricing data"
echo ""
echo "The pricing data is ready for cost estimation!"
