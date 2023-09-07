package logger

import (
	"bytes"
	"io"
	"path"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const timestep = 16 * time.Millisecond

func init() {
	isInTest = true
}

func resetOutputCallsTotal() {
	atomic.StoreUint64(&outputCallsTotal, 0)
}

func expectOutputCallsTotal(t *testing.T, calls uint64) {
	t.Helper()
	atomic.LoadUint64(&outputCallsTotal)
	if outputCallsTotal != calls {
		t.Fatalf("outputCallsTotal: expected %d, received %d", calls, outputCallsTotal)
	}
}

func restore[T any](vp *T, v T) {
	*vp = v
}

func TestLogLevelMapping(t *testing.T) {
	defer restore(loggerLevel, *loggerLevel)
	defer restore(&minLogLevel, minLogLevel)

	for lvl := logLevel(0); lvl < levelCount; lvl++ {
		levelFlag := strings.ToUpper(lvl.String())
		loggerLevel = &levelFlag

		setLoggerLevel()
		if minLogLevel != lvl {
			t.Errorf("expected level %q, received level %q", lvl, minLogLevel)
		}
	}
}

func TestLoggerSkipFrames(t *testing.T) {
	defer restore(disableTimestamps, *disableTimestamps)

	_, fullPath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to determine test filename")
	}
	prefix, file := path.Split(fullPath)
	prefix, parent := path.Split(path.Clean(prefix))
	_, grandparent := path.Split(path.Clean(prefix))

	// eg. "/lib/logger/logger_test.go"
	expectedPathSuffix := path.Join("/", grandparent, parent, file)

	f0 := func(location string) {
		t.Helper()

		if ind := strings.LastIndexByte(location, ':'); ind < 0 {
			t.Fatalf(`malformed location: %q, expected "<file>:<line>"`, location)
		} else {
			location = location[:ind]
		}
		if !strings.HasSuffix(location, expectedPathSuffix) {
			t.Errorf(`expected file path "[...]%s", received: %q`, expectedPathSuffix, location)
		}
	}

	f0(callerLocation(0))

	var capturedOutput bytes.Buffer
	defer SetOutput(SetOutput(&capturedOutput))
	*disableTimestamps = true

	f := func() {
		t.Helper()

		if capturedOutput.Len() == 0 {
			t.Fatal("expected logger output")
		}
		_, _ = capturedOutput.ReadString('\t') // skip the log level field
		location, _ := capturedOutput.ReadString('\t')
		capturedOutput.Reset()

		f0(location)
	}

	Infof("")
	f()

	Warnf("")
	f()

	WarnfSkipframes(0, "")
	f()

	Errorf("")
	f()

	ErrorfSkipframes(0, "")
	f()

	func() {
		// logf expects to be wrapped, so this must be inside a wrapper frame.
		logf(levelInfo, "")
		f()
	}()

	logfSkipframes(0, levelInfo, "")
	f()

	StdErrorLogger().Print("")
	f()

	{
		lt := newLogThrottler(timestep)

		lt.Warnf("")
		f()

		time.Sleep(timestep + timestep/4)

		lt.Errorf("")
		f()
	}
}

func TestLogLimiter(t *testing.T) {
	defer SetOutput(SetOutput(io.Discard))
	defer restore(&output, output)
	defer restore(&limitPeriod, limitPeriod)
	defer restore(&limiter, limiter)
	defer restore(warnsPerSecondLimit, *warnsPerSecondLimit)
	defer restore(errorsPerSecondLimit, *errorsPerSecondLimit)

	limitPeriod = timestep
	limiter = newLogLimiter()
	*warnsPerSecondLimit = 2
	*errorsPerSecondLimit = 2

	errorWithFixedLocation := func() {
		Errorf("")
	}

	resetOutputCallsTotal()

	for i := 0; i < 3; i++ {
		Warnf("")
		Warnf("")
		errorWithFixedLocation()
		errorWithFixedLocation()
	}
	expectOutputCallsTotal(t, 6)

	time.Sleep(timestep + timestep/4)

	for i := 0; i < 3; i++ {
		Warnf("")
		Warnf("")
		errorWithFixedLocation()
		errorWithFixedLocation()
	}
	expectOutputCallsTotal(t, 12)
}

func TestLogThrottler(t *testing.T) {
	defer SetOutput(SetOutput(io.Discard))

	lt := newLogThrottler(timestep)

	resetOutputCallsTotal()

	lt.Warnf("")
	lt.Warnf("") // should be discarded
	expectOutputCallsTotal(t, 1)

	time.Sleep(timestep + timestep/4)

	lt.Warnf("")
	lt.Warnf("") // should be discarded
	expectOutputCallsTotal(t, 2)
}
