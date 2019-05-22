package promql

import (
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

var (
	// Sink is a global sink for benchmarks.
	// It guarantees the compiler doesn't remove the code in benchmarks,
	// which writes data to the Sink.
	Sink float64

	// SinkLock locks Sink.
	SinkLock sync.Mutex
)

var benchValues = func() []float64 {
	values := make([]float64, 1000)
	for i := range values {
		values[i] = rand.Float64() * 100
	}
	return values
}()
