package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/bhangun/mandau/api/v1"
	"github.com/bhangun/mandau/pkg/agent/container"
	"github.com/bhangun/mandau/pkg/agent/filesystem"
	"github.com/bhangun/mandau/pkg/agent/operation"
	"github.com/bhangun/mandau/pkg/agent/stack"
	"github.com/bhangun/mandau/pkg/config"
	"github.com/bhangun/mandau/pkg/plugin"
	"github.com/bhangun/mandau/plugins/auth/rbac"
	"github.com/moby/moby/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Agent struct {
	agentv1.UnimplementedAgentServiceServer
	agentv1.UnimplementedStackServiceServer
	agentv1.UnimplementedContainerServiceServer
	agentv1.UnimplementedFilesystemServiceServer
	agentv1.UnimplementedOperationsServiceServer

	config       *Config
	serverConn   *grpc.ClientConn
	docker       *client.Client
	plugins      *plugin.Registry
	opMgr        *operation.Manager
	stackMgr     *stack.Manager
	containerMgr *container.Manager
	fsMgr        *filesystem.Manager
}

type Config struct {
	AgentID    string
	Hostname   string
	ListenAddr string
	ServerAddr string
	CertPath   string
	KeyPath    string
	CAPath     string
	StackRoot  string
	PluginDir  string
	Labels     map[string]string
	// Add a field to hold the full configuration
	FullConfig *config.AgentConfig
}

func main() {
	// Parse only the config flag from command line args
	// Skip the first argument (program name) and look for --config
	args := os.Args[1:]
	configFilePath := "config/agent/config.yaml"

	// Find and extract config file path
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			configFilePath = args[i+1]
			break
		} else if args[i] == "-config" && i+1 < len(args) {
			configFilePath = args[i+1]
			break
		} else if strings.HasPrefix(args[i], "--config=") {
			configFilePath = strings.TrimPrefix(args[i], "--config=")
			break
		} else if strings.HasPrefix(args[i], "-config=") {
			configFilePath = strings.TrimPrefix(args[i], "-config=")
			break
		}
	}

	// Load configuration from file if available
	agentConfig, err := config.LoadAgentConfig(configFilePath)
	if err != nil {
		fmt.Printf("Config file not found at %s, using defaults: %v\n", configFilePath, err)
		agentConfig = config.CreateDefaultAgentConfig()
	} else {
		fmt.Printf("Loaded configuration from %s\n", configFilePath)
	}

	// Filter out config-related arguments for regular flag parsing
	filteredArgs := filterConfigArgs(os.Args[1:])

	// Now parse all other flags
	cfg := parseFlags(filteredArgs)

	// Apply configuration file values as defaults, but allow command-line overrides
	if agentConfig.Server.ListenAddr != "" && cfg.ListenAddr == ":8444" {
		// Only use config file value if the default was used (not overridden by CLI)
		cfg.ListenAddr = agentConfig.Server.ListenAddr
	}
	if agentConfig.Server.TLS.CertPath != "" {
		cfg.CertPath = agentConfig.Server.TLS.CertPath
	}
	if agentConfig.Server.TLS.KeyPath != "" {
		cfg.KeyPath = agentConfig.Server.TLS.KeyPath
	}
	if agentConfig.Server.TLS.CAPath != "" {
		cfg.CAPath = agentConfig.Server.TLS.CAPath
	}

	// Use server connection config for core server address if available
	if agentConfig.ServerConnection.CoreAddr != "" {
		cfg.ServerAddr = agentConfig.ServerConnection.CoreAddr
	}

	if agentConfig.Stacks.RootDir != "" {
		cfg.StackRoot = agentConfig.Stacks.RootDir
	}
	if agentConfig.Agent.Labels != nil {
		for k, v := range agentConfig.Agent.Labels {
			cfg.Labels[k] = v
		}
	}

	// Store the full configuration
	cfg.FullConfig = agentConfig

	agent, err := NewAgent(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create agent: %v\n", err)
		os.Exit(1)
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- agent.Serve()
	}()

	select {
	case err := <-errChan:
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
		agent.Shutdown()
	}
}

