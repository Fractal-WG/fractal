# Docker Setup for Loadtest

This directory contains Docker configuration for running the database load tests in a containerized environment with PostgreSQL.

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Build and run both PostgreSQL and loadtest
docker-compose up --build

# Run in detached mode
docker-compose up -d --build

# View logs
docker-compose logs -f loadtest

# Stop and remove containers
docker-compose down

# Stop and remove containers with volumes (clean slate)
docker-compose down -v
```

### Custom Loadtest Parameters

You can customize the loadtest parameters by modifying the `command` in `docker-compose.yml` or by running the container directly:

```bash
# Build the image
docker-compose build

# Start only PostgreSQL
docker-compose up -d postgres

# Run loadtest with custom parameters
docker-compose run --rm loadtest ./loadtest -mints 5000 -offers 20 -workers 8

# Run with different presets
docker-compose run --rm loadtest ./loadtest -clean -mints 10000 -offers 20 -invoices 10 -balances 50 -workers 12
```

## Configuration

### PostgreSQL

The PostgreSQL database is configured with:
- **Host Port**: 5433 (mapped from container port 5432)
- **Username**: fractal
- **Password**: fractal_test_password
- **Database**: fractalstore

To connect from your host machine:
```bash
psql -h localhost -p 5433 -U fractal -d fractalstore
# Password: fractal_test_password
```

### Environment Variables

You can override the database connection by setting environment variables:

```bash
# In docker-compose.yml, modify the environment section:
environment:
  DATABASE_URL: "postgres://user:password@host:port/database?sslmode=disable"
```

## Using Only Docker (Without Compose)

### 1. Start PostgreSQL Container

```bash
docker run -d \
  --name fractal-loadtest-db \
  -e POSTGRES_USER=fractal \
  -e POSTGRES_PASSWORD=fractal_test_password \
  -e POSTGRES_DB=fractalstore \
  -p 5433:5432 \
  postgres:16-alpine
```

### 2. Build Loadtest Image

```bash
# From the repository root
docker build -f cmd/loadtest/Dockerfile -t fractal-loadtest .
```

### 3. Run Loadtest

```bash
docker run --rm \
  --network host \
  -e DATABASE_URL="postgres://fractal:fractal_test_password@localhost:5433/fractalstore?sslmode=disable" \
  fractal-loadtest \
  ./loadtest -mints 1000 -offers 10 -invoices 5 -balances 20
```

Or link containers with Docker network:

```bash
# Create network
docker network create fractal-network

# Run postgres on network
docker run -d \
  --name fractal-loadtest-db \
  --network fractal-network \
  -e POSTGRES_USER=fractal \
  -e POSTGRES_PASSWORD=fractal_test_password \
  -e POSTGRES_DB=fractalstore \
  -p 5433:5432 \
  postgres:16-alpine

# Run loadtest on same network
docker run --rm \
  --network fractal-network \
  -e DATABASE_URL="postgres://fractal:fractal_test_password@fractal-loadtest-db:5432/fractalstore?sslmode=disable" \
  fractal-loadtest \
  ./loadtest -mints 1000 -offers 10
```

## Accessing Test Reports

Test reports are saved in the container. To access them:

### Option 1: Volume Mount (Recommended)

Already configured in `docker-compose.yml`:
```yaml
volumes:
  - ./reports:/app/reports
```

Modify the loadtest command to save reports to the mounted directory:
```bash
docker-compose run --rm loadtest ./loadtest -report /app/reports/loadtest-report.json
```

### Option 2: Copy from Container

```bash
# List running containers
docker ps

# Copy report from container
docker cp fractal-loadtest:/app/loadtest-report.json ./loadtest-report.json
```

## Example Workflows

### Small Test Run
```bash
docker-compose up -d postgres
docker-compose run --rm loadtest ./loadtest -mints 100 -offers 5 -invoices 2 -balances 10
```

### Large Scale Test
```bash
docker-compose up -d postgres
docker-compose run --rm loadtest ./loadtest -mints 10000 -offers 20 -invoices 10 -balances 50 -workers 12
```

### Clean Database and Run
```bash
docker-compose up -d postgres
docker-compose run --rm loadtest ./loadtest -clean -mints 5000 -workers 8
```

### Test with Existing Data
```bash
docker-compose up -d postgres

# First run to generate data
docker-compose run --rm loadtest ./loadtest -mints 5000

# Subsequent runs without data generation
docker-compose run --rm loadtest ./loadtest -skip-data -iterations 500
```

### Performance Comparison
```bash
docker-compose up -d postgres

# Baseline test
docker-compose run --rm loadtest ./loadtest -report /app/reports/baseline.json

# Make optimizations (e.g., add indexes)
docker exec -it fractal-loadtest-db psql -U fractal -d fractalstore -c "CREATE INDEX idx_mints_owner ON mints(owner_address);"

# Comparison test
docker-compose run --rm loadtest ./loadtest -skip-data -report /app/reports/optimized.json

# Compare results
cat reports/baseline.json | jq '.SlowQueries'
cat reports/optimized.json | jq '.SlowQueries'
```

## Troubleshooting

### Container Fails to Start

Check logs:
```bash
docker-compose logs postgres
docker-compose logs loadtest
```

### Database Connection Issues

Verify PostgreSQL is running:
```bash
docker-compose ps
docker-compose exec postgres pg_isready -U fractal
```

Test connection from loadtest container:
```bash
docker-compose run --rm loadtest sh
# Inside container:
pg_isready -h postgres -U fractal
```

### Port Already in Use

If port 5433 is already in use, change it in `docker-compose.yml`:
```yaml
ports:
  - "5434:5432"  # Use different host port
```

And update the connection string accordingly when running from host.

### Out of Memory

Increase Docker resources:
- Docker Desktop: Settings → Resources → Memory
- Or reduce loadtest parameters (fewer mints, offers, etc.)

## Cleanup

```bash
# Stop all containers
docker-compose down

# Remove all data (fresh start)
docker-compose down -v

# Remove images
docker-compose down --rmi all

# Full cleanup
docker-compose down -v --rmi all --remove-orphans
```

## CI/CD Integration

Example GitHub Actions workflow:

```yaml
name: Load Test

on: [push, pull_request]

jobs:
  loadtest:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Run load test
        run: |
          docker-compose up -d postgres
          docker-compose run --rm loadtest ./loadtest -mints 1000 -report /app/reports/report.json

      - name: Check for slow queries
        run: |
          SLOW_COUNT=$(cat reports/report.json | jq '.SlowQueries | length')
          if [ "$SLOW_COUNT" -gt 5 ]; then
            echo "ERROR: Found $SLOW_COUNT slow queries"
            exit 1
          fi

      - name: Upload report
        uses: actions/upload-artifact@v3
        with:
          name: loadtest-report
          path: reports/report.json
```
