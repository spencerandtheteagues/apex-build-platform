package budget

import (
	"fmt"
	"time"

	"apex-build/internal/spend"

	"gorm.io/gorm"
)

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
	tracker *spend.SpendTracker
}

// NewBudgetEnforcer creates a new enforcer
func NewBudgetEnforcer(db *gorm.DB, tracker *spend.SpendTracker) *BudgetEnforcer {
	return &BudgetEnforcer{db: db, tracker: tracker}
}

// PreAuthorize loads active caps for the user and checks whether spending
// the estimatedCost would breach any cap. Returns denied if any "stop" cap
// is exceeded. Sets WarningPct when spend exceeds 80% of any cap.
func (be *BudgetEnforcer) PreAuthorize(userID uint, buildID string, estimatedCost float64) (*PreAuthResult, error) {
	caps, err := be.GetCaps(userID)
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
			val, _, qErr := be.tracker.GetDailySpend(userID, now)
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
			val, _, qErr := be.tracker.GetMonthlySpend(userID, now)
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
			val, _, qErr := be.tracker.GetBuildSpend(buildID)
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
			current, fetchErr = getDailySpend()
		case "monthly":
			current, fetchErr = getMonthlySpend()
		case "per_build":
			current, fetchErr = getBuildSpend()
		default:
			continue
		}

		if fetchErr != nil {
			return nil, fmt.Errorf("budget: failed to fetch %s spend: %w", cap.CapType, fetchErr)
		}

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
