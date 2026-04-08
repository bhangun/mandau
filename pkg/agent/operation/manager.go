package operation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/bhangun/mandau/pkg/agent/queue"
)

type Manager struct {
	mu         sync.RWMutex
	operations map[string]*Operation
	listeners  map[string][]chan Event
	queue      *queue.Queue
	connected  bool
	connMu     sync.RWMutex
}

type Operation struct {
	ID          string
	Type        OperationType
	State       OperationState
	CreatedAt   time.Time
	CompletedAt *time.Time
	Error       error
	Progress    int
	Metadata    map[string]string
	cancelFunc  context.CancelFunc
}

type OperationType string

const (
	OperationTypeStackApply  OperationType = "stack.apply"
	OperationTypeStackRemove OperationType = "stack.remove"
	OperationTypeImagePull   OperationType = "image.pull"
	OperationTypeExec        OperationType = "container.exec"
	OperationTypeBackup      OperationType = "backup"
)

type OperationState int

const (
	OperationStatePending OperationState = iota
	OperationStateRunning
	OperationStateCompleted
	OperationStateFailed
	OperationStateCancelled
)

type Event struct {
	OperationID string
	State       OperationState
	Timestamp   time.Time
	Message     string
	Progress    int
	Error       error
}

func NewManager(q *queue.Queue) *Manager {
	return &Manager{
		operations: make(map[string]*Operation),
		listeners:  make(map[string][]chan Event),
		queue:      q,
		connected:  true,
	}
}

// SetConnectionState updates the connection state and processes queued operations
func (m *Manager) SetConnectionState(connected bool) {
	m.connMu.Lock()
	defer m.connMu.Unlock()

	wasConnected := m.connected
	m.connected = connected

	// If we just reconnected, process queued operations
	if connected && !wasConnected && m.queue != nil {
		go m.processQueuedOperations()
	}
}

// IsConnected returns true if the agent is connected to core
func (m *Manager) IsConnected() bool {
	m.connMu.RLock()
	defer m.connMu.RUnlock()
	return m.connected
}

// processQueuedOperations processes operations from the queue when reconnected
func (m *Manager) processQueuedOperations() {
	if m.queue == nil {
		return
	}

	// Process up to 10 operations at a time
	for i := 0; i < 10; i++ {
		op := m.queue.Dequeue()
		if op == nil {
			break
		}

		// Create a new internal operation to track the queued one
		m.mu.Lock()
		internalOp := &Operation{
			ID:        op.ID,
			Type:      OperationType(op.Type),
			State:     OperationStateRunning,
			CreatedAt: op.CreatedAt,
			Metadata:  make(map[string]string),
		}

		// Restore metadata from payload
		for k, v := range op.Payload {
			if str, ok := v.(string); ok {
				internalOp.Metadata[k] = str
			}
		}

		m.operations[op.ID] = internalOp
		m.mu.Unlock()

		// Emit event that queued operation is now executing
		m.emitEventLocked(Event{
			OperationID: op.ID,
			State:       OperationStateRunning,
			Message:     "Executing queued operation after reconnection",
			Timestamp:   time.Now(),
		})

		// The actual execution is handled by the caller
		// Mark as executing in the queue
		m.queue.MarkExecuting(op.ID)
	}
}

// GetPendingQueueCount returns the number of operations waiting in the queue
func (m *Manager) GetPendingQueueCount() int {
	if m.queue == nil {
		return 0
	}
	return m.queue.PendingCount()
}

// emitEventLocked sends an event to all listeners for the operation
// Must be called with mu locked
func (m *Manager) emitEventLocked(event Event) {
	if listeners, exists := m.listeners[event.OperationID]; exists {
		for _, ch := range listeners {
			select {
			case ch <- event:
			default:
				// Channel is full, skip
			}
		}
	}
}

// CreateOperation creates a new operation
func (m *Manager) CreateOperation(opType OperationType, metadata map[string]string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	opID := uuid.New().String()

	_, cancel := context.WithCancel(context.Background())

	op := &Operation{
		ID:         opID,
		Type:       opType,
		State:      OperationStatePending,
		CreatedAt:  time.Now(),
		Metadata:   metadata,
		cancelFunc: cancel,
	}

	m.operations[opID] = op

	return opID
}

// GetOperation retrieves operation by ID
func (m *Manager) GetOperation(opID string) (*Operation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	op, exists := m.operations[opID]
	if !exists {
		return nil, fmt.Errorf("operation not found: %s", opID)
	}

	return op, nil
}

