package storage

import (
	"fmt"
	"io/fs"
	"os"
	"slices"
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
	s := MustOpenStorage(path, OpenOptions{})
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

func BenchmarkStorageInsertWithAndWithoutPerDayIndex(b *testing.B) {
	const (
		numBatches      = 100
		numRowsPerBatch = 10000
		concurrency     = 1
		splitBatches    = true
	)

	// Each batch corresponds to a unique date and has a unique set of metric
	// names.
	highChurnRateData, _ := testGenerateMetricRowBatches(&batchOptions{
		numBatches:           numBatches,
		numRowsPerBatch:      numRowsPerBatch,
		sameBatchMetricNames: false, // Each batch has unique set of metric names.
		sameRowMetricNames:   false, // Within a batch, each metric name is unique.
		sameBatchDates:       false, // Each batch has a unique date.
		sameRowDates:         true,  // Within a batch, the date is the same.
	})

	// Each batch corresponds to a unique date but has the same set of metric
	// names.
	noChurnRateData, _ := testGenerateMetricRowBatches(&batchOptions{
		numBatches:           numBatches,
		numRowsPerBatch:      numRowsPerBatch,
		sameBatchMetricNames: true,  // Each batch has the same set of metric names.
		sameRowMetricNames:   false, // Within a batch, each metric name is unique.
		sameBatchDates:       false, // Each batch has a unique date.
		sameRowDates:         true,  // Within a batch, the date is the same.
	})

	addRows := func(b *testing.B, disablePerDayIndex bool, batches [][]MetricRow) {
		b.Helper()

		var (
			rowsAddedTotal int
			dataSize       int64
			indexSize      int64
		)

		path := b.Name()
		for range b.N {
			s := MustOpenStorage(path, OpenOptions{
				DisablePerDayIndex: disablePerDayIndex,
			})
			s.AddRows(slices.Concat(batches...), defaultPrecisionBits)
			s.DebugFlush()
			if err := s.ForceMergePartitions(""); err != nil {
				b.Fatalf("ForceMergePartitions() failed unexpectedly: %v", err)
			}

			// Reopen storage to ensure that index has been written to disk.
			s.MustClose()
			s = MustOpenStorage(path, OpenOptions{
				DisablePerDayIndex: disablePerDayIndex,
			})

			rowsAddedTotal = numBatches * numRowsPerBatch
			dataSize = benchmarkDirSize(path + "/data")
			indexSize = benchmarkDirSize(path + "/indexdb")

			s.MustClose()
			vmfs.MustRemoveAll(path)
		}

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
