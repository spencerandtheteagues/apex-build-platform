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
	"math"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
)

// Environment constants
const (
	EnvProduction  = "production"
	EnvStaging     = "staging"
	EnvDevelopment = "development"
	EnvTest        = "test"
)

// Minimum entropy bits required for production secrets
const (
	MinEntropyBitsJWT       = 128
	MinEntropyBitsMasterKey = 256
	MinJWTSecretLength      = 32
	MinMasterKeyBytes       = 32
	MinDatabaseURLLength    = 10
	MinStripeKeyLength      = 20
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
	StripeSecretKey     string
	StripeWebhookSecret string

	// Database
	DatabaseURL string

	// Environment
	Environment  string
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
			MinLength:   MinJWTSecretLength,
			Validator:   validateJWTSecret,
		},
		{
			Name:        "Secrets Master Key",
			EnvVar:      "SECRETS_MASTER_KEY",
			Description: "AES-256 master key for encrypting user secrets (base64 encoded)",
			Required:    true,
			MinLength:   MinMasterKeyBytes,
			Validator:   validateMasterKey,
		},
		{
			Name:        "Database URL",
			EnvVar:      "DATABASE_URL",
			Description: "PostgreSQL connection string",
			Required:    true,
			MinLength:   MinDatabaseURLLength,
			Validator:   validateDatabaseURL,
		},
		{
			Name:        "Stripe Secret Key",
			EnvVar:      "STRIPE_SECRET_KEY",
			Description: "Stripe API secret key for payment processing",
			Required:    false, // Optional - payments disabled without it
			MinLength:   MinStripeKeyLength,
			Validator:   validateStripeKey,
		},
		{
			Name:        "Stripe Webhook Secret",
			EnvVar:      "STRIPE_WEBHOOK_SECRET",
			Description: "Stripe webhook signature verification secret",
			Required:    false,
			MinLength:   MinStripeKeyLength,
			Validator:   validateStripeWebhookSecret,
		},
	}
}

// ValidateSecrets validates all required secrets and returns a SecretsConfig.
// In production, this will return a non-nil error if any required secret is
// missing or invalid — callers MUST treat this as fatal.
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
			return nil, errors.New("FATAL: SECRETS_MASTER_KEY is required in production - encrypted user data will be lost without it")
		}
		if config.JWTSecret == "" {
			return nil, errors.New("FATAL: JWT_SECRET is required in production - authentication will not work")
		}
		if config.DatabaseURL == "" {
			return nil, errors.New("FATAL: DATABASE_URL is required in production - no database connection possible")
		}
	}

	// In production, fail on any validation errors
	if isProduction && validationErr.HasErrors() {
		return nil, validationErr
	}

	// In staging, fail on missing required secrets (treat like prod for secrets)
	if IsStagingEnvironment() && len(validationErr.Missing) > 0 {
		return nil, fmt.Errorf("staging environment requires all production secrets: %s",
			strings.Join(validationErr.Missing, ", "))
	}

	// Log warnings in development
	for _, warning := range validationErr.Warnings {
		log.Printf("WARNING: %s", warning)
	}

	return config, nil
}

// ValidateAndLogSecrets validates secrets and logs configuration status.
// This should be called at application startup.
// Returns a fatal error in production if secrets are invalid.
func ValidateAndLogSecrets() (*SecretsConfig, error) {
	log.Println("Validating secrets configuration...")

	config, err := ValidateSecrets()
	if err != nil {
		log.Printf("FATAL: Secrets validation failed: %v", err)
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

// --- Strict Validators ---

// validateJWTSecret enforces a strong JWT signing key.
func validateJWTSecret(secret string) error {
	// Blocklist of known weak/placeholder values
	weakSecrets := []string{
		"secret",
		"jwt-secret",
		"jwt_secret",
		"your-secret",
		"changeme",
		"password",
		"test",
		"dev",
		"development",
		"example",
		"default",
		"placeholder",
		"replace-me",
		"todo",
		"fixme",
		"apex-build-secret",
	}

	lower := strings.ToLower(secret)
	for _, weak := range weakSecrets {
		if lower == weak || strings.Contains(lower, weak) {
			return fmt.Errorf("contains weak/placeholder value %q", weak)
		}
	}

	// Reject if it's entirely alphabetic or entirely numeric
	allAlpha := true
	allDigit := true
	for _, c := range secret {
		if !unicode.IsLetter(c) {
			allAlpha = false
		}
		if !unicode.IsDigit(c) {
			allDigit = false
		}
	}
	if allAlpha {
		return errors.New("must contain non-alphabetic characters for sufficient entropy")
	}
	if allDigit {
		return errors.New("must contain non-numeric characters for sufficient entropy")
	}

	// Shannon entropy check
	entropy := shannonEntropy(secret)
	if entropy < 3.0 {
		return fmt.Errorf("entropy too low (%.1f bits/char, need >= 3.0)", entropy)
	}

	// Reject repeating patterns like "abcabc" or "aaaaaa"
	if hasRepeatingPattern(secret) {
		return errors.New("appears to contain a repeating pattern")
	}

	return nil
}

// validateMasterKey enforces a valid AES-256 key.
func validateMasterKey(key string) error {
	// Must be valid base64
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("must be valid base64 encoded: %w", err)
	}

	// Must be exactly 32 bytes (256 bits) for AES-256
	if len(decoded) != MinMasterKeyBytes {
		return fmt.Errorf("must decode to exactly %d bytes (got %d) for AES-256", MinMasterKeyBytes, len(decoded))
	}

	// Check that the key isn't all zeros or trivially weak
	allZero := true
	for _, b := range decoded {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return errors.New("master key is all zeros — this is not a valid encryption key")
	}

	// Check byte-level entropy
	byteEntropy := byteEntropy(decoded)
	if byteEntropy < 4.0 {
		return fmt.Errorf("master key byte entropy too low (%.1f, need >= 4.0)", byteEntropy)
	}

	return nil
}

// validateDatabaseURL checks for a valid PostgreSQL connection string.
func validateDatabaseURL(rawURL string) error {
	if !strings.HasPrefix(rawURL, "postgres://") && !strings.HasPrefix(rawURL, "postgresql://") {
		return errors.New("must be a PostgreSQL connection URL (postgres:// or postgresql://)")
	}

	// Parse the URL to validate structure
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}

	// Must have a host
	if parsed.Hostname() == "" {
		return errors.New("database URL must include a hostname")
	}

	// In production, reject default/example passwords
	if parsed.User != nil {
		password, hasPassword := parsed.User.Password()
		if hasPassword {
			weakPasswords := []string{"password", "postgres", "changeme", "test", "example", "apex_build_2024"}
			for _, weak := range weakPasswords {
				if strings.EqualFold(password, weak) {
					return fmt.Errorf("database password %q is a known default — use a strong password in production", weak)
				}
			}
		}
	}

	return nil
}

