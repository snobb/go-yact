package endpoint

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		clientID  string
		wantOwner string
	}{
		{"single client", "127.0.0.1:9000", "client-a", "client-a"},
		{"different address", "127.0.0.1:9001", "client-b", "client-b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockListener(tt.address)
			useMockListener(t, mock)
			reg := NewRegistry()

			if err := reg.Register(context.Background(), tt.address, tt.clientID, nil); err != nil {
				t.Fatalf("Register failed: %v", err)
			}
			defer reg.RemoveListenerByClientID(tt.clientID)

			listener, err := reg.GetListenerByAddress(tt.address)
			if err != nil {
				t.Fatalf("GetListenerByAddress failed: %v", err)
			}
			if listener.Address() != tt.address {
				t.Fatalf("Address() = %q, want %q", listener.Address(), tt.address)
			}
			if listener.OwnerClientID != tt.wantOwner {
				t.Fatalf("OwnerClientID = %q, want %q", listener.OwnerClientID, tt.wantOwner)
			}
		})
	}
}

func TestRegistry_DuplicateAddress(t *testing.T) {
	mock := newMockListener("127.0.0.1:9000")
	useMockListener(t, mock)
	reg := NewRegistry()

	if err := reg.Register(context.Background(), "127.0.0.1:9000", "client-a", nil); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	defer reg.RemoveListenerByClientID("client-a")

	err := reg.Register(context.Background(), "127.0.0.1:9000", "client-b", nil)
	if err != ErrListenerExists {
		t.Fatalf("got err %v, want ErrListenerExists", err)
	}
}

func TestRegistry_RemoveByClientID(t *testing.T) {
	mock := newMockListener("127.0.0.1:9000")
	useMockListener(t, mock)
	reg := NewRegistry()

	if err := reg.Register(context.Background(), "127.0.0.1:9000", "client-a", nil); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	listener, _ := reg.GetListenerByAddress("127.0.0.1:9000")
	reg.RemoveListenerByClientID("client-a")

	// wait for the goroutine to exit (eventChan closes when it does)
	select {
	case _, ok := <-listener.EventChannel():
		if ok {
			for range listener.EventChannel() {
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event channel to close")
	}

	// listener should be removed from both maps
	_, err := reg.GetListenerByAddress("127.0.0.1:9000")
	if err != ErrListenerNotFound {
		t.Fatalf("got err %v, want ErrListenerNotFound", err)
	}

	// mock should be closed
	mock.closeMu.Lock()
	closed := mock.closed
	mock.closeMu.Unlock()
	if !closed {
		t.Fatal("mock listener was not closed after RemoveListenerByClientID")
	}
}

func TestRegistry_RemoveByClientID_NotFound(t *testing.T) {
	reg := NewRegistry()

	// should not panic
	reg.RemoveListenerByClientID("nonexistent")
}

func TestRegistry_GetListener_NotFound(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.GetListenerByAddress("nonexistent")
	if err != ErrListenerNotFound {
		t.Fatalf("got err %v, want ErrListenerNotFound", err)
	}
}

func TestRegistry_MockIntegration(t *testing.T) {
	mock := newMockListener("127.0.0.1:9000")
	useMockListener(t, mock)
	reg := NewRegistry()

	ctx := context.Background()

	var receivedConns []string
	onNewConn := func(connectionID string, rawConn net.Conn) {
		receivedConns = append(receivedConns, connectionID)
	}

	if err := reg.Register(ctx, "127.0.0.1:9000", "client-a", onNewConn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer reg.RemoveListenerByClientID("client-a")

	listener, err := reg.GetListenerByAddress("127.0.0.1:9000")
	if err != nil {
		t.Fatalf("GetListenerByAddress failed: %v", err)
	}

	c, s := net.Pipe()
	defer s.Close()
	mock.injectConn(c)

	select {
	case event := <-listener.EventChannel():
		if event.ConnectionId == "" {
			t.Fatal("expected non-empty connection ID")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	if len(receivedConns) != 1 {
		t.Fatalf("expected 1 received conn, got %d", len(receivedConns))
	}
}
