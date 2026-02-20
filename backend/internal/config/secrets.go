// Package config provides production-grade secrets management and validation
// for the APEX.BUILD platform.
//
// SECURITY CRITICAL: This module ensures all required secrets are properly
// configured before the application starts, preventing security vulnerabilities
// from missing or weak credentials.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Environment constants
const (
	EnvProduction  = "production"
	EnvStaging     = "staging"
	EnvDevelopment = "development"
	EnvTest        = "test"
)

// SecretRequirement defines a required secret and its validation rules
type SecretRequirement struct {
	Name        string
	EnvVar      string
	Description string
	Required    bool // Required in production
	MinLength   int  // Minimum length for security
	Validator   func(string) error
}

// SecretsConfig holds validated secrets for the application
type SecretsConfig struct {
	// Core secrets
	JWTSecret    string
	JWTSecretOld string // For rotation support

	// Encryption
	SecretsMasterKey string

	// External services
	StripeSecretKey    string
	StripeWebhookSecret string

	// Database
	DatabaseURL string

	// Environment
	Environment string
	IsProduction bool
}

// SecretsValidationError represents a validation failure
type SecretsValidationError struct {
	Missing  []string
	Invalid  []string
	Warnings []string
}

func (e *SecretsValidationError) Error() string {
	var parts []string
	if len(e.Missing) > 0 {
		parts = append(parts, fmt.Sprintf("missing secrets: %s", strings.Join(e.Missing, ", ")))
	}
	if len(e.Invalid) > 0 {
		parts = append(parts, fmt.Sprintf("invalid secrets: %s", strings.Join(e.Invalid, ", ")))
	}
	return strings.Join(parts, "; ")
}

func (e *SecretsValidationError) HasErrors() bool {
	return len(e.Missing) > 0 || len(e.Invalid) > 0
}

// DefaultSecretRequirements returns the standard secret requirements for APEX.BUILD
func DefaultSecretRequirements() []SecretRequirement {
	return []SecretRequirement{
		{
			Name:        "JWT Secret",
			EnvVar:      "JWT_SECRET",
			Description: "Secret key for signing JWT tokens",
			Required:    true,
			MinLength:   32,
			Validator:   validateJWTSecret,
		},
		{
			Name:        "Secrets Master Key",
			EnvVar:      "SECRETS_MASTER_KEY",
			Description: "AES-256 master key for encrypting user secrets (base64 encoded)",
			Required:    true,
			MinLength:   32,
			Validator:   validateMasterKey,
		},
		{
			Name:        "Database URL",
			EnvVar:      "DATABASE_URL",
			Description: "PostgreSQL connection string",
			Required:    true,
			MinLength:   10,
			Validator:   validateDatabaseURL,
		},
		{
			Name:        "Stripe Secret Key",
			EnvVar:      "STRIPE_SECRET_KEY",
			Description: "Stripe API secret key for payment processing",
			Required:    false, // Optional - payments disabled without it
			MinLength:   20,
			Validator:   validateStripeKey,
		},
		{
			Name:        "Stripe Webhook Secret",
			EnvVar:      "STRIPE_WEBHOOK_SECRET",
			Description: "Stripe webhook signature verification secret",
			Required:    false,
			MinLength:   20,
			Validator:   nil,
		},
	}
}

