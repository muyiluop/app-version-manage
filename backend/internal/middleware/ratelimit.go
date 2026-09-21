package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"app_version_manage/internal/apierr"
)

// RateLimiter 基于令牌桶的按 key 限流器。
type RateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*entry
	limit    rate.Limit
	burst    int
	lastGC   time.Time
}

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter 构造限流器，perMinute<=0 表示不限流。
func NewRateLimiter(perMinute int) *RateLimiter {
	if perMinute <= 0 {
		return nil
	}
	return &RateLimiter{
		entries: make(map[string]*entry),
		// 均摊到秒，burst 取每分钟额度，允许短暂突发。
		limit:  rate.Limit(float64(perMinute) / 60.0),
		burst:  perMinute,
		lastGC: time.Now(),
	}
}

// Allow 判断某个 key 是否允许通过。
func (l *RateLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastGC) > 10*time.Minute {
		for k, e := range l.entries {
			if now.Sub(e.lastSeen) > 30*time.Minute {
				delete(l.entries, k)
			}
		}
		l.lastGC = now
	}

	e, ok := l.entries[key]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.entries[key] = e
	}
	e.lastSeen = now
	return e.limiter.Allow()
}

// RateLimit 限流中间件，keyFn 决定限流维度（默认按客户端 IP）。
func RateLimit(limiter *RateLimiter, keyFn func(*gin.Context) string) gin.HandlerFunc {
	if limiter == nil {
		return func(c *gin.Context) { c.Next() }
	}
	if keyFn == nil {
		keyFn = func(c *gin.Context) string { return c.ClientIP() }
	}

	return func(c *gin.Context) {
		if !limiter.Allow(keyFn(c)) {
			c.Header("Retry-After", "60")
			apierr.FailRaw(c, apierr.CodeTooManyRequests, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
			return
		}
		c.Next()
	}
}
