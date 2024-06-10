package prompb

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func BenchmarkWriteRequestUnmarshalProtobuf(b *testing.B) {
	data := benchWriteRequest.MarshalProtobuf(nil)

	b.ReportAllocs()
	b.SetBytes(int64(len(benchWriteRequest.Timeseries)))
	b.RunParallel(func(pb *testing.PB) {
		var wr WriteRequest
		for pb.Next() {
			if err := wr.UnmarshalProtobuf(data); err != nil {
				panic(fmt.Errorf("unexpected error: %s", err))
			}
		}
	})
}

var benchWriteRequest = func() *prompbmarshal.WriteRequest {
	var tss []prompbmarshal.TimeSeries
	for i := 0; i < 10_000; i++ {
		ts := prompbmarshal.TimeSeries{
			Labels: []prompbmarshal.Label{
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
			Samples: []prompbmarshal.Sample{
				{
					Value:     float64(i),
					Timestamp: 1e9 + int64(i)*1000,
				},
			},
		}
		tss = append(tss, ts)
	}
	wrm := &prompbmarshal.WriteRequest{
		Timeseries: tss,
	}
	return wrm
}()
