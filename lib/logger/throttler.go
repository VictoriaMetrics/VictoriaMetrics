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
	period time.Duration

	mu       sync.Mutex
	nextTime time.Time
}

func newLogThrottler(throttle time.Duration) *LogThrottler {
	return &LogThrottler{period: throttle}
}

// Errorf logs error message.
func (lt *LogThrottler) Errorf(format string, args ...interface{}) {
	if lt.tryLog() {
		ErrorfSkipframes(1, format, args...)
	}
}

// Warnf logs warn message.
func (lt *LogThrottler) Warnf(format string, args ...interface{}) {
	if lt.tryLog() {
		WarnfSkipframes(1, format, args...)
	}
}

func (lt *LogThrottler) tryLog() (ok bool) {
	// If already locked by another caller, there are two cases:
	//   - that caller is allowed to log now
	//   - that caller is not allowed to log yet
	// Since there can only be one message per log period, in both
	// cases, we can efficiently infer that we are not allowed to
	// log yet.
	if lt.mu.TryLock() {
		if t := time.Now(); t.After(lt.nextTime) {
			lt.nextTime = t.Add(lt.period)
			ok = true
		}
		lt.mu.Unlock()
	}
	return
}
