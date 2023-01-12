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

var limiter = newLogLimiter()
var limitPeriod = time.Second // Only changed for tests

func newLogLimiter() *logLimiter {
	// With a zero-valued expireTime, filterMessage() will naturally init on first use.
	return &logLimiter{}
}

type logLimiter struct {
	mu         sync.Mutex
	m          map[string]uint64
	expireTime time.Time
}

// filterMessage returns an empty message and false (discard) if the number of calls
// for the given location exceeds the given log level's rate limit.
//
// If the number of calls exactly equals the limit, a prefix is added to the message
// indicating that further messages will be suppressed, and true (accept) is returned.
//
// Otherwise true (accept) is returned along with the original unchanged message.
func (ll *logLimiter) filterMessage(timestamp time.Time, level logLevel, location, msg string) (_ string, ok bool) {
	if limit := level.limit(); limit != 0 {
		var n uint64
		ll.mu.Lock()

		if timestamp.After(ll.expireTime) {
			ll.m = make(map[string]uint64, len(ll.m))
			ll.expireTime = timestamp.Add(limitPeriod)

			n = 1
		} else {
			n = ll.m[location] + 1

			if n > limit {
				// Limit exceeded: suppress the message (no need to update the map).
				ll.mu.Unlock()
				return
			}
			if n == limit {
				// Limit hit: add a prefix indicating that further messages will be suppressed.
				// Release the lock before formatting the prefixed message.
				ll.m[location] = n
				ll.mu.Unlock()

				return fmt.Sprintf("suppressing log message with rate limit=%d: %s", limit, msg), true
			}
		}

		ll.m[location] = n
		ll.mu.Unlock()
	}
	return msg, true
}
