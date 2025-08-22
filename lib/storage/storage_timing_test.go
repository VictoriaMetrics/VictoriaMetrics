package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/google/go-cmp/cmp"
)

func BenchmarkStorageAddRows(b *testing.B) {
	defer fs.MustRemoveDir(b.Name())

	f := func(b *testing.B, numRows int) {
		b.Helper()

		s := MustOpenStorage(b.Name(), OpenOptions{})
		defer s.MustClose()

		var globalOffset atomic.Uint64

		b.SetBytes(int64(numRows))
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			mrs := make([]MetricRow, numRows)
			var mn MetricName
			mn.MetricGroup = []byte("rps")
			mn.Tags = []Tag{
				{[]byte("job"), []byte("webservice")},
				{[]byte("instance"), []byte("1.2.3.4")},
			}
			for pb.Next() {
				offset := int(globalOffset.Add(uint64(numRows)))
				for i := 0; i < numRows; i++ {
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

	for _, numRows := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("%d", numRows), func(b *testing.B) {
			f(b, numRows)
		})
	}
}

func BenchmarkStorageAddRows_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const numRows = 10_000
		mrs := make([]MetricRow, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			Tags: []Tag{
				{[]byte("job"), []byte("webservice")},
				{[]byte("instance"), []byte("1.2.3.4")},
			},
		}
		s := MustOpenStorage(b.Name(), OpenOptions{})

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		for n := range b.N {
			// Stop timer to exclude expensive initialization from measurement.
			b.StopTimer()
			for i := range numRows {
				mn.MetricGroup = []byte(fmt.Sprintf("metric_%d_%d", n, i))
				mrs[i].MetricNameRaw = mn.marshalRaw(nil)
				mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
				mrs[i].Value = float64(i)
			}
			b.StartTimer()

			s.AddRows(mrs, defaultPrecisionBits)
		}

		// Stop timer to exclude expensive cleanup from measurement.
		b.StopTimer()

		s.MustClose()
		fs.MustRemoveDir(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}

func BenchmarkStorageSearchMetricNames_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const numRows = 10_000
		mrs := make([]MetricRow, numRows)
		want := make([]string, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			Tags: []Tag{
				{[]byte("job"), []byte("webservice")},
				{[]byte("instance"), []byte("1.2.3.4")},
			},
		}
		for i := range numRows {
			name := fmt.Sprintf("metric_%d", i)
			mn.MetricGroup = []byte(name)
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = name
		}
		slices.Sort(want)
		s := MustOpenStorage(b.Name(), OpenOptions{})
		s.AddRows(mrs[:numRows/2], defaultPrecisionBits)
		// Rotate the indexDB to ensure that the search operation covers both current and prev indexDBs.
		s.mustRotateIndexDB(time.Now())
		s.AddRows(mrs[numRows/2:], defaultPrecisionBits)
		s.DebugFlush()

		tfss := NewTagFilters()
		if err := tfss.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
			b.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		var (
			got []string
			err error
		)
		for range b.N {
			got, err = s.SearchMetricNames(nil, []*TagFilters{tfss}, tr, 1e9, noDeadline)
			if err != nil {
				b.Fatalf("SearchMetricNames() failed unexpectedly: %v", err)
			}
		}

		// Stop timer to exclude expensive correctness check and cleanup from
		// measurement.
		b.StopTimer()

		for i, name := range got {
			var mn MetricName
			if err := mn.UnmarshalString(name); err != nil {
				b.Fatalf("Could not unmarshal metric name %q: %v", name, err)
			}
			got[i] = string(mn.MetricGroup)
		}
		slices.Sort(got)
		if diff := cmp.Diff(want, got); diff != "" {
			b.Errorf("unexpected metric names (-want, +got):\n%s", diff)
		}

		s.MustClose()
		fs.MustRemoveDir(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}

func BenchmarkStorageSearchLabelNames_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const numRows = 10_000
		mrs := make([]MetricRow, numRows)
		want := make([]string, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			MetricGroup: []byte("metric"),
			Tags: []Tag{
				{
					Key:   []byte("tbd"),
					Value: []byte("value"),
				},
			},
		}
		for i := range numRows {
			labelName := fmt.Sprintf("label_%d", i)
			mn.Tags[0].Key = []byte(labelName)
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = labelName
		}
		want = append(want, "__name__")
		slices.Sort(want)
		s := MustOpenStorage(b.Name(), OpenOptions{})
		s.AddRows(mrs[:numRows/2], defaultPrecisionBits)
		// Rotate the indexDB to ensure that the search operation covers both current and prev indexDBs.
		s.mustRotateIndexDB(time.Now())
		s.AddRows(mrs[numRows/2:], defaultPrecisionBits)
		s.DebugFlush()

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		var (
			got []string
			err error
		)

		for range b.N {
			got, err = s.SearchLabelNames(nil, nil, tr, 1e9, 1e9, noDeadline)
			if err != nil {
				b.Fatalf("SearchLabelNames() failed unexpectedly: %v", err)
			}
		}

		// Stop timer to exclude expensive correctness check and cleanup from
		// measurement.
		b.StopTimer()

		slices.Sort(got)
		if diff := cmp.Diff(want, got); diff != "" {
			b.Errorf("unexpected label names (-want, +got):\n%s", diff)
		}

		s.MustClose()
		fs.MustRemoveDir(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}

func BenchmarkStorageSearchLabelValues_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const numRows = 10_000
		mrs := make([]MetricRow, numRows)
		want := make([]string, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			MetricGroup: []byte("metric"),
			Tags: []Tag{
				{
					Key:   []byte("label"),
					Value: []byte("tbd"),
				},
			},
		}
		for i := range numRows {
			labelValue := fmt.Sprintf("value_%d", i)
			mn.Tags[0].Value = []byte(labelValue)
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = labelValue
		}
		slices.Sort(want)
		s := MustOpenStorage(b.Name(), OpenOptions{})
		s.AddRows(mrs[:numRows/2], defaultPrecisionBits)
		// Rotate the indexDB to ensure that the search operation covers both current and prev indexDBs.
		s.mustRotateIndexDB(time.Now())
		s.AddRows(mrs[numRows/2:], defaultPrecisionBits)
		s.DebugFlush()

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		var (
			got []string
			err error
		)
		for range b.N {
			got, err = s.SearchLabelValues(nil, "label", nil, tr, 1e9, 1e9, noDeadline)
			if err != nil {
				b.Fatalf("SearchLabelValues() failed unexpectedly: %v", err)
			}
		}

		// Stop timer to exclude expensive correctness check and cleanup from
		// measurement.
		b.StopTimer()

		slices.Sort(got)
		if diff := cmp.Diff(want, got); diff != "" {
			b.Errorf("unexpected label values (-want, +got):\n%s", diff)
		}

		s.MustClose()
		fs.MustRemoveDir(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}

func BenchmarkStorageSearchTagValueSuffixes_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const numMetrics = 10_000
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("prefix.metric%04d", i)
			mn := MetricName{MetricGroup: []byte(name)}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = fmt.Sprintf("metric%04d", i)
		}
		slices.Sort(want)

		s := MustOpenStorage(b.Name(), OpenOptions{})
		s.AddRows(mrs[:numMetrics/2], defaultPrecisionBits)
		// Rotate the indexDB to ensure that the search operation covers both current and prev indexDBs.
		s.mustRotateIndexDB(time.Now())
		s.AddRows(mrs[numMetrics/2:], defaultPrecisionBits)
		s.DebugFlush()

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		var (
			got []string
			err error
		)
		for range b.N {
			got, err = s.SearchTagValueSuffixes(nil, tr, "", "prefix.", '.', 1e9, noDeadline)
			if err != nil {
				b.Fatalf("SearchTagValueSuffixes() failed unexpectedly: %v", err)
			}
		}

		// Stop timer to exclude expensive correctness check and cleanup from
		// measurement.
		b.StopTimer()

		slices.Sort(got)
		if diff := cmp.Diff(want, got); diff != "" {
			b.Fatalf("unexpected tag value suffixes (-want, +got):\n%s", diff)
		}

		s.MustClose()
		fs.MustRemoveDir(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}

