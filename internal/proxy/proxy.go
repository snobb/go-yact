// Package proxy provides proxy server.
package proxy

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/snobb/go-yact/internal/logger"
	pb "github.com/snobb/go-yact/pkg/protos/gen/tunnel/v1"
)

// Run starts the proxy server.
func Run(ctx context.Context, log logger.Logger, cfg *Config) error {
	log.InfoContext(ctx, "starting proxy", "addr", cfg.ProxyAddr)

	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer cancel()

	logCtx := logger.WithAttrs(ctx,
		slog.String("proxy_addr", cfg.ProxyAddr),
	)

	listener, err := net.Listen("tcp", cfg.ProxyAddr)
	if err != nil {
		log.ErrorContext(logCtx, "failed to listen", "err", err)
		return err
	}

	tc, err := InitCredentials(cfg.TLS.CAPath, cfg.TLS.CertPath, cfg.TLS.KeyPath)
	if err != nil {
		log.ErrorContext(logCtx, "unable to initialize mTLS credentials", "error", err)
		return err
	}

	grpcServer := NewGRPCServer(
		tc,
		cfg.KeepAliveInterval,
		cfg.KeepAliveTimeout,
	)

	pb.RegisterTunnelServiceServer(grpcServer, NewServer(log))

	go func() {
		log.InfoContext(logCtx, "server started")
		if err := grpcServer.Serve(listener); err != nil && err != grpc.ErrServerStopped {
			log.ErrorContext(logCtx, "failed to serve", "error", err)
			return
		}
	}()

	<-ctx.Done()
	log.InfoContext(ctx, "shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		log.InfoContext(ctx, "gRPC server stopped")
	case <-shutdownCtx.Done():
		log.WarnContext(logCtx, "shutdown timeout reached, forcing stop")
		grpcServer.Stop()
	}

	return nil
}
