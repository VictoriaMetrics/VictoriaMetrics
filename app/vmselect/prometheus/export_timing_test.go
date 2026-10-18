package prometheus

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

type discardResponseWriter struct{}

func (discardResponseWriter) Header() http.Header         { return http.Header{} }
func (discardResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (discardResponseWriter) WriteHeader(int)             {}

func BenchmarkScalableWriter(b *testing.B) {
	const workers = 8

	mn := &storage.MetricName{
		MetricGroup: []byte("foo_bar_baz_total"),
		Tags: []storage.Tag{
			{Key: []byte("instance"), Value: []byte("foobarbaz:2344")},
			{Key: []byte("job"), Value: []byte("aaabbbccc")},
		},
	}

	f := func(samplesPerWorker int, release bool) {
		name := fmt.Sprintf("samples=%d/release=%v", samplesPerWorker, release)
		b.Run(name, func(b *testing.B) {
			xb := &exportBlock{
				mn:         mn,
				timestamps: make([]int64, 1000),
				values:     make([]float64, 1000),
			}
			for i := range xb.timestamps {
				xb.timestamps[i] = 1234567890000 + int64(i)*15000
				xb.values[i] = float64(i) * 1.37
			}
			linesPerWorker := samplesPerWorker / len(xb.timestamps)

			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				bw := bufferedwriter.Get(discardResponseWriter{})
				sw := newScalableWriter(bw)

				var wg sync.WaitGroup
				for w := uint(0); w < workers; w++ {
					wg.Add(1)
					go func(workerID uint) {
						defer wg.Done()
						for j := 0; j < linesPerWorker; j++ {
							bb := sw.getBuffer(workerID)
							WriteExportJSONLine(bb, xb)
							if err := sw.maybeFlushBuffer(bb); err != nil {
								panic(err)
							}
						}
					}(w)
				}
				wg.Wait()

				if err := sw.flush(); err != nil {
					b.Fatal(err)
				}
				if release {
					sw.release()
				}
				bufferedwriter.Put(bw)
			}
		})
	}

	for _, samples := range []int{10_000, 100_000, 1_000_000} {
		f(samples, false)
		f(samples, true)
	}
}
