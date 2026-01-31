// APEX.BUILD Usage Tracking System
// Production-ready usage tracking with database persistence and Redis caching
// Tracks: Projects, Storage, AI Requests, Execution Minutes

package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"apex-build/internal/cache"

	"gorm.io/gorm"
)

// UsageType represents different usage metrics
type UsageType string

const (
	UsageProjects         UsageType = "projects"
	UsageStorageBytes     UsageType = "storage_bytes"
	UsageAIRequests       UsageType = "ai_requests"
	UsageExecutionMinutes UsageType = "execution_minutes"
)

// PlanType represents subscription tiers
type PlanType string

const (
	PlanFree       PlanType = "free"
	PlanPro        PlanType = "pro"
	PlanTeam       PlanType = "team"
	PlanEnterprise PlanType = "enterprise"
	PlanOwner      PlanType = "owner" // Platform owner - unlimited
)

// PlanLimits defines limits for each plan
type PlanLimits struct {
	Projects         int   `json:"projects"`          // Max number of projects
	StorageBytes     int64 `json:"storage_bytes"`     // Max storage in bytes
	AIRequests       int   `json:"ai_requests"`       // Max AI requests per month
	ExecutionMinutes int   `json:"execution_minutes"` // Max execution minutes per day
}

// GetPlanLimits returns the limits for a given plan
func GetPlanLimits(plan PlanType) PlanLimits {
	switch plan {
	case PlanFree:
		return PlanLimits{
			Projects:         3,
			StorageBytes:     100 * 1024 * 1024, // 100MB
			AIRequests:       1000,              // 1000/month
			ExecutionMinutes: 10,                // 10 min/day
		}
	case PlanPro:
		return PlanLimits{
			Projects:         25,
			StorageBytes:     5 * 1024 * 1024 * 1024, // 5GB
			AIRequests:       10000,                   // 10000/month
			ExecutionMinutes: 120,                     // 120 min/day (2 hours)
		}
	case PlanTeam:
		return PlanLimits{
			Projects:         100,
			StorageBytes:     25 * 1024 * 1024 * 1024, // 25GB
			AIRequests:       50000,                    // 50000/month
			ExecutionMinutes: 480,                      // 480 min/day (8 hours)
		}
	case PlanEnterprise, PlanOwner:
		return PlanLimits{
			Projects:         -1, // Unlimited
			StorageBytes:     -1, // Unlimited
			AIRequests:       -1, // Unlimited
			ExecutionMinutes: -1, // Unlimited
		}
	default:
		// Default to free limits for unknown plans
		return GetPlanLimits(PlanFree)
	}
}

// UsageRecord represents a single usage event stored in the database
type UsageRecord struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
	UserID    uint      `json:"user_id" gorm:"not null;index"`
	Type      UsageType `json:"type" gorm:"not null;index;size:50"`
	Amount    int64     `json:"amount" gorm:"not null"`     // Amount used (bytes, count, seconds, etc.)
	ProjectID *uint     `json:"project_id,omitempty" gorm:"index"`
	Metadata  string    `json:"metadata,omitempty" gorm:"type:text"` // JSON metadata
}

// DailyUsageSummary aggregates usage per day
type DailyUsageSummary struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Date      time.Time `json:"date" gorm:"uniqueIndex:idx_daily_user_type,priority:1;not null"`
	UserID    uint      `json:"user_id" gorm:"uniqueIndex:idx_daily_user_type,priority:2;not null"`
	Type      UsageType `json:"type" gorm:"uniqueIndex:idx_daily_user_type,priority:3;not null;size:50"`
	Total     int64     `json:"total" gorm:"not null;default:0"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MonthlyUsageSummary aggregates usage per month
type MonthlyUsageSummary struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Month     string    `json:"month" gorm:"uniqueIndex:idx_monthly_user_type,priority:1;not null;size:7"` // "2026-01"
	UserID    uint      `json:"user_id" gorm:"uniqueIndex:idx_monthly_user_type,priority:2;not null"`
	Type      UsageType `json:"type" gorm:"uniqueIndex:idx_monthly_user_type,priority:3;not null;size:50"`
	Total     int64     `json:"total" gorm:"not null;default:0"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CurrentUsage represents the user's current usage snapshot
