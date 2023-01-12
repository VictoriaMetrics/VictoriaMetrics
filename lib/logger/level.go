package logger

import (
	"flag"
	"fmt"
	"sync"
	"time"
)

var (
	loggerLevel          = flag.String("loggerLevel", "INFO", "Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC")
	errorsPerSecondLimit = flag.Int("loggerErrorsPerSecondLimit", 0, `Per-second limit on the number of ERROR messages. If more than the given number of errors are emitted per second, the remaining errors are suppressed. Zero values disable the rate limit`)
	warnsPerSecondLimit  = flag.Int("loggerWarnsPerSecondLimit", 0, `Per-second limit on the number of WARN messages. If more than the given number of warns are emitted per second, then the remaining warns are suppressed. Zero values disable the rate limit`)
)

func setLoggerLevel() {
	switch *loggerLevel {
	case "INFO":
		minLogLevel = levelInfo
	case "WARN":
		minLogLevel = levelWarn
	case "ERROR":
		minLogLevel = levelError
	case "FATAL":
		minLogLevel = levelFatal
	case "PANIC":
		minLogLevel = levelPanic
	default:
		// We cannot use logger.Panicf here, since the logger isn't initialized yet.
		panic(fmt.Errorf("FATAL: unsupported `-loggerLevel` value: %q; supported values are: INFO, WARN, ERROR, FATAL, PANIC", *loggerLevel))
	}
}

var minLogLevel logLevel = levelInfo

type logLevel uint8

const (
	levelInfo logLevel = iota
	levelWarn
	levelError
	levelFatal
	levelPanic

	levelCount
)

var logLevelNames = [levelCount]string{
	"info",
	"warn",
	"error",
	"fatal",
	"panic",
}

func (lvl logLevel) String() string {
	if lvl >= levelCount {
		Panicf("BUG: unknown logLevel=%d", lvl)
	}
	return logLevelNames[lvl]
}

func (lvl logLevel) limit() uint64 {
	switch lvl {
	case levelWarn:
		return uint64(*warnsPerSecondLimit)
	case levelError:
		return uint64(*errorsPerSecondLimit)
	default:
		return 0
	}
}

func logLimiterCleaner() {
	for {
		time.Sleep(time.Second)
		limiter.reset()
	}
}

var limiter = newLogLimiter()

func newLogLimiter() *logLimiter {
	return &logLimiter{
		m: make(map[string]uint64),
	}
}

type logLimiter struct {
	mu sync.Mutex
	m  map[string]uint64
}

func (ll *logLimiter) reset() {
	ll.mu.Lock()
	ll.m = make(map[string]uint64, len(ll.m))
	ll.mu.Unlock()
}

// filterMessage returns an empty message and true (discard) if the number of calls
// for the given location exceeds the given log level's rate limit.
//
// If the number of calls exactly equals the limit, a prefix is added to the message
// indicating that further messages will be suppressed, and false (accept) is returned.
//
// Otherwise false (accept) is returned along with the original unchanged message.
func (ll *logLimiter) filterMessage(level logLevel, location, msg string) (bool, string) {
	limit := level.limit()
	// fast path
	if limit == 0 {
		return false, msg
	}

	ll.mu.Lock()
	defer ll.mu.Unlock()

	if n, ok := ll.m[location]; ok {
		if n >= limit {
			switch n {
			case limit:
				// Limit hit: add a prefix indicating that further messages will be suppressed.
				msg = fmt.Sprintf("suppressing log message with rate limit=%d: %s", limit, msg)
			default:
				// Limit exceeded: suppress the message (no need to update the map).
				return true, ""
			}
		}
		ll.m[location] = n + 1
	} else {
		ll.m[location] = 1
	}
	return false, msg
}
