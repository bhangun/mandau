package queue

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	dir := t.TempDir()
	
	q, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	if q == nil {
		t.Fatal("New returned nil")
	}
	if q.queueDir != dir {
		t.Errorf("expected queue dir %s, got %s", dir, q.queueDir)
	}
}

func TestNewCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-queue")
	
	q, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer q.Close()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("queue directory was not created")
	}
}

func TestEnqueue(t *testing.T) {
	dir := t.TempDir()
	q, _ := New(dir)

	op := &Operation{
		ID:       "test-op-1",
		Type:     "stack.apply",
		AgentID:  "agent-001",
		Payload:  map[string]interface{}{"key": "value"},
		MaxRetry: 3,
	}

	err := q.Enqueue(op)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// Verify operation is in memory
	if q.operations["test-op-1"] == nil {
		t.Error("operation not found in memory")
	}

	// Verify operation is persisted
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestEnqueueSetsDefaults(t *testing.T) {
	dir := t.TempDir()
	q, _ := New(dir)

	op := &Operation{
		ID:   "test-op-2",
		Type: "test",
	}

	err := q.Enqueue(op)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	retrieved, _ := q.Get("test-op-2")
	if retrieved.State != StatePending {
		t.Errorf("expected state pending, got %s", retrieved.State)
	}
	if retrieved.MaxRetry != 3 {
		t.Errorf("expected max retry 3, got %d", retrieved.MaxRetry)
	}
	if retrieved.CreatedAt.IsZero() {
		t.Error("created at was not set")
	}
}

func TestDequeue(t *testing.T) {
	dir := t.TempDir()
	q, _ := New(dir)

	// Enqueue two operations
	q.Enqueue(&Operation{ID: "op-1", Type: "test"})
	q.Enqueue(&Operation{ID: "op-2", Type: "test"})

	// Dequeue first
	op1 := q.Dequeue()
	if op1 == nil {
		t.Fatal("expected first operation")
	}
	if op1.ID != "op-1" {
		t.Errorf("expected op-1, got %s", op1.ID)
	}
	if op1.State != StateExecuting {
		t.Errorf("expected state executing, got %s", op1.State)
	}

	// Dequeue second
	op2 := q.Dequeue()
	if op2 == nil {
		t.Fatal("expected second operation")
	}
	if op2.ID != "op-2" {
		t.Errorf("expected op-2, got %s", op2.ID)
	}

	// No more operations
	op3 := q.Dequeue()
	if op3 != nil {
		t.Error("expected nil when queue is empty")
	}
}

func TestComplete(t *testing.T) {
	dir := t.TempDir()
	q, _ := New(dir)

	q.Enqueue(&Operation{ID: "op-1", Type: "test"})

	result := &OperationResult{
		Success:  true,
		Duration: 5 * time.Second,
	}

	err := q.Complete("op-1", result)
	if err != nil {
		t.Fatalf("failed to complete operation: %v", err)
	}

	op, _ := q.Get("op-1")
	if op.State != StateCompleted {
		t.Errorf("expected state completed, got %s", op.State)
	}
	if op.Result == nil || !op.Result.Success {
		t.Error("result not set correctly")
	}
}

func TestFail(t *testing.T) {
	dir := t.TempDir()
	q, _ := New(dir)

	t.Run("first failure resets to pending", func(t *testing.T) {
		q.Enqueue(&Operation{ID: "op-1", Type: "test", MaxRetry: 3})

		err := q.Fail("op-1", os.ErrNotExist)
		if err != nil {
			t.Fatalf("failed to mark operation as failed: %v", err)
		}

		op, _ := q.Get("op-1")
		if op.State != StatePending {
			t.Errorf("expected state pending for retry, got %s", op.State)
		}
		if op.Attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", op.Attempts)
		}
	})

	t.Run("exceeds max retry", func(t *testing.T) {
		q.Enqueue(&Operation{ID: "op-2", Type: "test", MaxRetry: 1})

		err := q.Fail("op-2", os.ErrNotExist)
		if err != nil {
			t.Fatalf("failed to mark operation as failed: %v", err)
		}

		op, _ := q.Get("op-2")
		if op.State != StateFailed {
			t.Errorf("expected state failed, got %s", op.State)
		}
	})
}

