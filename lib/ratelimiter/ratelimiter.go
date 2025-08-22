package ratelimiter

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

// RateLimiter limits per-second rate of arbitrary resources.
//
// Call Register() for registering the given amounts of resources.
type RateLimiter struct {
	// perSecondLimit is the per-second limit of resources.
	perSecondLimit int64

	// stopCh is used for unblocking rate limiting.
	stopCh <-chan struct{}

	// mu protects budget and deadline from concurrent access.
	mu sync.Mutex

	// The current budget. It is increased by perSecondLimit every second.
	budget int64

	// The next deadline for increasing the budget by perSecondLimit.
	deadline time.Time

	// limitReached is a counter, which is increased every time the limit is reached.
	limitReached *metrics.Counter
}

// New creates new rate limiter with the given perSecondLimit.
//
// stopCh is used for unblocking Register() calls when the rate limiter is no longer needed.
func New(perSecondLimit int64, limitReached *metrics.Counter, stopCh <-chan struct{}) *RateLimiter {
	return &RateLimiter{
		perSecondLimit: perSecondLimit,
		stopCh:         stopCh,
		limitReached:   limitReached,
	}
}

// Register registers count resources.
//
// Register blocks if the given per-second rate limit is exceeded.
// It may be forcibly unblocked by closing stopCh passed to New().
func (rl *RateLimiter) Register(count int) {
	if rl == nil {
		return
	}

	limit := rl.perSecondLimit
	if limit <= 0 {
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	for rl.budget <= 0 {
		if d := time.Until(rl.deadline); d > 0 {
			rl.limitReached.Inc()
			t := timerpool.Get(d)
			select {
			case <-rl.stopCh:
				timerpool.Put(t)
				return
			case <-t.C:
				timerpool.Put(t)
			}
		}
		rl.budget += limit
		rl.deadline = time.Now().Add(time.Second)
	}
	rl.budget -= int64(count)
}
