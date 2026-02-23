package auth

import (
	"testing"
	"time"

	"apex-build/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuthService(t *testing.T) {
	authService := NewAuthService("test-secret-key")

	assert.NotNil(t, authService)
	assert.NotNil(t, authService.jwtService)
	assert.NotNil(t, authService.passwordService)
	assert.NotNil(t, authService.oauthService)
	assert.Equal(t, 15*time.Minute, authService.tokenExpiry)
	assert.Equal(t, 7*24*time.Hour, authService.refreshExpiry)
	assert.Equal(t, 12, authService.bcryptCost)
}

func TestHashPassword(t *testing.T) {
	authService := NewAuthService("test-secret")

	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "normal password",
			password: "SecurePassword123!",
			wantErr:  false,
		},
		{
			name:     "short password",
			password: "short",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false,
		},
		{
			// bcrypt has a 72-byte limit; passwords exceeding this should return an error
			// rather than silently truncating (which would be a security issue)
			name:     "very long password",
			password: "VeryLongPasswordThatShouldStillWork!@#$%^&*()1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz",
			wantErr:  true, // bcrypt 72-byte limit exceeded
		},
		{
			name:     "password with special characters",
			password: "P@$$w0rd!#%^&*()",
			wantErr:  false,
		},
		{
			name:     "unicode password",
			password: "PassWithEmoji!",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := authService.HashPassword(tt.password)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, hash)
			assert.NotEqual(t, tt.password, hash) // Hash should be different from password

			// Verify the hash can be validated
			err = authService.CheckPassword(tt.password, hash)
			assert.NoError(t, err)
		})
	}
}

func TestCheckPassword(t *testing.T) {
	authService := NewAuthService("test-secret")

	// Create a known hash
	password := "TestPassword123!"
	hash, err := authService.HashPassword(password)
	require.NoError(t, err)

	tests := []struct {
		name        string
		password    string
		hash        string
		expectError bool
	}{
		{
			name:        "correct password",
			password:    password,
			hash:        hash,
			expectError: false,
		},
		{
			name:        "wrong password",
			password:    "WrongPassword123!",
			hash:        hash,
			expectError: true,
		},
		{
			name:        "empty password",
			password:    "",
			hash:        hash,
			expectError: true,
		},
		{
			name:        "empty hash",
			password:    password,
			hash:        "",
			expectError: true,
		},
		{
			name:        "invalid hash",
			password:    password,
			hash:        "not-a-valid-hash",
			expectError: true,
		},
		{
			name:        "similar password",
			password:    "TestPassword123",
			hash:        hash,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authService.CheckPassword(tt.password, tt.hash)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, ErrInvalidCredentials, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerateTokens(t *testing.T) {
	authService := NewAuthService("test-secret-key-for-tokens")

	tests := []struct {
		name    string
		user    *models.User
		wantErr bool
	}{
		{
			name: "valid user - free tier",
			user: &models.User{
				ID:               1,
				Username:         "testuser",
				Email:            "test@example.com",
				SubscriptionType: "free",
			},
			wantErr: false,
		},
		{
			name: "valid user - pro tier",
			user: &models.User{
				ID:               2,
				Username:         "prouser",
				Email:            "pro@example.com",
				SubscriptionType: "pro",
			},
			wantErr: false,
		},
		{
			name: "valid user - team tier",
			user: &models.User{
				ID:               3,
				Username:         "teamuser",
				Email:            "team@example.com",
				SubscriptionType: "team",
			},
			wantErr: false,
		},
		{
			name: "user with zero ID",
			user: &models.User{
				ID:               0,
				Username:         "zerouser",
				Email:            "zero@example.com",
				SubscriptionType: "free",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := authService.GenerateTokens(tt.user)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, tokens)

			assert.NotEmpty(t, tokens.AccessToken)
			assert.NotEmpty(t, tokens.RefreshToken)
			assert.Equal(t, "Bearer", tokens.TokenType)
			assert.True(t, tokens.AccessTokenExpiresAt.After(time.Now()))

			// Verify access token is different from refresh token
			assert.NotEqual(t, tokens.AccessToken, tokens.RefreshToken)
		})
	}
}

func TestValidateToken(t *testing.T) {
	authService := NewAuthService("test-secret-key")

	// Create a valid user and generate tokens
	user := &models.User{
		ID:               1,
		Username:         "testuser",
		Email:            "test@example.com",
		SubscriptionType: "pro",
	}

	tokens, err := authService.GenerateTokens(user)
	require.NoError(t, err)

	tests := []struct {
		name        string
		token       string
		expectError error
		checkClaims bool
	}{
		{
			name:        "valid access token",
			token:       tokens.AccessToken,
			expectError: nil,
			checkClaims: true,
		},
		{
			// Refresh tokens are opaque (not JWT) by design for better security
			// ValidateToken only works with access tokens (JWTs)
			name:        "refresh token is opaque (not JWT)",
			token:       tokens.RefreshToken,
			expectError: ErrInvalidToken,
			checkClaims: false,
		},
		{
			name:        "empty token",
			token:       "",
			expectError: ErrInvalidToken,
			checkClaims: false,
		},
		{
			name:        "malformed token",
			token:       "not.a.valid.jwt.token",
			expectError: ErrInvalidToken,
			checkClaims: false,
		},
		{
			name:        "token with wrong signature",
			token:       tokens.AccessToken + "tampered",
			expectError: ErrInvalidToken,
			checkClaims: false,
		},
		{
			name:        "completely invalid string",
			token:       "random-invalid-string",
			expectError: ErrInvalidToken,
			checkClaims: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := authService.ValidateToken(tt.token)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, claims)

				if tt.checkClaims {
					assert.Equal(t, user.ID, claims.UserID)
					assert.Equal(t, user.Username, claims.Username)
					assert.Equal(t, user.Email, claims.Email)
				}
			}
		})
	}
}

