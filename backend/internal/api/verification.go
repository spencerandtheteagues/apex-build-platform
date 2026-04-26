// verification.go — Email verification handlers and helpers.
//
// Flow:
//  1. User registers → tokens issued immediately → verification code emailed.
//  2. Frontend shows 6-digit code entry screen.
//  3. POST /auth/verify-email (authenticated) → validates code → marks user verified.
//  4. POST /auth/resend-verification (auth or body email) → sends a fresh code.
//  5. Login blocks unverified users and surfaces "needs_verification" error_code.
package api

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"apex-build/internal/auth"
	"apex-build/internal/email"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	verificationCodeTTL    = 15 * time.Minute
	verificationResendCool = 60 * time.Second // minimum gap between resends
)

// SetEmailService wires an email service into the Server.
func (s *Server) SetEmailService(svc *email.Service) {
	s.email = svc
}

// generateVerificationCode returns a zero-padded 6-digit code string.
func generateVerificationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func normalizeVerificationCode(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func verificationEmailLookup(raw string) (string, string, bool) {
	emailAddr := strings.ToLower(strings.TrimSpace(raw))
	if emailAddr == "" {
		return "", "", false
	}
	return "LOWER(email) = LOWER(?)", emailAddr, true
}

func shouldIssueVerificationCode(user *models.User, now time.Time) bool {
	if user == nil || user.IsVerified || user.EmailVerifiedAt != nil {
		return false
	}
	if user.VerificationCode == "" || user.VerificationCodeExpiresAt == nil {
		return true
	}
	return !now.Before(*user.VerificationCodeExpiresAt)
}

func (s *Server) issueVerificationCodeIfNeeded(user *models.User) error {
	if !shouldIssueVerificationCode(user, time.Now()) {
		return nil
	}
	return s.issueVerificationCode(user)
}

// issueVerificationCode generates a new code, hashes it, persists it to the
// user record, and sends the email. Email send failure does NOT abort the
// operation; we still persist the hashed code so a resend can be triggered.
func (s *Server) issueVerificationCode(user *models.User) error {
	code, err := generateVerificationCode()
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.MinCost)
	if err != nil {
		return fmt.Errorf("hash code: %w", err)
	}

	expires := time.Now().Add(verificationCodeTTL)
	if err := s.db.DB.Model(user).Updates(map[string]interface{}{
		"verification_code":            string(hashed),
		"verification_code_expires_at": expires,
	}).Error; err != nil {
		return fmt.Errorf("persist code: %w", err)
	}

	if s.email != nil {
		if err := s.email.SendVerificationCode(user.Email, user.Username, code); err != nil {
			log.Printf("[verification] email send failed for user %d: %v", user.ID, err)
			// non-fatal — code is stored, user can resend
		}
	} else {
		// No email service configured — log the code in dev so it's usable.
		log.Printf("[verification] DEV: code for user %s (%s): %s", user.Username, user.Email, code)
	}
	return nil
}

