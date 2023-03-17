package log

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// Logger is using lib/logger for logging
// but can be suppressed via Suppress method
type Logger struct {
	mu       sync.RWMutex
	disabled bool
}

// Suppress whether to ignore message logging.
// Once suppressed, logging continues to be ignored
// until logger is un-suppressed.
func (l *Logger) Suppress(v bool) {
	l.mu.Lock()
	l.disabled = v
	l.mu.Unlock()
}

func (l *Logger) isDisabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.disabled
}

// Errorf logs error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.isDisabled() {
		return
	}
	logger.Errorf(format, args...)
}

// Warnf logs warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	if l.isDisabled() {
		return
	}
	logger.Warnf(format, args...)
}

// Infof logs info message.
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.isDisabled() {
		return
	}
	logger.Infof(format, args...)
}

// Panicf logs panic message and panics.
// Panicf can't be suppressed
func (l *Logger) Panicf(format string, args ...interface{}) {
	logger.Panicf(format, args...)
}
