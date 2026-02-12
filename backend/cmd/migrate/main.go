// Package main - Database Migration CLI for APEX.BUILD
// Provides command-line tools for managing database migrations
//
// Usage:
//
//	go run cmd/migrate/main.go up           # Apply all pending migrations
//	go run cmd/migrate/main.go down         # Rollback last migration
//	go run cmd/migrate/main.go down-all     # Rollback all migrations
//	go run cmd/migrate/main.go version      # Show current migration version
//	go run cmd/migrate/main.go to N         # Migrate to specific version N
//	go run cmd/migrate/main.go force N      # Force version to N (fix dirty state)
//	go run cmd/migrate/main.go create NAME  # Create new migration files
package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"apex-build/internal/database"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		if err := godotenv.Load("../.env"); err != nil {
			if err := godotenv.Load("../../.env"); err != nil {
				log.Println("No .env file found, using environment variables")
			}
		}
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Get database configuration
	dbURL, dbType := getDatabaseConfig()
	migrationsPath := getMigrationsPath()

	log.Printf("Database type: %s", dbType)
	log.Printf("Migrations path: %s", migrationsPath)

	config := &database.MigrationConfig{
		DatabaseURL:    dbURL,
		DatabaseType:   dbType,
		MigrationsPath: migrationsPath,
	}

	switch command {
	case "up":
		runUp(config)
	case "down":
		runDown(config)
	case "down-all":
		runDownAll(config)
	case "version":
		showVersion(config)
	case "to":
		if len(os.Args) < 3 {
			log.Fatal("Usage: migrate to <version>")
		}
		version, err := strconv.ParseUint(os.Args[2], 10, 32)
		if err != nil {
			log.Fatalf("Invalid version number: %s", os.Args[2])
		}
		runTo(config, uint(version))
	case "force":
		if len(os.Args) < 3 {
			log.Fatal("Usage: migrate force <version>")
		}
		version, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("Invalid version number: %s", os.Args[2])
		}
		runForce(config, version)
	case "create":
		if len(os.Args) < 3 {
			log.Fatal("Usage: migrate create <migration_name>")
		}
		createMigration(migrationsPath, os.Args[2])
	case "help":
		printUsage()
	default:
		log.Printf("Unknown command: %s", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`
APEX.BUILD Database Migration Tool

Usage:
  migrate <command> [arguments]

Commands:
  up              Apply all pending migrations
  down            Rollback the last migration
  down-all        Rollback all migrations (WARNING: deletes all data!)
  version         Show current migration version
  to <N>          Migrate to specific version N
  force <N>       Force version to N (use to fix dirty state)
  create <name>   Create new migration files
  help            Show this help message

Environment Variables:
  DATABASE_URL    Full database connection URL
  DB_HOST         Database host (default: localhost)
  DB_PORT         Database port (default: 5432)
  DB_USER         Database user (default: postgres)
  DB_PASSWORD     Database password
  DB_NAME         Database name (default: apex_build)
  DB_SSL_MODE     SSL mode (default: disable)

Examples:
  # Apply all migrations
  go run cmd/migrate/main.go up

  # Rollback last migration
  go run cmd/migrate/main.go down

  # Check current version
  go run cmd/migrate/main.go version

  # Create new migration
  go run cmd/migrate/main.go create add_user_preferences

  # Fix dirty migration state
  go run cmd/migrate/main.go force 5
`)
}

func getDatabaseConfig() (string, string) {
	// Check for DATABASE_URL first
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		// Parse to determine database type
		u, err := url.Parse(databaseURL)
		if err == nil {
			scheme := u.Scheme
			if scheme == "postgres" || scheme == "postgresql" {
				return databaseURL, "postgres"
			} else if scheme == "sqlite" || scheme == "sqlite3" {
				return strings.TrimPrefix(databaseURL, scheme+"://"), "sqlite"
			}
		}
		// Default to postgres if scheme not recognized
		return databaseURL, "postgres"
	}

	// Build from individual environment variables
	host := getEnv("DB_HOST", "localhost")
	port := getEnvInt("DB_PORT", 5432)
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "password")
	dbname := getEnv("DB_NAME", "apex_build")
	sslmode := getEnv("DB_SSL_MODE", "disable")

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode,
	)

	return dsn, "postgres"
}

