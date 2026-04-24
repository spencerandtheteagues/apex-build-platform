package spend

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// mockDBSpendTracker wraps a real SpendTracker but lets us pre-seed events
// directly into the DB so tests are deterministic without the pricing engine.
func mockDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&SpendEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// ---------------------------------------------------------------------------
// RecordSpend — valid input with seeded DB
// ---------------------------------------------------------------------------

func TestRecordSpend_Mock_ValidInput(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	event, err := tracker.RecordSpend(RecordSpendInput{
		UserID:       1,
		BuildID:      "build-abc",
		Provider:     "openai",
		Model:        "gpt-4o",
		Capability:   "code",
		InputTokens:  1000,
		OutputTokens: 500,
		Status:       "success",
	})
	if err != nil {
		t.Fatalf("RecordSpend: %v", err)
	}
	if event.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if event.Provider != "openai" {
		t.Fatalf("expected Provider=openai, got %s", event.Provider)
	}
	if event.Model != "gpt-4o" {
		t.Fatalf("expected Model=gpt-4o, got %s", event.Model)
	}
	if event.BilledCost <= 0 {
		t.Fatalf("expected positive BilledCost, got %f", event.BilledCost)
	}
}

// ---------------------------------------------------------------------------
// Daily spend aggregation
// ---------------------------------------------------------------------------

func TestGetDailySpend_Mock_Aggregation(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	now := time.Now().UTC()
	dayKey := now.Format("2006-01-02")

	// Seed 3 events directly
	for i := 0; i < 3; i++ {
		db.Create(&SpendEvent{
			UserID:     1,
			Provider:   "claude",
			Model:      "claude-opus-4-6",
			BilledCost: 0.10,
			DayKey:     dayKey,
			MonthKey:   now.Format("2006-01"),
			Status:     "success",
		})
	}

	total, count, err := tracker.GetDailySpend(1, now)
	if err != nil {
		t.Fatalf("GetDailySpend: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count=3, got %d", count)
	}
	const epsilon = 1e-6
	expected := 0.30
	if total < expected-epsilon || total > expected+epsilon {
		t.Fatalf("expected total=%.2f, got %f", expected, total)
	}
}

// ---------------------------------------------------------------------------
// Monthly spend calculation
// ---------------------------------------------------------------------------

func TestGetMonthlySpend_Mock_Aggregation(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	now := time.Now().UTC()
	monthKey := now.Format("2006-01")

	// Seed 5 events across the month
	for i := 0; i < 5; i++ {
		db.Create(&SpendEvent{
			UserID:     1,
			Provider:   "gemini",
			Model:      "gemini-2.5-pro",
			BilledCost: 0.05,
			DayKey:     now.Format("2006-01-02"),
			MonthKey:   monthKey,
			Status:     "success",
		})
	}

	total, count, err := tracker.GetMonthlySpend(1, now)
	if err != nil {
		t.Fatalf("GetMonthlySpend: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected count=5, got %d", count)
	}
	const epsilon = 1e-6
	expected := 0.25
	if total < expected-epsilon || total > expected+epsilon {
		t.Fatalf("expected total=%.2f, got %f", expected, total)
	}
}

// ---------------------------------------------------------------------------
// Build-specific spend tracking
// ---------------------------------------------------------------------------

