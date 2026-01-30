// Package handlers - Managed Database Handlers for APEX.BUILD
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"apex-build/internal/database"
	"apex-build/internal/middleware"
	"apex-build/internal/secrets"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DatabaseHandler handles managed database operations
type DatabaseHandler struct {
	DB             *gorm.DB
	Manager        *database.DatabaseManager
	SecretsManager *secrets.SecretsManager
}

// NewDatabaseHandler creates a new database handler
func NewDatabaseHandler(db *gorm.DB, manager *database.DatabaseManager, secretsManager *secrets.SecretsManager) *DatabaseHandler {
	return &DatabaseHandler{
		DB:             db,
		Manager:        manager,
		SecretsManager: secretsManager,
	}
}

// CreateDatabaseRequest represents a request to create a new database
type CreateDatabaseRequest struct {
	Name string              `json:"name" binding:"required,min=1,max=64"`
	Type database.DatabaseType `json:"type" binding:"required"`
}

// UpdateDatabaseRequest represents a request to update database settings
type UpdateDatabaseRequest struct {
	BackupEnabled  *bool   `json:"backup_enabled,omitempty"`
	BackupSchedule *string `json:"backup_schedule,omitempty"`
}

// ExecuteQueryRequest represents a SQL query execution request
type ExecuteQueryRequest struct {
	Query string `json:"query" binding:"required"`
}

// CreateDatabase creates a new managed database for a project
func (h *DatabaseHandler) CreateDatabase(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Verify project ownership
	var projectCount int64
	h.DB.Table("projects").Where("id = ? AND owner_id = ?", projectID, userID).Count(&projectCount)
	if projectCount == 0 {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Project not found or access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	var req CreateDatabaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request: " + err.Error(),
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Validate database type
	validTypes := map[database.DatabaseType]bool{
		database.DatabaseTypePostgreSQL: true,
		database.DatabaseTypeRedis:      true,
		database.DatabaseTypeSQLite:     true,
	}
	if !validTypes[req.Type] {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid database type. Must be: postgresql, redis, or sqlite",
			Code:    "INVALID_DATABASE_TYPE",
		})
		return
	}

	// Check if a database with this name already exists for the project
	var existingCount int64
	h.DB.Model(&database.ManagedDatabase{}).
		Where("project_id = ? AND name = ?", projectID, req.Name).
		Count(&existingCount)
	if existingCount > 0 {
		c.JSON(http.StatusConflict, StandardResponse{
			Success: false,
			Error:   "A database with this name already exists in this project",
			Code:    "DATABASE_EXISTS",
		})
		return
	}

	// Check plan limits (number of databases per project)
	var dbCount int64
	h.DB.Model(&database.ManagedDatabase{}).Where("project_id = ?", projectID).Count(&dbCount)
	// Default limit: 3 databases per project (can be overridden by plan)
	maxDatabases := 3
	if dbCount >= int64(maxDatabases) {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Maximum number of databases reached for this project",
			Code:    "LIMIT_EXCEEDED",
		})
		return
	}

	// Create the managed database
	managedDB := &database.ManagedDatabase{
		ProjectID:      uint(projectID),
		UserID:         userID,
		Name:           req.Name,
		Type:           req.Type,
		Status:         database.DatabaseStatusProvisioning,
		BackupEnabled:  true,
		BackupSchedule: "0 0 * * *", // Daily at midnight
		MaxStorageMB:   100,         // Default 100MB
		MaxConnections: 5,           // Default 5 connections
	}

	// Provision the database
	if err := h.Manager.CreateDatabase(managedDB); err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to provision database: " + err.Error(),
			Code:    "PROVISIONING_FAILED",
		})
		return
	}

	// Encrypt the password before storing
	encryptedPassword, salt, _, err := h.SecretsManager.Encrypt(userID, managedDB.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to encrypt credentials",
			Code:    "ENCRYPTION_FAILED",
		})
		return
	}
	managedDB.Password = encryptedPassword
	managedDB.Salt = salt

	// Calculate next backup time
	nextBackup := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)
	managedDB.NextBackup = &nextBackup

	// Save to database
	if err := h.DB.Create(managedDB).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to save database record",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusCreated, StandardResponse{
		Success: true,
		Data: gin.H{
			"database": h.sanitizeDatabase(managedDB),
		},
		Message: "Database created successfully",
	})
}

// ListDatabases returns all databases for a project
func (h *DatabaseHandler) ListDatabases(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Verify project ownership
	var projectCount int64
	h.DB.Table("projects").Where("id = ? AND owner_id = ?", projectID, userID).Count(&projectCount)
	if projectCount == 0 {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Project not found or access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	var databases []database.ManagedDatabase
	if err := h.DB.Where("project_id = ?", projectID).Order("created_at DESC").Find(&databases).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch databases",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Sanitize output (remove passwords)
	sanitized := make([]gin.H, len(databases))
	for i, db := range databases {
		sanitized[i] = h.sanitizeDatabase(&db)
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: gin.H{
			"databases": sanitized,
			"count":     len(databases),
		},
	})
}

