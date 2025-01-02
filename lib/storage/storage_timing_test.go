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

func BenchmarkStorageAddRows_variousRowNumbers(b *testing.B) {
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

func BenchmarkStorageAddRows_variousTimeRanges(b *testing.B) {
	defer fs.MustRemoveAll(b.Name())

	const numRows = 10000

	addRows := func(path string, i int, tr TimeRange) {
		mrs := make([]MetricRow, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			Tags: []Tag{
				{[]byte("job"), []byte("webservice")},
				{[]byte("instance"), []byte("1.2.3.4")},
			},
		}
		for m := range numRows {
			mn.MetricGroup = []byte(fmt.Sprintf("metric_%d_%d", i, m))
			mrs[m].MetricNameRaw = mn.marshalRaw(nil)
			mrs[m].Timestamp = tr.MinTimestamp + int64(m)*step
			mrs[m].Value = float64(m)
		}
		s := MustOpenStorage(path, 0, 0, 0)
		s.AddRows(mrs, defaultPrecisionBits)
		s.MustClose()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, func(b *testing.B, tr TimeRange) {
		b.Helper()
		for i := range b.N {
			addRows(b.Name(), i, tr)
		}
	})
}

func BenchmarkStorageSearchMetricNames(b *testing.B) {
	defer fs.MustRemoveAll(b.Name())

	addRowsThenSearchMetricNames := func(b *testing.B, numRows int, tr TimeRange) {
		b.Helper()
		mrs := make([]MetricRow, numRows)
		want := make([]string, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			Tags: []Tag{
				{[]byte("job"), []byte("webservice")},
				{[]byte("instance"), []byte("1.2.3.4")},
			},
		}
		for m := range numRows {
			name := fmt.Sprintf("metric_%d", m)
			mn.MetricGroup = []byte(name)
			mrs[m].MetricNameRaw = mn.marshalRaw(nil)
			mrs[m].Timestamp = tr.MinTimestamp + int64(m)*step
			mrs[m].Value = float64(m)
			want[m] = name
		}
		slices.Sort(want)
		s := MustOpenStorage(b.Name(), 0, 0, 0)
		defer s.MustClose()
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		tfss := NewTagFilters()
		if err := tfss.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
			b.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}

		var (
			got []string
			err error
		)
		b.ResetTimer()
		for range b.N {
			got, err = s.SearchMetricNames(nil, []*TagFilters{tfss}, tr, 1e9, noDeadline)
			if err != nil {
				b.Fatalf("SearchMetricNames() failed unexpectedly: %v", err)
			}
		}
		b.StopTimer()

		for i, name := range got {
			var mn MetricName
			if err := mn.UnmarshalString(name); err != nil {
				b.Fatalf("Could not unmarshal metric name %q: %v", name, err)
			}
			got[i] = string(mn.MetricGroup)
		}
		slices.Sort(got)
		if diff := cmp.Diff(got, want); diff != "" {
			b.Errorf("unexpected metric names (-want, +got):\n%s", diff)
		}
	}

	for _, numRows := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("%d-rows", numRows), func(b *testing.B) {
			benchmarkStorageOpOnVariousTimeRanges(b, func(b *testing.B, tr TimeRange) {
				b.Helper()
				addRowsThenSearchMetricNames(b, numRows, tr)
			})
		})
	}
}

// benchmarkStorageOpOnVariousTimeRanges measures the execution time of some
// storage operation on various time ranges: 1h, 1d, 1m, etc.
func benchmarkStorageOpOnVariousTimeRanges(b *testing.B, op func(b *testing.B, tr TimeRange)) {
	b.Helper()

	b.Run("1h", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC).UnixMilli(),
		})
	})
	b.Run("1d", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("1m", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("1y", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 12, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	b.Run("10y", func(b *testing.B) {
		op(b, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2009, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
}
