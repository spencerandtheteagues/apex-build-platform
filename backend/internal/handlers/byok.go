package handlers

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"apex-build/internal/ai"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

// BYOKHandlers handles Bring Your Own Key API endpoints
type BYOKHandlers struct {
	byokManager *ai.BYOKManager
}

// NewBYOKHandlers creates new BYOK handlers
func NewBYOKHandlers(bm *ai.BYOKManager) *BYOKHandlers {
	return &BYOKHandlers{byokManager: bm}
}

// SaveKey stores an encrypted API key for a provider
func (h *BYOKHandlers) SaveKey(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req struct {
		Provider        string `json:"provider" binding:"required"`
		APIKey          string `json:"api_key" binding:"required"`
		ModelPreference string `json:"model_preference"`
		ProjectID       *uint  `json:"project_id"` // optional: scope key to a specific project
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: provider and api_key are required"})
		return
	}

	// Validate provider
	validProviders := map[string]bool{"claude": true, "gpt4": true, "gemini": true, "grok": true, "ollama": true}
	if !validProviders[req.Provider] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider. Must be one of: claude, gpt4, gemini, grok, ollama"})
		return
	}

	if err := h.byokManager.SaveKeyForProject(userID, req.Provider, req.APIKey, req.ModelPreference, req.ProjectID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save API key: " + err.Error()})
		return
	}

	resp := gin.H{
		"success":  true,
		"message":  "API key saved successfully",
		"provider": req.Provider,
	}
	if req.ProjectID != nil {
		resp["project_id"] = *req.ProjectID
	}
	c.JSON(http.StatusOK, resp)
}

// GetKeys lists configured providers for a user (no raw keys exposed)
func (h *BYOKHandlers) GetKeys(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Optionally filter by project_id query param
	var keys []models.UserAPIKey
	var err error
	if pidStr := c.Query("project_id"); pidStr != "" {
		pid, parseErr := strconv.ParseUint(pidStr, 10, 32)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project_id"})
			return
		}
		keys, err = h.byokManager.GetKeysForProject(userID, uint(pid))
	} else {
		keys, err = h.byokManager.GetKeys(userID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve keys"})
		return
	}

	// Return safe metadata only (no raw keys, encrypted values, salts, or fingerprints)
	type KeyInfo struct {
		Provider        string     `json:"provider"`
		ProjectID       *uint      `json:"project_id,omitempty"`
		ModelPreference string     `json:"model_preference"`
		IsActive        bool       `json:"is_active"`
		IsValid         bool       `json:"is_valid"`
		LastUsed        *time.Time `json:"last_used,omitempty"`
		UsageCount      int64      `json:"usage_count"`
		TotalCost       float64    `json:"total_cost"`
		RotationAgeDays *int       `json:"rotation_age_days,omitempty"`
	}

	keyInfos := make([]KeyInfo, len(keys))
	for i, k := range keys {
		info := KeyInfo{
			Provider:        k.Provider,
			ProjectID:       k.ProjectID,
			ModelPreference: k.ModelPreference,
			IsActive:        k.IsActive,
			IsValid:         k.IsValid,
			LastUsed:        k.LastUsed,
			UsageCount:      k.UsageCount,
			TotalCost:       k.TotalCost,
		}
		// Compute rotation_age_days from last_rotated_at
		if k.LastRotatedAt != nil {
			ageDays := int(math.Floor(time.Since(*k.LastRotatedAt).Hours() / 24))
			info.RotationAgeDays = &ageDays
		}
		keyInfos[i] = info
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    keyInfos,
	})
}

// DeleteKey removes a user's API key for a provider
func (h *BYOKHandlers) DeleteKey(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider parameter required"})
		return
	}

	if err := h.byokManager.DeleteKey(userID, provider); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "API key removed",
		"provider": provider,
	})
}

// ValidateKey tests if a stored key is valid
func (h *BYOKHandlers) ValidateKey(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider parameter required"})
		return
	}

	valid, err := h.byokManager.ValidateKey(c.Request.Context(), userID, provider)

	result := gin.H{
		"success":  true,
		"provider": provider,
		"valid":    valid,
	}
	if err != nil {
		result["error_detail"] = err.Error()
	}

	c.JSON(http.StatusOK, result)
}

// UpdateKeySettings updates is_active and/or model_preference for a provider
func (h *BYOKHandlers) UpdateKeySettings(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider parameter required"})
		return
	}

	var req struct {
		IsActive        *bool   `json:"is_active"`
		ModelPreference *string `json:"model_preference"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.byokManager.UpdateKeySettings(userID, provider, req.IsActive, req.ModelPreference); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Settings updated",
		"provider": provider,
	})
}

// GetUsage returns usage summary for the current user
func (h *BYOKHandlers) GetUsage(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	monthKey := c.DefaultQuery("month", time.Now().Format("2006-01"))

	summary, err := h.byokManager.GetUsageSummary(userID, monthKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get usage data"})
		return
	}

	var user models.User
	if err := h.byokManager.DB().Select("credit_balance", "has_unlimited_credits", "bypass_billing").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get billing data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summary,
		"month":   monthKey,
		"billing": gin.H{
			"credit_balance":        user.CreditBalance,
			"has_unlimited_credits": user.HasUnlimitedCredits,
			"bypass_billing":        user.BypassBilling,
		},
	})
}

// GetModels returns available models per provider
func (h *BYOKHandlers) GetModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    ai.GetAvailableModels(),
	})
}
