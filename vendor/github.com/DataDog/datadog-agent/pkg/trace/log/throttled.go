// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package log

import (
	"time"

	"go.uber.org/atomic"
)

// NewThrottled returns a new throttled logger. The returned logger will allow up to n calls in
// a time period of length d.
func NewThrottled(n int, d time.Duration) *ThrottledLogger {
	return &ThrottledLogger{
		n: uint64(n),
		c: atomic.NewUint64(0),
		d: d,
	}
}

// ThrottledLogger limits the number of log calls during a time window. To create a new logger
// use NewThrottled.
type ThrottledLogger struct {
	n uint64         // number of log calls allowed during interval d
	c *atomic.Uint64 // number of log calls performed during an interval d
	d time.Duration
}

type loggerFunc func(format string, params ...interface{})

func (tl *ThrottledLogger) log(logFunc loggerFunc, format string, params ...interface{}) {
	c := tl.c.Inc() - 1
	if c == 0 {
		// first call, trigger the reset
		time.AfterFunc(tl.d, func() { tl.c.Store(0) })
	}
	if c >= tl.n {
		if c == tl.n {
			logFunc("Too many similar messages, pausing up to %s...", tl.d)
		}
		return
	}
	logFunc(format, params...)
}

// Error logs the message at the error level.
func (tl *ThrottledLogger) Error(format string, params ...interface{}) {
	tl.log(Errorf, format, params...)
}

// Warn logs the message at the warning level.
func (tl *ThrottledLogger) Warn(format string, params ...interface{}) {
	tl.log(Warnf, format, params...)
}

// Write implements io.Writer.
func (tl *ThrottledLogger) Write(p []byte) (n int, err error) {
	tl.Error(string(p))
	return len(p), nil
}