type CurrentUsage struct {
	UserID           uint      `json:"user_id"`
	Plan             PlanType  `json:"plan"`
	Projects         int       `json:"projects"`
	ProjectsLimit    int       `json:"projects_limit"`
	StorageBytes     int64     `json:"storage_bytes"`
	StorageLimit     int64     `json:"storage_limit"`
	AIRequests       int       `json:"ai_requests"`       // This month
	AIRequestsLimit  int       `json:"ai_requests_limit"`
	ExecutionMinutes int       `json:"execution_minutes"` // Today
	ExecutionLimit   int       `json:"execution_limit"`
	PeriodStart      time.Time `json:"period_start"`
	PeriodEnd        time.Time `json:"period_end"`
	CachedAt         time.Time `json:"cached_at"`
}

// UsageHistory represents historical usage data
type UsageHistory struct {
	UserID uint                `json:"user_id"`
	Daily  []DailyUsagePoint   `json:"daily"`
	Monthly []MonthlyUsagePoint `json:"monthly"`
}

// DailyUsagePoint represents a single day's usage
type DailyUsagePoint struct {
	Date             string `json:"date"`
	AIRequests       int64  `json:"ai_requests"`
	ExecutionMinutes int64  `json:"execution_minutes"`
	StorageBytes     int64  `json:"storage_bytes"`
}

// MonthlyUsagePoint represents a month's usage
type MonthlyUsagePoint struct {
	Month            string `json:"month"`
	AIRequests       int64  `json:"ai_requests"`
	ExecutionMinutes int64  `json:"execution_minutes"`
	StorageBytes     int64  `json:"storage_bytes"`
}

// Tracker manages usage tracking with caching
type Tracker struct {
	db    *gorm.DB
	cache *cache.RedisCache
	mu    sync.RWMutex

	// Local cache for ultra-fast lookups (with TTL)
	localCache    map[uint]*cachedUsage
	localCacheTTL time.Duration
}

type cachedUsage struct {
	usage     *CurrentUsage
	expiresAt time.Time
}

// NewTracker creates a new usage tracker
func NewTracker(db *gorm.DB, redisCache *cache.RedisCache) *Tracker {
	tracker := &Tracker{
		db:            db,
		cache:         redisCache,
		localCache:    make(map[uint]*cachedUsage),
		localCacheTTL: 30 * time.Second, // Cache for 30 seconds locally
	}

	// Start background cleanup goroutine
	go tracker.cleanupLoop()

	return tracker
}

// Migrate runs database migrations for usage tables
func (t *Tracker) Migrate() error {
	return t.db.AutoMigrate(
		&UsageRecord{},
		&DailyUsageSummary{},
		&MonthlyUsageSummary{},
	)
}

// RecordUsage records a usage event
func (t *Tracker) RecordUsage(ctx context.Context, userID uint, usageType UsageType, amount int64, projectID *uint, metadata map[string]interface{}) error {
	// Create usage record
	record := &UsageRecord{
		UserID:    userID,
		Type:      usageType,
		Amount:    amount,
		ProjectID: projectID,
	}

	// Serialize metadata if provided
	if metadata != nil {
		metadataJSON, err := json.Marshal(metadata)
		if err == nil {
			record.Metadata = string(metadataJSON)
		}
	}

	// Save to database
	if err := t.db.Create(record).Error; err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}

	// Update daily summary
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if err := t.updateDailySummary(ctx, userID, usageType, amount, today); err != nil {
		log.Printf("Warning: failed to update daily summary: %v", err)
	}

	// Update monthly summary
	month := time.Now().UTC().Format("2006-01")
	if err := t.updateMonthlySummary(ctx, userID, usageType, amount, month); err != nil {
		log.Printf("Warning: failed to update monthly summary: %v", err)
	}

	// Invalidate cache
	t.invalidateCache(userID)

	return nil
}