func TestCancel(t *testing.T) {
	dir := t.TempDir()
	q, _ := New(dir)

	q.Enqueue(&Operation{ID: "op-1", Type: "test"})

	err := q.Cancel("op-1")
	if err != nil {
		t.Fatalf("failed to cancel operation: %v", err)
	}

	op, _ := q.Get("op-1")
	if op.State != StateCancelled {
		t.Errorf("expected state cancelled, got %s", op.State)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	q, _ := New(dir)

	q.Enqueue(&Operation{ID: "op-1", Type: "test"})

	op, err := q.Get("op-1")
	if err != nil {
		t.Fatalf("failed to get operation: %v", err)
	}
	if op.ID != "op-1" {
		t.Errorf("expected op-1, got %s", op.ID)
	}

	_, err = q.Get("non-existent")
	if err == nil {
		t.Error("expected error for non-existent operation")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	q, _ := New(dir)

	q.Enqueue(&Operation{ID: "op-1", Type: "test"})
	q.Enqueue(&Operation{ID: "op-2", Type: "test"})
	q.Enqueue(&Operation{ID: "op-3", Type: "test"})

	// List all
	all := q.List(nil)
	if len(all) != 3 {
		t.Errorf("expected 3 operations, got %d", len(all))
	}

	// List pending
	pendingFilter := StatePending
	pending := q.List(&pendingFilter)
	if len(pending) != 3 {
		t.Errorf("expected 3 pending operations, got %d", len(pending))
	}

	// Complete one
	q.Complete("op-1", &OperationResult{Success: true})

	completedFilter := StateCompleted
	completed := q.List(&completedFilter)
	if len(completed) != 1 {
		t.Errorf("expected 1 completed operation, got %d", len(completed))
	}
}

func TestPendingCount(t *testing.T) {
	dir := t.TempDir()
	q, _ := New(dir)

	if q.PendingCount() != 0 {
		t.Error("expected 0 pending operations")
	}

	q.Enqueue(&Operation{ID: "op-1", Type: "test"})
	q.Enqueue(&Operation{ID: "op-2", Type: "test"})
	q.Complete("op-1", &OperationResult{Success: true})

	if q.PendingCount() != 1 {
		t.Errorf("expected 1 pending operation, got %d", q.PendingCount())
	}
}

func TestClear(t *testing.T) {
	dir := t.TempDir()
	q, _ := New(dir)

	q.Enqueue(&Operation{ID: "op-1", Type: "test"})
	q.Enqueue(&Operation{ID: "op-2", Type: "test", MaxRetry: 1}) // Will fail after 1 attempt
	q.Enqueue(&Operation{ID: "op-3", Type: "test"})

	q.Complete("op-1", &OperationResult{Success: true})
	q.Fail("op-2", os.ErrNotExist) // Will be StateFailed since MaxRetry=1
	// op-3 remains pending

	err := q.Clear()
	if err != nil {
		t.Fatalf("failed to clear operations: %v", err)
	}

	// Only pending (op-3) should remain
	if len(q.operations) != 1 {
		t.Errorf("expected 1 operation after clear, got %d", len(q.operations))
	}

	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Errorf("expected 1 file after clear, got %d", len(files))
	}
}

func TestIsExecuting(t *testing.T) {
	dir := t.TempDir()
	q, _ := New(dir)

	if q.IsExecuting("op-1") {
		t.Error("expected false for non-executing operation")
	}

	q.MarkExecuting("op-1")
	if !q.IsExecuting("op-1") {
		t.Error("expected true for executing operation")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	// Create queue and add operations
	q1, _ := New(dir)
	q1.Enqueue(&Operation{ID: "op-1", Type: "test"})
	q1.Enqueue(&Operation{ID: "op-2", Type: "test"})
	q1.Complete("op-1", &OperationResult{Success: true})

	// Create new queue instance (simulates restart)
	q2, _ := New(dir)

	// Verify operations are loaded
	if len(q2.operations) != 2 {
		t.Errorf("expected 2 operations after reload, got %d", len(q2.operations))
	}

	op, _ := q2.Get("op-1")
	if op == nil {
		t.Fatal("operation op-1 not found after reload")
	}
	if op.State != StateCompleted {
		t.Errorf("expected state completed, got %s", op.State)
	}
}

func TestLoadFromDiskResetsExecuting(t *testing.T) {
	dir := t.TempDir()

	// Create queue and mark as executing
	q1, _ := New(dir)
	q1.Enqueue(&Operation{ID: "op-1", Type: "test"})
	q1.MarkExecuting("op-1")
	
	// Manually set state to executing (simulate crash during execution)
	q1.mu.Lock()
	q1.operations["op-1"].State = StateExecuting
	q1.mu.Unlock()

	// Reload
	q2, _ := New(dir)

	op, _ := q2.Get("op-1")
	if op.State != StatePending {
		t.Errorf("expected executing to reset to pending on reload, got %s", op.State)
	}
}
