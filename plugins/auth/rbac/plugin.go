package rbac

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/bhangun/mandau/pkg/plugin"
	"gopkg.in/yaml.v3"
)

type RBACPlugin struct {
	name    string
	version string
	mu      sync.RWMutex
	roles   map[string]*Role
	users   map[string]*User
}

type Role struct {
	Name        string
	Permissions []Permission
}

type Permission struct {
	Resource string   // e.g., "stack:*", "container:web-*"
	Actions  []string // e.g., ["read", "write", "execute"]
}

type User struct {
	ID    string
	Name  string
	Roles []string
}

func New() *RBACPlugin {
	return &RBACPlugin{
		name:    "rbac-auth",
		version: "1.0.0",
		roles:   make(map[string]*Role),
		users:   make(map[string]*User),
	}
}

func (p *RBACPlugin) Name() string    { return p.name }
func (p *RBACPlugin) Version() string { return p.version }

func (p *RBACPlugin) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilityAuth, plugin.CapabilityPolicy}
}

func (p *RBACPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	// Load roles from config
	rolesConfig, ok := config["roles"].(string)
	if !ok {
		return p.loadDefaultRoles()
	}

	return p.loadRolesFromYAML([]byte(rolesConfig))
}

func (p *RBACPlugin) loadDefaultRoles() error {
	// Admin role - full access
	p.roles["admin"] = &Role{
		Name: "admin",
		Permissions: []Permission{
			{Resource: "*", Actions: []string{"*"}},
		},
	}

	// Operator role - read/write stacks and containers
	p.roles["operator"] = &Role{
		Name: "operator",
		Permissions: []Permission{
			{Resource: "stack:*", Actions: []string{"read", "write"}},
			{Resource: "container:*", Actions: []string{"read", "exec"}},
			{Resource: "image:*", Actions: []string{"read"}},
		},
	}

	// Viewer role - read-only
	p.roles["viewer"] = &Role{
		Name: "viewer",
		Permissions: []Permission{
			{Resource: "*", Actions: []string{"read"}},
		},
	}

	return nil
}

func (p *RBACPlugin) loadRolesFromYAML(data []byte) error {
	var config struct {
		Roles []Role `yaml:"roles"`
		Users []User `yaml:"users"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, role := range config.Roles {
		p.roles[role.Name] = &role
	}

	for _, user := range config.Users {
		p.users[user.ID] = &user
	}

	return nil
}

func (p *RBACPlugin) Authenticate(ctx context.Context, req *plugin.AuthRequest) (*plugin.Identity, error) {
	// In certificate-based auth, the UserID from the identity comes from the certificate's Common Name
	// We need to verify that the certificate CN maps to an existing user in our system
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check if the identity's UserID (which comes from certificate CN) exists as a user
	_, exists := p.users[req.Identity.UserID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", req.Identity.UserID)
	}

	// Return the identity as-is, which will be used for authorization
	return req.Identity, nil
}

func (p *RBACPlugin) Authorize(ctx context.Context, identity *plugin.Identity, action *plugin.Action) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	user, exists := p.users[identity.UserID]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Check all user roles
	for _, roleName := range user.Roles {
		role, exists := p.roles[roleName]
		if !exists {
			continue
		}

		if p.roleHasPermission(role, action) {
			return nil
		}
	}

	return fmt.Errorf("permission denied")
}

func (p *RBACPlugin) roleHasPermission(role *Role, action *plugin.Action) bool {
	for _, perm := range role.Permissions {
		if p.matchesResource(perm.Resource, action.Resource) {
			if p.matchesAction(perm.Actions, action.Action) {
				return true
			}
		}
	}
	return false
}

func (p *RBACPlugin) matchesResource(pattern, resource string) bool {
	if pattern == "*" {
		return true
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(resource, prefix)
	}

	return pattern == resource
}

func (p *RBACPlugin) matchesAction(allowed []string, action string) bool {
	for _, a := range allowed {
		if a == "*" || a == action {
			return true
		}
	}
	return false
}

func (p *RBACPlugin) Shutdown(ctx context.Context) error {
	return nil
}

// Policy interface implementation
func (p *RBACPlugin) Evaluate(ctx context.Context, req *plugin.PolicyRequest) (*plugin.PolicyDecision, error) {
	err := p.Authorize(ctx, req.Identity, req.Action)

	decision := &plugin.PolicyDecision{
		Allowed: err == nil,
	}

	if err != nil {
		decision.Reason = err.Error()
	}

	return decision, nil
}
