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
	listenerMU sync.RWMutex
	listeners  map[string]*ProxyListener

	connMU       sync.RWMutex
	pendingConns map[string]*ProxyConn
}

// NewRegistry creates a new Registry.
func NewRegistry() *ListenerRegistry {
	return &ListenerRegistry{
		listeners: make(map[string]*ProxyListener),
	}
}

// Register registers a listener on the proxy server.
func (r *ListenerRegistry) Register(ctx context.Context, address, ownerID string, onNewConn OnNewConnectionFunc) error {
	r.listenerMU.Lock()
	defer r.listenerMU.Unlock()

	if _, ok := r.listeners[address]; ok {
		return ErrListenerExists
	}

	listener := NewListener(ctx, address, ownerID)

	r.listeners[address] = listener

	if err := listener.Listen(ctx, onNewConn); err != nil {
		return err
	}

	return nil
}

// GetListener returns a listener from the registry.
func (r *ListenerRegistry) GetListenerByAddress(address string) (*ProxyListener, error) {
	r.listenerMU.RLock()
	defer r.listenerMU.RUnlock()

	listener, ok := r.listeners[address]
	if !ok {
		return nil, ErrListenerNotFound
	}
	return listener, nil
}

// RemoveListener removes a listener from the registry.
func (r *ListenerRegistry) RemoveListenerByAddress(address string) {
	r.listenerMU.Lock()
	defer r.listenerMU.Unlock()
	delete(r.listeners, address)
}
