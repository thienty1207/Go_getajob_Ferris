package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateBucket struct {
	started time.Time
	count   int
}

const maxRateLimiterBuckets = 10000

// rateLimiter is an in-process, per-IP guard for the current loopback API.
// It is intentionally not presented as a distributed production quota.
type rateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	lastCleanup time.Time
	buckets     map[string]rateBucket
}

// newRateLimiter creates a bounded fixed-window limiter for one route group.
func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, buckets: make(map[string]rateBucket)}
}

// allow atomically consumes one request from a key's current window and evicts
// stale keys when the in-memory map reaches its safety bound.
func (limiter *rateLimiter) allow(key string) bool {
	now := time.Now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if limiter.limit <= 0 || limiter.window <= 0 {
		return false
	}
	bucket, exists := limiter.buckets[key]
	if !exists || now.Sub(bucket.started) >= limiter.window {
		if !exists && len(limiter.buckets) >= maxRateLimiterBuckets {
			if limiter.lastCleanup.IsZero() || now.Sub(limiter.lastCleanup) >= cleanupInterval(limiter.window) {
				limiter.cleanupExpired(now)
				limiter.lastCleanup = now
			}
			if len(limiter.buckets) >= maxRateLimiterBuckets {
				limiter.evictOldest()
			}
		}
		limiter.buckets[key] = rateBucket{started: now, count: 1}
		return true
	}
	if bucket.count >= limiter.limit {
		return false
	}
	bucket.count++
	limiter.buckets[key] = bucket
	return true
}

func (limiter *rateLimiter) cleanupExpired(now time.Time) {
	for bucketKey, bucket := range limiter.buckets {
		if now.Sub(bucket.started) >= limiter.window {
			delete(limiter.buckets, bucketKey)
		}
	}
}

func (limiter *rateLimiter) evictOldest() {
	var (
		oldestKey string
		oldestAt  time.Time
	)
	for key, bucket := range limiter.buckets {
		if oldestKey == "" || bucket.started.Before(oldestAt) {
			oldestKey = key
			oldestAt = bucket.started
		}
	}
	if oldestKey != "" {
		delete(limiter.buckets, oldestKey)
	}
}

func cleanupInterval(window time.Duration) time.Duration {
	if window < time.Second {
		return window
	}
	return time.Second
}

// rateLimitMiddleware translates the limiter decision into a stable 429 API
// error without exposing bucket internals.
func rateLimitMiddleware(limiter *rateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.allow(requestIP(c.Request)) {
			writeError(c, http.StatusTooManyRequests, "rate_limited", "Có quá nhiều yêu cầu. Vui lòng thử lại sau.")
			return
		}
		c.Next()
	}
}

// corsMiddleware uses an explicit allowlist and handles preflight requests
// before they reach application handlers. Wildcards are rejected in config.
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[strings.TrimSpace(origin)] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		_, isAllowed := allowed[origin]
		if origin != "" && isAllowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			if origin != "" && !isAllowed {
				writeError(c, http.StatusForbidden, "cors_origin_not_allowed", "Origin không được phép.")
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// requestIP uses the transport peer address; it intentionally does not trust
// spoofable X-Forwarded-For headers while the API is loopback-only.
func requestIP(request *http.Request) string {
	if host, _, err := net.SplitHostPort(request.RemoteAddr); err == nil && host != "" {
		return host
	}
	if request.RemoteAddr == "" {
		return "unknown"
	}
	return request.RemoteAddr
}
