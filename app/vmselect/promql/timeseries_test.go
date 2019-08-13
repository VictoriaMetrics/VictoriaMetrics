package promql

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestMain(m *testing.M) {
	n := m.Run()
	os.Exit(n)
}

func TestTimeseriesMarshalUnmarshalFast(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		var tsOrig timeseries
		buf := tsOrig.marshalFastNoTimestamps(nil)
		n := tsOrig.marshaledFastSizeNoTimestamps()
		if n != len(buf) {
			t.Fatalf("unexpected marshaled size; got %d; want %d", n, len(buf))
		}

		var tsGot timeseries
		tail, err := tsGot.unmarshalFastNoTimestamps(buf)
		if err != nil {
			t.Fatalf("cannot unmarshal timeseries: %s", err)
		}
		if len(tail) > 0 {
			t.Fatalf("unexpected non-empty tail left: len(tail)=%d; tail=%X", len(tail), tail)
		}
		tsOrig.denyReuse = true
		tsOrig.MetricName.MetricGroup = []byte{}
		if !reflect.DeepEqual(&tsOrig, &tsGot) {
			t.Fatalf("unexpected ts\ngot:\n%s\nwant:\n%s", &tsGot, &tsOrig)
		}
	})
	t.Run("multiple", func(t *testing.T) {
		var dst []byte
		var tssOrig []*timeseries
		timestamps := []int64{2}
		for i := 0; i < 10; i++ {
			var ts timeseries
			ts.denyReuse = true
			ts.MetricName.MetricGroup = []byte(fmt.Sprintf("metricGroup %d", i))
			ts.MetricName.Tags = []storage.Tag{{
				Key:   []byte(fmt.Sprintf("key %d", i)),
				Value: []byte(fmt.Sprintf("value %d", i)),
			}}
			ts.Values = []float64{float64(i) + 0.2}
			ts.Timestamps = timestamps

			dstLen := len(dst)
			dst = ts.marshalFastNoTimestamps(dst)
			n := ts.marshaledFastSizeNoTimestamps()
			if n != len(dst)-dstLen {
				t.Fatalf("unexpected marshaled size on iteration %d; got %d; want %d", i, n, len(dst)-dstLen)
			}

			var tsGot timeseries
			tsGot.Timestamps = ts.Timestamps
			tail, err := tsGot.unmarshalFastNoTimestamps(dst[dstLen:])
			if err != nil {
				t.Fatalf("cannot unmarshal timeseries on iteration %d: %s", i, err)
			}
			if len(tail) > 0 {
				t.Fatalf("unexpected non-empty tail left on iteration %d: len(tail)=%d; tail=%x", i, len(tail), tail)
			}
			if !reflect.DeepEqual(&ts, &tsGot) {
				t.Fatalf("unexpected ts on iteration %d\ngot:\n%s\nwant:\n%s", i, &tsGot, &ts)
			}

			tssOrig = append(tssOrig, &ts)
		}
		buf := marshalTimeseriesFast(nil, tssOrig, 1e6, 123)
		tssGot, err := unmarshalTimeseriesFast(buf)
		if err != nil {
			t.Fatalf("error in unmarshalTimeseriesFast: %s", err)
		}
		if !reflect.DeepEqual(tssOrig, tssGot) {
			t.Fatalf("unexpected unmarshaled timeseries\ngot:\n%s\nwant:\n%s", tssGot, tssOrig)
		}

		src := dst
		for i := 0; i < 10; i++ {
			tsOrig := tssOrig[i]
			var ts timeseries
			ts.Timestamps = tsOrig.Timestamps
			tail, err := ts.unmarshalFastNoTimestamps(src)
			if err != nil {
				t.Fatalf("cannot unmarshal timeseries[%d]: %s", i, err)
			}
			src = tail
			if !reflect.DeepEqual(tsOrig, &ts) {
				t.Fatalf("unexpected ts on iteration %d:\n%+v\nwant:\n%+v", i, &ts, tsOrig)
			}
		}
		if len(src) > 0 {
			t.Fatalf("unexpected tail left; len(tail)=%d; tail=%X", len(src), src)
		}
	})
}
