package plugin

import (
	"context"
	"time"
)

// Identity represents an authenticated entity
type Identity struct {
	UserID      string
	DeviceID    string
	Roles       []string
	Attributes  map[string]string
	Certificate []byte
}

// Action represents an operation being performed
type Action struct {
	Method   string
	Action   string // read, write, execute, delete
	Resource string // e.g., "stack:web", "container:nginx"
}

// Resource represents the target of an action
type Resource struct {
	Type       string // stack, container, image, file
	Identifier string
	Labels     map[string]string
}

// AuthRequest for authentication
type AuthRequest struct {
	Identity *Identity
	Method   string
	Token    string
}

// AuditFilter for querying audit logs
type AuditFilter struct {
	AgentID   string
	UserID    string
	Action    string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
}

// Context helpers
type contextKey string

const identityKey contextKey = "identity"

func WithIdentity(ctx context.Context, identity *Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}

func IdentityFromContext(ctx context.Context) *Identity {
	if identity, ok := ctx.Value(identityKey).(*Identity); ok {
		return identity
	}
	return nil
}
