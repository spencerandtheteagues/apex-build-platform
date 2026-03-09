package spend

import (
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// integrationDB opens a real Postgres connection.
// Tests are skipped when APEX_TEST_POSTGRES_DSN is unset.
func integrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("APEX_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("APEX_TEST_POSTGRES_DSN not set — skipping Postgres integration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if err := db.AutoMigrate(&SpendEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Use DELETE rather than TRUNCATE so the spend and budget integration test
	// packages can run concurrently against the same DB without stomping each
	// other. Both packages use user IDs in distinct ranges (spend: 1–99).
	db.Exec("DELETE FROM spend_events WHERE user_id < 100")
	t.Cleanup(func() {
		db.Exec("DELETE FROM spend_events WHERE user_id < 100")
	})
	return db
}

func TestIntegration_RecordSpend_Postgres(t *testing.T) {
	db := integrationDB(t)
	tracker := NewSpendTracker(db)

	event, err := tracker.RecordSpend(RecordSpendInput{
		UserID:       1,
		Provider:     "claude",
		Model:        "claude-opus-4-6",
		Capability:   "code",
		InputTokens:  1_000_000,
		OutputTokens: 500_000,
		Status:       "success",
	})
	if err != nil {
		t.Fatalf("RecordSpend: %v", err)
	}
	if event.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if event.RawCost <= 0 {
		t.Fatalf("expected positive RawCost, got %f", event.RawCost)
	}
	if event.BilledCost <= 0 {
		t.Fatalf("expected positive BilledCost, got %f", event.BilledCost)
	}
}

func TestIntegration_GetDailySpend_Postgres(t *testing.T) {
	db := integrationDB(t)
	tracker := NewSpendTracker(db)

	tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 500_000})
	tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 500_000})
	tracker.RecordSpend(RecordSpendInput{UserID: 2, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 500_000})

	total, count, err := tracker.GetDailySpend(1, time.Now().UTC())
	if err != nil {
		t.Fatalf("GetDailySpend: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count=2, got %d", count)
	}
	if total <= 0 {
		t.Fatalf("expected positive total, got %f", total)
	}

	// User 2 should be isolated
	_, count2, _ := tracker.GetDailySpend(2, time.Now().UTC())
	if count2 != 1 {
		t.Fatalf("user 2: expected count=1, got %d", count2)
	}
}

func TestIntegration_GetMonthlySpend_Postgres(t *testing.T) {
	db := integrationDB(t)
	tracker := NewSpendTracker(db)

	for i := 0; i < 4; i++ {
		tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 100_000})
	}

	total, count, err := tracker.GetMonthlySpend(1, time.Now().UTC())
	if err != nil {
		t.Fatalf("GetMonthlySpend: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected count=4, got %d", count)
	}
	if total <= 0 {
		t.Fatalf("expected positive monthly total, got %f", total)
	}
}

func TestIntegration_GetBuildSpend_Postgres(t *testing.T) {
	db := integrationDB(t)
	tracker := NewSpendTracker(db)

	tracker.RecordSpend(RecordSpendInput{UserID: 1, BuildID: "build-pg-1", Provider: "claude", Model: "claude-opus-4-6", InputTokens: 1_000_000})
	tracker.RecordSpend(RecordSpendInput{UserID: 1, BuildID: "build-pg-1", Provider: "claude", Model: "claude-opus-4-6", InputTokens: 1_000_000})
	tracker.RecordSpend(RecordSpendInput{UserID: 1, BuildID: "build-pg-2", Provider: "claude", Model: "claude-opus-4-6", InputTokens: 1_000_000})

	total, events, err := tracker.GetBuildSpend("build-pg-1")
	if err != nil {
		t.Fatalf("GetBuildSpend: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for build-pg-1, got %d", len(events))
	}
	if total <= 0 {
		t.Fatalf("expected positive total, got %f", total)
	}
}

func TestIntegration_GetBreakdown_Postgres(t *testing.T) {
	db := integrationDB(t)
	tracker := NewSpendTracker(db)

	tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 1_000_000})
	tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 1_000_000})
	tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "openai", Model: "gpt-4o", InputTokens: 1_000_000})

	items, err := tracker.GetBreakdown(BreakdownOpts{UserID: 1, GroupBy: "provider"})
	if err != nil {
		t.Fatalf("GetBreakdown: %v", err)
	}
	if len(items) < 1 {
		t.Fatalf("expected at least 1 breakdown item, got %d", len(items))
	}

	// Find claude row
	var claudeRow *SpendBreakdownItem
	for i := range items {
		if items[i].Key == "claude" {
			claudeRow = &items[i]
			break
		}
	}
	if claudeRow == nil {
		t.Fatalf("expected 'claude' key in breakdown, got: %+v", items)
	}
	if claudeRow.Count != 2 {
		t.Fatalf("expected claude count=2, got %d", claudeRow.Count)
	}
}

func TestIntegration_GetBreakdown_ByModel_Postgres(t *testing.T) {
	db := integrationDB(t)
	tracker := NewSpendTracker(db)

	tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 500_000})
	tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-haiku-4-5-20251001", InputTokens: 500_000})
	tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-haiku-4-5-20251001", InputTokens: 500_000})

	items, err := tracker.GetBreakdown(BreakdownOpts{UserID: 1, GroupBy: "model"})
	if err != nil {
		t.Fatalf("GetBreakdown by model: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 model rows, got %d", len(items))
	}
}

func TestIntegration_GetHistory_Pagination_Postgres(t *testing.T) {
	db := integrationDB(t)
	tracker := NewSpendTracker(db)

	for i := 0; i < 7; i++ {
		tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6"})
	}

	page1, total, err := tracker.GetHistory(1, 3, 0)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if total != 7 {
		t.Fatalf("expected total=7, got %d", total)
	}
	if len(page1) != 3 {
		t.Fatalf("expected 3 on page 1, got %d", len(page1))
	}

	page3, _, _ := tracker.GetHistory(1, 3, 6)
	if len(page3) != 1 {
		t.Fatalf("expected 1 on page 3, got %d", len(page3))
	}
}

func TestIntegration_ExportCSV_Postgres(t *testing.T) {
	db := integrationDB(t)
	tracker := NewSpendTracker(db)

	tracker.RecordSpend(RecordSpendInput{UserID: 1, Provider: "claude", Model: "claude-opus-4-6", InputTokens: 1000, OutputTokens: 500})

	from := time.Now().UTC().AddDate(-1, 0, 0)
	to := time.Now().UTC().AddDate(1, 0, 0)

	data, err := tracker.ExportCSV(1, from, to)
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty CSV")
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header + at least 1 data row, got %d lines", len(lines))
	}
	if !strings.HasPrefix(lines[0], "id,") {
		t.Fatalf("expected first line to be CSV header starting with 'id,', got: %q", lines[0])
	}
}
