package storage

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

type benchCacheState int

const (
	benchCacheStateCold benchCacheState = iota
	benchCacheStateWarm
	benchCacheStateRotated
)

var benchCacheStates = [...]benchCacheState{benchCacheStateCold, benchCacheStateWarm, benchCacheStateRotated}

func (s benchCacheState) String() string {
	return [...]string{"   cold", "   warm", "rotated"}[s]
}

func BenchmarkMetricIDCache_Has(b *testing.B) {
	f := func(b *testing.B, numMetricIDs, distance int64, hitsOnly bool, state benchCacheState) {
		b.Helper()
		c := newMetricIDCache()
		defer c.MustStop()

		warmUp := state == benchCacheStateWarm || state == benchCacheStateRotated
		rotate := state == benchCacheStateRotated

		metricIDMin := time.Now().UnixNano()
		metricIDMax := metricIDMin + numMetricIDs*distance
		for metricID := metricIDMin; metricID <= metricIDMax; metricID += distance {
			c.Set(uint64(metricID))
			if warmUp && !c.Has(uint64(metricID)) {
				b.Fatalf("metricID not in cache: %d", metricID)
			}
		}
		if rotate {
			c.rotate(rand.Intn(c.rotationGroupCount))
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
		b.ReportMetric(float64(c.Stats().SizeBytes), "sizeBytes")
	}

	subB := func(numMetricIDs, distance int64, hitsOnly bool, state benchCacheState) {
		hitsOrMisses := "  hits-only"
		if !hitsOnly {
			hitsOrMisses = "misses-only"
		}
		name := fmt.Sprintf("%s/%s/n%d/d%d", hitsOrMisses, state, numMetricIDs, distance)
		b.Run(name, func(b *testing.B) {
			f(b, numMetricIDs, distance, hitsOnly, state)
		})
	}
	for _, hitsOnly := range []bool{true, false} {
		for _, state := range benchCacheStates {
			for _, numMetricIDs := range []int64{100_000, 1_000_000, 10_000_000} {
				for _, distance := range []int64{1, 10, 100} {
					subB(numMetricIDs, distance, hitsOnly, state)
				}
			}
		}
	}
}
