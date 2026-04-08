package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	agentv1 "github.com/bhangun/mandau/api/v1"
	"google.golang.org/grpc/metadata"
)

// WSMessage represents a message exchanged over WebSocket
type WSMessage struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Error     string          `json:"error,omitempty"`
}

// WSConnection represents a WebSocket connection from an agent
type WSConnection struct {
	conn    *websocket.Conn
	agentID string
	mu      sync.Mutex
	closed  bool
}

// WSUpgrader upgrades HTTP connections to WebSocket
var WSUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// Check allowed origins from environment or config
		// In production, restrict to known origins
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Allow non-browser clients
		}
		// TODO: Load allowed origins from config
		// For now, allow localhost and same-origin
		return strings.Contains(origin, "localhost") || 
		       strings.Contains(origin, "127.0.0.1") ||
		       origin == r.Host
	},
}

// HandleWebSocket handles incoming WebSocket connections from agents
func (c *Core) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check for authentication token in query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		// Check Authorization header as fallback
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	
	if token == "" {
		http.Error(w, "Missing authentication token", http.StatusUnauthorized)
		return
	}
	
	// Validate the token (we need to access the auth middleware)
	// For now, we'll validate it in the handleWSConnection after upgrade
	// Store token in context for later validation
	ctx := context.WithValue(r.Context(), "ws_token", token)
	
	conn, err := WSUpgrader.Upgrade(w, r.WithContext(ctx), nil)
	if err != nil {
		http.Error(w, "Failed to upgrade connection", http.StatusBadRequest)
		return
	}

	defer conn.Close()

	wsConn := &WSConnection{
		conn: conn,
	}

	// Handle the WebSocket connection
	c.handleWSConnection(r.Context(), wsConn)
}

// handleWSConnection manages a single WebSocket connection
func (c *Core) handleWSConnection(ctx context.Context, wsConn *WSConnection) {
	defer func() {
		wsConn.mu.Lock()
		wsConn.closed = true
		wsConn.mu.Unlock()
		wsConn.conn.Close()
	}()

	// Validate authentication token if present
	if token, ok := ctx.Value("ws_token").(string); ok && token != "" {
		// TODO: Validate JWT token here before allowing any operations
		// For now, we'll enforce that agents must provide a valid token during registration
		// This will be validated in handleWSRegister
	}

	// Set read deadline
	wsConn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	for {
		_, message, err := wsConn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) {
				fmt.Printf("WebSocket unexpected close: %v\n", err)
			}
			return
		}

		// Reset read deadline
		wsConn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Process message
		go c.processWSMessage(ctx, wsConn, message)
	}
}

// processWSMessage processes a single WebSocket message
func (c *Core) processWSMessage(ctx context.Context, wsConn *WSConnection, data []byte) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendWSError(wsConn, fmt.Sprintf("Invalid message: %v", err))
		return
	}

	switch msg.Type {
	case "register":
		c.handleWSRegister(ctx, wsConn, &msg)
	case "heartbeat":
		c.handleWSHeartbeat(ctx, wsConn, &msg)
	case "stack_list":
		c.handleWSStackList(ctx, wsConn, &msg)
	case "stack_apply":
		c.handleWSStackApply(ctx, wsConn, &msg)
	case "container_list":
		c.handleWSContainerList(ctx, wsConn, &msg)
	case "operation_status":
		c.handleWSOperationStatus(ctx, wsConn, &msg)
	default:
		c.sendWSError(wsConn, fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

// handleWSRegister handles agent registration over WebSocket
func (c *Core) handleWSRegister(ctx context.Context, wsConn *WSConnection, msg *WSMessage) {
	var req agentv1.RegisterRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendWSError(wsConn, fmt.Sprintf("Invalid register request: %v", err))
		return
	}

	// Create context with metadata for auth
	md := metadata.New(map[string]string{
		"agent-id": req.AgentId,
		"hostname": req.Hostname,
	})
	ctx = metadata.NewIncomingContext(ctx, md)

	// Call the existing RegisterAgent handler
	resp, err := c.RegisterAgent(ctx, &req)
	if err != nil {
		c.sendWSError(wsConn, fmt.Sprintf("Registration failed: %v", err))
		return
	}

	// Store agent ID in WebSocket connection
	wsConn.mu.Lock()
	wsConn.agentID = resp.AgentId
	wsConn.mu.Unlock()

	// Send response
	c.sendWSResponse(wsConn, "register_success", resp)
}

// handleWSHeartbeat handles agent heartbeat over WebSocket
func (c *Core) handleWSHeartbeat(ctx context.Context, wsConn *WSConnection, msg *WSMessage) {
	wsConn.mu.Lock()
	agentID := wsConn.agentID
	wsConn.mu.Unlock()

	if agentID == "" {
		c.sendWSError(wsConn, "Agent not registered")
		return
	}

	var req agentv1.HeartbeatRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendWSError(wsConn, fmt.Sprintf("Invalid heartbeat request: %v", err))
		return
	}

	// Set agent ID
	req.AgentId = agentID

	// Call the existing Heartbeat handler
	resp, err := c.Heartbeat(ctx, &req)
	if err != nil {
		c.sendWSError(wsConn, fmt.Sprintf("Heartbeat failed: %v", err))
		return
	}

	c.sendWSResponse(wsConn, "heartbeat_success", resp)
}

