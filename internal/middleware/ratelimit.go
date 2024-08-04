package middleware

import (
	"net/http"
	"sync"
	"time"

	"tonkey/internal/logger"

	"golang.org/x/time/rate"
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rps      rate.Limit
	burst    int
	cleanup  *time.Ticker
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rps int, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		cleanup:  time.NewTicker(time.Minute),
	}

	// Cleanup old limiters every minute
	go rl.cleanupRoutine()

	return rl
}

// getLimiter gets or creates a limiter for a key
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(rl.rps, rl.burst)
		rl.limiters[key] = limiter
	}

	return limiter
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow(key string) bool {
	limiter := rl.getLimiter(key)
	return limiter.Allow()
}

// cleanupRoutine removes inactive limiters
func (rl *RateLimiter) cleanupRoutine() {
	for range rl.cleanup.C {
		rl.mu.Lock()
		for key, limiter := range rl.limiters {
			// Remove limiters with full buckets (inactive)
			if limiter.Tokens() == float64(rl.burst) {
				delete(rl.limiters, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Stop stops the cleanup routine
func (rl *RateLimiter) Stop() {
	rl.cleanup.Stop()
}

// RateLimitMiddleware creates HTTP middleware for rate limiting
func RateLimitMiddleware(rl *RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			if !rl.Allow(key) {
				logger.Warn.Printf("Rate limit exceeded for key: %s", key)
				w.Header().Set("Retry-After", "60")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IPKeyFunc extracts IP address as rate limit key
func IPKeyFunc(r *http.Request) string {
	// Check X-Forwarded-For header
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}
	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// UserKeyFunc extracts user from context as rate limit key
func UserKeyFunc(r *http.Request) string {
	// Try to get user from context
	if user := r.Context().Value("auth"); user != nil {
		if authCtx, ok := user.(interface{ GetUser() string }); ok {
			return "user:" + authCtx.GetUser()
		}
	}
	// Fall back to IP
	return "ip:" + IPKeyFunc(r)
}
