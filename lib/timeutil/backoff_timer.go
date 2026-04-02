package timeutil

import (
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
)

// BackoffTimer implements an exponential backoff timer with jitter.
type BackoffTimer struct {
	min     time.Duration
	max     time.Duration
	current time.Duration
}

// NewBackoffTimer returns a new BackoffTimer initialized with the given minDelay and maxDelay.
func NewBackoffTimer(minDelay, maxDelay time.Duration) *BackoffTimer {
	if maxDelay < minDelay {
		minDelay = maxDelay
	}
	return &BackoffTimer{
		min:     minDelay,
		max:     maxDelay,
		current: minDelay,
	}
}

// Wait sleeps for the current delay with jitter, doubling the delay for the next Wait.
// Use CurrentDelay to get the current backoff duration.
//
// Wait returns false if stopCh is closed.
func (bt *BackoffTimer) Wait(stopCh <-chan struct{}) bool {
	v := AddJitterToDuration(bt.current)
	bt.current *= 2
	if bt.current > bt.max {
		bt.current = bt.max
	}

	timer := timerpool.Get(v)
	defer timerpool.Put(timer)
	select {
	case <-stopCh:
		return false
	case <-timer.C:
		return true
	}
}

// CurrentDelay returns the current backoff duration.
func (bt *BackoffTimer) CurrentDelay() time.Duration {
	return bt.current
}

// SetDelay overrides the current delay. Useful for respecting Retry-After headers.
func (bt *BackoffTimer) SetDelay(d time.Duration) {
	if d < bt.min {
		d = bt.min
	}
	if d > bt.max {
		d = bt.max
	}
	bt.current = d
}

// Reset sets the backoff delay to its minimum.
func (bt *BackoffTimer) Reset() {
	bt.current = bt.min
}
