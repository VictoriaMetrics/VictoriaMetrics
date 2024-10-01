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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	loggerLevel    = flag.String("loggerLevel", "INFO", "Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC")
	loggerFormat   = flag.String("loggerFormat", "default", "Format for logs. Possible values: default, json")
	loggerOutput   = flag.String("loggerOutput", "stderr", "Output for the logs. Supported values: stderr, stdout")
	loggerTimezone = flag.String("loggerTimezone", "UTC", "Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. "+
		"For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local")
	disableTimestamps = flag.Bool("loggerDisableTimestamps", false, "Whether to disable writing timestamps in logs")
	maxLogArgLen      = flag.Int("loggerMaxArgLen", 5000, "The maximum length of a single logged argument. Longer arguments are replaced with 'arg_start..arg_end', "+
		"where 'arg_start' and 'arg_end' is prefix and suffix of the arg with the length not exceeding -loggerMaxArgLen / 2")

	errorsPerSecondLimit = flag.Int("loggerErrorsPerSecondLimit", 0, `Per-second limit on the number of ERROR messages. If more than the given number of errors are emitted per second, the remaining errors are suppressed. Zero values disable the rate limit`)
	warnsPerSecondLimit  = flag.Int("loggerWarnsPerSecondLimit", 0, `Per-second limit on the number of WARN messages. If more than the given number of warns are emitted per second, then the remaining warns are suppressed. Zero values disable the rate limit`)
)

// Init initializes the logger.
//
// Init must be called after flag.Parse()
//
// There is no need in calling Init from tests.
func Init() {
	initInternal(true)
}

// InitNoLogFlags initializes the logger without writing flags to stdout
//
// InitNoLogFlags must be called after flag.Parse()
func InitNoLogFlags() {
	initInternal(false)
}

func initInternal(logFlags bool) {
	setLoggerJSONFields()
	setLoggerOutput()
	validateLoggerLevel()
	validateLoggerFormat()
	initTimezone()
	go logLimiterCleaner()

	if logFlags {
		logAllFlags()
	}
}

func initTimezone() {
	tz, err := time.LoadLocation(*loggerTimezone)
	if err != nil {
		log.Fatalf("cannot load timezone %q: %s", *loggerTimezone, err)
	}
	timezone = tz
}

var timezone = time.UTC

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
		// We cannot use logger.Panicf here, since the logger isn't initialized yet.
		panic(fmt.Errorf("FATAL: unsupported `-loggerFormat` value: %q; supported values are: default, json", *loggerFormat))
	}
}

var stdErrorLogger = log.New(&logWriter{}, "", 0)

// StdErrorLogger returns standard error logger.
func StdErrorLogger() *log.Logger {
	return stdErrorLogger
}

// Infof logs info message.
func Infof(format string, args ...any) {
	logLevel("INFO", format, args)
}

// Warnf logs warn message.
func Warnf(format string, args ...any) {
	logLevel("WARN", format, args)
}

// Errorf logs error message.
func Errorf(format string, args ...any) {
	logLevel("ERROR", format, args)
}

// WarnfSkipframes logs warn message and skips the given number of frames for the caller.
func WarnfSkipframes(skipframes int, format string, args ...any) {
	logLevelSkipframes(skipframes, "WARN", format, args)
}

// ErrorfSkipframes logs error message and skips the given number of frames for the caller.
func ErrorfSkipframes(skipframes int, format string, args ...any) {
	logLevelSkipframes(skipframes, "ERROR", format, args)
}

// Fatalf logs fatal message and terminates the app.
func Fatalf(format string, args ...any) {
	logLevel("FATAL", format, args)
}

// Panicf logs panic message and panics.
func Panicf(format string, args ...any) {
	logLevel("PANIC", format, args)
}

func logLevel(level, format string, args []any) {
	logLevelSkipframes(1, level, format, args)
}

func logLevelSkipframes(skipframes int, level, format string, args []any) {
	if shouldSkipLog(level) {
		return
	}
	msg := formatLogMessage(*maxLogArgLen, format, args)
	logMessage(level, msg, 3+skipframes)
}

func formatLogMessage(maxArgLen int, format string, args []any) string {
	x := format
	// Limit the length of every string-like arg in order to prevent from too long log messages
	for i := range args {
		n := strings.IndexByte(x, '%')
		if n < 0 {
			break
		}
		x = x[n+1:]
		if strings.HasPrefix(x, "s") || strings.HasPrefix(x, "q") {
			s := fmt.Sprintf("%s", args[i])
			args[i] = stringsutil.LimitStringLen(s, maxArgLen)
		}
	}
	return fmt.Sprintf(format, args...)
}

func logLimiterCleaner() {
	for {
		time.Sleep(time.Second)
		logLimiter.reset()
	}
}

var logLimiter = newLogLimit()

func newLogLimit() *logLimit {
	return &logLimit{
		m: make(map[string]uint64),
	}
}

type logLimit struct {
	mu sync.Mutex
	m  map[string]uint64
}

func (ll *logLimit) reset() {
	ll.mu.Lock()
	ll.m = make(map[string]uint64, len(ll.m))
	ll.mu.Unlock()
}

// needSuppress checks if the number of calls for the given location exceeds the given limit.
//
// When the number of calls equals limit, log message prefix returned.
func (ll *logLimit) needSuppress(location string, limit uint64) (bool, string) {
	// fast path
	var msg string
	if limit == 0 {
		return false, msg
	}
	ll.mu.Lock()
	defer ll.mu.Unlock()

	if n, ok := ll.m[location]; ok {
		if n >= limit {
			switch n {
			// report only once
			case limit:
				msg = fmt.Sprintf("suppressing log message with rate limit=%d: ", limit)
			default:
				return true, msg
			}
		}
		ll.m[location] = n + 1
	} else {
		ll.m[location] = 1
	}
	return false, msg
}

type logWriter struct {
}

func (lw *logWriter) Write(p []byte) (int, error) {
	logLevelSkipframes(2, "ERROR", "%s", []any{p})
	return len(p), nil
}

func logMessage(level, msg string, skipframes int) {
	timestamp := ""
	if !*disableTimestamps {
		timestamp = time.Now().In(timezone).Format("2006-01-02T15:04:05.000Z0700")
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
			logMsg = fmt.Sprintf(
				`{%q:%q,%q:%q,%q:%q}`+"\n",
				fieldLevel, levelLowercase,
				fieldCaller, location,
				fieldMsg, msg,
			)
		} else {
			logMsg = fmt.Sprintf(
				`{%q:%q,%q:%q,%q:%q,%q:%q}`+"\n",
				fieldTs, timestamp,
				fieldLevel, levelLowercase,
				fieldCaller, location,
				fieldMsg, msg,
			)
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

// SetOutputForTests redefine output for logger. Use for Tests only. Call ResetOutputForTest to return output state to default
func SetOutputForTests(writer io.Writer) { output = writer }

// ResetOutputForTest set logger output to default value
func ResetOutputForTest() { output = os.Stderr }
