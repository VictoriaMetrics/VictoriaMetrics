package logger

import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	logThrottlerRegistryMu = sync.Mutex{}
	logThrottlerRegistry   = make(map[string]*LogThrottler)
)

// WithThrottler returns a logger throttled by time - only one message in throttle duration will be logged.
//
// New logger is created only once for each unique name passed.
// The function is thread-safe.
func WithThrottler(name string, throttle time.Duration) *LogThrottler {
	logThrottlerRegistryMu.Lock()
	defer logThrottlerRegistryMu.Unlock()

	lt, ok := logThrottlerRegistry[name]
	if ok {
		return lt
	}

	lt = newLogThrottler(name, throttle)
	logThrottlerRegistry[name] = lt
	return lt
}

// LogThrottler is a logger, which throttles log messages passed to Warnf and Errorf.
//
// LogThrottler must be created via WithThrottler() call.
type LogThrottler struct {
	ch chan struct{}
	name string
	dropped uint64
}

func newLogThrottler(name string, throttle time.Duration) *LogThrottler {
	lt := &LogThrottler{
		ch: make(chan struct{}, 1),
		name: name,
	}
	go func() {
		for {
			<-lt.ch
			time.Sleep(throttle)
		}
	}()
	// Reports suppressed (dropped) messages
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			n := atomic.SwapUint64(&lt.dropped, 0)
			if n > 0 {
				Infof("%s: suppressed %d log messages during the last 1m", lt.name, n)
			}
		}
	}()
	return lt
}

// Errorf logs error message.
func (lt *LogThrottler) Errorf(format string, args ...any) {
	select {
	case lt.ch <- struct{}{}:
		ErrorfSkipframes(1, format, args...)
	default:
		atomic.AddUint64(&lt.dropped, 1)
	}
}

// Warnf logs warn message.
func (lt *LogThrottler) Warnf(format string, args ...any) {
	select {
	case lt.ch <- struct{}{}:
		WarnfSkipframes(1, format, args...)
	default:
		atomic.AddUint64(&lt.dropped, 1)
	}
}
