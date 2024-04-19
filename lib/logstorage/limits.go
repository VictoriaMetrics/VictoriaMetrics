package logstorage

import "fmt"

type limiter interface {
	String() string
	Limit() int
}

var nl noopLimiter

type noopLimiter struct{}

func (noopLimiter) String() string {
	return ""
}

func (noopLimiter) Limit() int {
	return 0
}

type intLimiter struct {
	fieldName string
	limit     int
}

func (i *intLimiter) String() string {
	return fmt.Sprintf("%s: %d", i.fieldName, i.limit)
}

func (i *intLimiter) Limit() int {
	return i.limit
}
