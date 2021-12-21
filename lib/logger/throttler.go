package logger

import (
	"sync"
	"time"
)

var (
	logThrottlerRegistryMu = sync.Mutex{}
	logThrottlerRegistry   = make(map[string]*logThrottler)
)

// WithThrottler returns a logger throttled by time - only
// one message in throttle duration will be logged.
// New logger is created only once for each unique name passed.
// Thread-safe.
func WithThrottler(name string, throttle time.Duration) *logThrottler {
	logThrottlerRegistryMu.Lock()
	defer logThrottlerRegistryMu.Unlock()

	lt, ok := logThrottlerRegistry[name]
	if ok {
		return lt
	}

	lt = newLogThrottler(throttle)
	lt.warnF = Warnf
	lt.errorF = Errorf
	logThrottlerRegistry[name] = lt
	return lt
}

type logThrottler struct {
	ch chan struct{}

	warnF  func(format string, args ...interface{})
	errorF func(format string, args ...interface{})
}

func newLogThrottler(throttle time.Duration) *logThrottler {
	lt := &logThrottler{ch: make(chan struct{}, 1)}
	go func() {
		for {
			<-lt.ch
			time.Sleep(throttle)
		}
	}()
	return lt
}

// Errorf logs error message.
func (lt *logThrottler) Errorf(format string, args ...interface{}) {
	select {
	case lt.ch <- struct{}{}:
		lt.errorF(format, args...)
	default:
	}
}

// Warnf logs warn message.
func (lt *logThrottler) Warnf(format string, args ...interface{}) {
	select {
	case lt.ch <- struct{}{}:
		lt.warnF(format, args...)
	default:
	}
}
