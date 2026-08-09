package endpoint

import (
	"net"
	"sync"
)

type ProxyConn struct {
	ID            string
	OwnerClientID string

	Conn net.Conn
}

type ConnRegistry struct {
	mu    sync.Mutex
	conns map[string]*ProxyConn
}

// NewRegistry creates a new Registry.
func NewConnRegistry() *ConnRegistry {
	return &ConnRegistry{
		conns: make(map[string]*ProxyConn),
	}
}

func (r *ConnRegistry) AddConn(conn net.Conn, connectionID, ownerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.conns[connectionID] = &ProxyConn{
		ID:            connectionID,
		OwnerClientID: ownerID,
		Conn:          conn,
	}
}

// ConnByConnectionID returns a connection from the registry and must do the following:
//  1. Atomically Claims (Removes): Uses LoadAndDelete so no other concurrent OpenDataPipe call
//     can claim the same connection_id.
//  2. Validates Client Ownership: Verifies that the listener that accepted this conn belongs to
//     clientID. Otherwise, Client B could guess Client A's connectionID and steal the data
//     stream.
func (r *ConnRegistry) ConnByConnectionID(connectionID, clientID string) (net.Conn, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn, ok := r.conns[connectionID]
	if !ok {
		return nil, ErrListenerNotFound
	}

	if conn.OwnerClientID != clientID {
		return nil, ErrPermissionDenied
	}

	delete(r.conns, connectionID) // delete conn to make sure it's not claimed again.

	return conn.Conn, nil
}
