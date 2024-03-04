package streamaggr

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func BenchmarkDedupAggr(b *testing.B) {
	for _, samplesPerPush := range []int{1, 10, 100, 1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("samplesPerPush_%d", samplesPerPush), func(b *testing.B) {
			benchmarkDedupAggr(b, samplesPerPush)
		})
	}
}

func BenchmarkDedupAggrFlushSerial(b *testing.B) {
	as := newLastAggrState()
	benchSamples := newBenchSamples(100_000)
	da := newDedupAggr()

	b.ReportAllocs()
	b.SetBytes(int64(len(benchSamples)))
	for i := 0; i < b.N; i++ {
		da.pushSamples(benchSamples)
		da.flush(as.pushSamples)
	}
}

func benchmarkDedupAggr(b *testing.B, samplesPerPush int) {
	const loops = 100
	benchSamples := newBenchSamples(samplesPerPush)
	da := newDedupAggr()

	b.ReportAllocs()
	b.SetBytes(int64(samplesPerPush * loops))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < loops; i++ {
				da.pushSamples(benchSamples)
			}
		}
	})
}

func newBenchSamples(count int) []pushSample {
	var lc promutils.LabelsCompressor
	labels := []prompbmarshal.Label{
		{
			Name:  "instance",
			Value: "host-123",
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
	samples := make([]pushSample, count)
	var keyBuf []byte
	for i := range samples {
		sample := &samples[i]
		labels[0].Value = fmt.Sprintf("host-%d", i)
		keyBuf = lc.Compress(keyBuf[:0], labels)
		sample.key = string(keyBuf)
		sample.value = float64(i)
	}
	return samples
}

var Sink atomic.Uint64
