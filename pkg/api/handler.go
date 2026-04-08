// Package api provides REST API handlers for the web dashboard
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bhangun/mandau/pkg/auth"
	"github.com/bhangun/mandau/pkg/middleware"
)

// CoreInterface defines the interface for core operations
type CoreInterface interface {
	ListAgentsJSON() (interface{}, error)
}

// Handler creates REST API handlers for the web dashboard
type Handler struct {
	core        CoreInterface
	authMW      *auth.Middleware
	loginLimiter *middleware.RateLimiter
}

// NewHandler creates a new API handler
func NewHandler(c CoreInterface, authMW *auth.Middleware) *Handler {
	loginLimiter := middleware.DefaultLoginRateLimiter()
	loginLimiter.StartAutoCleanup(5*time.Minute, 10*time.Minute)
	
	apiLimiter := middleware.DefaultAPIRateLimiter()
	apiLimiter.StartAutoCleanup(5*time.Minute, 10*time.Minute)
	
	return &Handler{
		core:        c,
		authMW:      authMW,
		loginLimiter: loginLimiter,
	}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Auth endpoints (no auth required)
	mux.HandleFunc("/api/v1/auth/login", h.handleLogin)

	// Health (no auth required)
	mux.HandleFunc("/api/v1/health", h.handleHealth)

	// Agents
	mux.HandleFunc("/api/v1/agents", h.authWrapper(h.handleAgents))
	mux.HandleFunc("/api/v1/agents/", h.authWrapper(h.handleAgentDetail))

	// Stacks
	mux.HandleFunc("/api/v1/stacks", h.authWrapper(h.handleStacks))
	mux.HandleFunc("/api/v1/stacks/", h.authWrapper(h.handleStackDetail))

	// Containers
	mux.HandleFunc("/api/v1/containers", h.authWrapper(h.handleContainers))

	// Operations
	mux.HandleFunc("/api/v1/operations", h.authWrapper(h.handleOperations))
	mux.HandleFunc("/api/v1/operations/", h.authWrapper(h.handleOperationDetail))

	// Logs
	mux.HandleFunc("/api/v1/logs", h.authWrapper(h.handleLogs))
}

// authWrapper wraps a handler with authentication if auth middleware is configured
func (h *Handler) authWrapper(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.authMW != nil {
			h.authMW.RequireAuth(handler).ServeHTTP(w, r)
		} else {
			handler.ServeHTTP(w, r)
		}
	}
}

// handleLogin handles POST /api/v1/auth/login
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Rate limit login attempts
	ip := r.RemoteAddr
	if !h.loginLimiter.Allow(ip) {
		writeError(w, http.StatusTooManyRequests, "Too many login attempts. Please try again later.")
		return
	}

	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if h.authMW == nil {
		writeError(w, http.StatusServiceUnavailable, "Authentication not configured")
		return
	}

	resp, err := h.authMW.Authenticate(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    resp,
	})
}

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, errMsg string) {
	writeJSON(w, status, Response{
		Success: false,
		Error:   errMsg,
	})
}

// handleAgents handles GET /api/v1/agents
func (h *Handler) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get agents from core
	agents, err := h.core.ListAgentsJSON()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list agents: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    agents,
	})
}

// handleAgentDetail handles GET/DELETE /api/v1/agents/{id}
func (h *Handler) handleAgentDetail(w http.ResponseWriter, r *http.Request) {
	// Extract agent ID from URL
	// For now, this is a placeholder
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]string{"message": "Agent detail endpoint"},
	})
}

// handleStacks handles GET/POST /api/v1/stacks
func (h *Handler) handleStacks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// List all stacks across agents
		// This would iterate through all agents and collect their stacks
		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    []interface{}{}, // Would populate from agents
		})
	case http.MethodPost:
		// Deploy a new stack
		var req struct {
			AgentID       string            `json:"agent_id"`
			StackName     string            `json:"stack_name"`
			ComposeContent string           `json:"compose_content"`
			EnvVars       map[string]string `json:"env_vars,omitempty"`
			ForceRecreate bool              `json:"force_recreate,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
			return
		}

		if req.AgentID == "" || req.StackName == "" || req.ComposeContent == "" {
			writeError(w, http.StatusBadRequest, "agent_id, stack_name, and compose_content are required")
			return
		}

		writeJSON(w, http.StatusAccepted, Response{
			Success: true,
			Data: map[string]interface{}{
				"message":    "Stack deployment initiated",
				"agent_id":   req.AgentID,
				"stack_name": req.StackName,
			},
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleStackDetail handles GET/DELETE /api/v1/stacks/{id}
func (h *Handler) handleStackDetail(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    map[string]string{"message": "Stack detail"},
		})
	case http.MethodDelete:
		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    map[string]string{"message": "Stack removal initiated"},
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleContainers handles GET /api/v1/containers
func (h *Handler) handleContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    []interface{}{},
	})
}

// handleOperations handles GET /api/v1/operations
func (h *Handler) handleOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    []interface{}{},
	})
}

// handleOperationDetail handles GET /api/v1/operations/{id}
func (h *Handler) handleOperationDetail(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]string{"message": "Operation detail"},
	})
}

// handleLogs handles GET /api/v1/logs
func (h *Handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    []interface{}{},
	})
}

// handleHealth handles GET /api/v1/health
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Get agent count from core
	agents, err := h.core.ListAgentsJSON()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get system status")
		return
	}
	
	agentList, _ := agents.([]map[string]interface{})
	onlineCount := 0
	for _, agent := range agentList {
		if status, ok := agent["status"].(string); ok && status == "online" {
			onlineCount++
		}
	}
	
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"status":         "healthy",
			"version":        "0.0.16",
			"agents_online":  onlineCount,
			"agents_total":   len(agentList),
			"uptime":         time.Now().Unix(),
		},
	})
}
