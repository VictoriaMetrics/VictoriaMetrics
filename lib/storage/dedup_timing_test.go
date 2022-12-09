package storage

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkDeduplicateSamples(b *testing.B) {
	const blockSize = 8192
	timestamps := make([]int64, blockSize)
	values := make([]float64, blockSize)
	for i := 0; i < len(timestamps); i++ {
		timestamps[i] = int64(i) * 1e3
		values[i] = float64(i)
	}
	for _, minScrapeInterval := range []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second} {
		b.Run(fmt.Sprintf("minScrapeInterval=%s", minScrapeInterval), func(b *testing.B) {
			dedupInterval := minScrapeInterval.Milliseconds()
			b.ReportAllocs()
			b.SetBytes(blockSize)
			b.RunParallel(func(pb *testing.PB) {
				timestampsCopy := make([]int64, 0, blockSize)
				valuesCopy := make([]float64, 0, blockSize)
				for pb.Next() {
					timestampsCopy := append(timestampsCopy[:0], timestamps...)
					valuesCopy := append(valuesCopy[:0], values...)
					ts, vs := DeduplicateSamples(timestampsCopy, valuesCopy, dedupInterval)
					if len(ts) == 0 || len(vs) == 0 {
						panic(fmt.Errorf("expecting non-empty results; got\nts=%v\nvs=%v", ts, vs))
					}
				}
			})
		})
	}
}

func BenchmarkDeduplicateSamplesDuringMerge(b *testing.B) {
	const blockSize = 8192
	timestamps := make([]int64, blockSize)
	values := make([]int64, blockSize)
	for i := 0; i < len(timestamps); i++ {
		timestamps[i] = int64(i) * 1e3
		values[i] = int64(i)
	}
	for _, minScrapeInterval := range []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second} {
		b.Run(fmt.Sprintf("minScrapeInterval=%s", minScrapeInterval), func(b *testing.B) {
			dedupInterval := minScrapeInterval.Milliseconds()
			b.ReportAllocs()
			b.SetBytes(blockSize)
			b.RunParallel(func(pb *testing.PB) {
				timestampsCopy := make([]int64, 0, blockSize)
				valuesCopy := make([]int64, 0, blockSize)
				for pb.Next() {
					timestampsCopy := append(timestampsCopy[:0], timestamps...)
					valuesCopy := append(valuesCopy[:0], values...)
					ts, vs := deduplicateSamplesDuringMerge(timestampsCopy, valuesCopy, dedupInterval)
					if len(ts) == 0 || len(vs) == 0 {
						panic(fmt.Errorf("expecting non-empty results; got\nts=%v\nvs=%v", ts, vs))
					}
				}
			})
		})
	}
}