// validateStripeKey checks the Stripe secret key format.
func validateStripeKey(key string) error {
	if !strings.HasPrefix(key, "sk_live_") && !strings.HasPrefix(key, "sk_test_") {
		return errors.New("must start with sk_live_ or sk_test_")
	}

	// Reject obvious placeholders
	placeholderPattern := regexp.MustCompile(`^sk_(live|test)_[xX]+$`)
	if placeholderPattern.MatchString(key) {
		return errors.New("appears to be a placeholder value (all x's)")
	}

	if key == "sk_test_xxx" || key == "sk_live_xxx" {
		return errors.New("appears to be a placeholder value")
	}

	// Stripe keys are typically 100+ chars
	if len(key) < 30 {
		return errors.New("Stripe key appears truncated (expected 30+ characters)")
	}

	return nil
}

// validateStripeWebhookSecret checks webhook signing secret format.
func validateStripeWebhookSecret(secret string) error {
	if !strings.HasPrefix(secret, "whsec_") {
		return errors.New("must start with whsec_")
	}
	if len(secret) < 30 {
		return errors.New("webhook secret appears truncated")
	}
	return nil
}

// --- Entropy Helpers ---

// shannonEntropy calculates Shannon entropy in bits per character.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]float64)
	for _, c := range s {
		freq[c]++
	}
	length := float64(len([]rune(s)))
	entropy := 0.0
	for _, count := range freq {
		p := count / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// byteEntropy calculates Shannon entropy over raw bytes.
func byteEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	freq := make(map[byte]float64)
	for _, b := range data {
		freq[b]++
	}
	length := float64(len(data))
	entropy := 0.0
	for _, count := range freq {
		p := count / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// isLowEntropy checks if a string has suspiciously low entropy (legacy compat)
func isLowEntropy(s string) bool {
	if len(s) < 16 {
		return true
	}
	return shannonEntropy(s) < 2.5
}

// hasRepeatingPattern detects simple repeating patterns (e.g., "abcabc").
func hasRepeatingPattern(s string) bool {
	n := len(s)
	if n < 6 {
		return false
	}
	// Check for repeating substrings of length 1 to n/2
	for patLen := 1; patLen <= n/2; patLen++ {
		pattern := s[:patLen]
		isRepeat := true
		for i := patLen; i < n; i++ {
			if s[i] != pattern[i%patLen] {
				isRepeat = false
				break
			}
		}
		if isRepeat {
			return true
		}
	}
	return false
}

// --- Key Generation ---

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
	bytes := make([]byte, MinMasterKeyBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate master key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// --- JWT Rotation ---

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

// --- Production Enforcement Helpers ---

// RequireProductionSecrets returns an error if any production-required secrets are missing.
// Use this to enforce strict validation in critical code paths.
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

// MustValidateSecrets calls ValidateSecrets and fatally logs if it fails.
// This is the recommended startup call — it guarantees the process exits
// immediately if secrets are misconfigured.
func MustValidateSecrets() *SecretsConfig {
	config, err := ValidateAndLogSecrets()
	if err != nil {
		log.Fatalf("FATAL: Cannot start server — secrets validation failed: %v", err)
	}
	return config
}

// MustGetSecret returns the value of a secret or panics if not set.
// Only use this during initialization when the secret is absolutely required.
func MustGetSecret(envVar string) string {
	value := os.Getenv(envVar)
	if value == "" {
		panic(fmt.Sprintf("FATAL: Required secret %s is not set", envVar))
	}
	return value
}

// GetSecretWithDefault returns a secret value or a default for development.
// Logs a warning when using the default.
func GetSecretWithDefault(envVar, defaultValue string) string {
	value := os.Getenv(envVar)
	if value == "" {
		if IsProductionEnvironment() {
			log.Printf("FATAL: %s not set in production environment", envVar)
		} else {
			log.Printf("WARNING: %s not set, using development default", envVar)
		}
		return defaultValue
	}
	return value
}

// --- Rotation Info ---

// SecretRotationInfo provides information about secret rotation status
type SecretRotationInfo struct {
	SecretName      string
	HasCurrent      bool
	HasOld          bool
	RotationActive  bool
	RotationStarted time.Time
	RecommendedEnd  time.Time
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
		info.RotationStarted = time.Now()
		info.RecommendedEnd = info.RotationStarted.Add(24 * time.Hour)
	}

	return info
}
