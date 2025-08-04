package streamaggr

import (
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func BenchmarkDeduplicatorPush(b *testing.B) {
	pushFunc := func(_ []prompb.TimeSeries) {}
	d := NewDeduplicator(pushFunc, true, time.Hour, nil, "global")

	b.ReportAllocs()
	b.SetBytes(int64(len(benchSeries)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d.Push(benchSeries)
		}
	})
}
