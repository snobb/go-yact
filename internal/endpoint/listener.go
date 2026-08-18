package endpoint

import (
	"context"
	"net"

	"github.com/google/uuid"

	pb "github.com/snobb/go-yact/pkg/protos/gen/tunnel/v1"
)

const (
	EventBufferSize = 1024
)

var makeListenerFunc = func(address string) (net.Listener, error) {
	return net.Listen("tcp", address)
}

// OnNewConnectionFunc is a function that is called when a new connection is accepted.
type OnNewConnectionFunc func(connectionID string, rawConn net.Conn)

// ProxyListener represents a listener on the proxy server.
type ProxyListener struct {
	OwnerClientID string // mTLS Identity (e.g. "client-a.internal")
	address       string // "10.0.0.1:443"

	// Event channel used to trigger NewConnection events to the gRPC stream
	eventChan chan *pb.NewConnectionEvent

	cancelFunc context.CancelFunc
}

// NewListener returns a new ProxyListener.
func NewListener(ctx context.Context, address, ownerID string) *ProxyListener {
	return &ProxyListener{
		OwnerClientID: ownerID,
		address:       address,
	}
}

// Address returns the address of the listener.
func (l *ProxyListener) Address() string {
	return l.address
}

// Listen starts the listener on the proxy server.
func (l *ProxyListener) Listen(ctx context.Context, onNewConn OnNewConnectionFunc) error {
	if l.cancelFunc != nil {
		return ErrListenerAlreadyRunning
	}

	listener, err := makeListenerFunc(l.address)
	if err != nil {
		return err
	}

	l.eventChan = make(chan *pb.NewConnectionEvent, EventBufferSize)

	cancelCtx, cancelFunc := context.WithCancel(ctx)
	l.cancelFunc = cancelFunc

	go l.createListenLoop(cancelCtx, listener, onNewConn)

	return nil
}

func (l *ProxyListener) Close() {
	if l.cancelFunc != nil {
		l.cancelFunc()
	}
}

func (l *ProxyListener) EventChannel() <-chan *pb.NewConnectionEvent {
	return l.eventChan
}

func (l *ProxyListener) createListenLoop(ctx context.Context, rawListener net.Listener, onNewConn OnNewConnectionFunc) {
	defer close(l.eventChan)
	defer rawListener.Close()

	go func() {
		<-ctx.Done()
		rawListener.Close()
	}()

	for {
		rawConn, err := rawListener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue // keep on listening on temporary errors
			}
		}

		connectionID := uuid.NewString()
		remoteAddress := rawConn.RemoteAddr().String()

		if onNewConn != nil {
			onNewConn(connectionID, rawConn)
		}

		event := &pb.NewConnectionEvent{
			ConnectionId: connectionID,
			RemoteAddr:   remoteAddress,
		}

		select {
		case l.eventChan <- event:
		// Send event to gRPC stream
		case <-ctx.Done():
			return
		}
	}
}
