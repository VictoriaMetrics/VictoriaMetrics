package storage

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func BenchmarkMetricIDCache_Get(b *testing.B) {
	f := func(b *testing.B, numMetricIDs, distance int64) {
		b.Helper()
		c := newMetricIDCache()
		metricIDMin := time.Now().UnixNano()
		metricID := metricIDMin
		for range numMetricIDs {
			c.Set(uint64(metricID))
			metricID += 1 + rand.Int63n(distance)
		}
		metricIDMax := metricID

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metricID := metricIDMin + rand.Int63n(metricIDMax-metricIDMin)
				_ = c.Has(uint64(metricID))
			}
		})
		b.ReportAllocs()
	}

	for _, numMetricIDs := range []int64{100_000, 1_000_000, 10_000_000} {
		for _, distance := range []int64{1, 10, 100} {
			name := fmt.Sprintf("numMetricID-%d/distance-%d", numMetricIDs, distance)
			b.Run(name, func(b *testing.B) {
				f(b, numMetricIDs, distance)
			})
		}
	}
}
