package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateLimiter is a simple per-key token bucket rate limiter.
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     int           // max requests per window
	window   time.Duration // time window
	done     chan struct{}
}

type visitor struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(rate int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		window:   window,
		done:     make(chan struct{}),
	}
	registerRateLimiter(rl)
	go rl.cleanupLoop()
	return rl
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[key]
	if !exists || now.After(v.resetAt) {
		rl.visitors[key] = &visitor{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	v.count++
	return v.count <= rl.rate
}

func (rl *rateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rl.done:
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, v := range rl.visitors {
				if now.After(v.resetAt) {
					delete(rl.visitors, key)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// Global registry for graceful shutdown.
var (
	limiters   []*rateLimiter
	limitersMu sync.Mutex
)

func registerRateLimiter(rl *rateLimiter) {
	limitersMu.Lock()
	limiters = append(limiters, rl)
	limitersMu.Unlock()
}

// ShutdownRateLimiters stops all rate limiter cleanup goroutines.
// Call during graceful shutdown to prevent goroutine leaks.
func ShutdownRateLimiters() {
	limitersMu.Lock()
	defer limitersMu.Unlock()
	for _, rl := range limiters {
		close(rl.done)
	}
}

// RateLimitByTokenMiddleware limits requests by Bearer token (falls back to IP).
// rate: max requests allowed in the given window per token.
func RateLimitByTokenMiddleware(rate int, window time.Duration) gin.HandlerFunc {
	limiter := newRateLimiter(rate, window)
	return func(c *gin.Context) {
		key := c.ClientIP()
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			key = "token:" + strings.TrimPrefix(auth, "Bearer ")
		}

		if !limiter.allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	}
}
