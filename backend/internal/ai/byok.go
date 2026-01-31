package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"apex-build/internal/secrets"
	"apex-build/pkg/models"

	"gorm.io/gorm"
)

// BYOKManager manages user-provided API keys for Bring Your Own Key
type BYOKManager struct {
	db             *gorm.DB
	secretsManager *secrets.SecretsManager
	platformRouter *AIRouter // Fallback to platform keys
	mu             sync.RWMutex
}

// NewBYOKManager creates a new BYOK manager
func NewBYOKManager(db *gorm.DB, sm *secrets.SecretsManager, platformRouter *AIRouter) *BYOKManager {
	// Auto-migrate BYOK tables
	db.AutoMigrate(&models.UserAPIKey{}, &models.AIUsageLog{})

	return &BYOKManager{
		db:             db,
		secretsManager: sm,
		platformRouter: platformRouter,
	}
}

// SaveKey encrypts and stores a user's API key for a provider
func (m *BYOKManager) SaveKey(userID uint, provider, apiKey, modelPref string) error {
	// Encrypt the key
	encrypted, salt, fingerprint, err := m.secretsManager.Encrypt(userID, apiKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt API key: %w", err)
	}

	// Upsert: update existing or create new
	var existing models.UserAPIKey
	result := m.db.Where("user_id = ? AND provider = ?", userID, provider).First(&existing)

	if result.Error == nil {
		// Update existing
		return m.db.Model(&existing).Updates(models.UserAPIKey{
			EncryptedKey:    encrypted,
			KeySalt:         salt,
			KeyFingerprint:  fingerprint,
			ModelPreference: modelPref,
			IsActive:        true,
			IsValid:         false, // Re-validate after update
		}).Error
	}

	// Create new
	return m.db.Create(&models.UserAPIKey{
		UserID:          userID,
		Provider:        provider,
		EncryptedKey:    encrypted,
		KeySalt:         salt,
		KeyFingerprint:  fingerprint,
		ModelPreference: modelPref,
		IsActive:        true,
	}).Error
}

// GetKeys returns metadata about all configured keys for a user (no raw keys)
func (m *BYOKManager) GetKeys(userID uint) ([]models.UserAPIKey, error) {
	var keys []models.UserAPIKey
	err := m.db.Where("user_id = ? AND deleted_at IS NULL", userID).Find(&keys).Error
	return keys, err
}

// DeleteKey removes a user's API key for a provider
func (m *BYOKManager) DeleteKey(userID uint, provider string) error {
	return m.db.Where("user_id = ? AND provider = ?", userID, provider).Delete(&models.UserAPIKey{}).Error
}

// UpdateKeySettings updates is_active and/or model_preference for a provider
func (m *BYOKManager) UpdateKeySettings(userID uint, provider string, isActive *bool, modelPref *string) error {
	updates := make(map[string]interface{})
	if isActive != nil {
		updates["is_active"] = *isActive
	}
	if modelPref != nil {
		updates["model_preference"] = *modelPref
	}
	if len(updates) == 0 {
		return nil
	}
	result := m.db.Model(&models.UserAPIKey{}).
		Where("user_id = ? AND provider = ? AND deleted_at IS NULL", userID, provider).
		Updates(updates)
	if result.RowsAffected == 0 {
		return fmt.Errorf("no key found for provider %s", provider)
	}
	return result.Error
}

// ValidateKey tests if a stored key is valid by making a minimal API call
func (m *BYOKManager) ValidateKey(ctx context.Context, userID uint, provider string) (bool, error) {
	apiKey, err := m.decryptKey(userID, provider)
	if err != nil {
		return false, err
	}

	// Create a temporary client and test it
	var client AIClient
	switch AIProvider(provider) {
	case ProviderClaude:
		client = NewClaudeClient(apiKey)
	case ProviderGPT4:
		client = NewOpenAIClient(apiKey)
	case ProviderGemini:
		client = NewGeminiClient(apiKey)
	case ProviderGrok:
		client = NewGrokClient(apiKey)
	case ProviderOllama:
		// For Ollama, the "apiKey" is actually the base URL
		if !strings.HasPrefix(apiKey, "http://") && !strings.HasPrefix(apiKey, "https://") {
			return false, fmt.Errorf("invalid Ollama URL: must start with http:// or https://")
		}
		client = NewOllamaClient(apiKey)
	default:
		return false, fmt.Errorf("unsupported provider: %s", provider)
	}

	err = client.Health(ctx)
	valid := err == nil

	// Update validity status in DB
	m.db.Model(&models.UserAPIKey{}).
		Where("user_id = ? AND provider = ?", userID, provider).
		Update("is_valid", valid)

	return valid, err
}

