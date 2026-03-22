package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"apex-build/pkg/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrTokenExpired           = errors.New("token expired")
	ErrInvalidToken           = errors.New("invalid token")
	ErrTokenBlacklisted       = errors.New("token has been revoked")
	ErrUserNotFound           = errors.New("user not found")
	ErrUserExists             = errors.New("user already exists")
	ErrRefreshTokenUsed       = errors.New("refresh token has already been used")
	ErrRefreshTokenRevoked    = errors.New("refresh token has been revoked")
	ErrRefreshTokenExpired    = errors.New("refresh token has expired")
	ErrRefreshTokenNotFound   = errors.New("refresh token not found")
	ErrTokenFamilyCompromised = errors.New("token family compromised - possible token reuse attack")
)

// TokenBlacklist manages revoked tokens with automatic TTL-based cleanup
type TokenBlacklist struct {
	tokens map[string]time.Time // token -> expiration time
	mu     sync.RWMutex
	stopCh chan struct{}
}

// Global token blacklist instance
var tokenBlacklist *TokenBlacklist
var tokenBlacklistOnce sync.Once

// initTokenBlacklist initializes the global token blacklist with cleanup
func initTokenBlacklist() {
	tokenBlacklistOnce.Do(func() {
		tokenBlacklist = &TokenBlacklist{
			tokens: make(map[string]time.Time),
			stopCh: make(chan struct{}),
		}
		go tokenBlacklist.cleanupRoutine()
	})
}

// Add adds a token to the blacklist with its expiration time
func (tb *TokenBlacklist) Add(token string, expiresAt time.Time) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.tokens[token] = expiresAt
}

// IsBlacklisted checks if a token is in the blacklist
func (tb *TokenBlacklist) IsBlacklisted(token string) bool {
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	_, exists := tb.tokens[token]
	return exists
}

// cleanupRoutine removes expired tokens from the blacklist every 5 minutes
func (tb *TokenBlacklist) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tb.cleanup()
		case <-tb.stopCh:
			return
		}
	}
}

// cleanup removes tokens that have naturally expired
func (tb *TokenBlacklist) cleanup() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	for token, expiresAt := range tb.tokens {
		if now.After(expiresAt) {
			delete(tb.tokens, token)
		}
	}
}

// AuthService handles authentication and authorization
type AuthService struct {
	jwtService      *JWTService
	passwordService *PasswordService
	oauthService    *OAuthService
	jwtSecret       []byte
	refreshSecret   []byte
	tokenExpiry     time.Duration
	refreshExpiry   time.Duration
	bcryptCost      int
	db              *gorm.DB
}

// JWTClaims represents the JWT token claims
type JWTClaims struct {
	UserID              uint   `json:"user_id"`
	Username            string `json:"username"`
	Email               string `json:"email"`
	Role                string `json:"role"`
	SubscriptionType    string `json:"subscription_type"`
	IsAdmin             bool   `json:"is_admin"`
	IsSuperAdmin        bool   `json:"is_super_admin"`
	HasUnlimitedCredits bool   `json:"has_unlimited_credits"`
	BypassBilling       bool   `json:"bypass_billing"`
	BypassRateLimits    bool   `json:"bypass_rate_limits"`
	jwt.RegisteredClaims
}

// PreviewTokenClaims scopes access to preview proxy endpoints for a single project.
type PreviewTokenClaims struct {
	UserID    uint `json:"user_id"`
	ProjectID uint `json:"project_id"`
	jwt.RegisteredClaims
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken           string    `json:"access_token"`
	RefreshToken          string    `json:"refresh_token,omitempty"` // Omit if using cookies
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at,omitempty"`
	TokenType             string    `json:"token_type"`
}

// RefreshTokenMetadata contains metadata for refresh token creation
type RefreshTokenMetadata struct {
	IPAddress string
	UserAgent string
	DeviceID  string
	FamilyID  string // Empty for new family, set to reuse existing family
}

