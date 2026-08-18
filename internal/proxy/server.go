// Package proxy implements the ProxyService GRPC server.
package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/snobb/go-yact/internal/endpoint"
	"github.com/snobb/go-yact/internal/logger"
	pb "github.com/snobb/go-yact/pkg/protos/gen/tunnel/v1"
)

const bufferSize = 32 * 1024

// Server represents a WorkerService GRPC server
type Server struct {
	serverCtx context.Context
	logger    logger.Logger

	listenerRegistry *endpoint.ListenerRegistry
	connRegistry     *endpoint.ConnRegistry

	pb.UnimplementedTunnelServiceServer
}

// NewServer creates a new workerService GRPC server
func NewServer(ctx context.Context, logger logger.Logger) *Server {
	return &Server{
		serverCtx:        ctx,
		logger:           logger,
		listenerRegistry: endpoint.NewRegistry(),
		connRegistry:     endpoint.NewConnRegistry(),
	}
}

// RegisterProxy registers proxy listeners on the server.
// NOTE: secret is not used for now.
func (s *Server) RegisterProxy(ctx context.Context, handshakeRequest *pb.RegisterProxyRequest) (*pb.RegisterProxyResponse, error) {
	logCtx := logger.WithAttrs(ctx,
		slog.String("entity", "proxy"))

	clientID, err := extractUserIdentity(ctx)
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "authentication failed: %v", err)
	}

	bindAddress := handshakeRequest.GetBindAddress()

	onNewConnection := func(connectionID string, conn net.Conn) {
		s.connRegistry.AddConn(conn, connectionID, clientID)
		s.logger.DebugContext(logCtx, "stored pending tcp connection",
			"connection_id", connectionID,
			"client_id", clientID,
			"remote_addr", conn.RemoteAddr().String())

		// TTL Safeguard: Close and remove if client never opens Data Pipe within 15 seconds
		time.AfterFunc(15*time.Second, func() {
			// load and delete connection if not claimed within 15sec.
			conn, err := s.connRegistry.ConnByConnectionID(connectionID, clientID)
			if err != nil {
				return
			}
			_ = conn.Close() // close the connecton
		})
	}

	if err := s.listenerRegistry.Register(s.serverCtx, bindAddress, clientID, onNewConnection); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register proxy listener: %v", err)
	}

	return &pb.RegisterProxyResponse{Address: bindAddress}, nil
}

// ListenEvents subscribes client for the new connection events.
func (s *Server) ListenEvents(eventStreamRequest *pb.EventStreamRequest, stream grpc.ServerStreamingServer[pb.NewConnectionEvent]) error {
	streamCtx := stream.Context()

	clientID, err := extractUserIdentity(streamCtx)
	if err != nil {
		return status.Errorf(codes.PermissionDenied, "authentication failed: %v", err)
	}

	listener, err :=
		s.listenerRegistry.GetListenerByAddress(eventStreamRequest.GetListenerAddress())
	if err != nil {
		return status.Errorf(codes.NotFound, "failed to get listener: %v", err)
	}

	ctx := logger.WithAttrs(streamCtx,
		slog.String("entity", "proxy"),
		slog.String("client_id", clientID),
		slog.String("listener_id", listener.Address()))

	if listener.OwnerClientID != clientID {
		s.logger.WarnContext(ctx, "unauthorized listener access attempt")
		return status.Errorf(codes.PermissionDenied, "permission denied")
	}

	s.logger.InfoContext(ctx, "client subscribed to listener events")

	defer func() {
		// close and clear client's listeners.
		s.logger.DebugContext(ctx, "removing listeners")
		s.listenerRegistry.RemoveListenerByClientID(clientID)
	}()

	for {
		select {
		case <-streamCtx.Done():
			s.logger.DebugContext(ctx, "event stream canceled by context")
			return streamCtx.Err()

		case event, ok := <-listener.EventChannel():
			if !ok {
				s.logger.InfoContext(ctx, "listener event channel closed")
				return nil
			}

			if err := stream.Send(event); err != nil {
				s.logger.ErrorContext(ctx, "failed to send connection event", "error", err)
				return status.Error(codes.Unavailable, "failed to send connection event")
			}
		}
	}
}

