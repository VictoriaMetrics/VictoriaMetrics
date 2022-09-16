// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package log

import (
	"bytes"
	"fmt"
)

var _ Logger = (*buflogger)(nil)

// NewBufferLogger creates a new Logger which outputs everything to the given buffer.
func NewBufferLogger(out *bytes.Buffer) Logger {
	return &buflogger{out}
}

type buflogger struct{ buf *bytes.Buffer }

func (b *buflogger) logWithLevel(lvl string, msg string) {
	b.buf.WriteString(fmt.Sprintf("[%s] %s", lvl, msg))
}

// Trace implements Logger.
func (b *buflogger) Trace(v ...interface{}) { b.logWithLevel("TRACE", fmt.Sprint(v...)) }

// Tracef implements Logger.
func (b *buflogger) Tracef(format string, params ...interface{}) {
	b.logWithLevel("TRACE", fmt.Sprintf(format, params...))
}

// Debug implements Logger.
func (b *buflogger) Debug(v ...interface{}) { b.logWithLevel("DEBUG", fmt.Sprint(v...)) }

// Debugf implements Logger.
func (b *buflogger) Debugf(format string, params ...interface{}) {
	b.logWithLevel("DEBUG", fmt.Sprintf(format, params...))
}

// Info implements Logger.
func (b *buflogger) Info(v ...interface{}) { b.logWithLevel("INFO", fmt.Sprint(v...)) }

// Infof implements Logger.
func (b *buflogger) Infof(format string, params ...interface{}) {
	b.logWithLevel("INFO", fmt.Sprintf(format, params...))
}

// Warn implements Logger.
func (b *buflogger) Warn(v ...interface{}) error {
	b.logWithLevel("WARN", fmt.Sprint(v...))
	return nil
}

// Warnf implements Logger.
func (b *buflogger) Warnf(format string, params ...interface{}) error {
	b.logWithLevel("WARN", fmt.Sprintf(format, params...))
	return nil
}

// Error implements Logger.
func (b *buflogger) Error(v ...interface{}) error {
	b.logWithLevel("ERROR", fmt.Sprint(v...))
	return nil
}

// Errorf implements Logger.
func (b *buflogger) Errorf(format string, params ...interface{}) error {
	b.logWithLevel("ERROR", fmt.Sprintf(format, params...))
	return nil
}

// Critical implements Logger.
func (b *buflogger) Critical(v ...interface{}) error {
	b.logWithLevel("CRITICAL", fmt.Sprint(v...))
	return nil
}

// Criticalf implements Logger.
func (b *buflogger) Criticalf(format string, params ...interface{}) error {
	b.logWithLevel("CRITICAL", fmt.Sprintf(format, params...))
	return nil
}

// Flush implements Logger.
func (b *buflogger) Flush() {}
