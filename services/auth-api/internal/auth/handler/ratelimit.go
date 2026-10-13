package handler

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"restaurant-backend/internal/auth/interfaces"
	"restaurant-backend/internal/auth/service"
)

type window struct {
	count   int
	resetAt time.Time
}

// RateLimiter is a small in-memory fixed-window limiter keyed by client IP.
// It stops one machine hammering the login endpoint (even with many different
// emails). Per-account lockout lives in the service.
type RateLimiter struct {
	mu        sync.Mutex
	limit     int
	period    time.Duration
	clock     interfaces.Clock
	windows   map[string]*window
	lastSweep time.Time
}

func NewRateLimiter(limit int, period time.Duration, clock interfaces.Clock) *RateLimiter {
	if clock == nil {
		clock = service.SystemClock{}
	}
	return &RateLimiter{limit: limit, period: period, clock: clock, windows: map[string]*window{}}
}

// Allow reports whether the key may proceed, or how long until it may.
func (r *RateLimiter) Allow(key string) (bool, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.clock.Now()
	if now.Sub(r.lastSweep) > r.period {
		for k, w := range r.windows {
			if !now.Before(w.resetAt) {
				delete(r.windows, k)
			}
		}
		r.lastSweep = now
	}

	w, ok := r.windows[key]
	if !ok || !now.Before(w.resetAt) {
		r.windows[key] = &window{count: 1, resetAt: now.Add(r.period)}
		return true, 0
	}
	if w.count >= r.limit {
		return false, w.resetAt.Sub(now)
	}
	w.count++
	return true, 0
}

func (r *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, retry := r.Allow(c.ClientIP())
		if !ok {
			c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
			c.AbortWithStatusJSON(http.StatusTooManyRequests,
				errorBody("RATE_LIMITED", "Too many requests, slow down"))
			return
		}
		c.Next()
	}
}
