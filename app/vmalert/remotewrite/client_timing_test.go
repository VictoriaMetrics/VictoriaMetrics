package remotewrite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func BenchmarkClientFlushEndToEnd(b *testing.B) {
	srv := newRWServer()
	defer srv.Close()

	client, err := NewClient(context.Background(), Config{
		Addr:          srv.URL,
		MaxBatchSize:  10000,
		Concurrency:   1,
		MaxQueueSize:  100000,
		FlushInterval: time.Hour,
	})
	if err != nil {
		b.Fatalf("failed to create client: %s", err)
	}
	defer client.Close()

	const tsCount = 100000
	tss := make([]prompb.TimeSeries, tsCount)
	for i := range tss {
		tss[i] = prompb.TimeSeries{
			Labels:  []prompb.Label{{Name: "__name__", Value: fmt.Sprintf("metric_%d", i)}},
			Samples: []prompb.Sample{{Value: float64(i), Timestamp: 1000}},
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// WriteRequest is stack-allocated each iter; re-assigning tss is just a slice header copy
		wr := prompb.WriteRequest{Timeseries: tss}
		client.flush(context.Background(), &wr)
		// flush calls wr.Reset() via defer; restore the slice for next iteration
		wr.Timeseries = tss
	}
}
