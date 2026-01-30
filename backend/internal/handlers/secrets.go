// Package handlers - Secrets Manager HTTP Handlers
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"apex-build/internal/secrets"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SecretsHandler handles secrets management endpoints
type SecretsHandler struct {
	db      *gorm.DB
	manager *secrets.SecretsManager
}

// NewSecretsHandler creates a new secrets handler
func NewSecretsHandler(db *gorm.DB, manager *secrets.SecretsManager) *SecretsHandler {
	return &SecretsHandler{
		db:      db,
		manager: manager,
	}
}

// CreateSecretRequest is the request body for creating a secret
type CreateSecretRequest struct {
	Name        string             `json:"name" binding:"required"`
	Value       string             `json:"value" binding:"required"`
	Description string             `json:"description,omitempty"`
	Type        secrets.SecretType `json:"type" binding:"required"`
	ProjectID   *uint              `json:"project_id,omitempty"`
}

// UpdateSecretRequest is the request body for updating a secret
type UpdateSecretRequest struct {
	Name        *string `json:"name,omitempty"`
	Value       *string `json:"value,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ListSecrets returns all secrets for the authenticated user (metadata only, no values)
func (h *SecretsHandler) ListSecrets(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Query("project_id")

	var secretsList []secrets.Secret
	query := h.db.Where("user_id = ?", userID)

	if projectIDStr != "" {
		projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project_id"})
			return
		}
		query = query.Where("project_id = ?", uint(projectID))
	}

	if err := query.Order("created_at DESC").Find(&secretsList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch secrets"})
		return
	}

	// Convert to metadata (no values exposed)
	metadata := make([]secrets.SecretMetadata, len(secretsList))
	for i, s := range secretsList {
		metadata[i] = s.ToMetadata()
	}

	c.JSON(http.StatusOK, gin.H{
		"secrets": metadata,
		"count":   len(metadata),
	})
}

// CreateSecret creates a new encrypted secret
func (h *SecretsHandler) CreateSecret(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req CreateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check for duplicate name
	var existing secrets.Secret
	query := h.db.Where("user_id = ? AND name = ?", userID, req.Name)
	if req.ProjectID != nil {
		query = query.Where("project_id = ?", *req.ProjectID)
	} else {
		query = query.Where("project_id IS NULL")
	}
	if query.First(&existing).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Secret with this name already exists"})
		return
	}

	// Verify project ownership if projectID provided
	if req.ProjectID != nil {
		var project models.Project
		if err := h.db.Where("id = ? AND owner_id = ?", *req.ProjectID, userID).First(&project).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Project not found or access denied"})
			return
		}
	}

	// Encrypt the secret value
	encryptedValue, salt, fingerprint, err := h.manager.Encrypt(userID, req.Value)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt secret"})
		return
	}

	secret := &secrets.Secret{
		UserID:         userID,
		ProjectID:      req.ProjectID,
		Name:           req.Name,
		Description:    req.Description,
		Type:           req.Type,
		EncryptedValue: encryptedValue,
		Salt:           salt,
		KeyFingerprint: fingerprint,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.db.Create(secret).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save secret"})
		return
	}

	// Log the creation
	h.logAccess(secret.ID, userID, "create", c.ClientIP(), c.GetHeader("User-Agent"), true, "")

	c.JSON(http.StatusCreated, gin.H{
		"message": "Secret created successfully",
		"secret":  secret.ToMetadata(),
	})
}

// GetSecret retrieves and decrypts a secret value
func (h *SecretsHandler) GetSecret(c *gin.Context) {
	userID := c.GetUint("user_id")
	secretID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid secret ID"})
		return
	}

	var secret secrets.Secret
	if err := h.db.Where("id = ? AND owner_id = ?", secretID, userID).First(&secret).Error; err != nil {
		h.logAccess(uint(secretID), userID, "read", c.ClientIP(), c.GetHeader("User-Agent"), false, "Not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Secret not found"})
		return
	}

	// Decrypt the value
	value, err := h.manager.Decrypt(userID, secret.EncryptedValue, secret.Salt)
	if err != nil {
		h.logAccess(secret.ID, userID, "read", c.ClientIP(), c.GetHeader("User-Agent"), false, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decrypt secret"})
		return
	}

	// Update last accessed
	now := time.Now()
	h.db.Model(&secret).Update("last_accessed", now)

	// Log the access
	h.logAccess(secret.ID, userID, "read", c.ClientIP(), c.GetHeader("User-Agent"), true, "")

	c.JSON(http.StatusOK, gin.H{
		"secret": secret.ToMetadata(),
		"value":  value,
	})
}

// UpdateSecret updates an existing secret
func (h *SecretsHandler) UpdateSecret(c *gin.Context) {
	userID := c.GetUint("user_id")
	secretID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid secret ID"})
		return
	}

	var req UpdateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var secret secrets.Secret
	if err := h.db.Where("id = ? AND owner_id = ?", secretID, userID).First(&secret).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Secret not found"})
		return
	}

	// Update fields
	if req.Name != nil {
		secret.Name = *req.Name
	}
	if req.Description != nil {
		secret.Description = *req.Description
	}
	if req.Value != nil {
		// Re-encrypt with new value
		encryptedValue, salt, fingerprint, err := h.manager.Encrypt(userID, *req.Value)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt secret"})
			return
		}
		secret.EncryptedValue = encryptedValue
		secret.Salt = salt
		secret.KeyFingerprint = fingerprint
	}
	secret.UpdatedAt = time.Now()

	if err := h.db.Save(&secret).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update secret"})
		return
	}

	h.logAccess(secret.ID, userID, "update", c.ClientIP(), c.GetHeader("User-Agent"), true, "")

	c.JSON(http.StatusOK, gin.H{
		"message": "Secret updated successfully",
		"secret":  secret.ToMetadata(),
	})
}

// DeleteSecret removes a secret
func (h *SecretsHandler) DeleteSecret(c *gin.Context) {
	userID := c.GetUint("user_id")
	secretID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid secret ID"})
		return
	}

	var secret secrets.Secret
	if err := h.db.Where("id = ? AND owner_id = ?", secretID, userID).First(&secret).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Secret not found"})
		return
	}

	if err := h.db.Delete(&secret).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete secret"})
		return
	}

	h.logAccess(uint(secretID), userID, "delete", c.ClientIP(), c.GetHeader("User-Agent"), true, "")

	c.JSON(http.StatusOK, gin.H{"message": "Secret deleted successfully"})
}

// RotateSecret generates a new encryption for an existing secret
func (h *SecretsHandler) RotateSecret(c *gin.Context) {
	userID := c.GetUint("user_id")
	secretID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid secret ID"})
		return
	}

	var secret secrets.Secret
	if err := h.db.Where("id = ? AND owner_id = ?", secretID, userID).First(&secret).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Secret not found"})
		return
	}

	// Decrypt with old key
	value, err := h.manager.Decrypt(userID, secret.EncryptedValue, secret.Salt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decrypt secret for rotation"})
		return
	}

	// Re-encrypt with new salt
	encryptedValue, salt, fingerprint, err := h.manager.Encrypt(userID, value)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to re-encrypt secret"})
		return
	}

	secret.EncryptedValue = encryptedValue
	secret.Salt = salt
	secret.KeyFingerprint = fingerprint
	secret.UpdatedAt = time.Now()

	if err := h.db.Save(&secret).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save rotated secret"})
		return
	}

	h.logAccess(secret.ID, userID, "rotate", c.ClientIP(), c.GetHeader("User-Agent"), true, "")

	c.JSON(http.StatusOK, gin.H{
		"message": "Secret rotated successfully",
		"secret":  secret.ToMetadata(),
	})
}

// GetProjectSecrets gets all secrets for a specific project as environment variables
func (h *SecretsHandler) GetProjectSecrets(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Verify project ownership
	var project models.Project
	if err := h.db.Where("id = ? AND owner_id = ?", projectID, userID).First(&project).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Project not found or access denied"})
		return
	}

	var secretsList []secrets.Secret
	if err := h.db.Where("user_id = ? AND (project_id = ? OR project_id IS NULL)", userID, projectID).
		Where("type = ?", secrets.SecretTypeEnvironment).
		Find(&secretsList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch secrets"})
		return
	}

	// Decrypt all and build env map
	envVars := make(map[string]string)
	for _, secret := range secretsList {
		value, err := h.manager.Decrypt(userID, secret.EncryptedValue, secret.Salt)
		if err != nil {
			continue // Skip failed decryptions
		}
		envVars[secret.Name] = value

		// Update last accessed
		now := time.Now()
		h.db.Model(&secret).Update("last_accessed", now)
	}

	c.JSON(http.StatusOK, gin.H{
		"environment_variables": envVars,
		"count":                 len(envVars),
	})
}

// GetAuditLog retrieves the access audit log for a secret
func (h *SecretsHandler) GetAuditLog(c *gin.Context) {
	userID := c.GetUint("user_id")
	secretID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid secret ID"})
		return
	}

	// Verify ownership
	var secret secrets.Secret
	if err := h.db.Where("id = ? AND owner_id = ?", secretID, userID).First(&secret).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Secret not found"})
		return
	}

	var logs []secrets.SecretAuditLog
	if err := h.db.Where("secret_id = ?", secretID).Order("created_at DESC").Limit(100).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audit log"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"audit_log": logs,
		"count":     len(logs),
	})
}

// logAccess records an access attempt in the audit log
func (h *SecretsHandler) logAccess(secretID, userID uint, action, ipAddress, userAgent string, success bool, errorMsg string) {
	log := &secrets.SecretAuditLog{
		SecretID:  secretID,
		UserID:    userID,
		Action:    action,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   success,
		ErrorMsg:  errorMsg,
		CreatedAt: time.Now(),
	}
	h.db.Create(log)
}
