package prompbmarshal

import (
	"fmt"
	"testing"
)

func BenchmarkWriteRequestMarshalProtobuf(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchWriteRequest.Timeseries)))
	b.RunParallel(func(pb *testing.PB) {
		var data []byte
		for pb.Next() {
			data = benchWriteRequest.MarshalProtobuf(data[:0])
		}
	})
}

var benchWriteRequest = func() *WriteRequest {
	var tss []TimeSeries
	for i := 0; i < 1_000; i++ {
		ts := TimeSeries{
			Labels: []Label{
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
			Samples: []Sample{
				{
					Value:     float64(i),
					Timestamp: 1e9 + int64(i)*1000,
				},
			},
		}
		tss = append(tss, ts)
	}
	wr := &WriteRequest{
		Timeseries: tss,
	}
	return wr
}()
