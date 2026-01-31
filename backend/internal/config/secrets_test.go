package config

import (
	"encoding/base64"
	"os"
	"testing"
)

func TestGetEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "defaults to development",
			envVars:  map[string]string{},
			expected: "development",
		},
		{
			name:     "GO_ENV takes precedence",
			envVars:  map[string]string{"GO_ENV": "production", "APEX_ENV": "staging"},
			expected: "production",
		},
		{
			name:     "APEX_ENV used when GO_ENV not set",
			envVars:  map[string]string{"APEX_ENV": "staging"},
			expected: "staging",
		},
		{
			name:     "ENVIRONMENT used as fallback",
			envVars:  map[string]string{"ENVIRONMENT": "test"},
			expected: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars
			os.Unsetenv("GO_ENV")
			os.Unsetenv("APEX_ENV")
			os.Unsetenv("ENVIRONMENT")
			os.Unsetenv("ENV")

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			result := GetEnvironment()
			if result != tt.expected {
				t.Errorf("GetEnvironment() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsProductionEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"production", "production", true},
		{"prod", "prod", true},
		{"development", "development", false},
		{"staging", "staging", false},
		{"test", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("GO_ENV")
			os.Unsetenv("APEX_ENV")
			os.Unsetenv("ENVIRONMENT")
			os.Unsetenv("ENV")
			os.Setenv("GO_ENV", tt.envValue)

			result := IsProductionEnvironment()
			if result != tt.expected {
				t.Errorf("IsProductionEnvironment() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateJWTSecret(t *testing.T) {
	tests := []struct {
		name      string
		secret    string
		shouldErr bool
	}{
		{"valid secret", "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6", false},
		{"weak - contains 'secret'", "my-jwt-secret-key", true},
		{"weak - contains 'password'", "password123456789012345678901234", true},
		{"weak - contains 'changeme'", "please-changeme-before-production", true},
		{"too short", "short", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJWTSecret(tt.secret)
			if (err != nil) != tt.shouldErr {
				t.Errorf("validateJWTSecret() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestValidateMasterKey(t *testing.T) {
	// Generate a valid 32-byte key
	validKey := make([]byte, 32)
	for i := range validKey {
		validKey[i] = byte(i)
	}
	validKeyBase64 := base64.StdEncoding.EncodeToString(validKey)

	// Invalid key (too short)
	shortKey := make([]byte, 16)
	shortKeyBase64 := base64.StdEncoding.EncodeToString(shortKey)

	tests := []struct {
		name      string
		key       string
		shouldErr bool
	}{
		{"valid 32-byte key", validKeyBase64, false},
		{"too short (16 bytes)", shortKeyBase64, true},
		{"invalid base64", "not-valid-base64!!!", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMasterKey(tt.key)
			if (err != nil) != tt.shouldErr {
				t.Errorf("validateMasterKey() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestValidateStripeKey(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		shouldErr bool
	}{
		{"valid test key", "sk_test_1234567890abcdef", false},
		{"valid live key", "sk_live_1234567890abcdef", false},
		{"placeholder test key", "sk_test_xxx", true},
		{"invalid prefix", "api_key_1234567890", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStripeKey(tt.key)
			if (err != nil) != tt.shouldErr {
				t.Errorf("validateStripeKey() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestGenerateSecureSecret(t *testing.T) {
	secret1, err := GenerateSecureSecret(32)
	if err != nil {
		t.Fatalf("GenerateSecureSecret() error = %v", err)
	}

	secret2, err := GenerateSecureSecret(32)
	if err != nil {
		t.Fatalf("GenerateSecureSecret() error = %v", err)
	}

	// Secrets should be unique
	if secret1 == secret2 {
		t.Error("GenerateSecureSecret() generated duplicate secrets")
	}

	// Should be base64 encoded
	if len(secret1) == 0 {
		t.Error("GenerateSecureSecret() generated empty secret")
	}
}

func TestGenerateMasterKey(t *testing.T) {
	key, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey() error = %v", err)
	}

	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("GenerateMasterKey() generated invalid base64: %v", err)
	}

	// Should be exactly 32 bytes
	if len(decoded) != 32 {
		t.Errorf("GenerateMasterKey() generated %d bytes, want 32", len(decoded))
	}
}

func TestValidateSecrets_Development(t *testing.T) {
	// Clear all env vars
	os.Unsetenv("GO_ENV")
	os.Unsetenv("APEX_ENV")
	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("ENV")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("SECRETS_MASTER_KEY")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("STRIPE_SECRET_KEY")

	// In development mode, missing secrets should not fail
	os.Setenv("GO_ENV", "development")

	config, err := ValidateSecrets()
	if err != nil {
		t.Fatalf("ValidateSecrets() in development mode should not fail: %v", err)
	}

	if config.IsProduction {
		t.Error("ValidateSecrets() should set IsProduction=false in development")
	}
}

func TestValidateSecrets_Production(t *testing.T) {
	// Clear all env vars
	os.Unsetenv("GO_ENV")
	os.Unsetenv("APEX_ENV")
	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("ENV")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("SECRETS_MASTER_KEY")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("STRIPE_SECRET_KEY")

	// In production mode, missing required secrets should fail
	os.Setenv("GO_ENV", "production")

	_, err := ValidateSecrets()
	if err == nil {
		t.Error("ValidateSecrets() in production mode should fail with missing secrets")
	}

	// Check that it's a validation error
	if _, ok := err.(*SecretsValidationError); !ok {
		t.Errorf("ValidateSecrets() error should be *SecretsValidationError, got %T", err)
	}
}

func TestSecretsValidationError(t *testing.T) {
	err := &SecretsValidationError{
		Missing:  []string{"JWT_SECRET", "DATABASE_URL"},
		Invalid:  []string{"STRIPE_SECRET_KEY"},
		Warnings: []string{"some warning"},
	}

	if !err.HasErrors() {
		t.Error("HasErrors() should return true when there are missing or invalid secrets")
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error() should return a non-empty string")
	}

	// No errors case
	noErr := &SecretsValidationError{
		Warnings: []string{"just a warning"},
	}
	if noErr.HasErrors() {
		t.Error("HasErrors() should return false when there are only warnings")
	}
}

func TestJWTRotationValidator(t *testing.T) {
	// This test would need JWT token generation which requires more setup
	// Skipping detailed token validation tests for now

	validator := NewJWTRotationValidator("current-secret", "old-secret")
	if validator == nil {
		t.Error("NewJWTRotationValidator() returned nil")
	}

	if validator.currentSecret == nil {
		t.Error("currentSecret should be set")
	}

	if validator.oldSecret == nil {
		t.Error("oldSecret should be set when provided")
	}

	// Test without old secret
	validatorNoOld := NewJWTRotationValidator("current-secret", "")
	if validatorNoOld.oldSecret != nil {
		t.Error("oldSecret should be nil when empty string provided")
	}
}
