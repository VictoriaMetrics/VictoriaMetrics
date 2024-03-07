package streamaggr

import (
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func BenchmarkDeduplicatorPush(b *testing.B) {
	pushFunc := func(tss []prompbmarshal.TimeSeries) {}
	d := NewDeduplicator(pushFunc, time.Hour, nil)

	b.ReportAllocs()
	b.SetBytes(int64(len(benchSeries)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d.Push(benchSeries)
		}
	})
}
