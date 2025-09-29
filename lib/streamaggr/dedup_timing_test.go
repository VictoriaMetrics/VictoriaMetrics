package streamaggr

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func BenchmarkDedupAggr(b *testing.B) {
	for _, samplesPerPush := range []int{1, 10, 100, 1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("samplesPerPush_%d", samplesPerPush), func(b *testing.B) {
			benchmarkDedupAggr(b, samplesPerPush)
		})
	}
}

func benchmarkDedupAggr(b *testing.B, samplesPerPush int) {
	flushSamples := func(samples []pushSample, _ int64, _ bool) {
		Sink.Add(uint64(len(samples)))
	}

	const loops = 2
	benchSamples := newBenchSamples(samplesPerPush)
	da := newDedupAggr()

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(samplesPerPush * loops))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < loops; i++ {
				da.pushSamples(benchSamples, 0, false)
			}
			da.flush(flushSamples, 0, false)
		}
	})
}

func newBenchSamples(count int) []pushSample {
	labels := []prompb.Label{
		{
			Name:  "app",
			Value: "app-123",
		},
		{
			Name:  "job",
			Value: "foo-bar-baz",
		},
		{
			Name:  "pod",
			Value: "pod-1-dsfdsf-dsfdsf",
		},
		{
			Name:  "namespace",
			Value: "ns-asdfdsfpfd-fddf",
		},
		{
			Name:  "__name__",
			Value: "process_cpu_seconds_total",
		},
	}
	labelsLen := len(labels)
	samples := make([]pushSample, count)
	var keyBuf []byte
	for i := range samples {
		sample := &samples[i]
		labels = append(labels[:labelsLen], prompb.Label{
			Name:  "app",
			Value: fmt.Sprintf("instance-%d", i),
		})
		keyBuf = compressLabels(keyBuf[:0], labels[:labelsLen], labels[labelsLen:])
		sample.key = string(keyBuf)
		sample.value = float64(i)
	}
	return samples
}

var Sink atomic.Uint64
