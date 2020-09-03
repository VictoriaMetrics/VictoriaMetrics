package pacelimiter

import (
	"sync"
	"sync/atomic"
)

// PaceLimiter throttles WaitIfNeeded callers while the number of Inc calls is bigger than the number of Dec calls.
//
// It is expected that Inc is called before performing high-priority work,
// while Dec is called when the work is done.
// WaitIfNeeded must be called inside the work which must be throttled (i.e. lower-priority work).
// It may be called in the loop before performing a part of low-priority work.
type PaceLimiter struct {
	mu          sync.Mutex
	cond        *sync.Cond
	delaysTotal uint64
	n           int32
}

// New returns pace limiter that throttles WaitIfNeeded callers while the number of Inc calls is bigger than the number of Dec calls.
func New() *PaceLimiter {
	var pl PaceLimiter
	pl.cond = sync.NewCond(&pl.mu)
	return &pl
}

// Inc increments pl.
func (pl *PaceLimiter) Inc() {
	atomic.AddInt32(&pl.n, 1)
}

// Dec decrements pl.
func (pl *PaceLimiter) Dec() {
	if atomic.AddInt32(&pl.n, -1) == 0 {
		// Wake up all the goroutines blocked in WaitIfNeeded,
		// since the number of Dec calls equals the number of Inc calls.
		pl.cond.Broadcast()
	}
}

// WaitIfNeeded blocks while the number of Inc calls is bigger than the number of Dec calls.
func (pl *PaceLimiter) WaitIfNeeded() {
	if atomic.LoadInt32(&pl.n) <= 0 {
		// Fast path - there is no need in lock.
		return
	}
	// Slow path - wait until Dec is called.
	pl.mu.Lock()
	for atomic.LoadInt32(&pl.n) > 0 {
		pl.delaysTotal++
		pl.cond.Wait()
	}
	pl.mu.Unlock()
}

// DelaysTotal returns the number of delays inside WaitIfNeeded.
func (pl *PaceLimiter) DelaysTotal() uint64 {
	pl.mu.Lock()
	n := pl.delaysTotal
	pl.mu.Unlock()
	return n
}
