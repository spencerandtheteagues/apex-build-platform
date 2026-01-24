package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2 parameters (time=1, memory=64MB, threads=4)
	ArgonTime    = 1
	ArgonMemory  = 64 * 1024
	ArgonThreads = 4
	ArgonKeyLen  = 32
	SaltLen      = 16
)

type PasswordService struct{}

func NewPasswordService() *PasswordService {
	return &PasswordService{}
}

// HashPassword creates a secure hash of the password using Argon2
func (p *PasswordService) HashPassword(password string) (string, error) {
	// Generate random salt
	salt := make([]byte, SaltLen)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}

	// Hash password with Argon2
	hash := argon2.IDKey([]byte(password), salt, ArgonTime, ArgonMemory, ArgonThreads, ArgonKeyLen)

	// Encode salt and hash to base64
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)

	// Format: $argon2id$v=19$m=65536,t=1,p=4$salt$hash
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		ArgonMemory, ArgonTime, ArgonThreads, saltB64, hashB64), nil
}

// VerifyPassword verifies if the provided password matches the stored hash
func (p *PasswordService) VerifyPassword(password, storedHash string) (bool, error) {
	// Parse stored hash
	parts := strings.Split(storedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, errors.New("invalid hash format")
	}

	// Parse parameters
	var m, t, threads uint32
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &threads)
	if err != nil {
		return false, errors.New("invalid parameters in hash")
	}

	// Decode salt and hash
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errors.New("invalid salt in hash")
	}

	storedHashBytes, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, errors.New("invalid hash value")
	}

	// Hash the provided password with the same parameters
	computedHash := argon2.IDKey([]byte(password), salt, t, m, uint8(threads), uint32(len(storedHashBytes)))

	// Compare hashes using constant-time comparison
	return subtle.ConstantTimeCompare(storedHashBytes, computedHash) == 1, nil
}

// ValidatePasswordStrength checks if password meets security requirements
func (p *PasswordService) ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	if len(password) > 128 {
		return errors.New("password must be less than 128 characters long")
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one digit")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	// Check for common weak passwords
	commonPasswords := []string{
		"password", "123456", "password123", "admin", "qwerty",
		"letmein", "welcome", "monkey", "1234567890", "abc123",
	}

	lowerPassword := strings.ToLower(password)
	for _, weak := range commonPasswords {
		if lowerPassword == weak {
			return errors.New("password is too common, please choose a stronger password")
		}
	}

	return nil
}