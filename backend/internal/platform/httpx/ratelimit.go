package httpx

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	mu       sync.Mutex
	every    time.Duration
	burst    int
	visitors map[string]*visitor
	lastGC   time.Time
}

type visitor struct {
	limiter *rate.Limiter
	seen    time.Time
}

func NewRateLimiter(every time.Duration, burst int) *RateLimiter {
	return &RateLimiter{
		every:    every,
		burst:    burst,
		visitors: make(map[string]*visitor),
		lastGC:   time.Now(),
	}
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		r := rl.reserve(c.ClientIP())
		if !r.OK() {
			tooMany(c, rl.every)
			return
		}
		if delay := r.Delay(); delay > 0 {
			r.Cancel()
			tooMany(c, delay)
			return
		}
		c.Next()
	}
}

func (rl *RateLimiter) reserve(key string) *rate.Reservation {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.lastGC) > time.Minute {
		ttl := rl.every * time.Duration(rl.burst+1)
		for k, v := range rl.visitors {
			if now.Sub(v.seen) > ttl {
				delete(rl.visitors, k)
			}
		}
		rl.lastGC = now
	}

	v, ok := rl.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rate.Every(rl.every), rl.burst)}
		rl.visitors[key] = v
	}
	v.seen = now
	return v.limiter.ReserveN(now, 1)
}

func tooMany(c *gin.Context, retry time.Duration) {
	c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
	Error(c, http.StatusTooManyRequests, "rate_limited", "Too many requests, please try again later", nil)
}