// GetDatabase returns details for a specific database
func (h *DatabaseHandler) GetDatabase(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	dbID, err := strconv.ParseUint(c.Param("dbId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid database ID",
			Code:    "INVALID_DATABASE_ID",
		})
		return
	}

	var managedDB database.ManagedDatabase
	if err := h.DB.Where("id = ? AND project_id = ? AND user_id = ?", dbID, projectID, userID).First(&managedDB).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Database not found",
			Code:    "NOT_FOUND",
		})
		return
	}

	// Optionally include credentials (reveal mode)
	includeCredentials := c.Query("include_credentials") == "true"

	response := h.sanitizeDatabase(&managedDB)

	if includeCredentials {
		// Decrypt password
		decryptedPassword, err := h.SecretsManager.Decrypt(userID, managedDB.Password, managedDB.Salt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Success: false,
				Error:   "Failed to decrypt credentials",
				Code:    "DECRYPTION_FAILED",
			})
			return
		}

		credentials := h.Manager.GetCredentials(&managedDB, decryptedPassword)
		response["credentials"] = credentials
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: gin.H{
			"database": response,
		},
	})
}

// DeleteDatabase removes a managed database
func (h *DatabaseHandler) DeleteDatabase(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	dbID, err := strconv.ParseUint(c.Param("dbId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid database ID",
			Code:    "INVALID_DATABASE_ID",
		})
		return
	}

	var managedDB database.ManagedDatabase
	if err := h.DB.Where("id = ? AND project_id = ? AND user_id = ?", dbID, projectID, userID).First(&managedDB).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Database not found",
			Code:    "NOT_FOUND",
		})
		return
	}

	// Update status to deleting
	managedDB.Status = database.DatabaseStatusDeleting
	h.DB.Save(&managedDB)

	// Delete the actual database resources
	if err := h.Manager.DeleteDatabase(&managedDB); err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to delete database: " + err.Error(),
			Code:    "DELETE_FAILED",
		})
		return
	}

	// Delete record from database
	if err := h.DB.Delete(&managedDB).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to delete database record",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Database deleted successfully",
	})
}

// ResetCredentials generates new credentials for a database
func (h *DatabaseHandler) ResetCredentials(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	dbID, err := strconv.ParseUint(c.Param("dbId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid database ID",
			Code:    "INVALID_DATABASE_ID",
		})
		return
	}

	var managedDB database.ManagedDatabase
	if err := h.DB.Where("id = ? AND project_id = ? AND user_id = ?", dbID, projectID, userID).First(&managedDB).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Database not found",
			Code:    "NOT_FOUND",
		})
		return
	}

	// Generate new credentials
	newPassword, err := h.Manager.ResetCredentials(&managedDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to reset credentials: " + err.Error(),
			Code:    "RESET_FAILED",
		})
		return
	}

	// Encrypt new password
	encryptedPassword, salt, _, err := h.SecretsManager.Encrypt(userID, newPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to encrypt new credentials",
			Code:    "ENCRYPTION_FAILED",
		})
		return
	}

	managedDB.Password = encryptedPassword
	managedDB.Salt = salt

	if err := h.DB.Save(&managedDB).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to update credentials",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Return new credentials
	credentials := h.Manager.GetCredentials(&managedDB, newPassword)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: gin.H{
			"credentials": credentials,
		},
		Message: "Credentials reset successfully",
	})
}

// ExecuteQuery runs a SQL query on the database
func (h *DatabaseHandler) ExecuteQuery(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	dbID, err := strconv.ParseUint(c.Param("dbId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid database ID",
			Code:    "INVALID_DATABASE_ID",
		})
		return
	}

	var req ExecuteQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request: query is required",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	var managedDB database.ManagedDatabase
	if err := h.DB.Where("id = ? AND project_id = ? AND user_id = ?", dbID, projectID, userID).First(&managedDB).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Database not found",
			Code:    "NOT_FOUND",
		})
		return
	}

	if managedDB.Status != database.DatabaseStatusActive {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Database is not active",
			Code:    "DATABASE_NOT_ACTIVE",
		})
		return
	}

	// Decrypt password
	decryptedPassword, err := h.SecretsManager.Decrypt(userID, managedDB.Password, managedDB.Salt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to decrypt credentials",
			Code:    "DECRYPTION_FAILED",
		})
		return
	}

	// Execute query
	result, err := h.Manager.ExecuteQuery(&managedDB, req.Query, decryptedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Query execution failed: " + err.Error(),
			Code:    "QUERY_FAILED",
		})
		return
	}

	// Update usage stats
	now := time.Now()
	managedDB.QueryCount++
	managedDB.LastQueried = &now
	h.DB.Save(&managedDB)

	c.JSON(http.StatusOK, StandardResponse{
		Success: result.Error == "",
		Data: gin.H{
			"result": result,
		},
		Error: result.Error,
	})
}

