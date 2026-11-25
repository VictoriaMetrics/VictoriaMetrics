package promql

import (
	"context"
	"math/rand"
	"sync"
	"testing"
)

func BenchmarkRollupAvg(b *testing.B) {
	rfa := &rollupFuncArg{
		values: benchValues,
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(benchValues)))
	b.RunParallel(func(pb *testing.PB) {
		var vSum float64
		for pb.Next() {
			vSum += rollupAvg(rfa)
		}
		SinkLock.Lock()
		Sink += vSum
		SinkLock.Unlock()
	})
}

// BenchmarkRollupDo measures the full per-series rollup loop, including timestamp lookups.
func BenchmarkRollupDo(b *testing.B) {
	for _, tc := range []struct {
		name   string
		window int64
	}{
		{"SmallWindow", 1_000},
		{"LargeWindow", 1_000_000},
	} {
		b.Run(tc.name, func(b *testing.B) {
			rc, values, timestamps := newBenchRollupConfig(tc.window)
			benchmarkRollupDo(b, rc, values, timestamps)
		})
	}
}

func newBenchRollupConfig(window int64) (*rollupConfig, []float64, []int64) {
	const points = 1024
	values := make([]float64, points)
	timestamps := make([]int64, points)
	for i := range values {
		values[i] = benchValues[i%len(benchValues)]
		timestamps[i] = int64(i) * 1_000
	}
	rc := &rollupConfig{
		Func:               rollupAvg,
		Start:              0,
		End:                timestamps[len(timestamps)-1],
		Step:               1_000,
		Window:             window,
		MaxPointsPerSeries: points,
		Timestamps:         timestamps,
	}
	return rc, values, timestamps
}

func benchmarkRollupDo(b *testing.B, rc *rollupConfig, values []float64, timestamps []int64) {
	dst := make([]float64, 0, len(rc.Timestamps))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst, _ = rc.Do(dst[:0], values, timestamps)
	}
	b.StopTimer()
	SinkLock.Lock()
	Sink += dst[len(dst)-1]
	SinkLock.Unlock()
}

// BenchmarkRollupDoWithContext measures the cost of checking a live query context.
func BenchmarkRollupDoWithContext(b *testing.B) {
	for _, tc := range []struct {
		name   string
		window int64
	}{
		{"SmallWindow", 1_000},
		{"LargeWindow", 1_000_000},
	} {
		b.Run(tc.name, func(b *testing.B) {
			rc, values, timestamps := newBenchRollupConfig(tc.window)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rc.ctx = ctx
			benchmarkRollupDo(b, rc, values, timestamps)
		})
	}
}

var (
	// Sink is a global sink for benchmarks.
	// It guarantees the compiler doesn't remove the code in benchmarks,
	// which writes data to the Sink.
	Sink float64

	// SinkLock locks Sink.
	SinkLock sync.Mutex
)

var benchValues = func() []float64 {
	r := rand.New(rand.NewSource(1))
	values := make([]float64, 1000)
	for i := range values {
		values[i] = r.Float64() * 100
	}
	return values
}()
