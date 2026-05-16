package handlers

import (
	"strconv"
	"testing"
	"time"

	"apex-build/internal/payments"
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

func reloadUser(t *testing.T, db *gorm.DB, userID uint) models.User {
	t.Helper()
	var user models.User
	require.NoError(t, db.First(&user, userID).Error)
	return user
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

func TestHandleSubscriptionCheckoutReplayUpdatesUserWithoutCreditGrant(t *testing.T) {
	db := openTestDB(t)
	user := seedUser(t, db, 0)
	h := newTestHandler(db)

	periodStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	event := &payments.WebhookEvent{
		EventID:        "evt_subscription_checkout_replay",
		Type:           "checkout.session.completed",
		CustomerID:     "cus_subscription_checkout_replay",
		SubscriptionID: "sub_checkout_replay",
		Status:         payments.StatusActive,
		PlanType:       payments.PlanBuilder,
		PeriodStart:    periodStart,
		Metadata: map[string]string{
			"user_id": strconv.FormatUint(uint64(user.ID), 10),
		},
	}

	require.NoError(t, h.handleCheckoutCompleted(event))
	require.NoError(t, h.handleCheckoutCompleted(event))

	updated := reloadUser(t, db, user.ID)
	assert.Equal(t, "cus_subscription_checkout_replay", updated.StripeCustomerID)
	assert.Equal(t, "sub_checkout_replay", updated.SubscriptionID)
	assert.Equal(t, string(payments.StatusActive), updated.SubscriptionStatus)
	assert.Equal(t, string(payments.PlanBuilder), updated.SubscriptionType)
	assert.WithinDuration(t, periodStart, updated.BillingCycleStart, time.Second)
	assert.InDelta(t, 0, updated.CreditBalance, 0.001)

	var ledgerCount int64
	require.NoError(t, db.Model(&models.CreditLedgerEntry{}).Where("user_id = ?", user.ID).Count(&ledgerCount).Error)
	assert.Equal(t, int64(0), ledgerCount, "subscription checkout must not grant credits before invoice.paid")

	var dedupCount int64
	require.NoError(t, db.Model(&models.ProcessedStripeEvent{}).Count(&dedupCount).Error)
	assert.Equal(t, int64(0), dedupCount, "subscription metadata replay is overwrite-idempotent and does not use the credit dedup table")
}

func TestHandleCreditPurchaseCompletedReplayIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	user := seedUser(t, db, 5)
	h := newTestHandler(db)

	event := &payments.WebhookEvent{
		EventID:    "evt_credit_purchase_replay",
		Type:       "checkout.session.completed",
		CustomerID: "cus_credit_purchase_replay",
		Metadata: map[string]string{
			"type":       "credit_purchase",
			"user_id":    strconv.FormatUint(uint64(user.ID), 10),
			"credit_usd": "25",
		},
	}

	require.NoError(t, h.handleCheckoutCompleted(event))
	require.NoError(t, h.handleCheckoutCompleted(event))

	updated := reloadUser(t, db, user.ID)
	assert.Equal(t, "cus_credit_purchase_replay", updated.StripeCustomerID)
	assert.InDelta(t, 30, updated.CreditBalance, 0.001)

	var entries []models.CreditLedgerEntry
	require.NoError(t, db.Where("user_id = ?", user.ID).Find(&entries).Error)
	require.Len(t, entries, 1)
	assert.InDelta(t, 25, entries[0].AmountUSD, 0.001)
	assert.InDelta(t, 30, entries[0].BalanceAfterUSD, 0.001)
	assert.Equal(t, "credit_purchase", entries[0].EntryType)
	assert.Equal(t, "evt_credit_purchase_replay", entries[0].StripeEventID)

	var processed []models.ProcessedStripeEvent
	require.NoError(t, db.Where("stripe_event_id = ?", event.EventID).Find(&processed).Error)
	require.Len(t, processed, 1)
	assert.Equal(t, "credit_purchase", processed[0].EventType)
	require.NotNil(t, processed[0].UserID)
	assert.Equal(t, user.ID, *processed[0].UserID)
}

func TestHandleInvoicePaidReplayIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	user := seedUser(t, db, 2)
	require.NoError(t, db.Model(&user).Updates(map[string]interface{}{
		"stripe_customer_id":  "cus_invoice_paid_replay",
		"subscription_type":   string(payments.PlanPro),
		"subscription_status": string(payments.StatusPastDue),
	}).Error)
	h := newTestHandler(db)

	event := &payments.WebhookEvent{
		EventID:        "evt_invoice_paid_replay",
		Type:           "invoice.paid",
		CustomerID:     "cus_invoice_paid_replay",
		SubscriptionID: "sub_invoice_paid_replay",
		InvoiceID:      "in_invoice_paid_replay",
		Amount:         5900,
		Currency:       "usd",
	}

	require.NoError(t, h.handleInvoicePaid(event))
	require.NoError(t, h.handleInvoicePaid(event))

	plan := payments.GetPlanByType(payments.PlanPro)
	require.NotNil(t, plan)
	updated := reloadUser(t, db, user.ID)
	assert.Equal(t, string(payments.StatusActive), updated.SubscriptionStatus)
	assert.InDelta(t, 2+plan.MonthlyCreditsUSD, updated.CreditBalance, 0.001)

	var entries []models.CreditLedgerEntry
	require.NoError(t, db.Where("user_id = ?", user.ID).Find(&entries).Error)
	require.Len(t, entries, 1)
	assert.InDelta(t, plan.MonthlyCreditsUSD, entries[0].AmountUSD, 0.001)
	assert.Equal(t, "monthly_allocation", entries[0].EntryType)
	assert.Equal(t, "evt_invoice_paid_replay", entries[0].StripeEventID)
	assert.Equal(t, "in_invoice_paid_replay", entries[0].StripeInvoiceID)
	assert.Equal(t, string(payments.PlanPro), entries[0].PlanType)

	var processedCount int64
	require.NoError(t, db.Model(&models.ProcessedStripeEvent{}).Where("stripe_event_id = ?", event.EventID).Count(&processedCount).Error)
	assert.Equal(t, int64(1), processedCount)
}

