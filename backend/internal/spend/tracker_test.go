package spend

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
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

func TestRecordSpend_CreatesEvent(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	event, err := tracker.RecordSpend(RecordSpendInput{
		UserID:       1,
		Provider:     "claude",
		Model:        "claude-opus-4-6",
		Capability:   "code",
		InputTokens:  1000,
		OutputTokens: 500,
		Status:       "success",
	})
	if err != nil {
		t.Fatalf("RecordSpend: %v", err)
	}
	if event.ID == 0 {
		t.Fatal("expected non-zero ID after create")
	}
}

func TestRecordSpend_SetsDefaultStatus(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	event, err := tracker.RecordSpend(RecordSpendInput{
		UserID: 1, Provider: "claude", Model: "claude-opus-4-6",
	})
	if err != nil {
		t.Fatalf("RecordSpend: %v", err)
	}
	if event.Status != "success" {
		t.Fatalf("expected default status 'success', got %q", event.Status)
	}
}

func TestRecordSpend_SetsDayAndMonthKeys(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	event, err := tracker.RecordSpend(RecordSpendInput{
		UserID: 1, Provider: "claude", Model: "claude-opus-4-6",
	})
	if err != nil {
		t.Fatalf("RecordSpend: %v", err)
	}

	now := time.Now().UTC()
	expectedDay := now.Format("2006-01-02")
	expectedMonth := now.Format("2006-01")

	if event.DayKey != expectedDay {
		t.Fatalf("expected DayKey=%q, got %q", expectedDay, event.DayKey)
	}
	if event.MonthKey != expectedMonth {
		t.Fatalf("expected MonthKey=%q, got %q", expectedMonth, event.MonthKey)
	}
}

func TestGetDailySpend_SumsCurrentDay(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	// Insert two events for user 1
	_, _ = tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 1000000, OutputTokens: 0})
	_, _ = tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 1000000, OutputTokens: 0})

	total, count, err := tracker.GetDailySpend(1, time.Now().UTC())
	if err != nil {
		t.Fatalf("GetDailySpend: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 events, got %d", count)
	}
	if total <= 0 {
		t.Fatalf("expected positive total, got %f", total)
	}
}

func TestGetDailySpend_IsolatesByUser(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	_, _ = tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 1000000})
	_, _ = tracker.RecordSpend(RecordSpendInput{UserID: 2, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 1000000})

	_, count1, _ := tracker.GetDailySpend(1, time.Now().UTC())
	_, count2, _ := tracker.GetDailySpend(2, time.Now().UTC())

	if count1 != 1 || count2 != 1 {
		t.Fatalf("expected each user to have 1 event, got user1=%d user2=%d", count1, count2)
	}
}

func TestGetDailySpend_NoEventsReturnsZero(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	total, count, err := tracker.GetDailySpend(99, time.Now().UTC())
	if err != nil {
		t.Fatalf("GetDailySpend: %v", err)
	}
	if total != 0 || count != 0 {
		t.Fatalf("expected 0/0 for unknown user, got %f/%d", total, count)
	}
}

func TestGetDailySpend_FiltersOtherDays(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	// Manually insert an event with a different day_key
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	db.Create(&SpendEvent{
		CreatedAt:  yesterday,
		UserID:     1,
		Provider:   "claude",
		Model:      "claude-opus-4-6",
		BilledCost: 1.0,
		DayKey:     yesterday.Format("2006-01-02"),
		MonthKey:   yesterday.Format("2006-01"),
		Status:     "success",
	})

	total, count, err := tracker.GetDailySpend(1, time.Now().UTC())
	if err != nil {
		t.Fatalf("GetDailySpend: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 events today, got %d", count)
	}
	if total != 0 {
		t.Fatalf("expected 0 total today, got %f", total)
	}
}

func TestGetMonthlySpend_SumsCurrentMonth(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	_, _ = tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 500000})
	_, _ = tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 500000})
	_, _ = tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 500000})

	total, count, err := tracker.GetMonthlySpend(1, time.Now().UTC())
	if err != nil {
		t.Fatalf("GetMonthlySpend: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 monthly events, got %d", count)
	}
	if total <= 0 {
		t.Fatalf("expected positive monthly total, got %f", total)
	}
}

