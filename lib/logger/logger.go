package logger

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/metrics"
)

var loggerOutput = flag.String("loggerOutput", "stderr", "Output for the logs. Supported values: stderr, stdout")

// Init initializes the logger.
//
// Init must be called after flag.Parse()
//
// There is no need in calling Init from tests.
func Init() {
	setLoggerJSONFields()
	setLoggerOutput()
	setLoggerLevel()
	setLoggerFormat()
	initTimezone()
	go logLimiterCleaner()
	logAllFlags()
}

func setLoggerOutput() {
	switch *loggerOutput {
	case "stderr":
		output = os.Stderr
	case "stdout":
		output = os.Stdout
	default:
		panic(fmt.Errorf("FATAL: unsupported `loggerOutput` value: %q; supported values are: stderr, stdout", *loggerOutput))
	}
}

var output io.Writer = os.Stderr

// Infof logs info message.
func Infof(format string, args ...interface{}) {
	logf(levelInfo, format, args...)
}

// Warnf logs warn message.
func Warnf(format string, args ...interface{}) {
	logf(levelWarn, format, args...)
}

// WarnfSkipframes logs warn message and skips the given number of frames for the caller.
func WarnfSkipframes(skipframes int, format string, args ...interface{}) {
	logfSkipframes(1+skipframes, levelWarn, format, args...)
}

// Errorf logs error message.
func Errorf(format string, args ...interface{}) {
	logf(levelError, format, args...)
}

// ErrorfSkipframes logs error message and skips the given number of frames for the caller.
func ErrorfSkipframes(skipframes int, format string, args ...interface{}) {
	logfSkipframes(1+skipframes, levelError, format, args...)
}

// Fatalf logs fatal message and terminates the app.
func Fatalf(format string, args ...interface{}) {
	logf(levelFatal, format, args...)
}

// Panicf logs panic message and panics.
func Panicf(format string, args ...interface{}) {
	logf(levelPanic, format, args...)
}

// logf is an internal convenience helper that expects to be wrapped by
// an outer function, so it uses skipframes=2: 1 for the wrapper and 1 for itself.
func logf(level logLevel, format string, args ...interface{}) {
	logfSkipframes(2, level, format, args...)
}

func logfSkipframes(skipframes int, level logLevel, format string, args ...interface{}) {
	if level < minLogLevel {
		return
	}
	msg := fmt.Sprintf(format, args...)
	logMessage(1+skipframes, level, msg)
}

func logMessage(skipframes int, level logLevel, msg string) {
	timestamp := time.Now()
	location := callerLocation(1 + skipframes)

	ok, msg := limiter.filterMessage(level, location, msg)
	if ok {
		return
	}

	for len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	formattedMsg := formatter.formatMessage(timestamp, level, location, msg)

	// Serialize writes to log.
	mu.Lock()
	_, _ = output.Write([]byte(formattedMsg))
	mu.Unlock()

	// Increment vm_log_messages_total
	counterName := fmt.Sprintf(`vm_log_messages_total{app_version=%q, level=%q, location=%q}`, buildinfo.Version, level, location)
	metrics.GetOrCreateCounter(counterName).Inc()

	switch level {
	case levelPanic:
		if formatter.shouldSuppressPanicStacktrace() {
			os.Exit(-1)
		}
		panic(errors.New(msg))
	case levelFatal:
		os.Exit(-1)
	}
}

var mu sync.Mutex

func callerLocation(skipframes int) string {
	_, file, line, ok := runtime.Caller(1 + skipframes)
	if !ok {
		file = "???"
		line = 0
	}
	if n := strings.Index(file, "/VictoriaMetrics/"); n >= 0 {
		// Strip [...]/VictoriaMetrics/ prefix
		file = file[n+len("/VictoriaMetrics/"):]
	}
	return fmt.Sprintf("%s:%d", file, line)
}

// StdErrorLogger returns standard error logger.
func StdErrorLogger() *log.Logger {
	return stdErrorLogger
}

var stdErrorLogger = log.New(&stdErrorWriter{}, "", 0)

type stdErrorWriter struct {
}

func (lw *stdErrorWriter) Write(p []byte) (int, error) {
	ErrorfSkipframes(3, "%s", p)
	return len(p), nil
}

// SetOutputForTests redefine output for logger. Use for Tests only. Call ResetOutputForTest to return output state to default
func SetOutputForTests(writer io.Writer) { output = writer }

// ResetOutputForTest set logger output to default value
func ResetOutputForTest() { output = os.Stderr }
