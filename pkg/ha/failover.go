// Package ha provides high availability support for Mandau,
// including multi-core connection management and automatic failover.
package ha

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/bhangun/mandau/pkg/transport"
	"google.golang.org/grpc"
)

// CoreNode represents a Mandau Core server instance
type CoreNode struct {
	ID       string
	Address  string
	Priority int
	Active   *transport.Manager
	State    NodeState
	lastSeen time.Time
}

// NodeState represents the state of a core node
type NodeState string

const (
	NodeStateActive      NodeState = "active"
	NodeStateStandby     NodeState = "standby"
	NodeStateUnreachable NodeState = "unreachable"
)

// FailoverManager manages connections to multiple core nodes
type FailoverManager struct {
	nodes       map[string]*CoreNode
	activeNode  *CoreNode
	mu          sync.RWMutex
	config      FailoverConfig
	stopChan    chan struct{}
	healthCheck *time.Ticker
}

// FailoverConfig holds configuration for failover management
type FailoverConfig struct {
	// HealthCheckInterval is how often to check node health
	HealthCheckInterval time.Duration
	// HealthCheckTimeout is the timeout for health check requests
	HealthCheckTimeout time.Duration
	// FailoverThreshold is the number of failed health checks before failover
	FailoverThreshold int
	// MaxRetries is the maximum number of reconnection attempts
	MaxRetries int
	// RetryDelay is the base delay between retry attempts
	RetryDelay time.Duration
}

// NewFailoverManager creates a new failover manager
func NewFailoverManager(config FailoverConfig) *FailoverManager {
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 10 * time.Second
	}
	if config.HealthCheckTimeout == 0 {
		config.HealthCheckTimeout = 5 * time.Second
	}
	if config.FailoverThreshold == 0 {
		config.FailoverThreshold = 3
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 5
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 2 * time.Second
	}

	return &FailoverManager{
		nodes:       make(map[string]*CoreNode),
		stopChan:    make(chan struct{}),
		config:      config,
		healthCheck: time.NewTicker(config.HealthCheckInterval),
	}
}

// AddNode adds a core node to the failover pool
func (fm *FailoverManager) AddNode(node *CoreNode) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	node.State = NodeStateStandby
	fm.nodes[node.ID] = node
}

// AddNodes adds multiple core nodes
func (fm *FailoverManager) AddNodes(nodes []*CoreNode) {
	for _, node := range nodes {
		fm.AddNode(node)
	}
}

// Connect attempts to connect to the highest priority available node
func (fm *FailoverManager) Connect(ctx context.Context) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Sort nodes by priority (highest first)
	sortedNodes := fm.getSortedNodes()

	var lastErr error
	for _, node := range sortedNodes {
		if node.State == NodeStateUnreachable {
			continue
		}

		err := node.Active.Connect(ctx)
		if err == nil {
			node.State = NodeStateActive
			fm.activeNode = node
			return nil
		}

		lastErr = err
		node.State = NodeStateUnreachable
	}

	return fmt.Errorf("failed to connect to any core node: %w", lastErr)
}

// ConnectWithFailover attempts to connect with automatic failover on failure
func (fm *FailoverManager) ConnectWithFailover(ctx context.Context) error {
	if err := fm.Connect(ctx); err != nil {
		return err
	}

	// Start health monitoring
	go fm.startHealthMonitoring()

	return nil
}

// GetActiveConnection returns the currently active gRPC connection
func (fm *FailoverManager) GetActiveConnection() *grpc.ClientConn {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if fm.activeNode == nil {
		return nil
	}

	return fm.activeNode.Active.GRPCConn()
}

// GetActiveNode returns the currently active core node
func (fm *FailoverManager) GetActiveNode() *CoreNode {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.activeNode
}

// Failover switches to the next available node
func (fm *FailoverManager) Failover(ctx context.Context) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if fm.activeNode != nil {
		fm.activeNode.State = NodeStateUnreachable
	}

	// Find next available node
	sortedNodes := fm.getSortedNodes()
	for _, node := range sortedNodes {
		if node.ID == fm.activeNode.ID {
			continue
		}

		if node.State == NodeStateUnreachable {
			continue
		}

		// Try to connect to this node
		ctx, cancel := context.WithTimeout(ctx, fm.config.HealthCheckTimeout)
		defer cancel()

		err := node.Active.Connect(ctx)
		if err == nil {
			node.State = NodeStateActive
			fm.activeNode = node
			return nil
		}

		node.State = NodeStateUnreachable
	}

	return fmt.Errorf("no available nodes for failover")
}

