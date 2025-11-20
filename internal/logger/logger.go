package logger

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

var (
	defaultLogger *slog.Logger
	once          sync.Once
)

// Init initializes the global logger
func Init(level string) {
	once.Do(func() {
		var logLevel slog.Level
		switch level {
		case "DEBUG":
			logLevel = slog.LevelDebug
		case "WARN":
			logLevel = slog.LevelWarn
		case "ERROR":
			logLevel = slog.LevelError
		default:
			logLevel = slog.LevelInfo
		}

		opts := &slog.HandlerOptions{
			Level: logLevel,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				// Rename "msg" to "message" for consistency
				if a.Key == slog.MessageKey {
					a.Key = "message"
				}
				return a
			},
		}

		handler := slog.NewJSONHandler(os.Stdout, opts)
		defaultLogger = slog.New(handler)
		slog.SetDefault(defaultLogger)
	})
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	if defaultLogger == nil {
		Init("INFO")
	}
	defaultLogger.Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	if defaultLogger == nil {
		Init("INFO")
	}
	defaultLogger.Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	if defaultLogger == nil {
		Init("INFO")
	}
	defaultLogger.Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	if defaultLogger == nil {
		Init("INFO")
	}
	defaultLogger.Error(msg, args...)
}

// With returns a logger with attributes
func With(args ...any) *slog.Logger {
	if defaultLogger == nil {
		Init("INFO")
	}
	return defaultLogger.With(args...)
}

// WithContext returns a logger with context (placeholder for tracing)
func WithContext(ctx context.Context) *slog.Logger {
	if defaultLogger == nil {
		Init("INFO")
	}
	// TODO: Extract trace ID from context
	return defaultLogger
}