// GetTables returns a list of tables in the database
func (h *DatabaseHandler) GetTables(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	dbID, err := strconv.ParseUint(c.Param("dbId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid database ID",
			Code:    "INVALID_DATABASE_ID",
		})
		return
	}

	var managedDB database.ManagedDatabase
	if err := h.DB.Where("id = ? AND project_id = ? AND user_id = ?", dbID, projectID, userID).First(&managedDB).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Database not found",
			Code:    "NOT_FOUND",
		})
		return
	}

	// Decrypt password
	decryptedPassword, err := h.SecretsManager.Decrypt(userID, managedDB.Password, managedDB.Salt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to decrypt credentials",
			Code:    "DECRYPTION_FAILED",
		})
		return
	}

	tables, err := h.Manager.GetTables(&managedDB, decryptedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to get tables: " + err.Error(),
			Code:    "QUERY_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: gin.H{
			"tables": tables,
		},
	})
}

// GetTableSchema returns the schema for a specific table
func (h *DatabaseHandler) GetTableSchema(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	dbID, err := strconv.ParseUint(c.Param("dbId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid database ID",
			Code:    "INVALID_DATABASE_ID",
		})
		return
	}

	tableName := c.Param("table")
	if tableName == "" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Table name is required",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	var managedDB database.ManagedDatabase
	if err := h.DB.Where("id = ? AND project_id = ? AND user_id = ?", dbID, projectID, userID).First(&managedDB).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Database not found",
			Code:    "NOT_FOUND",
		})
		return
	}

	// Decrypt password
	decryptedPassword, err := h.SecretsManager.Decrypt(userID, managedDB.Password, managedDB.Salt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to decrypt credentials",
			Code:    "DECRYPTION_FAILED",
		})
		return
	}

	columns, err := h.Manager.GetTableSchema(&managedDB, tableName, decryptedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to get table schema: " + err.Error(),
			Code:    "QUERY_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: gin.H{
			"table":   tableName,
			"columns": columns,
		},
	})
}

// GetMetrics returns usage metrics for a database
func (h *DatabaseHandler) GetMetrics(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	dbID, err := strconv.ParseUint(c.Param("dbId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid database ID",
			Code:    "INVALID_DATABASE_ID",
		})
		return
	}

	var managedDB database.ManagedDatabase
	if err := h.DB.Where("id = ? AND project_id = ? AND user_id = ?", dbID, projectID, userID).First(&managedDB).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Database not found",
			Code:    "NOT_FOUND",
		})
		return
	}

	// Decrypt password
	decryptedPassword, err := h.SecretsManager.Decrypt(userID, managedDB.Password, managedDB.Salt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to decrypt credentials",
			Code:    "DECRYPTION_FAILED",
		})
		return
	}

	metrics, err := h.Manager.GetMetrics(&managedDB, decryptedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to get metrics: " + err.Error(),
			Code:    "METRICS_FAILED",
		})
		return
	}

	// Add plan limits
	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: gin.H{
			"metrics": metrics,
			"limits": gin.H{
				"max_storage_mb":   managedDB.MaxStorageMB,
				"max_connections":  managedDB.MaxConnections,
			},
		},
	})
}

// sanitizeDatabase removes sensitive data from database response
func (h *DatabaseHandler) sanitizeDatabase(db *database.ManagedDatabase) gin.H {
	return gin.H{
		"id":               db.ID,
		"project_id":       db.ProjectID,
		"type":             db.Type,
		"name":             db.Name,
		"host":             db.Host,
		"port":             db.Port,
		"database_name":    db.DatabaseName,
		"status":           db.Status,
		"storage_used_mb":  db.StorageUsedMB,
		"connection_count": db.ConnectionCount,
		"query_count":      db.QueryCount,
		"last_queried":     db.LastQueried,
		"backup_enabled":   db.BackupEnabled,
		"backup_schedule":  db.BackupSchedule,
		"last_backup":      db.LastBackup,
		"next_backup":      db.NextBackup,
		"max_storage_mb":   db.MaxStorageMB,
		"max_connections":  db.MaxConnections,
		"created_at":       db.CreatedAt,
		"updated_at":       db.UpdatedAt,
	}
}

// RegisterDatabaseRoutes registers all database routes
func (h *DatabaseHandler) RegisterDatabaseRoutes(rg *gin.RouterGroup) {
	// All routes are under /projects/:id/databases
	databases := rg.Group("/projects/:id/databases")
	{
		databases.POST("", h.CreateDatabase)
		databases.GET("", h.ListDatabases)
		databases.GET("/:dbId", h.GetDatabase)
		databases.DELETE("/:dbId", h.DeleteDatabase)
		databases.POST("/:dbId/reset", h.ResetCredentials)
		databases.POST("/:dbId/query", h.ExecuteQuery)
		databases.GET("/:dbId/tables", h.GetTables)
		databases.GET("/:dbId/tables/:table/schema", h.GetTableSchema)
		databases.GET("/:dbId/metrics", h.GetMetrics)
	}
}
