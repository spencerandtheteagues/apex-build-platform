package payments

import (
	"testing"

	"apex-build/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestApplyCreditGrant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.CreditLedgerEntry{}, &models.ProcessedStripeEvent{}))

	user := &models.User{
		Username:         "trial-user",
		Email:            "trial@example.com",
		PasswordHash:     "hash",
		IsActive:         true,
		SubscriptionType: string(PlanFree),
	}
	require.NoError(t, db.Create(user).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return ApplyCreditGrant(
			tx,
			user.ID,
			FreeSignupTrialCreditsUSD,
			CreditEntryTypeSignupTrial,
			"One-time free managed trial credits",
			"",
			"",
			string(PlanFree),
		)
	}))

	var updated models.User
	require.NoError(t, db.First(&updated, user.ID).Error)
	assert.InDelta(t, FreeSignupTrialCreditsUSD, updated.CreditBalance, 0.001)

	var entries []models.CreditLedgerEntry
	require.NoError(t, db.Find(&entries).Error)
	require.Len(t, entries, 1)
	assert.Equal(t, CreditEntryTypeSignupTrial, entries[0].EntryType)
	assert.InDelta(t, FreeSignupTrialCreditsUSD, entries[0].AmountUSD, 0.001)
	assert.InDelta(t, FreeSignupTrialCreditsUSD, entries[0].BalanceAfterUSD, 0.001)
}
