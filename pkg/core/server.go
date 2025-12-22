package core

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	agentv1 "github.com/bhangun/mandau/api/v1"
	"github.com/bhangun/mandau/pkg/plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Core is the central control plane that manages multiple agents
type Core struct {
	agentv1.UnimplementedCoreServiceServer
	config  *CoreConfig
	agents  *AgentRegistry
	plugins *plugin.Registry
	audit   *AuditLogger
	authz   *Authorizer
}

type CoreConfig struct {
	ListenAddr string
	CertPath   string
	KeyPath    string
	CAPath     string
	PluginDir  string
}

type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]*AgentConnection
}

type AgentConnection struct {
	ID           string
	Hostname     string
	Address      string
	Labels       map[string]string
	Capabilities []string
	Client       grpc.ClientConnInterface
	LastSeen     time.Time
	Status       AgentStatus
}

type AgentStatus string

const (
	AgentStatusOnline  AgentStatus = "online"
	AgentStatusOffline AgentStatus = "offline"
	AgentStatusError   AgentStatus = "error"
)

type Authorizer struct {
	plugins *plugin.Registry
}

func NewAuthorizer(plugins *plugin.Registry) *Authorizer {
	return &Authorizer{plugins: plugins}
}

type AuditLogger struct {
	plugins *plugin.Registry
}

func NewAuditLogger(plugins *plugin.Registry) *AuditLogger {
	return &AuditLogger{plugins: plugins}
}

func (a *AuditLogger) LogAgentRegistration(ctx context.Context, agentID, hostname string) {
	log.Printf("Agent registered: ID=%s, Hostname=%s", agentID, hostname)
}

func (a *AuditLogger) LogAgentOffline(ctx context.Context, agentID string) {
	log.Printf("Agent went offline: ID=%s", agentID)
}

func NewCore(cfg *CoreConfig) (*Core, error) {
	plugins := plugin.NewRegistry()

	// Load plugins
	if err := loadPlugins(plugins, cfg.PluginDir); err != nil {
		return nil, fmt.Errorf("load plugins: %w", err)
	}

	return &Core{
		config:  cfg,
		agents:  &AgentRegistry{agents: make(map[string]*AgentConnection)},
		plugins: plugins,
		audit:   NewAuditLogger(plugins),
		authz:   NewAuthorizer(plugins),
	}, nil
}

func loadPlugins(plugins *plugin.Registry, dir string) error {
	// TODO: implement plugin loading logic
	return nil
}

