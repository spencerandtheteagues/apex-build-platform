package config

import (
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"apex-build/internal/secrets"

	"gorm.io/gorm"
)

// RotationResult tracks the outcome of a key rotation
type RotationResult struct {
	TotalSecrets    int       `json:"total_secrets"`
	Migrated        int       `json:"migrated"`
	Failed          int       `json:"failed"`
	Errors          []string  `json:"errors,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
	DurationSeconds float64   `json:"duration_seconds"`
}

// RotateMasterKey re-encrypts all user secrets from oldKey to newKey.
// This must be run as a one-time migration when rotating SECRETS_MASTER_KEY.
//
// Steps:
//  1. Create SecretsManager with old key
//  2. Create SecretsManager with new key
//  3. For each encrypted secret in DB: decrypt with old, re-encrypt with new, update row
//  4. All done in a transaction for atomicity
func RotateMasterKey(db *gorm.DB, oldKeyBase64, newKeyBase64 string) (*RotationResult, error) {
	result := &RotationResult{
		StartedAt: time.Now(),
	}

	// Validate both keys
	if _, err := base64.StdEncoding.DecodeString(oldKeyBase64); err != nil {
		return nil, fmt.Errorf("invalid old key (not valid base64): %w", err)
	}
	if _, err := base64.StdEncoding.DecodeString(newKeyBase64); err != nil {
		return nil, fmt.Errorf("invalid new key (not valid base64): %w", err)
	}

	oldManager, err := secrets.NewSecretsManager(oldKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to init old key manager: %w", err)
	}

	newManager, err := secrets.NewSecretsManager(newKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to init new key manager: %w", err)
	}

	// Fetch all secrets
	var allSecrets []secrets.Secret
	if err := db.Find(&allSecrets).Error; err != nil {
		return nil, fmt.Errorf("failed to query secrets: %w", err)
	}

	result.TotalSecrets = len(allSecrets)
	log.Printf("Key rotation: migrating %d secrets", result.TotalSecrets)

	// Process in a transaction
	txErr := db.Transaction(func(tx *gorm.DB) error {
		for i, s := range allSecrets {
			// Decrypt with old key
			plaintext, err := oldManager.Decrypt(s.UserID, s.EncryptedValue, s.Salt)
			if err != nil {
				errMsg := fmt.Sprintf("secret %d (user %d, name %q): decrypt failed: %v", s.ID, s.UserID, s.Name, err)
				result.Errors = append(result.Errors, errMsg)
				result.Failed++
				log.Printf("Key rotation WARNING: %s", errMsg)
				continue
			}

			// Re-encrypt with new key
			newEncrypted, newSalt, newFingerprint, err := newManager.Encrypt(s.UserID, plaintext)
			if err != nil {
				errMsg := fmt.Sprintf("secret %d (user %d, name %q): re-encrypt failed: %v", s.ID, s.UserID, s.Name, err)
				result.Errors = append(result.Errors, errMsg)
				result.Failed++
				log.Printf("Key rotation WARNING: %s", errMsg)
				continue
			}

			// Update in DB
			if err := tx.Model(&secrets.Secret{}).Where("id = ?", s.ID).Updates(map[string]interface{}{
				"encrypted_value": newEncrypted,
				"salt":            newSalt,
				"key_fingerprint": newFingerprint,
				"updated_at":      time.Now(),
			}).Error; err != nil {
				errMsg := fmt.Sprintf("secret %d: DB update failed: %v", s.ID, err)
				result.Errors = append(result.Errors, errMsg)
				result.Failed++
				continue
			}

			result.Migrated++

			if (i+1)%100 == 0 {
				log.Printf("Key rotation: %d/%d secrets migrated", result.Migrated, result.TotalSecrets)
			}
		}

		// If more than 10% failed, roll back the entire transaction
		if result.TotalSecrets > 0 && float64(result.Failed)/float64(result.TotalSecrets) > 0.1 {
			return fmt.Errorf("too many failures (%d/%d) - rolling back", result.Failed, result.TotalSecrets)
		}

		return nil
	})

	result.CompletedAt = time.Now()
	result.DurationSeconds = result.CompletedAt.Sub(result.StartedAt).Seconds()

	if txErr != nil {
		return result, fmt.Errorf("key rotation failed (transaction rolled back): %w", txErr)
	}

	log.Printf("Key rotation complete: %d migrated, %d failed out of %d total (%.1fs)",
		result.Migrated, result.Failed, result.TotalSecrets, result.DurationSeconds)

	return result, nil
}

// ValidateRotation verifies that all secrets can be decrypted with the new key
func ValidateRotation(db *gorm.DB, newKeyBase64 string) (int, int, error) {
	newManager, err := secrets.NewSecretsManager(newKeyBase64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to init new key manager: %w", err)
	}

	var allSecrets []secrets.Secret
	if err := db.Find(&allSecrets).Error; err != nil {
		return 0, 0, fmt.Errorf("failed to query secrets: %w", err)
	}

	ok, fail := 0, 0
	for _, s := range allSecrets {
		if _, err := newManager.Decrypt(s.UserID, s.EncryptedValue, s.Salt); err != nil {
			fail++
		} else {
			ok++
		}
	}

	return ok, fail, nil
}