// OpenDataPipe initiates a bi-directional data stream bound to a specific connection_id.
func (s *Server) OpenDataPipe(stream grpc.BidiStreamingServer[pb.DataPacket, pb.DataPacket]) error {
	streamCtx := stream.Context()

	clientID, err := extractUserIdentity(streamCtx)
	if err != nil {
		return status.Errorf(codes.PermissionDenied, "authentication failed: %v", err)
	}

	initialPacket, err := stream.Recv()
	if err != nil {
		s.logger.ErrorContext(streamCtx, "failed to receive initial packet",
			"error", err, "client_id", clientID)
		return status.Error(codes.Unavailable, "failed to receive initial packet")
	}

	connectionID := initialPacket.GetConnectionId()

	logCtx := logger.WithAttrs(streamCtx,
		slog.String("entity", "proxy"),
		slog.String("client_id", clientID),
		slog.String("connection_id", connectionID))

	conn, err := s.connRegistry.ConnByConnectionID(connectionID, clientID)
	if err != nil {
		s.logger.ErrorContext(logCtx, "failed to find pending connection", "error", err)
		return status.Error(codes.NotFound, "failed to get listener")
	}

	if len(initialPacket.Payload) > 0 {
		// if the initial packet contains payload - send it over right away.
		if n, err := conn.Write(initialPacket.Payload); err != nil {
			s.logger.ErrorContext(logCtx, "failed to write initial packet payload",
				"error", err, "bytes_written", n)
			_ = conn.Close()
			return status.Error(codes.Unavailable, "failed to write initial packet payload")
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	cancelCtx, cancel := context.WithCancel(logCtx)
	defer cancel()

	go func() {
		defer cancel()
		defer s.logger.DebugContext(logCtx, "server: recv pipe closed")
		s.receivePackets(cancelCtx, &wg, conn, stream)
	}()

	go func() {
		defer cancel()
		defer s.logger.DebugContext(logCtx, "server: send pipe closed")
		s.sendPackets(cancelCtx, &wg, conn, stream)
	}()

	wg.Wait()

	s.logger.DebugContext(logCtx, "tunnel closed and cleaned up successfully")

	return nil
}

func (s *Server) receivePackets(ctx context.Context, wg *sync.WaitGroup, conn net.Conn, stream grpc.BidiStreamingServer[pb.DataPacket, pb.DataPacket]) {
	defer wg.Done()
	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			s.logger.DebugContext(ctx, "data pipe canceled by context")
			return
		default:
		}

		packet, err := stream.Recv()
		if err != nil {
			// Expected normal closures when context is canceled or client closes stream
			if errors.Is(err, io.EOF) {
				s.logger.DebugContext(ctx, "grpc receive stream closed cleanly", "reason", err)
			} else {
				s.logger.ErrorContext(ctx, "failed to receive packet", "error", err)
			}
			return
		}

		if len(packet.Payload) > 0 {
			if _, err := conn.Write(packet.Payload); err != nil {
				s.logger.ErrorContext(ctx, "failed to write packet payload", "error", err)
				return
			}
		}
	}
}

func (s *Server) sendPackets(ctx context.Context, wg *sync.WaitGroup, conn net.Conn, stream grpc.BidiStreamingServer[pb.DataPacket, pb.DataPacket]) {
	defer wg.Done()
	defer conn.Close()
	defer func() {
		if err := stream.Send(&pb.DataPacket{Closed: true}); err != nil {
			s.logger.ErrorContext(ctx, "failed to send close packet", "error", err)
		}
	}()

	buf := make([]byte, bufferSize)

	for {
		select {
		case <-ctx.Done():
			s.logger.DebugContext(ctx, "data pipe canceled by context")
			return
		default:
		}

		n, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				s.logger.DebugContext(ctx, "connection closed")
			} else {
				s.logger.ErrorContext(ctx, "failed to read packet payload", "error", err)
			}
			return
		}

		if n == 0 {
			continue
		}

		payloadCopy := make([]byte, n)
		copy(payloadCopy, buf[:n])

		packet := &pb.DataPacket{
			Payload: payloadCopy[:n],
		}

		if err := stream.Send(packet); err != nil {
			s.logger.ErrorContext(ctx, "failed to send packet", "error", err)
			return
		}
	}
}

func extractUserIdentity(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", errors.New("missing peer info")
	}

	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return "", errors.New("not a TLS connection")
	}

	// only trust verified certs.
	if len(tlsInfo.State.VerifiedChains) != 1 || len(tlsInfo.State.VerifiedChains[0]) == 0 {
		return "", errors.New("invalid or missing client certificate")
	}

	leaf := tlsInfo.State.VerifiedChains[0][0]

	if len(leaf.URIs) > 0 {
		return leaf.URIs[0].String(), nil
	}

	return "", errors.New("no valid user identity")
}
