package handlers

import (
	"net/http"
	"time"

	"apex-build/internal/ai"

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

	if err := h.byokManager.SaveKey(userID, req.Provider, req.APIKey, req.ModelPreference); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "API key saved successfully",
		"provider": req.Provider,
	})
}

// GetKeys lists configured providers for a user (no raw keys exposed)
func (h *BYOKHandlers) GetKeys(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	keys, err := h.byokManager.GetKeys(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve keys"})
		return
	}

	// Return safe metadata only (no raw keys, encrypted values, salts, or fingerprints)
	type KeyInfo struct {
		Provider        string     `json:"provider"`
		ModelPreference string     `json:"model_preference"`
		IsActive        bool       `json:"is_active"`
		IsValid         bool       `json:"is_valid"`
		LastUsed        *time.Time `json:"last_used,omitempty"`
		UsageCount      int64      `json:"usage_count"`
		TotalCost       float64    `json:"total_cost"`
	}

	keyInfos := make([]KeyInfo, len(keys))
	for i, k := range keys {
		keyInfos[i] = KeyInfo{
			Provider:        k.Provider,
			ModelPreference: k.ModelPreference,
			IsActive:        k.IsActive,
			IsValid:         k.IsValid,
			LastUsed:        k.LastUsed,
			UsageCount:      k.UsageCount,
			TotalCost:       k.TotalCost,
		}
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

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summary,
		"month":   monthKey,
	})
}

// GetModels returns available models per provider
func (h *BYOKHandlers) GetModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    ai.GetAvailableModels(),
	})
}
