package promql

import (
	"fmt"
	"testing"
	"time"
)

func TestQueryLoggerShrink(t *testing.T) {
	f := func(addItems, limit, expectedLen int) {
		t.Helper()
		ql := &queryLogger{
			limit:                 10,
			maxQueryLogRecordTime: time.Second * 5,
		}
		for i := 0; i < addItems; i++ {
			ql.insertQuery(fmt.Sprintf("random-q-%d", i), int64(i), time.Now().Add(-time.Second), 500+time.Duration(i))
		}
		if len(ql.s) != expectedLen {
			t.Fatalf("unxpected len=%d, for querylogger slice, want=%d", len(ql.s), expectedLen)
		}
	}
	f(10, 5, 10)
	f(30, 5, 20)
	f(16, 5, 16)
}

func TestQueryLoggerTopCount(t *testing.T) {}
