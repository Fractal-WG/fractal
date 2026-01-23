package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	"dogecoin.org/fractal-engine/db/migrations"
	"dogecoin.org/fractal-engine/pkg/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

type TokenisationStore struct {
	DB      *sql.DB
	backend string
	cfg     config.Config
}

func NewTokenisationStore(dbUrl string, cfg config.Config) (*TokenisationStore, error) {
	u, err := url.Parse(dbUrl)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "postgres" {
		postgres, err := sql.Open("postgres", dbUrl)
		if err != nil {
			return nil, err
		}

		return &TokenisationStore{DB: postgres, backend: "postgres", cfg: cfg}, nil
	}

	sqlite, err := sql.Open("sqlite3", dbUrl)
	if err != nil {
		return nil, err
	}

	return &TokenisationStore{DB: sqlite, backend: "sqlite", cfg: cfg}, nil
}

func (s *TokenisationStore) Migrate(ctx context.Context) error {
	driver, err := s.getMigrationDriver(ctx)
	if err != nil {
		return err
	}

	src, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", src, s.backend, driver)
	if err != nil {
		return err
	}

	err = m.Up()
	if err != nil {
		return err
	}

	return nil
}

func (s *TokenisationStore) getMigrationDriver(ctx context.Context) (database.Driver, error) {
	if s.backend == "postgres" {
		driver, err := postgres.WithInstance(s.DB, &postgres.Config{})
		if err != nil {
			return nil, err
		}

		return driver, nil
	}

	if s.backend == "sqlite" {
		driver, err := sqlite.WithInstance(s.DB, &sqlite.Config{})
		if err != nil {
			return nil, err
		}

		return driver, nil
	}

	if s.backend == "duckdb" {
		driver, err := sqlite.WithInstance(s.DB, &sqlite.Config{})
		if err != nil {
			return nil, err
		}

		return driver, nil
	}

	return nil, fmt.Errorf("unsupported database scheme: %s", s.backend)
}

func (s *TokenisationStore) Close(ctx context.Context) error {
	fmt.Println("Closing store")
	return s.DB.Close()
}
