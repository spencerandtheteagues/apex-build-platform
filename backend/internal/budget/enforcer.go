package budget

import (
	"fmt"
	"time"

	"apex-build/internal/cache"
	"apex-build/internal/spend"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SpendTrackerInterface is the subset of *spend.SpendTracker that BudgetEnforcer needs.
// It exists so tests can inject a lightweight mock instead of a real DB-backed tracker.
type SpendTrackerInterface interface {
	GetDailySpend(userID uint, day time.Time) (float64, int, error)
	GetMonthlySpend(userID uint, month time.Time) (float64, int, error)
	GetBuildSpend(buildID string) (float64, []spend.SpendEvent, error)
	SetCache(c *cache.RedisCache)
}

// BudgetCap is a GORM model for the budget_caps table
type BudgetCap struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	UserID    uint       `gorm:"not null" json:"user_id"`
	CapType   string     `gorm:"not null" json:"cap_type"` // daily, monthly, per_build
	ProjectID *uint      `json:"project_id,omitempty"`
	LimitUSD  float64    `gorm:"not null;type:numeric(12,6)" json:"limit_usd"`
	Action    string     `gorm:"not null;default:stop" json:"action"` // stop, warn
	IsActive  bool       `gorm:"default:true" json:"is_active"`
}

func (BudgetCap) TableName() string { return "budget_caps" }

// BudgetReservation is a short-lived hold against a user's budget while an AI
// provider request is in flight. It prevents concurrent requests from each
// seeing the same remaining budget and collectively overspending a hard cap.
type BudgetReservation struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	UserID       uint       `gorm:"not null;index" json:"user_id"`
	ProjectID    *uint      `gorm:"index" json:"project_id,omitempty"`
	BuildID      string     `gorm:"not null;index" json:"build_id"`
	Provider     string     `gorm:"not null" json:"provider"`
	Model        string     `gorm:"not null" json:"model"`
	EstimatedUSD float64    `gorm:"not null;type:numeric(12,6)" json:"estimated_usd"`
	ActualUSD    *float64   `gorm:"type:numeric(12,6)" json:"actual_usd,omitempty"`
	Status       string     `gorm:"not null;index" json:"status"` // reserved, settled, released, expired
	ExpiresAt    time.Time  `gorm:"not null;index" json:"expires_at"`
	SettledAt    *time.Time `json:"settled_at,omitempty"`
}

func (BudgetReservation) TableName() string { return "budget_reservations" }

// PreAuthResult tells whether a request is allowed
type PreAuthResult struct {
	Allowed      bool    `json:"allowed"`
	Reason       string  `json:"reason,omitempty"`
	CapType      string  `json:"cap_type,omitempty"`
	LimitUSD     float64 `json:"limit_usd,omitempty"`
	CurrentUSD   float64 `json:"current_usd,omitempty"`
	RemainingUSD float64 `json:"remaining_usd,omitempty"`
	WarningPct   float64 `json:"warning_pct,omitempty"` // 0-1, set if >0.8
}

// BudgetEnforcer checks and enforces budget caps
type BudgetEnforcer struct {
	db      *gorm.DB
	tracker SpendTrackerInterface
}

// NewBudgetEnforcer creates a new enforcer
func NewBudgetEnforcer(db *gorm.DB, tracker SpendTrackerInterface) *BudgetEnforcer {
	return &BudgetEnforcer{db: db, tracker: tracker}
}

// SetCache injects the Redis cache so the tracker can cache spend lookups.
func (be *BudgetEnforcer) SetCache(c *cache.RedisCache) {
	if be.tracker != nil {
		be.tracker.SetCache(c)
	}
}

// PreAuthorize loads active caps for the user and checks whether spending
// the estimatedCost would breach any cap. Returns denied if any "stop" cap
// is exceeded. Sets WarningPct when spend exceeds 80% of any cap.
func (be *BudgetEnforcer) PreAuthorize(userID uint, buildID string, estimatedCost float64) (*PreAuthResult, error) {
	return be.PreAuthorizeForProject(userID, nil, buildID, estimatedCost)
}

