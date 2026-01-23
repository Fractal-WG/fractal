
# Analyze load test results and provide actionable insights

set -e

REPORT_FILE="${1:-loadtest-report.json}"

if [ ! -f "$REPORT_FILE" ]; then
    echo "Error: Report file not found: $REPORT_FILE"
    echo "Usage: $0 [report-file.json]"
    exit 1
fi

echo "Analyzing load test results from: $REPORT_FILE"
echo "================================================================"
echo ""

# Extract key metrics
TOTAL_DURATION=$(jq -r '.TotalDuration / 1000000000 | floor' "$REPORT_FILE")
DATA_GEN_DURATION=$(jq -r '.DataGeneration / 1000000000 | floor' "$REPORT_FILE")
SLOW_QUERY_COUNT=$(jq '.SlowQueries | length' "$REPORT_FILE")

echo "Summary:"
echo "  Total Duration: ${TOTAL_DURATION}s"
echo "  Data Generation: ${DATA_GEN_DURATION}s"
echo "  Slow Queries (>100ms): $SLOW_QUERY_COUNT"
echo ""

# Show record counts
echo "Record Counts:"
jq -r '.RecordCounts | to_entries[] | "  \(.key): \(.value)"' "$REPORT_FILE"
echo ""

# Show all query results sorted by duration
echo "Query Performance (sorted by duration):"
echo "----------------------------------------------------------------"
printf "%-45s %15s %10s\n" "Query Name" "Duration" "Rows"
echo "----------------------------------------------------------------"
jq -r '.QueryTests | sort_by(.Duration) | reverse[] |
    "\(.QueryName)|\(.Duration / 1000000 | floor)ms|\(.RowsReturned)"' "$REPORT_FILE" |
    while IFS='|' read -r name duration rows; do
        printf "%-45s %15s %10s\n" "$name" "$duration" "$rows"
    done
echo ""

# Show slow queries with recommendations
if [ "$SLOW_QUERY_COUNT" -gt "0" ]; then
    echo "⚠️  SLOW QUERIES DETECTED:"
    echo "----------------------------------------------------------------"
    jq -r '.SlowQueries[] |
        "\(.QueryName): \(.Duration / 1000000 | floor)ms (avg \(.RowsReturned) rows)"' "$REPORT_FILE"
    echo ""
    echo "Recommendations:"
    echo "  1. Run EXPLAIN ANALYZE on slow queries to identify bottlenecks"
    echo "  2. Check if indexes exist on frequently queried columns"
    echo "  3. Consider adding composite indexes for multi-column filters"
    echo "  4. Review pagination limits for large result sets"
    echo "  5. Check database connection pool settings"
    echo ""

    # Specific recommendations based on query names
    jq -r '.SlowQueries[] | .QueryName' "$REPORT_FILE" | while read -r query; do
        case $query in
            *"GetMints"*)
                echo "  → GetMints: Consider adding index on created_at for faster pagination"
                ;;
            *"Balance"*)
                echo "  → Balance queries: Ensure composite index on (mint_hash, address)"
                ;;
            *"Offers"*)
                echo "  → Offer queries: Check indexes on mint_hash and offerer_address"
                ;;
            *"Aggregate"*)
                echo "  → Aggregation queries: Consider materialized views for common aggregations"
                ;;
        esac
    done
    echo ""
fi

# Performance grade
AVG_QUERY_TIME=$(jq '[.QueryTests[].Duration] | add / length / 1000000 | floor' "$REPORT_FILE")

echo "Performance Grade:"
if [ "$AVG_QUERY_TIME" -lt 10 ]; then
    echo "  ✅ EXCELLENT (avg ${AVG_QUERY_TIME}ms) - All queries are fast"
elif [ "$AVG_QUERY_TIME" -lt 50 ]; then
    echo "  ✓ GOOD (avg ${AVG_QUERY_TIME}ms) - Most queries are responsive"
elif [ "$AVG_QUERY_TIME" -lt 100 ]; then
    echo "  ⚠ ACCEPTABLE (avg ${AVG_QUERY_TIME}ms) - Some optimization recommended"
else
    echo "  ❌ POOR (avg ${AVG_QUERY_TIME}ms) - Optimization needed"
fi
echo ""

# Identify fastest queries
echo "Top 3 Fastest Queries:"
jq -r '.QueryTests | sort_by(.Duration) | .[0:3][] |
    "  ✓ \(.QueryName): \(.Duration / 1000000 | floor)ms"' "$REPORT_FILE"
echo ""

# Database info
DB_URL=$(jq -r '.DatabaseURL' "$REPORT_FILE")
echo "Database: $DB_URL"
echo ""

echo "================================================================"
echo "For detailed query analysis, run:"
echo "  jq '.QueryTests' $REPORT_FILE"
