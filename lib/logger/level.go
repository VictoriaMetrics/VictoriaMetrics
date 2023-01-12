package logger

import (
	"flag"
	"fmt"
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
