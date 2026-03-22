package payments

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

const (
	CreditEntryTypeSignupTrial = "signup_trial"
)

func isDuplicateCreditInsertError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") || strings.Contains(message, "unique")
}

// ApplyCreditGrant records an immutable credit ledger entry and updates the user's cached balance.
// If stripeEventID is present, it is used as the idempotency key.
func ApplyCreditGrant(
	tx *gorm.DB,
	userID uint,
	amount float64,
	entryType, description, stripeEventID, stripeInvoiceID, planType string,
) error {
	if stripeEventID != "" {
		dedup := models.ProcessedStripeEvent{
			StripeEventID: stripeEventID,
			EventType:     entryType,
			ProcessedAt:   time.Now(),
		}
		dedup.UserID = &userID
		result := tx.Create(&dedup)
		if result.Error != nil {
			if isDuplicateCreditInsertError(result.Error) {
				return nil
			}
			return fmt.Errorf("dedup insert failed: %w", result.Error)
		}
	}

	var user models.User
	if err := tx.Select("id, credit_balance").First(&user, userID).Error; err != nil {
		return fmt.Errorf("user lookup in credit tx: %w", err)
	}

	balanceAfter := user.CreditBalance + amount
	entry := models.CreditLedgerEntry{
		UserID:          userID,
		AmountUSD:       amount,
		BalanceAfterUSD: balanceAfter,
		EntryType:       entryType,
		Description:     description,
		StripeEventID:   stripeEventID,
		StripeInvoiceID: stripeInvoiceID,
		PlanType:        planType,
	}
	if err := tx.Create(&entry).Error; err != nil {
		return fmt.Errorf("ledger entry insert failed: %w", err)
	}

	if err := tx.Model(&models.User{}).
		Where("id = ?", userID).
		Update("credit_balance", gorm.Expr("credit_balance + ?", amount)).Error; err != nil {
		return fmt.Errorf("credit_balance update failed: %w", err)
	}

	return nil
}
