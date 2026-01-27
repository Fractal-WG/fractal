package support

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/store"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	sharedPostgresContainer testcontainers.Container
	sharedPostgresURL       string
	containerMutex          sync.Mutex
	containerRefCount       int
)

// InitSharedPostgres initializes a shared PostgreSQL container for all tests
func InitSharedPostgres() error {
	containerMutex.Lock()
	defer containerMutex.Unlock()

	if sharedPostgresContainer != nil {
		containerRefCount++
		return nil
	}

	ctx := context.Background()

	// Start a single PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		return fmt.Errorf("failed to start postgres container: %w", err)
	}

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable", "dbname=postgres")
	if err != nil {
		return fmt.Errorf("failed to get connection string: %w", err)
	}

	sharedPostgresContainer = postgresContainer
	sharedPostgresURL = connStr
	containerRefCount = 1

	return nil
}

// CleanupSharedPostgres cleans up the shared PostgreSQL container
func CleanupSharedPostgres() {
	containerMutex.Lock()
	defer containerMutex.Unlock()

	containerRefCount--
	if containerRefCount <= 0 && sharedPostgresContainer != nil {
		ctx := context.Background()
		_ = sharedPostgresContainer.Terminate(ctx)
		sharedPostgresContainer = nil
		sharedPostgresURL = ""
	}
}

// SetupTestDBShared creates a unique database in the shared PostgreSQL instance
func SetupTestDBShared() *store.TokenisationStore {
	// Initialize shared container if needed
	if err := InitSharedPostgres(); err != nil {
		log.Fatalf("Failed to initialize shared postgres: %v", err)
	}

	// Generate unique database name for this test
	time.Sleep(100 * time.Millisecond) // Ensure unique timestamp
	dbName := fmt.Sprintf("testdb_%d", time.Now().UnixNano())

	// Remove any special characters that might cause issues
	dbName = strings.ReplaceAll(dbName, "-", "_")
	dbName = strings.ReplaceAll(dbName, " ", "_")

	// Create the database
	db, err := sql.Open("postgres", sharedPostgresURL)
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer db.Close()

	// Create new database
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		log.Fatalf("Failed to create database %s: %v", dbName, err)
	}

	// Build connection string for the new database
	newConnStr := strings.Replace(sharedPostgresURL, "dbname=postgres", fmt.Sprintf("dbname=%s", dbName), 1)

	log.Printf("Using shared database: %s\n", newConnStr)

	tokenisationStore, err := store.NewTokenisationStore(newConnStr, config.Config{})
	if err != nil {
		log.Fatalf("Failed to create tokenisation store: %v", err)
	}

	err = tokenisationStore.Migrate(context.Background())
	if err != nil && err.Error() != "no change" {
		log.Fatalf("Failed to migrate tokenisation store: %v", err)
	}

	return tokenisationStore
}
