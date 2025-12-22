package plugin

import (
	"context"
	"time"
)

// Plugin is the core interface all Mandau plugins must implement
type Plugin interface {
	// Name returns unique plugin identifier
	Name() string

	// Version returns semantic version
	Version() string

	// Init is called once during plugin load
	Init(ctx context.Context, config map[string]interface{}) error

	// Shutdown gracefully stops the plugin
	Shutdown(ctx context.Context) error

	// Capabilities returns what this plugin provides
	Capabilities() []Capability
}

type Capability string

const (
	CapabilityAuth    Capability = "auth"
	CapabilityAudit   Capability = "audit"
	CapabilitySecrets Capability = "secrets"
	CapabilityPolicy  Capability = "policy"
	CapabilityStorage Capability = "storage"
	CapabilityMonitor Capability = "monitoring"
	CapabilityNotify  Capability = "notifications"
)

// AuthPlugin handles authentication and authorization
type AuthPlugin interface {
	Plugin

	// Authenticate validates credentials and returns identity
	Authenticate(ctx context.Context, req *AuthRequest) (*Identity, error)

	// Authorize checks if identity can perform action
	Authorize(ctx context.Context, identity *Identity, action *Action) error
}

// AuditPlugin logs all security-relevant events
type AuditPlugin interface {
	Plugin

	// Log writes an audit entry (must not fail)
	Log(ctx context.Context, entry *AuditEntry)

	// Query retrieves audit logs
	Query(ctx context.Context, filter *AuditFilter) ([]AuditEntry, error)
}

type AuditEntry struct {
	Timestamp      time.Time
	AgentID        string
	Identity       *Identity
	Action         string
	Resource       string
	Result         string
	Duration       time.Duration
	Metadata       map[string]string
	TranscriptHash string // For terminal sessions
}

// SecretsPlugin manages secret injection
type SecretsPlugin interface {
	Plugin

	// Get retrieves a secret by key
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a secret
	Set(ctx context.Context, key string, value []byte) error

	// Delete removes a secret
	Delete(ctx context.Context, key string) error

	// InjectEnv injects secrets into environment map
	InjectEnv(ctx context.Context, env map[string]string) error
}

// PolicyPlugin enforces fine-grained access control
type PolicyPlugin interface {
	Plugin

	// Evaluate checks if action is permitted
	Evaluate(ctx context.Context, req *PolicyRequest) (*PolicyDecision, error)
}

type PolicyRequest struct {
	Identity *Identity
	Action   *Action
	Resource *Resource
	Context  map[string]interface{}
}

type PolicyDecision struct {
	Allowed     bool
	Reason      string
	Obligations []string // Additional requirements
}