func TestHandleInvoicePaidUsesWebhookPlanOverStaleUserPlan(t *testing.T) {
	db := openTestDB(t)
	user := seedUser(t, db, 2)
	require.NoError(t, db.Model(&user).Updates(map[string]interface{}{
		"stripe_customer_id":  "cus_invoice_paid_plan_switch",
		"subscription_type":   string(payments.PlanBuilder),
		"subscription_status": string(payments.StatusPastDue),
	}).Error)
	h := newTestHandler(db)

	event := &payments.WebhookEvent{
		EventID:        "evt_invoice_paid_plan_switch",
		Type:           "invoice.paid",
		CustomerID:     "cus_invoice_paid_plan_switch",
		SubscriptionID: "sub_invoice_paid_plan_switch",
		InvoiceID:      "in_invoice_paid_plan_switch",
		Amount:         5900,
		Currency:       "usd",
		PlanType:       payments.PlanPro,
	}

	require.NoError(t, h.handleInvoicePaid(event))

	plan := payments.GetPlanByType(payments.PlanPro)
	require.NotNil(t, plan)
	updated := reloadUser(t, db, user.ID)
	assert.Equal(t, string(payments.PlanPro), updated.SubscriptionType)
	assert.Equal(t, string(payments.StatusActive), updated.SubscriptionStatus)
	assert.Equal(t, "sub_invoice_paid_plan_switch", updated.SubscriptionID)
	assert.InDelta(t, 2+plan.MonthlyCreditsUSD, updated.CreditBalance, 0.001)

	var entries []models.CreditLedgerEntry
	require.NoError(t, db.Where("user_id = ?", user.ID).Find(&entries).Error)
	require.Len(t, entries, 1)
	assert.Equal(t, string(payments.PlanPro), entries[0].PlanType)
}

