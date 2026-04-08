package operation

import (
	"testing"
	"time"

	"github.com/bhangun/mandau/pkg/agent/queue"
)

func TestNewManager(t *testing.T) {
	q, _ := queue.New(t.TempDir())
	mgr := NewManager(q)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.operations == nil {
		t.Error("operations map not initialized")
	}
	if mgr.listeners == nil {
		t.Error("listeners map not initialized")
	}
	if mgr.queue != q {
		t.Error("queue not set correctly")
	}
	if !mgr.IsConnected() {
		t.Error("manager should be connected by default")
	}
}

func TestCreateOperation(t *testing.T) {
	mgr := NewManager(nil)

	metadata := map[string]string{"stack": "web-app", "agent": "agent-001"}
	opID := mgr.CreateOperation(OperationTypeStackApply, metadata)

	if opID == "" {
		t.Fatal("CreateOperation returned empty ID")
	}

	op, err := mgr.GetOperation(opID)
	if err != nil {
		t.Fatalf("GetOperation failed: %v", err)
	}

	if op.Type != OperationTypeStackApply {
		t.Errorf("expected type %s, got %s", OperationTypeStackApply, op.Type)
	}
	if op.State != OperationStatePending {
		t.Errorf("expected state pending, got %d", op.State)
	}
	if op.Metadata["stack"] != "web-app" {
		t.Errorf("expected metadata stack=web-app, got %s", op.Metadata["stack"])
	}
}

