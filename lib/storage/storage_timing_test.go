package storage

import (
	"fmt"
	"io/fs"
	"os"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	vmfs "github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/google/go-cmp/cmp"
)

func BenchmarkStorageAddRows(b *testing.B) {
	defer vmfs.MustRemoveAll(b.Name())

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
					mn.AccountID = uint32(i)
					mn.ProjectID = uint32(i % 3)
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

		const (
			accountID = 2
			projectID = 3
			numRows   = 10_000
		)
		mrs := make([]MetricRow, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			AccountID: accountID,
			ProjectID: projectID,
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
		vmfs.MustRemoveAll(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}

func BenchmarkStorageSearchMetricNames_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const (
			accountID = 2
			projectID = 3
			numRows   = 10_000
		)
		mrs := make([]MetricRow, numRows)
		want := make([]string, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			AccountID: accountID,
			ProjectID: projectID,
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
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		tfss := NewTagFilters(accountID, projectID)
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
		vmfs.MustRemoveAll(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}

func BenchmarkStorageSearchLabelNames_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const (
			accountID = 2
			projectID = 3
			numRows   = 10_000
		)
		mrs := make([]MetricRow, numRows)
		want := make([]string, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			MetricGroup: []byte("metric"),
			AccountID:   accountID,
			ProjectID:   projectID,
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
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		var (
			got []string
			err error
		)

		for range b.N {
			got, err = s.SearchLabelNames(nil, accountID, projectID, nil, tr, 1e9, 1e9, noDeadline)
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
		vmfs.MustRemoveAll(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}

func BenchmarkStorageSearchLabelValues_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const (
			accountID = 2
			projectID = 3
			numRows   = 10_000
		)
		mrs := make([]MetricRow, numRows)
		want := make([]string, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			MetricGroup: []byte("metric"),
			AccountID:   accountID,
			ProjectID:   projectID,
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
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		var (
			got []string
			err error
		)
		for range b.N {
			got, err = s.SearchLabelValues(nil, accountID, projectID, "label", nil, tr, 1e9, 1e9, noDeadline)
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
		vmfs.MustRemoveAll(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}

func BenchmarkStorageSearchTagValueSuffixes_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const (
			accountID  = 2
			projectID  = 3
			numMetrics = 10_000
		)
		mn := MetricName{
			AccountID: accountID,
			ProjectID: projectID,
		}
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("prefix.metric%04d", i)
			mn.MetricGroup = []byte(name)
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = fmt.Sprintf("metric%04d", i)
		}
		slices.Sort(want)

		s := MustOpenStorage(b.Name(), OpenOptions{})
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		var (
			got []string
			err error
		)
		for range b.N {
			got, err = s.SearchTagValueSuffixes(nil, accountID, projectID, tr, "", "prefix.", '.', 1e9, noDeadline)
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
		vmfs.MustRemoveAll(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}

func BenchmarkStorageSearchGraphitePaths_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const (
			accountID  = 2
			projectID  = 3
			numMetrics = 10_000
		)
		mn := MetricName{
			AccountID: accountID,
			ProjectID: projectID,
		}
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("prefix.metric%04d", i)
			mn.MetricGroup = []byte(name)
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = name
		}
		slices.Sort(want)

		s := MustOpenStorage(b.Name(), OpenOptions{})
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		var (
			got []string
			err error
		)
		for range b.N {
			got, err = s.SearchGraphitePaths(nil, accountID, projectID, tr, []byte("*.*"), 1e9, noDeadline)
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
		vmfs.MustRemoveAll(b.Name())

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
		accountID       = 12
		projectID       = 34
	)

	// Each batch corresponds to a unique date and has a unique set of metric
	// names.
	highChurnRateData, _ := testGenerateMetricRowBatches(accountID, projectID, &batchOptions{
		numBatches:           numBatches,
		numRowsPerBatch:      numRowsPerBatch,
		sameBatchMetricNames: false, // Each batch has unique set of metric names.
		sameRowMetricNames:   false, // Within a batch, each metric name is unique.
		sameBatchDates:       false, // Each batch has a unique date.
		sameRowDates:         true,  // Within a batch, the date is the same.
	})

	// Each batch corresponds to a unique date but has the same set of metric
	// names.
	noChurnRateData, _ := testGenerateMetricRowBatches(accountID, projectID, &batchOptions{
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

			s.MustClose()
			vmfs.MustRemoveAll(path)
		}

		b.ReportMetric(float64(rowsAddedTotal)/float64(b.Elapsed().Seconds()), "rows/s")
		b.ReportMetric(float64(dataSize)/(1024*1024), "data-MiB")
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
