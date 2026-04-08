package transport

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
)

// Manager handles transport with automatic fallback
type Manager struct {
	primary   Client
	fallback  Client
	active    Client
	config    Config
	mu        sync.RWMutex
	reconnect chan struct{}
}

// NewManager creates a new transport manager with fallback support
func NewManager(config Config) *Manager {
	primary := NewGRPCClient(config)

	var fallback Client
	if config.Type == WebSocket {
		fallback = NewWSClient(config)
	}

	return &Manager{
		primary:   primary,
		fallback:  fallback,
		active:    primary,
		config:    config,
		reconnect: make(chan struct{}, 1),
	}
}

// Connect attempts to connect with primary, falls back to secondary on failure
func (m *Manager) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try primary (gRPC)
	err := m.primary.Connect(ctx)
	if err == nil {
		m.active = m.primary
		return nil
	}

	// Primary failed, try fallback
	if m.fallback != nil {
		err = m.fallback.Connect(ctx)
		if err == nil {
			m.active = m.fallback
			return nil
		}
	}

	return fmt.Errorf("failed to connect via all transports: %w", err)
}

// ConnectWithRetry attempts to connect with exponential backoff
func (m *Manager) ConnectWithRetry(ctx context.Context, maxRetries int, baseDelay time.Duration) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := m.Connect(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Exponential backoff
		waitTime := baseDelay * time.Duration(1<<uint(attempt))
		if waitTime > 30*time.Second {
			waitTime = 30 * time.Second
		}

		timer := time.NewTimer(waitTime)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}

	return fmt.Errorf("exhausted %d retries: %w", maxRetries, lastErr)
}

// Close closes all transport connections
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	if err := m.primary.Close(); err != nil {
		errs = append(errs, err)
	}

	if m.fallback != nil {
		if err := m.fallback.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing transports: %v", errs)
	}

	return nil
}

// GRPCConn returns the active gRPC connection (may be nil if using WebSocket)
func (m *Manager) GRPCConn() *grpc.ClientConn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active.GRPCConn()
}

// Active returns the currently active transport
func (m *Manager) Active() Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// ActiveType returns the type of the active transport
func (m *Manager) ActiveType() Type {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active.Type()
}

// IsConnected returns true if any transport is connected
func (m *Manager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active.IsConnected()
}

// Failover switches to the fallback transport if available
func (m *Manager) Failover(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fallback == nil {
		return fmt.Errorf("no fallback transport configured")
	}

	// If currently on primary, switch to fallback
	if m.active == m.primary {
		err := m.fallback.Connect(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect to fallback: %w", err)
		}
		m.active = m.fallback
		return nil
	}

	// If on fallback, try to reconnect to primary
	err := m.primary.Connect(ctx)
	if err == nil {
		m.active = m.primary
		return nil
	}

	return fmt.Errorf("failed to failover: %w", err)
}

// Reconnect attempts to reconnect to the preferred transport
func (m *Manager) Reconnect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close current active
	if err := m.active.Close(); err != nil {
		// Continue anyway
	}

	// Try primary first
	err := m.primary.Connect(ctx)
	if err == nil {
		m.active = m.primary
		return nil
	}

	// Try fallback
	if m.fallback != nil {
		err = m.fallback.Connect(ctx)
		if err == nil {
			m.active = m.fallback
			return nil
		}
	}

	return fmt.Errorf("failed to reconnect: %w", err)
}
