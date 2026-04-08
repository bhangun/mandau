package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewMiddleware(t *testing.T) {
	mw := NewMiddleware(Config{
		SecretKey:     []byte("test-secret-key"),
		TokenExpiry:   time.Hour,
		AdminUsername: "admin",
		AdminPassword: "password",
	})

	if mw == nil {
		t.Fatal("NewMiddleware returned nil")
	}
	if mw.config.TokenExpiry != time.Hour {
		t.Errorf("expected token expiry %v, got %v", time.Hour, mw.config.TokenExpiry)
	}
}

func TestAuthenticate(t *testing.T) {
	mw := NewMiddleware(Config{
		SecretKey:     []byte("test-secret"),
		TokenExpiry:   time.Hour,
		AdminUsername: "admin",
		AdminPassword: "password",
	})

	t.Run("valid credentials", func(t *testing.T) {
		resp, err := mw.Authenticate("admin", "password")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Token == "" {
			t.Error("expected token, got empty string")
		}
		if resp.Username != "admin" {
			t.Errorf("expected username admin, got %s", resp.Username)
		}
		if len(resp.Roles) != 2 {
			t.Errorf("expected 2 roles, got %d", len(resp.Roles))
		}
	})

	t.Run("invalid username", func(t *testing.T) {
		_, err := mw.Authenticate("wrong", "password")
		if err == nil {
			t.Error("expected error for invalid username")
		}
	})

	t.Run("invalid password", func(t *testing.T) {
		_, err := mw.Authenticate("admin", "wrong")
		if err == nil {
			t.Error("expected error for invalid password")
		}
	})
}

func TestGenerateToken(t *testing.T) {
	mw := NewMiddleware(Config{
		SecretKey:     []byte("test-secret"),
		TokenExpiry:   time.Hour,
		AdminUsername: "admin",
		AdminPassword: "password",
	})

	token, err := mw.GenerateToken("admin")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if token == "" {
		t.Error("generated empty token")
	}

	// Verify token can be parsed
	claims, err := mw.validateToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}
	if claims.Username != "admin" {
		t.Errorf("expected username admin, got %s", claims.Username)
	}
}

func TestValidateToken(t *testing.T) {
	mw := NewMiddleware(Config{
		SecretKey:     []byte("test-secret"),
		TokenExpiry:   time.Hour,
		AdminUsername: "admin",
		AdminPassword: "password",
	})

	t.Run("valid token", func(t *testing.T) {
		token, _ := mw.GenerateToken("admin")
		claims, err := mw.validateToken(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if claims.Username != "admin" {
			t.Errorf("expected username admin, got %s", claims.Username)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := mw.validateToken("invalid-token")
		if err == nil {
			t.Error("expected error for invalid token")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		// Create middleware with very short expiry
		shortMw := NewMiddleware(Config{
			SecretKey:     []byte("test-secret"),
			TokenExpiry:   -time.Hour, // Already expired
			AdminUsername: "admin",
			AdminPassword: "password",
		})

		token, _ := shortMw.GenerateToken("admin")
		_, err := mw.validateToken(token)
		if err == nil {
			t.Error("expected error for expired token")
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		token, _ := mw.GenerateToken("admin")
		
		wrongMw := NewMiddleware(Config{
			SecretKey:     []byte("wrong-secret"),
			TokenExpiry:   time.Hour,
			AdminUsername: "admin",
			AdminPassword: "password",
		})

		_, err := wrongMw.validateToken(token)
		if err == nil {
			t.Error("expected error for token signed with wrong secret")
		}
	})
}

func TestGenerateSecretKey(t *testing.T) {
	key1, err := GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate secret key: %v", err)
	}
	if len(key1) != 32 {
		t.Errorf("expected key length 32, got %d", len(key1))
	}

	// Verify keys are random
	key2, _ := GenerateSecretKey()
	if string(key1) == string(key2) {
		t.Error("generated identical random keys")
	}
}

func TestRequireAuth(t *testing.T) {
	mw := NewMiddleware(Config{
		SecretKey:     []byte("test-secret"),
		TokenExpiry:   time.Hour,
		AdminUsername: "admin",
		AdminPassword: "password",
	})

	t.Run("skip auth for health endpoint", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := mw.RequireAuth(handler)
		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("skip auth for login endpoint", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := mw.RequireAuth(handler)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("missing auth header", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := mw.RequireAuth(handler)
		req := httptest.NewRequest("GET", "/api/v1/agents", nil)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("valid auth header", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		token, _ := mw.GenerateToken("admin")
		wrapped := mw.RequireAuth(handler)
		req := httptest.NewRequest("GET", "/api/v1/agents", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

func TestContextHelpers(t *testing.T) {
	t.Run("add and get claims", func(t *testing.T) {
		claims := &Claims{
			Username: "testuser",
			Roles:    []string{"admin", "user"},
		}

		ctx := AddClaimsToContext(context.Background(), claims)
		
		retrieved := GetClaimsFromContext(ctx)
		if retrieved == nil {
			t.Fatal("failed to retrieve claims from context")
		}
		if retrieved.Username != "testuser" {
			t.Errorf("expected username testuser, got %s", retrieved.Username)
		}
	})

	t.Run("get username from context", func(t *testing.T) {
		claims := &Claims{Username: "testuser"}
		ctx := AddClaimsToContext(context.Background(), claims)

		username := GetUsernameFromContext(ctx)
		if username != "testuser" {
			t.Errorf("expected username testuser, got %s", username)
		}
	})

	t.Run("has role in context", func(t *testing.T) {
		claims := &Claims{
			Username: "admin",
			Roles:    []string{"admin", "user"},
		}
		ctx := AddClaimsToContext(context.Background(), claims)

		if !HasRoleInContext(ctx, "admin") {
			t.Error("expected user to have admin role")
		}
		if !HasRoleInContext(ctx, "user") {
			t.Error("expected user to have user role")
		}
		if HasRoleInContext(ctx, "superadmin") {
			t.Error("expected user to not have superadmin role")
		}
	})

	t.Run("missing claims", func(t *testing.T) {
		ctx := context.Background()
		
		claims := GetClaimsFromContext(ctx)
		if claims != nil {
			t.Error("expected nil claims for empty context")
		}

		username := GetUsernameFromContext(ctx)
		if username != "" {
			t.Errorf("expected empty username, got %s", username)
		}

		if HasRoleInContext(ctx, "admin") {
			t.Error("expected no role for empty context")
		}
	})
}

func TestRequireRole(t *testing.T) {
	mw := NewMiddleware(Config{
		SecretKey:     []byte("test-secret"),
		TokenExpiry:   time.Hour,
		AdminUsername: "admin",
		AdminPassword: "password",
	})

	t.Run("user has required role", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		token, _ := mw.GenerateToken("admin")
		req := httptest.NewRequest("GET", "/api/v1/agents", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		wrapped := mw.RequireAuth(RequireRole("admin")(handler))
		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("user lacks required role", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Create regular user token
		claims := &Claims{
			Username: "user",
			Roles:    []string{"user"},
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := token.SignedString([]byte("test-secret"))

		req := httptest.NewRequest("GET", "/api/v1/admin", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		rr := httptest.NewRecorder()

		wrapped := mw.RequireAuth(RequireRole("admin")(handler))
		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})
}

func TestBasicAuthMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	basicAuth := BasicAuthMiddleware("admin", "password")
	wrapped := basicAuth(handler)

	t.Run("valid basic auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/agents", nil)
		req.SetBasicAuth("admin", "password")
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("invalid basic auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/agents", nil)
		req.SetBasicAuth("admin", "wrong")
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("skip health endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}
