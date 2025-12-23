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
	"github.com/bhangun/mandau/pkg/config"
	"github.com/bhangun/mandau/pkg/plugin"
	"github.com/bhangun/mandau/plugins/auth/rbac"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
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
	// Add a field to hold the full configuration
	FullConfig *config.CoreConfig
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

	// Load the full configuration from file if available
	configPath := config.GetConfigPath("config/core/config.yaml")
	fullConfig, err := config.LoadCoreConfig(configPath)
	if err != nil {
		// If config file doesn't exist, create a default one
		log.Printf("Config file not found at %s, using defaults: %v", configPath, err)
		fullConfig = config.CreateDefaultCoreConfig()
	} else {
		log.Printf("Loaded configuration from %s", configPath)
	}

	// Load plugins
	if err := loadPlugins(plugins, cfg.PluginDir, fullConfig.Plugins); err != nil {
		return nil, fmt.Errorf("load plugins: %w", err)
	}

	// Update the CoreConfig with values from the loaded config
	if fullConfig.Server.ListenAddr != "" {
		cfg.ListenAddr = fullConfig.Server.ListenAddr
	}
	if fullConfig.Server.TLS.CertPath != "" {
		cfg.CertPath = fullConfig.Server.TLS.CertPath
	}
	if fullConfig.Server.TLS.KeyPath != "" {
		cfg.KeyPath = fullConfig.Server.TLS.KeyPath
	}
	if fullConfig.Server.TLS.CAPath != "" {
		cfg.CAPath = fullConfig.Server.TLS.CAPath
	}
	if fullConfig.PluginDir != "" {
		cfg.PluginDir = fullConfig.PluginDir
	}

	// Store the full configuration
	cfg.FullConfig = fullConfig

	return &Core{
		config:  cfg,
		agents:  &AgentRegistry{agents: make(map[string]*AgentConnection)},
		plugins: plugins,
		audit:   NewAuditLogger(plugins),
		authz:   NewAuthorizer(plugins),
	}, nil
}

func loadPlugins(plugins *plugin.Registry, dir string, pluginConfig config.PluginConfig) error {
	// Load plugins based on configuration
	for pluginName, isEnabled := range pluginConfig.Enabled {
		if !isEnabled {
			continue
		}

		switch pluginName {
		case "rbac-auth":
			rbacPlugin := rbac.New()
			if err := plugins.Register(rbacPlugin); err != nil {
				return fmt.Errorf("register rbac plugin: %w", err)
			}
		case "file-audit":
			// For now, we'll log that this plugin is not implemented
			log.Printf("File audit plugin not implemented in this build")
		default:
			log.Printf("Unknown plugin: %s", pluginName)
		}
	}

	// Initialize plugins with their configurations
	if err := plugins.Init(context.Background(), pluginConfig.Configs); err != nil {
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

	// Use provided agent ID if available, otherwise generate new one
	var agentID string
	if req.AgentId != "" {
		agentID = req.AgentId
	} else {
		agentID = generateAgentID(req.Hostname)
	}

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

	// Only update status to online if it was offline, to avoid unnecessary log messages
	if agent.Status == AgentStatusOffline {
		fmt.Printf("Agent %s is back online via heartbeat\n", agentID)
	}
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

	// If agent is offline, try to update its status by checking if it's recently sent a heartbeat
	if agentConn.Status == AgentStatusOffline {
		// If agent has sent a heartbeat in the last 30 seconds, consider it online again
		if time.Since(agentConn.LastSeen) <= 30*time.Second {
			agentConn.Status = AgentStatusOnline
			fmt.Printf("Agent %s is back online\n", agentID)
		} else {
			// Agent is still offline, return error
			return nil, fmt.Errorf("agent offline: %s", agentID)
		}
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

		// Create gRPC connection to agent with retry options
		conn, err := grpc.Dial(agentAddr,
			grpc.WithTransportCredentials(creds),
			grpc.WithConnectParams(grpc.ConnectParams{
				Backoff: backoff.Config{
					BaseDelay:  1.0 * time.Second,
					Multiplier: 1.6,
					Jitter:     0.2,
					MaxDelay:   10.0 * time.Second,
				},
				MinConnectTimeout: 5 * time.Second,
			}),
			// Add keepalive to detect broken connections
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                10 * time.Second,
				Timeout:             5 * time.Second,
				PermitWithoutStream: true,
			}),
		)
		if err != nil {
			return nil, fmt.Errorf("dial agent %s at %s: %w", agentID, agentAddr, err)
		}

		agentConn.Client = conn
		agentConn.Address = agentAddr
	}

	return agentConn, nil
}

// monitorAgents checks agent health periodically and attempts reconnection
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
				elapsed := time.Since(agent.LastSeen)

				// Mark as offline if no heartbeat for more than 90 seconds
				if elapsed > 90*time.Second {
					if agent.Status != AgentStatusOffline {
						agent.Status = AgentStatusOffline
						c.audit.LogAgentOffline(ctx, id)
						fmt.Printf("Agent %s marked as offline (last seen: %v ago)\n", id, elapsed)
					}
				}

				// Attempt to clean up stale connections for offline agents
				if agent.Status == AgentStatusOffline && agent.Client != nil {
					// Close the stale connection
					agent.Client.Close()
					agent.Client = nil
					fmt.Printf("Closed stale connection for offline agent %s\n", id)
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