// filterConfigArgs removes config-related arguments from the command line args
func filterConfigArgs(args []string) []string {
	var filtered []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" || args[i] == "-config" {
			// Skip this argument and the next one (the value)
			i++
			continue
		} else if strings.HasPrefix(args[i], "--config=") || strings.HasPrefix(args[i], "-config=") {
			// Skip this argument entirely
			continue
		} else {
			// Keep this argument
			filtered = append(filtered, args[i])
		}
	}
	return filtered
}

func parseFlags(configArgs []string) *Config {
	cfg := &Config{
		Labels: make(map[string]string),
	}

	// Create a new flag set that uses the filtered arguments
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	flagSet.StringVar(&cfg.AgentID, "id", "", "Agent ID (auto-generated if empty)")
	flagSet.StringVar(&cfg.ListenAddr, "listen", ":8444", "Listen address")
	flagSet.StringVar(&cfg.ServerAddr, "server", "localhost:8443", "Core server address")
	flagSet.StringVar(&cfg.CertPath, "cert", "/etc/mandau/agent.crt", "Certificate path")
	flagSet.StringVar(&cfg.KeyPath, "key", "/etc/mandau/agent.key", "Key path")
	flagSet.StringVar(&cfg.CAPath, "ca", "/etc/mandau/ca.crt", "CA certificate path")
	flagSet.StringVar(&cfg.StackRoot, "stack-root", "/var/lib/mandau/stacks", "Stack root directory")
	flagSet.StringVar(&cfg.PluginDir, "plugin-dir", "/usr/lib/mandau/plugins", "Plugin directory")

	// Parse the filtered arguments
	flagSet.Parse(configArgs)

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	cfg.Hostname = hostname

	// Use provided agent ID, or load from persistent storage, or generate new one
	if cfg.AgentID == "" {
		// Try to load persistent agent ID from file
		persistentID := loadPersistentAgentID()
		if persistentID != "" {
			cfg.AgentID = persistentID
		} else {
			// Generate new agent ID based on hostname
			cfg.AgentID = fmt.Sprintf("agent-%s", hostname)
			// Save the new ID for persistence
			savePersistentAgentID(cfg.AgentID)
		}
	} else {
		// If agent ID is provided via CLI, save it for persistence
		savePersistentAgentID(cfg.AgentID)
	}

	return cfg
}

func NewAgent(cfg *Config) (*Agent, error) {
	// Docker client
	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}

	// On macOS, if DOCKER_HOST is not set but the default socket doesn't exist,
	// try the Docker Desktop socket path
	if runtime.GOOS == "darwin" {
		dockerHost := os.Getenv("DOCKER_HOST")
		if dockerHost == "" {
			// Check if default socket exists
			if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
				// Try Docker Desktop socket path
				defaultDockerDesktopSock := "/Users/" + os.Getenv("USER") + "/.docker/run/docker.sock"
				if _, err := os.Stat(defaultDockerDesktopSock); err == nil {
					os.Setenv("DOCKER_HOST", "unix://"+defaultDockerDesktopSock)
					opts = append(opts, client.WithHost("unix://"+defaultDockerDesktopSock))
				}
			}
		}
	}

	docker, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}

	// Test docker connection
	ctx := context.Background()
	if _, err := docker.Ping(ctx, client.PingOptions{}); err != nil {
		return nil, fmt.Errorf("docker ping: %w", err)
	}

	// Plugin registry
	plugins := plugin.NewRegistry()

	// Load plugins
	if err := loadPluginsFromDir(plugins, cfg.PluginDir, cfg.FullConfig.Plugins); err != nil {
		fmt.Printf("Warning: plugin loading failed: %v\n", err)
		// Continue without plugins - they're optional
	}

	// Initialize plugins with configuration from config file
	if err := plugins.Init(ctx, cfg.FullConfig.Plugins.Configs); err != nil {
		return nil, fmt.Errorf("plugin init: %w", err)
	}

	// Create managers
	opMgr := operation.NewManager()
	stackMgr := stack.NewManager(cfg.StackRoot, docker, opMgr)
	containerMgr := container.NewManager()
	fsMgr := filesystem.NewManager()

	// Create gRPC connection to core server
	serverConn, err := createServerConnection(cfg)
	if err != nil {
		return nil, fmt.Errorf("create server connection: %w", err)
	}

	agent := &Agent{
		config:       cfg,
		serverConn:   serverConn,
		docker:       docker,
		plugins:      plugins,
		opMgr:        opMgr,
		stackMgr:     stackMgr,
		containerMgr: containerMgr,
		fsMgr:        fsMgr,
	}

	// Register with core server
	if err := agent.registerWithServer(); err != nil {
		return nil, fmt.Errorf("register with server: %w", err)
	}

	// Start heartbeat goroutine
	go agent.startHeartbeat()

	return agent, nil
}