// GetRouterForUser creates an AI router that uses the user's keys where available,
// falling back to platform keys for unconfigured providers
func (m *BYOKManager) GetRouterForUser(userID uint) (*AIRouter, bool, error) {
	var keys []models.UserAPIKey
	if err := m.db.Where("user_id = ? AND is_active = ? AND deleted_at IS NULL", userID, true).Find(&keys).Error; err != nil {
		return m.platformRouter, false, nil
	}

	if len(keys) == 0 {
		return m.platformRouter, false, nil
	}

	// Build a custom router with user's keys
	clients := make(map[AIProvider]AIClient)
	hasBYOK := false

	for _, key := range keys {
		apiKey, err := m.decryptUserKey(key)
		if err != nil {
			log.Printf("BYOK: Failed to decrypt key for user %d provider %s: %v", userID, key.Provider, err)
			continue
		}

		switch AIProvider(key.Provider) {
		case ProviderClaude:
			clients[ProviderClaude] = NewClaudeClient(apiKey)
			hasBYOK = true
		case ProviderGPT4:
			clients[ProviderGPT4] = NewOpenAIClient(apiKey)
			hasBYOK = true
		case ProviderGemini:
			clients[ProviderGemini] = NewGeminiClient(apiKey)
			hasBYOK = true
		case ProviderGrok:
			clients[ProviderGrok] = NewGrokClient(apiKey)
			hasBYOK = true
		case ProviderOllama:
			// For Ollama, the "apiKey" is actually the base URL
			clients[ProviderOllama] = NewOllamaClient(apiKey)
			hasBYOK = true
		}
	}

	if !hasBYOK {
		return m.platformRouter, false, nil
	}

	// Fill in missing providers from platform router
	m.mu.RLock()
	for provider, client := range m.platformRouter.clients {
		if _, exists := clients[provider]; !exists {
			// STRICT POLICY:
			// 1. If user has ANY BYOK key enabled, disable all paid platform models.
			// 2. NEVER allow a global/shared Ollama instance to leak to users.
			//    Ollama must ALWAYS be provided by the user (BYOK).

			if provider == ProviderOllama {
				continue // Explicitly block global Ollama
			}

			if hasBYOK {
				// If user is bringing keys, we disable platform's paid models to save costs.
				if provider == ProviderClaude || provider == ProviderGPT4 ||
					provider == ProviderGemini || provider == ProviderGrok {
					continue
				}
			}
			clients[provider] = client
		}
	}
	m.mu.RUnlock()

	config := DefaultRouterConfig()
	rateLimits := make(map[AIProvider]*rateLimiter)
	for provider, limit := range config.RateLimits {
		rateLimits[provider] = &rateLimiter{
			tokens:     limit,
			maxTokens:  limit,
			lastRefill: time.Now(),
		}
	}

	router := &AIRouter{
		clients:     clients,
		config:      config,
		rateLimits:  rateLimits,
		healthCheck: make(map[AIProvider]bool),
	}

	return router, true, nil
}

// RecordUsage logs an AI API call for cost tracking
func (m *BYOKManager) RecordUsage(userID uint, projectID *uint, provider, model string, isBYOK bool,
	inputTokens, outputTokens int, cost float64, capability string, duration time.Duration, status string) {

	now := time.Now()
	monthKey := now.Format("2006-01")

	log := models.AIUsageLog{
		CreatedAt:    now,
		UserID:       userID,
		ProjectID:    projectID,
		Provider:     provider,
		Model:        model,
		IsBYOK:       isBYOK,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		Cost:         cost,
		Capability:   capability,
		Duration:     duration,
		Status:       status,
		MonthKey:     monthKey,
	}

	if err := m.db.Create(&log).Error; err != nil {
		// Don't fail the request if logging fails
		fmt.Printf("BYOK: Failed to record usage: %v\n", err)
	}

	// Update user API key usage stats if BYOK
	if isBYOK {
		m.db.Model(&models.UserAPIKey{}).
			Where("user_id = ? AND provider = ?", userID, provider).
			Updates(map[string]interface{}{
				"usage_count": gorm.Expr("usage_count + 1"),
				"total_cost":  gorm.Expr("total_cost + ?", cost),
				"last_used":   now,
			})
	}
}

// GetUsageSummary returns usage totals for a user within a date range
func (m *BYOKManager) GetUsageSummary(userID uint, monthKey string) (*UsageSummary, error) {
	summary := &UsageSummary{
		ByProvider: make(map[string]*ProviderUsageSummary),
	}

	var logs []models.AIUsageLog
	query := m.db.Where("user_id = ?", userID)
	if monthKey != "" {
		query = query.Where("month_key = ?", monthKey)
	}
	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}

	for _, l := range logs {
		summary.TotalCost += l.Cost
		summary.TotalTokens += l.TotalTokens
		summary.TotalRequests++

		ps, exists := summary.ByProvider[l.Provider]
		if !exists {
			ps = &ProviderUsageSummary{Provider: l.Provider}
			summary.ByProvider[l.Provider] = ps
		}
		ps.Cost += l.Cost
		ps.Tokens += l.TotalTokens
		ps.Requests++
		if l.IsBYOK {
			ps.BYOKRequests++
		}
	}

	return summary, nil
}

