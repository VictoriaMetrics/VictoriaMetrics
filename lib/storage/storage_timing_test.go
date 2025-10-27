package storage

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
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

func BenchmarkStorageAddRows_VariousDataPatterns(b *testing.B) {
	f := func(b *testing.B, sameSeries, sameDate bool) {
		const numSeries = 1000
		rng := rand.New(rand.NewSource(1))
		start := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
		end := start + numSeries*msecPerDay - 1
		if sameDate {
			end = start + msecPerDay - 1
		}
		mrs := make([]MetricRow, numSeries)
		for i := range numSeries {
			name := fmt.Sprintf("metric_%09d", i)
			if sameSeries {
				name = "metric"
			}
			mn := MetricName{
				MetricGroup: []byte(name),
			}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = start + rng.Int63n(end-start)
			mrs[i].Value = rng.NormFloat64() * 1e6
		}

		b.ResetTimer()
		for i := range b.N {
			b.StopTimer()
			path := filepath.Join(b.Name(), fmt.Sprintf("%09d", i))
			s := MustOpenStorage(path, OpenOptions{})
			b.StartTimer()

			s.AddRows(mrs, defaultPrecisionBits)

			b.StopTimer()
			s.MustClose()
			b.StartTimer()
		}

		fs.MustRemoveDir(b.Name())
	}

	b.Run("SameSeries-SameDate", func(b *testing.B) {
		f(b, true, true)
	})
	b.Run("SameSeries-DiffDate", func(b *testing.B) {
		f(b, true, false)
	})
	b.Run("DiffSeries-SameDate", func(b *testing.B) {
		f(b, false, true)
	})
	b.Run("DiffSeries-DiffDate", func(b *testing.B) {
		f(b, false, false)
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

// BenchmarkSearch measures various storage search ops with various index and
// data configurations.
//
// Running all of the most probably won't make sense since it will take too long
// and it will be difficult to make any manual comparisons. However the
// benchmark can be useful when only a small subset of results is needed. For
// example, if one wants to compare the search of 100k metric names with
// different index configurations, then one would need to run the benchmark as
// follows:
//
// go test ./lib/storage --loggerLevel=ERROR -run=^$ -bench=^BenchmarkSearch/MetricNames/.*/VariableSeries/100000$
//
// For possible search ops, index configs, and data configs, see searchOpNames,
// indexConfigNames, and dataConfigs below.
func BenchmarkSearch(b *testing.B) {
	for i, search := range searchOps {
		b.Run(searchOpNames[i], func(b *testing.B) {
			for j, split := range indexConfigs {
				b.Run(indexConfigNames[j], func(b *testing.B) {
					for _, genDataConfig := range dataConfigGenerators {
						for _, dataConfig := range genDataConfig() {
							b.Run(dataConfig.name, func(b *testing.B) {
								benchmarkSearch(b, dataConfig, split, search)
							})
						}
					}
				})
			}
		})
	}
}

// dataConfig holds the test dataset config. Such as now many deleted and visible
// series to insert and on which time range.
type dataConfig struct {
	name             string
	numSeries        int
	numDeletedSeries int
	tr               TimeRange
}

// searchFunc is a func that measures the search of some data on the given time
// range. It also accepts the metric rows stored in the database to ensure that
// the search result is correct.
type searchFunc func(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow)

// splitFunc split the test data between prev and curr indexDBs.
type splitFunc func(total dataConfig) (prev, curr dataConfig)

// benchmarkSearch implements the core logic of benchmark of a search operation.
//
// It generates the test data, inserts it into the storage, and runs the search
// op against it.
//
// The number of series is controlled with dataConfig.numSeries. The function also
// generates the deleted series and saves them to the storage. If the deleted
// series are not needed, set dataConfig.numDeletedSeries to 0. The data is spread
// evenly across the time range provided via dataConfig.tr. The data is split
// across prev and curr idb using the split callback func.
//
// The generated data is designed so that it can be reused by all types of
// search ops. It is also passed to the search op so that it could perform all
// the necessary assertions to make sure that the search result is correct.
func benchmarkSearch(b *testing.B, dataConfig dataConfig, split splitFunc, search searchFunc) {
	b.Helper()
	graphitePrefix := ""
	if isGraphite(search) {
		graphitePrefix = "graphite."
	}
	genRows := func(n int, prefix string, tr TimeRange) []MetricRow {
		mrs := make([]MetricRow, n)
		if n == 0 {
			return mrs
		}
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(n)
		for i := range n {
			name := fmt.Sprintf("%s%s_%09d", graphitePrefix, prefix, i)
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
		re := fmt.Sprintf(`%s%s.*`, graphitePrefix, prefix)
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

	cfgPrev, cfgCurr := split(dataConfig)
	mrsToDeletePrev := genRows(cfgPrev.numDeletedSeries, "prev", cfgPrev.tr)
	mrsToDeleteCurr := genRows(cfgCurr.numDeletedSeries, "curr", cfgCurr.tr)
	mrsPrev := genRows(cfgPrev.numSeries, "prev", cfgPrev.tr)
	mrsCurr := genRows(cfgCurr.numSeries, "curr", cfgCurr.tr)

	s := MustOpenStorage(b.Name(), OpenOptions{})
	s.AddRows(mrsToDeletePrev, defaultPrecisionBits)
	s.DebugFlush()
	deleteSeries(s, "prev", cfgPrev.numDeletedSeries)
	s.DebugFlush()
	s.AddRows(mrsPrev, defaultPrecisionBits)
	s.DebugFlush()

	s.mustRotateIndexDB(time.Now())

	s.AddRows(mrsToDeleteCurr, defaultPrecisionBits)
	s.DebugFlush()
	deleteSeries(s, "curr", cfgCurr.numDeletedSeries)
	s.DebugFlush()
	s.AddRows(mrsCurr, defaultPrecisionBits)
	s.DebugFlush()

	mrs := slices.Concat(mrsPrev, mrsCurr)
	search(b, s, dataConfig.tr, mrs)

	s.MustClose()
	_ = os.RemoveAll(b.Name())
}

// searchOps is the collection of storage search ops for which BenchmarkSearch()
// will perform the measurements.
var searchOps = []searchFunc{
	benchmarkSearchData,
	benchmarkSearchMetricNames,
	benchmarkSearchLabelNames,
	benchmarkSearchLabelValues,
	benchmarkSearchTagValueSuffixes,
	benchmarkSearchGraphitePaths,
}
var searchOpNames = []string{
	"Data",
	"MetricNames",
	"LabelNames",
	"LabelValues",
	"TagValueSuffixes",
	"GraphitePaths",
}

// benchmarkSearchMetricNames measures the search of all metric names on the
// given time range. It also ensures that the search result is correct by
// comparing it with metric rows stored in the database.
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

// benchmarkSearchLabelNames measures the search of all label names on the
// given time range. It also ensures that the search result is correct by
// comparing it with metric rows stored in the database.
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

// benchmarkSearchLabelValues measures the search of all label values on the
// given time range. It also ensures that the search result is correct by
// comparing it with metric rows stored in the database.
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

// benchmarkSearchTagValueSuffixes measures the search of all tag value suffixes
// on the given time range. It also ensures that the search result is correct by
// comparing it with metric rows stored in the database.
func benchmarkSearchTagValueSuffixes(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow) {
	b.Helper()
	var (
		prefix = "graphite."
		got    []string
		err    error
	)
	for b.Loop() {
		got, err = s.SearchTagValueSuffixes(nil, tr, "", prefix, '.', 1e9, noDeadline)
		if err != nil {
			b.Fatalf("SearchTagValueSuffixes() failed unexpectedly: %v", err)
		}
	}
	slices.Sort(got)
	want := make([]string, len(mrs))
	for i, mr := range mrs {
		var mn MetricName
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			b.Fatalf("could not unmarshal metric row: %v", err)
		}
		var found bool
		metricName := string(mn.MetricGroup)
		want[i], found = strings.CutPrefix(metricName, prefix)
		if !found {
			b.Fatalf("metric name %q does not have %q prefix", metricName, prefix)
		}
	}
	slices.Sort(want)
	if diff := cmp.Diff(want, got); diff != "" {
		b.Fatalf("unexpected tag value suffixes (-want, +got):\n%s", diff)
	}
}

// benchmarkSearchGraphitePaths measures the search of all graphite paths on the
// given time range. It also ensures that the search result is correct by
// comparing it with metric rows stored in the database.
func benchmarkSearchGraphitePaths(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow) {
	b.Helper()
	var (
		got []string
		err error
	)
	for b.Loop() {
		got, err = s.SearchGraphitePaths(nil, tr, []byte("*.*"), 1e9, noDeadline)
		if err != nil {
			b.Fatalf("SearchGraphitePaths() failed unexpectedly: %v", err)
		}
	}
	slices.Sort(got)
	want := make([]string, len(mrs))
	for i, mr := range mrs {
		var mn MetricName
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			b.Fatalf("could not unmarshal metric row: %v", err)
		}
		want[i] = string(mn.MetricGroup)
	}
	slices.Sort(want)
	if diff := cmp.Diff(want, got); diff != "" {
		b.Fatalf("unexpected graphite paths (-want, +got):\n%s", diff)
	}
}

// isGraphite returns true if a given belongs to the Graphite API.
func isGraphite(op searchFunc) bool {
	opPtr := reflect.ValueOf(op).Pointer()
	searchTagValueSuffixesPtr := reflect.ValueOf(benchmarkSearchTagValueSuffixes).Pointer()
	searchGraphitePathsPtr := reflect.ValueOf(benchmarkSearchGraphitePaths).Pointer()
	return opPtr == searchTagValueSuffixesPtr || opPtr == searchGraphitePathsPtr
}

// indexConfigs holds the index configurations for which BenchmarkSearch() will
// perform the measurements.
var indexConfigs = []splitFunc{prevOnly, currOnly, prevCurr}
var indexConfigNames = []string{"PrevOnly", "CurrOnly", "PrevCurr"}

// prevOnly is an index config func that puts all index data into prev indexDB.
// No index data goes to curr indexDB.
//
// This config corresponds to a state when indexDBs have just been rotated.
// I.e. most of the index entries are in the prev indexDB.
func prevOnly(total dataConfig) (prev, curr dataConfig) {
	prev = total
	return prev, curr
}

// currOnly is an index config func that puts all index data into curr
// indexDB. No index data goes to prev indexDB.
//
// This config corresponds to a state when indexDBs haven't been rotated yet or
// rotated long time ago. I.e. most of the index entries are in the curr
// indexDB.
func currOnly(total dataConfig) (prev, curr dataConfig) {
	curr = total
	return prev, curr
}

// prevCurr is an index config func that splits index data evenly between
// prev and curr indexDBs.
//
// This config corresponds to a state when the indexDB rotation has happened
// some time ago. I.e. index entries are in both prev and curr indexDBs.
func prevCurr(total dataConfig) (prev, curr dataConfig) {
	prev.numSeries = total.numSeries / 2
	prev.numDeletedSeries = total.numDeletedSeries / 2
	prev.tr.MinTimestamp = total.tr.MinTimestamp
	prev.tr.MaxTimestamp = total.tr.MinTimestamp + (total.tr.MaxTimestamp-total.tr.MinTimestamp)/2

	curr.numSeries = total.numSeries - prev.numSeries
	curr.numDeletedSeries = total.numDeletedSeries - prev.numDeletedSeries
	curr.tr.MinTimestamp = prev.tr.MaxTimestamp + 1
	curr.tr.MaxTimestamp = total.tr.MaxTimestamp

	return prev, curr
}

// dataConfigFunc generates a collection of data configs. For example, various
// numbers of timeseries, deleted timeseries, and/or time ranges of various
// durations.
type dataConfigGenerator func() []dataConfig

// dataConfigGenerators is the collection of funcs that generate data configs
// for which BenchmarkSearch() will perform the measurements.
var dataConfigGenerators = []dataConfigGenerator{
	variableSeries,
	variableDeletedSeries,
	variableTimeRange,
}

// variableSeries generates a collection of data configs with variable number of
// series, 0 deleted series, and fixed time range (1d).
func variableSeries() []dataConfig {
	var cfgs []dataConfig
	// Using only a few numbers that represent orders of magnitude so that
	// routine running of the benchmarks does not take too long. However, when
	// debugging it is often helpful to add more numbers in between these
	// numbers.
	for _, numSeries := range []int{100, 1000, 10_000, 100_000, 1_000_000} {
		cfgs = append(cfgs, dataConfig{
			name:             fmt.Sprintf("VariableSeries/%d", numSeries),
			numSeries:        numSeries,
			numDeletedSeries: 0,
			tr: TimeRange{
				MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
				MaxTimestamp: time.Date(2025, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
			},
		})
	}
	return cfgs
}

// variableDeletedSeries generates a collection of data configs with fixed number of
// series (100k), variable number of deleted series, and fixed time range (1d).
//
// Why 100k: the eployments that we are aware of often have tens and hundreds of
// thouthands series in their query results, sometimes even millions. Chosen
// 100K as something in the middle.
func variableDeletedSeries() []dataConfig {
	var cfgs []dataConfig
	for _, numDeletedSeries := range []int{100, 1000, 10_000, 100_000, 1_000_000} {
		cfgs = append(cfgs, dataConfig{
			name:             fmt.Sprintf("VariableDeletedSeries/%d", numDeletedSeries),
			numSeries:        100_000,
			numDeletedSeries: numDeletedSeries,
			tr: TimeRange{
				MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
				MaxTimestamp: time.Date(2025, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
			},
		})
	}
	return cfgs
}

// variableTimeRange generates a collection of data configs with fixed number of
// series (100k), 0 deleted series, and time ranges of various duration.
func variableTimeRange() []dataConfig {
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
	tr2m := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 2, 28, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	tr6m := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	trNames := []string{"1d", "1w", "1m", "2m", "6m"}
	var cfgs []dataConfig
	for i, tr := range []TimeRange{tr1d, tr1w, tr2m, tr1m, tr6m} {
		cfgs = append(cfgs, dataConfig{
			name:             fmt.Sprintf("VariableTimeRange/%s", trNames[i]),
			numSeries:        100_000,
			numDeletedSeries: 0,
			tr:               tr,
		})
	}
	return cfgs
}
