package logger

import "context"

//go:generate moq -rm -fmt goimports -out logger_mock.go . Logger

type Logger interface {
	// Info logs at [LevelInfo].
	Info(msg string, args ...any)

	// InfoContext logs at [LevelInfo] with the given context.
	InfoContext(ctx context.Context, msg string, args ...any)

	// Warn logs at [LevelWarn].
	Warn(msg string, args ...any)

	// WarnContext logs at [LevelWarn] with the given context.
	WarnContext(ctx context.Context, msg string, args ...any)

	// Error logs at [LevelError].
	Error(msg string, args ...any)

	// ErrorContext logs at [LevelError] with the given context.
	ErrorContext(ctx context.Context, msg string, args ...any)

	// Debug logs at [LevelDebug].
	Debug(msg string, args ...any)

	// DebugContext logs at [LevelDebug] with the given context.
	DebugContext(ctx context.Context, msg string, args ...any)
}
