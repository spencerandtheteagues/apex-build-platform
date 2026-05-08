package budget

import (
	"os"
	"testing"
	"time"

	"apex-build/internal/spend"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

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
	if err := db.AutoMigrate(&BudgetCap{}, &BudgetReservation{}, &spend.SpendEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Budget integration tests use user IDs >= 1000 to avoid collision with
	// the spend package's integration tests (user IDs < 100) when both run in
	// parallel against the same database.
	db.Exec("DELETE FROM spend_events WHERE user_id >= 1000")
	db.Exec("DELETE FROM budget_reservations WHERE user_id >= 1000")
	db.Exec("DELETE FROM budget_caps WHERE user_id >= 1000")
	t.Cleanup(func() {
		db.Exec("DELETE FROM spend_events WHERE user_id >= 1000")
		db.Exec("DELETE FROM budget_reservations WHERE user_id >= 1000")
		db.Exec("DELETE FROM budget_caps WHERE user_id >= 1000")
	})
	return db
}

func integrationEnforcer(t *testing.T) (*BudgetEnforcer, *gorm.DB) {
	t.Helper()
	db := integrationDB(t)
	tracker := spend.NewSpendTracker(db)
	return NewBudgetEnforcer(db, tracker), db
}

func seedSpendPG(t *testing.T, db *gorm.DB, userID uint, buildID string, billedCost float64) {
	t.Helper()
	now := time.Now().UTC()
	ev := spend.SpendEvent{
		UserID:     userID,
		BuildID:    buildID,
		Provider:   "claude",
		Model:      "claude-opus-4-6",
		BilledCost: billedCost,
		DayKey:     now.Format("2006-01-02"),
		MonthKey:   now.Format("2006-01"),
		Status:     "success",
	}
	if err := db.Create(&ev).Error; err != nil {
		t.Fatalf("seedSpendPG: %v", err)
	}
}

func TestIntegration_PreAuthorize_NoCaps_Postgres(t *testing.T) {
	enforcer, _ := integrationEnforcer(t)
	result, err := enforcer.PreAuthorize(1000, "build-1", 1.0)
	if err != nil {
		t.Fatalf("PreAuthorize: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true with no caps")
	}
}

func TestIntegration_PreAuthorize_DailyCap_Stop_Exceeded_Postgres(t *testing.T) {
	enforcer, db := integrationEnforcer(t)
	enforcer.SetCap(1000, "daily", nil, 5.00, "stop")
	seedSpendPG(t, db, 1000, "", 4.99)

	result, err := enforcer.PreAuthorize(1000, "", 0.02)
	if err != nil {
		t.Fatalf("PreAuthorize: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false, cap is breached")
	}
	if result.CapType != "daily" {
		t.Fatalf("expected CapType=daily, got %q", result.CapType)
	}
}

func TestIntegration_PreAuthorize_MonthlyCap_Stop_Exceeded_Postgres(t *testing.T) {
	enforcer, db := integrationEnforcer(t)
	enforcer.SetCap(1000, "monthly", nil, 20.00, "stop")
	seedSpendPG(t, db, 1000, "", 19.99)

	result, err := enforcer.PreAuthorize(1000, "", 0.02)
	if err != nil {
		t.Fatalf("PreAuthorize: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false for monthly cap breach")
	}
}

func TestIntegration_PreAuthorize_PerBuildCap_Postgres(t *testing.T) {
	enforcer, db := integrationEnforcer(t)
	enforcer.SetCap(1000, "per_build", nil, 1.00, "stop")
	seedSpendPG(t, db, 1000, "build-pg-x", 0.99)

	result, err := enforcer.PreAuthorize(1000, "build-pg-x", 0.02)
	if err != nil {
		t.Fatalf("PreAuthorize: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false for per_build cap breach")
	}
}

func TestIntegration_PreAuthorize_WarnAction_Postgres(t *testing.T) {
	enforcer, db := integrationEnforcer(t)
	enforcer.SetCap(1000, "daily", nil, 5.00, "warn")
	seedSpendPG(t, db, 1000, "", 4.99)

	result, err := enforcer.PreAuthorize(1000, "", 0.02)
	if err != nil {
		t.Fatalf("PreAuthorize: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true for warn cap (over limit but warn action)")
	}
	if result.WarningPct == 0 {
		t.Fatalf("expected WarningPct>0 for warn cap breach")
	}
}

func TestIntegration_PreAuthorize_ApproachingCap_Postgres(t *testing.T) {
	enforcer, db := integrationEnforcer(t)
	enforcer.SetCap(1000, "daily", nil, 10.00, "stop")
	seedSpendPG(t, db, 1000, "", 8.50) // 85% used

	result, err := enforcer.PreAuthorize(1000, "", 0.10) // projected: ~86%
	if err != nil {
		t.Fatalf("PreAuthorize: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true (still under cap)")
	}
	if result.WarningPct == 0 {
		t.Fatalf("expected WarningPct>0 when >80%% used")
	}
}

func TestIntegration_SetCap_Upsert_Postgres(t *testing.T) {
	enforcer, _ := integrationEnforcer(t)

	cap1, err := enforcer.SetCap(1000, "daily", nil, 5.00, "stop")
	if err != nil {
		t.Fatalf("SetCap: %v", err)
	}

	// Update the same cap
	cap2, err := enforcer.SetCap(1000, "daily", nil, 10.00, "warn")
	if err != nil {
		t.Fatalf("SetCap update: %v", err)
	}

	// Should be same row, updated values
	_ = cap1
	if cap2.LimitUSD != 10.00 {
		t.Fatalf("expected LimitUSD=10.00 after upsert, got %f", cap2.LimitUSD)
	}
	if cap2.Action != "warn" {
		t.Fatalf("expected action=warn after upsert, got %s", cap2.Action)
	}

	// Only 1 cap should exist
	caps, _ := enforcer.GetCaps(1000)
	if len(caps) != 1 {
		t.Fatalf("expected 1 cap after upsert, got %d", len(caps))
	}
}

func TestIntegration_DeleteCap_SoftDelete_Postgres(t *testing.T) {
	enforcer, _ := integrationEnforcer(t)
	cap, _ := enforcer.SetCap(1000, "daily", nil, 5.00, "stop")

	if err := enforcer.DeleteCap(cap.ID, 1000); err != nil {
		t.Fatalf("DeleteCap: %v", err)
	}

	// GetCaps should not return deleted cap
	caps, _ := enforcer.GetCaps(1000)
	if len(caps) != 0 {
		t.Fatalf("expected 0 caps after soft-delete, got %d", len(caps))
	}

	// Raw DB query confirms soft-delete (deleted_at set, is_active=false)
	var raw BudgetCap
	enforcer.db.Unscoped().First(&raw, cap.ID)
	if raw.DeletedAt == nil {
		t.Fatal("expected deleted_at to be set after soft-delete")
	}
	if raw.IsActive {
		t.Fatal("expected is_active=false after soft-delete")
	}
}

func TestIntegration_MultipleUsers_Isolation_Postgres(t *testing.T) {
	enforcer, db := integrationEnforcer(t)

	enforcer.SetCap(1000, "daily", nil, 2.00, "stop")
	enforcer.SetCap(1001, "daily", nil, 100.00, "stop")

	// User 1 is at cap
	seedSpendPG(t, db, 1000, "", 1.99)

	result1, _ := enforcer.PreAuthorize(1000, "", 0.02)
	result2, _ := enforcer.PreAuthorize(1001, "", 0.02)

	if result1.Allowed {
		t.Fatal("user 1 should be denied (at cap)")
	}
	if !result2.Allowed {
		t.Fatal("user 2 should be allowed (well under cap)")
	}
}

func TestIntegration_CheckBudget_Postgres(t *testing.T) {
	enforcer, db := integrationEnforcer(t)
	enforcer.SetCap(1000, "daily", nil, 1.00, "stop")
	seedSpendPG(t, db, 1000, "", 1.05) // already over

	result, err := enforcer.CheckBudget(1000, "")
	if err != nil {
		t.Fatalf("CheckBudget: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected Allowed=false when already over cap")
	}
}
