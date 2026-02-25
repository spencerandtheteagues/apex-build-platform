package spend

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"apex-build/internal/pricing"

	"gorm.io/gorm"
)

// SpendTracker records and queries AI spend events.
type SpendTracker struct {
	db *gorm.DB
}

// NewSpendTracker creates a new SpendTracker backed by the given database.
func NewSpendTracker(db *gorm.DB) *SpendTracker {
	return &SpendTracker{db: db}
}

// RecordSpend computes costs via the pricing engine, persists a SpendEvent, and returns it.
func (t *SpendTracker) RecordSpend(input RecordSpendInput) (*SpendEvent, error) {
	engine := pricing.Get()
	now := time.Now().UTC()

	rawCost := engine.RawCost(input.Provider, input.Model, input.InputTokens, input.OutputTokens)
	billedCost := engine.BilledCost(input.Provider, input.Model, input.InputTokens, input.OutputTokens, input.PowerMode, input.IsBYOK)

	event := SpendEvent{
		UserID:       input.UserID,
		ProjectID:    input.ProjectID,
		BuildID:      input.BuildID,
		AgentID:      input.AgentID,
		AgentRole:    input.AgentRole,
		Provider:     input.Provider,
		Model:        input.Model,
		Capability:   input.Capability,
		IsBYOK:       input.IsBYOK,
		InputTokens:  input.InputTokens,
		OutputTokens: input.OutputTokens,
		RawCost:      rawCost,
		BilledCost:   billedCost,
		PowerMode:    input.PowerMode,
		DurationMs:   input.DurationMs,
		Status:       input.Status,
		TargetFile:   input.TargetFile,
		DayKey:       now.Format("2006-01-02"),
		MonthKey:     now.Format("2006-01"),
	}

	if event.Status == "" {
		event.Status = "success"
	}

	if err := t.db.Create(&event).Error; err != nil {
		return nil, fmt.Errorf("spend: failed to create event: %w", err)
	}
	return &event, nil
}

// GetDailySpend returns the total billed cost and event count for a user on a given day.
func (t *SpendTracker) GetDailySpend(userID uint, day time.Time) (float64, int, error) {
	dayKey := day.Format("2006-01-02")

	var result struct {
		Total float64
		Count int
	}

	err := t.db.Model(&SpendEvent{}).
		Select("COALESCE(SUM(billed_cost), 0) as total, COUNT(*) as count").
		Where("user_id = ? AND day_key = ?", userID, dayKey).
		Scan(&result).Error
	if err != nil {
		return 0, 0, fmt.Errorf("spend: daily query failed: %w", err)
	}
	return result.Total, result.Count, nil
}

// GetMonthlySpend returns the total billed cost and event count for a user in a given month.
func (t *SpendTracker) GetMonthlySpend(userID uint, month time.Time) (float64, int, error) {
	monthKey := month.Format("2006-01")

	var result struct {
		Total float64
		Count int
	}

	err := t.db.Model(&SpendEvent{}).
		Select("COALESCE(SUM(billed_cost), 0) as total, COUNT(*) as count").
		Where("user_id = ? AND month_key = ?", userID, monthKey).
		Scan(&result).Error
	if err != nil {
		return 0, 0, fmt.Errorf("spend: monthly query failed: %w", err)
	}
	return result.Total, result.Count, nil
}

// GetSummary returns a combined daily and monthly spend summary for the current period.
func (t *SpendTracker) GetSummary(userID uint) (*SpendSummary, error) {
	now := time.Now().UTC()

	dailySpend, dailyCount, err := t.GetDailySpend(userID, now)
	if err != nil {
		return nil, err
	}

	monthlySpend, monthlyCount, err := t.GetMonthlySpend(userID, now)
	if err != nil {
		return nil, err
	}

	return &SpendSummary{
		DailySpend:   dailySpend,
		MonthlySpend: monthlySpend,
		DailyCount:   dailyCount,
		MonthlyCount: monthlyCount,
	}, nil
}

