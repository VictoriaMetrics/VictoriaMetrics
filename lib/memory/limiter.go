package memory

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// Limiter is the memory limiter.
//
// It limits memory to MaxSize.
type Limiter struct {
	// The maximum allowed memory
	MaxSize uint64

	mu    sync.Mutex
	usage uint64
}

// Get obtains n bytes of memory from ml.
//
// It returns true on success, false on error.
func (ml *Limiter) Get(n uint64) bool {
	ml.mu.Lock()
	ok := n <= ml.MaxSize && ml.MaxSize-n >= ml.usage
	if ok {
		ml.usage += n
	}
	ml.mu.Unlock()
	return ok
}

// Put returns back n bytes of memory to ml.
func (ml *Limiter) Put(n uint64) {
	ml.mu.Lock()
	if n > ml.usage {
		logger.Panicf("BUG: n=%d cannot exceed %d", n, ml.usage)
	}
	ml.usage -= n
	ml.mu.Unlock()
}
