package prompb

import (
	"fmt"
	"testing"
)

func BenchmarkUnmarshalProtobuf_ClassicSamples(b *testing.B) {
	var src []byte
	for i := 0; i < 1000; i++ {
		ts := encodeTimeSeries(
			[]Label{
				{
					Name:  "__name__",
					Value: "process_cpu_seconds_total",
				},
				{
					Name:  "instance",
					Value: fmt.Sprintf("host-%d:4567", i),
				},
				{
					Name:  "job",
					Value: "node-exporter",
				},
				{
					Name:  "pod",
					Value: "foo-bar-pod-8983423843",
				},
				{
					Name:  "cpu",
					Value: "1",
				},
				{
					Name:  "mode",
					Value: "system",
				},
				{
					Name:  "node",
					Value: "host-123",
				},
				{
					Name:  "namespace",
					Value: "foo-bar-baz",
				},
				{
					Name:  "container",
					Value: fmt.Sprintf("aaa-bb-cc-dd-ee-%d", i),
				},
			},
			[]Sample{{Value: float64(i), Timestamp: int64(i * 1000)}},
			nil,
		)
		src = pbAppendBytes(src, 1, ts)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		wru := GetWriteRequestUnmarshaler()
		if _, err := wru.UnmarshalProtobuf(src); err != nil {
			b.Fatal(err)
		}
		PutWriteRequestUnmarshaler(wru)
	}
}

func BenchmarkUnmarshalProtobuf_NativeHistogram(b *testing.B) {
	var src []byte
	histogram := encodeHistogram(nativeHistogramContext{
		countInt:       1072,
		sum:            1750000.5,
		schema:         0,
		zeroThreshold:  0.00001,
		zeroCountInt:   2,
		positiveSpans:  []bucketSpan{{offset: 0, length: 10}, {offset: 2, length: 10}},
		positiveDeltas: []int64{2, -1, 2, -1, 1, 1, 1, 100, 1, 1, 2, -1, -80, -1, 1, 1, 100, 1, 1, 1},
		timestamp:      1000,
	})
	for i := 0; i < 100; i++ {
		ts := encodeTimeSeries(
			[]Label{{Name: "__name__", Value: "rpc_latency_seconds"}, {Name: "job", Value: "node-exporter"}},
			nil,
			[][]byte{histogram},
		)
		src = pbAppendBytes(src, 1, ts)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		wru := GetWriteRequestUnmarshaler()
		if _, err := wru.UnmarshalProtobuf(src); err != nil {
			b.Fatal(err)
		}
		PutWriteRequestUnmarshaler(wru)
	}
}
