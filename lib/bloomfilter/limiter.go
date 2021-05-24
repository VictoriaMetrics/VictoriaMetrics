package bloomfilter

import (
	"sync/atomic"
	"time"
)

// Limiter limits the number of added items.
//
// It is safe using the Limiter from concurrent goroutines.
type Limiter struct {
	maxItems int
	v        atomic.Value
}

// NewLimiter creates new Limiter, which can hold up to maxItems unique items during the given refreshInterval.
func NewLimiter(maxItems int, refreshInterval time.Duration) *Limiter {
	l := &Limiter{
		maxItems: maxItems,
	}
	l.v.Store(newLimiter(maxItems))
	go func() {
		for {
			time.Sleep(refreshInterval)
			l.v.Store(newLimiter(maxItems))
		}
	}()
	return l
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