func BenchmarkStorageSearchGraphitePaths_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const numMetrics = 10_000
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("prefix.metric%04d", i)
			mn := MetricName{MetricGroup: []byte(name)}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = name
		}
		slices.Sort(want)

		s := MustOpenStorage(b.Name(), OpenOptions{})
		s.AddRows(mrs[:numMetrics/2], defaultPrecisionBits)
		// Rotate the indexDB to ensure that the search operation covers both current and prev indexDBs.
		s.mustRotateIndexDB(time.Now())
		s.AddRows(mrs[numMetrics/2:], defaultPrecisionBits)
		s.DebugFlush()

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		var (
			got []string
			err error
		)
		for range b.N {
			got, err = s.SearchGraphitePaths(nil, tr, []byte("*.*"), 1e9, noDeadline)
			if err != nil {
				b.Fatalf("SearchGraphitePaths() failed unexpectedly: %v", err)
			}
		}

		// Stop timer to exclude expensive correctness check and cleanup from
		// measurement.
		b.StopTimer()

		slices.Sort(got)
		if diff := cmp.Diff(want, got); diff != "" {
			b.Fatalf("unexpected graphite paths (-want, +got):\n%s", diff)
		}

		s.MustClose()
		fs.MustRemoveDir(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}

// benchmarkStorageOpOnVariousTimeRanges measures the execution time of some
// storage operation on various time ranges: 1h, 1d, 1m, etc.
func benchmarkStorageOpOnVariousTimeRanges(b *testing.B, op func(b *testing.B, tr TimeRange)) {
	b.Helper()

	b.Run("1h", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 1, 0, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("2h", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 1, 1, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("4h", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 1, 3, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("1d", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("2d", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 2, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("4d", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 4, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("1m", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("2m", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 2, 29, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("4m", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 3, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("1y", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 12, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("2y", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2001, 12, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("4y", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2003, 12, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
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
			fs.MustRemoveDir(path)
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
	err := filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
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

func BenchmarkSearchMetricNames_variableSeries(b *testing.B) {
	benchmarkSearch_variableSeries(b, benchmarkSearchMetricNames)
}

func BenchmarkSearchMetricNames_variableDeletedSeries(b *testing.B) {
	benchmarkSearch_variableDeletedSeries(b, benchmarkSearchMetricNames)
}

func BenchmarkSearchMetricNames_variableTimeRange(b *testing.B) {
	benchmarkSearch_variableTimeRange(b, benchmarkSearchMetricNames)
}

func BenchmarkSearchMetricNames_variableAll(b *testing.B) {
	benchmarkSearch_variableAll(b, benchmarkSearchMetricNames)
}

func BenchmarkSearchLabelNames_variableSeries(b *testing.B) {
	benchmarkSearch_variableSeries(b, benchmarkSearchLabelNames)
}

func BenchmarkSearchLabelNames_variableTimeRange(b *testing.B) {
	benchmarkSearch_variableTimeRange(b, benchmarkSearchLabelNames)
}

func BenchmarkSearchLabelNames_variableDeletedSeries(b *testing.B) {
	benchmarkSearch_variableDeletedSeries(b, benchmarkSearchLabelNames)
}

func BenchmarkSearchLabelNames_variableAll(b *testing.B) {
	benchmarkSearch_variableAll(b, benchmarkSearchLabelNames)
}

func BenchmarkSearchLabelValues_variableSeries(b *testing.B) {
	benchmarkSearch_variableSeries(b, benchmarkSearchLabelValues)
}

func BenchmarkSearchLabelValues_variableDeletedSeries(b *testing.B) {
	benchmarkSearch_variableDeletedSeries(b, benchmarkSearchLabelValues)
}

func BenchmarkSearchLabelValues_variableTimeRange(b *testing.B) {
	benchmarkSearch_variableTimeRange(b, benchmarkSearchLabelValues)
}

func BenchmarkSearchLabelValues_variableAll(b *testing.B) {
	benchmarkSearch_variableAll(b, benchmarkSearchLabelValues)
}

// benchmarkSearch_variableSeries measures the execution time of some search
// operation on a fixed time trange and variable number of series. The number of
// deleted series is 0.
func benchmarkSearch_variableSeries(b *testing.B, op func(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow)) {
	const numDeletedSeries = 0
	tr := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	for _, numSeries := range []int{
		1e2,
		1e3, 2e3, 3e3, 4e3, 5e3, 6e3, 7e3, 8e3, 9e3,
		1e4,
		1e5, 2e5, 3e5, 4e5, 5e5, 6e5, 7e5, 8e5, 9e5,
		1e6,
	} {
		name := fmt.Sprintf("%d", numSeries)
		b.Run(name, func(b *testing.B) {
			benchmarkSearch(b, numSeries, numDeletedSeries, tr, op)
		})
	}
}

// benchmarkSearch_variableDeletedSeries measures the execution time of some
// storage operation on a fixed time, fixed number of series and variable number
// of deleted series.
func benchmarkSearch_variableDeletedSeries(b *testing.B, op func(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow)) {
	tr := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	for _, numSeries := range []int{100, 1000, 10_000, 100_000, 1_000_000} {
		for _, numDeletedSeries := range []int{100, 1000, 10_000, 100_000, 1_000_000} {
			name := fmt.Sprintf("%d-%d", numSeries, numDeletedSeries)
			b.Run(name, func(b *testing.B) {
				benchmarkSearch(b, numSeries, numDeletedSeries, tr, op)
			})
		}
	}
}

func benchmarkSearch_variableDeletedSeries_ORIG(b *testing.B, op func(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow)) {
	const numSeries = 100
	tr := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	for _, numDeletedSeries := range []int{100, 1000, 10_000, 100_000} {
		name := fmt.Sprintf("%d", numDeletedSeries)
		b.Run(name, func(b *testing.B) {
			benchmarkSearch(b, numSeries, numDeletedSeries, tr, op)
		})
	}
}

// benchmarkSearch_variableTimeRange measures the execution time of some search
// operation on various time trages and fixed number of series. The number of
// deleted series is 0.
func benchmarkSearch_variableTimeRange(b *testing.B, op func(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow)) {
	const (
		numSeries        = 100
		numDeletedSeries = 0
	)
	tr1d := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	tr1w := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 7, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	tr1m := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	tr6m := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	trNames := []string{"1d", "1w", "1m", "6m"}
	for i, tr := range []TimeRange{tr1d, tr1w, tr1m, tr6m} {
		name := trNames[i]
		b.Run(name, func(b *testing.B) {
			benchmarkSearch(b, numSeries, numDeletedSeries, tr, op)
		})
	}
}

// benchmarkSearch_variableAll measures the execution time of some search
// operation on various time trages, with various number of series and various
// number of deleted series.
func benchmarkSearch_variableAll(b *testing.B, op func(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow)) {
	tr1d := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	tr1w := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 7, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	tr1m := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	tr6m := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	trNames := []string{"1h", "1w", "1m", "6m"}
	for i, tr := range []TimeRange{tr1d, tr1w, tr1m, tr6m} {
		for _, numSeries := range []int{100, 1000, 10_000, 100_000} {
			for _, numDeletedSeries := range []int{0, 100, 1000, 10_000, 100_000} {
				name := fmt.Sprintf("%s-%d-d%d", trNames[i], numSeries, numDeletedSeries)
				b.Run(name, func(b *testing.B) {
					benchmarkSearch(b, numSeries, numDeletedSeries, tr, op)
				})
			}
		}
	}
}

// benchmarkSearchMetricNames is a helper function used in various
// SearchMetricNames benchmarks.
func benchmarkSearchMetricNames(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow) {
	b.Helper()
	tfss := NewTagFilters()
	if err := tfss.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		b.Fatalf("unexpected error in TagFilters.Add: %v", err)
	}
	var (
		got []string
		err error
	)
	for b.Loop() {
		got, err = s.SearchMetricNames(nil, []*TagFilters{tfss}, tr, 1e9, noDeadline)
		if err != nil {
			b.Fatalf("SearchMetricNames() failed unexpectedly: %v", err)
		}
	}

	var mn MetricName
	for i, name := range got {
		if err := mn.UnmarshalString(name); err != nil {
			b.Fatalf("Could not unmarshal metric name %q: %v", name, err)
		}
		got[i] = string(mn.MetricGroup)
	}
	slices.Sort(got)
	want := make([]string, len(mrs))
	for i, mr := range mrs {
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			b.Fatalf("could not unmarshal metric row: %v", err)
		}
		want[i] = string(mn.MetricGroup)
	}
	slices.Sort(want)
	if diff := cmp.Diff(want, got); diff != "" {
		b.Fatalf("unexpected metric names (-want, +got):\n%s", diff)
	}
}

// benchmarkSearchLabelNames is a helper function used in various
// SearchLabelNames benchmarks.
func benchmarkSearchLabelNames(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow) {
	b.Helper()
	var (
		got []string
		err error
	)
	for b.Loop() {
		got, err = s.SearchLabelNames(nil, nil, tr, 1e9, 1e9, noDeadline)
		if err != nil {
			b.Fatalf("SearchLabelNames() failed unexpectedly: %v", err)
		}
	}
	slices.Sort(got)
	var mn MetricName
	want := make([]string, len(mrs))
	for i, mr := range mrs {
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			b.Fatalf("could not unmarshal metric row: %v", err)
		}
		for _, tag := range mn.Tags {
			labelName := string(tag.Key)
			if labelName != "label" {
				want[i] = labelName
			}
		}
	}
	want = append(want, "__name__", "label")
	slices.Sort(want)
	if diff := cmp.Diff(want, got); diff != "" {
		b.Fatalf("unexpected label names (-want, +got):\n%s", diff)
	}
}

// benchmarkSearchLabelValues is a helper function used in various
// SearchLabelValues benchmarks.
func benchmarkSearchLabelValues(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow) {
	b.Helper()
	var (
		got []string
		err error
	)
	for b.Loop() {
		got, err = s.SearchLabelValues(nil, "label", nil, tr, 1e9, 1e9, noDeadline)
		if err != nil {
			b.Fatalf("SearchLabelValues() failed unexpectedly: %v", err)
		}
	}
	slices.Sort(got)
	want := make([]string, len(mrs))
	for i, mr := range mrs {
		var mn MetricName
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			b.Fatalf("could not unmarshal metric row: %v", err)
		}
		for _, tag := range mn.Tags {
			if string(tag.Key) == "label" {
				want[i] = string(tag.Value)
			}
		}
	}
	slices.Sort(want)
	if diff := cmp.Diff(want, got); diff != "" {
		b.Fatalf("unexpected label values (-want, +got):\n%s", diff)
	}
}

// benchmarkSearch implements the core logic of benchmark of a search operation.
//
// It generates the test data, inserts it into the storage and runs the search
// operation against it. The index data is split evenly across prev and curr
// indexDBs.
//
// The number of series is controlled with numSeries.
//
// The function also generates the deleted series and saves them to the storage.
// If the deleted series are not needed, set numDeletedSeries to 0.
//
// The data is spread evenly across the provided time range.
//
// The test data is designed so that it can be reused by all types of search
// operations. It is also passes to the search op callback to that the search
// operation could perform all necessary assertions to make sure that the search
// result is correct.
func benchmarkSearch(b *testing.B, numSeries, numDeletedSeries int, tr TimeRange, op func(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow)) {
	b.Helper()
	genRows := func(n int, prefix string, tr TimeRange) []MetricRow {
		mrs := make([]MetricRow, n)
		if n == 0 {
			return mrs
		}
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(n)
		for i := range n {
			name := fmt.Sprintf("%s_%09d", prefix, i)
			labelName := fmt.Sprintf("%s_label_%09d", prefix, i)
			labelValue := fmt.Sprintf("%s_value_%09d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte(name),
				Tags: []Tag{
					{[]byte(labelName), []byte("value")},
					{[]byte("label"), []byte(labelValue)},
				},
			}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
		}
		return mrs
	}

	deleteSeries := func(s *Storage, prefix string, want int) {
		b.Helper()
		tfs := NewTagFilters()
		re := fmt.Sprintf(`%s.*`, prefix)
		if err := tfs.Add(nil, []byte(re), false, true); err != nil {
			b.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		got, err := s.DeleteSeries(nil, []*TagFilters{tfs}, 1e9)
		if err != nil {
			b.Fatalf("could not delete series unexpectedly: %v", err)
		}
		if got != want {
			b.Fatalf("unexpected number of deleted series: got %d, want %d", got, want)
		}

	}

	trPrev := TimeRange{
		MinTimestamp: tr.MinTimestamp,
		MaxTimestamp: tr.MinTimestamp + (tr.MaxTimestamp-tr.MinTimestamp)/2,
	}
	trCurr := TimeRange{
		MinTimestamp: tr.MinTimestamp + (tr.MaxTimestamp-tr.MinTimestamp)/2 + 1,
		MaxTimestamp: tr.MaxTimestamp,
	}

	numDeletedSeriesPrev := numDeletedSeries / 2
	mrsToDeletePrev := genRows(numDeletedSeriesPrev, "prev", trPrev)
	mrsPrev := genRows(numSeries/2, "prev", trPrev)
	numDeletedSeriesCurr := numDeletedSeries / 2
	mrsToDeleteCurr := genRows(numDeletedSeriesCurr, "curr", trCurr)
	mrsCurr := genRows(numSeries/2, "curr", trCurr)

	s := MustOpenStorage(b.Name(), OpenOptions{})
	s.AddRows(mrsToDeletePrev, defaultPrecisionBits)
	s.DebugFlush()
	deleteSeries(s, "prev", numDeletedSeriesPrev)
	s.DebugFlush()
	s.AddRows(mrsPrev, defaultPrecisionBits)
	s.DebugFlush()
	// Rotate the indexDB to ensure that the search operation covers both current and prev indexDBs.
	s.mustRotateIndexDB(time.Now())
	s.AddRows(mrsToDeleteCurr, defaultPrecisionBits)
	s.DebugFlush()
	deleteSeries(s, "curr", numDeletedSeriesCurr)
	s.DebugFlush()
	s.AddRows(mrsCurr, defaultPrecisionBits)
	s.DebugFlush()

	mrs := slices.Concat(mrsPrev, mrsCurr)
	op(b, s, tr, mrs)

	s.MustClose()
	_ = os.RemoveAll(b.Name())
}
