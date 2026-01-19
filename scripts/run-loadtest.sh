
# Convenience script for running database load tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Fractal Engine Load Test${NC}"
echo "================================"
echo ""

# Check if DATABASE_URL is set
if [ -z "$DATABASE_URL" ]; then
    echo -e "${YELLOW}Warning: DATABASE_URL not set, will use default SQLite database${NC}"
    echo ""
fi

# Parse command line arguments
PRESET=""
CUSTOM_ARGS=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --preset)
            PRESET="$2"
            shift 2
            ;;
        *)
            CUSTOM_ARGS="$CUSTOM_ARGS $1"
            shift
            ;;
    esac
done

# Apply preset configurations
case $PRESET in
    small)
        echo "Using SMALL preset (100 mints)"
        ARGS="-mints 100 -offers 5 -invoices 2 -balances 10"
        ;;
    medium)
        echo "Using MEDIUM preset (1000 mints, default)"
        ARGS="-mints 1000 -offers 10 -invoices 5 -balances 20"
        ;;
    large)
        echo "Using LARGE preset (5000 mints)"
        ARGS="-mints 5000 -offers 15 -invoices 8 -balances 30 -workers 8"
        ;;
    xlarge)
        echo "Using XLARGE preset (10000 mints)"
        ARGS="-mints 10000 -offers 20 -invoices 10 -balances 50 -workers 12"
        ;;
    *)
        if [ -n "$PRESET" ]; then
            echo -e "${RED}Unknown preset: $PRESET${NC}"
            echo "Available presets: small, medium, large, xlarge"
            exit 1
        fi
        ARGS=""
        ;;
esac

# Combine preset args with custom args
ARGS="$ARGS $CUSTOM_ARGS"

# Run the load test
echo ""
echo "Running: go run cmd/loadtest/main.go $ARGS"
echo ""

go run cmd/loadtest/main.go $ARGS

EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo ""
    echo -e "${GREEN}Load test completed successfully!${NC}"

    # Check for slow queries in report
    if [ -f "loadtest-report.json" ]; then
        SLOW_COUNT=$(jq '.SlowQueries | length' loadtest-report.json 2>/dev/null || echo "0")
        if [ "$SLOW_COUNT" -gt "0" ]; then
            echo -e "${YELLOW}Warning: Found $SLOW_COUNT slow queries (>100ms)${NC}"
            echo "Review loadtest-report.json for details"
        else
            echo -e "${GREEN}All queries performed well (<100ms average)${NC}"
        fi
    fi
else
    echo ""
    echo -e "${RED}Load test failed with exit code $EXIT_CODE${NC}"
    exit $EXIT_CODE
fi
