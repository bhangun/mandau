// Package election provides Raft-based leader election for multi-core HA setups
package election

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// State represents the current state of a node in the cluster
type State string

const (
	StateFollower  State = "follower"
	StateCandidate State = "candidate"
	StateLeader    State = "leader"
)

// Cluster represents a Raft consensus cluster for leader election
type Cluster struct {
	raft         *raft.Raft
	transport    *raft.NetworkTransport
	config       *Config
	mu           sync.RWMutex
	state        State
	listeners    []chan StateChange
	listenersMu  sync.RWMutex
	storageDir   string
}

// Config holds configuration for the Raft cluster
type Config struct {
	// NodeID is the unique identifier for this node
	NodeID string
	// Addr is the address this node listens on
	Addr string
	// Peers is the list of all peer addresses in the cluster
	Peers []string
	// BootstrapExpect is the number of nodes needed to bootstrap
	BootstrapExpect int
	// HeartbeatTimeout is the timeout for leader heartbeats
	HeartbeatTimeout time.Duration
	// ElectionTimeout is the timeout for elections
	ElectionTimeout time.Duration
	// CommitTimeout is the timeout for commit operations
	CommitTimeout time.Duration
}

// StateChange represents a change in cluster state
type StateChange struct {
	OldState State
	NewState State
	Leader   string
}

// NewCluster creates a new Raft cluster for leader election
func NewCluster(config Config) (*Cluster, error) {
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 1000 * time.Millisecond
	}
	if config.ElectionTimeout == 0 {
		config.ElectionTimeout = 1000 * time.Millisecond
	}
	if config.CommitTimeout == 0 {
		config.CommitTimeout = 50 * time.Millisecond
	}

	storageDir := filepath.Join(os.TempDir(), "mandau-raft", config.NodeID)
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}

	c := &Cluster{
		config:     &config,
		state:      StateFollower,
		storageDir: storageDir,
	}

	if err := c.setupRaft(); err != nil {
		return nil, fmt.Errorf("setup raft: %w", err)
	}

	return c, nil
}

// setupRaft initializes the Raft node
func (c *Cluster) setupRaft() error {
	// Create Raft configuration
	raftConfig := &raft.Config{
		LocalID:            raft.ServerID(c.config.NodeID),
		HeartbeatTimeout:   c.config.HeartbeatTimeout,
		ElectionTimeout:    c.config.ElectionTimeout,
		CommitTimeout:      c.config.CommitTimeout,
		MaxAppendEntries:   64,
		SnapshotThreshold:  8192,
		TrailingLogs:       10240,
	}

	// Create transport
	addr, err := net.ResolveTCPAddr("tcp", c.config.Addr)
	if err != nil {
		return fmt.Errorf("resolve address: %w", err)
	}

	transport, err := raft.NewTCPTransport(c.config.Addr, nil, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return fmt.Errorf("create transport: %w", err)
	}
	c.transport = transport

	// Create BoltDB stable store
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(c.storageDir, "raft.db"))
	if err != nil {
		return fmt.Errorf("create stable store: %w", err)
	}

	// Create in-memory snapshots store (for simplicity)
	snapshots, err := raft.NewFileSnapshotStore(c.storageDir, 3, os.Stderr)
	if err != nil {
		return fmt.Errorf("create snapshot store: %w", err)
	}

	// Create Raft instance with correct parameter order:
	// NewRaft(config, fsm, logStore, stableStore, snapshots, transport)
	c.raft, err = raft.NewRaft(raftConfig, &fsm{}, stableStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %w", err)
	}

	_ = addr // Address is used by the transport internally

	// Bootstrap cluster if this is the first node
	if len(c.config.Peers) == 0 || len(c.config.Peers) < c.config.BootstrapExpect {
		if err := c.bootstrapCluster(); err != nil {
			log.Printf("Warning: bootstrap failed: %v", err)
		}
	}

	// Join existing cluster if peers are specified
	for _, peer := range c.config.Peers {
		if peer == c.config.Addr {
			continue
		}

		future := c.raft.AddVoter(raft.ServerID(peer), raft.ServerAddress(peer), 0, 0)
		if err := future.Error(); err != nil {
			log.Printf("Warning: failed to add peer %s: %v", peer, err)
		}
	}

	// Start monitoring goroutine
	go c.monitorState()

	return nil
}

// bootstrapCluster bootstraps the Raft cluster with initial configuration
func (c *Cluster) bootstrapCluster() error {
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(c.config.NodeID),
				Address: raft.ServerAddress(c.config.Addr),
			},
		},
	}

	// Add peers to configuration
	for _, peer := range c.config.Peers {
		if peer == c.config.Addr {
			continue
		}
		configuration.Servers = append(configuration.Servers, raft.Server{
			ID:      raft.ServerID(peer),
			Address: raft.ServerAddress(peer),
		})
	}

	future := c.raft.BootstrapCluster(configuration)
	return future.Error()
}