// PreAuthorizeForProject checks caps applicable to the given project. Global
// caps always apply; project-scoped caps only apply to matching projects.
func (be *BudgetEnforcer) PreAuthorizeForProject(userID uint, projectID *uint, buildID string, estimatedCost float64) (*PreAuthResult, error) {
	caps, err := be.GetApplicableCaps(userID, projectID)
	if err != nil {
		return nil, fmt.Errorf("budget: failed to load caps: %w", err)
	}

	// No caps configured -- always allowed
	if len(caps) == 0 {
		return &PreAuthResult{Allowed: true}, nil
	}

	now := time.Now().UTC()

	// Pre-fetch spend values so we only query each dimension once
	var dailySpend, monthlySpend, buildSpend float64
	var dailyFetched, monthlyFetched, buildFetched bool

	getDailySpend := func() (float64, error) {
		if !dailyFetched {
			val, qErr := be.getDailySpend(userID, now)
			if qErr != nil {
				return 0, qErr
			}
			dailySpend = val
			dailyFetched = true
		}
		return dailySpend, nil
	}

	getMonthlySpend := func() (float64, error) {
		if !monthlyFetched {
			val, qErr := be.getMonthlySpend(userID, now)
			if qErr != nil {
				return 0, qErr
			}
			monthlySpend = val
			monthlyFetched = true
		}
		return monthlySpend, nil
	}

	getBuildSpend := func() (float64, error) {
		if buildID == "" {
			return 0, nil
		}
		if !buildFetched {
			val, qErr := be.getBuildSpend(buildID)
			if qErr != nil {
				return 0, qErr
			}
			buildSpend = val
			buildFetched = true
		}
		return buildSpend, nil
	}

	// Track the highest warning percentage across all caps
	var highestWarning float64
	result := &PreAuthResult{Allowed: true}

	for _, cap := range caps {
		var current float64
		var fetchErr error

		switch cap.CapType {
		case "daily":
			if cap.ProjectID != nil {
				current, fetchErr = be.getProjectPeriodSpend(userID, cap.ProjectID, now.Format("2006-01-02"), "")
			} else {
				current, fetchErr = getDailySpend()
			}
		case "monthly":
			if cap.ProjectID != nil {
				current, fetchErr = be.getProjectPeriodSpend(userID, cap.ProjectID, "", now.Format("2006-01"))
			} else {
				current, fetchErr = getMonthlySpend()
			}
		case "per_build":
			current, fetchErr = getBuildSpend()
		default:
			continue
		}

		if fetchErr != nil {
			return nil, fmt.Errorf("budget: failed to fetch %s spend: %w", cap.CapType, fetchErr)
		}
		reserved, reservationErr := be.activeReservationTotal(userID, cap, buildID, now)
		if reservationErr != nil {
			return nil, reservationErr
		}
		current += reserved

		projected := current + estimatedCost
		remaining := cap.LimitUSD - current
		if remaining < 0 {
			remaining = 0
		}

		// Check if this cap is exceeded
		if projected > cap.LimitUSD {
			if cap.Action == "stop" {
				return &PreAuthResult{
					Allowed:      false,
					Reason:       fmt.Sprintf("%s budget cap of $%.2f exceeded (current: $%.6f, estimated: $%.6f)", cap.CapType, cap.LimitUSD, current, estimatedCost),
					CapType:      cap.CapType,
					LimitUSD:     cap.LimitUSD,
					CurrentUSD:   current,
					RemainingUSD: remaining,
				}, nil
			}
			// action == "warn": still allowed but flag it
			pct := 1.0
			if pct > highestWarning {
				highestWarning = pct
				result.CapType = cap.CapType
				result.LimitUSD = cap.LimitUSD
				result.CurrentUSD = current
				result.RemainingUSD = remaining
			}
			continue
		}

		// Check if approaching the cap (>80%)
		if cap.LimitUSD > 0 {
			pct := projected / cap.LimitUSD
			if pct > 0.8 && pct > highestWarning {
				highestWarning = pct
				result.CapType = cap.CapType
				result.LimitUSD = cap.LimitUSD
				result.CurrentUSD = current
				result.RemainingUSD = remaining
			}
		}
	}

	if highestWarning > 0 {
		result.WarningPct = highestWarning
	}
	return result, nil
}

