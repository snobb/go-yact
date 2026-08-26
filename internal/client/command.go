package client

import (
	"context"

	"github.com/snobb/go-yact/internal/logger"
	pb "github.com/snobb/go-yact/pkg/protos/gen/tunnel/v1"
)

const bufferSize = 32 * 1024

// Run starts the client.
func Run(ctx context.Context, logger logger.Logger, cfg *Config) error {
	logger.InfoContext(ctx, "starting client - connecting to proxy server", "addr", cfg.ProxyAddr)
	// create a control plane connection.
	conn, closeFunc, err := makeControlConn(cfg)
	if err != nil {
		return err
	}
	defer closeFunc()

	grpcClient := pb.NewTunnelServiceClient(conn)

	client := NewClient(logger, grpcClient)
	return client.Run(ctx, cfg.ToAddrs[0].BindAddress, cfg.ToAddrs[0].LocalPort)
}
