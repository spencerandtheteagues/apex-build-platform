package budget

import (
	"testing"
	"time"

	"apex-build/internal/spend"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&BudgetCap{}, &BudgetReservation{}, &spend.SpendEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func testEnforcer(t *testing.T) (*BudgetEnforcer, *gorm.DB) {
	t.Helper()
	db := testDB(t)
	tracker := spend.NewSpendTracker(db)
	return NewBudgetEnforcer(db, tracker), db
}

// seedSpend inserts a SpendEvent with a known billed_cost directly, bypassing
// the pricing engine so tests are deterministic.
func seedSpend(t *testing.T, db *gorm.DB, userID uint, buildID string, billedCost float64) {
	t.Helper()
	seedProjectSpend(t, db, userID, nil, buildID, billedCost)
}

func seedProjectSpend(t *testing.T, db *gorm.DB, userID uint, projectID *uint, buildID string, billedCost float64) {
	t.Helper()
	now := time.Now().UTC()
	ev := spend.SpendEvent{
		UserID:     userID,
		ProjectID:  projectID,
		BuildID:    buildID,
		Provider:   "claude",
		Model:      "claude-opus-4-6",
		BilledCost: billedCost,
		DayKey:     now.Format("2006-01-02"),
		MonthKey:   now.Format("2006-01"),
		Status:     "success",
	}
	if err := db.Create(&ev).Error; err != nil {
		t.Fatalf("seedSpend: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PreAuthorize — no caps
// ---------------------------------------------------------------------------

func TestPreAuthorize_NoCaps_Allowed(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	result, err := enforcer.PreAuthorize(1, "build-1", 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true with no caps, got false")
	}
}

// ---------------------------------------------------------------------------
// PreAuthorize — daily cap exceeded (action=stop)
// ---------------------------------------------------------------------------

func TestPreAuthorize_DailyCap_Stop_Exceeded(t *testing.T) {
	enforcer, db := testEnforcer(t)

	// Set a $1.00 daily cap
	enforcer.SetCap(1, "daily", nil, 1.00, "stop")

	// Seed $0.95 of spend today
	seedSpend(t, db, 1, "", 0.95)

	// Requesting $0.10 more should breach the cap
	result, err := enforcer.PreAuthorize(1, "", 0.10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false when daily cap exceeded")
	}
	if result.CapType != "daily" {
		t.Fatalf("expected CapType=daily, got %q", result.CapType)
	}
	if result.LimitUSD != 1.00 {
		t.Fatalf("expected LimitUSD=1.00, got %f", result.LimitUSD)
	}
}

// ---------------------------------------------------------------------------
// PreAuthorize — daily cap not yet exceeded
// ---------------------------------------------------------------------------

func TestPreAuthorize_DailyCap_Stop_NotExceeded(t *testing.T) {
	enforcer, db := testEnforcer(t)
	enforcer.SetCap(1, "daily", nil, 10.00, "stop")
	seedSpend(t, db, 1, "", 1.00) // only $1 of $10 used

	result, err := enforcer.PreAuthorize(1, "", 0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true, got false: %s", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// PreAuthorize — monthly cap exceeded (action=stop)
// ---------------------------------------------------------------------------

func TestPreAuthorize_MonthlyCap_Stop_Exceeded(t *testing.T) {
	enforcer, db := testEnforcer(t)
	enforcer.SetCap(1, "monthly", nil, 5.00, "stop")
	seedSpend(t, db, 1, "", 4.99)

	result, err := enforcer.PreAuthorize(1, "", 0.02)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false when monthly cap exceeded")
	}
	if result.CapType != "monthly" {
		t.Fatalf("expected CapType=monthly, got %q", result.CapType)
	}
}

// ---------------------------------------------------------------------------
// PreAuthorize — per_build cap exceeded (action=stop)
// ---------------------------------------------------------------------------

func TestPreAuthorize_PerBuildCap_Stop_Exceeded(t *testing.T) {
	enforcer, db := testEnforcer(t)
	enforcer.SetCap(1, "per_build", nil, 2.00, "stop")
	seedSpend(t, db, 1, "build-x", 1.99)

	result, err := enforcer.PreAuthorize(1, "build-x", 0.02)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false when per_build cap exceeded")
	}
	if result.CapType != "per_build" {
		t.Fatalf("expected CapType=per_build, got %q", result.CapType)
	}
}

// ---------------------------------------------------------------------------
// PreAuthorize — warn action: allows but flags
// ---------------------------------------------------------------------------

func TestPreAuthorize_DailyCap_Warn_AllowedWithWarning(t *testing.T) {
	enforcer, db := testEnforcer(t)
	enforcer.SetCap(1, "daily", nil, 1.00, "warn")
	seedSpend(t, db, 1, "", 0.95) // at 95%, exceeds cap with $0.10

	result, err := enforcer.PreAuthorize(1, "", 0.10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true for warn cap, got false")
	}
	if result.WarningPct == 0 {
		t.Fatalf("expected WarningPct>0 when warn cap exceeded, got 0")
	}
}

// ---------------------------------------------------------------------------
// PreAuthorize — approaching cap (>80%) triggers WarningPct
// ---------------------------------------------------------------------------

func TestPreAuthorize_ApproachingCap_SetsWarningPct(t *testing.T) {
	enforcer, db := testEnforcer(t)
	enforcer.SetCap(1, "daily", nil, 1.00, "stop")
	seedSpend(t, db, 1, "", 0.82) // 82% used

	result, err := enforcer.PreAuthorize(1, "", 0.01) // projected: 83% — still under cap
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true, got false")
	}
	if result.WarningPct == 0 {
		t.Fatalf("expected WarningPct>0 when >80%%, got 0")
	}
}

func TestPreAuthorize_Below80Pct_NoWarning(t *testing.T) {
	enforcer, db := testEnforcer(t)
	enforcer.SetCap(1, "daily", nil, 10.00, "stop")
	seedSpend(t, db, 1, "", 1.00) // 10% used

	result, err := enforcer.PreAuthorize(1, "", 0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true")
	}
	if result.WarningPct != 0 {
		t.Fatalf("expected WarningPct=0 when below 80%%, got %f", result.WarningPct)
	}
}

func TestPreAuthorizeForProject_AppliesOnlyMatchingProjectCaps(t *testing.T) {
	enforcer, db := testEnforcer(t)
	projectOne := uint(10)
	projectTwo := uint(20)
	_, _ = enforcer.SetCap(1, "daily", &projectOne, 1.00, "stop")
	seedProjectSpend(t, db, 1, &projectTwo, "", 0.95)

	otherProject, err := enforcer.PreAuthorizeForProject(1, &projectTwo, "", 0.10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !otherProject.Allowed {
		t.Fatalf("project-specific cap blocked unrelated project: %s", otherProject.Reason)
	}

	seedProjectSpend(t, db, 1, &projectOne, "", 0.95)
	matchingProject, err := enforcer.PreAuthorizeForProject(1, &projectOne, "", 0.10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matchingProject.Allowed {
		t.Fatalf("expected matching project cap to block projected spend")
	}
}

func TestReserveCountsActiveReservationsAgainstHardCap(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	_, _ = enforcer.SetCap(1, "daily", nil, 1.00, "stop")

	first, firstResult, err := enforcer.Reserve(1, nil, "build-1", "claude", "model", 0.60, time.Minute)
	if err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	if first == nil || !firstResult.Allowed {
		t.Fatalf("expected first reservation to be allowed, got reservation=%+v result=%+v", first, firstResult)
	}

	second, secondResult, err := enforcer.Reserve(1, nil, "build-2", "claude", "model", 0.50, time.Minute)
	if err != nil {
		t.Fatalf("second reserve: %v", err)
	}
	if second != nil {
		t.Fatalf("expected second reservation to be denied, got %+v", second)
	}
	if secondResult == nil || secondResult.Allowed {
		t.Fatalf("expected second reservation to be blocked by active reservation, got %+v", secondResult)
	}

	if err := enforcer.ReleaseReservation(first.ID); err != nil {
		t.Fatalf("release first reservation: %v", err)
	}
	third, thirdResult, err := enforcer.Reserve(1, nil, "build-3", "claude", "model", 0.50, time.Minute)
	if err != nil {
		t.Fatalf("third reserve: %v", err)
	}
	if third == nil || !thirdResult.Allowed {
		t.Fatalf("expected reservation after release to be allowed, got reservation=%+v result=%+v", third, thirdResult)
	}
}

// ---------------------------------------------------------------------------
// CheckBudget
// ---------------------------------------------------------------------------

func TestCheckBudget_NoCost_ReturnsAllowed(t *testing.T) {
	enforcer, db := testEnforcer(t)
	enforcer.SetCap(1, "daily", nil, 5.00, "stop")
	seedSpend(t, db, 1, "", 2.00) // 40% used — under cap even with 0 estimate

	result, err := enforcer.CheckBudget(1, "")
	if err != nil {
		t.Fatalf("CheckBudget: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true with 0 estimated cost")
	}
}

func TestCheckBudget_AlreadyOverCap_Denied(t *testing.T) {
	enforcer, db := testEnforcer(t)
	enforcer.SetCap(1, "daily", nil, 1.00, "stop")
	seedSpend(t, db, 1, "", 1.05) // already over cap

	result, err := enforcer.CheckBudget(1, "")
	if err != nil {
		t.Fatalf("CheckBudget: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false when already over cap")
	}
}

// ---------------------------------------------------------------------------
// SetCap validation
// ---------------------------------------------------------------------------

func TestSetCap_InvalidCapType(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	_, err := enforcer.SetCap(1, "weekly", nil, 10.0, "stop")
	if err == nil {
		t.Fatal("expected error for invalid cap_type 'weekly'")
	}
}

func TestSetCap_NonPositiveLimitUSD(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	_, err := enforcer.SetCap(1, "daily", nil, 0.0, "stop")
	if err == nil {
		t.Fatal("expected error for limit_usd=0")
	}
	_, err = enforcer.SetCap(1, "daily", nil, -5.0, "stop")
	if err == nil {
		t.Fatal("expected error for negative limit_usd")
	}
}

func TestSetCap_InvalidAction(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	_, err := enforcer.SetCap(1, "daily", nil, 10.0, "ignore")
	if err == nil {
		t.Fatal("expected error for invalid action 'ignore'")
	}
}

func TestSetCap_DefaultActionIsStop(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	cap, err := enforcer.SetCap(1, "daily", nil, 10.0, "")
	if err != nil {
		t.Fatalf("SetCap: %v", err)
	}
	if cap.Action != "stop" {
		t.Fatalf("expected default action=stop, got %q", cap.Action)
	}
}

func TestSetCap_ValidCapTypes(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	for _, capType := range []string{"daily", "monthly", "per_build"} {
		_, err := enforcer.SetCap(1, capType, nil, 10.0, "stop")
		if err != nil {
			t.Fatalf("SetCap(%q): unexpected error: %v", capType, err)
		}
	}
}

// ---------------------------------------------------------------------------
// GetCaps
// ---------------------------------------------------------------------------

func TestGetCaps_ReturnsOnlyActiveCaps(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	enforcer.SetCap(1, "daily", nil, 5.0, "stop")
	enforcer.SetCap(1, "monthly", nil, 100.0, "warn")
	enforcer.SetCap(2, "daily", nil, 3.0, "stop") // different user

	caps, err := enforcer.GetCaps(1)
	if err != nil {
		t.Fatalf("GetCaps: %v", err)
	}
	if len(caps) != 2 {
		t.Fatalf("expected 2 caps for user 1, got %d", len(caps))
	}
	for _, c := range caps {
		if c.UserID != 1 {
			t.Fatalf("got cap for wrong user: %d", c.UserID)
		}
	}
}

func TestGetCaps_EmptyForUnknownUser(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	caps, err := enforcer.GetCaps(999)
	if err != nil {
		t.Fatalf("GetCaps: %v", err)
	}
	if len(caps) != 0 {
		t.Fatalf("expected 0 caps for unknown user, got %d", len(caps))
	}
}

// ---------------------------------------------------------------------------
// DeleteCap
// ---------------------------------------------------------------------------

func TestDeleteCap_OwnershipCheck(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	cap, _ := enforcer.SetCap(1, "daily", nil, 10.0, "stop")

	// Wrong user should fail
	err := enforcer.DeleteCap(cap.ID, 2)
	if err == nil {
		t.Fatal("expected error when deleting cap owned by another user")
	}
}

func TestDeleteCap_OwnerCanDelete(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	cap, _ := enforcer.SetCap(1, "daily", nil, 10.0, "stop")

	if err := enforcer.DeleteCap(cap.ID, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cap should no longer be returned
	caps, _ := enforcer.GetCaps(1)
	if len(caps) != 0 {
		t.Fatalf("expected 0 active caps after delete, got %d", len(caps))
	}
}

func TestDeleteCap_DeletedCapCannotBeDeletedAgain(t *testing.T) {
	enforcer, _ := testEnforcer(t)
	cap, _ := enforcer.SetCap(1, "daily", nil, 10.0, "stop")
	enforcer.DeleteCap(cap.ID, 1)

	err := enforcer.DeleteCap(cap.ID, 1)
	if err == nil {
		t.Fatal("expected error when deleting an already-deleted cap")
	}
}

// ---------------------------------------------------------------------------
// Multi-cap: first stop violation wins
// ---------------------------------------------------------------------------

func TestPreAuthorize_MultipleCaps_FirstStopWins(t *testing.T) {
	enforcer, db := testEnforcer(t)
	enforcer.SetCap(1, "daily", nil, 1.00, "stop")
	enforcer.SetCap(1, "monthly", nil, 50.00, "stop")

	// Breach only the daily cap
	seedSpend(t, db, 1, "", 0.99)

	result, err := enforcer.PreAuthorize(1, "", 0.02)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false when daily cap is exceeded")
	}
	if result.CapType != "daily" {
		t.Fatalf("expected CapType=daily, got %q", result.CapType)
	}
}
