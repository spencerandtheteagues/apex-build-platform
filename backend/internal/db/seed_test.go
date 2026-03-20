package db

import "testing"

func TestGetSeedPassword(t *testing.T) {
	t.Run("explicit environment password wins", func(t *testing.T) {
		t.Setenv("GO_ENV", "development")
		t.Setenv("ADMIN_SEED_PASSWORD", "custom-seed-password")

		if got := getSeedPassword("ADMIN_SEED_PASSWORD", defaultAdminSeedPassword); got != "custom-seed-password" {
			t.Fatalf("getSeedPassword() = %q, want explicit env value", got)
		}
	})

	t.Run("test environment keeps built in defaults", func(t *testing.T) {
		t.Setenv("GO_ENV", "test")

		if got := getSeedPassword("ADMIN_SEED_PASSWORD", defaultAdminSeedPassword); got != defaultAdminSeedPassword {
			t.Fatalf("getSeedPassword() = %q, want built-in test default", got)
		}
	})

	t.Run("development keeps built in defaults without extra flags", func(t *testing.T) {
		t.Setenv("GO_ENV", "development")
		t.Setenv("ALLOW_DEFAULT_SEED_PASSWORDS", "false")

		if got := getSeedPassword("ADMIN_SEED_PASSWORD", defaultAdminSeedPassword); got != defaultAdminSeedPassword {
			t.Fatalf("getSeedPassword() = %q, want built-in development default", got)
		}
	})

	t.Run("production refuses built in defaults", func(t *testing.T) {
		t.Setenv("GO_ENV", "production")
		t.Setenv("ALLOW_DEFAULT_SEED_PASSWORDS", "true")

		if got := getSeedPassword("ADMIN_SEED_PASSWORD", defaultAdminSeedPassword); got != "" {
			t.Fatalf("getSeedPassword() = %q, want empty string in production", got)
		}
	})

	t.Run("staging refuses built in defaults", func(t *testing.T) {
		t.Setenv("GO_ENV", "staging")
		t.Setenv("ALLOW_DEFAULT_SEED_PASSWORDS", "true")

		if got := getSeedPassword("ADMIN_SEED_PASSWORD", defaultAdminSeedPassword); got != "" {
			t.Fatalf("getSeedPassword() = %q, want empty string in staging", got)
		}
	})
}

func TestHasExplicitSeedPasswords(t *testing.T) {
	t.Run("false when neither seed password is configured", func(t *testing.T) {
		t.Setenv("ADMIN_SEED_PASSWORD", "")
		t.Setenv("SPENCER_SEED_PASSWORD", "")

		if HasExplicitSeedPasswords() {
			t.Fatal("HasExplicitSeedPasswords() = true, want false")
		}
	})

	t.Run("true when admin seed password is configured", func(t *testing.T) {
		t.Setenv("ADMIN_SEED_PASSWORD", defaultAdminSeedPassword)
		t.Setenv("SPENCER_SEED_PASSWORD", "")

		if !HasExplicitSeedPasswords() {
			t.Fatal("HasExplicitSeedPasswords() = false, want true")
		}
	})

	t.Run("true when spencer seed password is configured", func(t *testing.T) {
		t.Setenv("ADMIN_SEED_PASSWORD", "")
		t.Setenv("SPENCER_SEED_PASSWORD", "another-strong-password")

		if !HasExplicitSeedPasswords() {
			t.Fatal("HasExplicitSeedPasswords() = false, want true")
		}
	})
}
