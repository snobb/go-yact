package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"google.golang.org/grpc"

	"github.com/snobb/go-yact/internal/logger"

	pb "github.com/snobb/go-yact/pkg/protos/gen/tunnel/v1"
)

type Client struct {
	logger  logger.Logger
	address string

	grpcClient pb.TunnelServiceClient
}

func NewClient(logger logger.Logger, grpcClient pb.TunnelServiceClient, addr string) *Client {
	return &Client{
		logger:     logger,
		address:    addr,
		grpcClient: grpcClient,
	}
}

func (c *Client) Run(ctx context.Context, localPort int) error {
	logCtx := logger.WithAttrs(ctx,
		slog.String("entity", "client"))

	// register Listener
	registerProxyRequest := &pb.RegisterProxyRequest{BindAddress: c.address}

	registerProxyResponse, err := c.grpcClient.RegisterProxy(logCtx, registerProxyRequest)
	if err != nil {
		return err
	}

	c.logger.DebugContext(logCtx, "registered proxy listener", "address", registerProxyResponse.Address)

	eventStreamRequest := pb.EventStreamRequest{
		ListenerAddress: registerProxyResponse.Address,
	}

	eventStream, err := c.grpcClient.ListenEvents(logCtx, &eventStreamRequest)
	if err != nil {
		return err
	}

	for {
		select {
		case <-logCtx.Done():
			c.logger.DebugContext(logCtx, "client canceled by context")
			return nil
		default:
		}

		event, err := eventStream.Recv()
		if err != nil {
			return err
		}

		go c.handleConnectionEvent(logCtx, localPort, event)
	}
}

func (c *Client) handleConnectionEvent(ctx context.Context, localPort int, event *pb.NewConnectionEvent) {
	logCtx := logger.WithAttrs(ctx,
		slog.String("entity", "client"),
		slog.String("connection_id", event.ConnectionId),
		slog.String("remote_addr", event.RemoteAddr))

	c.logger.DebugContext(logCtx, "new connection event received")

	cancelCtx, cancel := context.WithCancel(logCtx)
	defer cancel()

	conn, err := makeLocalConn(localPort)
	if err != nil {
		c.logger.ErrorContext(logCtx, "failed to create local connection", "error", err)
		return
	}

	stream, err := c.grpcClient.OpenDataPipe(cancelCtx)
	if err != nil {
		c.logger.ErrorContext(logCtx, "failed to open data pipe", "error", err)
		_ = conn.Close()
		return
	}

	packet := &pb.DataPacket{ConnectionId: event.ConnectionId}

	if err := stream.Send(packet); err != nil {
		c.logger.ErrorContext(logCtx, "failed to send packet", "error", err)
		_ = conn.Close()
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer cancel()
		defer c.logger.DebugContext(logCtx, "client: recv pipe closed")
		c.receivePackets(cancelCtx, &wg, conn, stream)
	}()

	go func() {
		defer cancel()
		defer c.logger.DebugContext(logCtx, "client: send pipe closed")
		defer func() {
			_ = stream.CloseSend()
		}()

		c.sendPackets(cancelCtx, &wg, conn, stream)
	}()

	wg.Wait()
}

func (c *Client) receivePackets(
	ctx context.Context,
	wg *sync.WaitGroup,
	conn net.Conn,
	stream grpc.BidiStreamingClient[pb.DataPacket, pb.DataPacket],
) {
	defer wg.Done()
	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			c.logger.DebugContext(ctx, "data pipe canceled by context")
			return
		default:
		}

		packet, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				c.logger.DebugContext(ctx, "connection closed")
			} else {
				c.logger.ErrorContext(ctx, "failed to receive packet", "error", err)
			}
			return
		}

		if packet.Closed {
			c.logger.DebugContext(ctx, "connection closed")
			return
		}

		if len(packet.Payload) > 0 {
			if _, err := conn.Write(packet.Payload); err != nil {
				c.logger.ErrorContext(ctx, "failed to write packet payload", "error", err)
				return
			}
		}
	}
}

func (c *Client) sendPackets(
	ctx context.Context,
	wg *sync.WaitGroup,
	conn net.Conn,
	stream grpc.BidiStreamingClient[pb.DataPacket, pb.DataPacket],
) {
	defer wg.Done()
	defer conn.Close()

	buf := make([]byte, bufferSize)

	for {
		select {
		case <-ctx.Done():
			c.logger.DebugContext(ctx, "data pipe canceled by context")
			return
		default:
		}

		n, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				c.logger.DebugContext(ctx, "local socket closed")
			} else {
				c.logger.ErrorContext(ctx, "failed to read from local socket", "error", err)
			}
			return
		}

		if n == 0 {
			continue
		}

		payloadCopy := make([]byte, n)
		copy(payloadCopy, buf[:n])

		packet := &pb.DataPacket{Payload: payloadCopy}

		if err := stream.Send(packet); err != nil {
			c.logger.ErrorContext(ctx, "failed to send packet", "error", err)
			return
		}
	}
}

func makeLocalConn(port int) (net.Conn, error) {
	return net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
}
