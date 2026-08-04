package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type visitor struct {
	count   int
	expires time.Time
}

type RateLimiter struct {
	mu          sync.Mutex
	visitors    map[string]*visitor
	rate        int
	window      time.Duration
	nextCleanup time.Time
}

func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors:    make(map[string]*visitor),
		rate:        rate,
		window:      window,
		nextCleanup: time.Now().Add(window),
	}
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if !now.Before(rl.nextCleanup) {
		for key, item := range rl.visitors {
			if !now.Before(item.expires) {
				delete(rl.visitors, key)
			}
		}
		rl.nextCleanup = now.Add(rl.window)
	}

	v, exists := rl.visitors[ip]
	if !exists || !now.Before(v.expires) {
		rl.visitors[ip] = &visitor{count: 1, expires: now.Add(rl.window)}
		return true
	}

	if v.count >= rl.rate {
		return false
	}

	v.count++
	return true
}

func LimitRequests(rl *RateLimiter) gin.HandlerFunc {
	if rl == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		if !rl.Allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "too many requests, please try again later",
				"data":    nil,
			})
			return
		}
		c.Next()
	}
}
