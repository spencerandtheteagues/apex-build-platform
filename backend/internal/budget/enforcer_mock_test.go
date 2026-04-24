package budget

import (
	"errors"
	"testing"
	"time"

	"apex-build/internal/cache"
	"apex-build/internal/spend"
)
// ---------------------------------------------------------------------------
// Mock SpendTracker (implements SpendTrackerInterface)
// ---------------------------------------------------------------------------

type mockSpendTracker struct {
	dailySpend   float64
	monthlySpend float64
	buildSpend   float64
	dailyErr     error
	monthlyErr   error
	buildErr     error
}

func (m *mockSpendTracker) GetDailySpend(userID uint, day time.Time) (float64, int, error) {
	return m.dailySpend, 1, m.dailyErr
}

func (m *mockSpendTracker) GetMonthlySpend(userID uint, month time.Time) (float64, int, error) {
	return m.monthlySpend, 1, m.monthlyErr
}

func (m *mockSpendTracker) GetBuildSpend(buildID string) (float64, []spend.SpendEvent, error) {
	return m.buildSpend, nil, m.buildErr
}

func (m *mockSpendTracker) SetCache(c *cache.RedisCache) {}

// ---------------------------------------------------------------------------
// PreAuthorize — budget NOT exceeded
// ---------------------------------------------------------------------------

func TestPreAuthorize_Mock_BudgetNotExceeded(t *testing.T) {
	mock := &mockSpendTracker{dailySpend: 1.00, monthlySpend: 5.00}
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)

	// Set a $10 daily cap
	_, _ = enforcer.SetCap(1, "daily", nil, 10.00, "stop")

	result, err := enforcer.PreAuthorize(1, "build-1", 2.00)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true when under cap, got false: %s", result.Reason)
	}
	if result.WarningPct != 0 {
		t.Fatalf("expected no warning below 80%%, got WarningPct=%f", result.WarningPct)
	}
}

// ---------------------------------------------------------------------------
// PreAuthorize — budget IS exceeded (stop action)
// ---------------------------------------------------------------------------

func TestPreAuthorize_Mock_BudgetExceeded_Stop(t *testing.T) {
	mock := &mockSpendTracker{dailySpend: 8.00}
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)

	// $10 daily cap, already at $8.00, requesting $3.00 → projected $11.00 > $10.00
	_, _ = enforcer.SetCap(1, "daily", nil, 10.00, "stop")

	result, err := enforcer.PreAuthorize(1, "", 3.00)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false when daily cap exceeded")
	}
	if result.CapType != "daily" {
		t.Fatalf("expected CapType=daily, got %q", result.CapType)
	}
	if result.LimitUSD != 10.00 {
		t.Fatalf("expected LimitUSD=10.00, got %f", result.LimitUSD)
	}
	if result.CurrentUSD != 8.00 {
		t.Fatalf("expected CurrentUSD=8.00, got %f", result.CurrentUSD)
	}
}

// ---------------------------------------------------------------------------
// CheckBudget with various caps
// ---------------------------------------------------------------------------

func TestCheckBudget_Mock_UnderCap(t *testing.T) {
	mock := &mockSpendTracker{dailySpend: 2.00}
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)
	_, _ = enforcer.SetCap(1, "daily", nil, 10.00, "stop")

	result, err := enforcer.CheckBudget(1, "build-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true when under cap")
	}
}

func TestCheckBudget_Mock_OverCap(t *testing.T) {
	mock := &mockSpendTracker{dailySpend: 12.00}
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)
	_, _ = enforcer.SetCap(1, "daily", nil, 10.00, "stop")

	result, err := enforcer.CheckBudget(1, "build-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false when already over cap")
	}
	if result.CapType != "daily" {
		t.Fatalf("expected CapType=daily, got %q", result.CapType)
	}
}

func TestCheckBudget_Mock_MonthlyCapExceeded(t *testing.T) {
	mock := &mockSpendTracker{dailySpend: 1.00, monthlySpend: 55.00}
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)
	_, _ = enforcer.SetCap(1, "monthly", nil, 50.00, "stop")

	result, err := enforcer.CheckBudget(1, "")
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

