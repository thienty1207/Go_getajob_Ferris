package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiterAllowsConfiguredRequestsThenReturnsFalse(t *testing.T) {
	limiter := newRateLimiter(2, time.Minute)
	if !limiter.allow("203.0.113.10") || !limiter.allow("203.0.113.10") {
		t.Fatal("first two requests should be allowed")
	}
	if limiter.allow("203.0.113.10") {
		t.Fatal("third request in the window should be rejected")
	}
	if !limiter.allow("203.0.113.11") {
		t.Fatal("a different source IP should have its own window")
	}
}

func newTestRateRouter(limiter *rateLimiter) *gin.Engine {
	router := gin.New()
	router.POST("/", rateLimitMiddleware(limiter), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router
}

func TestRateLimitMiddlewareReturnsStable429(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)
	router := newTestRateRouter(limiter)

	for index := 0; index < 2; index++ {
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		request.RemoteAddr = "203.0.113.20:4321"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if index == 0 && response.Code != http.StatusNoContent {
			t.Fatalf("first status = %d, want 204", response.Code)
		}
		if index == 1 && response.Code != http.StatusTooManyRequests {
			t.Fatalf("second status = %d, want 429", response.Code)
		}
	}
}

func TestRateLimiterCapsActiveSourceKeys(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)
	for index := 0; index < maxRateLimiterBuckets+100; index++ {
		limiter.allow(fmt.Sprintf("198.51.100.%d", index))
	}
	if got := len(limiter.buckets); got > maxRateLimiterBuckets {
		t.Fatalf("bucket count = %d, want at most %d", got, maxRateLimiterBuckets)
	}
}

func TestRateLimiterAdmitsNewSourceByEvictingOldestBucket(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)
	for index := 0; index < maxRateLimiterBuckets; index++ {
		limiter.allow(fmt.Sprintf("203.0.113.%d", index))
	}
	if !limiter.allow("new-legitimate-source") {
		t.Fatal("new source should be admitted by bounded eviction")
	}
	if got := len(limiter.buckets); got > maxRateLimiterBuckets {
		t.Fatalf("bucket count = %d, want at most %d", got, maxRateLimiterBuckets)
	}
}
