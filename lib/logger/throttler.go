package logger

import (
	"sync"
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

	lt = newLogThrottler(throttle)
	logThrottlerRegistry[name] = lt
	return lt
}

// LogThrottler is a logger, which throttles log messages passed to Warnf and Errorf.
//
// LogThrottler must be created via WithThrottler() call.
type LogThrottler struct {
	ch chan struct{}
}

func newLogThrottler(throttle time.Duration) *LogThrottler {
	lt := &LogThrottler{
		ch: make(chan struct{}, 1),
	}
	go func() {
		for {
			<-lt.ch
			time.Sleep(throttle)
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
	}
}

// Warnf logs warn message.
func (lt *LogThrottler) Warnf(format string, args ...any) {
	select {
	case lt.ch <- struct{}{}:
		WarnfSkipframes(1, format, args...)
	default:
	}
}
