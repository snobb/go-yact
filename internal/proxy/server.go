// Package proxy implements the ProxyService GRPC server.
package proxy

import (
	"log/slog"

	pb "github.com/snobb/go-yact/pkg/protos/gen/tunnel/v1"
)

const (
	defaultOutputBufferSize = 4096
)

// Server represents a WorkerService GRPC server
type Server struct {
	logger *slog.Logger

	pb.UnimplementedProxyServiceServer
}

// NewServer creates a new workerService GRPC server
func NewServer(logger *slog.Logger) *Server {
	return &Server{
		logger: logger,
	}
}
