package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"apex-build/internal/config"

	"gorm.io/gorm"
)

const (
	adminPromotionTokenMinLength = 32
	adminPromotionMaxWindow      = 24 * time.Hour
)

// adminPromotionExpectedTokenDigest returns the lowercase hex SHA-256 digest
// of the expected ADMIN_PROMOTE_TOKEN, read from ADMIN_PROMOTE_TOKEN_DIGEST.
// Operators set the digest in protected secret storage so the cleartext token
// never has to live alongside the digest in the same env.
func adminPromotionExpectedTokenDigest() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_PROMOTE_TOKEN_DIGEST")))
}

// adminPromotionTokenDigest returns the lowercase hex SHA-256 of the supplied
// token, used to compare against the operator-installed expected digest.
func adminPromotionTokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func runAdminPromotions(db *gorm.DB, now time.Time) error {
	emails := parseAdminPromotionEmails(os.Getenv("ADMIN_PROMOTE_EMAIL"))
	if len(emails) == 0 {
		return nil
	}

	env := config.GetEnvironment()
	if adminPromotionRequiresGuard(env) {
		if err := validateAdminPromotionGuard(env, os.Getenv("ADMIN_PROMOTE_TOKEN"), os.Getenv("ADMIN_PROMOTE_EXPIRES_AT"), now); err != nil {
			return err
		}
	}

	for _, email := range emails {
		res := db.Exec(
			`UPDATE users SET is_admin = true, is_super_admin = true WHERE email = ?`,
			email,
		)
		if res.Error != nil {
			log.Printf("WARNING: admin promotion for %s failed: %v", email, res.Error)
			continue
		}
		log.Printf("ADMIN_PROMOTE_AUDIT: granted is_admin+is_super_admin to %s (%d row(s) updated, env=%s)", email, res.RowsAffected, env)
	}
	return nil
}

func parseAdminPromotionEmails(raw string) []string {
	parts := strings.Split(raw, ",")
	emails := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		email := strings.ToLower(strings.TrimSpace(part))
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		emails = append(emails, email)
	}
	return emails
}

func adminPromotionRequiresGuard(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "production", "prod", "staging", "stage":
		return true
	default:
		return false
	}
}

func validateAdminPromotionGuard(environment, token, expiresAt string, now time.Time) error {
	if !adminPromotionRequiresGuard(environment) {
		return nil
	}
	trimmedToken := strings.TrimSpace(token)
	if len(trimmedToken) < adminPromotionTokenMinLength {
		return fmt.Errorf("ADMIN_PROMOTE_EMAIL is set in %s but ADMIN_PROMOTE_TOKEN is missing or shorter than %d characters", environment, adminPromotionTokenMinLength)
	}

	// Token must match a pre-installed digest. Without this comparison, the
	// length-only gate is defense-by-inconvenience: anyone with env access
	// could set any 32-char string. Requiring an out-of-band digest means
	// even env-level access cannot trigger admin grants without knowledge
	// of the original token.
	expectedDigest := adminPromotionExpectedTokenDigest()
	if expectedDigest == "" {
		return fmt.Errorf("ADMIN_PROMOTE_EMAIL is set in %s but ADMIN_PROMOTE_TOKEN_DIGEST is missing — refuse to grant admin without an out-of-band digest match", environment)
	}
	actualDigest := adminPromotionTokenDigest(trimmedToken)
	if subtle.ConstantTimeCompare([]byte(actualDigest), []byte(expectedDigest)) != 1 {
		return fmt.Errorf("ADMIN_PROMOTE_TOKEN does not match ADMIN_PROMOTE_TOKEN_DIGEST")
	}

	rawExpiresAt := strings.TrimSpace(expiresAt)
	if rawExpiresAt == "" {
		return fmt.Errorf("ADMIN_PROMOTE_EMAIL is set in %s but ADMIN_PROMOTE_EXPIRES_AT is missing", environment)
	}
	parsedExpiresAt, err := time.Parse(time.RFC3339, rawExpiresAt)
	if err != nil {
		return fmt.Errorf("ADMIN_PROMOTE_EXPIRES_AT must be RFC3339: %w", err)
	}
	if !parsedExpiresAt.After(now) {
		return fmt.Errorf("ADMIN_PROMOTE_EXPIRES_AT is expired")
	}
	if parsedExpiresAt.Sub(now) > adminPromotionMaxWindow {
		return fmt.Errorf("ADMIN_PROMOTE_EXPIRES_AT must be within %s", adminPromotionMaxWindow)
	}
	return nil
}
