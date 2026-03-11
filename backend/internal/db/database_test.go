package db

import (
	"testing"

	"gorm.io/gorm/logger"
)

func TestResolveGormLogLevel(t *testing.T) {
	t.Run("defaults to info in development", func(t *testing.T) {
		t.Setenv("GO_ENV", "development")
		t.Setenv("GORM_LOG_LEVEL", "")

		if got := resolveGormLogLevel(); got != logger.Info {
			t.Fatalf("resolveGormLogLevel() = %v, want %v", got, logger.Info)
		}
	})

	t.Run("defaults to warn in production", func(t *testing.T) {
		t.Setenv("GO_ENV", "production")
		t.Setenv("GORM_LOG_LEVEL", "")

		if got := resolveGormLogLevel(); got != logger.Warn {
			t.Fatalf("resolveGormLogLevel() = %v, want %v", got, logger.Warn)
		}
	})

	t.Run("defaults to warn in staging", func(t *testing.T) {
		t.Setenv("GO_ENV", "staging")
		t.Setenv("GORM_LOG_LEVEL", "")

		if got := resolveGormLogLevel(); got != logger.Warn {
			t.Fatalf("resolveGormLogLevel() = %v, want %v", got, logger.Warn)
		}
	})

	t.Run("accepts explicit override", func(t *testing.T) {
		t.Setenv("GO_ENV", "production")
		t.Setenv("GORM_LOG_LEVEL", "error")

		if got := resolveGormLogLevel(); got != logger.Error {
			t.Fatalf("resolveGormLogLevel() = %v, want %v", got, logger.Error)
		}
	})

	t.Run("invalid override falls back to environment default", func(t *testing.T) {
		t.Setenv("GO_ENV", "production")
		t.Setenv("GORM_LOG_LEVEL", "loud")

		if got := resolveGormLogLevel(); got != logger.Warn {
			t.Fatalf("resolveGormLogLevel() = %v, want %v", got, logger.Warn)
		}
	})
}
