package httpserver

import (
	"errors"
	"sync/atomic"
	"time"
)

type Limiter struct {
	sem         chan struct{}
	waitTimeout time.Duration

	maxPendingRequests     int32
	currentPendingRequests int32
}

func (l *Limiter) MaxPendingRequests() int32 {
	return l.maxPendingRequests
}

func (l *Limiter) CurrentPendingRequests() int32 {
	return l.currentPendingRequests
}

func (l *Limiter) CurrentConcurrentRequests() int {
	return len(l.sem)
}

func (l *Limiter) MaxConcurrentRequests() int {
	return len(l.sem)
}

func NewLimiter(maxConcurrentRequests, maxPendingRequests int) *Limiter {
	wait := *connTimeout - time.Second
	if wait < 0 {
		wait = 0
	}

	return &Limiter{
		sem:                make(chan struct{}, maxConcurrentRequests),
		waitTimeout:        wait,
		maxPendingRequests: int32(maxPendingRequests),
	}
}

func (l *Limiter) Acquire() error {
	// fast path: try acquire without pending
	select {
	case l.sem <- struct{}{}:
		return nil
	default:
		// no slot, go to pending queue
	}

	pending := atomic.AddInt32(&l.currentPendingRequests, 1)
	if pending > l.maxPendingRequests {
		atomic.AddInt32(&l.currentPendingRequests, -1)
		return errors.New("too many pending requests (queue full)")
	}

	timer := time.NewTimer(l.waitTimeout)
	defer timer.Stop()

	select {
	case l.sem <- struct{}{}:
		atomic.AddInt32(&l.currentPendingRequests, -1)
		return nil
	case <-timer.C:
		atomic.AddInt32(&l.currentPendingRequests, -1)
		return errors.New("timeout waiting for concurrency slot")
	}
}

func (l *Limiter) Release() {
	<-l.sem
}