// GetNodeConnection returns a gRPC connection to the specified node
func (fm *FailoverManager) GetNodeConnection(nodeID string) (*grpc.ClientConn, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	node, exists := fm.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	conn := node.Active.GRPCConn()
	if conn == nil {
		return nil, fmt.Errorf("node %s is not connected", nodeID)
	}

	return conn, nil
}

// ListNodes returns all known core nodes
func (fm *FailoverManager) ListNodes() []*CoreNode {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	var nodes []*CoreNode
	for _, node := range fm.nodes {
		nodes = append(nodes, node)
	}

	return nodes
}

// GetNodeState returns the state of a specific node
func (fm *FailoverManager) GetNodeState(nodeID string) (NodeState, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	node, exists := fm.nodes[nodeID]
	if !exists {
		return "", fmt.Errorf("node %s not found", nodeID)
	}

	return node.State, nil
}

// startHealthMonitoring begins periodic health checks
func (fm *FailoverManager) startHealthMonitoring() {
	consecutiveFailures := 0

	for {
		select {
		case <-fm.healthCheck.C:
			if fm.activeNode == nil {
				continue
			}

			if fm.checkNodeHealth(fm.activeNode) {
				consecutiveFailures = 0
				fm.activeNode.lastSeen = time.Now()
			} else {
				consecutiveFailures++
				if consecutiveFailures >= fm.config.FailoverThreshold {
					// Trigger failover
					failoverCtx, failoverCancel := context.WithTimeout(context.Background(), fm.config.HealthCheckTimeout)
					if err := fm.Failover(failoverCtx); err != nil {
						// Log error but continue monitoring
					}
					failoverCancel() // Cancel immediately, not deferred
					consecutiveFailures = 0
				}
			}

		case <-fm.stopChan:
			fm.healthCheck.Stop()
			return
		}
	}
}

// checkNodeHealth performs a health check on a node
func (fm *FailoverManager) checkNodeHealth(node *CoreNode) bool {
	conn := node.Active.GRPCConn()
	if conn == nil {
		return false
	}

	// Try a simple gRPC call to check connectivity
	// Using GetState as a health check
	state := conn.GetState()
	return state.String() != "SHUTDOWN" && state.String() != "TRANSIENT_FAILURE"
}

// getSortedNodes returns nodes sorted by priority (highest first)
func (fm *FailoverManager) getSortedNodes() []*CoreNode {
	nodes := make([]*CoreNode, 0, len(fm.nodes))
	for _, node := range fm.nodes {
		nodes = append(nodes, node)
	}

	// Sort by priority (highest first)
	slices.SortFunc(nodes, func(a, b *CoreNode) int {
		return b.Priority - a.Priority
	})

	return nodes
}

// Stop stops the failover manager
func (fm *FailoverManager) Stop() {
	close(fm.stopChan)
}

// NewCoreNode creates a new core node with transport manager
func NewCoreNode(id, address string, priority int, tlsConfig transport.TLSConfig) *CoreNode {
	config := transport.Config{
		Type:       transport.GRPC,
		ServerAddr: address,
		TLS:        tlsConfig,
		WebSocket: transport.WSConfig{
			PingInterval: 30 * time.Second,
			ReadDeadline: 60 * time.Second,
		},
	}

	return &CoreNode{
		ID:       id,
		Address:  address,
		Priority: priority,
		Active:   transport.NewManager(config),
		State:    NodeStateStandby,
	}
}

// NewCoreNodeWithTLS creates a new core node with mTLS configuration
func NewCoreNodeWithTLS(id, address string, priority int, certPath, keyPath, caPath string) *CoreNode {
	return NewCoreNode(id, address, priority, transport.TLSConfig{
		CertPath:   certPath,
		KeyPath:    keyPath,
		CAPath:     caPath,
		MinVersion: "TLS1.3",
		ServerName: "mandau-core",
	})
}

// NewCoreNodeInsecure creates a new core node without TLS (for development)
func NewCoreNodeInsecure(id, address string, priority int) *CoreNode {
	config := transport.Config{
		Type:       transport.GRPC,
		ServerAddr: address,
	}

	return &CoreNode{
		ID:       id,
		Address:  address,
		Priority: priority,
		Active:   transport.NewManager(config),
		State:    NodeStateStandby,
	}
}

// NewCoreNodeInsecureWithCreds creates a new core node without TLS but with custom credentials
func NewCoreNodeInsecureWithCreds(id, address string, priority int) *CoreNode {
	return NewCoreNodeInsecure(id, address, priority)
}
