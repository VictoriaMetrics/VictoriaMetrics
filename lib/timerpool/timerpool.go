package timerpool

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// Get returns a timer for the given duration d from the pool.
//
// Return back the timer to the pool with Put.
func Get(d time.Duration) *time.Timer {
	if v := timerPool.Get(); v != nil {
		t := v.(*time.Timer)
		if t.Reset(d) {
			logger.Panicf("BUG: active timer trapped to the pool!")
		}
		return t
	}
	return time.NewTimer(d)
}

// Put returns t to the pool.
//
// t cannot be accessed after returning to the pool.
func Put(t *time.Timer) {
	if !t.Stop() {
		// Drain t.C if it wasn't obtained by the caller yet.
		select {
		case <-t.C:
		default:
		}
	}
	timerPool.Put(t)
}

var timerPool sync.Pool