// ListOperations returns all operations
func (m *Manager) ListOperations(filter func(*Operation) bool) []*Operation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Operation, 0)
	for _, op := range m.operations {
		if filter == nil || filter(op) {
			result = append(result, op)
		}
	}

	return result
}

// SetState updates operation state
func (m *Manager) SetState(opID string, state OperationState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	op, exists := m.operations[opID]
	if !exists {
		return
	}

	op.State = state

	m.emitEventLocked(Event{
		OperationID: opID,
		State:       state,
		Timestamp:   time.Now(),
	})
}

// SetProgress updates operation progress
func (m *Manager) SetProgress(opID string, progress int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	op, exists := m.operations[opID]
	if !exists {
		return
	}

	op.Progress = progress

	m.emitEventLocked(Event{
		OperationID: opID,
		State:       op.State,
		Progress:    progress,
		Timestamp:   time.Now(),
	})
}

// EmitEvent sends a message event
func (m *Manager) EmitEvent(opID string, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	op, exists := m.operations[opID]
	if !exists {
		return
	}

	m.emitEventLocked(Event{
		OperationID: opID,
		State:       op.State,
		Message:     message,
		Timestamp:   time.Now(),
	})
}

// SetError marks operation as failed
func (m *Manager) SetError(opID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	op, exists := m.operations[opID]
	if !exists {
		return
	}

	// If disconnected, queue the operation instead of failing
	if !m.connected && m.queue != nil {
		queueOp := &queue.Operation{
			ID:        opID,
			Type:      string(op.Type),
			Payload:   make(map[string]interface{}),
			CreatedAt: op.CreatedAt,
			Attempts:  0,
			MaxRetry:  3,
			State:     queue.StatePending,
		}

		// Store metadata in payload
		for k, v := range op.Metadata {
			queueOp.Payload[k] = v
		}

		if queueErr := m.queue.Enqueue(queueOp); queueErr != nil {
			// If queue fails, proceed with normal error handling
			op.State = OperationStateFailed
			op.Error = err
			now := time.Now()
			op.CompletedAt = &now
		} else {
			// Operation queued successfully
			op.State = OperationStatePending
			op.Error = fmt.Errorf("operation queued for retry: %w", err)
			m.emitEventLocked(Event{
				OperationID: opID,
				State:       OperationStatePending,
				Message:     "Operation queued for retry when connected",
				Timestamp:   time.Now(),
			})
			return
		}
	} else {
		op.State = OperationStateFailed
		op.Error = err
		now := time.Now()
		op.CompletedAt = &now
	}

	m.emitEventLocked(Event{
		OperationID: opID,
		State:       OperationStateFailed,
		Error:       err,
		Timestamp:   time.Now(),
	})
}

// SetCompleted marks operation as completed
func (m *Manager) SetCompleted(opID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	op, exists := m.operations[opID]
	if !exists {
		return
	}

	op.State = OperationStateCompleted
	op.Progress = 100
	now := time.Now()
	op.CompletedAt = &now

	m.emitEventLocked(Event{
		OperationID: opID,
		State:       OperationStateCompleted,
		Progress:    100,
		Timestamp:   now,
	})
}

// Cancel cancels a running operation
func (m *Manager) Cancel(opID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	op, exists := m.operations[opID]
	if !exists {
		return fmt.Errorf("operation not found: %s", opID)
	}

	if op.State == OperationStateCompleted || op.State == OperationStateFailed {
		return fmt.Errorf("operation already finished")
	}

	op.cancelFunc()
	op.State = OperationStateCancelled
	now := time.Now()
	op.CompletedAt = &now

	m.emitEventLocked(Event{
		OperationID: opID,
		State:       OperationStateCancelled,
	})
	return nil
}

// Subscribe adds a listener for operation events
func (m *Manager) Subscribe(opID string) <-chan Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan Event, 10) // Buffered channel to prevent blocking
	m.listeners[opID] = append(m.listeners[opID], ch)

	// Send current state as first event
	if op, exists := m.operations[opID]; exists {
		event := Event{
			OperationID: opID,
			State:       op.State,
			Timestamp:   time.Now(),
			Message:     "Subscribed to operation",
			Progress:    op.Progress,
		}
		select {
		case ch <- event:
		default:
		}
	}

	return ch
}

// Unsubscribe removes a listener for operation events
func (m *Manager) Unsubscribe(opID string, ch <-chan Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if listeners, exists := m.listeners[opID]; exists {
		for i, listener := range listeners {
			if listener == ch {
				// Remove the listener
				m.listeners[opID] = append(listeners[:i], listeners[i+1:]...)
				// Close the channel
				close(listener)
				break
			}
		}
	}
}
