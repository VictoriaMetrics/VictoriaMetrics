package storage

import (
	"fmt"
	"io/fs"
	"os"
	"sync/atomic"
	"testing"

	vmfs "github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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
	s := MustOpenStorage(path, 0, 0, 0, false)
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
			s.AddRows(mrs, defaultPrecisionBits)
		}
	})
	b.StopTimer()
}

func BenchmarkStorageInsertVariousDataPatterns(b *testing.B) {
	defer vmfs.MustRemoveAll(b.Name())

	const (
		numBatches      = 100
		numRowsPerBatch = 10000
		concurrency     = 1
	)

	highChurnRateData, _ := testGenerateMetricRowBatches(&BatchOptions{
		numBatches:           numBatches,
		numRowsPerBatch:      numRowsPerBatch,
		sameBatchMetricNames: false,
		sameRowMetricNames:   false,
		sameBatchDates:       false,
		sameRowDates:         true,
	})

	noChurnRateData, _ := testGenerateMetricRowBatches(&BatchOptions{
		numBatches:           numBatches,
		numRowsPerBatch:      numRowsPerBatch,
		sameBatchMetricNames: true,
		sameRowMetricNames:   false,
		sameBatchDates:       false,
		sameRowDates:         true,
	})

	addRows := func(b *testing.B, disablePerDayIndexes bool, batches [][]MetricRow) {
		b.Helper()

		var (
			rowsAddedTotal int
			dataSize       int64
			indexSize      int64
		)

		path := b.Name()
		s := MustOpenStorage(path, 0, 0, 0, disablePerDayIndexes)
		defer s.MustClose()

		b.ResetTimer()
		for range b.N {
			splitBatches := true
			testDoConcurrently(s, func(s *Storage, mrs []MetricRow) {
				s.AddRows(mrs, defaultPrecisionBits)
			}, concurrency, splitBatches, batches)
			rowsAddedTotal += numBatches * numRowsPerBatch
		}
		b.StopTimer()

		s.DebugFlush()
		if err := s.ForceMergePartitions(""); err != nil {
			b.Fatalf("ForceMergePartitions() failed unexpectedly: %v", err)
		}

		dataSize = benchmarkDirSize(path + "/data")
		indexSize = benchmarkDirSize(path + "/indexdb")
		b.ReportMetric(float64(rowsAddedTotal)/float64(b.Elapsed().Seconds()), "rows/s")
		b.ReportMetric(float64(dataSize)/(1024*1024), "data-MiB")
		b.ReportMetric(float64(indexSize)/(1024*1024), "indexdb-MiB")
	}

	b.Run("HighChurnRate/perDayIndexes", func(b *testing.B) {
		addRows(b, false, highChurnRateData)
	})

	b.Run("HighChurnRate/noPerDayIndexes", func(b *testing.B) {
		addRows(b, true, highChurnRateData)
	})

	b.Run("NoChurnRate/perDayIndexes", func(b *testing.B) {
		addRows(b, false, noChurnRateData)
	})

	b.Run("NoChurnRate/noPerDayIndexes", func(b *testing.B) {
		addRows(b, true, noChurnRateData)
	})
}

// benchmarkDirSize calculates the size of a directory.
func benchmarkDirSize(path string) int64 {
	var size int64
	err := fs.WalkDir(os.DirFS(path), ".", func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			panic(err)
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				panic(err)
			}
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return size
}