// IsLeader returns true if this node is the leader
func (c *Cluster) IsLeader() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state == StateLeader
}

// GetLeader returns the address of the current leader
func (c *Cluster) GetLeader() string {
	if c.raft == nil {
		return ""
	}
	return string(c.raft.Leader())
}

// GetState returns the current state of this node
func (c *Cluster) GetState() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// GetRaft returns the underlying Raft instance
func (c *Cluster) GetRaft() *raft.Raft {
	return c.raft
}

// Apply applies a command to the Raft cluster
func (c *Cluster) Apply(cmd []byte, timeout time.Duration) error {
	if !c.IsLeader() {
		return fmt.Errorf("not the leader")
	}

	future := c.raft.Apply(cmd, timeout)
	return future.Error()
}

// Subscribe subscribes to state changes
func (c *Cluster) Subscribe() <-chan StateChange {
	ch := make(chan StateChange, 10)
	c.listenersMu.Lock()
	c.listeners = append(c.listeners, ch)
	c.listenersMu.Unlock()
	return ch
}

// Unsubscribe unsubscribes from state changes
func (c *Cluster) Unsubscribe(ch <-chan StateChange) {
	c.listenersMu.Lock()
	defer c.listenersMu.Unlock()

	for i, listener := range c.listeners {
		// Compare channels by pointer
		if fmt.Sprintf("%p", listener) == fmt.Sprintf("%p", ch) {
			c.listeners = append(c.listeners[:i], c.listeners[i+1:]...)
			// Don't close the channel here - the subscriber owns it
			break
		}
	}
}

// monitorState monitors Raft state changes and notifies listeners
func (c *Cluster) monitorState() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if c.raft == nil {
			continue
		}

		oldState := c.state
		var newState State

		switch c.raft.State() {
		case raft.Leader:
			newState = StateLeader
		case raft.Candidate:
			newState = StateCandidate
		default:
			newState = StateFollower
		}

		if oldState != newState {
			c.mu.Lock()
			c.state = newState
			c.mu.Unlock()

			change := StateChange{
				OldState: oldState,
				NewState: newState,
				Leader:   c.GetLeader(),
			}

			c.notifyListeners(change)
		}
	}
}

// notifyListeners notifies all listeners of state changes
func (c *Cluster) notifyListeners(change StateChange) {
	c.listenersMu.RLock()
	defer c.listenersMu.RUnlock()

	for _, ch := range c.listeners {
		select {
		case ch <- change:
		default:
			// Channel full, skip
		}
	}
}

// Shutdown gracefully shuts down the Raft cluster
func (c *Cluster) Shutdown() error {
	if c.raft != nil {
		future := c.raft.Shutdown()
		if err := future.Error(); err != nil {
			return fmt.Errorf("shutdown raft: %w", err)
		}
	}
	return nil
}

// Leave gracefully removes this node from the cluster
func (c *Cluster) Leave() error {
	if c.raft == nil {
		return nil
	}

	future := c.raft.RemoveServer(raft.ServerID(c.config.NodeID), 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("remove server: %w", err)
	}

	return c.Shutdown()
}

// GetClusterSize returns the number of nodes in the cluster
func (c *Cluster) GetClusterSize() int {
	if c.raft == nil {
		return 0
	}

	configFuture := c.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return 0
	}

	return len(configFuture.Configuration().Servers)
}

// ListPeers returns the list of peers in the cluster
func (c *Cluster) ListPeers() []string {
	if c.raft == nil {
		return nil
	}

	configFuture := c.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return nil
	}

	peers := make([]string, 0)
	for _, server := range configFuture.Configuration().Servers {
		peers = append(peers, string(server.Address))
	}

	return peers
}

// fsm implements raft.FSM (Finite State Machine)
type fsm struct {
	mu sync.RWMutex
}

func (f *fsm) Apply(log *raft.Log) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Apply a log entry to the FSM
	// In a real implementation, this would update the state
	return nil
}

func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	// Return a snapshot of the FSM state
	return &fsmSnapshot{}, nil
}

func (f *fsm) Restore(old io.ReadCloser) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Restore the FSM from a snapshot
	return nil
}

// fsmSnapshot implements raft.FSMSnapshot
type fsmSnapshot struct{}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	// Persist the snapshot
	return sink.Close()
}

func (s *fsmSnapshot) Release() {
	// Release resources
}
