package storage

import (
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/google/go-cmp/cmp"
)

func BenchmarkStorageAddRows(b *testing.B) {
	defer fs.MustRemoveAll(b.Name())

	f := func(b *testing.B, numRows int) {
		b.Helper()

		s := MustOpenStorage(b.Name(), 0, 0, 0)
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
		s := MustOpenStorage(b.Name(), 0, 0, 0)

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
		fs.MustRemoveAll(b.Name())

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
		s := MustOpenStorage(b.Name(), 0, 0, 0)
		s.AddRows(mrs, defaultPrecisionBits)
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
		fs.MustRemoveAll(b.Name())

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
		s := MustOpenStorage(b.Name(), 0, 0, 0)
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		var (
			got []string
			err error
		)

		for range b.N {
			got, err = s.SearchLabelNamesWithFiltersOnTimeRange(nil, nil, tr, 1e9, 1e9, noDeadline)
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
		fs.MustRemoveAll(b.Name())

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
		s := MustOpenStorage(b.Name(), 0, 0, 0)
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		var (
			got []string
			err error
		)
		for range b.N {
			got, err = s.SearchLabelValuesWithFiltersOnTimeRange(nil, "label", nil, tr, 1e9, 1e9, noDeadline)
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
		fs.MustRemoveAll(b.Name())

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

		s := MustOpenStorage(b.Name(), 0, 0, 0)
		s.AddRows(mrs, defaultPrecisionBits)
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
		fs.MustRemoveAll(b.Name())

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