// LoginRequest represents a login request — accepts username or email
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username         string `json:"username" binding:"required,min=3,max=50"`
	Email            string `json:"email" binding:"required,email"`
	Password         string `json:"password" binding:"required,min=8"`
	FullName         string `json:"full_name" binding:"max=100"`
	AcceptLegalTerms bool   `json:"accept_legal_terms"`
	AcceptanceIP     string `json:"-"`
	AcceptanceAgent  string `json:"-"`
}

const CurrentLegalPolicyVersion = "2026-03-21"

// NewAuthService creates a new authentication service with enhanced security
func NewAuthService(jwtSecret string) *AuthService {
	refreshSecret := strings.TrimSpace(os.Getenv("JWT_REFRESH_SECRET"))
	if refreshSecret == "" {
		refreshSecret = jwtSecret + "_refresh" // Backward-compatible fallback for local/test setups
	}

	// Initialize the global token blacklist
	initTokenBlacklist()

	return &AuthService{
		jwtService:      NewJWTService(jwtSecret, refreshSecret, "apex-build"),
		passwordService: NewPasswordService(),
		oauthService:    NewOAuthService(),
		jwtSecret:       []byte(jwtSecret),
		refreshSecret:   []byte(refreshSecret),
		tokenExpiry:     15 * time.Minute,   // Short-lived access tokens
		refreshExpiry:   7 * 24 * time.Hour, // 7 day refresh tokens
		bcryptCost:      12,                 // Strong bcrypt cost (legacy fallback)
		db:              nil,                // Set via SetDB for database-backed refresh tokens
	}
}

// SetDB sets the database connection for refresh token storage.
// Must be called during server startup before any authentication requests are
// processed. Without a database connection, refresh tokens are issued but not
// persisted and cannot be rotated or revoked after a server restart.
func (a *AuthService) SetDB(db *gorm.DB) {
	a.db = db
	log.Println("[auth] database configured — refresh tokens will be persisted and rotatable")
}

// warnRefreshTokenNoDBOnce ensures the no-DB warning is logged at most once per
// process startup so it is visible without flooding the log.
var warnRefreshTokenNoDBOnce sync.Once

func warnRefreshTokenNoDB() {
	warnRefreshTokenNoDBOnce.Do(func() {
		log.Println("[auth] WARNING: AuthService has no database connection — refresh tokens will not be persisted. " +
			"Token rotation and revocation are unavailable until SetDB() is called. " +
			"This is a configuration error in production.")
	})
}

// GetDB returns the database connection
func (a *AuthService) GetDB() *gorm.DB {
	return a.db
}