// UsageSummary represents aggregated usage data
type UsageSummary struct {
	TotalCost     float64                        `json:"total_cost"`
	TotalTokens   int                            `json:"total_tokens"`
	TotalRequests int                            `json:"total_requests"`
	ByProvider    map[string]*ProviderUsageSummary `json:"by_provider"`
}

// ProviderUsageSummary represents per-provider usage
type ProviderUsageSummary struct {
	Provider     string  `json:"provider"`
	Cost         float64 `json:"cost"`
	Tokens       int     `json:"tokens"`
	Requests     int     `json:"requests"`
	BYOKRequests int     `json:"byok_requests"`
}

// decryptKey decrypts a stored API key
func (m *BYOKManager) decryptKey(userID uint, provider string) (string, error) {
	var key models.UserAPIKey
	if err := m.db.Where("user_id = ? AND provider = ? AND is_active = ?", userID, provider, true).First(&key).Error; err != nil {
		return "", fmt.Errorf("key not found for provider %s: %w", provider, err)
	}
	return m.decryptUserKey(key)
}

// decryptUserKey decrypts a UserAPIKey record
func (m *BYOKManager) decryptUserKey(key models.UserAPIKey) (string, error) {
	return m.secretsManager.Decrypt(key.UserID, key.EncryptedKey, key.KeySalt)
}

// GetAvailableModels returns available models per provider (January 2026)
func GetAvailableModels() map[string][]ModelInfo {
	return map[string][]ModelInfo{
		"claude": {
			{ID: "claude-opus-4-5-20251101", Name: "Claude Opus 4.5", Speed: "slow", CostTier: "high", Description: "Flagship reasoning model"},
			{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Speed: "medium", CostTier: "medium", Description: "Balanced performance"},
			{ID: "claude-haiku-3-5-20241022", Name: "Claude Haiku 3.5", Speed: "fast", CostTier: "low", Description: "Fast and affordable"},
		},
		"gpt4": {
			{ID: "gpt-5.2-pro", Name: "GPT-5.2 Pro", Speed: "slow", CostTier: "high", Description: "Most intelligent, 100% AIME"},
			{ID: "gpt-5.2-thinking", Name: "GPT-5.2 Thinking", Speed: "medium", CostTier: "medium", Description: "Deep reasoning"},
			{ID: "gpt-5.2-instant", Name: "GPT-5.2 Instant", Speed: "fast", CostTier: "low", Description: "Fast everyday tasks"},
			{ID: "gpt-5.2-codex", Name: "GPT-5.2 Codex", Speed: "medium", CostTier: "high", Description: "Agentic coding specialist"},
		},
		"gemini": {
			{ID: "gemini-3-pro", Name: "Gemini 3 Pro", Speed: "medium", CostTier: "medium", Description: "Advanced reasoning"},
			{ID: "gemini-3-deep-think", Name: "Gemini 3 Deep Think", Speed: "slow", CostTier: "high", Description: "Extended thinking mode"},
			{ID: "gemini-3-flash", Name: "Gemini 3 Flash", Speed: "fast", CostTier: "low", Description: "Pro-grade at Flash speed"},
		},
		"grok": {
			{ID: "grok-4-heavy", Name: "Grok 4 Heavy", Speed: "slow", CostTier: "high", Description: "50% HLE, parallel compute"},
			{ID: "grok-4.1-thinking", Name: "Grok 4.1 Thinking", Speed: "medium", CostTier: "medium", Description: "#1 LMArena overall"},
			{ID: "grok-4.1", Name: "Grok 4.1", Speed: "fast", CostTier: "medium", Description: "#2 non-reasoning"},
			{ID: "grok-4-fast", Name: "Grok 4 Fast", Speed: "fast", CostTier: "low", Description: "Budget-friendly option"},
		},
		"ollama": {
			{ID: "deepseek-r1:8b", Name: "DeepSeek-R1 (8b)", Speed: "variable", CostTier: "free", Description: "Reasoning model (local)"},
			{ID: "qwen3-coder:30b", Name: "Qwen 3 Coder (30b)", Speed: "variable", CostTier: "free", Description: "Advanced code model (local)"},
			{ID: "deepseek-v3.2", Name: "DeepSeek-V3.2", Speed: "variable", CostTier: "free", Description: "Efficient long-context (local)"},
			{ID: "llama3.3-70b", Name: "Llama 3.3 70B", Speed: "variable", CostTier: "free", Description: "405B performance (local)"},
		},
	}
}

// ModelInfo describes an available model
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Speed       string `json:"speed"`        // slow, medium, fast
	CostTier    string `json:"cost_tier"`     // low, medium, high
	Description string `json:"description"`
}
