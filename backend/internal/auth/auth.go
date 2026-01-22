package auth

import (
	"errors"
	"fmt"
	"time"

	"apex-build/pkg/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired      = errors.New("token expired")
	ErrInvalidToken      = errors.New("invalid token")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("user already exists")
)

// AuthService handles authentication and authorization
type AuthService struct {
	jwtSecret       []byte
	tokenExpiry     time.Duration
	refreshExpiry   time.Duration
	bcryptCost      int
}

// JWTClaims represents the JWT token claims
type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"max=100"`
}

// NewAuthService creates a new authentication service
func NewAuthService(jwtSecret string) *AuthService {
	return &AuthService{
		jwtSecret:     []byte(jwtSecret),
		tokenExpiry:   24 * time.Hour,    // Access token expires in 24 hours
		refreshExpiry: 30 * 24 * time.Hour, // Refresh token expires in 30 days
		bcryptCost:    12,               // Strong bcrypt cost
	}
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
func (a *AuthService) GenerateTokens(user *models.User) (*TokenPair, error) {
	now := time.Now()
	expiresAt := now.Add(a.tokenExpiry)

	// Create access token claims
	accessClaims := &JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     a.getUserRole(user),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "apex-build",
			Subject:   fmt.Sprintf("user:%d", user.ID),
			ID:        fmt.Sprintf("access:%d:%d", user.ID, now.Unix()),
		},
	}

	// Create refresh token claims
	refreshClaims := &JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     a.getUserRole(user),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(a.refreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "apex-build",
			Subject:   fmt.Sprintf("user:%d", user.ID),
			ID:        fmt.Sprintf("refresh:%d:%d", user.ID, now.Unix()),
		},
	}

	// Generate tokens
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)

	accessTokenString, err := accessToken.SignedString(a.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	refreshTokenString, err := refreshToken.SignedString(a.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresAt:    expiresAt,
		TokenType:    "Bearer",
	}, nil
}

// ValidateToken validates and parses a JWT token
func (a *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
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

// RefreshTokens generates new tokens using a refresh token
func (a *AuthService) RefreshTokens(refreshToken string, user *models.User) (*TokenPair, error) {
	claims, err := a.ValidateToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Verify that this is actually a refresh token
	if claims.ID == "" || claims.ID[:7] != "refresh" {
		return nil, ErrInvalidToken
	}

	// Verify user matches
	if claims.UserID != user.ID {
		return nil, ErrInvalidToken
	}

	// Generate new token pair
	return a.GenerateTokens(user)
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
		Username:         req.Username,
		Email:           req.Email,
		PasswordHash:    hashedPassword,
		FullName:        req.FullName,
		IsActive:        true,
		IsVerified:      false,
		SubscriptionType: "free",
		PreferredTheme:  "cyberpunk",
		PreferredAI:     "auto",
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