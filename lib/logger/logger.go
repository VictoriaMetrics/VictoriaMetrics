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

var (
	loggerLevel       = flag.String("loggerLevel", "INFO", "Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC")
	loggerFormat      = flag.String("loggerFormat", "default", "Format for logs. Possible values: default, json")
	loggerOutput      = flag.String("loggerOutput", "stderr", "Output for the logs. Supported values: stderr, stdout")
	disableTimestamps = flag.Bool("loggerDisableTimestamps", false, "Whether to disable writing timestamps in logs")

	errorsPerSecondLimit = flag.Int("loggerErrorsPerSecondLimit", 10, "Per-second limit on the number of ERROR messages. If more than the given number of errors "+
		"are emitted per second, then the remaining errors are suppressed. Zero value disables the rate limit")
	warnsPerSecondLimit = flag.Int("loggerWarnsPerSecondLimit", 10, "Per-second limit on the number of WARN messages. If more than the given number of warns "+
		"are emitted per second, then the remaining warns are suppressed. Zero value disables the rate limit")
)

// Init initializes the logger.
//
// Init must be called after flag.Parse()
//
// There is no need in calling Init from tests.
func Init() {
	setLoggerOutput()
	validateLoggerLevel()
	validateLoggerFormat()
	go errorsAndWarnsLoggedCleaner()
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

func validateLoggerLevel() {
	switch *loggerLevel {
	case "INFO", "WARN", "ERROR", "FATAL", "PANIC":
	default:
		// We cannot use logger.Panicf here, since the logger isn't initialized yet.
		panic(fmt.Errorf("FATAL: unsupported `-loggerLevel` value: %q; supported values are: INFO, WARN, ERROR, FATAL, PANIC", *loggerLevel))
	}
}

func validateLoggerFormat() {
	switch *loggerFormat {
	case "default", "json":
	default:
		// We cannot use logger.Pancif here, since the logger isn't initialized yet.
		panic(fmt.Errorf("FATAL: unsupported `-loggerFormat` value: %q; supported values are: default, json", *loggerFormat))
	}
}

var stdErrorLogger = log.New(&logWriter{}, "", 0)

// StdErrorLogger returns standard error logger.
func StdErrorLogger() *log.Logger {
	return stdErrorLogger
}

// Infof logs info message.
func Infof(format string, args ...interface{}) {
	logLevel("INFO", format, args...)
}

// Warnf logs warn message.
func Warnf(format string, args ...interface{}) {
	logLevel("WARN", format, args...)
}

// Errorf logs error message.
func Errorf(format string, args ...interface{}) {
	logLevel("ERROR", format, args...)
}

// WarnfSkipframes logs warn message and skips the given number of frames for the caller.
func WarnfSkipframes(skipframes int, format string, args ...interface{}) {
	logLevelSkipframes(skipframes, "WARN", format, args...)
}

// ErrorfSkipframes logs error message and skips the given number of frames for the caller.
func ErrorfSkipframes(skipframes int, format string, args ...interface{}) {
	logLevelSkipframes(skipframes, "ERROR", format, args...)
}

// Fatalf logs fatal message and terminates the app.
func Fatalf(format string, args ...interface{}) {
	logLevel("FATAL", format, args...)
}

// Panicf logs panic message and panics.
func Panicf(format string, args ...interface{}) {
	logLevel("PANIC", format, args...)
}

func logLevel(level, format string, args ...interface{}) {
	logLevelSkipframes(1, level, format, args...)
}

func logLevelSkipframes(skipframes int, level, format string, args ...interface{}) {
	if shouldSkipLog(level) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	logMessage(level, msg, 3+skipframes)
}

func errorsAndWarnsLoggedCleaner() {
	for {
		time.Sleep(time.Second)
		logLimiter.reset()
	}
}

var (
	logLimiter = newLogLimit()
)

func newLogLimit() logLimit {
	return logLimit{
		s:  map[string]uint64{},
		mu: sync.Mutex{},
	}
}

type logLimit struct {
	mu sync.Mutex
	s  map[string]uint64
}

func (ll *logLimit) reset() {
	ll.mu.Lock()
	ll.s = make(map[string]uint64)
	ll.mu.Unlock()
}

// needSuppress check if given string exceeds limit,
// when count equals limit, log message prefix returned.
func (ll *logLimit) needSuppress(file string, limit uint64) (bool, string) {
	// fast path
	var msg string
	if limit == 0 {
		return false, msg
	}
	ll.mu.Lock()
	defer ll.mu.Unlock()

	if n, ok := ll.s[file]; ok {
		if n >= limit {
			switch n {
			// report only once
			case limit:
				msg = "suppressing log message with rate limit: "
			default:
				return true, msg
			}
		}
		ll.s[file] = n + 1
	} else {
		ll.s[file] = 1
	}
	return false, msg
}

type logWriter struct {
}

func (lw *logWriter) Write(p []byte) (int, error) {
	logLevelSkipframes(2, "ERROR", "%s", p)
	return len(p), nil
}

func logMessage(level, msg string, skipframes int) {

	timestamp := ""
	if !*disableTimestamps {
		timestamp = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	}
	levelLowercase := strings.ToLower(level)
	_, file, line, ok := runtime.Caller(skipframes)
	if !ok {
		file = "???"
		line = 0
	}
	if n := strings.Index(file, "/VictoriaMetrics/"); n >= 0 {
		// Strip /VictoriaMetrics/ prefix
		file = file[n+len("/VictoriaMetrics/"):]
	}
	location := fmt.Sprintf("%s:%d", file, line)

	// rate limit ERROR and WARN log messages with given limit.
	if level == "ERROR" || level == "WARN" {
		limit := uint64(*errorsPerSecondLimit)
		if level == "WARN" {
			limit = uint64(*warnsPerSecondLimit)
		}
		ok, suppressMessage := logLimiter.needSuppress(location, limit)
		if ok {
			return
		}
		if len(suppressMessage) > 0 {
			msg = suppressMessage + msg
		}
	}

	for len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	var logMsg string
	switch *loggerFormat {
	case "json":
		if *disableTimestamps {
			logMsg = fmt.Sprintf(`{"level":%q,"caller":%q,"msg":%q}`+"\n", levelLowercase, location, msg)
		} else {
			logMsg = fmt.Sprintf(`{"ts":%q,"level":%q,"caller":%q,"msg":%q}`+"\n", timestamp, levelLowercase, location, msg)
		}
	default:
		if *disableTimestamps {
			logMsg = fmt.Sprintf("%s\t%s\t%s\n", levelLowercase, location, msg)
		} else {
			logMsg = fmt.Sprintf("%s\t%s\t%s\t%s\n", timestamp, levelLowercase, location, msg)
		}
	}

	// Serialize writes to log.
	mu.Lock()
	fmt.Fprint(output, logMsg)
	mu.Unlock()

	// Increment vm_log_messages_total
	counterName := fmt.Sprintf(`vm_log_messages_total{app_version=%q, level=%q, location=%q}`, buildinfo.Version, levelLowercase, location)
	metrics.GetOrCreateCounter(counterName).Inc()

	switch level {
	case "PANIC":
		if *loggerFormat == "json" {
			// Do not clutter `json` output with panic stack trace
			os.Exit(-1)
		}
		panic(errors.New(msg))
	case "FATAL":
		os.Exit(-1)
	}
}

var mu sync.Mutex

func shouldSkipLog(level string) bool {
	switch *loggerLevel {
	case "WARN":
		switch level {
		case "WARN", "ERROR", "FATAL", "PANIC":
			return false
		default:
			return true
		}
	case "ERROR":
		switch level {
		case "ERROR", "FATAL", "PANIC":
			return false
		default:
			return true
		}
	case "FATAL":
		switch level {
		case "FATAL", "PANIC":
			return false
		default:
			return true
		}
	case "PANIC":
		return level != "PANIC"
	default:
		return false
	}
}
