// Package middleware provides HTTP middleware for the REST API
package middleware

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter provides token bucket rate limiting
type RateLimiter struct {
	tokens   map[string]*tokenBucket
	mu       sync.Mutex
	maxTokens int
	refillRate time.Duration
}

type tokenBucket struct {
	tokens     int
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxTokens int, refillRate time.Duration) *RateLimiter {
	return &RateLimiter{
		tokens:     make(map[string]*tokenBucket),
		maxTokens:  maxTokens,
		refillRate: refillRate,
	}
}

// StartAutoCleanup starts a background goroutine that periodically cleans up stale entries
func (rl *RateLimiter) StartAutoCleanup(interval time.Duration, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for range ticker.C {
			rl.Cleanup(maxAge)
		}
	}()
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.tokens[ip]
	if !exists {
		rl.tokens[ip] = &tokenBucket{
			tokens:     rl.maxTokens - 1, // Use one token for this request
			lastRefill: time.Now(),
		}
		return true
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)
	tokensToAdd := int(elapsed / rl.refillRate)
	if tokensToAdd > 0 {
		bucket.tokens += tokensToAdd
		if bucket.tokens > rl.maxTokens {
			bucket.tokens = rl.maxTokens
		}
		bucket.lastRefill = now
	}

	// Check if we have tokens available
	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// Cleanup removes stale entries older than the given duration
func (rl *RateLimiter) Cleanup(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, bucket := range rl.tokens {
		if now.Sub(bucket.lastRefill) > maxAge {
			delete(rl.tokens, ip)
		}
	}
}

// RateLimitMiddleware creates HTTP middleware for rate limiting
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP
			ip := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = forwarded
			}

			if !limiter.Allow(ip) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// DefaultLoginRateLimiter returns a rate limiter for login endpoints
func DefaultLoginRateLimiter() *RateLimiter {
	// 5 requests per minute per IP
	return NewRateLimiter(5, 12*time.Second)
}

// DefaultAPIRateLimiter returns a rate limiter for general API endpoints
func DefaultAPIRateLimiter() *RateLimiter {
	// 60 requests per minute per IP
	return NewRateLimiter(60, time.Second)
}
