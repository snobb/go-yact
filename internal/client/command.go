package client

import (
	"context"

	"github.com/snobb/go-yact/internal/logger"
)

// Run starts the client.
func Run(ctx context.Context, logger logger.Logger, addr string) error {
	logger.InfoContext(ctx, "starting client", "addr", addr)
	logger.DebugContext(ctx, "debug message")
	return nil
}
