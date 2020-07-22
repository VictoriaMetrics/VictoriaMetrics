package pacelimiter

import (
	"sync"
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
	n           int
}

// New returns pace limiter that throttles WaitIfNeeded callers while the number of Inc calls is bigger than the number of Dec calls.
func New() *PaceLimiter {
	var pl PaceLimiter
	pl.cond = sync.NewCond(&pl.mu)
	return &pl
}

// Inc increments pl.
func (pl *PaceLimiter) Inc() {
	pl.mu.Lock()
	pl.n++
	pl.mu.Unlock()
}

// Dec decrements pl.
func (pl *PaceLimiter) Dec() {
	pl.mu.Lock()
	pl.n--
	if pl.n == 0 {
		// Wake up all the goroutines blocked in WaitIfNeeded,
		// since the number of Dec calls equals the number of Inc calls.
		pl.cond.Broadcast()
	}
	pl.mu.Unlock()
}

// WaitIfNeeded blocks while the number of Inc calls is bigger than the number of Dec calls.
func (pl *PaceLimiter) WaitIfNeeded() {
	pl.mu.Lock()
	for pl.n > 0 {
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
