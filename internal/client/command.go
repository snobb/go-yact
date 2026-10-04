package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/snobb/go-yact/internal/logger"
	pb "github.com/snobb/go-yact/pkg/protos/gen/tunnel/v1"
)

const bufferSize = 32 * 1024

// Run starts the client.
func Run(ctx context.Context, logger logger.Logger, cfg *Config) error {
	if len(cfg.ToAddrs) == 0 {
		return fmt.Errorf("no listeners specified")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errs := make(chan error, len(cfg.ToAddrs))
	defer close(errs)

	for _, bindAddress := range cfg.ToAddrs {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := handleListener(ctx, logger, cfg, &bindAddress); err != nil {
				errs <- err
			}
		}()
	}

	var err error

	select {
	case err = <-errs:
	case <-ctx.Done():
		err = ctx.Err()
	}

	cancel()
	wg.Wait()

	return err
}

func handleListener(ctx context.Context, logger logger.Logger, cfg *Config, bindAddress *BindAddress) error {
	logger.InfoContext(ctx, "starting client for listener", "listener", bindAddress.BindAddress)
	logger.InfoContext(ctx, "connecting to proxy server", "addr", cfg.ProxyAddr)

	// create a control plane connection.
	conn, closeFunc, err := makeControlConn(cfg)
	if err != nil {
		return err
	}
	defer closeFunc()

	grpcClient := pb.NewTunnelServiceClient(conn)

	client := NewClient(logger, grpcClient)
	return client.Run(ctx, bindAddress.BindAddress, bindAddress.LocalPort)
}
