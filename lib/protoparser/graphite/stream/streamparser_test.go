package stream

import (
	"reflect"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/graphite"
)

func Test_streamContext_Read(t *testing.T) {
	f := func(s string, rowsExpected *graphite.Rows) {
		t.Helper()
		ctx := getStreamContext(strings.NewReader(s))
		if !ctx.Read() {
			t.Fatalf("expecting successful read")
		}
		uw := getUnmarshalWork()
		callbackCalls := 0
		uw.ctx = ctx
		uw.callback = func(rows []graphite.Row) error {
			callbackCalls++
			if len(rows) != len(rowsExpected.Rows) {
				t.Fatalf("different len of expected rows;\ngot\n%+v;\nwant\n%+v", rows, rowsExpected.Rows)
			}
			if !reflect.DeepEqual(rows, rowsExpected.Rows) {
				t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows, rowsExpected.Rows)
			}
			return nil
		}
		uw.reqBuf = append(uw.reqBuf[:0], ctx.reqBuf...)
		ctx.wg.Add(1)
		uw.Unmarshal()
		if callbackCalls != 1 {
			t.Fatalf("unexpected number of callback calls; got %d; want 1", callbackCalls)
		}
	}

	// Full line without tags
	f("aaa 1123 345", &graphite.Rows{
		Rows: []graphite.Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: 345 * 1000,
		}},
	})
	// Full line with tags
	f("aaa;x=y 1123 345", &graphite.Rows{
		Rows: []graphite.Row{{
			Metric: "aaa",
			Tags: []graphite.Tag{{
				Key:   "x",
				Value: "y",
			}},
			Value:     1123,
			Timestamp: 345 * 1000,
		}},
	})
	// missing timestamp.
	// Note that this test may be flaky due to timing issues. TODO: fix it
	f("aaa 1123", &graphite.Rows{
		Rows: []graphite.Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: int64(fasttime.UnixTimestamp()) * 1000,
		}},
	})
	// -1 timestamp. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/610
	// Note that this test may be flaky due to timing issues. TODO: fix it.
	f("aaa 1123 -1", &graphite.Rows{
		Rows: []graphite.Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: int64(fasttime.UnixTimestamp()) * 1000,
		}},
	})
}