// createServerConnection creates a secure gRPC connection to the core server with retry logic
func createServerConnection(cfg *Config) (*grpc.ClientConn, error) {
	// Load certificates
	cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("load cert: %w", err)
	}

	// Load CA
	caCert, err := ioutil.ReadFile(cfg.CAPath)
	if err != nil {
		return nil, fmt.Errorf("load CA: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("parse CA cert")
	}

	// mTLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		ServerName:   "mandau-core", // Use the server name from the certificate
		MinVersion:   tls.VersionTLS13,
	}

	creds := credentials.NewTLS(tlsConfig)

	// Create connection with retry options
	conn, err := grpc.Dial(cfg.ServerAddr,
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
		return nil, fmt.Errorf("dial server: %w", err)
	}

	return conn, nil
}

// registerWithServer registers the agent with the core server
func (a *Agent) registerWithServer() error {
	client := agentv1.NewCoreServiceClient(a.serverConn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.RegisterAgent(ctx, &agentv1.RegisterRequest{
		Hostname:     a.config.Hostname,
		AgentId:      a.config.AgentID, // Send persistent agent ID
		Labels:       map[string]string{}, // Add agent labels
		Capabilities: []string{"docker", "stack", "container", "logs", "exec"},
	})
	if err != nil {
		return fmt.Errorf("register agent: %w", err)
	}

	fmt.Printf("Agent registered with ID: %s\n", resp.AgentId)
	return nil
}

// startHeartbeat starts the periodic heartbeat to the core server with reconnection logic
func (a *Agent) startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second) // Heartbeat every 30 seconds
	defer ticker.Stop()

	// Create a context that will be cancelled when the agent shuts down
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case <-ticker.C:
			if err := a.sendHeartbeat(); err != nil {
				fmt.Printf("Heartbeat failed: %v\n", err)
				// Try to reconnect if heartbeat fails
				if a.shouldReconnect(err) {
					fmt.Println("Attempting to reconnect to core server...")
					if err := a.reconnectToServer(); err != nil {
						fmt.Printf("Reconnection failed: %v\n", err)
					} else {
						fmt.Println("Reconnected to core server successfully")
					}
				}
			}
		case <-ctx.Done():
			// Agent is shutting down
			fmt.Println("Heartbeat routine stopped")
			return
		}
	}
}

// shouldReconnect determines if the agent should attempt to reconnect based on the error
func (a *Agent) shouldReconnect(err error) bool {
	// Check if the error indicates a connection issue
	return status.Code(err) == codes.Unavailable ||
		   status.Code(err) == codes.DeadlineExceeded ||
		   strings.Contains(err.Error(), "connection refused") ||
		   strings.Contains(err.Error(), "connection reset") ||
		   strings.Contains(err.Error(), "broken pipe")
}

