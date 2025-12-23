package core

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	agentv1 "github.com/bhangun/mandau/api/v1"
	"github.com/bhangun/mandau/pkg/plugin"
	"github.com/bhangun/mandau/plugins/auth/rbac"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Core is the central control plane that manages multiple agents
type Core struct {
	agentv1.UnimplementedCoreServiceServer
	agentv1.UnimplementedStackServiceServer
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
	Client       *grpc.ClientConn  // Changed from grpc.ClientConnInterface to *grpc.ClientConn
	LastSeen     time.Time
	Status       AgentStatus
	Stacks       []string // List of stack IDs/names on this agent
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
	// For now, register built-in plugins with default configuration
	// In a real system, this would load plugin configurations from files

	// Load RBAC plugin with default configuration
	rbacPlugin := rbac.New()

	// Default configuration for RBAC plugin - map certificate CN to users
	// In production, this would be loaded from a config file
	rbacConfig := map[string]interface{}{
		"roles": `
roles:
  - name: admin
    permissions:
      - resource: "*"
        actions: ["*"]
  - name: operator
    permissions:
      - resource: "stack:*"
        actions: ["read", "write", "delete"]
      - resource: "container:*"
        actions: ["read", "exec", "logs"]
      - resource: "image:*"
        actions: ["read", "pull"]
      - resource: "file:*"
        actions: ["read", "write"]
  - name: viewer
    permissions:
      - resource: "*"
        actions: ["read", "logs"]
users:
  - id: "mandau-cli"
    name: "CLI User"
    roles: ["admin"]
  - id: "mandau-agent"
    name: "Agent User"
    roles: ["admin"]  # Agent needs admin rights to manage stacks on its host
  - id: "admin@example.com"
    name: "Administrator"
    roles: ["admin"]
  - id: "ops@example.com"
    name: "Operations Team"
    roles: ["operator"]
`,
	}

	if err := plugins.Register(rbacPlugin); err != nil {
		return fmt.Errorf("register rbac plugin: %w", err)
	}

	// Initialize plugins with configuration
	if err := plugins.Init(context.Background(), map[string]map[string]interface{}{
		"rbac-auth": rbacConfig,
	}); err != nil {
		return fmt.Errorf("init plugins: %w", err)
	}

	return nil
}

