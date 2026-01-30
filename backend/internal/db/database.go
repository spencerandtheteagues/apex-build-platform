package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"apex-build/internal/git"
	"apex-build/internal/mcp"
	"apex-build/internal/secrets"
	"apex-build/pkg/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database wraps the GORM database instance
type Database struct {
	DB *gorm.DB
}

// Config holds database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	TimeZone string
}

// NewDatabase creates a new database connection
func NewDatabase(config *Config) (*Database, error) {
	// Configure GORM with custom logger
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	var db *gorm.DB
	var err error

	// Construct DSN for PostgreSQL
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		config.Host, config.Port, config.User, config.Password,
		config.DBName, config.SSLMode, config.TimeZone,
	)
	
	// Open PostgreSQL connection
	db, err = gorm.Open(postgres.Open(dsn), gormConfig)

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	database := &Database{DB: db}

	// Run migrations
	if err := database.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("✅ Database connected successfully")
	return database, nil
}

// Migrate runs database migrations
func (d *Database) Migrate() error {
	log.Println("🔄 Running database migrations...")

	// Auto-migrate all models
	err := d.DB.AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.File{},
		&models.Session{},
		&models.AIRequest{},
		&models.Execution{},
		&models.CollabRoom{},
		&models.CursorPosition{},
		&models.ChatMessage{},
		&models.UserCollabRoom{},
		// Secrets management
		&secrets.Secret{},
		&secrets.SecretAuditLog{},
		// MCP server integration
		&mcp.ExternalMCPServer{},
		// Git integration
		&git.Repository{},
	)

	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Create indexes for performance
	if err := d.createIndexes(); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	log.Println("✅ Database migrations completed successfully")

	// Seed admin user
	if err := d.seedAdminUser(); err != nil {
		log.Printf("⚠️ Admin user seeding: %v", err)
	}

	return nil
}

// seedAdminUser creates the platform owner/admin account
func (d *Database) seedAdminUser() error {
	// Check if admin already exists
	var existingUser models.User
	result := d.DB.Where("email = ?", "spencerandtheteagues@gmail.com").First(&existingUser)

	// Admin password hash - credentials managed via environment variables
	// Use ADMIN_PASSWORD_HASH env var in production
	passwordHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if passwordHash == "" {
		// Fallback hash for development only - change in production
		passwordHash = "$2a$10$gkuvs.57YtZctLHfPY8Jr.OKcM725LVvlFV7/8agtpyyEBDNiTvA."
	}

	if result.Error == nil {
		// User exists, ensure admin privileges and password are set
		existingUser.PasswordHash = passwordHash
		existingUser.IsAdmin = true
		existingUser.IsSuperAdmin = true
		existingUser.HasUnlimitedCredits = true
		existingUser.BypassBilling = true
		existingUser.BypassRateLimits = true
		existingUser.SubscriptionType = "owner"
		existingUser.IsActive = true
		existingUser.IsVerified = true
		existingUser.CreditBalance = 999999999.99

		if err := d.DB.Save(&existingUser).Error; err != nil {
			return fmt.Errorf("failed to update admin privileges: %w", err)
		}
		log.Println("👑 Admin user privileges and password updated: spencerandtheteagues@gmail.com")
		return nil
	}

	// Create new admin user
	adminUser := models.User{
		Username:            "spencer",
		Email:               "spencerandtheteagues@gmail.com",
		PasswordHash:        passwordHash,
		FullName:            "Spencer Teague",
		IsActive:            true,
		IsVerified:          true,
		IsAdmin:             true,
		IsSuperAdmin:        true,
		HasUnlimitedCredits: true,
		BypassBilling:       true,
		BypassRateLimits:    true,
		SubscriptionType:    "owner",
		CreditBalance:       999999999.99,
		PreferredTheme:      "cyberpunk",
		PreferredAI:         "auto",
	}

	if err := d.DB.Create(&adminUser).Error; err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	log.Println("👑 Admin user created: spencerandtheteagues@gmail.com")
	return nil
}

// createIndexes creates additional database indexes for performance
func (d *Database) createIndexes() error {
	// User indexes
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_username_active ON users(username) WHERE is_active = true")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email_verified ON users(email) WHERE is_verified = true")

	// Project indexes
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_projects_owner_active ON projects(owner_id) WHERE deleted_at IS NULL")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_projects_public ON projects(is_public) WHERE is_public = true AND deleted_at IS NULL")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_projects_language ON projects(language) WHERE deleted_at IS NULL")

	// File indexes
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_files_project_path ON files(project_id, path) WHERE deleted_at IS NULL")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_files_hash ON files(hash) WHERE deleted_at IS NULL")

	// AI Request indexes
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ai_requests_user_date ON ai_requests(user_id, created_at DESC)")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ai_requests_provider_status ON ai_requests(provider, status)")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ai_requests_capability ON ai_requests(capability)")

	// Execution indexes
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_project_date ON executions(project_id, created_at DESC)")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_user_status ON executions(user_id, status)")

	// Session indexes
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_user_active ON sessions(user_id) WHERE is_active = true")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_expires ON sessions(expires_at) WHERE is_active = true")

	// Collaboration indexes
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_collab_rooms_project ON collab_rooms(project_id) WHERE is_active = true")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cursor_positions_room_active ON cursor_positions(room_id) WHERE is_active = true")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_messages_room_date ON chat_messages(room_id, created_at DESC) WHERE deleted_at IS NULL")

	// Secrets indexes
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_secrets_user_name ON secrets(user_id, name)")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_secrets_project ON secrets(project_id) WHERE project_id IS NOT NULL")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_secret_audit_logs_secret ON secret_audit_logs(secret_id, created_at DESC)")

	// MCP server indexes
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_external_mcp_servers_user ON external_mcp_servers(user_id)")
	d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_external_mcp_servers_project ON external_mcp_servers(project_id) WHERE project_id IS NOT NULL")

	return nil
}