// reconnectToServer attempts to reconnect to the core server
func (a *Agent) reconnectToServer() error {
	// Close existing connection if it exists
	if a.serverConn != nil {
		a.serverConn.Close()
	}

	// Create new connection
	newConn, err := createServerConnection(a.config)
	if err != nil {
		return fmt.Errorf("create new server connection: %w", err)
	}

	// Update the connection
	a.serverConn = newConn

	// Re-register with server
	if err := a.registerWithServer(); err != nil {
		return fmt.Errorf("re-register with server: %w", err)
	}

	return nil
}

// sendHeartbeat sends a heartbeat to the core server
func (a *Agent) sendHeartbeat() error {
	client := agentv1.NewCoreServiceClient(a.serverConn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Heartbeat(ctx, &agentv1.HeartbeatRequest{
		AgentId: a.config.AgentID,
		Status:  map[string]string{"status": "healthy"},
	})
	if err != nil {
		return fmt.Errorf("send heartbeat: %w", err)
	}

	return nil
}

func (a *Agent) Serve() error {
	// Load certificates
	cert, err := tls.LoadX509KeyPair(a.config.CertPath, a.config.KeyPath)
	if err != nil {
		return fmt.Errorf("load cert: %w", err)
	}

	// Load CA
	caCert, err := ioutil.ReadFile(a.config.CAPath)
	if err != nil {
		return fmt.Errorf("load CA: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("parse CA cert")
	}

	// mTLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}

	creds := credentials.NewTLS(tlsConfig)

	// gRPC server with security interceptors
	server := grpc.NewServer(
		grpc.Creds(creds),
		grpc.MaxRecvMsgSize(10*1024*1024), // 10MB
		grpc.MaxSendMsgSize(10*1024*1024),
		grpc.ChainUnaryInterceptor(
			a.authInterceptor,
			a.auditInterceptor,
			a.policyInterceptor,
			a.recoveryInterceptor,
		),
		grpc.ChainStreamInterceptor(
			a.authStreamInterceptor,
			a.auditStreamInterceptor,
			a.recoveryStreamInterceptor,
		),
	)

	// Register all services
	agentv1.RegisterAgentServiceServer(server, a)
	agentv1.RegisterStackServiceServer(server, a)
	agentv1.RegisterContainerServiceServer(server, a)
	agentv1.RegisterFilesystemServiceServer(server, a)
	agentv1.RegisterOperationsServiceServer(server, a)

	// Listen
	lis, err := net.Listen("tcp", a.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	fmt.Printf("Mandau Agent %s listening on %s\n", a.config.AgentID, a.config.ListenAddr)
	fmt.Printf("Hostname: %s\n", a.config.Hostname)
	fmt.Printf("Stack root: %s\n", a.config.StackRoot)
	fmt.Printf("Plugins loaded: %d\n", len(a.plugins.ListAll()))

	return server.Serve(lis)
}

func (a *Agent) Shutdown() {
	fmt.Println("Shutting down agent...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown plugins
	if err := a.plugins.ShutdownAll(ctx); err != nil {
		fmt.Printf("Plugin shutdown error: %v\n", err)
	}

	// Close server connection
	if a.serverConn != nil {
		a.serverConn.Close()
	}

	// Close docker client
	if a.docker != nil {
		a.docker.Close()
	}

	fmt.Println("Agent stopped")
}

// =============================================================================
// SECURITY INTERCEPTORS
// =============================================================================

func (a *Agent) authInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	identity, err := a.extractIdentity(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication failed")
	}

	// Authenticate via plugin
	if auth := a.plugins.Auth(); auth != nil {
		identity, err = auth.Authenticate(ctx, &plugin.AuthRequest{
			Identity: identity,
			Method:   info.FullMethod,
		})
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "authentication failed")
		}
	}

	ctx = plugin.WithIdentity(ctx, identity)
	return handler(ctx, req)
}

func (a *Agent) policyInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	identity := plugin.IdentityFromContext(ctx)

	// Policy evaluation
	if policy := a.plugins.Policy(); policy != nil {
		decision, err := policy.Evaluate(ctx, &plugin.PolicyRequest{
			Identity: identity,
			Action: &plugin.Action{
				Method: info.FullMethod,
			},
			Resource: extractResourceFromRequest(req),
		})

		if err != nil || !decision.Allowed {
			return nil, status.Errorf(codes.PermissionDenied, "access denied: %s", decision.Reason)
		}
	}

	return handler(ctx, req)
}

