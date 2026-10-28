package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/httpx"
)

type bucket struct {
	count   int
	resetAt time.Time
}

// RateLimiter is a small in-memory fixed-window limiter for auth endpoints.
// Replace with Redis when running multiple instances.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]bucket
	limit   int
	window  time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 30
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{buckets: make(map[string]bucket), limit: limit, window: window}
}

func (l *RateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.buckets[key]
	if !ok || now.After(entry.resetAt) {
		l.buckets[key] = bucket{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	l.buckets[key] = entry
	return true
}

func RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow(ClientIP(r)) {
				httpx.WriteError(w, r, domain.TooManyRequests("rate_limited", "Bạn thao tác quá nhanh. Thử lại sau một phút."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