func TestGetBuildSpend_Mock_Tracking(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	now := time.Now().UTC()
	buildID := "build-xyz"

	// Seed 2 events for this build
	for i := 0; i < 2; i++ {
		db.Create(&SpendEvent{
			UserID:     1,
			BuildID:    buildID,
			Provider:   "claude",
			Model:      "claude-opus-4-6",
			BilledCost: 0.15,
			DayKey:     now.Format("2006-01-02"),
			MonthKey:   now.Format("2006-01"),
			Status:     "success",
		})
	}

	// Seed 1 event for a different build
	db.Create(&SpendEvent{
		UserID:     1,
		BuildID:    "build-other",
		Provider:   "claude",
		Model:      "claude-opus-4-6",
		BilledCost: 1.00,
		DayKey:     now.Format("2006-01-02"),
		MonthKey:   now.Format("2006-01"),
		Status:     "success",
	})

	total, events, err := tracker.GetBuildSpend(buildID)
	if err != nil {
		t.Fatalf("GetBuildSpend: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	const epsilon = 1e-6
	expected := 0.30
	if total < expected-epsilon || total > expected+epsilon {
		t.Fatalf("expected total=%.2f, got %f", expected, total)
	}
}

func TestGetBuildSpend_Mock_NoEventsReturnsZero(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	total, events, err := tracker.GetBuildSpend("nonexistent-build")
	if err != nil {
		t.Fatalf("GetBuildSpend: %v", err)
	}
	if len(events) != 0 || total != 0 {
		t.Fatalf("expected 0 events and 0 total, got %d events and %f", len(events), total)
	}
}

// ---------------------------------------------------------------------------
// RecordSpend edge cases
// ---------------------------------------------------------------------------

func TestRecordSpend_Mock_ZeroTokensStillCreatesEvent(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	event, err := tracker.RecordSpend(RecordSpendInput{
		UserID:       1,
		Provider:     "claude",
		Model:        "claude-opus-4-6",
		InputTokens:  0,
		OutputTokens: 0,
	})
	if err != nil {
		t.Fatalf("RecordSpend: %v", err)
	}
	if event.ID == 0 {
		t.Fatal("expected event to be created even with zero tokens")
	}
}

func TestRecordSpend_Mock_MissingStatusDefaultsToSuccess(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	event, err := tracker.RecordSpend(RecordSpendInput{
		UserID:   1,
		Provider: "claude",
		Model:    "claude-opus-4-6",
	})
	if err != nil {
		t.Fatalf("RecordSpend: %v", err)
	}
	if event.Status != "success" {
		t.Fatalf("expected default status=success, got %s", event.Status)
	}
}

// ---------------------------------------------------------------------------
// GetDailySpend / GetMonthlySpend edge cases
// ---------------------------------------------------------------------------

func TestGetDailySpend_Mock_UserIsolation(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	now := time.Now().UTC()
	dayKey := now.Format("2006-01-02")

	// User 1: 2 events
	db.Create(&SpendEvent{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", BilledCost: 0.10, DayKey: dayKey, MonthKey: now.Format("2006-01"), Status: "success"})
	db.Create(&SpendEvent{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", BilledCost: 0.10, DayKey: dayKey, MonthKey: now.Format("2006-01"), Status: "success"})
	// User 2: 1 event
	db.Create(&SpendEvent{UserID: 2, Provider: "claude", Model: "claude-opus-4-6", BilledCost: 0.50, DayKey: dayKey, MonthKey: now.Format("2006-01"), Status: "success"})

	_, count1, _ := tracker.GetDailySpend(1, now)
	_, count2, _ := tracker.GetDailySpend(2, now)

	if count1 != 2 {
		t.Fatalf("expected user1 count=2, got %d", count1)
	}
	if count2 != 1 {
		t.Fatalf("expected user2 count=1, got %d", count2)
	}
}

func TestGetDailySpend_Mock_DayIsolation(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	now := time.Now().UTC()
	dayKey := now.Format("2006-01-02")
	yesterdayKey := now.AddDate(0, 0, -1).Format("2006-01-02")
	monthKey := now.Format("2006-01")

	// Today
	db.Create(&SpendEvent{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", BilledCost: 0.10, DayKey: dayKey, MonthKey: monthKey, Status: "success"})
	// Yesterday
	db.Create(&SpendEvent{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", BilledCost: 0.50, DayKey: yesterdayKey, MonthKey: monthKey, Status: "success"})

	_, count, _ := tracker.GetDailySpend(1, now)
	if count != 1 {
		t.Fatalf("expected count=1 for today only, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

func TestGetBuildSpend_Mock_DBError(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	// Close DB to force errors
	sqlDB, _ := db.DB()
	sqlDB.Close()

	_, _, err := tracker.GetBuildSpend("build-1")
	if err == nil {
		t.Fatal("expected error when DB is closed")
	}
}

func TestGetDailySpend_Mock_DBError(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	sqlDB, _ := db.DB()
	sqlDB.Close()

	_, _, err := tracker.GetDailySpend(1, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error when DB is closed")
	}
}

func TestRecordSpend_Mock_DBError(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	sqlDB, _ := db.DB()
	sqlDB.Close()

	_, err := tracker.RecordSpend(RecordSpendInput{
		UserID:   1,
		Provider: "claude",
		Model:    "claude-opus-4-6",
	})
	if err == nil {
		t.Fatal("expected error when DB is closed")
	}
}

// ---------------------------------------------------------------------------
// SpendSummary
// ---------------------------------------------------------------------------

func TestGetSummary_Mock_ReturnsDailyAndMonthly(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	now := time.Now().UTC()
	dayKey := now.Format("2006-01-02")
	monthKey := now.Format("2006-01")

	db.Create(&SpendEvent{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", BilledCost: 0.25, DayKey: dayKey, MonthKey: monthKey, Status: "success"})

	summary, err := tracker.GetSummary(1)
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if summary.DailyCount != 1 {
		t.Fatalf("expected DailyCount=1, got %d", summary.DailyCount)
	}
	if summary.MonthlyCount != 1 {
		t.Fatalf("expected MonthlyCount=1, got %d", summary.MonthlyCount)
	}
	if summary.DailySpend <= 0 {
		t.Fatalf("expected positive DailySpend, got %f", summary.DailySpend)
	}
	if summary.MonthlySpend <= 0 {
		t.Fatalf("expected positive MonthlySpend, got %f", summary.MonthlySpend)
	}
}

// ---------------------------------------------------------------------------
// Breakdown
// ---------------------------------------------------------------------------

func TestGetBreakdown_Mock_GroupByProvider(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	now := time.Now().UTC()
	dayKey := now.Format("2006-01-02")
	monthKey := now.Format("2006-01")

	db.Create(&SpendEvent{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", BilledCost: 0.20, DayKey: dayKey, MonthKey: monthKey, Status: "success"})
	db.Create(&SpendEvent{UserID: 1, Provider: "openai", Model: "gpt-4o", BilledCost: 0.30, DayKey: dayKey, MonthKey: monthKey, Status: "success"})

	items, err := tracker.GetBreakdown(BreakdownOpts{UserID: 1, DayKey: dayKey, GroupBy: "provider"})
	if err != nil {
		t.Fatalf("GetBreakdown: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(items))
	}

	// Verify totals
	var claudeTotal, openaiTotal float64
	for _, item := range items {
		if item.Key == "claude" {
			claudeTotal = item.BilledCost
		}
		if item.Key == "openai" {
			openaiTotal = item.BilledCost
		}
	}
	if claudeTotal <= 0 {
		t.Fatalf("expected positive claude total, got %f", claudeTotal)
	}
	if openaiTotal <= 0 {
		t.Fatalf("expected positive openai total, got %f", openaiTotal)
	}
}

func TestGetBreakdown_Mock_EmptyReturnsEmptySlice(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	items, err := tracker.GetBreakdown(BreakdownOpts{UserID: 999, GroupBy: "provider"})
	if err != nil {
		t.Fatalf("GetBreakdown: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items for unknown user, got %d", len(items))
	}
}

// ---------------------------------------------------------------------------
// ExportCSV
// ---------------------------------------------------------------------------

func TestExportCSV_Mock_EmptyRangeReturnsHeaderOnly(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	// Create one event now
	tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6"})

	// Query a past range with no events
	from := time.Now().UTC().Add(-48 * time.Hour)
	to := time.Now().UTC().Add(-47 * time.Hour)

	data, err := tracker.ExportCSV(1, from, to)
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected header-only CSV")
	}
	lines := bytesCountLines(data)
	if lines != 1 {
		t.Fatalf("expected 1 line (header only), got %d", lines)
	}
}

func bytesCountLines(data []byte) int {
	count := 0
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// GetHistory
// ---------------------------------------------------------------------------

func TestGetHistory_Mock_Pagination(t *testing.T) {
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	for i := 0; i < 5; i++ {
		_, _ = tracker.RecordSpend(RecordSpendInput{
			UserID:   1,
			Provider: "claude",
			Model:    "claude-opus-4-6",
		})
	}

	page1, total, err := tracker.GetHistory(1, 3, 0)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total=5, got %d", total)
	}
	if len(page1) != 3 {
		t.Fatalf("expected 3 events on page 1, got %d", len(page1))
	}

	page2, _, err := tracker.GetHistory(1, 3, 3)
	if err != nil {
		t.Fatalf("GetHistory page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("expected 2 events on page 2, got %d", len(page2))
	}
}

// ---------------------------------------------------------------------------
// ErrBudgetEnforcer integration: SpendTracker errors propagate correctly
// ---------------------------------------------------------------------------

func TestSpendTracker_Mock_ErrorPropagation(t *testing.T) {
	// This test verifies that when the DB returns an error, it is wrapped
	// and propagated rather than swallowed.
	db := mockDB(t)
	tracker := NewSpendTracker(db)

	sqlDB, _ := db.DB()
	sqlDB.Close()

	// All public methods should return errors
	_, _, err1 := tracker.GetDailySpend(1, time.Now().UTC())
	if err1 == nil {
		t.Error("GetDailySpend should error on closed DB")
	}
	_, _, err2 := tracker.GetMonthlySpend(1, time.Now().UTC())
	if err2 == nil {
		t.Error("GetMonthlySpend should error on closed DB")
	}
	_, _, err3 := tracker.GetBuildSpend("build-1")
	if err3 == nil {
		t.Error("GetBuildSpend should error on closed DB")
	}
	_, err4 := tracker.GetSummary(1)
	if err4 == nil {
		t.Error("GetSummary should error on closed DB")
	}
	_, err5 := tracker.GetBreakdown(BreakdownOpts{UserID: 1})
	if err5 == nil {
		t.Error("GetBreakdown should error on closed DB")
	}
	_, _, err6 := tracker.GetHistory(1, 10, 0)
	if err6 == nil {
		t.Error("GetHistory should error on closed DB")
	}
	_, err7 := tracker.ExportCSV(1, time.Now().UTC().AddDate(0,0,-1), time.Now().UTC())
	if err7 == nil {
		t.Error("ExportCSV should error on closed DB")
	}
}
