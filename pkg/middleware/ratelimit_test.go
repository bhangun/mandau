package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(5, time.Second)
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.maxTokens != 5 {
		t.Errorf("expected maxTokens 5, got %d", rl.maxTokens)
	}
}

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !rl.Allow("127.0.0.1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if rl.Allow("127.0.0.1") {
		t.Error("4th request should be denied")
	}
}

func TestRateLimiterDifferentIPs(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)

	// Each IP should have its own bucket
	if !rl.Allow("127.0.0.1") {
		t.Error("first IP should be allowed")
	}

	if !rl.Allow("127.0.0.2") {
		t.Error("second IP should be allowed")
	}

	// Both should be denied now
	if rl.Allow("127.0.0.1") {
		t.Error("first IP should be denied")
	}
	if rl.Allow("127.0.0.2") {
		t.Error("second IP should be denied")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(2, 100*time.Millisecond)

	// Use all tokens
	rl.Allow("127.0.0.1")
	rl.Allow("127.0.0.1")

	// Should be denied
	if rl.Allow("127.0.0.1") {
		t.Error("should be denied")
	}

	// Wait for refill
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	if !rl.Allow("127.0.0.1") {
		t.Error("should be allowed after refill")
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)

	// Add some entries
	rl.Allow("127.0.0.1")
	rl.Allow("127.0.0.2")
	rl.Allow("127.0.0.3")

	if len(rl.tokens) != 3 {
		t.Errorf("expected 3 tokens, got %d", len(rl.tokens))
	}

	// Cleanup with very short max age
	rl.Cleanup(1 * time.Nanosecond)

	if len(rl.tokens) != 0 {
		t.Errorf("expected 0 tokens after cleanup, got %d", len(rl.tokens))
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		rl := NewRateLimiter(2, time.Minute)
		mw := RateLimitMiddleware(rl)
		
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := mw(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("denies requests over limit", func(t *testing.T) {
		rl := NewRateLimiter(2, time.Minute)
		mw := RateLimitMiddleware(rl)
		
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := mw(handler)

		// Use all tokens for this IP (middleware extracts IP from RemoteAddr)
		rl.Allow("10.0.0.1:12345")
		rl.Allow("10.0.0.1:12345")

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusTooManyRequests {
			t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rr.Code)
		}
	})

	t.Run("uses X-Forwarded-For header", func(t *testing.T) {
		rl := NewRateLimiter(2, time.Minute)
		mw := RateLimitMiddleware(rl)
		
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := mw(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.2")
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

func TestDefaultLimiters(t *testing.T) {
	loginLimiter := DefaultLoginRateLimiter()
	if loginLimiter.maxTokens != 5 {
		t.Errorf("expected login maxTokens 5, got %d", loginLimiter.maxTokens)
	}

	apiLimiter := DefaultAPIRateLimiter()
	if apiLimiter.maxTokens != 60 {
		t.Errorf("expected API maxTokens 60, got %d", apiLimiter.maxTokens)
	}
}
