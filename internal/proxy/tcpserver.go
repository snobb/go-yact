package proxy

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/snobb/go-yact/internal/logger"
)

// Handler is a tcp listener callback.
type Handler interface {
	Handle(ctx context.Context, conn net.Conn)
}

// HandlerFunc is a tcp listener callback.
type HandlerFunc func(ctx context.Context, conn net.Conn)

// Handle handles the given TCP connection.
func (f HandlerFunc) Handle(ctx context.Context, conn net.Conn) {
	f(ctx, conn)
}

// Listen creates a new tcp listener.
func Listen(ctx context.Context, logger logger.Logger, port int, handler Handler) error {
	logger.InfoContext(ctx, "Started listening", "port", port)

	// Initialize ListenConfig instead of using net.Listen directly
	lc := net.ListenConfig{
		KeepAlive: 15 * time.Second, // Enables TCP keep-alive probes
	}

	addr := fmt.Sprintf(":%d", port)

	// Bind the listener using the context
	listener, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to bind listener", "addr", addr, "err", err)
		return err
	}
	defer func() { _ = listener.Close() }()

	// Goroutine to handle graceful shutdown when context is canceled.
	// the listener accepts ctx but it only used during the name resolution and address binding, so
	// context cancellation should still be handled separately and listener.Accept() call may still
	// hang.
	go func() {
		<-ctx.Done()
		fmt.Println("\nShutting down server listener...")
		_ = listener.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			logger.ErrorContext(ctx, "Failed to accept connection", "err", err)
		}

		logger.InfoContext(ctx, "Accepted connection", "source_address", conn.RemoteAddr().String())

		go func() {
			defer func() { _ = conn.Close() }()
			handler.Handle(ctx, conn)
		}()
	}
}