// ValidateSecrets validates all required secrets and returns a SecretsConfig
func ValidateSecrets() (*SecretsConfig, error) {
	env := GetEnvironment()
	isProduction := IsProductionEnvironment()

	config := &SecretsConfig{
		Environment:  env,
		IsProduction: isProduction,
	}

	validationErr := &SecretsValidationError{}
	requirements := DefaultSecretRequirements()

	for _, req := range requirements {
		value := os.Getenv(req.EnvVar)

		// Check if required
		if value == "" {
			if req.Required && isProduction {
				validationErr.Missing = append(validationErr.Missing, req.EnvVar)
			} else if req.Required {
				validationErr.Warnings = append(validationErr.Warnings,
					fmt.Sprintf("%s not set - using development default (NOT SECURE FOR PRODUCTION)", req.EnvVar))
			}
			continue
		}

		// Check minimum length
		if len(value) < req.MinLength {
			if isProduction {
				validationErr.Invalid = append(validationErr.Invalid,
					fmt.Sprintf("%s: too short (min %d characters)", req.EnvVar, req.MinLength))
			} else {
				validationErr.Warnings = append(validationErr.Warnings,
					fmt.Sprintf("%s: shorter than recommended (%d chars, recommend %d+)", req.EnvVar, len(value), req.MinLength))
			}
		}

		// Run custom validator if present
		if req.Validator != nil {
			if err := req.Validator(value); err != nil {
				if isProduction {
					validationErr.Invalid = append(validationErr.Invalid,
						fmt.Sprintf("%s: %s", req.EnvVar, err.Error()))
				} else {
					validationErr.Warnings = append(validationErr.Warnings,
						fmt.Sprintf("%s: %s (allowed in development)", req.EnvVar, err.Error()))
				}
			}
		}
	}

	// Populate config with validated values
	config.JWTSecret = os.Getenv("JWT_SECRET")
	config.JWTSecretOld = os.Getenv("JWT_SECRET_OLD") // For rotation
	config.SecretsMasterKey = os.Getenv("SECRETS_MASTER_KEY")
	config.StripeSecretKey = os.Getenv("STRIPE_SECRET_KEY")
	config.StripeWebhookSecret = os.Getenv("STRIPE_WEBHOOK_SECRET")
	config.DatabaseURL = os.Getenv("DATABASE_URL")

	// In production, enforce critical secrets are present
	if isProduction {
		if config.SecretsMasterKey == "" {
			return nil, errors.New("CRITICAL: SECRETS_MASTER_KEY is required in production - encrypted user data will be lost without it")
		}
		if config.JWTSecret == "" {
			return nil, errors.New("CRITICAL: JWT_SECRET is required in production - authentication will not work")
		}
	}

	// In production, fail on any validation errors
	if isProduction && validationErr.HasErrors() {
		return nil, validationErr
	}

	// Log warnings in development
	for _, warning := range validationErr.Warnings {
		log.Printf("WARNING: %s", warning)
	}

	return config, nil
}

// ValidateAndLogSecrets validates secrets and logs configuration status
// This should be called at application startup
func ValidateAndLogSecrets() (*SecretsConfig, error) {
	log.Println("Validating secrets configuration...")

	config, err := ValidateSecrets()
	if err != nil {
		log.Printf("CRITICAL: Secrets validation failed: %v", err)
		return nil, err
	}

	// Log which secrets are configured (names only, never values)
	log.Println("Secrets configuration status:")
	logSecretStatus("JWT_SECRET", config.JWTSecret != "")
	logSecretStatus("JWT_SECRET_OLD (rotation)", config.JWTSecretOld != "")
	logSecretStatus("SECRETS_MASTER_KEY", config.SecretsMasterKey != "")
	logSecretStatus("STRIPE_SECRET_KEY", config.StripeSecretKey != "")
	logSecretStatus("STRIPE_WEBHOOK_SECRET", config.StripeWebhookSecret != "")
	logSecretStatus("DATABASE_URL", config.DatabaseURL != "")

	if config.IsProduction {
		log.Println("Running in PRODUCTION mode - strict secret validation enforced")
	} else {
		log.Printf("Running in %s mode - development defaults allowed", config.Environment)
	}

	return config, nil
}

func logSecretStatus(name string, configured bool) {
	if configured {
		log.Printf("  [OK] %s: configured", name)
	} else {
		log.Printf("  [--] %s: not configured", name)
	}
}

// GetEnvironment returns the current environment
func GetEnvironment() string {
	// Check multiple environment variables for compatibility
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = os.Getenv("APEX_ENV")
	}
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	if env == "" {
		env = os.Getenv("ENV")
	}
	if env == "" {
		env = EnvDevelopment
	}
	return strings.ToLower(env)
}

// IsProductionEnvironment returns true if running in production
func IsProductionEnvironment() bool {
	env := GetEnvironment()
	return env == EnvProduction || env == "prod"
}

// IsStagingEnvironment returns true if running in staging
func IsStagingEnvironment() bool {
	env := GetEnvironment()
	return env == EnvStaging || env == "stage"
}

// Validators for specific secret types

func validateJWTSecret(secret string) error {
	// Check for known weak/placeholder values
	weakSecrets := []string{
		"secret",
		"jwt-secret",
		"your-secret",
		"changeme",
		"password",
		"test",
		"dev",
		"development",
	}

	lower := strings.ToLower(secret)
	for _, weak := range weakSecrets {
		if strings.Contains(lower, weak) {
			return errors.New("contains weak/placeholder value")
		}
	}

	// Check for sufficient entropy (at least some variety)
	if isLowEntropy(secret) {
		return errors.New("appears to have low entropy")
	}

	return nil
}

func validateMasterKey(key string) error {
	// Must be valid base64
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("must be valid base64 encoded: %w", err)
	}

	// Must be exactly 32 bytes (256 bits) for AES-256
	if len(decoded) != 32 {
		return fmt.Errorf("must decode to exactly 32 bytes, got %d", len(decoded))
	}

	return nil
}