func getMigrationsPath() string {
	// Check for explicit path first
	if path := os.Getenv("MIGRATIONS_PATH"); path != "" {
		return path
	}

	// Try to find migrations directory relative to executable
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		candidates := []string{
			filepath.Join(execDir, "migrations"),
			filepath.Join(execDir, "..", "migrations"),
			filepath.Join(execDir, "..", "..", "migrations"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	// Try relative to working directory
	cwd, err := os.Getwd()
	if err == nil {
		candidates := []string{
			filepath.Join(cwd, "migrations"),
			filepath.Join(cwd, "backend", "migrations"),
			filepath.Join(cwd, "..", "migrations"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	// Default fallback
	return "./migrations"
}

func runUp(config *database.MigrationConfig) {
	log.Println("Applying all pending migrations...")

	runner, err := database.NewMigrationRunner(config)
	if err != nil {
		log.Fatalf("Failed to create migration runner: %v", err)
	}
	defer runner.Close()

	if err := runner.RunMigrations(); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("All migrations applied successfully!")
}

func runDown(config *database.MigrationConfig) {
	log.Println("Rolling back last migration...")

	runner, err := database.NewMigrationRunner(config)
	if err != nil {
		log.Fatalf("Failed to create migration runner: %v", err)
	}
	defer runner.Close()

	if err := runner.RollbackMigration(); err != nil {
		log.Fatalf("Rollback failed: %v", err)
	}

	log.Println("Rollback completed successfully!")
}

func runDownAll(config *database.MigrationConfig) {
	log.Println("WARNING: This will rollback ALL migrations and delete all data!")
	log.Println("Press Ctrl+C within 5 seconds to cancel...")

	time.Sleep(5 * time.Second)

	runner, err := database.NewMigrationRunner(config)
	if err != nil {
		log.Fatalf("Failed to create migration runner: %v", err)
	}
	defer runner.Close()

	if err := runner.RollbackAll(); err != nil {
		log.Fatalf("Rollback all failed: %v", err)
	}

	log.Println("All migrations rolled back!")
}

func showVersion(config *database.MigrationConfig) {
	runner, err := database.NewMigrationRunner(config)
	if err != nil {
		log.Fatalf("Failed to create migration runner: %v", err)
	}
	defer runner.Close()

	status, err := runner.GetVersion()
	if err != nil {
		log.Fatalf("Failed to get version: %v", err)
	}

	fmt.Println("Current Migration Status:")
	fmt.Printf("  Version: %d\n", status.Version)
	fmt.Printf("  Dirty:   %v\n", status.Dirty)
	fmt.Printf("  Applied: %v\n", status.Applied)

	if status.Dirty {
		fmt.Println("\nWARNING: Database is in dirty state!")
		fmt.Println("This usually means a migration failed halfway.")
		fmt.Printf("Use 'migrate force %d' to fix, then retry.\n", status.Version-1)
	}
}

func runTo(config *database.MigrationConfig, version uint) {
	log.Printf("Migrating to version %d...", version)

	runner, err := database.NewMigrationRunner(config)
	if err != nil {
		log.Fatalf("Failed to create migration runner: %v", err)
	}
	defer runner.Close()

	if err := runner.MigrateToVersion(version); err != nil {
		log.Fatalf("Migration to version %d failed: %v", version, err)
	}

	log.Printf("Successfully migrated to version %d", version)
}

func runForce(config *database.MigrationConfig, version int) {
	log.Printf("Forcing migration version to %d...", version)
	log.Println("WARNING: This does not run any migrations, it only updates the version!")

	runner, err := database.NewMigrationRunner(config)
	if err != nil {
		log.Fatalf("Failed to create migration runner: %v", err)
	}
	defer runner.Close()

	if err := runner.Force(version); err != nil {
		log.Fatalf("Force failed: %v", err)
	}

	log.Printf("Version forced to %d", version)
}

func createMigration(migrationsPath, name string) {
	// Sanitize name
	name = strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	name = strings.ReplaceAll(name, "-", "_")

	// Find the next version number
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		log.Fatalf("Failed to read migrations directory: %v", err)
	}

	maxVersion := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if len(filename) >= 6 {
			if v, err := strconv.Atoi(filename[:6]); err == nil && v > maxVersion {
				maxVersion = v
			}
		}
	}

	nextVersion := maxVersion + 1
	prefix := fmt.Sprintf("%06d_%s", nextVersion, name)

	upFile := filepath.Join(migrationsPath, prefix+".up.sql")
	downFile := filepath.Join(migrationsPath, prefix+".down.sql")

	upContent := fmt.Sprintf(`-- Migration: %s
-- Created: %s
--
-- Description: TODO: Add description
--

-- Add your UP migration SQL here

`, name, time.Now().Format(time.RFC3339))

	downContent := fmt.Sprintf(`-- Rollback: %s
-- Created: %s
--
-- Description: Rollback for %s
--

-- Add your DOWN migration SQL here (reverse of UP)

`, name, time.Now().Format(time.RFC3339), name)

	if err := os.WriteFile(upFile, []byte(upContent), 0644); err != nil {
		log.Fatalf("Failed to create up migration: %v", err)
	}

	if err := os.WriteFile(downFile, []byte(downContent), 0644); err != nil {
		log.Fatalf("Failed to create down migration: %v", err)
	}

	fmt.Printf("Created migration files:\n")
	fmt.Printf("  %s\n", upFile)
	fmt.Printf("  %s\n", downFile)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