func TestRefreshTokens(t *testing.T) {
	authService := NewAuthService("test-secret-key")

	user := &models.User{
		ID:               1,
		Username:         "testuser",
		Email:            "test@example.com",
		SubscriptionType: "free",
	}

	tokens, err := authService.GenerateTokens(user)
	require.NoError(t, err)

	tests := []struct {
		name        string
		refreshTok  string
		user        *models.User
		expectError bool
	}{
		{
			// NOTE: Refresh tokens are now opaque (not JWT) by design.
			// Without a database configured, RefreshTokens falls back to JWT validation
			// which will fail because the token is opaque. This is expected behavior.
			// In production, the database should be configured for proper token rotation.
			name:        "opaque refresh token without database",
			refreshTok:  tokens.RefreshToken,
			user:        user,
			expectError: true, // Opaque tokens need database for validation
		},
		{
			name:        "using access token instead of refresh",
			refreshTok:  tokens.AccessToken,
			user:        user,
			expectError: true,
		},
		{
			name:        "empty refresh token",
			refreshTok:  "",
			user:        user,
			expectError: true,
		},
		{
			name:        "mismatched user ID",
			refreshTok:  tokens.RefreshToken,
			user:        &models.User{ID: 999, Username: "other", Email: "other@test.com"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newTokens, err := authService.RefreshTokens(tt.refreshTok, tt.user)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, newTokens)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, newTokens)
				assert.NotEmpty(t, newTokens.AccessToken)
				assert.NotEmpty(t, newTokens.RefreshToken)
			}
		})
	}
}

func TestGetUserRole(t *testing.T) {
	authService := NewAuthService("test-secret")

	tests := []struct {
		name         string
		subscription string
		expectedRole string
	}{
		{
			name:         "free subscription",
			subscription: "free",
			expectedRole: "free",
		},
		{
			name:         "pro subscription",
			subscription: "pro",
			expectedRole: "pro",
		},
		{
			name:         "team subscription",
			subscription: "team",
			expectedRole: "team",
		},
		{
			name:         "empty subscription",
			subscription: "",
			expectedRole: "free",
		},
		{
			name:         "unknown subscription",
			subscription: "enterprise",
			expectedRole: "free",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &models.User{SubscriptionType: tt.subscription}
			role := authService.getUserRole(user)
			assert.Equal(t, tt.expectedRole, role)
		})
	}
}

func TestValidateRegistration(t *testing.T) {
	authService := NewAuthService("test-secret")

	tests := []struct {
		name    string
		req     *RegisterRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid registration",
			req: &RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "SecurePass123!",
				FullName: "Test User",
			},
			wantErr: false,
		},
		{
			name: "username too short",
			req: &RegisterRequest{
				Username: "ab",
				Email:    "test@example.com",
				Password: "SecurePass123!",
			},
			wantErr: true,
			errMsg:  "username must be between 3 and 50 characters",
		},
		{
			name: "username too long",
			req: &RegisterRequest{
				Username: "a" + string(make([]byte, 50)),
				Email:    "test@example.com",
				Password: "SecurePass123!",
			},
			wantErr: true,
			errMsg:  "username must be between 3 and 50 characters",
		},
		{
			name: "password too short",
			req: &RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "short",
			},
			wantErr: true,
			errMsg:  "password must be at least 8 characters",
		},
		{
			name: "full name too long",
			req: &RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "SecurePass123!",
				FullName: string(make([]byte, 101)),
			},
			wantErr: true,
			errMsg:  "full name must be less than 100 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authService.ValidateRegistration(tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateUser(t *testing.T) {
	authService := NewAuthService("test-secret")

	tests := []struct {
		name    string
		req     *RegisterRequest
		wantErr bool
	}{
		{
			name: "valid user creation",
			req: &RegisterRequest{
				Username: "newuser",
				Email:    "new@example.com",
				Password: "SecurePassword123!",
				FullName: "New User",
			},
			wantErr: false,
		},
		{
			name: "user creation with minimal fields",
			req: &RegisterRequest{
				Username: "minuser",
				Email:    "min@example.com",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "invalid registration - short password",
			req: &RegisterRequest{
				Username: "baduser",
				Email:    "bad@example.com",
				Password: "short",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := authService.CreateUser(tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				require.NotNil(t, user)

				assert.Equal(t, tt.req.Username, user.Username)
				assert.Equal(t, tt.req.Email, user.Email)
				assert.Equal(t, tt.req.FullName, user.FullName)
				assert.NotEmpty(t, user.PasswordHash)
				assert.NotEqual(t, tt.req.Password, user.PasswordHash)
				assert.True(t, user.IsActive)
				assert.False(t, user.IsVerified)
				assert.Equal(t, "free", user.SubscriptionType)
				assert.True(t, user.HasUnlimitedCredits)
				assert.False(t, user.BypassBilling)
				assert.Equal(t, "cyberpunk", user.PreferredTheme)
				assert.Equal(t, "auto", user.PreferredAI)
			}
		})
	}
}

