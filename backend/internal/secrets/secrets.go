// Package secrets - Secure Secrets Manager with AES-256 Encryption
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

var (
	ErrInvalidKey      = errors.New("invalid encryption key")
	ErrDecryptionFailed = errors.New("decryption failed - data may be corrupted or key is wrong")
	ErrSecretNotFound  = errors.New("secret not found")
	ErrInvalidSecret   = errors.New("invalid secret format")
)

// SecretType categorizes secrets for better organization
type SecretType string

const (
	SecretTypeAPIKey      SecretType = "api_key"
	SecretTypeDatabase    SecretType = "database"
	SecretTypeOAuth       SecretType = "oauth"
	SecretTypeEnvironment SecretType = "environment"
	SecretTypeSSH         SecretType = "ssh"
	SecretTypeGeneric     SecretType = "generic"
)

// Secret represents an encrypted secret
type Secret struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	UserID       uint       `json:"user_id" gorm:"index;not null"`
	ProjectID    *uint      `json:"project_id,omitempty" gorm:"index"`
	Name         string     `json:"name" gorm:"not null"`
	Description  string     `json:"description,omitempty"`
	Type         SecretType `json:"type" gorm:"default:'generic'"`
	EncryptedValue string   `json:"-" gorm:"not null"` // Never expose in JSON
	KeyFingerprint string   `json:"-" gorm:"not null"` // For key rotation
	Salt           string   `json:"-" gorm:"not null"` // Unique salt per secret
	LastAccessed *time.Time `json:"last_accessed,omitempty"`
	RotationDue  *time.Time `json:"rotation_due,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// SecretMetadata is the safe-to-expose secret info (no values)
type SecretMetadata struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Type        SecretType `json:"type"`
	ProjectID   *uint      `json:"project_id,omitempty"`
	LastAccessed *time.Time `json:"last_accessed,omitempty"`
	RotationDue *time.Time `json:"rotation_due,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ToMetadata converts a Secret to safe metadata
func (s *Secret) ToMetadata() SecretMetadata {
	return SecretMetadata{
		ID:           s.ID,
		Name:         s.Name,
		Description:  s.Description,
		Type:         s.Type,
		ProjectID:    s.ProjectID,
		LastAccessed: s.LastAccessed,
		RotationDue:  s.RotationDue,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

// EncryptionKey represents a derived key for encryption/decryption
type EncryptionKey struct {
	key         []byte
	fingerprint string
}

// SecretsManager handles secure secret storage and retrieval
type SecretsManager struct {
	masterKey     []byte
	keyCache      map[uint]*EncryptionKey // UserID -> derived key
	mu            sync.RWMutex
	iterations    int // PBKDF2 iterations
}

// NewSecretsManager creates a new secrets manager with the given master key
func NewSecretsManager(masterKeyBase64 string) (*SecretsManager, error) {
	if masterKeyBase64 == "" {
		return nil, ErrInvalidKey
	}

	masterKey, err := base64.StdEncoding.DecodeString(masterKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid master key format: %w", err)
	}

	if len(masterKey) < 32 {
		return nil, ErrInvalidKey
	}

	return &SecretsManager{
		masterKey:  masterKey,
		keyCache:   make(map[uint]*EncryptionKey),
		iterations: 100000, // OWASP recommended minimum
	}, nil
}

// GenerateMasterKey creates a new random master key for initial setup
func GenerateMasterKey() (string, error) {
	key := make([]byte, 32) // 256 bits
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate master key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// deriveUserKey creates a unique encryption key for each user
func (sm *SecretsManager) deriveUserKey(userID uint, salt []byte) *EncryptionKey {
	// Combine master key with user-specific data
	userBytes := []byte(fmt.Sprintf("user:%d", userID))
	combined := append(sm.masterKey, userBytes...)

	// Derive key using PBKDF2
	key := pbkdf2.Key(combined, salt, sm.iterations, 32, sha256.New)

	// Create fingerprint for key rotation tracking
	fingerprint := sha256.Sum256(key)

	return &EncryptionKey{
		key:         key,
		fingerprint: base64.StdEncoding.EncodeToString(fingerprint[:8]),
	}
}

// generateSalt creates a random salt for each secret
func generateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// Encrypt encrypts a secret value for storage
func (sm *SecretsManager) Encrypt(userID uint, value string) (encryptedValue, saltBase64, keyFingerprint string, err error) {
	salt, err := generateSalt()
	if err != nil {
		return "", "", "", err
	}

	encKey := sm.deriveUserKey(userID, salt)

	// Create AES cipher
	block, err := aes.NewCipher(encKey.key)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode for authenticated encryption
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nonce, nonce, []byte(value), nil)

	return base64.StdEncoding.EncodeToString(ciphertext),
		base64.StdEncoding.EncodeToString(salt),
		encKey.fingerprint,
		nil
}

// Decrypt decrypts a secret value
func (sm *SecretsManager) Decrypt(userID uint, encryptedValue, saltBase64 string) (string, error) {
	salt, err := base64.StdEncoding.DecodeString(saltBase64)
	if err != nil {
		return "", fmt.Errorf("invalid salt: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedValue)
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext: %w", err)
	}

	encKey := sm.deriveUserKey(userID, salt)

	// Create AES cipher
	block, err := aes.NewCipher(encKey.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", ErrDecryptionFailed
	}

	// Extract nonce and decrypt
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// EncryptJSON encrypts a JSON-serializable value
func (sm *SecretsManager) EncryptJSON(userID uint, value interface{}) (encryptedValue, saltBase64, keyFingerprint string, err error) {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to marshal value: %w", err)
	}
	return sm.Encrypt(userID, string(jsonBytes))
}

// DecryptJSON decrypts and unmarshals a JSON value
func (sm *SecretsManager) DecryptJSON(userID uint, encryptedValue, saltBase64 string, target interface{}) error {
	plaintext, err := sm.Decrypt(userID, encryptedValue, saltBase64)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(plaintext), target)
}

// ValidateKeyFingerprint checks if a secret was encrypted with the current key
func (sm *SecretsManager) ValidateKeyFingerprint(userID uint, saltBase64, storedFingerprint string) (bool, error) {
	salt, err := base64.StdEncoding.DecodeString(saltBase64)
	if err != nil {
		return false, fmt.Errorf("invalid salt: %w", err)
	}

	encKey := sm.deriveUserKey(userID, salt)
	return encKey.fingerprint == storedFingerprint, nil
}

// EnvironmentVariables represents a set of env vars for a project
type EnvironmentVariables map[string]string

// EncryptEnvVars encrypts environment variables for a project
func (sm *SecretsManager) EncryptEnvVars(userID uint, envVars EnvironmentVariables) (encryptedValue, saltBase64, keyFingerprint string, err error) {
	return sm.EncryptJSON(userID, envVars)
}

// DecryptEnvVars decrypts environment variables for a project
func (sm *SecretsManager) DecryptEnvVars(userID uint, encryptedValue, saltBase64 string) (EnvironmentVariables, error) {
	var envVars EnvironmentVariables
	err := sm.DecryptJSON(userID, encryptedValue, saltBase64, &envVars)
	return envVars, err
}

// OAuthCredentials represents OAuth client credentials
type OAuthCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

// DatabaseCredentials represents database connection info
type DatabaseCredentials struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode,omitempty"`
}

// SecretAuditLog tracks access to secrets
type SecretAuditLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	SecretID  uint      `json:"secret_id" gorm:"index;not null"`
	UserID    uint      `json:"user_id" gorm:"index;not null"`
	Action    string    `json:"action"` // read, write, delete, rotate
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Success   bool      `json:"success"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
