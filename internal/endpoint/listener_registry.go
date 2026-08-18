package endpoint

import (
	"context"
	"errors"
	"sync"
)

// Errors for the endpoint package.
var (
	ErrListenerAlreadyRunning = errors.New("listener already running")
	ErrListenerExists         = errors.New("listener already exists")
	ErrListenerNotFound       = errors.New("listener not found")
	ErrPermissionDenied       = errors.New("permission denied")
)

// ListenerRegistry represents a collection of listeners on the proxy server.
type ListenerRegistry struct {
	listenerMU          sync.RWMutex
	listenersByAddress  map[string]*ProxyListener
	listenersByClientID map[string]*ProxyListener
}

// NewRegistry creates a new Registry.
func NewRegistry() *ListenerRegistry {
	return &ListenerRegistry{
		listenersByAddress:  make(map[string]*ProxyListener),
		listenersByClientID: make(map[string]*ProxyListener),
	}
}

// Register registers a listener on the proxy server.
func (r *ListenerRegistry) Register(ctx context.Context, address, ownerID string, onNewConn OnNewConnectionFunc) error {
	r.listenerMU.Lock()
	defer r.listenerMU.Unlock()

	if _, ok := r.listenersByAddress[address]; ok {
		return ErrListenerExists
	}

	listener := NewListener(ctx, address, ownerID)

	if err := listener.Listen(ctx, onNewConn); err != nil {
		return err
	}

	r.listenersByAddress[address] = listener
	r.listenersByClientID[ownerID] = listener

	return nil
}

// GetListener returns a listener from the registry.
func (r *ListenerRegistry) GetListenerByAddress(address string) (*ProxyListener, error) {
	r.listenerMU.RLock()
	defer r.listenerMU.RUnlock()

	listener, ok := r.listenersByAddress[address]
	if !ok {
		return nil, ErrListenerNotFound
	}
	return listener, nil
}

func (r *ListenerRegistry) RemoveListenerByClientID(clientID string) {
	r.listenerMU.Lock()
	defer r.listenerMU.Unlock()

	listener, ok := r.listenersByClientID[clientID]
	if !ok {
		return
	}

	listener.Close()

	delete(r.listenersByClientID, clientID)
	delete(r.listenersByAddress, listener.Address())

}