// updateDailySummary updates or creates a daily summary
func (t *Tracker) updateDailySummary(ctx context.Context, userID uint, usageType UsageType, amount int64, date time.Time) error {
	return t.db.WithContext(ctx).Exec(`
		INSERT INTO daily_usage_summaries (date, user_id, type, total, updated_at)
		VALUES (?, ?, ?, ?, NOW())
		ON CONFLICT (date, user_id, type)
		DO UPDATE SET total = daily_usage_summaries.total + ?, updated_at = NOW()
	`, date, userID, usageType, amount, amount).Error
}

// updateMonthlySummary updates or creates a monthly summary
func (t *Tracker) updateMonthlySummary(ctx context.Context, userID uint, usageType UsageType, amount int64, month string) error {
	return t.db.WithContext(ctx).Exec(`
		INSERT INTO monthly_usage_summaries (month, user_id, type, total, updated_at)
		VALUES (?, ?, ?, ?, NOW())
		ON CONFLICT (month, user_id, type)
		DO UPDATE SET total = monthly_usage_summaries.total + ?, updated_at = NOW()
	`, month, userID, usageType, amount, amount).Error
}

// GetCurrentUsage retrieves current usage for a user with caching
func (t *Tracker) GetCurrentUsage(ctx context.Context, userID uint, plan PlanType) (*CurrentUsage, error) {
	// Check local cache first (fastest)
	t.mu.RLock()
	if cached, ok := t.localCache[userID]; ok && time.Now().Before(cached.expiresAt) {
		t.mu.RUnlock()
		return cached.usage, nil
	}
	t.mu.RUnlock()

	// Check Redis cache
	cacheKey := fmt.Sprintf("usage:current:%d", userID)
	if t.cache != nil {
		var usage CurrentUsage
		if err := t.cache.GetJSON(ctx, cacheKey, &usage); err == nil {
			// Store in local cache
			t.mu.Lock()
			t.localCache[userID] = &cachedUsage{
				usage:     &usage,
				expiresAt: time.Now().Add(t.localCacheTTL),
			}
			t.mu.Unlock()
			return &usage, nil
		}
	}

	// Calculate from database
	usage, err := t.calculateCurrentUsage(ctx, userID, plan)
	if err != nil {
		return nil, err
	}

	// Store in Redis cache (60 second TTL)
	if t.cache != nil {
		_ = t.cache.SetJSON(ctx, cacheKey, usage, 60*time.Second)
	}

	// Store in local cache
	t.mu.Lock()
	t.localCache[userID] = &cachedUsage{
		usage:     usage,
		expiresAt: time.Now().Add(t.localCacheTTL),
	}
	t.mu.Unlock()

	return usage, nil
}

// calculateCurrentUsage calculates usage from the database
func (t *Tracker) calculateCurrentUsage(ctx context.Context, userID uint, plan PlanType) (*CurrentUsage, error) {
	limits := GetPlanLimits(plan)

	usage := &CurrentUsage{
		UserID:           userID,
		Plan:             plan,
		ProjectsLimit:    limits.Projects,
		StorageLimit:     limits.StorageBytes,
		AIRequestsLimit:  limits.AIRequests,
		ExecutionLimit:   limits.ExecutionMinutes,
		PeriodStart:      time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, -time.Now().Day()+1), // First of month
		PeriodEnd:        time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 1, -time.Now().Day()),   // Last of month
		CachedAt:         time.Now().UTC(),
	}

	// Get project count
	var projectCount int64
	if err := t.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM projects
		WHERE owner_id = ? AND deleted_at IS NULL
	`, userID).Scan(&projectCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count projects: %w", err)
	}
	usage.Projects = int(projectCount)

	// Get storage usage (sum of all file sizes)
	var storageBytes int64
	if err := t.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(f.size), 0)
		FROM files f
		JOIN projects p ON f.project_id = p.id
		WHERE p.owner_id = ? AND p.deleted_at IS NULL AND f.deleted_at IS NULL
	`, userID).Scan(&storageBytes).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate storage: %w", err)
	}
	usage.StorageBytes = storageBytes

	// Get AI requests this month
	currentMonth := time.Now().UTC().Format("2006-01")
	var aiRequests int64
	if err := t.db.WithContext(ctx).Raw(`
		SELECT COALESCE(total, 0) FROM monthly_usage_summaries
		WHERE user_id = ? AND type = ? AND month = ?
	`, userID, UsageAIRequests, currentMonth).Scan(&aiRequests).Error; err != nil {
		// If no record exists, count from ai_requests table
		t.db.WithContext(ctx).Raw(`
			SELECT COUNT(*) FROM ai_requests
			WHERE user_id = ? AND created_at >= date_trunc('month', CURRENT_TIMESTAMP AT TIME ZONE 'UTC')
		`, userID).Scan(&aiRequests)
	}
	usage.AIRequests = int(aiRequests)

	// Get execution minutes today
	today := time.Now().UTC().Truncate(24 * time.Hour)
	var execMinutes int64
	if err := t.db.WithContext(ctx).Raw(`
		SELECT COALESCE(total, 0) FROM daily_usage_summaries
		WHERE user_id = ? AND type = ? AND date = ?
	`, userID, UsageExecutionMinutes, today).Scan(&execMinutes).Error; err != nil {
		// If no record exists, calculate from executions table
		t.db.WithContext(ctx).Raw(`
			SELECT COALESCE(SUM(duration / 60000), 0) FROM executions
			WHERE user_id = ? AND created_at >= ? AND created_at < ?
		`, userID, today, today.Add(24*time.Hour)).Scan(&execMinutes)
	}
	usage.ExecutionMinutes = int(execMinutes)

	return usage, nil
}