func (a *Agent) auditInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	identity := plugin.IdentityFromContext(ctx)

	resp, err := handler(ctx, req)

	// Audit all calls
	a.plugins.AuditAll(ctx, &plugin.AuditEntry{
		Timestamp: start,
		AgentID:   a.config.AgentID,
		Identity:  identity,
		Action:    info.FullMethod,
		Resource:  extractResourceFromRequest(req).Identifier,
		Result:    resultString(err),
		Duration:  time.Since(start),
		Metadata:  extractMetadata(req),
	})

	return resp, err
}

func (a *Agent) recoveryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in %s: %v\n", info.FullMethod, r)
			err = status.Errorf(codes.Internal, "internal error")
		}
	}()

	return handler(ctx, req)
}

func (a *Agent) authStreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	ctx := ss.Context()

	identity, err := a.extractIdentity(ctx)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "authentication failed")
	}

	if auth := a.plugins.Auth(); auth != nil {
		identity, err = auth.Authenticate(ctx, &plugin.AuthRequest{
			Identity: identity,
			Method:   info.FullMethod,
		})
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "authentication failed")
		}
	}

	wrapped := &wrappedStream{
		ServerStream: ss,
		ctx:          plugin.WithIdentity(ctx, identity),
	}

	return handler(srv, wrapped)
}

func (a *Agent) auditStreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()
	ctx := ss.Context()
	identity := plugin.IdentityFromContext(ctx)

	err := handler(srv, ss)

	a.plugins.AuditAll(ctx, &plugin.AuditEntry{
		Timestamp: start,
		AgentID:   a.config.AgentID,
		Identity:  identity,
		Action:    info.FullMethod,
		Result:    resultString(err),
		Duration:  time.Since(start),
	})

	return err
}

func (a *Agent) recoveryStreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in stream %s: %v\n", info.FullMethod, r)
			err = status.Errorf(codes.Internal, "internal error")
		}
	}()

	return handler(srv, ss)
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

func (a *Agent) extractIdentity(ctx context.Context) (*plugin.Identity, error) {
	// Extract identity from mTLS certificate
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no peer info")
	}

	tlsInfo, ok := peer.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, fmt.Errorf("no TLS info")
	}

	if len(tlsInfo.State.VerifiedChains) == 0 || len(tlsInfo.State.VerifiedChains[0]) == 0 {
		return nil, fmt.Errorf("no verified certificate")
	}

	cert := tlsInfo.State.VerifiedChains[0][0]

	return &plugin.Identity{
		UserID:      cert.Subject.CommonName,
		DeviceID:    extractDeviceID(cert),
		Certificate: cert.Raw,
		Attributes:  make(map[string]string),
	}, nil
}

func extractDeviceID(cert *x509.Certificate) string {
	// Extract from certificate extensions or subject
	return cert.Subject.CommonName
}

func extractResourceFromRequest(req interface{}) *plugin.Resource {
	// Extract resource info based on request type
	// This is simplified - production would use type assertions
	return &plugin.Resource{
		Type:       "unknown",
		Identifier: "",
		Labels:     make(map[string]string),
	}
}

func extractMetadata(req interface{}) map[string]string {
	return make(map[string]string)
}