func TestGetBuildSpend_SumsBilledCost(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	now := time.Now().UTC()
	// Insert events with known billed cost directly
	db.Create(&SpendEvent{
		UserID: 1, BuildID: "build-abc",
		Provider: "claude", Model: "claude-opus-4-6",
		BilledCost: 0.25, DayKey: now.Format("2006-01-02"), MonthKey: now.Format("2006-01"), Status: "success",
	})
	db.Create(&SpendEvent{
		UserID: 1, BuildID: "build-abc",
		Provider: "claude", Model: "claude-opus-4-6",
		BilledCost: 0.50, DayKey: now.Format("2006-01-02"), MonthKey: now.Format("2006-01"), Status: "success",
	})

	total, events, err := tracker.GetBuildSpend("build-abc")
	if err != nil {
		t.Fatalf("GetBuildSpend: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	const epsilon = 1e-6
	expected := 0.75
	if total < expected-epsilon || total > expected+epsilon {
		t.Fatalf("expected total=0.75, got %f", total)
	}
}

func TestGetBuildSpend_UnknownBuildReturnsZero(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	total, events, err := tracker.GetBuildSpend("nonexistent-build")
	if err != nil {
		t.Fatalf("GetBuildSpend: %v", err)
	}
	if len(events) != 0 || total != 0 {
		t.Fatalf("expected 0 events and 0 total, got %d events and %f", len(events), total)
	}
}

func TestGetUserBuildSpendScopesByUser(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	now := time.Now().UTC()
	db.Create(&SpendEvent{
		UserID: 1, BuildID: "shared-build",
		Provider: "gpt4", Model: "gpt-4.1",
		BilledCost: 0.35, DayKey: now.Format("2006-01-02"), MonthKey: now.Format("2006-01"), Status: "success",
	})
	db.Create(&SpendEvent{
		UserID: 2, BuildID: "shared-build",
		Provider: "gpt4", Model: "gpt-4.1",
		BilledCost: 9.99, DayKey: now.Format("2006-01-02"), MonthKey: now.Format("2006-01"), Status: "success",
	})

	total, events, err := tracker.GetUserBuildSpend(1, "shared-build")
	if err != nil {
		t.Fatalf("GetUserBuildSpend: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 scoped event, got %d", len(events))
	}

	const epsilon = 1e-6
	expected := 0.35
	if total < expected-epsilon || total > expected+epsilon {
		t.Fatalf("expected total=0.35, got %f", total)
	}
}

func TestGetHistory_Pagination(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	for i := 0; i < 5; i++ {
		_, _ = tracker.RecordSpend(RecordSpendInput{
			UserID: 1, Provider: "claude", Model: "claude-opus-4-6",
		})
	}

	// Page 1: first 3
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

	// Page 2: remaining 2
	page2, _, err := tracker.GetHistory(1, 3, 3)
	if err != nil {
		t.Fatalf("GetHistory page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("expected 2 events on page 2, got %d", len(page2))
	}
}

func TestGetHistory_IsolatesByUser(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	for i := 0; i < 3; i++ {
		tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6"})
	}
	tracker.RecordSpend(RecordSpendInput{UserID: 2, Provider: "claude", Model: "claude-opus-4-6"})

	_, total1, _ := tracker.GetHistory(1, 100, 0)
	_, total2, _ := tracker.GetHistory(2, 100, 0)

	if total1 != 3 {
		t.Fatalf("user 1: expected total=3, got %d", total1)
	}
	if total2 != 1 {
		t.Fatalf("user 2: expected total=1, got %d", total2)
	}
}

func TestExportCSV_HeaderAndRows(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	tracker.RecordSpend(RecordSpendInput{
		UserID:       1,
		Provider:     "claude",
		Model:        "claude-opus-4-6",
		InputTokens:  100,
		OutputTokens: 50,
		Status:       "success",
	})

	from := time.Now().UTC().AddDate(-1, 0, 0)
	to := time.Now().UTC().AddDate(1, 0, 0)

	data, err := tracker.ExportCSV(1, from, to)
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}

	r := csv.NewReader(bytes.NewReader(data))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected header + at least 1 data row, got %d rows", len(rows))
	}

	header := rows[0]
	expectedCols := []string{"id", "created_at", "build_id", "provider", "model", "input_tokens", "output_tokens", "billed_cost"}
	for _, col := range expectedCols {
		found := false
		for _, h := range header {
			if strings.EqualFold(h, col) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected column %q in CSV header, got: %v", col, header)
		}
	}
}

func TestExportCSV_EmptyRangeReturnsHeaderOnly(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	// Range in the past before any events
	from := time.Now().UTC().Add(-48 * time.Hour)
	to := time.Now().UTC().Add(-47 * time.Hour)

	data, err := tracker.ExportCSV(1, from, to)
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}

	r := csv.NewReader(bytes.NewReader(data))
	rows, _ := r.ReadAll()
	if len(rows) != 1 {
		t.Fatalf("expected header-only CSV (1 row), got %d rows", len(rows))
	}
}

func TestGetSummary_ReturnsCurrentPeriod(t *testing.T) {
	db := testDB(t)
	tracker := NewSpendTracker(db)

	tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 1000000})

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
}
