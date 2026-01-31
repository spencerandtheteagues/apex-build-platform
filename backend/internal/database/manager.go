// Package database - Managed Database Service for APEX.BUILD
// Provides PostgreSQL, Redis, and SQLite instances per project
package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// validIdentifier validates that a SQL identifier (table/column name) contains only safe characters
var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func sanitizeIdentifier(name string) (string, error) {
	if !validIdentifier.MatchString(name) {
		return "", fmt.Errorf("invalid identifier: %q", name)
	}
	return name, nil
}

// quoteIdentifier quotes a PostgreSQL identifier (table, database, user name)
// using double quotes to prevent SQL injection
func quoteIdentifier(name string) (string, error) {
	// First validate the identifier contains only safe characters
	if !validIdentifier.MatchString(name) {
		return "", fmt.Errorf("invalid identifier: %q", name)
	}
	// Double any existing double quotes (PostgreSQL escaping)
	escaped := strings.ReplaceAll(name, `"`, `""`)
	return `"` + escaped + `"`, nil
}

// escapeLiteral escapes a string for use in PostgreSQL SQL as a literal value
// This is used for values like passwords that can contain any characters
func escapeLiteral(value string) string {
	// PostgreSQL uses '' to escape single quotes in string literals
	escaped := strings.ReplaceAll(value, `'`, `''`)
	// Also escape backslashes for standard_conforming_strings = on (default)
	escaped = strings.ReplaceAll(escaped, `\`, `\\`)
	return escaped
}

// DatabaseType represents the type of managed database
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "postgresql"
	DatabaseTypeRedis      DatabaseType = "redis"
	DatabaseTypeSQLite     DatabaseType = "sqlite"
)

// DatabaseStatus represents the current state of a database
type DatabaseStatus string

const (
	DatabaseStatusProvisioning DatabaseStatus = "provisioning"
	DatabaseStatusActive       DatabaseStatus = "active"
	DatabaseStatusSuspended    DatabaseStatus = "suspended"
	DatabaseStatusError        DatabaseStatus = "error"
	DatabaseStatusDeleting     DatabaseStatus = "deleting"
)

// ManagedDatabase represents a user-managed database instance
type ManagedDatabase struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	ProjectID      uint           `json:"project_id" gorm:"index;not null"`
	UserID         uint           `json:"user_id" gorm:"index;not null"`
	Type           DatabaseType   `json:"type" gorm:"not null"`
	Name           string         `json:"name" gorm:"not null"`
	Host           string         `json:"host,omitempty"`
	Port           int            `json:"port,omitempty"`
	Username       string         `json:"username,omitempty"`
	Password       string         `json:"-" gorm:"not null"` // Encrypted, never expose
	Salt           string         `json:"-" gorm:"not null"` // For password encryption
	DatabaseName   string         `json:"database_name,omitempty"`
	Status         DatabaseStatus `json:"status" gorm:"default:'provisioning'"`
	ConnectionURL  string         `json:"-"` // Computed, not stored
	FilePath       string         `json:"-"` // For SQLite only

	// Auto-provisioning flag (Replit parity)
	IsAutoProvisioned bool `json:"is_auto_provisioned" gorm:"default:false"` // True if auto-created with project

	// Usage metrics
	StorageUsedMB  float64   `json:"storage_used_mb" gorm:"default:0"`
	ConnectionCount int      `json:"connection_count" gorm:"default:0"`
	QueryCount      int64    `json:"query_count" gorm:"default:0"`
	LastQueried     *time.Time `json:"last_queried,omitempty"`

	// Backup configuration
	BackupEnabled   bool      `json:"backup_enabled" gorm:"default:true"`
	BackupSchedule  string    `json:"backup_schedule,omitempty"` // Cron expression
	LastBackup      *time.Time `json:"last_backup,omitempty"`
	NextBackup      *time.Time `json:"next_backup,omitempty"`

	// Plan limits
	MaxStorageMB    int       `json:"max_storage_mb" gorm:"default:100"`
	MaxConnections  int       `json:"max_connections" gorm:"default:5"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// DatabaseCredentials contains connection information (for API response)
type DatabaseCredentials struct {
	Host         string `json:"host,omitempty"`
	Port         int    `json:"port,omitempty"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	DatabaseName string `json:"database_name,omitempty"`
	ConnectionURL string `json:"connection_url"`
}

// DatabaseMetrics contains usage statistics
type DatabaseMetrics struct {
	StorageUsedMB   float64   `json:"storage_used_mb"`
	ConnectionCount int       `json:"connection_count"`
	QueryCount      int64     `json:"query_count"`
	LastQueried     *time.Time `json:"last_queried,omitempty"`
	TableCount      int       `json:"table_count"`
	RowCount        int64     `json:"row_count"`
}

// TableInfo represents metadata about a database table
type TableInfo struct {
	Name       string `json:"name"`
	RowCount   int64  `json:"row_count"`
	SizeBytes  int64  `json:"size_bytes"`
	ColumnCount int   `json:"column_count"`
}

// ColumnInfo represents metadata about a table column
type ColumnInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Nullable     bool   `json:"nullable"`
	DefaultValue string `json:"default_value,omitempty"`
	IsPrimaryKey bool   `json:"is_primary_key"`
}

// QueryResult represents the result of a SQL query
type QueryResult struct {
	Columns      []string        `json:"columns"`
	Rows         [][]interface{} `json:"rows"`
	RowCount     int             `json:"row_count"`
	AffectedRows int64           `json:"affected_rows,omitempty"`
	Duration     time.Duration   `json:"duration_ms"`
	Error        string          `json:"error,omitempty"`
}

// DatabaseManager handles managed database operations
type DatabaseManager struct {
	baseDir       string
	postgresHost  string
	postgresPort  int
	redisHost     string
	redisPort     int
	encryptionKey []byte

	// Connection pools
	pgConnections    map[uint]*sql.DB // projectID -> connection
	redisConnections map[uint]*redis.Client
	sqliteConnections map[uint]*sql.DB
	mu               sync.RWMutex
}

// ManagerConfig holds configuration for the database manager
type ManagerConfig struct {
	BaseDir       string
	PostgresHost  string
	PostgresPort  int
	RedisHost     string
	RedisPort     int
	EncryptionKey string
}

// NewDatabaseManager creates a new database manager
func NewDatabaseManager(config *ManagerConfig) (*DatabaseManager, error) {
	if config.BaseDir == "" {
		config.BaseDir = "/tmp/apex-build-databases"
	}

	// Ensure base directory exists
	if err := os.MkdirAll(config.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	var encKey []byte
	if config.EncryptionKey != "" {
		var err error
		encKey, err = hex.DecodeString(config.EncryptionKey)
		if err != nil {
			// Generate a new key if decoding fails
			encKey = make([]byte, 32)
			if _, err := io.ReadFull(rand.Reader, encKey); err != nil {
				return nil, fmt.Errorf("failed to generate encryption key: %w", err)
			}
		}
	} else {
		encKey = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, encKey); err != nil {
			return nil, fmt.Errorf("failed to generate encryption key: %w", err)
		}
	}

	return &DatabaseManager{
		baseDir:          config.BaseDir,
		postgresHost:     config.PostgresHost,
		postgresPort:     config.PostgresPort,
		redisHost:        config.RedisHost,
		redisPort:        config.RedisPort,
		encryptionKey:    encKey,
		pgConnections:    make(map[uint]*sql.DB),
		redisConnections: make(map[uint]*redis.Client),
		sqliteConnections: make(map[uint]*sql.DB),
	}, nil
}

// CreateDatabase provisions a new managed database
func (dm *DatabaseManager) CreateDatabase(db *ManagedDatabase) error {
	// Generate credentials
	db.Username = generateUsername(db.ProjectID)
	password := generateSecurePassword(24)
	db.Password = password // Will be encrypted before storage
	db.Salt = generateSalt()

	switch db.Type {
	case DatabaseTypeSQLite:
		return dm.createSQLiteDatabase(db)
	case DatabaseTypePostgreSQL:
		return dm.createPostgreSQLDatabase(db)
	case DatabaseTypeRedis:
		return dm.createRedisDatabase(db)
	default:
		return fmt.Errorf("unsupported database type: %s", db.Type)
	}
}

// createSQLiteDatabase creates a new SQLite database file
func (dm *DatabaseManager) createSQLiteDatabase(db *ManagedDatabase) error {
	// Create project-specific directory
	projectDir := filepath.Join(dm.baseDir, fmt.Sprintf("project_%d", db.ProjectID))
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Create database file
	dbPath := filepath.Join(projectDir, fmt.Sprintf("%s.db", db.Name))
	db.FilePath = dbPath
	db.DatabaseName = db.Name

	// Initialize SQLite database
	sqliteDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to create SQLite database: %w", err)
	}

	// Test connection
	if err := sqliteDB.Ping(); err != nil {
		sqliteDB.Close()
		return fmt.Errorf("failed to connect to SQLite database: %w", err)
	}

	// Store connection
	dm.mu.Lock()
	dm.sqliteConnections[db.ID] = sqliteDB
	dm.mu.Unlock()

	db.Status = DatabaseStatusActive
	db.Host = "localhost"
	db.Port = 0 // SQLite doesn't use a port

	return nil
}

// createPostgreSQLDatabase creates a new PostgreSQL database
func (dm *DatabaseManager) createPostgreSQLDatabase(db *ManagedDatabase) error {
	if dm.postgresHost == "" {
		// Use default embedded/local PostgreSQL
		dm.postgresHost = "localhost"
		dm.postgresPort = 5432
	}

	db.Host = dm.postgresHost
	db.Port = dm.postgresPort
	db.DatabaseName = fmt.Sprintf("apex_project_%d_%s", db.ProjectID, strings.ToLower(db.Name))

	// Connect to PostgreSQL server to create the database
	adminDSN := fmt.Sprintf("host=%s port=%d user=postgres sslmode=disable",
		dm.postgresHost, dm.postgresPort)

	adminDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		// If PostgreSQL is not available, fall back to SQLite
		db.Type = DatabaseTypeSQLite
		return dm.createSQLiteDatabase(db)
	}
	defer adminDB.Close()

	// Validate and quote identifiers to prevent SQL injection
	quotedDBName, err := quoteIdentifier(db.DatabaseName)
	if err != nil {
		return fmt.Errorf("invalid database name: %w", err)
	}
	quotedUsername, err := quoteIdentifier(db.Username)
	if err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}
	escapedPassword := escapeLiteral(db.Password)

	// Create the database
	_, err = adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", quotedDBName))
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create PostgreSQL database: %w", err)
	}

	// Create user with password
	_, err = adminDB.Exec(fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", quotedUsername, escapedPassword))
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create PostgreSQL user: %w", err)
	}

	// Grant privileges
	_, err = adminDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", quotedDBName, quotedUsername))
	if err != nil {
		return fmt.Errorf("failed to grant privileges: %w", err)
	}

	db.Status = DatabaseStatusActive
	return nil
}

// createRedisDatabase creates a new Redis database (namespace)
func (dm *DatabaseManager) createRedisDatabase(db *ManagedDatabase) error {
	if dm.redisHost == "" {
		dm.redisHost = "localhost"
		dm.redisPort = 6379
	}

	db.Host = dm.redisHost
	db.Port = dm.redisPort
	// Redis uses database numbers (0-15) or key prefixes
	db.DatabaseName = fmt.Sprintf("apex:project_%d:%s", db.ProjectID, db.Name)

	// Test Redis connection
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", dm.redisHost, dm.redisPort),
		Password: "", // Redis auth handled separately
		DB:       0,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	dm.mu.Lock()
	dm.redisConnections[db.ID] = client
	dm.mu.Unlock()

	db.Status = DatabaseStatusActive
	return nil
}

// GetConnectionURL returns the connection URL for a database
func (dm *DatabaseManager) GetConnectionURL(db *ManagedDatabase, password string) string {
	switch db.Type {
	case DatabaseTypeSQLite:
		return fmt.Sprintf("sqlite://%s", db.FilePath)
	case DatabaseTypePostgreSQL:
		return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable",
			db.Username, password, db.Host, db.Port, db.DatabaseName)
	case DatabaseTypeRedis:
		return fmt.Sprintf("redis://%s:%d/0#%s", db.Host, db.Port, db.DatabaseName)
	default:
		return ""
	}
}

// GetCredentials returns decrypted credentials for a database
func (dm *DatabaseManager) GetCredentials(db *ManagedDatabase, decryptedPassword string) *DatabaseCredentials {
	return &DatabaseCredentials{
		Host:          db.Host,
		Port:          db.Port,
		Username:      db.Username,
		Password:      decryptedPassword,
		DatabaseName:  db.DatabaseName,
		ConnectionURL: dm.GetConnectionURL(db, decryptedPassword),
	}
}

// ResetCredentials generates new credentials for a database
func (dm *DatabaseManager) ResetCredentials(db *ManagedDatabase) (string, error) {
	newPassword := generateSecurePassword(24)
	db.Salt = generateSalt()

	switch db.Type {
	case DatabaseTypePostgreSQL:
		// Update password in PostgreSQL
		adminDSN := fmt.Sprintf("host=%s port=%d user=postgres sslmode=disable",
			db.Host, db.Port)
		adminDB, err := sql.Open("postgres", adminDSN)
		if err != nil {
			return "", fmt.Errorf("failed to connect to PostgreSQL: %w", err)
		}
		defer adminDB.Close()

		quotedUsername, qErr := quoteIdentifier(db.Username)
		if qErr != nil {
			return "", fmt.Errorf("invalid username: %w", qErr)
		}
		escapedPassword := escapeLiteral(newPassword)
		_, err = adminDB.Exec(fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s'", quotedUsername, escapedPassword))
		if err != nil {
			return "", fmt.Errorf("failed to reset password: %w", err)
		}
	case DatabaseTypeRedis:
		// Redis password changes require server restart or AUTH command
		// For now, we just update the stored password
	case DatabaseTypeSQLite:
		// SQLite doesn't use passwords
	}

	return newPassword, nil
}

// DeleteDatabase removes a managed database
func (dm *DatabaseManager) DeleteDatabase(db *ManagedDatabase) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	switch db.Type {
	case DatabaseTypeSQLite:
		// Close connection if exists
		if conn, exists := dm.sqliteConnections[db.ID]; exists {
			conn.Close()
			delete(dm.sqliteConnections, db.ID)
		}
		// Delete file
		if db.FilePath != "" {
			os.Remove(db.FilePath)
		}

	case DatabaseTypePostgreSQL:
		// Close connection if exists
		if conn, exists := dm.pgConnections[db.ID]; exists {
			conn.Close()
			delete(dm.pgConnections, db.ID)
		}
		// Drop database and user
		adminDSN := fmt.Sprintf("host=%s port=%d user=postgres sslmode=disable",
			db.Host, db.Port)
		adminDB, err := sql.Open("postgres", adminDSN)
		if err == nil {
			// Quote identifiers safely to prevent SQL injection
			if quotedDBName, qErr := quoteIdentifier(db.DatabaseName); qErr == nil {
				adminDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", quotedDBName))
			}
			if quotedUsername, qErr := quoteIdentifier(db.Username); qErr == nil {
				adminDB.Exec(fmt.Sprintf("DROP USER IF EXISTS %s", quotedUsername))
			}
			adminDB.Close()
		}

	case DatabaseTypeRedis:
		// Close connection if exists
		if conn, exists := dm.redisConnections[db.ID]; exists {
			// Delete all keys with the prefix
			ctx := context.Background()
			keys, _ := conn.Keys(ctx, db.DatabaseName+"*").Result()
			if len(keys) > 0 {
				conn.Del(ctx, keys...)
			}
			conn.Close()
			delete(dm.redisConnections, db.ID)
		}
	}

	return nil
}

// ExecuteQuery runs a SQL query on a managed database
func (dm *DatabaseManager) ExecuteQuery(db *ManagedDatabase, query string, password string) (*QueryResult, error) {
	switch db.Type {
	case DatabaseTypeSQLite:
		return dm.executeSQLiteQuery(db, query)
	case DatabaseTypePostgreSQL:
		return dm.executePostgreSQLQuery(db, query, password)
	case DatabaseTypeRedis:
		return nil, fmt.Errorf("Redis does not support SQL queries. Use Redis commands instead.")
	default:
		return nil, fmt.Errorf("unsupported database type")
	}
}

func (dm *DatabaseManager) executeSQLiteQuery(db *ManagedDatabase, query string) (*QueryResult, error) {
	start := time.Now()

	dm.mu.RLock()
	conn, exists := dm.sqliteConnections[db.ID]
	dm.mu.RUnlock()

	if !exists {
		var err error
		conn, err = sql.Open("sqlite", db.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open SQLite database: %w", err)
		}
		dm.mu.Lock()
		dm.sqliteConnections[db.ID] = conn
		dm.mu.Unlock()
	}

	return dm.executeSQL(conn, query, start)
}

func (dm *DatabaseManager) executePostgreSQLQuery(db *ManagedDatabase, query string, password string) (*QueryResult, error) {
	start := time.Now()

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		db.Host, db.Port, db.Username, password, db.DatabaseName)

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	defer conn.Close()

	return dm.executeSQL(conn, query, start)
}

func (dm *DatabaseManager) executeSQL(conn *sql.DB, query string, start time.Time) (*QueryResult, error) {
	result := &QueryResult{}

	// Determine if query is a SELECT or not
	queryType := strings.TrimSpace(strings.ToUpper(query))

	if strings.HasPrefix(queryType, "SELECT") || strings.HasPrefix(queryType, "SHOW") ||
	   strings.HasPrefix(queryType, "DESCRIBE") || strings.HasPrefix(queryType, "EXPLAIN") {
		// Query that returns rows
		rows, err := conn.Query(query)
		if err != nil {
			result.Error = err.Error()
			result.Duration = time.Since(start)
			return result, nil
		}
		defer rows.Close()

		// Get column names
		columns, err := rows.Columns()
		if err != nil {
			result.Error = err.Error()
			result.Duration = time.Since(start)
			return result, nil
		}
		result.Columns = columns

		// Read rows
		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				result.Error = err.Error()
				break
			}

			// Convert to JSON-safe values
			row := make([]interface{}, len(columns))
			for i, v := range values {
				switch val := v.(type) {
				case []byte:
					row[i] = string(val)
				case time.Time:
					row[i] = val.Format(time.RFC3339)
				default:
					row[i] = val
				}
			}
			result.Rows = append(result.Rows, row)
		}
		result.RowCount = len(result.Rows)

	} else {
		// Statement that doesn't return rows
		res, err := conn.Exec(query)
		if err != nil {
			result.Error = err.Error()
			result.Duration = time.Since(start)
			return result, nil
		}
		result.AffectedRows, _ = res.RowsAffected()
	}

	result.Duration = time.Since(start)
	return result, nil
}

// GetTables returns a list of tables in the database
func (dm *DatabaseManager) GetTables(db *ManagedDatabase, password string) ([]TableInfo, error) {
	var query string

	switch db.Type {
	case DatabaseTypeSQLite:
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
	case DatabaseTypePostgreSQL:
		query = `SELECT tablename as name FROM pg_tables WHERE schemaname = 'public'`
	default:
		return nil, fmt.Errorf("unsupported database type")
	}

	result, err := dm.ExecuteQuery(db, query, password)
	if err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf(result.Error)
	}

	tables := make([]TableInfo, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) > 0 {
			tableName := fmt.Sprintf("%v", row[0])

			// Sanitize table name to prevent SQL injection
			safeName, err := sanitizeIdentifier(tableName)
			if err != nil {
				continue // skip invalid table names
			}

			// Get row count
			countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", safeName)
			countResult, _ := dm.ExecuteQuery(db, countQuery, password)
			var rowCount int64
			if countResult != nil && len(countResult.Rows) > 0 && len(countResult.Rows[0]) > 0 {
				if count, ok := countResult.Rows[0][0].(int64); ok {
					rowCount = count
				}
			}

			tables = append(tables, TableInfo{
				Name:     tableName,
				RowCount: rowCount,
			})
		}
	}

	return tables, nil
}

// GetTableSchema returns column information for a table
func (dm *DatabaseManager) GetTableSchema(db *ManagedDatabase, tableName string, password string) ([]ColumnInfo, error) {
	// Sanitize table name to prevent SQL injection
	safeName, err := sanitizeIdentifier(tableName)
	if err != nil {
		return nil, fmt.Errorf("invalid table name: %w", err)
	}

	var query string

	switch db.Type {
	case DatabaseTypeSQLite:
		query = fmt.Sprintf("PRAGMA table_info(%s)", safeName)
	case DatabaseTypePostgreSQL:
		query = fmt.Sprintf(`
			SELECT column_name, data_type, is_nullable, column_default,
				   CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_pk
			FROM information_schema.columns c
			LEFT JOIN (
				SELECT kcu.column_name
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
				WHERE tc.table_name = '%s' AND tc.constraint_type = 'PRIMARY KEY'
			) pk ON c.column_name = pk.column_name
			WHERE c.table_name = '%s'
			ORDER BY c.ordinal_position`, safeName, safeName)
	default:
		return nil, fmt.Errorf("unsupported database type")
	}

	result, err := dm.ExecuteQuery(db, query, password)
	if err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf(result.Error)
	}

	columns := make([]ColumnInfo, 0, len(result.Rows))
	for _, row := range result.Rows {
		var col ColumnInfo

		switch db.Type {
		case DatabaseTypeSQLite:
			if len(row) >= 6 {
				col.Name = fmt.Sprintf("%v", row[1])
				col.Type = fmt.Sprintf("%v", row[2])
				col.Nullable = row[3] == int64(0)
				if row[4] != nil {
					col.DefaultValue = fmt.Sprintf("%v", row[4])
				}
				col.IsPrimaryKey = row[5] == int64(1)
			}
		case DatabaseTypePostgreSQL:
			if len(row) >= 5 {
				col.Name = fmt.Sprintf("%v", row[0])
				col.Type = fmt.Sprintf("%v", row[1])
				col.Nullable = row[2] == "YES"
				if row[3] != nil {
					col.DefaultValue = fmt.Sprintf("%v", row[3])
				}
				col.IsPrimaryKey = row[4] == true
			}
		}

		columns = append(columns, col)
	}

	return columns, nil
}

// GetMetrics returns usage metrics for a database
func (dm *DatabaseManager) GetMetrics(db *ManagedDatabase, password string) (*DatabaseMetrics, error) {
	metrics := &DatabaseMetrics{
		StorageUsedMB:   db.StorageUsedMB,
		ConnectionCount: db.ConnectionCount,
		QueryCount:      db.QueryCount,
		LastQueried:     db.LastQueried,
	}

	// Get table count
	tables, err := dm.GetTables(db, password)
	if err == nil {
		metrics.TableCount = len(tables)
		for _, t := range tables {
			metrics.RowCount += t.RowCount
		}
	}

	return metrics, nil
}

// AutoProvisionPostgreSQLForProject creates a PostgreSQL database automatically for a new project
// This is used during project creation to provide Replit-like auto-provisioned database experience
func (dm *DatabaseManager) AutoProvisionPostgreSQLForProject(projectID, userID uint, projectName string) (*ManagedDatabase, error) {
	// Sanitize project name for database name (lowercase, alphanumeric + underscore only)
	safeName := sanitizeDBName(projectName)
	if safeName == "" {
		safeName = "main"
	}

	// Create the managed database record
	db := &ManagedDatabase{
		ProjectID:         projectID,
		UserID:            userID,
		Name:              safeName,
		Type:              DatabaseTypePostgreSQL,
		Status:            DatabaseStatusProvisioning,
		IsAutoProvisioned: true, // Mark as auto-provisioned
		BackupEnabled:     true,
		BackupSchedule:    "0 0 * * *", // Daily at midnight
		MaxStorageMB:      100,         // Default 100MB for free tier
		MaxConnections:    5,           // Default 5 connections
	}

	// Provision the actual database
	if err := dm.CreateDatabase(db); err != nil {
		return nil, fmt.Errorf("failed to provision database: %w", err)
	}

	return db, nil
}

// GetConnectionString returns the connection string for environment variable injection
func (dm *DatabaseManager) GetConnectionString(db *ManagedDatabase, password string) string {
	return dm.GetConnectionURL(db, password)
}

// sanitizeDBName ensures the database name is safe for PostgreSQL
func sanitizeDBName(name string) string {
	result := ""
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			result += string(r)
		} else if r == ' ' || r == '-' {
			result += "_"
		}
	}
	// Ensure it doesn't start with a number
	if len(result) > 0 && result[0] >= '0' && result[0] <= '9' {
		result = "db_" + result
	}
	// Limit length
	if len(result) > 32 {
		result = result[:32]
	}
	return result
}

// Helper functions

func generateUsername(projectID uint) string {
	return fmt.Sprintf("apex_p%d", projectID)
}

func generateSecurePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		// Fallback to less secure random
		for i := range b {
			b[i] = charset[i%len(charset)]
		}
	} else {
		for i := range b {
			b[i] = charset[int(b[i])%len(charset)]
		}
	}
	return string(b)
}

func generateSalt() string {
	salt := make([]byte, 16)
	io.ReadFull(rand.Reader, salt)
	return hex.EncodeToString(salt)
}
