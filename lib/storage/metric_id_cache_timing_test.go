package storage

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func BenchmarkMetricIDCache_Has(b *testing.B) {
	f := func(b *testing.B, numMetricIDs, distance int64, hitsOnly, warmUp, rotate bool) {
		b.Helper()
		c := newMetricIDCache()
		defer c.MustStop()
		metricIDMin := time.Now().UnixNano()
		metricIDMax := metricIDMin + numMetricIDs*distance
		for metricID := metricIDMin; metricID <= metricIDMax; metricID += distance {
			c.Set(uint64(metricID))
			if warmUp && !c.Has(uint64(metricID)) {
				b.Fatalf("metricID not in cache: %d", metricID)
			}
		}
		if rotate {
			c.rotate(0)
		}
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			if hitsOnly {
				metricID := metricIDMin + rand.Int63n(numMetricIDs)*distance
				for pb.Next() {
					if !c.Has(uint64(metricID)) {
						b.Fatalf("metricID not in cache: %d", metricID)
					}
					metricID += distance
					if metricID > metricIDMax {
						metricID = metricIDMin
					}
				}
			} else {
				// misses only
				metricID := metricIDMax + distance
				for pb.Next() {
					if c.Has(uint64(metricID)) {
						b.Fatalf("metricID is in cache: %d", metricID)
					}
					metricID += distance
				}
			}
		})
		b.ReportAllocs()
	}

	subB := func(numMetricIDs, distance int64, hitsOnly, warmUp, rotate bool) {
		hitsOrMisses := "hitsss"
		if !hitsOnly {
			hitsOrMisses = "misses"
		}
		coldOrWarm := "cold"
		if warmUp {
			coldOrWarm = "warm"
		}
		rotateOrNot := "not"
		if rotate {
			rotateOrNot = "rot"
		}
		name := fmt.Sprintf("%s/%s/%s/n%d/d%d", hitsOrMisses, coldOrWarm, rotateOrNot, numMetricIDs, distance)
		b.Run(name, func(b *testing.B) {
			f(b, numMetricIDs, distance, hitsOnly, warmUp, rotate)
		})
	}
	for _, hitsOnly := range []bool{true, false} {
		for _, warmUp := range []bool{false, true} {
			for _, numMetricIDs := range []int64{100_000, 1_000_000, 10_000_000} {
				for _, distance := range []int64{1, 10, 100} {
					for _, rotate := range []bool{true, false} {
						if !warmUp && rotate {
							// cannot rotate cold cache
							continue
						}
						subB(numMetricIDs, distance, hitsOnly, warmUp, rotate)
					}
				}
			}
		}
	}
}