// HashPassword hashes a password using bcrypt
func (a *AuthService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), a.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword compares a password with its hash
func (a *AuthService) CheckPassword(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// GenerateTokens generates access and refresh tokens for a user
// This is the simple version that doesn't store refresh tokens in the database
func (a *AuthService) GenerateTokens(user *models.User) (*TokenPair, error) {
	return a.GenerateTokensWithMetadata(user, nil)
}

// GenerateTokensWithMetadata generates access and refresh tokens with optional metadata
// If metadata is provided and database is configured, refresh tokens are stored for rotation
func (a *AuthService) GenerateTokensWithMetadata(user *models.User, metadata *RefreshTokenMetadata) (*TokenPair, error) {
	now := time.Now()
	accessExpiresAt := now.Add(a.tokenExpiry)
	refreshExpiresAt := now.Add(a.refreshExpiry)

	// Create access token claims
	accessClaims := &JWTClaims{
		UserID:              user.ID,
		Username:            user.Username,
		Email:               user.Email,
		Role:                a.getUserRole(user),
		SubscriptionType:    user.SubscriptionType,
		IsAdmin:             user.IsAdmin,
		IsSuperAdmin:        user.IsSuperAdmin,
		HasUnlimitedCredits: user.HasUnlimitedCredits,
		BypassBilling:       user.BypassBilling,
		BypassRateLimits:    user.BypassRateLimits,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "apex-build",
			Subject:   fmt.Sprintf("user:%d", user.ID),
			ID:        fmt.Sprintf("access:%d:%d", user.ID, now.Unix()),
		},
	}

	// Generate access token
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(a.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate refresh token - use secure random bytes for better security
	refreshTokenString, err := a.generateSecureRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// If database is configured, store refresh token for rotation.
	// Without a database the refresh token is issued but not persisted — it
	// cannot be rotated or revoked and will stop working on server restart.
	// Call AuthService.SetDB() during server startup to enable full rotation.
	if a.db == nil {
		warnRefreshTokenNoDB()
	}
	if a.db != nil {
		familyID := generateUUID()
		if metadata != nil && metadata.FamilyID != "" {
			familyID = metadata.FamilyID
		}

		tokenHash := hashToken(refreshTokenString)

		refreshTokenRecord := &models.RefreshToken{
			Token:     tokenHash,
			TokenHash: tokenHash,
			UserID:    user.ID,
			ExpiresAt: refreshExpiresAt,
			IssuedAt:  now,
			Used:      false,
			Revoked:   false,
			FamilyID:  familyID,
		}

		if metadata != nil {
			refreshTokenRecord.IPAddress = metadata.IPAddress
			refreshTokenRecord.UserAgent = metadata.UserAgent
			refreshTokenRecord.DeviceID = metadata.DeviceID
		}

		if err := a.db.Create(refreshTokenRecord).Error; err != nil {
			return nil, fmt.Errorf("failed to store refresh token: %w", err)
		}
	}

	return &TokenPair{
		AccessToken:           accessTokenString,
		RefreshToken:          refreshTokenString,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshTokenExpiresAt: refreshExpiresAt,
		TokenType:             "Bearer",
	}, nil
}

// generateSecureRefreshToken generates a cryptographically secure refresh token
func (a *AuthService) generateSecureRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// hashToken creates a SHA-256 hash of a token for secure storage and lookup
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// generateUUID generates a simple UUID for token family tracking
func generateUUID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// ValidateToken validates and parses a JWT token
func (a *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	// Check if token is blacklisted (revoked on logout)
	if tokenBlacklist != nil && tokenBlacklist.IsBlacklisted(tokenString) {
		return nil, ErrTokenBlacklisted
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.jwtSecret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GeneratePreviewToken creates a short-lived token scoped to preview access for a single project.
func (a *AuthService) GeneratePreviewToken(userID, projectID uint, ttl time.Duration) (string, error) {
	if userID == 0 || projectID == 0 {
		return "", fmt.Errorf("preview token requires both user_id and project_id")
	}
	if ttl <= 0 {
		ttl = time.Hour
	}

	now := time.Now()
	claims := &PreviewTokenClaims{
		UserID:    userID,
		ProjectID: projectID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("preview:%d:%d", userID, projectID),
			Audience:  jwt.ClaimStrings{"preview"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret)
}

// ValidatePreviewToken validates a preview-scoped JWT and optionally binds it to a project.
func (a *AuthService) ValidatePreviewToken(tokenString string, projectID uint) (*PreviewTokenClaims, error) {
	claims := &PreviewTokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.jwtSecret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	hasPreviewAudience := false
	for _, audience := range claims.Audience {
		if audience == "preview" {
			hasPreviewAudience = true
			break
		}
	}
	if !hasPreviewAudience {
		return nil, ErrInvalidToken
	}
	if projectID != 0 && claims.ProjectID != projectID {
		return nil, ErrInvalidToken
	}
	if claims.UserID == 0 || claims.ProjectID == 0 {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// BlacklistToken adds a token to the blacklist to prevent reuse after logout
// The token remains blacklisted until its natural expiration time
func (a *AuthService) BlacklistToken(tokenString string) error {
	if tokenBlacklist == nil {
		initTokenBlacklist()
	}

	// Parse token to get expiration time (don't validate since we're blacklisting)
	token, _ := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return a.jwtSecret, nil
	})

	var expiresAt time.Time
	if token != nil {
		if claims, ok := token.Claims.(*JWTClaims); ok && claims.ExpiresAt != nil {
			expiresAt = claims.ExpiresAt.Time
		}
	}

	// If we couldn't parse expiration, use default token expiry
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(a.tokenExpiry)
	}

	tokenBlacklist.Add(tokenString, expiresAt)
	return nil
}

// RefreshTokens generates new tokens using a refresh token (legacy JWT-based)
func (a *AuthService) RefreshTokens(refreshToken string, user *models.User) (*TokenPair, error) {
	// First try database-backed rotation if available
	if a.db != nil {
		return a.RotateRefreshToken(refreshToken, nil)
	}

	// Fall back to JWT-based refresh tokens
	claims, err := a.ValidateToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Verify that this is actually a refresh token
	if claims.ID == "" || len(claims.ID) < 7 || claims.ID[:7] != "refresh" {
		return nil, ErrInvalidToken
	}

	// Verify user matches
	if claims.UserID != user.ID {
		return nil, ErrInvalidToken
	}

	// Generate new token pair
	return a.GenerateTokens(user)
}

// RotateRefreshToken validates a refresh token, marks it as used, and issues new tokens
// This implements secure token rotation - each refresh token can only be used once
func (a *AuthService) RotateRefreshToken(refreshToken string, metadata *RefreshTokenMetadata) (*TokenPair, error) {
	if a.db == nil {
		return nil, errors.New("database not configured for refresh token rotation")
	}

	tokenHash := hashToken(refreshToken)

	// Find the refresh token in database
	var storedToken models.RefreshToken
	err := a.db.Where("token_hash = ?", tokenHash).First(&storedToken).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("failed to lookup refresh token: %w", err)
	}

	// Check if token has been revoked
	if storedToken.Revoked {
		return nil, ErrRefreshTokenRevoked
	}

	// Check if token has expired
	if time.Now().After(storedToken.ExpiresAt) {
		return nil, ErrRefreshTokenExpired
	}

	// CRITICAL: Check if token has already been used (token reuse attack detection)
	if storedToken.Used {
		// Token reuse detected! This could indicate a token theft
		// Revoke the entire token family to protect the user
		if err := a.RevokeTokenFamily(storedToken.FamilyID); err != nil {
			// Log the error but continue with the security response
			fmt.Printf("Failed to revoke token family %s: %v\n", storedToken.FamilyID, err)
		}
		return nil, ErrTokenFamilyCompromised
	}

	// Get the user
	var user models.User
	if err := a.db.First(&user, storedToken.UserID).Error; err != nil {
		return nil, ErrUserNotFound
	}

	// Check if user is still active
	if !user.IsActive {
		return nil, errors.New("user account is disabled")
	}

	// Mark the current token as used (atomically to prevent race conditions)
	now := time.Now()
	result := a.db.Model(&models.RefreshToken{}).
		Where("id = ? AND used = ?", storedToken.ID, false).
		Updates(map[string]interface{}{
			"used":    true,
			"used_at": now,
		})

	if result.Error != nil {
		return nil, fmt.Errorf("failed to mark token as used: %w", result.Error)
	}

	// If no rows were updated, another request beat us to it
	if result.RowsAffected == 0 {
		return nil, ErrRefreshTokenUsed
	}

	// Generate new tokens with the same family ID (for tracking)
	newMetadata := &RefreshTokenMetadata{
		FamilyID: storedToken.FamilyID,
	}
	if metadata != nil {
		newMetadata.IPAddress = metadata.IPAddress
		newMetadata.UserAgent = metadata.UserAgent
		newMetadata.DeviceID = metadata.DeviceID
	} else {
		// Preserve original metadata
		newMetadata.IPAddress = storedToken.IPAddress
		newMetadata.UserAgent = storedToken.UserAgent
		newMetadata.DeviceID = storedToken.DeviceID
	}

	return a.GenerateTokensWithMetadata(&user, newMetadata)
}

// RevokeRefreshToken revokes a specific refresh token
func (a *AuthService) RevokeRefreshToken(refreshToken string) error {
	if a.db == nil {
		return errors.New("database not configured for refresh token management")
	}

	tokenHash := hashToken(refreshToken)
	now := time.Now()

	result := a.db.Model(&models.RefreshToken{}).
		Where("token_hash = ?", tokenHash).
		Updates(map[string]interface{}{
			"revoked":    true,
			"revoked_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", result.Error)
	}

	return nil
}

// RevokeUserRefreshTokens revokes all refresh tokens for a user (logout from all devices)
func (a *AuthService) RevokeUserRefreshTokens(userID uint) error {
	if a.db == nil {
		return errors.New("database not configured for refresh token management")
	}

	now := time.Now()
	result := a.db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked = ?", userID, false).
		Updates(map[string]interface{}{
			"revoked":    true,
			"revoked_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to revoke user refresh tokens: %w", result.Error)
	}

	return nil
}

// RevokeTokenFamily revokes all refresh tokens in a token family
// Used when token reuse is detected (potential security breach)
func (a *AuthService) RevokeTokenFamily(familyID string) error {
	if a.db == nil {
		return errors.New("database not configured for refresh token management")
	}

	now := time.Now()
	result := a.db.Model(&models.RefreshToken{}).
		Where("family_id = ? AND revoked = ?", familyID, false).
		Updates(map[string]interface{}{
			"revoked":    true,
			"revoked_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to revoke token family: %w", result.Error)
	}

	return nil
}

// ValidateRefreshToken validates a refresh token without rotating it
func (a *AuthService) ValidateRefreshToken(refreshToken string) (*models.RefreshToken, error) {
	if a.db == nil {
		return nil, errors.New("database not configured for refresh token management")
	}

	tokenHash := hashToken(refreshToken)

	var storedToken models.RefreshToken
	err := a.db.Where("token_hash = ?", tokenHash).First(&storedToken).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("failed to lookup refresh token: %w", err)
	}

	if storedToken.Revoked {
		return nil, ErrRefreshTokenRevoked
	}

	if storedToken.Used {
		return nil, ErrRefreshTokenUsed
	}

	if time.Now().After(storedToken.ExpiresAt) {
		return nil, ErrRefreshTokenExpired
	}

	return &storedToken, nil
}

// CleanupExpiredRefreshTokens removes expired refresh tokens from the database
func (a *AuthService) CleanupExpiredRefreshTokens() (int64, error) {
	if a.db == nil {
		return 0, errors.New("database not configured for refresh token management")
	}

	result := a.db.Where("expires_at < ?", time.Now()).Delete(&models.RefreshToken{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup expired refresh tokens: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// GetUserRefreshTokens gets all active refresh tokens for a user (for session management UI)
func (a *AuthService) GetUserRefreshTokens(userID uint) ([]models.RefreshToken, error) {
	if a.db == nil {
		return nil, errors.New("database not configured for refresh token management")
	}

	var tokens []models.RefreshToken
	err := a.db.Where("user_id = ? AND revoked = ? AND used = ? AND expires_at > ?",
		userID, false, false, time.Now()).
		Order("created_at DESC").
		Find(&tokens).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get user refresh tokens: %w", err)
	}

	// Clear sensitive data before returning
	for i := range tokens {
		tokens[i].Token = ""
		tokens[i].TokenHash = ""
	}

	return tokens, nil
}

// getUserRole determines the user's role based on subscription type
func (a *AuthService) getUserRole(user *models.User) string {
	switch user.SubscriptionType {
	case "team":
		return "team"
	case "pro":
		return "pro"
	default:
		return "free"
	}
}

// ValidateRegistration validates registration data
func (a *AuthService) ValidateRegistration(req *RegisterRequest) error {
	if len(req.Username) < 3 || len(req.Username) > 50 {
		return errors.New("username must be between 3 and 50 characters")
	}

	if len(req.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	// Add more validation rules as needed
	if len(req.FullName) > 100 {
		return errors.New("full name must be less than 100 characters")
	}

	if !req.AcceptLegalTerms {
		return errors.New("you must accept the terms, privacy policy, and platform policies")
	}

	return nil
}

// CreateUser creates a new user with hashed password
func (a *AuthService) CreateUser(req *RegisterRequest) (*models.User, error) {
	if err := a.ValidateRegistration(req); err != nil {
		return nil, err
	}

	hashedPassword, err := a.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Username:             req.Username,
		Email:                req.Email,
		PasswordHash:         hashedPassword,
		FullName:             req.FullName,
		IsActive:             true,
		IsVerified:           false,
		SubscriptionType:     "free",
		HasUnlimitedCredits:  false,
		PreferredTheme:       "cyberpunk",
		PreferredAI:          "auto",
		LegalAcceptedAt:      func() *time.Time { now := time.Now().UTC(); return &now }(),
		LegalPolicyVersion:   CurrentLegalPolicyVersion,
		LegalAcceptanceIP:    req.AcceptanceIP,
		LegalAcceptanceAgent: req.AcceptanceAgent,
	}

	return user, nil
}

// AuthenticateUser authenticates a user with username/password
func (a *AuthService) AuthenticateUser(username, password, userHash string) error {
	return a.CheckPassword(password, userHash)
}

// Middleware functions for Gin

// RequireAuth middleware that requires authentication
func (a *AuthService) RequireAuth() func(c interface{}) {
	// This would be implemented with Gin context
	// For now, returning a placeholder
	return func(c interface{}) {
		// Implementation would go here
	}
}

// RequireRole middleware that requires specific role
func (a *AuthService) RequireRole(role string) func(c interface{}) {
	return func(c interface{}) {
		// Implementation would go here
	}
}

// ExtractUserFromToken extracts user information from token
func (a *AuthService) ExtractUserFromToken(tokenString string) (uint, error) {
	claims, err := a.ValidateToken(tokenString)
	if err != nil {
		return 0, err
	}

	return claims.UserID, nil
}

// TokenInfo represents token information for API responses
type TokenInfo struct {
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	IssuedAt  time.Time `json:"issued_at"`
}

// GetTokenInfo returns information about a token
func (a *AuthService) GetTokenInfo(tokenString string) (*TokenInfo, error) {
	claims, err := a.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	return &TokenInfo{
		UserID:    claims.UserID,
		Username:  claims.Username,
		Email:     claims.Email,
		Role:      claims.Role,
		ExpiresAt: claims.ExpiresAt.Time,
		IssuedAt:  claims.IssuedAt.Time,
	}, nil
}

// PasswordStrengthCheck checks password strength
func (a *AuthService) PasswordStrengthCheck(password string) (bool, []string) {
	var issues []string

	if len(password) < 8 {
		issues = append(issues, "Password must be at least 8 characters long")
	}

	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasNumber = true
		case char >= 33 && char <= 126: // Printable ASCII special characters
			if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')) {
				hasSpecial = true
			}
		}
	}

	if !hasUpper {
		issues = append(issues, "Password must contain at least one uppercase letter")
	}
	if !hasLower {
		issues = append(issues, "Password must contain at least one lowercase letter")
	}
	if !hasNumber {
		issues = append(issues, "Password must contain at least one number")
	}
	if !hasSpecial {
		issues = append(issues, "Password must contain at least one special character")
	}

	return len(issues) == 0, issues
}