func TestPasswordStrengthCheck(t *testing.T) {
	authService := NewAuthService("test-secret")

	tests := []struct {
		name       string
		password   string
		wantStrong bool
		wantIssues int
	}{
		{
			name:       "strong password",
			password:   "SecureP@ss123!",
			wantStrong: true,
			wantIssues: 0,
		},
		{
			name:       "no uppercase",
			password:   "securep@ss123!",
			wantStrong: false,
			wantIssues: 1,
		},
		{
			name:       "no lowercase",
			password:   "SECUREP@SS123!",
			wantStrong: false,
			wantIssues: 1,
		},
		{
			name:       "no numbers",
			password:   "SecureP@ssword!",
			wantStrong: false,
			wantIssues: 1,
		},
		{
			name:       "no special characters",
			password:   "SecurePass123",
			wantStrong: false,
			wantIssues: 1,
		},
		{
			name:       "too short",
			password:   "Ab1!",
			wantStrong: false,
			wantIssues: 1,
		},
		{
			name:       "all issues",
			password:   "abc",
			wantStrong: false,
			wantIssues: 4, // too short, no upper, no number, no special
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isStrong, issues := authService.PasswordStrengthCheck(tt.password)

			assert.Equal(t, tt.wantStrong, isStrong)
			assert.Equal(t, tt.wantIssues, len(issues))
		})
	}
}

func TestExtractUserFromToken(t *testing.T) {
	authService := NewAuthService("test-secret-key")

	user := &models.User{
		ID:               42,
		Username:         "extracttest",
		Email:            "extract@example.com",
		SubscriptionType: "pro",
	}

	tokens, err := authService.GenerateTokens(user)
	require.NoError(t, err)

	tests := []struct {
		name        string
		token       string
		expectedID  uint
		expectError bool
	}{
		{
			name:        "valid token",
			token:       tokens.AccessToken,
			expectedID:  42,
			expectError: false,
		},
		{
			name:        "invalid token",
			token:       "invalid.token.here",
			expectedID:  0,
			expectError: true,
		},
		{
			name:        "empty token",
			token:       "",
			expectedID:  0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, err := authService.ExtractUserFromToken(tt.token)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, userID)
			}
		})
	}
}

func TestGetTokenInfo(t *testing.T) {
	authService := NewAuthService("test-secret-key")

	user := &models.User{
		ID:               1,
		Username:         "infotest",
		Email:            "info@example.com",
		SubscriptionType: "team",
	}

	tokens, err := authService.GenerateTokens(user)
	require.NoError(t, err)

	t.Run("valid token returns correct info", func(t *testing.T) {
		info, err := authService.GetTokenInfo(tokens.AccessToken)

		require.NoError(t, err)
		require.NotNil(t, info)

		assert.Equal(t, user.ID, info.UserID)
		assert.Equal(t, user.Username, info.Username)
		assert.Equal(t, user.Email, info.Email)
		assert.Equal(t, "team", info.Role)
		assert.True(t, info.ExpiresAt.After(time.Now()))
		assert.True(t, info.IssuedAt.Before(time.Now().Add(time.Second)))
	})

	t.Run("invalid token returns error", func(t *testing.T) {
		info, err := authService.GetTokenInfo("invalid-token")

		assert.Error(t, err)
		assert.Nil(t, info)
	})
}

// Benchmarks
func BenchmarkHashPassword(b *testing.B) {
	authService := NewAuthService("test-secret")
	password := "TestPassword123!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = authService.HashPassword(password)
	}
}

func BenchmarkCheckPassword(b *testing.B) {
	authService := NewAuthService("test-secret")
	password := "TestPassword123!"
	hash, _ := authService.HashPassword(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = authService.CheckPassword(password, hash)
	}
}

func BenchmarkGenerateTokens(b *testing.B) {
	authService := NewAuthService("test-secret")
	user := &models.User{
		ID:               1,
		Username:         "benchuser",
		Email:            "bench@example.com",
		SubscriptionType: "pro",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = authService.GenerateTokens(user)
	}
}

func BenchmarkValidateToken(b *testing.B) {
	authService := NewAuthService("test-secret")
	user := &models.User{
		ID:               1,
		Username:         "benchuser",
		Email:            "bench@example.com",
		SubscriptionType: "pro",
	}
	tokens, _ := authService.GenerateTokens(user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = authService.ValidateToken(tokens.AccessToken)
	}
}
