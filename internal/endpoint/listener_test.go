package endpoint

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestListener_Address(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{"ipv4 loopback", "127.0.0.1:8080"},
		{"any address", "0.0.0.0:9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockListener(tt.address)
			useMockListener(t, mock)
			l := NewListener(context.Background(), tt.address, "client-a")

			if got := l.Address(); got != tt.address {
				t.Fatalf("Address() = %q, want %q", got, tt.address)
			}
		})
	}
}

func TestListener_ListenAndEvent(t *testing.T) {
	mock := newMockListener("127.0.0.1:8080")
	useMockListener(t, mock)

	ctx := context.Background()
	l := NewListener(ctx, "127.0.0.1:8080", "client-a")

	if err := l.Listen(ctx, nil); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	c, s := net.Pipe()
	defer s.Close()
	mock.injectConn(c)

	select {
	case event := <-l.EventChannel():
		if event.ConnectionId == "" {
			t.Fatal("expected non-empty connection ID")
		}
		if event.RemoteAddr == "" {
			t.Fatal("expected non-empty remote addr")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestListener_ListenWithOnNewConn(t *testing.T) {
	mock := newMockListener("127.0.0.1:8080")
	useMockListener(t, mock)

	ctx := context.Background()
	l := NewListener(ctx, "127.0.0.1:8080", "client-a")

	var receivedID string
	var receivedConn net.Conn

	onNewConn := func(connectionID string, rawConn net.Conn) {
		receivedID = connectionID
		receivedConn = rawConn
	}

	if err := l.Listen(ctx, onNewConn); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	c, s := net.Pipe()
	defer s.Close()
	mock.injectConn(c)

	select {
	case <-l.EventChannel():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	if receivedID == "" {
		t.Fatal("onNewConn was not called")
	}
	if receivedConn != c {
		t.Fatal("onNewConn received wrong conn")
	}
}

func TestListener_AlreadyRunning(t *testing.T) {
	mock := newMockListener("127.0.0.1:8080")
	useMockListener(t, mock)

	ctx := context.Background()
	l := NewListener(ctx, "127.0.0.1:8080", "client-a")

	if err := l.Listen(ctx, nil); err != nil {
		t.Fatalf("first Listen failed: %v", err)
	}
	defer l.Close()

	err := l.Listen(ctx, nil)
	if err != ErrListenerAlreadyRunning {
		t.Fatalf("second Listen: got err %v, want ErrListenerAlreadyRunning", err)
	}
}

func TestListener_Close(t *testing.T) {
	mock := newMockListener("127.0.0.1:8080")
	useMockListener(t, mock)

	ctx := context.Background()
	l := NewListener(ctx, "127.0.0.1:8080", "client-a")

	if err := l.Listen(ctx, nil); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	l.Close()

	// wait for the goroutine to exit (eventChan closes when it does)
	select {
	case _, ok := <-l.EventChannel():
		if ok {
			for range l.EventChannel() {
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event channel to close")
	}

	mock.closeMu.Lock()
	closed := mock.closed
	mock.closeMu.Unlock()
	if !closed {
		t.Fatal("mock listener was not closed")
	}
}

func TestListener_CloseWithoutListen(t *testing.T) {
	l := NewListener(context.Background(), "127.0.0.1:8080", "client-a")

	// Close without Listen should not panic
	l.Close()
}

func TestListener_NilOnNewConn(t *testing.T) {
	mock := newMockListener("127.0.0.1:8080")
	useMockListener(t, mock)

	ctx := context.Background()
	l := NewListener(ctx, "127.0.0.1:8080", "client-a")

	if err := l.Listen(ctx, nil); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	c, s := net.Pipe()
	defer s.Close()
	mock.injectConn(c)

	select {
	case event := <-l.EventChannel():
		if event.ConnectionId == "" {
			t.Fatal("expected non-empty connection ID even with nil onNewConn")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestListener_ListenFailsOnMakeError(t *testing.T) {
	useErrListener(t)
	l := NewListener(context.Background(), "127.0.0.1:8080", "client-a")

	err := l.Listen(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error from Listen when makeListenerFunc fails")
	}
}
