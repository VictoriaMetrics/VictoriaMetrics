package stream

import (
	"testing"
)

func TestDetectTimestamp(t *testing.T) {
	tsDefault := int64(123)
	f := func(ts, tsExpected int64) {
		t.Helper()
		tsResult := detectTimestamp(ts, tsDefault)
		if tsResult != tsExpected {
			t.Fatalf("unexpected timestamp for detectTimestamp(%d, %d); got %d; want %d", ts, tsDefault, tsResult, tsExpected)
		}
	}
	f(0, tsDefault)
	f(1, 1e3)
	f(1e7, 1e10)
	f(1e8, 1e11)
	f(1e9, 1e12)
	f(1e10, 1e13)
	f(1e11, 1e11)
	f(1e12, 1e12)
	f(1e13, 1e13)
	f(1e14, 1e11)
	f(1e15, 1e12)
	f(1e16, 1e13)
	f(1e17, 1e11)
	f(1e18, 1e12)
}
