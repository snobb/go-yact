// Package proxy provides proxy server.
package proxy

import (
	"context"

	"github.com/snobb/go-yact/internal/logger"
)

// Run starts the proxy server.
func Run(ctx context.Context, logger logger.Logger, addr string) error {
	logger.InfoContext(ctx, "starting proxy", "addr", addr)
	logger.DebugContext(ctx, "debug message")
	return nil
}
