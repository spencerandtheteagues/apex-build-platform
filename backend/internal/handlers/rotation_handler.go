package handlers

import (
	"net/http"

	"apex-build/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RotationHandler handles admin secret rotation endpoints
type RotationHandler struct {
	db *gorm.DB
}

// NewRotationHandler creates a new rotation handler
func NewRotationHandler(db *gorm.DB) *RotationHandler {
	return &RotationHandler{db: db}
}

// RotateSecretsRequest is the request body for key rotation
type RotateSecretsRequest struct {
	OldMasterKey string `json:"old_master_key" binding:"required"`
	NewMasterKey string `json:"new_master_key" binding:"required"`
}

// RotateSecrets re-encrypts all user secrets with a new master key.
// POST /api/v1/admin/rotate-secrets
// Requires: super admin
func (h *RotationHandler) RotateSecrets(c *gin.Context) {
	var req RotateSecretsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "old_master_key and new_master_key are required"})
		return
	}

	result, err := config.RotateMasterKey(h.db, req.OldMasterKey, req.NewMasterKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  err.Error(),
			"result": result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Key rotation completed successfully",
		"result":  result,
	})
}

// ValidateSecrets checks that all secrets can be decrypted with the current key.
// GET /api/v1/admin/validate-secrets?key=<base64>
// Requires: super admin
func (h *RotationHandler) ValidateSecrets(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key query parameter required"})
		return
	}

	ok, fail, err := config.ValidateRotation(h.db, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":       ok + fail,
		"decryptable": ok,
		"failed":      fail,
		"healthy":     fail == 0,
	})
}
