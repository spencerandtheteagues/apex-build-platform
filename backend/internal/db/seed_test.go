package db

import "testing"

func TestGetSeedPassword(t *testing.T) {
	t.Run("explicit environment password wins", func(t *testing.T) {
		t.Setenv("GO_ENV", "development")
		t.Setenv("ADMIN_SEED_PASSWORD", "custom-seed-password")

		if got := getSeedPassword("ADMIN_SEED_PASSWORD", "admin-dev-password"); got != "custom-seed-password" {
			t.Fatalf("getSeedPassword() = %q, want explicit env value", got)
		}
	})

	t.Run("test environment keeps built in defaults", func(t *testing.T) {
		t.Setenv("GO_ENV", "test")

		if got := getSeedPassword("ADMIN_SEED_PASSWORD", "admin-dev-password"); got != "admin-dev-password" {
			t.Fatalf("getSeedPassword() = %q, want built-in test default", got)
		}
	})

	t.Run("development skips defaults unless explicitly enabled", func(t *testing.T) {
		t.Setenv("GO_ENV", "development")
		t.Setenv("ALLOW_DEFAULT_SEED_PASSWORDS", "false")

		if got := getSeedPassword("ADMIN_SEED_PASSWORD", "admin-dev-password"); got != "" {
			t.Fatalf("getSeedPassword() = %q, want empty string when defaults are disabled", got)
		}
	})

	t.Run("development allows defaults when explicitly enabled", func(t *testing.T) {
		t.Setenv("GO_ENV", "development")
		t.Setenv("ALLOW_DEFAULT_SEED_PASSWORDS", "true")

		if got := getSeedPassword("ADMIN_SEED_PASSWORD", "admin-dev-password"); got != "admin-dev-password" {
			t.Fatalf("getSeedPassword() = %q, want built-in development default", got)
		}
	})

	t.Run("production refuses built in defaults", func(t *testing.T) {
		t.Setenv("GO_ENV", "production")
		t.Setenv("ALLOW_DEFAULT_SEED_PASSWORDS", "true")

		if got := getSeedPassword("ADMIN_SEED_PASSWORD", "admin-dev-password"); got != "" {
			t.Fatalf("getSeedPassword() = %q, want empty string in production", got)
		}
	})

	t.Run("staging refuses built in defaults", func(t *testing.T) {
		t.Setenv("GO_ENV", "staging")
		t.Setenv("ALLOW_DEFAULT_SEED_PASSWORDS", "true")

		if got := getSeedPassword("ADMIN_SEED_PASSWORD", "admin-dev-password"); got != "" {
			t.Fatalf("getSeedPassword() = %q, want empty string in staging", got)
		}
	})
}