// VerifyEmail validates the 6-digit code submitted by an authenticated or unauthenticated user.
//
// POST /api/v1/auth/verify-email
// Body (authenticated): {"code": "847392"}
// Body (unauthenticated): {"email": "user@example.com", "code": "847392"}
// Auth: Bearer / cookie optional — if absent, email must be in the body.
func (s *Server) VerifyEmail(c *gin.Context) {
	var req struct {
		Code  string `json:"code"  binding:"required"`
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}
	code := normalizeVerificationCode(req.Code)
	if len(code) != 6 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Invalid verification code",
			"error_code": "code_invalid",
		})
		return
	}

	var user models.User
	if userID, ok := getAuthUserID(c); ok {
		// Authenticated path — look up by token user ID
		if err := s.db.DB.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
	} else {
		// Unauthenticated path — require email in body
		clause, emailAddr, ok := verificationEmailLookup(req.Email)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication or email required"})
			return
		}
		if err := s.db.DB.Where(clause, emailAddr).First(&user).Error; err != nil {
			// Return same error to avoid enumeration
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":      "Invalid verification code",
				"error_code": "code_invalid",
			})
			return
		}
	}

	if user.EmailVerifiedAt != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Email already verified"})
		return
	}

	if user.VerificationCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No verification code issued — please request a new one"})
		return
	}

	if user.VerificationCodeExpiresAt == nil || time.Now().After(*user.VerificationCodeExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Verification code has expired",
			"error_code": "code_expired",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.VerificationCode), []byte(code)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Invalid verification code",
			"error_code": "code_invalid",
		})
		return
	}

	now := time.Now()
	if err := s.db.DB.Model(&user).Updates(map[string]interface{}{
		"email_verified_at":            now,
		"is_verified":                  true,
		"verification_code":            "",
		"verification_code_expires_at": nil,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
		return
	}

	log.Printf("[verification] user %d (%s) verified email", user.ID, user.Username)

	// Issue fresh tokens so the client can proceed directly to the dashboard
	// without a separate login step (especially important for the unauthenticated path).
	resp := gin.H{
		"message":           "Email verified successfully",
		"email_verified_at": now,
	}
	if tokens, err := s.auth.GenerateTokens(&user); err == nil {
		auth.SetAccessTokenCookie(c, tokens.AccessToken)
		auth.SetRefreshTokenCookie(c, tokens.RefreshToken)
		resp["access_token"] = tokens.AccessToken
		resp["refresh_token"] = tokens.RefreshToken
		resp["user"] = gin.H{
			"id":                    user.ID,
			"username":              user.Username,
			"email":                 user.Email,
			"full_name":             user.FullName,
			"is_admin":              user.IsAdmin,
			"is_super_admin":        user.IsSuperAdmin,
			"has_unlimited_credits": user.HasUnlimitedCredits,
			"subscription_type":     user.SubscriptionType,
			"credit_balance":        user.CreditBalance,
		}
	}
	c.JSON(http.StatusOK, resp)
}

// ResendVerification issues a fresh verification code.
//
// POST /api/v1/auth/resend-verification
// Body (unauthenticated path): {"email": "user@example.com"}
// Auth: optional — if authenticated, uses the token user; otherwise uses body email.
func (s *Server) ResendVerification(c *gin.Context) {
	var user models.User

	// Try authenticated path first.
	if userID, ok := getAuthUserID(c); ok {
		if err := s.db.DB.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
	} else {
		// Unauthenticated: look up by email from body.
		var req struct {
			Email string `json:"email" binding:"required,email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
			return
		}
		clause, emailAddr, ok := verificationEmailLookup(req.Email)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
			return
		}
		if err := s.db.DB.Where(clause, emailAddr).First(&user).Error; err != nil {
			// Return 200 to avoid user enumeration.
			c.JSON(http.StatusOK, gin.H{"message": "If that email is registered and unverified, a code has been sent"})
			return
		}
	}

	if user.EmailVerifiedAt != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Email is already verified"})
		return
	}

	// Rate-limit resends.
	if user.VerificationCodeExpiresAt != nil {
		remaining := time.Until(user.VerificationCodeExpiresAt.Add(-verificationCodeTTL + verificationResendCool))
		if remaining > 0 {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":         fmt.Sprintf("Please wait %d seconds before requesting a new code", int(remaining.Seconds())+1),
				"retry_after_s": int(remaining.Seconds()) + 1,
			})
			return
		}
	}

	if err := s.issueVerificationCode(&user); err != nil {
		log.Printf("[verification] resend failed for user %d: %v", user.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification code"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Verification code sent to %s", maskEmail(user.Email)),
	})
}

// getAuthUserID extracts the authenticated user ID from the Gin context.
// Returns (id, true) on success.
func getAuthUserID(c *gin.Context) (uint, bool) {
	raw, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	switch v := raw.(type) {
	case uint:
		return v, true
	case float64:
		return uint(v), true
	case int:
		return uint(v), true
	}
	return 0, false
}

// maskEmail partially redacts an email address for display in responses.
// "spencer@example.com" → "sp****@example.com"
func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "****"
	}
	local := parts[0]
	if len(local) <= 2 {
		return "**@" + parts[1]
	}
	return local[:2] + strings.Repeat("*", len(local)-2) + "@" + parts[1]
}
