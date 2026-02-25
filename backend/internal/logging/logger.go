// Package logging provides structured logging for APEX.BUILD.
//
// DEPENDENCY: This package requires go.uber.org/zap.
// Run: go get go.uber.org/zap
package logging

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
	sugar  *zap.SugaredLogger
	once   sync.Once
)

// Init initializes the global logger. Safe to call multiple times.
func Init() {
	once.Do(func() {
		var cfg zap.Config
		if os.Getenv("ENVIRONMENT") == "production" {
			cfg = zap.NewProductionConfig()
			cfg.EncoderConfig.TimeKey = "ts"
			cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		} else {
			cfg = zap.NewDevelopmentConfig()
			cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		}

		var err error
		logger, err = cfg.Build(zap.AddCallerSkip(1))
		if err != nil {
			// Fallback to nop logger
			logger = zap.NewNop()
		}
		sugar = logger.Sugar()
	})
}

// L returns the global structured logger
func L() *zap.Logger {
	if logger == nil {
		Init()
	}
	return logger
}

// S returns the global sugared logger (printf-style)
func S() *zap.SugaredLogger {
	if sugar == nil {
		Init()
	}
	return sugar
}

// Sync flushes any buffered log entries. Call before app exit.
func Sync() {
	if logger != nil {
		_ = logger.Sync()
	}
}

// WithContext returns a logger with additional structured fields
func WithContext(fields ...zap.Field) *zap.Logger {
	return L().With(fields...)
}
