// Package queue provides persistent operation queuing for disconnected scenarios.
// Operations are queued to disk and executed when connectivity is restored.
package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Operation represents a queued operation to be executed when connected
type Operation struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	AgentID   string                 `json:"agent_id"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
	Attempts  int                    `json:"attempts"`
	MaxRetry  int                    `json:"max_retry"`
	State     OperationState         `json:"state"`
	Result    *OperationResult       `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// OperationState represents the state of a queued operation
type OperationState string

const (
	StatePending   OperationState = "pending"
	StateExecuting OperationState = "executing"
	StateCompleted OperationState = "completed"
	StateFailed    OperationState = "failed"
	StateCancelled OperationState = "cancelled"
)

// OperationResult holds the result of an executed operation
type OperationResult struct {
	Success  bool                   `json:"success"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Duration time.Duration          `json:"duration"`
}

// Queue manages persistent operation queuing
type Queue struct {
	queueDir   string
	operations map[string]*Operation
	mu         sync.RWMutex
	executing  map[string]bool
	execMu     sync.RWMutex
}

// New creates a new operation queue
func New(queueDir string) (*Queue, error) {
	if err := os.MkdirAll(queueDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create queue directory: %w", err)
	}

	q := &Queue{
		queueDir:   queueDir,
		operations: make(map[string]*Operation),
		executing:  make(map[string]bool),
	}

	// Load existing operations from disk
	if err := q.loadFromDisk(); err != nil {
		return nil, fmt.Errorf("failed to load operations from disk: %w", err)
	}

	return q, nil
}

// Enqueue adds an operation to the queue
func (q *Queue) Enqueue(op *Operation) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if op.State == "" {
		op.State = StatePending
	}
	if op.MaxRetry == 0 {
		op.MaxRetry = 3
	}
	if op.CreatedAt.IsZero() {
		op.CreatedAt = time.Now()
	}

	q.operations[op.ID] = op

	// Persist to disk
	if err := q.saveOperation(op); err != nil {
		delete(q.operations, op.ID)
		return fmt.Errorf("failed to persist operation: %w", err)
	}

	return nil
}

// Dequeue returns the next pending operation
func (q *Queue) Dequeue() *Operation {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Find the oldest pending operation by CreatedAt
	var oldest *Operation
	for _, op := range q.operations {
		if op.State == StatePending {
			if oldest == nil || op.CreatedAt.Before(oldest.CreatedAt) {
				oldest = op
			}
		}
	}

	if oldest != nil {
		oldest.State = StateExecuting
		q.saveOperation(oldest)
		return oldest
	}

	return nil
}

// Complete marks an operation as completed
func (q *Queue) Complete(opID string, result *OperationResult) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	op, exists := q.operations[opID]
	if !exists {
		return fmt.Errorf("operation %s not found", opID)
	}

	op.State = StateCompleted
	op.Result = result

	q.execMu.Lock()
	delete(q.executing, opID)
	q.execMu.Unlock()

	return q.saveOperation(op)
}

// Fail marks an operation as failed, with retry logic
func (q *Queue) Fail(opID string, err error) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	op, exists := q.operations[opID]
	if !exists {
		return fmt.Errorf("operation %s not found", opID)
	}

	op.Attempts++
	op.Error = err.Error()

	q.execMu.Lock()
	delete(q.executing, opID)
	q.execMu.Unlock()

	if op.Attempts >= op.MaxRetry {
		op.State = StateFailed
	} else {
		// Reset to pending for retry
		op.State = StatePending
	}

	return q.saveOperation(op)
}

// Cancel marks an operation as cancelled
func (q *Queue) Cancel(opID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	op, exists := q.operations[opID]
	if !exists {
		return fmt.Errorf("operation %s not found", opID)
	}

	op.State = StateCancelled
	return q.saveOperation(op)
}

// Get returns a specific operation by ID
func (q *Queue) Get(opID string) (*Operation, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	op, exists := q.operations[opID]
	if !exists {
		return nil, fmt.Errorf("operation %s not found", opID)
	}

	return op, nil
}

// List returns all operations, optionally filtered by state
func (q *Queue) List(filter *OperationState) []*Operation {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []*Operation
	for _, op := range q.operations {
		if filter == nil || op.State == *filter {
			result = append(result, op)
		}
	}

	return result
}

// PendingCount returns the number of pending operations
func (q *Queue) PendingCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	count := 0
	for _, op := range q.operations {
		if op.State == StatePending {
			count++
		}
	}

	return count
}

// Clear removes all completed/failed/cancelled operations
func (q *Queue) Clear() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for id, op := range q.operations {
		if op.State == StateCompleted || op.State == StateFailed || op.State == StateCancelled {
			delete(q.operations, id)
			if err := q.deleteOperationFile(id); err != nil {
				return fmt.Errorf("failed to delete operation file: %w", err)
			}
		}
	}

	return nil
}

// IsExecuting returns true if an operation is currently executing
func (q *Queue) IsExecuting(opID string) bool {
	q.execMu.RLock()
	defer q.execMu.RUnlock()
	return q.executing[opID]
}

// MarkExecuting marks an operation as executing
func (q *Queue) MarkExecuting(opID string) {
	q.execMu.Lock()
	defer q.execMu.Unlock()
	q.executing[opID] = true
}

// saveOperation persists an operation to disk
func (q *Queue) saveOperation(op *Operation) error {
	data, err := json.MarshalIndent(op, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal operation: %w", err)
	}

	path := filepath.Join(q.queueDir, op.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// deleteOperationFile removes an operation file from disk
func (q *Queue) deleteOperationFile(opID string) error {
	path := filepath.Join(q.queueDir, opID+".json")
	return os.Remove(path)
}

// loadFromDisk loads operations from disk into memory
func (q *Queue) loadFromDisk() error {
	entries, err := os.ReadDir(q.queueDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(q.queueDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read operation file %s: %w", path, err)
		}

		var op Operation
		if err := json.Unmarshal(data, &op); err != nil {
			return fmt.Errorf("failed to unmarshal operation file %s: %w", path, err)
		}

		// Reset executing state on load (operations should retry)
		if op.State == StateExecuting {
			op.State = StatePending
		}

		q.operations[op.ID] = &op
	}

	return nil
}

// Close cleans up the queue (no-op for disk-based queue)
func (q *Queue) Close() error {
	// Disk-based queue doesn't need cleanup on close
	return nil
}
