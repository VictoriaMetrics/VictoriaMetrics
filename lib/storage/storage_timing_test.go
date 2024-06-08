package storage

import (
	"fmt"
	"math/rand"
	"os"
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func BenchmarkStorageAddRows(b *testing.B) {
	for _, rowsPerBatch := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("rowsPerBatch_%d", rowsPerBatch), func(b *testing.B) {
			benchmarkStorageAddRows(b, rowsPerBatch)
		})
	}
}

func benchmarkStorageAddRows(b *testing.B, rowsPerBatch int) {
	path := fmt.Sprintf("BenchmarkStorageAddRows_%d", rowsPerBatch)
	s := MustOpenStorage(path, 0, 0, 0)
	defer func() {
		s.MustClose()
		if err := os.RemoveAll(path); err != nil {
			b.Fatalf("cannot remove storage at %q: %s", path, err)
		}
	}()

	var globalOffset atomic.Uint64

	b.SetBytes(int64(rowsPerBatch))
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		mrs := make([]MetricRow, rowsPerBatch)
		var mn MetricName
		mn.MetricGroup = []byte("rps")
		mn.Tags = []Tag{
			{[]byte("job"), []byte("webservice")},
			{[]byte("instance"), []byte("1.2.3.4")},
		}
		for pb.Next() {
			offset := int(globalOffset.Add(uint64(rowsPerBatch)))
			for i := 0; i < rowsPerBatch; i++ {
				mr := &mrs[i]
				mr.MetricNameRaw = mn.marshalRaw(mr.MetricNameRaw[:0])
				mr.Timestamp = int64(offset + i)
				mr.Value = float64(offset + i)
			}
			if err := s.AddRows(mrs, defaultPrecisionBits); err != nil {
				panic(fmt.Errorf("cannot add rows to storage: %w", err))
			}
		}
	})
	b.StopTimer()
}

func BenchmarkStorageAddHistoricalRowsConcurrently(b *testing.B) {
	defer fs.MustRemoveAll(b.Name())

	numBatches := 100
	numRowsPerBatch := 1000
	numRowsTotal := numBatches * numRowsPerBatch
	mrsBatches := make([][]MetricRow, numBatches)
	rng := rand.New(rand.NewSource(1))
	for batch := range numBatches {
		mrsBatches[batch] = testGenerateMetricRows(rng, uint64(numRowsPerBatch), int64(batch*1000), int64((batch+1)*1000-1))
	}
	tr := TimeRange{int64(0), int64(numBatches * numRowsPerBatch)}

	for _, concurrency := range []int{1, 2, 4, 8, 16, 32} {
		b.Run(fmt.Sprintf("%d", concurrency), func(b *testing.B) {
			var rowsAdded, slowInserts, nameCnt, idCnt int
			for range b.N {
				path := b.Name()
				fs.MustRemoveAll(path)
				s := MustOpenStorage(path, 0, 0, 0)
				testAddConcurrently(s, mrsBatches, concurrency, false)

				rowsAdded = numRowsTotal
				slowInserts = int(s.slowRowInserts.Load())
				nameCnt, idCnt = testCountAllMetricNamesAndIDs(s, tr)

				s.MustClose()
			}

			b.ReportMetric(float64(slowInserts), "slow-inserts")
			b.ReportMetric(float64(nameCnt), "ts-names")
			b.ReportMetric(float64(idCnt), "ts-ids")
			b.ReportMetric(float64(rowsAdded)/float64(b.Elapsed().Seconds()), "rows/s")
		})
	}
}
