package promql

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type memoryLimiter struct {
	MaxSize uint64

	mu    sync.Mutex
	usage uint64
}

func (ml *memoryLimiter) Get(n uint64) bool {
	ml.mu.Lock()
	ok := n <= ml.MaxSize && ml.MaxSize-n >= ml.usage
	if ok {
		ml.usage += n
	}
	ml.mu.Unlock()
	return ok
}

func (ml *memoryLimiter) Put(n uint64) {
	ml.mu.Lock()
	if n > ml.usage {
		logger.Panicf("BUG: n=%d cannot exceed %d", n, ml.usage)
	}
	ml.usage -= n
	ml.mu.Unlock()
}