func resultString(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

// =============================================================================
// AGENT SERVICE IMPLEMENTATIONS
// =============================================================================

func (a *Agent) Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	return &agentv1.RegisterResponse{
		AgentId:           a.config.AgentID,
		HeartbeatInterval: durationpb.New(30 * time.Second),
	}, nil
}

func (a *Agent) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	return &agentv1.HeartbeatResponse{
		Status: "healthy",
	}, nil
}

func (a *Agent) GetCapabilities(ctx context.Context, req *agentv1.CapabilitiesRequest) (*agentv1.CapabilitiesResponse, error) {
	return &agentv1.CapabilitiesResponse{
		Capabilities: []string{
			"stack.apply",
			"stack.remove",
			"container.exec",
			"logs.stream",
			"files.manage",
		},
	}, nil
}

func (a *Agent) GetHealth(ctx context.Context, req *agentv1.HealthRequest) (*agentv1.HealthResponse, error) {
	// Check Docker health
	_, err := a.docker.Ping(ctx, client.PingOptions{})

	healthy := err == nil

	return &agentv1.HealthResponse{
		Healthy: healthy,
		Status: map[string]string{
			"docker": healthStatus(err),
		},
	}, nil
}

// =============================================================================
// STACK SERVICE IMPLEMENTATIONS
// =============================================================================

func (a *Agent) ListStacks(ctx context.Context, req *agentv1.ListStacksRequest) (*agentv1.ListStacksResponse, error) {
	stacks, err := a.stackMgr.ListStacks(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list stacks: %v", err)
	}

	result := make([]*agentv1.Stack, len(stacks))
	for i, stack := range stacks {
		result[i] = &agentv1.Stack{
			Id:         stack.ID,
			Name:       stack.Name,
			Path:       stack.Path,
			State:      convertStackState(stack.State),
			Containers: convertContainers(stack.Containers),
			CreatedAt:  convertTimeToProto(stack.CreatedAt),
			UpdatedAt:  convertTimeToProto(stack.UpdatedAt),
			Labels:     stack.Labels,
		}
	}

	return &agentv1.ListStacksResponse{
		Stacks: result,
	}, nil
}

func (a *Agent) GetStack(ctx context.Context, req *agentv1.GetStackRequest) (*agentv1.GetStackResponse, error) {
	stack, err := a.stackMgr.GetStack(ctx, req.StackId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "get stack: %v", err)
	}

	return &agentv1.GetStackResponse{
		Stack: &agentv1.Stack{
			Id:         stack.ID,
			Name:       stack.Name,
			Path:       stack.Path,
			State:      convertStackState(stack.State),
			Containers: convertContainers(stack.Containers),
			CreatedAt:  convertTimeToProto(stack.CreatedAt),
			UpdatedAt:  convertTimeToProto(stack.UpdatedAt),
			Labels:     stack.Labels,
		},
	}, nil
}

func (a *Agent) ApplyStack(req *agentv1.ApplyStackRequest, stream agentv1.StackService_ApplyStackServer) error {
	ctx := stream.Context()

	// Convert proto request to internal request
	internalReq := &stack.ApplyStackRequest{
		StackName:      req.StackName,
		ComposeContent: req.ComposeContent,
		EnvVars:        req.EnvVars,
		ForceRecreate:  req.ForceRecreate,
		Services:       req.Services,
		PullImages:     req.PullImages,
	}

	opID, err := a.stackMgr.ApplyStack(ctx, internalReq)
	if err != nil {
		return status.Errorf(codes.Internal, "apply stack: %v", err)
	}

	// Stream operation events
	events := a.opMgr.Subscribe(opID)
	defer a.opMgr.Unsubscribe(opID, events)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}

			errorMsg := ""
			if event.Error != nil {
				errorMsg = event.Error.Error()
			}

			resp := &agentv1.OperationEvent{
				OperationId: event.OperationID,
				State:       convertOperationState(event.State),
				Timestamp:   timestamppb.Now(),
				Message:     event.Message,
				Progress:    int32(event.Progress),
				Error:       errorMsg,
			}

			if err := stream.Send(resp); err != nil {
				return err
			}

			// If operation is completed, exit
			if event.State == operation.OperationStateCompleted || event.State == operation.OperationStateFailed {
				return nil
			}
		}
	}
}