func validateDatabaseURL(url string) error {
	// Must contain postgres:// or postgresql://
	if !strings.HasPrefix(url, "postgres://") && !strings.HasPrefix(url, "postgresql://") {
		return errors.New("must be a PostgreSQL connection URL")
	}

	// Check for placeholder values
	if strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1") {
		// Only warn, don't fail - might be valid for some setups
		return nil
	}

	return nil
}

func validateStripeKey(key string) error {
	// Stripe keys have specific prefixes
	if !strings.HasPrefix(key, "sk_live_") && !strings.HasPrefix(key, "sk_test_") {
		return errors.New("must start with sk_live_ or sk_test_")
	}

	// Check for placeholder
	if key == "sk_test_xxx" || key == "sk_live_xxx" {
		return errors.New("appears to be a placeholder value")
	}

	return nil
}

// isLowEntropy checks if a string has suspiciously low entropy
func isLowEntropy(s string) bool {
	if len(s) < 16 {
		return true
	}

	// Count unique characters
	unique := make(map[rune]bool)
	for _, c := range s {
		unique[c] = true
	}

	// Should have reasonable variety
	return len(unique) < len(s)/4
}

// GenerateSecureSecret generates a cryptographically secure random secret
func GenerateSecureSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateMasterKey generates a new AES-256 master key
func GenerateMasterKey() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate master key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// JWTRotationValidator supports validating tokens during key rotation
type JWTRotationValidator struct {
	currentSecret []byte
	oldSecret     []byte
}

// NewJWTRotationValidator creates a validator that supports key rotation
func NewJWTRotationValidator(currentSecret, oldSecret string) *JWTRotationValidator {
	v := &JWTRotationValidator{
		currentSecret: []byte(currentSecret),
	}
	if oldSecret != "" {
		v.oldSecret = []byte(oldSecret)
	}
	return v
}

// ValidateToken validates a JWT token, trying the current key first, then the old key
func (v *JWTRotationValidator) ValidateToken(tokenString string, claims jwt.Claims) (*jwt.Token, error) {
	// Try current key first
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.currentSecret, nil
	})

	if err == nil && token.Valid {
		return token, nil
	}

	// If old secret is configured, try it for rotation support
	if v.oldSecret != nil {
		token, err = jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return v.oldSecret, nil
		})

		if err == nil && token.Valid {
			log.Println("WARNING: Token validated with old JWT secret - user should re-authenticate soon")
			return token, nil
		}
	}

	return nil, fmt.Errorf("token validation failed: %w", err)
}

// RequireProductionSecrets returns an error if any production-required secrets are missing
// Use this to enforce strict validation in critical code paths
func RequireProductionSecrets(secrets ...string) error {
	var missing []string
	for _, secret := range secrets {
		if os.Getenv(secret) == "" {
			missing = append(missing, secret)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required secrets not configured: %s", strings.Join(missing, ", "))
	}
	return nil
}

// MustGetSecret returns the value of a secret or panics if not set
// Only use this during initialization when the secret is absolutely required
func MustGetSecret(envVar string) string {
	value := os.Getenv(envVar)
	if value == "" {
		panic(fmt.Sprintf("CRITICAL: Required secret %s is not set", envVar))
	}
	return value
}

// GetSecretWithDefault returns a secret value or a default for development
// Logs a warning when using the default
func GetSecretWithDefault(envVar, defaultValue string) string {
	value := os.Getenv(envVar)
	if value == "" {
		if IsProductionEnvironment() {
			log.Printf("CRITICAL: %s not set in production environment", envVar)
		} else {
			log.Printf("WARNING: %s not set, using development default", envVar)
		}
		return defaultValue
	}
	return value
}

// SecretRotationInfo provides information about secret rotation status
type SecretRotationInfo struct {
	SecretName       string
	HasCurrent       bool
	HasOld           bool
	RotationActive   bool
	RotationStarted  time.Time
	RecommendedEnd   time.Time
}

// GetJWTRotationInfo returns information about JWT secret rotation status
func GetJWTRotationInfo() SecretRotationInfo {
	current := os.Getenv("JWT_SECRET")
	old := os.Getenv("JWT_SECRET_OLD")

	info := SecretRotationInfo{
		SecretName:     "JWT_SECRET",
		HasCurrent:     current != "",
		HasOld:         old != "",
		RotationActive: current != "" && old != "",
	}

	if info.RotationActive {
		// Recommend completing rotation within 24 hours
		info.RotationStarted = time.Now() // Would need persistence for accurate tracking
		info.RecommendedEnd = info.RotationStarted.Add(24 * time.Hour)
	}

	return info
}
