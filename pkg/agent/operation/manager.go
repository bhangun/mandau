package operation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Manager struct {
	mu         sync.RWMutex
	operations map[string]*Operation
	listeners  map[string][]chan Event
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

func NewManager() *Manager {
	return &Manager{
		operations: make(map[string]*Operation),
		listeners:  make(map[string][]chan Event),
	}
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

	op.State = OperationStateFailed
	op.Error = err
	now := time.Now()
	op.CompletedAt = &now

	m.emitEventLocked(Event{
		OperationID: opID,
		State:       OperationStateFailed,
		Error:       err,
		Timestamp:   now,
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
