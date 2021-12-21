package logger

import (
	"testing"
	"time"
)

func TestLoggerWithThrottler(t *testing.T) {
	lName := "test"
	lThrottle := 50 * time.Millisecond

	lt := WithThrottler(lName, lThrottle)
	var i int
	lt.warnF = func(format string, args ...interface{}) {
		i++
	}

	lt.Warnf("")
	lt.Warnf("")
	lt.Warnf("")

	if i != 1 {
		t.Fatalf("expected logger will be throttled to 1; got %d instead", i)
	}

	time.Sleep(lThrottle * 2) // wait to throttle to fade off
	// the same logger supposed to be return for the same name
	WithThrottler(lName, lThrottle).Warnf("")
	if i != 2 {
		t.Fatalf("expected logger to have 2 iterations; got %d instead", i)
	}

	logThrottlerRegistryMu.Lock()
	registeredN := len(logThrottlerRegistry)
	logThrottlerRegistryMu.Unlock()

	if registeredN != 1 {
		t.Fatalf("expected only 1 logger to be registered; got %d", registeredN)
	}
}
