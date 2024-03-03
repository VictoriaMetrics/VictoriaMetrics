package streamaggr

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkDedupAggr(b *testing.B) {
	for _, samplesPerPush := range []int{1, 10, 100, 1_000, 10_000, 100_000, 1_000_000} {
		b.Run(fmt.Sprintf("samplesPerPush_%d", samplesPerPush), func(b *testing.B) {
			benchmarkDedupAggr(b, samplesPerPush)
		})
	}
}

func benchmarkDedupAggr(b *testing.B, samplesPerPush int) {
	flushSamples := func(samples []pushSample) {
		Sink.Add(uint64(len(samples)))
	}

	const loops = 2
	benchSamples := newBenchSamples(samplesPerPush)
	da := newDedupAggr()

	b.ReportAllocs()
	b.SetBytes(int64(samplesPerPush * loops))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < loops; i++ {
				da.pushSamples(benchSamples)
			}
			da.flush(flushSamples)
		}
	})
}

func newBenchSamples(count int) []pushSample {
	samples := make([]pushSample, count)
	for i := range samples {
		sample := &samples[i]
		sample.key = fmt.Sprintf("key_%d", i)
		sample.value = float64(i)
	}
	return samples
}

var Sink atomic.Uint64
