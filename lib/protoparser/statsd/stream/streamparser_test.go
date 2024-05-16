package stream

import (
	"reflect"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/statsd"
)

func Test_streamContext_Read(t *testing.T) {
	f := func(s string, rowsExpected *statsd.Rows) {
		t.Helper()
		ctx := getStreamContext(strings.NewReader(s))
		if !ctx.Read() {
			t.Fatalf("expecting successful read")
		}
		uw := getUnmarshalWork()
		callbackCalls := 0
		uw.ctx = ctx
		uw.callback = func(rows []statsd.Row) error {
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
	f("aaa:1123|c", &statsd.Rows{
		Rows: []statsd.Row{{
			Metric: "aaa",
			Tags: []statsd.Tag{
				{
					Key:   "__statsd_metric_type__",
					Value: "c",
				},
			},
			Values:    []float64{1123},
			Timestamp: int64(fasttime.UnixTimestamp()) * 1000,
		}},
	})
	// Full line with tags
	f("aaa:1123|c|#x:y", &statsd.Rows{
		Rows: []statsd.Row{{
			Metric: "aaa",
			Tags: []statsd.Tag{
				{
					Key:   "__statsd_metric_type__",
					Value: "c",
				},
				{
					Key:   "x",
					Value: "y",
				},
			},
			Values:    []float64{1123},
			Timestamp: int64(fasttime.UnixTimestamp()) * 1000,
		}},
	})
}