func (a *Agent) RemoveStack(req *agentv1.RemoveStackRequest, stream agentv1.StackService_RemoveStackServer) error {
	ctx := stream.Context()

	// Extract stack name from stack ID (in our case, stack ID is the name)
	stackName := req.StackId

	opID, err := a.stackMgr.RemoveStack(ctx, stackName, false) // Don't remove volumes by default
	if err != nil {
		return status.Errorf(codes.Internal, "remove stack: %v", err)
	}

	// Stream operation events
	events := a.opMgr.Subscribe(opID)
	defer a.opMgr.Unsubscribe(opID, events)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}

			errorMsg := ""
			if event.Error != nil {
				errorMsg = event.Error.Error()
			}

			resp := &agentv1.OperationEvent{
				OperationId: event.OperationID,
				State:       convertOperationState(event.State),
				Timestamp:   timestamppb.Now(),
				Message:     event.Message,
				Progress:    int32(event.Progress),
				Error:       errorMsg,
			}

			if err := stream.Send(resp); err != nil {
				return err
			}

			// If operation is completed, exit
			if event.State == operation.OperationStateCompleted || event.State == operation.OperationStateFailed {
				return nil
			}
		}
	}
}

func (a *Agent) DiffStack(ctx context.Context, req *agentv1.DiffStackRequest) (*agentv1.DiffStackResponse, error) {
	result, err := a.stackMgr.DiffStack(ctx, req.StackName, req.NewComposeContent)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "diff stack: %v", err)
	}

	// Convert internal diff result to proto
	protoServices := make([]*agentv1.ServiceDiff, len(result.Services))
	for i, svcDiff := range result.Services {
		protoServices[i] = &agentv1.ServiceDiff{
			Name:    svcDiff.Name,
			Action:  convertDiffAction(svcDiff.Action),
			Changes: svcDiff.Changes,
		}
	}

	return &agentv1.DiffStackResponse{
		Services:   protoServices,
		HasChanges: result.HasChanges,
	}, nil
}

func (a *Agent) GetStackLogs(req *agentv1.GetStackLogsRequest, stream agentv1.StackService_GetStackLogsServer) error {
	ctx := stream.Context()

	// Get containers for the stack to stream logs from
	stack, err := a.stackMgr.GetStack(ctx, req.StackName)
	if err != nil {
		return status.Errorf(codes.NotFound, "get stack: %v", err)
	}

	// Stream logs from each container in the stack
	for _, container := range stack.Containers {
		// For now, we'll send a simple log entry - in production this would connect to the actual container logs
		logEntry := &agentv1.LogEntry{
			Timestamp:   timestamppb.Now(),
			Stream:      "stdout",
			Content:     []byte(fmt.Sprintf("Logs for container %s in stack %s", container.Name, req.StackName)),
			ContainerId: container.ID,
			ServiceName: container.Service,
		}

		if err := stream.Send(logEntry); err != nil {
			return err
		}
	}

	return nil
}

func healthStatus(err error) string {
	if err != nil {
		return "unhealthy"
	}
	return "healthy"
}

// Helper functions for converting between internal and proto types
func convertStackState(state stack.StackState) agentv1.StackState {
	switch state {
	case stack.StateRunning:
		return agentv1.StackState_STACK_STATE_RUNNING
	case stack.StateStopped:
		return agentv1.StackState_STACK_STATE_STOPPED
	case stack.StateError:
		return agentv1.StackState_STACK_STATE_ERROR
	case stack.StatePartial:
		return agentv1.StackState_STACK_STATE_PARTIAL
	default:
		return agentv1.StackState_STACK_STATE_UNKNOWN
	}
}

