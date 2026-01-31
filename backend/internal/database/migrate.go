// Package database - Database Migration Runner for APEX.BUILD
// Provides safe, versioned database migrations using golang-migrate
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// MigrationConfig holds configuration for the migration runner
type MigrationConfig struct {
	// Database connection string (PostgreSQL or SQLite)
	DatabaseURL string

	// Database type: "postgres" or "sqlite"
	DatabaseType string

	// Path to migrations directory (relative to project root or absolute)
	MigrationsPath string

	// Logger for migration output
	Logger *log.Logger
}

// MigrationRunner handles database migrations
type MigrationRunner struct {
	config   *MigrationConfig
	migrate  *migrate.Migrate
	db       *sql.DB
	dbDriver string
}

// MigrationStatus represents the current migration state
type MigrationStatus struct {
	Version uint   `json:"version"`
	Dirty   bool   `json:"dirty"`
	Applied bool   `json:"applied"`
	Error   string `json:"error,omitempty"`
}

// NewMigrationRunner creates a new migration runner
func NewMigrationRunner(config *MigrationConfig) (*MigrationRunner, error) {
	if config == nil {
		return nil, errors.New("migration config is required")
	}

	if config.Logger == nil {
		config.Logger = log.New(os.Stdout, "[MIGRATE] ", log.LstdFlags)
	}

	// Resolve migrations path
	migrationsPath := config.MigrationsPath
	if migrationsPath == "" {
		// Default to migrations directory relative to this file
		_, filename, _, _ := runtime.Caller(0)
		baseDir := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
		migrationsPath = filepath.Join(baseDir, "migrations")
	}

	// Convert to absolute path if relative
	if !filepath.IsAbs(migrationsPath) {
		absPath, err := filepath.Abs(migrationsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve migrations path: %w", err)
		}
		migrationsPath = absPath
	}

	// Verify migrations directory exists
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("migrations directory not found: %s", migrationsPath)
	}

	config.MigrationsPath = migrationsPath

	runner := &MigrationRunner{
		config:   config,
		dbDriver: config.DatabaseType,
	}

	if err := runner.initialize(); err != nil {
		return nil, err
	}

	return runner, nil
}

// initialize sets up the migration instance
func (r *MigrationRunner) initialize() error {
	var err error
	var driver database.Driver

	// Open database connection based on type
	switch r.dbDriver {
	case "postgres", "postgresql":
		r.db, err = sql.Open("postgres", r.config.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
		}

		driver, err = postgres.WithInstance(r.db, &postgres.Config{})
		if err != nil {
			return fmt.Errorf("failed to create PostgreSQL driver: %w", err)
		}
		r.dbDriver = "postgres"

	case "sqlite", "sqlite3":
		r.db, err = sql.Open("sqlite", r.config.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to open SQLite connection: %w", err)
		}

		driver, err = sqlite3.WithInstance(r.db, &sqlite3.Config{})
		if err != nil {
			return fmt.Errorf("failed to create SQLite driver: %w", err)
		}
		r.dbDriver = "sqlite3"

	default:
		return fmt.Errorf("unsupported database type: %s", r.dbDriver)
	}

	// Create migrate instance
	sourceURL := fmt.Sprintf("file://%s", r.config.MigrationsPath)
	r.migrate, err = migrate.NewWithDatabaseInstance(sourceURL, r.dbDriver, driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	return nil
}

// RunMigrations applies all pending migrations
func (r *MigrationRunner) RunMigrations() error {
	r.config.Logger.Println("Running database migrations...")

	err := r.migrate.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			r.config.Logger.Println("No migrations to apply - database is up to date")
			return nil
		}
		return fmt.Errorf("migration failed: %w", err)
	}

	version, dirty, _ := r.migrate.Version()
	r.config.Logger.Printf("Migrations completed successfully. Current version: %d (dirty: %v)", version, dirty)

	return nil
}

// MigrateUp applies N migrations
func (r *MigrationRunner) MigrateUp(n int) error {
	r.config.Logger.Printf("Applying %d migration(s)...", n)

	err := r.migrate.Steps(n)
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			r.config.Logger.Println("No migrations to apply")
			return nil
		}
		return fmt.Errorf("migration failed: %w", err)
	}

	version, dirty, _ := r.migrate.Version()
	r.config.Logger.Printf("Applied %d migration(s). Current version: %d (dirty: %v)", n, version, dirty)

	return nil
}