func (c *Core) Serve() error {
	// mTLS configuration
	cert, err := tls.LoadX509KeyPair(c.config.CertPath, c.config.KeyPath)
	if err != nil {
		return fmt.Errorf("load cert: %w", err)
	}

	// Load CA certificate to verify client certificates
	caCert, err := ioutil.ReadFile(c.config.CAPath)
	if err != nil {
		return fmt.Errorf("load CA cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("parse CA cert")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
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
	agentv1.RegisterStackServiceServer(server, c)

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

	// Create agent connection record without client initially
	// The agent should provide its address or we need to discover it
	// For now, we'll create a placeholder and try to connect later
	agentConn := &AgentConnection{
		ID:           agentID,
		Hostname:     req.Hostname,
		Labels:       req.Labels,
		Capabilities: req.Capabilities,
		LastSeen:     time.Now(),
		Status:       AgentStatusOnline,
		Stacks:       []string{}, // Initialize empty stack list
	}

	c.agents.agents[agentID] = agentConn

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

func (c *Core) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	c.agents.mu.Lock()
	defer c.agents.mu.Unlock()

	agentID := req.AgentId

	agent, exists := c.agents.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	// Update last seen time and status
	agent.LastSeen = time.Now()
	agent.Status = AgentStatusOnline

	return &agentv1.HeartbeatResponse{
		Status: "healthy",
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
	c.agents.mu.Lock() // Need to write lock since we might update the connection
	defer c.agents.mu.Unlock()

	agentConn, exists := c.agents.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	if agentConn.Status != AgentStatusOnline {
		return nil, fmt.Errorf("agent offline: %s", agentID)
	}

	// If we don't have a client connection yet, try to establish one
	if agentConn.Client == nil {
		// Construct agent address - in a real system, this would come from the agent during registration
		// Extract just the hostname part from the agent ID (format: agent-<hostname>-<timestamp>)
		hostname := agentConn.Hostname
		if hostname == "" {
			// If hostname is empty, try to extract from agent ID
			// Format is typically "agent-<hostname>-<timestamp>" or similar
			parts := strings.Split(agentID, "-")
			if len(parts) > 1 {
				// Take all parts except the first ("agent") and last (timestamp) as hostname
				if len(parts) > 2 {
					hostname = strings.Join(parts[1:len(parts)-1], "-")
				} else {
					hostname = parts[1]
				}
			}
		}

		agentAddr := fmt.Sprintf("%s:8444", hostname) // Default agent port

		// Load certificates for connecting to agent (mTLS)
		cert, err := tls.LoadX509KeyPair(c.config.CertPath, c.config.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("load core cert for agent connection: %w", err)
		}

		// Load CA certificate to verify agent certificates
		caCert, err := ioutil.ReadFile(c.config.CAPath)
		if err != nil {
			return nil, fmt.Errorf("load CA cert for agent connection: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("parse CA cert for agent connection")
		}

		// Use mTLS for connection to agent
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCertPool,
			ServerName:   "mandau-agent", // Verify agent certificate against this name
			MinVersion:   tls.VersionTLS13,
		}

		creds := credentials.NewTLS(tlsConfig)

		// Create gRPC connection to agent
		conn, err := grpc.Dial(agentAddr, grpc.WithTransportCredentials(creds))
		if err != nil {
			return nil, fmt.Errorf("dial agent %s at %s: %w", agentID, agentAddr, err)
		}

		agentConn.Client = conn
		agentConn.Address = agentAddr
	}

	return agentConn, nil
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
	return &plugin.Identity{
		UserID: subject.CommonName,
	}, nil
}

func resultString(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

// =============================================================================
// STACK SERVICE IMPLEMENTATIONS (PROXY TO AGENTS)
// =============================================================================

func (c *Core) ListStacks(ctx context.Context, req *agentv1.ListStacksRequest) (*agentv1.ListStacksResponse, error) {
	agentID := req.AgentId

	conn, err := c.getAgentConnection(agentID)
	if err != nil {
		return nil, fmt.Errorf("get agent connection: %w", err)
	}

	// Create stack service client for this agent
	stackClient := agentv1.NewStackServiceClient(conn.Client)

	// Forward the request to the agent
	resp, err := stackClient.ListStacks(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("forward to agent: %w", err)
	}

	// Update the agent's stack list in our registry
	stackIDs := make([]string, len(resp.Stacks))
	for i, stack := range resp.Stacks {
		stackIDs[i] = stack.Id
	}
	c.updateAgentStacks(agentID, stackIDs)

	return resp, nil
}

func (c *Core) GetStack(ctx context.Context, req *agentv1.GetStackRequest) (*agentv1.GetStackResponse, error) {
	// Find which agent has this stack
	agentID, err := c.findAgentWithStack(req.StackId)
	if err != nil {
		return nil, fmt.Errorf("find agent with stack: %w", err)
	}

	conn, err := c.getAgentConnection(agentID)
	if err != nil {
		return nil, fmt.Errorf("get agent connection: %w", err)
	}

	// Create stack service client for this agent
	stackClient := agentv1.NewStackServiceClient(conn.Client)

	// Forward the request to the agent
	resp, err := stackClient.GetStack(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("forward to agent: %w", err)
	}

	return resp, nil
}

func (c *Core) ApplyStack(req *agentv1.ApplyStackRequest, stream agentv1.StackService_ApplyStackServer) error {
	agentID := req.AgentId

	conn, err := c.getAgentConnection(agentID)
	if err != nil {
		return fmt.Errorf("get agent connection: %w", err)
	}

	// Create stack service client for this agent
	stackClient := agentv1.NewStackServiceClient(conn.Client)

	// Forward the request to the agent
	agentStream, err := stackClient.ApplyStack(stream.Context(), req)
	if err != nil {
		return fmt.Errorf("forward to agent: %w", err)
	}

	// Stream responses back to client
	for {
		event, err := agentStream.Recv()
		if err != nil {
			return err
		}

		if err := stream.Send(event); err != nil {
			return err
		}
	}
}

func (c *Core) RemoveStack(req *agentv1.RemoveStackRequest, stream agentv1.StackService_RemoveStackServer) error {
	// Find which agent has this stack
	agentID, err := c.findAgentWithStack(req.StackId)
	if err != nil {
		return fmt.Errorf("find agent with stack: %w", err)
	}

	conn, err := c.getAgentConnection(agentID)
	if err != nil {
		return fmt.Errorf("get agent connection: %w", err)
	}

	// Create stack service client for this agent
	stackClient := agentv1.NewStackServiceClient(conn.Client)

	// Forward the request to the agent
	agentStream, err := stackClient.RemoveStack(stream.Context(), req)
	if err != nil {
		return fmt.Errorf("forward to agent: %w", err)
	}

	// Stream responses back to client
	for {
		event, err := agentStream.Recv()
		if err != nil {
			return err
		}

		if err := stream.Send(event); err != nil {
			return err
		}
	}
}

func (c *Core) DiffStack(ctx context.Context, req *agentv1.DiffStackRequest) (*agentv1.DiffStackResponse, error) {
	// Since DiffStack doesn't have an agent ID in the request, we need to determine it
	// For now, we'll return an error indicating this limitation
	return nil, fmt.Errorf("DiffStack not implemented in core proxy - agent ID required in request")
}

// findAgentWithStack finds which agent has a specific stack
func (c *Core) findAgentWithStack(stackID string) (string, error) {
	c.agents.mu.RLock()
	defer c.agents.mu.RUnlock()

	for agentID, agent := range c.agents.agents {
		for _, stack := range agent.Stacks {
			if stack == stackID {
				return agentID, nil
			}
		}
	}

	return "", fmt.Errorf("stack not found on any agent: %s", stackID)
}

// updateAgentStacks updates the list of stacks for an agent
func (c *Core) updateAgentStacks(agentID string, stacks []string) error {
	c.agents.mu.Lock()
	defer c.agents.mu.Unlock()

	agent, exists := c.agents.agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	agent.Stacks = stacks
	return nil
}

func (c *Core) GetStackLogs(req *agentv1.GetStackLogsRequest, stream agentv1.StackService_GetStackLogsServer) error {
	agentID := req.AgentId

	conn, err := c.getAgentConnection(agentID)
	if err != nil {
		return fmt.Errorf("get agent connection: %w", err)
	}

	// Create stack service client for this agent
	stackClient := agentv1.NewStackServiceClient(conn.Client)

	// Forward the request to the agent
	agentStream, err := stackClient.GetStackLogs(stream.Context(), req)
	if err != nil {
		return fmt.Errorf("forward to agent: %w", err)
	}

	// Stream responses back to client
	for {
		logEntry, err := agentStream.Recv()
		if err != nil {
			return err
		}

		if err := stream.Send(logEntry); err != nil {
			return err
		}
	}
}