func convertContainers(containers []stack.ContainerInfo) []*agentv1.Container {
	result := make([]*agentv1.Container, len(containers))
	for i, container := range containers {
		result[i] = &agentv1.Container{
			Id:     container.ID,
			Name:   container.Name,
			Image:  container.Image,
			State:  container.State,
			Status: container.Status,
			Labels: map[string]string{}, // Add labels if available
		}
	}
	return result
}

func convertTimeToProto(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

func convertOperationState(state operation.OperationState) agentv1.OperationState {
	switch state {
	case operation.OperationStateRunning:
		return agentv1.OperationState_OPERATION_STATE_RUNNING
	case operation.OperationStateCompleted:
		return agentv1.OperationState_OPERATION_STATE_COMPLETED
	case operation.OperationStateFailed:
		return agentv1.OperationState_OPERATION_STATE_FAILED
	case operation.OperationStateCancelled:
		return agentv1.OperationState_OPERATION_STATE_CANCELLED
	default:
		return agentv1.OperationState_OPERATION_STATE_PENDING
	}
}

func convertDiffAction(action stack.DiffAction) agentv1.DiffAction {
	switch action {
	case stack.DiffActionCreate:
		return agentv1.DiffAction_DIFF_ACTION_CREATE
	case stack.DiffActionUpdate:
		return agentv1.DiffAction_DIFF_ACTION_UPDATE
	case stack.DiffActionDelete:
		return agentv1.DiffAction_DIFF_ACTION_DELETE
	default:
		return agentv1.DiffAction_DIFF_ACTION_NONE
	}
}

func loadPluginsFromDir(registry *plugin.Registry, dir string, pluginConfig config.PluginConfig) error {
	// Load plugins based on configuration
	for pluginName, isEnabled := range pluginConfig.Enabled {
		if !isEnabled {
			continue
		}

		switch pluginName {
		case "rbac-auth":
			rbacPlugin := rbac.New()
			if err := registry.Register(rbacPlugin); err != nil {
				return fmt.Errorf("register rbac plugin: %w", err)
			}
		default:
			fmt.Printf("Unknown plugin: %s\n", pluginName)
		}
	}

	return nil
}

// loadPersistentAgentID loads the agent ID from a persistent file
func loadPersistentAgentID() string {
	// Try to read agent ID from a persistent file
	idFile := getAgentIDFilePath()
	if _, err := os.Stat(idFile); os.IsNotExist(err) {
		return "" // File doesn't exist yet
	}

	data, err := ioutil.ReadFile(idFile)
	if err != nil {
		fmt.Printf("Warning: could not read agent ID file: %v\n", err)
		return ""
	}

	id := strings.TrimSpace(string(data))
	if id == "" {
		return ""
	}

	return id
}

// savePersistentAgentID saves the agent ID to a persistent file
func savePersistentAgentID(id string) {
	idFile := getAgentIDFilePath()

	// Create directory if it doesn't exist
	dir := filepath.Dir(idFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Warning: could not create directory for agent ID file: %v\n", err)
		return
	}

	if err := ioutil.WriteFile(idFile, []byte(id), 0600); err != nil {
		fmt.Printf("Warning: could not save agent ID to file: %v\n", err)
	}
}

// getAgentIDFilePath returns the path to the agent ID file
func getAgentIDFilePath() string {
	// Use the stack root directory to store the agent ID
	stackRoot := "./stacks" // Default from config - we'll get this from agent config
	if _, err := os.Stat(stackRoot); os.IsNotExist(err) {
		// Create stacks directory if it doesn't exist
		os.MkdirAll(stackRoot, 0755)
	}
	return filepath.Join(stackRoot, ".agent_id")
}
