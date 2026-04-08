// Package auth provides JWT authentication middleware for the REST API
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Config holds JWT authentication configuration
type Config struct {
	// SecretKey is the key used to sign JWT tokens
	SecretKey []byte
	// TokenExpiry is how long tokens are valid
	TokenExpiry time.Duration
	// AdminUsername is the admin username for login
	AdminUsername string
	// AdminPassword is the admin password for login
	AdminPassword string
}

// Claims represents JWT claims
type Claims struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// Middleware provides JWT authentication middleware
type Middleware struct {
	config Config
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(config Config) *Middleware {
	return &Middleware{
		config: config,
	}
}

// RequireAuth wraps an HTTP handler with JWT authentication
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for login and health endpoints
		if r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			return
		}

		// Check for Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Parse and validate token
		claims, err := m.validateToken(tokenString)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid token: %v", err), http.StatusUnauthorized)
			return
		}

		// Add claims to request context
		ctx := r.Context()
		ctx = AddClaimsToContext(ctx, claims)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// GenerateToken generates a new JWT token for a user
func (m *Middleware) GenerateToken(username string) (string, error) {
	roles := []string{"user"}
	if username == m.config.AdminUsername {
		roles = []string{"admin", "user"}
	}

	claims := &Claims{
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.config.TokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "mandau",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.config.SecretKey)
}

// validateToken validates a JWT token and returns the claims
func (m *Middleware) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.config.SecretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Username  string    `json:"username"`
	Roles     []string  `json:"roles"`
}

// Authenticate authenticates a user and returns a JWT token
func (m *Middleware) Authenticate(username, password string) (*LoginResponse, error) {
	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(username), []byte(m.config.AdminUsername)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(m.config.AdminPassword)) != 1 {
		return nil, fmt.Errorf("invalid credentials")
	}

	token, err := m.GenerateToken(username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	roles := []string{"admin", "user"}
	
	return &LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(m.config.TokenExpiry),
		Username:  username,
		Roles:     roles,
	}, nil
}

// GenerateSecretKey generates a cryptographically random secret key for JWT signing
func GenerateSecretKey() ([]byte, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	return key, nil
}

// BasicAuthMiddleware provides basic authentication using Authorization header
func BasicAuthMiddleware(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health endpoint
			if r.URL.Path == "/api/v1/health" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("WWW-Authenticate", `Basic realm="Mandau API"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check Basic auth
			if !strings.HasPrefix(authHeader, "Basic ") {
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
			if err != nil {
				http.Error(w, "Invalid encoding", http.StatusUnauthorized)
				return
			}

			creds := strings.SplitN(string(decoded), ":", 2)
			if len(creds) != 2 {
				http.Error(w, "Invalid credentials format", http.StatusUnauthorized)
				return
			}

			if subtle.ConstantTimeCompare([]byte(creds[0]), []byte(username)) != 1 ||
				subtle.ConstantTimeCompare([]byte(creds[1]), []byte(password)) != 1 {
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