func TestHandleSubscriptionPlanChangeAndDeletionReplayAreIdempotent(t *testing.T) {
	db := openTestDB(t)
	user := seedUser(t, db, 0)
	require.NoError(t, db.Model(&user).Updates(map[string]interface{}{
		"stripe_customer_id":  "cus_plan_change_replay",
		"subscription_id":     "sub_original",
		"subscription_type":   string(payments.PlanBuilder),
		"subscription_status": string(payments.StatusActive),
	}).Error)
	h := newTestHandler(db)

	periodStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	updateEvent := &payments.WebhookEvent{
		EventID:        "evt_subscription_update_replay",
		Type:           "customer.subscription.updated",
		CustomerID:     "cus_plan_change_replay",
		SubscriptionID: "sub_updated",
		Status:         payments.StatusActive,
		PlanType:       payments.PlanTeam,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	}

	require.NoError(t, h.handleSubscriptionUpdate(updateEvent))
	require.NoError(t, h.handleSubscriptionUpdate(updateEvent))

	updated := reloadUser(t, db, user.ID)
	assert.Equal(t, "sub_updated", updated.SubscriptionID)
	assert.Equal(t, string(payments.StatusActive), updated.SubscriptionStatus)
	assert.Equal(t, string(payments.PlanTeam), updated.SubscriptionType)
	assert.WithinDuration(t, periodStart, updated.BillingCycleStart, time.Second)
	assert.WithinDuration(t, periodEnd, updated.SubscriptionEnd, time.Second)

	deleteEvent := &payments.WebhookEvent{
		EventID:        "evt_subscription_deleted_replay",
		Type:           "customer.subscription.deleted",
		CustomerID:     "cus_plan_change_replay",
		SubscriptionID: "sub_updated",
		Status:         payments.StatusCanceled,
	}
	require.NoError(t, h.handleSubscriptionDeleted(deleteEvent))
	require.NoError(t, h.handleSubscriptionDeleted(deleteEvent))

	deleted := reloadUser(t, db, user.ID)
	assert.Equal(t, "", deleted.SubscriptionID)
	assert.Equal(t, string(payments.StatusCanceled), deleted.SubscriptionStatus)
	assert.Equal(t, string(payments.PlanFree), deleted.SubscriptionType)

	var ledgerCount int64
	require.NoError(t, db.Model(&models.CreditLedgerEntry{}).Where("user_id = ?", user.ID).Count(&ledgerCount).Error)
	assert.Equal(t, int64(0), ledgerCount)
}

func TestHandleInvoicePaymentFailedReplaySetsPastDueOnce(t *testing.T) {
	db := openTestDB(t)
	user := seedUser(t, db, 0)
	require.NoError(t, db.Model(&user).Updates(map[string]interface{}{
		"stripe_customer_id":  "cus_invoice_failed_replay",
		"subscription_id":     "sub_invoice_failed_replay",
		"subscription_type":   string(payments.PlanPro),
		"subscription_status": string(payments.StatusActive),
	}).Error)
	h := newTestHandler(db)

	event := &payments.WebhookEvent{
		EventID:        "evt_invoice_failed_replay",
		Type:           "invoice.payment_failed",
		CustomerID:     "cus_invoice_failed_replay",
		SubscriptionID: "sub_invoice_failed_replay",
		InvoiceID:      "in_invoice_failed_replay",
		Amount:         5900,
		Currency:       "usd",
		Status:         payments.StatusPastDue,
	}

	require.NoError(t, h.handleInvoicePaymentFailed(event))
	require.NoError(t, h.handleInvoicePaymentFailed(event))

	updated := reloadUser(t, db, user.ID)
	assert.Equal(t, string(payments.StatusPastDue), updated.SubscriptionStatus)
	assert.InDelta(t, 0, updated.CreditBalance, 0.001)

	var ledgerCount int64
	require.NoError(t, db.Model(&models.CreditLedgerEntry{}).Where("user_id = ?", user.ID).Count(&ledgerCount).Error)
	assert.Equal(t, int64(0), ledgerCount)
}
