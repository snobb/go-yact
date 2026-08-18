package endpoint

import (
	"net"
	"sync"
	"testing"
)

type mockAddr struct {
	network string
	address string
}

func (a mockAddr) Network() string { return a.network }
func (a mockAddr) String() string  { return a.address }

// mockListener implements net.Listener for testing.
type mockListener struct {
	addr    net.Addr
	conns   chan net.Conn
	closeCh chan struct{}

	closed  bool
	closeMu sync.Mutex
}

func newMockListener(addr string) *mockListener {
	return &mockListener{
		addr:    mockAddr{network: "tcp", address: addr},
		conns:   make(chan net.Conn, 1),
		closeCh: make(chan struct{}),
	}
}

func (m *mockListener) Accept() (net.Conn, error) {
	select {
	case conn := <-m.conns:
		return conn, nil
	case <-m.closeCh:
		return nil, net.ErrClosed
	}
}

func (m *mockListener) Close() error {
	m.closeMu.Lock()
	defer m.closeMu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.closeCh)
	}
	return nil
}

func (m *mockListener) Addr() net.Addr {
	return m.addr
}

// injectConn makes a conn available on the next Accept call.
func (m *mockListener) injectConn(conn net.Conn) {
	m.conns <- conn
}

// useMockListener overrides the package-level makeListenerFunc and restores it on test cleanup.
func useMockListener(t *testing.T, mock *mockListener) {
	t.Helper()
	orig := makeListenerFunc
	makeListenerFunc = func(address string) (net.Listener, error) {
		return mock, nil
	}
	t.Cleanup(func() { makeListenerFunc = orig })
}

// useErrListener overrides makeListenerFunc to always fail.
func useErrListener(t *testing.T) {
	t.Helper()
	orig := makeListenerFunc
	makeListenerFunc = func(address string) (net.Listener, error) {
		return nil, &net.OpError{Op: "listen", Net: "tcp"}
	}
	t.Cleanup(func() { makeListenerFunc = orig })
}
