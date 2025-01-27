package netstorage

import (
	"fmt"
	"sync"
)

var (
	maxMemoryUsagePerQuery int64
)

// getMaxMemoryUsagePerQuery returns the maximum memory usage per query.
func getMaxMemoryUsagePerQuery() int64 {
	return maxMemoryUsagePerQuery
}

// SetMaxMemoryUsagePerQuery sets the maximum memory usage per query.
func SetMaxMemoryUsagePerQuery(v int64) {
	maxMemoryUsagePerQuery = v
}

// memoryLimiter tracks and limits memory usage for operations
type memoryLimiter struct {
	maxSize uint64

	mu    sync.Mutex
	usage uint64
}

func newMemoryLimiter() *memoryLimiter {
	maxSize := uint64(getMaxMemoryUsagePerQuery())
	return &memoryLimiter{
		maxSize: maxSize,
	}
}

func (ml *memoryLimiter) Get(n uint64) error {
	if ml.maxSize <= 0 {
		return nil
	}

	ml.mu.Lock()
	ok := n <= ml.maxSize && ml.maxSize-n >= ml.usage
	if ok {
		ml.usage += n
	}
	ml.mu.Unlock()
	if !ok {
		return &limitExceededErr{
			err: fmt.Errorf("cannot allocate %d bytes; max allowed memory usage is %d bytes", n, ml.maxSize),
		}
	}

	return nil
}

// GetStringSlice accounts for and limits memory usage of string slices
func (ml *memoryLimiter) GetStringSlice(v []string) error {
	if len(v) == 0 {
		return nil
	}

	// Account for:
	// - Slice header (24 bytes)
	// - String headers (16 bytes each)
	// - String data
	overhead := uint64(24 + (16 * len(v)))
	dataSize := uint64(0)
	for _, s := range v {
		dataSize += uint64(len(s))
	}
	return ml.Get(overhead + dataSize)
}
