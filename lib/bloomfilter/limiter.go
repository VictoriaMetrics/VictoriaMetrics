package bloomfilter

import (
	"sync"
	"sync/atomic"
	"time"
)

// Limiter limits the number of added items.
//
// It is safe using the Limiter from concurrent goroutines.
type Limiter struct {
	maxItems int
	v        atomic.Value

	wg     sync.WaitGroup
	stopCh chan struct{}
}

// NewLimiter creates new Limiter, which can hold up to maxItems unique items during the given refreshInterval.
func NewLimiter(maxItems int, refreshInterval time.Duration) *Limiter {
	l := &Limiter{
		maxItems: maxItems,
		stopCh:   make(chan struct{}),
	}
	l.v.Store(newLimiter(maxItems))
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		t := time.NewTicker(refreshInterval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				l.v.Store(newLimiter(maxItems))
			case <-l.stopCh:
				return
			}
		}
	}()
	return l
}

// MustStop stops the given limiter.
// It is expected that nobody access the limiter at MustStop call.
func (l *Limiter) MustStop() {
	close(l.stopCh)
	l.wg.Wait()
}

// MaxItems returns the maxItems passed to NewLimiter.
func (l *Limiter) MaxItems() int {
	return l.maxItems
}

// CurrentItems return the current number of items registered in l.
func (l *Limiter) CurrentItems() int {
	lm := l.v.Load().(*limiter)
	n := atomic.LoadUint64(&lm.currentItems)
	return int(n)
}

// Add adds h to the limiter.
//
// It is safe calling Add from concurrent goroutines.
//
// True is returned if h is added or already exists in l.
// False is returned if h cannot be added to l, since it already has maxItems unique items.
func (l *Limiter) Add(h uint64) bool {
	lm := l.v.Load().(*limiter)
	return lm.Add(h)
}

type limiter struct {
	currentItems uint64
	f            *filter
}

func newLimiter(maxItems int) *limiter {
	return &limiter{
		f: newFilter(maxItems),
	}
}

func (l *limiter) Add(h uint64) bool {
	currentItems := atomic.LoadUint64(&l.currentItems)
	if currentItems >= uint64(l.f.maxItems) {
		return l.f.Has(h)
	}
	if l.f.Add(h) {
		atomic.AddUint64(&l.currentItems, 1)
	}
	return true
}