func TestCheckBudget_Mock_PerBuildCapExceeded(t *testing.T) {
	mock := &mockSpendTracker{buildSpend: 5.50}
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)
	_, _ = enforcer.SetCap(1, "per_build", nil, 5.00, "stop")

	result, err := enforcer.CheckBudget(1, "build-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false when per-build cap exceeded")
	}
	if result.CapType != "per_build" {
		t.Fatalf("expected CapType=per_build, got %q", result.CapType)
	}
}

func TestCheckBudget_Mock_WarnCapAllows(t *testing.T) {
	mock := &mockSpendTracker{dailySpend: 11.00}
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)
	_, _ = enforcer.SetCap(1, "daily", nil, 10.00, "warn")

	result, err := enforcer.CheckBudget(1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true for warn action even when over cap")
	}
	if result.WarningPct == 0 {
		t.Fatalf("expected WarningPct > 0 when cap exceeded with warn action")
	}
}

// ---------------------------------------------------------------------------
// Warning threshold behavior
// ---------------------------------------------------------------------------

func TestPreAuthorize_Mock_WarningAt80Percent(t *testing.T) {
	mock := &mockSpendTracker{dailySpend: 8.10} // 81% of $10 cap
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)
	_, _ = enforcer.SetCap(1, "daily", nil, 10.00, "stop")

	result, err := enforcer.PreAuthorize(1, "", 0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true")
	}
	if result.WarningPct == 0 {
		t.Fatalf("expected WarningPct > 0 when >80%%")
	}
	if result.WarningPct < 0.80 {
		t.Fatalf("expected WarningPct >= 0.80, got %f", result.WarningPct)
	}
}

func TestPreAuthorize_Mock_NoWarningBelow80Percent(t *testing.T) {
	mock := &mockSpendTracker{dailySpend: 7.00} // 70% of $10 cap
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)
	_, _ = enforcer.SetCap(1, "daily", nil, 10.00, "stop")

	result, err := enforcer.PreAuthorize(1, "", 0.50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true")
	}
	if result.WarningPct != 0 {
		t.Fatalf("expected WarningPct=0 below 80%%, got %f", result.WarningPct)
	}
}

func TestPreAuthorize_Mock_WarnActionSetsWarningPctTo100(t *testing.T) {
	mock := &mockSpendTracker{dailySpend: 9.50}
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)
	_, _ = enforcer.SetCap(1, "daily", nil, 10.00, "warn")

	// Requesting $1.00 projects to $10.50 > $10.00 cap. Warn action allows it.
	result, err := enforcer.PreAuthorize(1, "", 1.00)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true for warn action")
	}
	if result.WarningPct != 1.0 {
		t.Fatalf("expected WarningPct=1.0 for exceeded warn cap, got %f", result.WarningPct)
	}
}

// ---------------------------------------------------------------------------
// Multiple caps — first violation wins
// ---------------------------------------------------------------------------

func TestPreAuthorize_Mock_FirstStopViolationWins(t *testing.T) {
	mock := &mockSpendTracker{dailySpend: 9.00, monthlySpend: 95.00}
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)
	_, _ = enforcer.SetCap(1, "daily", nil, 10.00, "stop")
	_, _ = enforcer.SetCap(1, "monthly", nil, 100.00, "stop")

	// Breach only the daily cap (projected $9+2=11 > 10)
	result, err := enforcer.PreAuthorize(1, "", 2.00)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected Allowed=false")
	}
	if result.CapType != "daily" {
		t.Fatalf("expected daily cap to trigger first, got %q", result.CapType)
	}
}

// ---------------------------------------------------------------------------
// Error propagation
// ---------------------------------------------------------------------------

func TestPreAuthorize_Mock_TrackerErrorPropagated(t *testing.T) {
	mock := &mockSpendTracker{dailyErr: errors.New("db down")}
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)
	_, _ = enforcer.SetCap(1, "daily", nil, 10.00, "stop")

	_, err := enforcer.PreAuthorize(1, "", 1.00)
	if err == nil {
		t.Fatal("expected error when tracker fails")
	}
}

func TestCheckBudget_Mock_TrackerErrorPropagated(t *testing.T) {
	mock := &mockSpendTracker{monthlyErr: errors.New("db down")}
	db := testDB(t)
	enforcer := NewBudgetEnforcer(db, mock)
	_, _ = enforcer.SetCap(1, "monthly", nil, 10.00, "stop")

	_, err := enforcer.CheckBudget(1, "")
	if err == nil {
		t.Fatal("expected error when tracker fails")
	}
}
