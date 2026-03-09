package handlers

import (
	"testing"
	"time"

	"apex-build/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openTestDB opens an in-memory SQLite database and migrates the tables needed
// for payments idempotency tests.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.ProcessedStripeEvent{},
		&models.CreditLedgerEntry{},
	))
	return db
}

func seedUser(t *testing.T, db *gorm.DB, balance float64) models.User {
	t.Helper()
	u := models.User{
		Username:      "testuser",
		Email:         "test@example.com",
		PasswordHash:  "hash",
		CreditBalance: balance,
		IsActive:      true,
	}
	require.NoError(t, db.Create(&u).Error)
	return u
}

func newTestHandler(db *gorm.DB) *PaymentHandlers {
	return &PaymentHandlers{db: db}
}

// TestApplyCredit_AddsBalanceAndLedgerEntry verifies that applyCredit increments
// the user's credit_balance and records exactly one CreditLedgerEntry.
func TestApplyCredit_AddsBalanceAndLedgerEntry(t *testing.T) {
	db := openTestDB(t)
	user := seedUser(t, db, 10.00)
	h := newTestHandler(db)

	err := db.Transaction(func(tx *gorm.DB) error {
		return h.applyCredit(tx, user.ID, 25.00, "monthly_allocation", "Test allocation", "evt_001", "inv_001", "pro")
	})
	require.NoError(t, err)

	// credit_balance should be 35.00
	var updated models.User
	require.NoError(t, db.First(&updated, user.ID).Error)
	assert.InDelta(t, 35.00, updated.CreditBalance, 0.001)

	// One ledger entry
	var entries []models.CreditLedgerEntry
	require.NoError(t, db.Where("user_id = ?", user.ID).Find(&entries).Error)
	require.Len(t, entries, 1)
	assert.InDelta(t, 25.00, entries[0].AmountUSD, 0.001)
	assert.InDelta(t, 35.00, entries[0].BalanceAfterUSD, 0.001)
	assert.Equal(t, "monthly_allocation", entries[0].EntryType)
	assert.Equal(t, "evt_001", entries[0].StripeEventID)
	assert.Equal(t, "inv_001", entries[0].StripeInvoiceID)
	assert.Equal(t, "pro", entries[0].PlanType)

	// Dedup row recorded
	var dedup models.ProcessedStripeEvent
	require.NoError(t, db.Where("stripe_event_id = ?", "evt_001").First(&dedup).Error)
	assert.Equal(t, "monthly_allocation", dedup.EventType)
}

// TestApplyCredit_Idempotent verifies that calling applyCredit twice with the
// same stripe_event_id results in no double-credit.
func TestApplyCredit_Idempotent(t *testing.T) {
	db := openTestDB(t)
	user := seedUser(t, db, 0.00)
	h := newTestHandler(db)

	apply := func() error {
		return db.Transaction(func(tx *gorm.DB) error {
			return h.applyCredit(tx, user.ID, 10.00, "credit_purchase", "Purchase", "evt_dup", "", "")
		})
	}

	require.NoError(t, apply()) // first call: credits applied
	require.NoError(t, apply()) // second call: silently skipped

	var updated models.User
	require.NoError(t, db.First(&updated, user.ID).Error)
	assert.InDelta(t, 10.00, updated.CreditBalance, 0.001, "credits must not double on duplicate event")

	var count int64
	db.Model(&models.CreditLedgerEntry{}).Where("user_id = ?", user.ID).Count(&count)
	assert.Equal(t, int64(1), count, "ledger must have exactly one entry")
}

// TestApplyCredit_NoEventID verifies that when no stripe_event_id is present
// (e.g. dev mode) applyCredit still works but skips the dedup insert.
func TestApplyCredit_NoEventID(t *testing.T) {
	db := openTestDB(t)
	user := seedUser(t, db, 5.00)
	h := newTestHandler(db)

	err := db.Transaction(func(tx *gorm.DB) error {
		return h.applyCredit(tx, user.ID, 5.00, "admin_grant", "Manual grant", "", "", "")
	})
	require.NoError(t, err)

	var updated models.User
	require.NoError(t, db.First(&updated, user.ID).Error)
	assert.InDelta(t, 10.00, updated.CreditBalance, 0.001)

	// No dedup row (event ID was empty)
	var dedupCount int64
	db.Model(&models.ProcessedStripeEvent{}).Count(&dedupCount)
	assert.Equal(t, int64(0), dedupCount)
}

// TestApplyCredit_MultipleDistinctEvents verifies that different event IDs each
// apply their credit independently.
func TestApplyCredit_MultipleDistinctEvents(t *testing.T) {
	db := openTestDB(t)
	user := seedUser(t, db, 0.00)
	h := newTestHandler(db)

	events := []struct {
		id  string
		amt float64
	}{
		{"evt_jan", 35.00},
		{"evt_feb", 35.00},
		{"evt_mar", 35.00},
	}

	for _, ev := range events {
		err := db.Transaction(func(tx *gorm.DB) error {
			return h.applyCredit(tx, user.ID, ev.amt, "monthly_allocation", "Monthly", ev.id, "", "pro")
		})
		require.NoError(t, err)
	}

	var updated models.User
	require.NoError(t, db.First(&updated, user.ID).Error)
	assert.InDelta(t, 105.00, updated.CreditBalance, 0.001)

	var count int64
	db.Model(&models.CreditLedgerEntry{}).Where("user_id = ?", user.ID).Count(&count)
	assert.Equal(t, int64(3), count)
}

// TestCreditLedgerEntry_BalanceAfterProgression verifies that balance_after_usd
// in successive ledger entries reflects the running balance correctly.
func TestCreditLedgerEntry_BalanceAfterProgression(t *testing.T) {
	db := openTestDB(t)
	user := seedUser(t, db, 0.00)
	h := newTestHandler(db)

	credits := []float64{10.00, 25.00, 5.00}
	expectedBalances := []float64{10.00, 35.00, 40.00}

	for i, amt := range credits {
		evtID := "evt_" + time.Now().Format("150405.000000")
		_ = i // suppress unused warning
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			return h.applyCredit(tx, user.ID, amt, "monthly_allocation", "Monthly", evtID, "", "pro")
		}))
	}

	var entries []models.CreditLedgerEntry
	require.NoError(t, db.Where("user_id = ?", user.ID).Order("created_at ASC").Find(&entries).Error)
	require.Len(t, entries, 3)

	for i, e := range entries {
		assert.InDelta(t, expectedBalances[i], e.BalanceAfterUSD, 0.001,
			"entry %d balance_after mismatch", i)
	}
}
