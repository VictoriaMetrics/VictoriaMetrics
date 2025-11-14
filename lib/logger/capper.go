package logger

import "sync/atomic"

type LogCapper struct {
	maxLogs   int
	errorFunc func(string)
	count     int32
}

func NewLogCapper(maxLogs int, errorFunc func(string)) *LogCapper {
	return &LogCapper{
		maxLogs:   maxLogs,
		errorFunc: errorFunc,
	}
}

func (l *LogCapper) Error(s string) {
	c := atomic.AddInt32(&l.count, 1)
	if int(c) <= l.maxLogs {
		l.errorFunc(s)
	}
}
