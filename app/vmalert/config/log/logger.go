package log

import "github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

// Logger is using lib/logger for logging
// but can be disabled via Disable method
type Logger struct {
	disabled bool
}

// Disable whether to ignore message logging.
// Once disabled, logging continues to be ignored
// until logger is enabled again.
// Not thread-safe.
func (l *Logger) Disable(v bool) {
	l.disabled = v
}

// Errorf logs error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.disabled {
		return
	}
	logger.Errorf(format, args...)
}

// Warnf logs warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	if l.disabled {
		return
	}
	logger.Warnf(format, args...)
}

// Infof logs info message.
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.disabled {
		return
	}
	logger.Infof(format, args...)
}

// Panicf logs panic message and panics.
func (l *Logger) Panicf(format string, args ...interface{}) {
	logger.Panicf(format, args...)
}
