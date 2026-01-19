# Database Load Test Tool

A comprehensive load testing tool for the Fractal Engine database that generates realistic test data and measures query performance.

## Features

- **Data Generation**: Creates thousands of test records across all main tables (mints, offers, invoices, balances)
- **Concurrent Workers**: Parallel data generation for faster setup
- **Query Performance Testing**: Measures actual query performance using real store methods
- **Performance Reporting**: Identifies slow queries (>100ms) and generates detailed reports
- **Flexible Configuration**: Command-line flags for all parameters

## Usage

### Basic Usage

```bash
# Run with default settings (1000 mints)
go run cmd/loadtest/main.go

# Run with PostgreSQL database
DATABASE_URL="postgres://fractal:fractal_test_password@localhost:5433/fractalstore?sslmode=disable" go run cmd/loadtest/main.go

# Run with custom parameters
go run cmd/loadtest/main.go -mints 5000 -offers 20 -invoices 10 -balances 50
```

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-mints` | 1000 | Number of mints to create |
| `-offers` | 10 | Number of buy/sell offers per mint |
| `-invoices` | 5 | Number of invoices per mint |
| `-balances` | 20 | Number of token balances per mint |
| `-addresses` | 500 | Number of unique addresses to generate |
| `-workers` | 4 | Number of concurrent workers for data generation |
| `-iterations` | 100 | Number of iterations for each query test |
| `-db` | (auto) | Database URL (overrides DATABASE_URL env) |
| `-clean` | false | Clean database before running test |
| `-skip-data` | false | Skip data generation (use existing data) |
| `-report` | loadtest-report.json | Output file for JSON report |

### Example Commands

```bash
# Large-scale test with 10,000 mints
go run cmd/loadtest/main.go -mints 10000 -offers 20 -workers 8

# Test with existing data (skip generation)
go run cmd/loadtest/main.go -skip-data -iterations 500

# Clean database and run fresh test
go run cmd/loadtest/main.go -clean -mints 2000

# Quick test with small dataset
go run cmd/loadtest/main.go -mints 100 -offers 5 -invoices 2 -balances 10
```

## What It Tests

### Data Generation

The tool generates realistic test data for:

1. **Mints** - Token definitions with metadata, requirements, and owner addresses
2. **Sell Offers** - Public market sell orders
3. **Buy Offers** - Targeted buy orders between specific addresses
4. **Invoices** - Payment requests with quantities and prices
5. **Token Balances** - Token holdings across addresses

### Query Performance Tests

The following queries are tested:

1. **GetMints (limit=50)** - Paginated mint retrieval
2. **GetMints (limit=500)** - Large page mint retrieval
3. **GetMintByHash** - Single mint lookup by hash
4. **GetSellOffers** - Paginated sell offer retrieval
5. **GetBuyOffers** - Paginated buy offer retrieval
6. **GetInvoices** - Paginated invoice retrieval
7. **GetBalance** - Token balance lookup for address
8. **GetStats** - System statistics aggregation
9. **GetSellOffersForMint** - Filtered offers by mint hash
10. **AggregateBalancesByMint** - Balance aggregation with GROUP BY

Each query is run multiple times (default 100 iterations) and the average duration is reported.

## Output

### Console Output

The tool provides real-time progress updates:

```
Starting load test with configuration:
  Database URL: postgres://localhost:5432/fractalstore
  Mints: 1000
  Offers per mint: 10
  ...

Generating test data...
  Created 100/1000 mints
  Created 200/1000 mints
  ...

Running query performance tests...

================================================================================
LOAD TEST REPORT
================================================================================
Database: postgres://localhost:5432/fractalstore
Total Duration: 2m30s
Data Generation: 2m15s

Record Counts:
  mints:                         1000
  sell_offers:                  10000
  buy_offers:                   10000
  invoices:                      5000
  token_balances:               20000

Query Performance Results:
--------------------------------------------------------------------------------
Query                                    Avg Duration        Avg Rows
--------------------------------------------------------------------------------
GetMints(limit=50)                           1.2ms              50
GetMints(limit=500)                         12.5ms             500
GetMintByHash                              234.5µs               1
...

SLOW QUERIES (>100ms):
--------------------------------------------------------------------------------
  GetMints(limit=500)                         12.5ms
