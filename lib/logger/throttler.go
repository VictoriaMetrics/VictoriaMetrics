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
// Each unique `name` gets its own throttler.
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

// LogThrottler throttles Warnf/Errorf calls and emits a periodic summary
// showing how many logs were suppressed.
type LogThrottler struct {
	name          string
	throttle      time.Duration
	ch            chan struct{}
	mu            sync.Mutex
	suppressedCnt int
	lastSummary   time.Time
}

// newLogThrottler creates a throttled logger that allows one message per throttle duration.
// Suppressed logs are counted and periodically summarized.
func newLogThrottler(name string, throttle time.Duration) *LogThrottler {
	lt := &LogThrottler{
		name:        name,
		throttle:    throttle,
		ch:          make(chan struct{}, 1),
		lastSummary: time.Now(),
	}
	go lt.background()
	return lt
}

// background runs a loop that sleeps after each accepted log.
// It also prints summary logs periodically.
func (lt *LogThrottler) background() {
	ticker := time.NewTicker(1 * time.Minute) // summary interval
	defer ticker.Stop()

	for {
		select {
		case <-lt.ch:
			time.Sleep(lt.throttle)
		case <-ticker.C:
			lt.emitSummary()
		}
	}
}

// emitSummary prints how many log messages were suppressed in the last interval.
func (lt *LogThrottler) emitSummary() {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if lt.suppressedCnt > 0 {
		Warnf("suppressed %d log messages similar to \"%s\" during the last minute", lt.suppressedCnt, lt.name)
		lt.suppressedCnt = 0
		lt.lastSummary = time.Now()
	}
}

// recordSuppressed increments the suppressed log counter safely.
func (lt *LogThrottler) recordSuppressed() {
	lt.mu.Lock()
	lt.suppressedCnt++
	lt.mu.Unlock()
}

// Errorf logs an error message with throttling and suppression tracking.
func (lt *LogThrottler) Errorf(format string, args ...any) {
	select {
	case lt.ch <- struct{}{}:
		ErrorfSkipframes(1, format, args...)
	default:
		lt.recordSuppressed()
	}
}

// Warnf logs a warning message with throttling and suppression tracking.
func (lt *LogThrottler) Warnf(format string, args ...any) {
	select {
	case lt.ch <- struct{}{}:
		WarnfSkipframes(1, format, args...)
	default:
		lt.recordSuppressed()
	}
}
