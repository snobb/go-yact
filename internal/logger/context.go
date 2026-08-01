package logger

import (
	"context"
	"log/slog"
)

type ctxAttrKey struct{}
type ctxLoggerKey struct{}

// WithAttrs adds attributes to context.
func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	ctxAttrs, ok := ctx.Value(ctxAttrKey{}).([]slog.Attr)
	if !ok {
		ctxAttrs = nil
	}

	mergedAttrs := make([]slog.Attr, len(attrs)+len(ctxAttrs))
	mergedAttrs = append(mergedAttrs, ctxAttrs...)
	mergedAttrs = append(mergedAttrs, attrs...)

	return context.WithValue(ctx, ctxAttrKey{}, mergedAttrs)
}

func Attrs(ctx context.Context) []slog.Attr {
	attrs := ctx.Value(ctxAttrKey{}).([]slog.Attr)
	return attrs
}

func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, ctxLoggerKey{}, logger)
}

func CtxLogger(ctx context.Context) (Logger, bool) {
	logger, ok := ctx.Value(ctxLoggerKey{}).(Logger)
	return logger, ok
}