func (be *BudgetEnforcer) getDailySpend(userID uint, day time.Time) (float64, error) {
	if be.useDirectSpendQueries() {
		dayKey := time.Date(day.UTC().Year(), day.UTC().Month(), day.UTC().Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		var result struct {
			Total float64
		}
		err := be.db.Model(&spend.SpendEvent{}).
			Select("COALESCE(SUM(billed_cost), 0) as total").
			Where("user_id = ? AND day_key = ?", userID, dayKey).
			Scan(&result).Error
		if err != nil {
			return 0, fmt.Errorf("budget: daily spend query failed: %w", err)
		}
		return result.Total, nil
	}
	val, _, err := be.tracker.GetDailySpend(userID, day)
	return val, err
}

func (be *BudgetEnforcer) getMonthlySpend(userID uint, month time.Time) (float64, error) {
	if be.useDirectSpendQueries() {
		monthUTC := month.UTC()
		monthKey := time.Date(monthUTC.Year(), monthUTC.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
		var result struct {
			Total float64
		}
		err := be.db.Model(&spend.SpendEvent{}).
			Select("COALESCE(SUM(billed_cost), 0) as total").
			Where("user_id = ? AND month_key = ?", userID, monthKey).
			Scan(&result).Error
		if err != nil {
			return 0, fmt.Errorf("budget: monthly spend query failed: %w", err)
		}
		return result.Total, nil
	}
	val, _, err := be.tracker.GetMonthlySpend(userID, month)
	return val, err
}

func (be *BudgetEnforcer) getBuildSpend(buildID string) (float64, error) {
	if be.useDirectSpendQueries() {
		var result struct {
			Total float64
		}
		err := be.db.Model(&spend.SpendEvent{}).
			Select("COALESCE(SUM(billed_cost), 0) as total").
			Where("build_id = ?", buildID).
			Scan(&result).Error
		if err != nil {
			return 0, fmt.Errorf("budget: build spend query failed: %w", err)
		}
		return result.Total, nil
	}
	val, _, err := be.tracker.GetBuildSpend(buildID)
	return val, err
}

func (be *BudgetEnforcer) useDirectSpendQueries() bool {
	if be == nil || be.db == nil {
		return false
	}
	if be.tracker == nil {
		return true
	}
	_, ok := be.tracker.(*spend.SpendTracker)
	return ok
}

// CheckBudget is like PreAuthorize but with estimatedCost=0 (just checks current state).
func (be *BudgetEnforcer) CheckBudget(userID uint, buildID string) (*PreAuthResult, error) {
	return be.PreAuthorize(userID, buildID, 0)
}

// GetCaps returns all active budget caps for a user.
func (be *BudgetEnforcer) GetCaps(userID uint) ([]BudgetCap, error) {
	var caps []BudgetCap
	err := be.db.Where("user_id = ? AND is_active = ?", userID, true).
		Where("deleted_at IS NULL").
		Order("cap_type ASC").
		Find(&caps).Error
	if err != nil {
		return nil, fmt.Errorf("budget: failed to query caps: %w", err)
	}
	return caps, nil
}

// GetApplicableCaps returns active caps that apply to a build/project. Global
// caps apply to every request. Project caps only apply when the project matches.
func (be *BudgetEnforcer) GetApplicableCaps(userID uint, projectID *uint) ([]BudgetCap, error) {
	return be.getApplicableCaps(userID, projectID, false)
}

func (be *BudgetEnforcer) getApplicableCaps(userID uint, projectID *uint, lockRows bool) ([]BudgetCap, error) {
	var caps []BudgetCap
	query := be.db.Where("user_id = ? AND is_active = ?", userID, true).
		Where("deleted_at IS NULL")
	if projectID != nil {
		query = query.Where("(project_id IS NULL OR project_id = ?)", *projectID)
	} else {
		query = query.Where("project_id IS NULL")
	}
	if lockRows {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.Order("cap_type ASC").Find(&caps).Error
	if err != nil {
		return nil, fmt.Errorf("budget: failed to query applicable caps: %w", err)
	}
	return caps, nil
}

func (be *BudgetEnforcer) getProjectPeriodSpend(userID uint, projectID *uint, dayKey, monthKey string) (float64, error) {
	if projectID == nil {
		return 0, nil
	}
	var result struct {
		Total float64
	}
	query := be.db.Model(&spend.SpendEvent{}).
		Select("COALESCE(SUM(billed_cost), 0) as total").
		Where("user_id = ? AND project_id = ?", userID, *projectID)
	if dayKey != "" {
		query = query.Where("day_key = ?", dayKey)
	}
	if monthKey != "" {
		query = query.Where("month_key = ?", monthKey)
	}
	if err := query.Scan(&result).Error; err != nil {
		return 0, fmt.Errorf("budget: failed to fetch project spend: %w", err)
	}
	return result.Total, nil
}

func (be *BudgetEnforcer) activeReservationTotal(userID uint, cap BudgetCap, buildID string, now time.Time) (float64, error) {
	var result struct {
		Total float64
	}
	query := be.db.Model(&BudgetReservation{}).
		Select("COALESCE(SUM(estimated_usd), 0) as total").
		Where("user_id = ? AND status = ? AND expires_at > ?", userID, "reserved", now)
	if cap.ProjectID != nil {
		query = query.Where("project_id = ?", *cap.ProjectID)
	}
	if cap.CapType == "per_build" {
		if buildID == "" {
			return 0, nil
		}
		query = query.Where("build_id = ?", buildID)
	}
	if err := query.Scan(&result).Error; err != nil {
		return 0, fmt.Errorf("budget: failed to fetch active reservations: %w", err)
	}
	return result.Total, nil
}

// Reserve atomically checks applicable caps and records an in-flight estimate.
func (be *BudgetEnforcer) Reserve(userID uint, projectID *uint, buildID, provider, model string, estimatedCost float64, ttl time.Duration) (*BudgetReservation, *PreAuthResult, error) {
	if estimatedCost <= 0 {
		return nil, &PreAuthResult{Allowed: true}, nil
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	now := time.Now().UTC()
	var reservation *BudgetReservation
	var result *PreAuthResult

	err := be.db.Transaction(func(tx *gorm.DB) error {
		scoped := &BudgetEnforcer{db: tx, tracker: be.tracker}
		if err := scoped.ExpireReservations(now); err != nil {
			return err
		}
		if _, err := scoped.getApplicableCaps(userID, projectID, true); err != nil {
			return err
		}
		preAuth, err := scoped.PreAuthorizeForProject(userID, projectID, buildID, estimatedCost)
		if err != nil {
			return err
		}
		result = preAuth
		if !preAuth.Allowed {
			return nil
		}
		reservation = &BudgetReservation{
			UserID:       userID,
			ProjectID:    projectID,
			BuildID:      buildID,
			Provider:     provider,
			Model:        model,
			EstimatedUSD: estimatedCost,
			Status:       "reserved",
			ExpiresAt:    now.Add(ttl),
		}
		if err := tx.Create(reservation).Error; err != nil {
			return fmt.Errorf("budget: failed to create reservation: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return reservation, result, nil
}

// SettleReservation records actual cost and removes the active hold.
func (be *BudgetEnforcer) SettleReservation(reservationID uint, actualUSD float64) error {
	now := time.Now().UTC()
	return be.db.Model(&BudgetReservation{}).
		Where("id = ? AND status = ?", reservationID, "reserved").
		Updates(map[string]any{
			"status":     "settled",
			"actual_usd": actualUSD,
			"settled_at": &now,
		}).Error
}

// ReleaseReservation removes a hold without billing it, usually after provider failure.
func (be *BudgetEnforcer) ReleaseReservation(reservationID uint) error {
	now := time.Now().UTC()
	return be.db.Model(&BudgetReservation{}).
		Where("id = ? AND status = ?", reservationID, "reserved").
		Updates(map[string]any{
			"status":     "released",
			"settled_at": &now,
		}).Error
}

// ExpireReservations marks stale reservations inactive so future checks do not
// permanently block builds if a worker crashes before releasing the hold.
func (be *BudgetEnforcer) ExpireReservations(now time.Time) error {
	return be.db.Model(&BudgetReservation{}).
		Where("status = ? AND expires_at <= ?", "reserved", now.UTC()).
		Update("status", "expired").Error
}

// SetCap upserts a budget cap. Uses the GORM Assign + FirstOrCreate pattern
// with the unique constraint on (user_id, cap_type, project_id).
func (be *BudgetEnforcer) SetCap(userID uint, capType string, projectID *uint, limitUSD float64, action string) (*BudgetCap, error) {
	if capType != "daily" && capType != "monthly" && capType != "per_build" {
		return nil, fmt.Errorf("budget: invalid cap_type %q (must be daily, monthly, or per_build)", capType)
	}
	if limitUSD <= 0 {
		return nil, fmt.Errorf("budget: limit_usd must be positive")
	}
	if action == "" {
		action = "stop"
	}
	if action != "stop" && action != "warn" {
		return nil, fmt.Errorf("budget: invalid action %q (must be stop or warn)", action)
	}

	// Build the lookup condition
	where := BudgetCap{
		UserID:  userID,
		CapType: capType,
	}
	if projectID != nil {
		where.ProjectID = projectID
	}

	cap := BudgetCap{}
	result := be.db.Where(where).
		Where("deleted_at IS NULL").
		Assign(BudgetCap{
			LimitUSD: limitUSD,
			Action:   action,
			IsActive: true,
		}).
		FirstOrCreate(&cap)

	if result.Error != nil {
		return nil, fmt.Errorf("budget: failed to upsert cap: %w", result.Error)
	}

	// If the record already existed, the Assign values may not have been
	// persisted by FirstOrCreate when the record was found (not created).
	// Explicitly save the updated fields.
	if result.RowsAffected == 0 {
		cap.LimitUSD = limitUSD
		cap.Action = action
		cap.IsActive = true
		if err := be.db.Save(&cap).Error; err != nil {
			return nil, fmt.Errorf("budget: failed to update existing cap: %w", err)
		}
	}

	return &cap, nil
}

// DeleteCap soft-deletes a budget cap, verifying ownership first.
func (be *BudgetEnforcer) DeleteCap(capID uint, userID uint) error {
	var cap BudgetCap
	err := be.db.Where("id = ? AND user_id = ? AND deleted_at IS NULL", capID, userID).First(&cap).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("budget: cap %d not found or not owned by user %d", capID, userID)
		}
		return fmt.Errorf("budget: failed to find cap: %w", err)
	}

	now := time.Now().UTC()
	cap.DeletedAt = &now
	cap.IsActive = false
	if err := be.db.Save(&cap).Error; err != nil {
		return fmt.Errorf("budget: failed to soft-delete cap: %w", err)
	}
	return nil
}