```

### JSON Report

A detailed JSON report is saved to `loadtest-report.json` (or custom filename with `-report` flag):

```json
{
  "TotalDuration": 150000000000,
  "DataGeneration": 135000000000,
  "QueryTests": [
    {
      "QueryName": "GetMints(limit=50)",
      "Duration": 1200000,
      "RowsReturned": 50,
      "Error": null
    },
    ...
  ],
  "SlowQueries": [...],
  "DatabaseURL": "postgres://localhost:5432/fractalstore",
  "RecordCounts": {
    "mints": 1000,
    "sell_offers": 10000,
    ...
  }
}
```

## Database Support

The tool works with:

- **PostgreSQL** - Production database (recommended for load testing)
- **SQLite** - Development/testing (may have different performance characteristics)

## Performance Considerations

### Data Generation Performance

- Use more workers (`-workers`) for faster data generation on multi-core systems
- PostgreSQL handles concurrent inserts better than SQLite
- Large datasets (>10,000 mints) may take several minutes

### Query Performance Factors

- **Database type**: PostgreSQL typically faster for complex queries
- **Indexes**: Ensure migrations have created appropriate indexes
- **Data size**: Performance degrades with larger datasets
- **Hardware**: CPU, RAM, and disk speed all affect results

### Recommended Test Scenarios

1. **Small dataset** (100 mints): Quick sanity check
2. **Medium dataset** (1,000 mints): Typical production simulation
3. **Large dataset** (10,000+ mints): Stress test for scaling
4. **Very large dataset** (100,000+ mints): Enterprise scale testing

## Identifying Slow Queries

Queries are automatically flagged as "SLOW" if they exceed 100ms average duration.

### Common Causes of Slow Queries

1. **Missing indexes** - Check migration files for index definitions
2. **Large result sets** - Consider pagination limits
3. **Complex joins** - Review query structure in store methods
4. **Table scans** - Use EXPLAIN ANALYZE to diagnose
5. **Unoptimized queries** - Review SQL in `pkg/store/*.go` files

### Optimization Workflow

1. Run load test to identify slow queries
2. Use `EXPLAIN ANALYZE` on slow queries:
   ```sql
   EXPLAIN ANALYZE SELECT * FROM mints LIMIT 500;
   ```
3. Add indexes if needed:
   ```sql
   CREATE INDEX idx_mints_owner ON mints(owner_address);
   ```
4. Re-run load test to verify improvement

## Troubleshooting

### Connection Issues

```
Failed to connect to database: ...
```

**Solution**: Verify DATABASE_URL or use `-db` flag with correct connection string.

### Out of Memory

**Solution**: Reduce `-mints`, `-offers`, `-invoices`, or `-balances` values.

### Slow Data Generation

**Solution**: Increase `-workers` or use PostgreSQL instead of SQLite.

### Query Errors

Check that database migrations are up to date:
```bash
go run cmd/fractal-engine/fractal_engine.go migrate
```

## Example Workflow

### 1. Clean Run with Fresh Data

```bash
# Clean database and generate 5000 mints
go run cmd/loadtest/main.go -clean -mints 5000 -workers 8

# Review report
cat loadtest-report.json | jq '.SlowQueries'
```

### 2. Test Existing Production-Size Data

```bash
# Skip generation, test with existing data
go run cmd/loadtest/main.go -skip-data -iterations 1000

# Save report with timestamp
go run cmd/loadtest/main.go -skip-data -report "report-$(date +%Y%m%d-%H%M%S).json"
```

### 3. Compare Performance Before/After Optimization

```bash
# Baseline test
go run cmd/loadtest/main.go -report baseline.json

# ... make optimizations (add indexes, refactor queries) ...

# Comparison test
go run cmd/loadtest/main.go -skip-data -report optimized.json

# Compare results
jq '.QueryTests[] | {name: .QueryName, duration: .Duration}' baseline.json > baseline-summary.json
jq '.QueryTests[] | {name: .QueryName, duration: .Duration}' optimized.json > optimized-summary.json
```

## Integration with CI/CD

Add performance regression testing to your pipeline:

```bash
#!/bin/bash
# performance-test.sh

# Run load test
go run cmd/loadtest/main.go -mints 1000 -report perf-report.json

# Check for slow queries
SLOW_COUNT=$(jq '.SlowQueries | length' perf-report.json)

if [ "$SLOW_COUNT" -gt 5 ]; then
  echo "ERROR: Found $SLOW_COUNT slow queries (threshold: 5)"
  jq '.SlowQueries' perf-report.json
  exit 1
fi

echo "Performance test passed: $SLOW_COUNT slow queries"
```

## Contributing

When adding new queries to `pkg/store/`, add corresponding tests in `runQueryTests()` in this tool.