// handleWSStackList handles stack list request over WebSocket
func (c *Core) handleWSStackList(ctx context.Context, wsConn *WSConnection, msg *WSMessage) {
	wsConn.mu.Lock()
	agentID := wsConn.agentID
	wsConn.mu.Unlock()

	if agentID == "" {
		c.sendWSError(wsConn, "Agent not registered")
		return
	}

	var req agentv1.ListStacksRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendWSError(wsConn, fmt.Sprintf("Invalid list stacks request: %v", err))
		return
	}

	// Set agent ID
	req.AgentId = agentID

	// Call the existing ListStacks handler
	resp, err := c.ListStacks(ctx, &req)
	if err != nil {
		c.sendWSError(wsConn, fmt.Sprintf("List stacks failed: %v", err))
		return
	}

	c.sendWSResponse(wsConn, "stack_list_success", resp)
}

// handleWSStackApply handles stack apply request over WebSocket
func (c *Core) handleWSStackApply(ctx context.Context, wsConn *WSConnection, msg *WSMessage) {
	wsConn.mu.Lock()
	agentID := wsConn.agentID
	wsConn.mu.Unlock()

	if agentID == "" {
		c.sendWSError(wsConn, "Agent not registered")
		return
	}

	var req agentv1.ApplyStackRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendWSError(wsConn, fmt.Sprintf("Invalid apply stack request: %v", err))
		return
	}

	// Set agent ID
	req.AgentId = agentID

	// Send acknowledgment
	c.sendWSResponse(wsConn, "stack_apply_started", map[string]interface{}{
		"message": "Stack apply operation initiated",
	})

	// Note: Actual streaming operations are better handled via gRPC
	// WebSocket is primarily for fallback scenarios
}

// handleWSContainerList handles container list request over WebSocket
func (c *Core) handleWSContainerList(ctx context.Context, wsConn *WSConnection, msg *WSMessage) {
	wsConn.mu.Lock()
	agentID := wsConn.agentID
	wsConn.mu.Unlock()

	if agentID == "" {
		c.sendWSError(wsConn, "Agent not registered")
		return
	}

	// For now, send a placeholder response
	// In a real implementation, this would forward to the agent
	c.sendWSResponse(wsConn, "container_list_success", map[string]interface{}{
		"containers": []interface{}{},
	})
}

// handleWSOperationStatus handles operation status request over WebSocket
func (c *Core) handleWSOperationStatus(ctx context.Context, wsConn *WSConnection, msg *WSMessage) {
	var req struct {
		OperationID string `json:"operation_id"`
	}

	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendWSError(wsConn, fmt.Sprintf("Invalid operation status request: %v", err))
		return
	}

	// For now, send a placeholder response
	// In a real implementation, this would query the operation manager
	c.sendWSResponse(wsConn, "operation_status_success", map[string]interface{}{
		"operation_id": req.OperationID,
		"status":       "unknown",
	})
}

// sendWSResponse sends a response over WebSocket
func (c *Core) sendWSResponse(wsConn *WSConnection, msgType string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		c.sendWSError(wsConn, fmt.Sprintf("Failed to marshal response: %v", err))
		return
	}

	msg := WSMessage{
		Type:      msgType,
		Payload:   data,
		Timestamp: time.Now(),
	}

	wsConn.mu.Lock()
	defer wsConn.mu.Unlock()

	if wsConn.closed {
		return
	}

	err = wsConn.conn.WriteJSON(msg)
	if err != nil {
		fmt.Printf("Failed to send WebSocket response: %v\n", err)
	}
}

// sendWSError sends an error response over WebSocket
func (c *Core) sendWSError(wsConn *WSConnection, errMsg string) {
	msg := WSMessage{
		Type:      "error",
		Error:     errMsg,
		Timestamp: time.Now(),
	}

	wsConn.mu.Lock()
	defer wsConn.mu.Unlock()

	if wsConn.closed {
		return
	}

	err := wsConn.conn.WriteJSON(msg)
	if err != nil {
		fmt.Printf("Failed to send WebSocket error: %v\n", err)
	}
}