// CheckQuota checks if a user has exceeded their quota for a specific usage type
func (t *Tracker) CheckQuota(ctx context.Context, userID uint, plan PlanType, usageType UsageType, additionalAmount int64) (allowed bool, currentUsage int64, limit int64, err error) {
	// Get current usage
	usage, err := t.GetCurrentUsage(ctx, userID, plan)
	if err != nil {
		return false, 0, 0, err
	}

	limits := GetPlanLimits(plan)

	switch usageType {
	case UsageProjects:
		limit = int64(limits.Projects)
		currentUsage = int64(usage.Projects)
	case UsageStorageBytes:
		limit = limits.StorageBytes
		currentUsage = usage.StorageBytes
	case UsageAIRequests:
		limit = int64(limits.AIRequests)
		currentUsage = int64(usage.AIRequests)
	case UsageExecutionMinutes:
		limit = int64(limits.ExecutionMinutes)
		currentUsage = int64(usage.ExecutionMinutes)
	default:
		return true, 0, -1, nil // Unknown type, allow
	}

	// -1 means unlimited
	if limit == -1 {
		return true, currentUsage, -1, nil
	}

	// Check if adding the additional amount would exceed the limit
	allowed = (currentUsage + additionalAmount) <= limit
	return allowed, currentUsage, limit, nil
}

