package secrets

import (
	"encoding/base64"
	"testing"
)

func TestNormalizeMasterKeyBytes_Base64Key(t *testing.T) {
	source := make([]byte, 32)
	for i := range source {
		source[i] = byte(i + 1)
	}

	normalized, err := normalizeMasterKeyBytes(base64.StdEncoding.EncodeToString(source))
	if err != nil {
		t.Fatalf("normalizeMasterKeyBytes() error = %v", err)
	}

	if len(normalized) != 32 {
		t.Fatalf("normalizeMasterKeyBytes() length = %d, want 32", len(normalized))
	}

	for i := range source {
		if normalized[i] != source[i] {
			t.Fatalf("normalizeMasterKeyBytes() altered decoded byte at %d", i)
		}
	}
}

func TestNormalizeMasterKeyBytes_RawSecretIsDeterministic(t *testing.T) {
	raw := "Rdr7pQ2zLm4xVn8cHs5wTy9kBd3fJu6mZa1rNe0q"

	first, err := normalizeMasterKeyBytes(raw)
	if err != nil {
		t.Fatalf("normalizeMasterKeyBytes() error = %v", err)
	}
	second, err := normalizeMasterKeyBytes(raw)
	if err != nil {
		t.Fatalf("normalizeMasterKeyBytes() second call error = %v", err)
	}

	if len(first) != 32 || len(second) != 32 {
		t.Fatalf("normalizeMasterKeyBytes() lengths = %d/%d, want 32/32", len(first), len(second))
	}

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("normalizeMasterKeyBytes() not deterministic at byte %d", i)
		}
	}
}

func TestNewSecretsManager_RawMasterKeyEncryptDecrypt(t *testing.T) {
	manager, err := NewSecretsManager("Rdr7pQ2zLm4xVn8cHs5wTy9kBd3fJu6mZa1rNe0q")
	if err != nil {
		t.Fatalf("NewSecretsManager() error = %v", err)
	}

	encrypted, salt, _, err := manager.Encrypt(42, "render-compatible-secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := manager.Decrypt(42, encrypted, salt)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if decrypted != "render-compatible-secret" {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, "render-compatible-secret")
	}
}