// Health checks database connectivity
func (d *Database) Health() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// Close closes the database connection
func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GetDB returns the underlying GORM database instance
func (d *Database) GetDB() *gorm.DB {
	return d.DB
}

// GetStats returns database connection statistics
func (d *Database) GetStats() map[string]interface{} {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	stats := sqlDB.Stats()
	return map[string]interface{}{
		"max_open_connections":     stats.MaxOpenConnections,
		"open_connections":         stats.OpenConnections,
		"in_use":                  stats.InUse,
		"idle":                    stats.Idle,
		"wait_count":              stats.WaitCount,
		"wait_duration_ms":        stats.WaitDuration.Milliseconds(),
		"max_idle_closed":         stats.MaxIdleClosed,
		"max_idle_time_closed":    stats.MaxIdleTimeClosed,
		"max_lifetime_closed":     stats.MaxLifetimeClosed,
	}
}

// Transaction wraps a function in a database transaction
func (d *Database) Transaction(fn func(*gorm.DB) error) error {
	return d.DB.Transaction(fn)
}

// Repository interfaces for clean architecture
type UserRepository interface {
	Create(user *models.User) error
	GetByID(id uint) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	Update(user *models.User) error
	Delete(id uint) error
	List(limit, offset int) ([]models.User, error)
}

type ProjectRepository interface {
	Create(project *models.Project) error
	GetByID(id uint) (*models.Project, error)
	GetByUserID(userID uint, limit, offset int) ([]models.Project, error)
	GetPublic(limit, offset int) ([]models.Project, error)
	Update(project *models.Project) error
	Delete(id uint) error
}

type FileRepository interface {
	Create(file *models.File) error
	GetByID(id uint) (*models.File, error)
	GetByProjectID(projectID uint) ([]models.File, error)
	GetByPath(projectID uint, path string) (*models.File, error)
	Update(file *models.File) error
	Delete(id uint) error
	LockFile(fileID uint, userID uint) error
	UnlockFile(fileID uint, userID uint) error
}

type AIRequestRepository interface {
	Create(request *models.AIRequest) error
	GetByID(id uint) (*models.AIRequest, error)
	GetByUserID(userID uint, limit, offset int) ([]models.AIRequest, error)
	GetByProjectID(projectID uint, limit, offset int) ([]models.AIRequest, error)
	Update(request *models.AIRequest) error
	GetUsageStats(userID uint, fromDate time.Time) (*UsageStats, error)
}

type UsageStats struct {
	TotalRequests int     `json:"total_requests"`
	TotalCost     float64 `json:"total_cost"`
	TotalTokens   int     `json:"total_tokens"`
	ByProvider    map[string]*ProviderStats `json:"by_provider"`
}

type ProviderStats struct {
	Requests int     `json:"requests"`
	Cost     float64 `json:"cost"`
	Tokens   int     `json:"tokens"`
}

// ExecutionRepository interface
type ExecutionRepository interface {
	Create(execution *models.Execution) error
	GetByID(id uint) (*models.Execution, error)
	GetByProjectID(projectID uint, limit, offset int) ([]models.Execution, error)
	GetByUserID(userID uint, limit, offset int) ([]models.Execution, error)
	Update(execution *models.Execution) error
	Delete(id uint) error
}

// CollaborationRepository interface
type CollabRepository interface {
	CreateRoom(room *models.CollabRoom) error
	GetRoom(roomID string) (*models.CollabRoom, error)
	JoinRoom(roomID string, userID uint) error
	LeaveRoom(roomID string, userID uint) error
	UpdateCursor(cursor *models.CursorPosition) error
	GetActiveCursors(roomID uint) ([]models.CursorPosition, error)
	AddChatMessage(message *models.ChatMessage) error
	GetChatHistory(roomID uint, limit, offset int) ([]models.ChatMessage, error)
}

// DefaultConfig returns default database configuration
func DefaultConfig() *Config {
	return &Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "password",
		DBName:   "apex_build",
		SSLMode:  "disable",
		TimeZone: "UTC",
	}
}