// GetBreakdown returns spend grouped by the dimension specified in opts.GroupBy.
func (t *SpendTracker) GetBreakdown(opts BreakdownOpts) ([]SpendBreakdownItem, error) {
	groupCol := "provider"
	switch opts.GroupBy {
	case "model":
		groupCol = "model"
	case "agent_role":
		groupCol = "agent_role"
	case "build_id":
		groupCol = "build_id"
	default:
		groupCol = "provider"
	}

	query := t.db.Model(&SpendEvent{}).
		Select(
			groupCol+" as `key`, "+
				"COALESCE(SUM(billed_cost), 0) as billed_cost, "+
				"COALESCE(SUM(raw_cost), 0) as raw_cost, "+
				"COALESCE(SUM(input_tokens), 0) as input_tokens, "+
				"COALESCE(SUM(output_tokens), 0) as output_tokens, "+
				"COUNT(*) as count").
		Group(groupCol).
		Order("billed_cost DESC")

	if opts.UserID != 0 {
		query = query.Where("user_id = ?", opts.UserID)
	}
	if opts.DayKey != "" {
		query = query.Where("day_key = ?", opts.DayKey)
	}
	if opts.MonthKey != "" {
		query = query.Where("month_key = ?", opts.MonthKey)
	}
	if opts.BuildID != "" {
		query = query.Where("build_id = ?", opts.BuildID)
	}
	if opts.ProjectID != nil {
		query = query.Where("project_id = ?", *opts.ProjectID)
	}

	var items []SpendBreakdownItem
	if err := query.Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("spend: breakdown query failed: %w", err)
	}
	return items, nil
}

// GetBuildSpend returns the total billed cost and all events for a specific build.
func (t *SpendTracker) GetBuildSpend(buildID string) (float64, []SpendEvent, error) {
	var events []SpendEvent
	if err := t.db.Where("build_id = ?", buildID).Order("created_at ASC").Find(&events).Error; err != nil {
		return 0, nil, fmt.Errorf("spend: build query failed: %w", err)
	}

	var total float64
	for _, e := range events {
		total += e.BilledCost
	}
	return total, events, nil
}

// GetHistory returns paginated spend events for a user.
// It returns the page of events and the total count across all pages.
func (t *SpendTracker) GetHistory(userID uint, limit, offset int) ([]SpendEvent, int64, error) {
	var total int64
	if err := t.db.Model(&SpendEvent{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("spend: count query failed: %w", err)
	}

	var events []SpendEvent
	if err := t.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&events).Error; err != nil {
		return nil, 0, fmt.Errorf("spend: history query failed: %w", err)
	}

	return events, total, nil
}

// ExportCSV generates a CSV file of spend events for a user within a time range.
func (t *SpendTracker) ExportCSV(userID uint, from, to time.Time) ([]byte, error) {
	var events []SpendEvent
	if err := t.db.Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, from, to).
		Order("created_at ASC").
		Find(&events).Error; err != nil {
		return nil, fmt.Errorf("spend: export query failed: %w", err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header
	header := []string{
		"id", "created_at", "build_id", "agent_id", "agent_role",
		"provider", "model", "capability", "is_byok",
		"input_tokens", "output_tokens", "raw_cost", "billed_cost",
		"power_mode", "duration_ms", "status", "target_file",
		"day_key", "month_key",
	}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("spend: csv header write failed: %w", err)
	}

	for _, e := range events {
		row := []string{
			strconv.FormatInt(e.ID, 10),
			e.CreatedAt.Format(time.RFC3339),
			e.BuildID,
			e.AgentID,
			e.AgentRole,
			e.Provider,
			e.Model,
			e.Capability,
			strconv.FormatBool(e.IsBYOK),
			strconv.Itoa(e.InputTokens),
			strconv.Itoa(e.OutputTokens),
			strconv.FormatFloat(e.RawCost, 'f', 6, 64),
			strconv.FormatFloat(e.BilledCost, 'f', 6, 64),
			e.PowerMode,
			strconv.Itoa(e.DurationMs),
			e.Status,
			e.TargetFile,
			e.DayKey,
			e.MonthKey,
		}
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("spend: csv row write failed: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("spend: csv flush failed: %w", err)
	}

	return buf.Bytes(), nil
}
