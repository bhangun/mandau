package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages plugin lifecycle
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	auth    []AuthPlugin
	audit   []AuditPlugin
	secrets []SecretsPlugin
	policy  []PolicyPlugin
}

func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

// Register adds a plugin to the registry
func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	r.plugins[name] = p

	// Type-specific registration
	if auth, ok := p.(AuthPlugin); ok {
		r.auth = append(r.auth, auth)
	}
	if audit, ok := p.(AuditPlugin); ok {
		r.audit = append(r.audit, audit)
	}
	if secrets, ok := p.(SecretsPlugin); ok {
		r.secrets = append(r.secrets, secrets)
	}
	if policy, ok := p.(PolicyPlugin); ok {
		r.policy = append(r.policy, policy)
	}

	return nil
}

// Init initializes all registered plugins
func (r *Registry) Init(ctx context.Context, configs map[string]map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, p := range r.plugins {
		config := configs[name]
		if err := p.Init(ctx, config); err != nil {
			return fmt.Errorf("plugin %s init failed: %w", name, err)
		}
	}

	return nil
}

// Auth returns the first auth plugin (chain support later)
func (r *Registry) Auth() AuthPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.auth) > 0 {
		return r.auth[0]
	}
	return nil
}

// AuditAll logs to all audit plugins
func (r *Registry) AuditAll(ctx context.Context, entry *AuditEntry) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, audit := range r.audit {
		// Never fail on audit - just log errors
		audit.Log(ctx, entry)
	}
}

// ListAll returns all registered plugins
func (r *Registry) ListAll() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// ShutdownAll shuts down all plugins
func (r *Registry) ShutdownAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var lastErr error
	for _, p := range r.plugins {
		if err := p.Shutdown(ctx); err != nil {
			lastErr = err // Keep the last error
		}
	}
	return lastErr
}

// Policy returns the first policy plugin
func (r *Registry) Policy() PolicyPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.policy) > 0 {
		return r.policy[0]
	}
	return nil
}
