package limiter

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
)

// NewLimiter creates a Limiter object
// for the given perSecondLimit
func NewLimiter(perSecondLimit int64) *Limiter {
	return &Limiter{perSecondLimit: perSecondLimit}
}

// Limiter controls the amount of budget
// that can be spent according to configured perSecondLimit
type Limiter struct {
	perSecondLimit int64

	// mu protects budget and deadline from concurrent access.
	mu sync.Mutex

	// The current budget. It is increased by perSecondLimit every second.
	budget int64

	// The next deadline for increasing the budget by perSecondLimit
	deadline time.Time
}

// Register blocks for amount of time
// needed to process the given dataLen according
// to the configured perSecondLimit.
func (l *Limiter) Register(dataLen int) {
	limit := l.perSecondLimit
	if limit <= 0 {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	for l.budget <= 0 {
		if d := time.Until(l.deadline); d > 0 {
			t := timerpool.Get(d)
			<-t.C
			timerpool.Put(t)
		}
		l.budget += limit
		l.deadline = time.Now().Add(time.Second)
	}
	l.budget -= int64(dataLen)
}
