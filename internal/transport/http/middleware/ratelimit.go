package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimiterConfig struct {
	// Requests per minute per user.
	Rate float64
	// Maximum burst size.
	Burst int
	// How often to sweep stale buckets. 0 disables cleanup.
	CleanupInterval time.Duration
	// Buckets idle longer than this are removed during cleanup.
	BucketTTL time.Duration
}

type tokenBucket struct {
	tokens    float64
	lastFill  time.Time
	maxTokens float64
	rate      float64 // tokens per second
}

type RateLimiter struct {
	cfg      RateLimiterConfig
	buckets  map[string]*tokenBucket
	mu       sync.Mutex
	stopOnce sync.Once
	stopCh   chan struct{}
}

func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	if cfg.Burst <= 0 {
		cfg.Burst = 10
	}
	if cfg.Rate <= 0 {
		cfg.Rate = 10 // 10 req/min default
	}
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}
	if cfg.BucketTTL == 0 {
		cfg.BucketTTL = 10 * time.Minute
	}

	rl := &RateLimiter{
		cfg:     cfg,
		buckets: make(map[string]*tokenBucket),
		stopCh:  make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stopCh) })
}

func (rl *RateLimiter) allow(key string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &tokenBucket{
			tokens:    float64(rl.cfg.Burst),
			lastFill:  now,
			maxTokens: float64(rl.cfg.Burst),
			rate:      rl.cfg.Rate / 60, // convert per-minute to per-second
		}
		rl.buckets[key] = b
	}

	// Refill tokens based on elapsed time.
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastFill = now

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}

	// Calculate wait time until next token.
	deficit := 1 - b.tokens
	wait := time.Duration(deficit/b.rate) * time.Second
	return false, wait
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cfg.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for k, b := range rl.buckets {
				if now.Sub(b.lastFill) > rl.cfg.BucketTTL {
					delete(rl.buckets, k)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// RateLimit returns middleware that limits requests per user (by user_id) or per IP.
func RateLimit(cfg RateLimiterConfig) gin.HandlerFunc {
	rl := NewRateLimiter(cfg)
	return func(c *gin.Context) {
		key := rateLimitKey(c)
		ok, retryAfter := rl.allow(key)
		if !ok {
			c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter.String(),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func rateLimitKey(c *gin.Context) string {
	if uid, exists := c.Get("user_id"); exists {
		if id, ok := uid.(uint64); ok && id > 0 {
			return "u:" + strconv.FormatUint(id, 10)
		}
	}
	return "ip:" + c.ClientIP()
}