// RollbackMigration rolls back the last migration
func (r *MigrationRunner) RollbackMigration() error {
	r.config.Logger.Println("Rolling back last migration...")

	err := r.migrate.Steps(-1)
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			r.config.Logger.Println("No migrations to rollback")
			return nil
		}
		return fmt.Errorf("rollback failed: %w", err)
	}

	version, dirty, _ := r.migrate.Version()
	r.config.Logger.Printf("Rollback completed. Current version: %d (dirty: %v)", version, dirty)

	return nil
}

// RollbackAll rolls back all migrations
func (r *MigrationRunner) RollbackAll() error {
	r.config.Logger.Println("Rolling back all migrations...")

	err := r.migrate.Down()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			r.config.Logger.Println("No migrations to rollback")
			return nil
		}
		return fmt.Errorf("rollback all failed: %w", err)
	}

	r.config.Logger.Println("All migrations rolled back successfully")
	return nil
}

// MigrateToVersion migrates to a specific version
func (r *MigrationRunner) MigrateToVersion(version uint) error {
	r.config.Logger.Printf("Migrating to version %d...", version)

	err := r.migrate.Migrate(version)
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			r.config.Logger.Printf("Already at version %d", version)
			return nil
		}
		return fmt.Errorf("migration to version %d failed: %w", version, err)
	}

	currentVersion, dirty, _ := r.migrate.Version()
	r.config.Logger.Printf("Migration completed. Current version: %d (dirty: %v)", currentVersion, dirty)

	return nil
}

// GetVersion returns the current migration version
func (r *MigrationRunner) GetVersion() (MigrationStatus, error) {
	version, dirty, err := r.migrate.Version()

	status := MigrationStatus{
		Version: version,
		Dirty:   dirty,
		Applied: version > 0,
	}

	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			// No migrations applied yet
			status.Version = 0
			status.Applied = false
			return status, nil
		}
		status.Error = err.Error()
		return status, err
	}

	return status, nil
}

// Force sets the migration version without running migrations
// Use with caution - this is for fixing dirty states
func (r *MigrationRunner) Force(version int) error {
	r.config.Logger.Printf("Forcing version to %d...", version)

	err := r.migrate.Force(version)
	if err != nil {
		return fmt.Errorf("force failed: %w", err)
	}

	r.config.Logger.Printf("Version forced to %d", version)
	return nil
}

// Close closes the migration runner and database connection
func (r *MigrationRunner) Close() error {
	if r.migrate != nil {
		srcErr, dbErr := r.migrate.Close()
		if srcErr != nil {
			return fmt.Errorf("failed to close source: %w", srcErr)
		}
		if dbErr != nil {
			return fmt.Errorf("failed to close database: %w", dbErr)
		}
	}
	return nil
}

// --- Helper Functions for Easy Integration ---

// RunMigrations is a convenience function to run all migrations
// with default configuration from environment variables
func RunMigrations(databaseURL, databaseType, migrationsPath string) error {
	config := &MigrationConfig{
		DatabaseURL:    databaseURL,
		DatabaseType:   databaseType,
		MigrationsPath: migrationsPath,
	}

	runner, err := NewMigrationRunner(config)
	if err != nil {
		return err
	}
	defer runner.Close()

	return runner.RunMigrations()
}

// GetMigrationVersion returns the current migration version
func GetMigrationVersion(databaseURL, databaseType, migrationsPath string) (MigrationStatus, error) {
	config := &MigrationConfig{
		DatabaseURL:    databaseURL,
		DatabaseType:   databaseType,
		MigrationsPath: migrationsPath,
	}

	runner, err := NewMigrationRunner(config)
	if err != nil {
		return MigrationStatus{Error: err.Error()}, err
	}
	defer runner.Close()

	return runner.GetVersion()
}

// RollbackLastMigration rolls back the most recent migration
func RollbackLastMigration(databaseURL, databaseType, migrationsPath string) error {
	config := &MigrationConfig{
		DatabaseURL:    databaseURL,
		DatabaseType:   databaseType,
		MigrationsPath: migrationsPath,
	}

	runner, err := NewMigrationRunner(config)
	if err != nil {
		return err
	}
	defer runner.Close()

	return runner.RollbackMigration()
}

// --- PostgreSQL-Specific Helper ---

// BuildPostgresDSN constructs a PostgreSQL connection string from components
func BuildPostgresDSN(host string, port int, user, password, dbname, sslmode string) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode,
	)
}

// BuildPostgresDSNFromConfig builds DSN from a Config struct
func BuildPostgresDSNFromConfig(host string, port int, user, password, dbname, sslmode, timezone string) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		host, port, user, password, dbname, sslmode, timezone,
	)
}