// GetUsageHistory retrieves historical usage data
func (t *Tracker) GetUsageHistory(ctx context.Context, userID uint, days int) (*UsageHistory, error) {
	history := &UsageHistory{
		UserID:  userID,
		Daily:   make([]DailyUsagePoint, 0),
		Monthly: make([]MonthlyUsagePoint, 0),
	}

	// Get daily usage for the last N days
	startDate := time.Now().UTC().AddDate(0, 0, -days).Truncate(24 * time.Hour)

	// Query daily summaries
	type dailyResult struct {
		Date  time.Time `gorm:"column:date"`
		Type  UsageType `gorm:"column:type"`
		Total int64     `gorm:"column:total"`
	}

	var dailyResults []dailyResult
	if err := t.db.WithContext(ctx).Raw(`
		SELECT date, type, total FROM daily_usage_summaries
		WHERE user_id = ? AND date >= ?
		ORDER BY date ASC
	`, userID, startDate).Scan(&dailyResults).Error; err != nil {
		return nil, fmt.Errorf("failed to get daily history: %w", err)
	}

	// Aggregate by date
	dailyMap := make(map[string]*DailyUsagePoint)
	for _, r := range dailyResults {
		dateStr := r.Date.Format("2006-01-02")
		if _, exists := dailyMap[dateStr]; !exists {
			dailyMap[dateStr] = &DailyUsagePoint{Date: dateStr}
		}
		switch r.Type {
		case UsageAIRequests:
			dailyMap[dateStr].AIRequests = r.Total
		case UsageExecutionMinutes:
			dailyMap[dateStr].ExecutionMinutes = r.Total
		case UsageStorageBytes:
			dailyMap[dateStr].StorageBytes = r.Total
		}
	}

	for _, point := range dailyMap {
		history.Daily = append(history.Daily, *point)
	}

	// Get monthly usage for the last 12 months
	var monthlyResults []struct {
		Month string    `gorm:"column:month"`
		Type  UsageType `gorm:"column:type"`
		Total int64     `gorm:"column:total"`
	}

	if err := t.db.WithContext(ctx).Raw(`
		SELECT month, type, total FROM monthly_usage_summaries
		WHERE user_id = ?
		ORDER BY month DESC
		LIMIT 36
	`, userID).Scan(&monthlyResults).Error; err != nil {
		return nil, fmt.Errorf("failed to get monthly history: %w", err)
	}

	// Aggregate by month
	monthlyMap := make(map[string]*MonthlyUsagePoint)
	for _, r := range monthlyResults {
		if _, exists := monthlyMap[r.Month]; !exists {
			monthlyMap[r.Month] = &MonthlyUsagePoint{Month: r.Month}
		}
		switch r.Type {
		case UsageAIRequests:
			monthlyMap[r.Month].AIRequests = r.Total
		case UsageExecutionMinutes:
			monthlyMap[r.Month].ExecutionMinutes = r.Total
		case UsageStorageBytes:
			monthlyMap[r.Month].StorageBytes = r.Total
		}
	}

	for _, point := range monthlyMap {
		history.Monthly = append(history.Monthly, *point)
	}

	return history, nil
}

// GetLimits returns the limits for a plan
func (t *Tracker) GetLimits(plan PlanType) PlanLimits {
	return GetPlanLimits(plan)
}

// invalidateCache invalidates all caches for a user
func (t *Tracker) invalidateCache(userID uint) {
	// Invalidate local cache
	t.mu.Lock()
	delete(t.localCache, userID)
	t.mu.Unlock()

	// Invalidate Redis cache
	if t.cache != nil {
		ctx := context.Background()
		cacheKey := fmt.Sprintf("usage:current:%d", userID)
		_ = t.cache.Delete(ctx, cacheKey)
	}
}

// cleanupLoop periodically cleans up expired local cache entries
func (t *Tracker) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		t.mu.Lock()
		now := time.Now()
		for userID, cached := range t.localCache {
			if now.After(cached.expiresAt) {
				delete(t.localCache, userID)
			}
		}
		t.mu.Unlock()
	}
}

// ForceRefresh forces a cache refresh for a user
func (t *Tracker) ForceRefresh(ctx context.Context, userID uint) error {
	t.invalidateCache(userID)
	return nil
}

// RecordAIRequest is a convenience method for recording AI requests
func (t *Tracker) RecordAIRequest(ctx context.Context, userID uint, projectID *uint, provider string, tokens int) error {
	return t.RecordUsage(ctx, userID, UsageAIRequests, 1, projectID, map[string]interface{}{
		"provider": provider,
		"tokens":   tokens,
	})
}

// RecordExecution is a convenience method for recording code executions
func (t *Tracker) RecordExecution(ctx context.Context, userID uint, projectID *uint, durationMs int64) error {
	minutes := durationMs / 60000
	if minutes < 1 {
		minutes = 1 // Minimum 1 minute
	}
	return t.RecordUsage(ctx, userID, UsageExecutionMinutes, minutes, projectID, map[string]interface{}{
		"duration_ms": durationMs,
	})
}

// RecordStorageChange is a convenience method for recording storage changes
func (t *Tracker) RecordStorageChange(ctx context.Context, userID uint, projectID *uint, bytesChange int64) error {
	return t.RecordUsage(ctx, userID, UsageStorageBytes, bytesChange, projectID, nil)
}

// RecordProjectCreation is a convenience method for recording project creation
func (t *Tracker) RecordProjectCreation(ctx context.Context, userID uint, projectID uint) error {
	pid := projectID
	return t.RecordUsage(ctx, userID, UsageProjects, 1, &pid, nil)
}