func (c *Core) Serve() error {
	// mTLS configuration
	cert, err := tls.LoadX509KeyPair(c.config.CertPath, c.config.KeyPath)
	if err != nil {
		return fmt.Errorf("load cert: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}

	creds := credentials.NewTLS(tlsConfig)

	server := grpc.NewServer(
		grpc.Creds(creds),
		grpc.ChainUnaryInterceptor(
			c.authInterceptor,
			c.auditInterceptor,
		),
	)

	// Register Core API services
	agentv1.RegisterCoreServiceServer(server, c)

	lis, err := net.Listen("tcp", c.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	fmt.Printf("Core listening on %s\n", c.config.ListenAddr)

	// Start background services
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.monitorAgents(ctx)

	// Graceful shutdown
	go func() {
		// Wait for interrupt signal
		<-ctx.Done()
		log.Println("Shutting down server...")
		server.GracefulStop()
	}()

	return server.Serve(lis)
}

// RegisterAgent handles agent registration
func (c *Core) RegisterAgent(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	c.agents.mu.Lock()
	defer c.agents.mu.Unlock()

	agentID := generateAgentID(req.Hostname)

	// Create agent connection
	conn := &AgentConnection{
		ID:           agentID,
		Hostname:     req.Hostname,
		Labels:       req.Labels,
		Capabilities: req.Capabilities,
		LastSeen:     time.Now(),
		Status:       AgentStatusOnline,
	}

	c.agents.agents[agentID] = conn

	c.audit.LogAgentRegistration(ctx, agentID, req.Hostname)

	return &agentv1.RegisterResponse{
		AgentId:           agentID,
		HeartbeatInterval: durationpb.New(30 * time.Second),
	}, nil
}

// ListAgents returns all registered agents
func (c *Core) ListAgents(ctx context.Context, req *agentv1.ListAgentsRequest) (*agentv1.ListAgentsResponse, error) {
	c.agents.mu.RLock()
	defer c.agents.mu.RUnlock()

	agents := make([]*agentv1.Agent, 0, len(c.agents.agents))

	for _, agent := range c.agents.agents {
		agents = append(agents, &agentv1.Agent{
			Id:           agent.ID,
			Hostname:     agent.Hostname,
			Status:       string(agent.Status),
			Labels:       agent.Labels,
			Capabilities: agent.Capabilities,
			LastSeen:     timestamppb.New(agent.LastSeen),
		})
	}

	return &agentv1.ListAgentsResponse{
		Agents: agents,
	}, nil
}

// ProxyStackOperation forwards stack operations to the target agent
func (c *Core) ProxyStackOperation(ctx context.Context, agentID string, req *agentv1.ApplyStackRequest) (string, error) {
	conn, err := c.getAgentConnection(agentID)
	if err != nil {
		return "", err
	}

	// Create stack service client for this agent
	stackClient := agentv1.NewStackServiceClient(conn.Client)

	// Forward the operation
	stream, err := stackClient.ApplyStack(ctx, req)
	if err != nil {
		return "", fmt.Errorf("forward to agent: %w", err)
	}

	// Get operation ID from first event
	event, err := stream.Recv()
	if err != nil {
		return "", err
	}

	return event.OperationId, nil
}

func (c *Core) getAgentConnection(agentID string) (*AgentConnection, error) {
	c.agents.mu.RLock()
	defer c.agents.mu.RUnlock()

	conn, exists := c.agents.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	if conn.Status != AgentStatusOnline {
		return nil, fmt.Errorf("agent offline: %s", agentID)
	}

	return conn, nil
}

// monitorAgents checks agent health periodically
func (c *Core) monitorAgents(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.agents.mu.Lock()

			for id, agent := range c.agents.agents {
				if time.Since(agent.LastSeen) > 90*time.Second {
					agent.Status = AgentStatusOffline
					c.audit.LogAgentOffline(ctx, id)
				}
			}

			c.agents.mu.Unlock()
		}
	}
}

func generateAgentID(hostname string) string {
	// Simple ID generation, in production use UUID
	return fmt.Sprintf("agent-%s-%d", hostname, time.Now().Unix())
}

func (c *Core) authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	identity, err := extractIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}

	if auth := c.plugins.Auth(); auth != nil {
		identity, err = auth.Authenticate(ctx, &plugin.AuthRequest{
			Identity: identity,
			Method:   info.FullMethod,
		})
		if err != nil {
			return nil, fmt.Errorf("auth failed: %w", err)
		}
	}

	ctx = plugin.WithIdentity(ctx, identity)
	return handler(ctx, req)
}

func (c *Core) auditInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	identity := plugin.IdentityFromContext(ctx)

	resp, err := handler(ctx, req)

	c.plugins.AuditAll(ctx, &plugin.AuditEntry{
		Timestamp: start,
		Identity:  identity,
		Action:    info.FullMethod,
		Result:    resultString(err),
		Duration:  time.Since(start),
	})

	return resp, err
}

// extractIdentity extracts the client identity from the gRPC context
func extractIdentity(ctx context.Context) (*plugin.Identity, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no peer found")
	}

	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, fmt.Errorf("unexpected peer transport credentials")
	}

	if len(tlsInfo.State.VerifiedChains) == 0 || len(tlsInfo.State.VerifiedChains[0]) == 0 {
		return nil, fmt.Errorf("could not verify peer certificate")
	}

	// Use the subject of the client certificate as identity
	subject := tlsInfo.State.VerifiedChains[0][0].Subject
	return &plugin.Identity{CommonName: subject.CommonName}, nil
}
