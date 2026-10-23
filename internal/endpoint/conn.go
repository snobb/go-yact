// Package endpoint provides server endpoint functionality.
package endpoint

import (
	"errors"
	"net"
	"sync"
)

// ErrConnectionNotFound is returned when a connection is not found in the registry.
var ErrConnectionNotFound = errors.New("connection not found")

// ProxyConn represents a connection on the proxy server.
type ProxyConn struct {
	ID            string
	OwnerClientID string

	Conn net.Conn
}

// ConnRegistry represents a collection of connections on the proxy server.
type ConnRegistry struct {
	mu    sync.Mutex
	conns map[string]*ProxyConn
}

// NewConnRegistry creates a new ConnRegistry.
func NewConnRegistry() *ConnRegistry {
	return &ConnRegistry{
		conns: make(map[string]*ProxyConn),
	}
}

// AddConn adds a connection to the registry.
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
		return nil, ErrConnectionNotFound
	}

	if conn.OwnerClientID != clientID {
		return nil, ErrPermissionDenied
	}

	delete(r.conns, connectionID) // delete conn to make sure it's not claimed again.

	return conn.Conn, nil
}