func TestGetOperation(t *testing.T) {
	mgr := NewManager(nil)
	opID := mgr.CreateOperation(OperationTypeStackApply, nil)

	t.Run("existing operation", func(t *testing.T) {
		op, err := mgr.GetOperation(opID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if op.ID != opID {
			t.Errorf("expected ID %s, got %s", opID, op.ID)
		}
	})

	t.Run("non-existent operation", func(t *testing.T) {
		_, err := mgr.GetOperation("non-existent")
		if err == nil {
			t.Error("expected error for non-existent operation")
		}
	})
}

func TestListOperations(t *testing.T) {
	mgr := NewManager(nil)

	mgr.CreateOperation(OperationTypeStackApply, nil)
	mgr.CreateOperation(OperationTypeStackRemove, nil)
	mgr.CreateOperation(OperationTypeStackApply, nil)

	// List all
	all := mgr.ListOperations(nil)
	if len(all) != 3 {
		t.Errorf("expected 3 operations, got %d", len(all))
	}

	// Filter by type
	filter := func(op *Operation) bool {
		return op.Type == OperationTypeStackApply
	}
	filtered := mgr.ListOperations(filter)
	if len(filtered) != 2 {
		t.Errorf("expected 2 stack apply operations, got %d", len(filtered))
	}
}

func TestSetState(t *testing.T) {
	mgr := NewManager(nil)
	opID := mgr.CreateOperation(OperationTypeStackApply, nil)

	// Subscribe to events
	ch := mgr.Subscribe(opID)

	// Drain initial subscription event
	select {
	case <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for initial event")
		return
	}

	mgr.SetState(opID, OperationStateRunning)

	// Wait for state change event
	select {
	case event := <-ch:
		if event.State != OperationStateRunning {
			t.Errorf("expected state running, got %d", event.State)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for state change event")
	}

	// Verify state
	op, _ := mgr.GetOperation(opID)
	if op.State != OperationStateRunning {
		t.Errorf("expected state running, got %d", op.State)
	}
}

func TestSetProgress(t *testing.T) {
	mgr := NewManager(nil)
	opID := mgr.CreateOperation(OperationTypeStackApply, nil)

	ch := mgr.Subscribe(opID)

	// Drain initial event
	<-ch

	mgr.SetProgress(opID, 50)

	select {
	case event := <-ch:
		if event.Progress != 50 {
			t.Errorf("expected progress 50, got %d", event.Progress)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for progress event")
	}

	op, _ := mgr.GetOperation(opID)
	if op.Progress != 50 {
		t.Errorf("expected progress 50, got %d", op.Progress)
	}
}

func TestEmitEvent(t *testing.T) {
	mgr := NewManager(nil)
	opID := mgr.CreateOperation(OperationTypeStackApply, nil)

	ch := mgr.Subscribe(opID)

	// Drain initial event
	<-ch

	mgr.EmitEvent(opID, "Pulling images...")

	select {
	case event := <-ch:
		if event.Message != "Pulling images..." {
			t.Errorf("expected message 'Pulling images...', got %s", event.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestSetError(t *testing.T) {
	mgr := NewManager(nil)
	opID := mgr.CreateOperation(OperationTypeStackApply, nil)

	ch := mgr.Subscribe(opID)

	// Drain initial event
	<-ch

	mgr.SetError(opID, testError("operation failed"))

	select {
	case event := <-ch:
		if event.State != OperationStateFailed {
			t.Errorf("expected state failed, got %d", event.State)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for error event")
	}

	op, _ := mgr.GetOperation(opID)
	if op.State != OperationStateFailed {
		t.Errorf("expected state failed, got %d", op.State)
	}
	if op.Error == nil {
		t.Error("expected error to be set")
	}
	if op.CompletedAt == nil {
		t.Error("expected completed at to be set")
	}
}

func TestSetCompleted(t *testing.T) {
	mgr := NewManager(nil)
	opID := mgr.CreateOperation(OperationTypeStackApply, nil)

	ch := mgr.Subscribe(opID)

	// Drain initial event
	<-ch

	mgr.SetCompleted(opID)

	select {
	case event := <-ch:
		if event.State != OperationStateCompleted {
			t.Errorf("expected state completed, got %d", event.State)
		}
		if event.Progress != 100 {
			t.Errorf("expected progress 100, got %d", event.Progress)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for completion event")
	}

	op, _ := mgr.GetOperation(opID)
	if op.State != OperationStateCompleted {
		t.Errorf("expected state completed, got %d", op.State)
	}
	if op.Progress != 100 {
		t.Errorf("expected progress 100, got %d", op.Progress)
	}
}

func TestCancel(t *testing.T) {
	mgr := NewManager(nil)
	opID := mgr.CreateOperation(OperationTypeStackApply, nil)

	ch := mgr.Subscribe(opID)

	// Drain initial event
	<-ch

	err := mgr.Cancel(opID)
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	select {
	case event := <-ch:
		if event.State != OperationStateCancelled {
			t.Errorf("expected state cancelled, got %d", event.State)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for cancel event")
	}

	op, _ := mgr.GetOperation(opID)
	if op.State != OperationStateCancelled {
		t.Errorf("expected state cancelled, got %d", op.State)
	}
}

func TestCancelAlreadyCompleted(t *testing.T) {
	mgr := NewManager(nil)
	opID := mgr.CreateOperation(OperationTypeStackApply, nil)
	mgr.SetCompleted(opID)

	err := mgr.Cancel(opID)
	if err == nil {
		t.Error("expected error when cancelling completed operation")
	}
}

func TestSubscribe(t *testing.T) {
	mgr := NewManager(nil)
	opID := mgr.CreateOperation(OperationTypeStackApply, nil)

	ch := mgr.Subscribe(opID)

	// Should receive initial state event
	select {
	case event := <-ch:
		if event.Message != "Subscribed to operation" {
			t.Errorf("expected subscription message, got %s", event.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for initial event")
	}
}

func TestUnsubscribe(t *testing.T) {
	mgr := NewManager(nil)
	opID := mgr.CreateOperation(OperationTypeStackApply, nil)

	ch := mgr.Subscribe(opID)
	
	// Drain initial event
	<-ch
	
	// Verify listener count before unsubscribe
	mgr.mu.Lock()
	listenerCountBefore := len(mgr.listeners[opID])
	mgr.mu.Unlock()
	
	if listenerCountBefore != 1 {
		t.Errorf("expected 1 listener before unsubscribe, got %d", listenerCountBefore)
	}

	mgr.Unsubscribe(opID, ch)

	// Give it a moment for the unsubscribe to process
	time.Sleep(10 * time.Millisecond)
	
	// Verify listener was removed
	mgr.mu.Lock()
	listenerCountAfter := len(mgr.listeners[opID])
	mgr.mu.Unlock()
	
	if listenerCountAfter != 0 {
		t.Errorf("expected 0 listeners after unsubscribe, got %d", listenerCountAfter)
	}
}

func TestSetConnectionState(t *testing.T) {
	q, _ := queue.New(t.TempDir())
	mgr := NewManager(q)

	// Enqueue some operations
	q.Enqueue(&queue.Operation{
		ID:   "queued-op-1",
		Type: "stack.apply",
	})

	// Disconnect
	mgr.SetConnectionState(false)
	if mgr.IsConnected() {
		t.Error("expected disconnected")
	}

	// Reconnect
	mgr.SetConnectionState(true)
	if !mgr.IsConnected() {
		t.Error("expected connected")
	}
}

func TestGetPendingQueueCount(t *testing.T) {
	q, _ := queue.New(t.TempDir())
	mgr := NewManager(q)

	if mgr.GetPendingQueueCount() != 0 {
		t.Error("expected 0 pending operations")
	}

	q.Enqueue(&queue.Operation{ID: "op-1", Type: "test"})
	q.Enqueue(&queue.Operation{ID: "op-2", Type: "test"})

	if mgr.GetPendingQueueCount() != 2 {
		t.Errorf("expected 2 pending operations, got %d", mgr.GetPendingQueueCount())
	}
}

func TestOperationErrorQueuesWhenDisconnected(t *testing.T) {
	q, _ := queue.New(t.TempDir())
	mgr := NewManager(q)
	opID := mgr.CreateOperation(OperationTypeStackApply, map[string]string{"key": "value"})

	// Disconnect
	mgr.SetConnectionState(false)

	// Set error - should queue instead of fail
	mgr.SetError(opID, testError("connection lost"))

	op, _ := mgr.GetOperation(opID)
	if op.State != OperationStatePending {
		t.Errorf("expected operation to be queued as pending, got state %d", op.State)
	}

	// Verify in queue
	queuedOp, _ := q.Get(opID)
	if queuedOp == nil {
		t.Error("expected operation to be in queue")
	}
}

type testError string

func (e testError) Error() string {
	return string(e)
}
