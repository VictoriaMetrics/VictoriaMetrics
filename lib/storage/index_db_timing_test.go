package storage

import (
	"fmt"
	"regexp"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
)

func BenchmarkRegexpFilterMatch(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		re := regexp.MustCompile(`.*foo-bar-baz.*`)
		b := []byte("fdsffd foo-bar-baz assd fdsfad dasf dsa")
		for pb.Next() {
			if !re.Match(b) {
				panic("BUG: regexp must match!")
			}
			b[0]++
		}
	})
}

func BenchmarkRegexpFilterMismatch(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		re := regexp.MustCompile(`.*foo-bar-baz.*`)
		b := []byte("fdsffd foo-bar sfddsf assd nmn,mfdsdsakj")
		for pb.Next() {
			if re.Match(b) {
				panic("BUG: regexp mustn't match!")
			}
			b[0]++
		}
	})
}

func BenchmarkIndexDBAddTSIDs(b *testing.B) {
	const path = "BenchmarkIndexDBAddTSIDs"
	timestamp := time.Date(2025, 3, 17, 0, 0, 0, 0, time.UTC).UnixMilli()
	s := MustOpenStorage(path, OpenOptions{})
	ptw := s.tb.MustGetPartition(timestamp)
	db := ptw.pt.idb

	const recordsPerLoop = 1e3

	b.ReportAllocs()
	b.SetBytes(recordsPerLoop)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var mn MetricName
		var tsid TSID

		// The most common tags.
		mn.Tags = []Tag{
			{
				Key: []byte("job"),
			},
			{
				Key: []byte("instance"),
			},
		}

		startOffset := 0
		for pb.Next() {
			benchmarkIndexDBAddTSIDs(db, &tsid, &mn, timestamp, startOffset, recordsPerLoop)
			startOffset += recordsPerLoop
		}
	})
	b.StopTimer()

	s.tb.PutPartition(ptw)
	s.MustClose()
	fs.MustRemoveDir(path)
}

func benchmarkIndexDBAddTSIDs(db *indexDB, tsid *TSID, mn *MetricName, timestamp int64, startOffset, recordsPerLoop int) {
	date := uint64(timestamp) / msecPerDay
	for i := range recordsPerLoop {
		mn.MetricGroup = strconv.AppendUint(mn.MetricGroup[:0], uint64(i+startOffset), 10)
		for j := range mn.Tags {
			mn.Tags[j].Value = strconv.AppendUint(mn.Tags[j].Value[:0], uint64(i*j), 16)
		}
		mn.sortTags()

		generateTSID(tsid, mn)
		createAllIndexesForMetricName(db, mn, tsid, date)
	}
}

func BenchmarkHeadPostingForMatchers(b *testing.B) {
	// This benchmark is equivalent to https://github.com/prometheus/prometheus/blob/23c0299d85bfeb5d9b59e994861553a25ca578e5/tsdb/head_bench_test.go#L52
	// See https://www.robustperception.io/evaluating-performance-and-correctness for more details.
	const path = "BenchmarkHeadPostingForMatchers"
	timestamp := int64(0)
	s := MustOpenStorage(path, OpenOptions{})
	ptw := s.tb.MustGetPartition(timestamp)
	db := ptw.pt.idb

	// Fill the db with data as in https://github.com/prometheus/prometheus/blob/23c0299d85bfeb5d9b59e994861553a25ca578e5/tsdb/head_bench_test.go#L66
	var mn MetricName
	var tsid TSID
	date := uint64(timestamp) / msecPerDay
	addSeries := func(kvs ...string) {
		mn.Reset()
		for i := 0; i < len(kvs); i += 2 {
			mn.AddTag(kvs[i], kvs[i+1])
		}
		mn.sortTags()
		generateTSID(&tsid, &mn)
		createAllIndexesForMetricName(db, &mn, &tsid, date)
	}
	for n := range 10 {
		ns := strconv.Itoa(n)
		for i := range 100000 {
			ix := strconv.Itoa(i)
			addSeries("i", ix, "n", ns, "j", "foo")
			// Have some series that won't be matched, to properly test inverted matches.
			addSeries("i", ix, "n", ns, "j", "bar")
			addSeries("i", ix, "n", "0_"+ns, "j", "bar")
			addSeries("i", ix, "n", "1_"+ns, "j", "bar")
			addSeries("i", ix, "n", "2_"+ns, "j", "foo")
		}
	}

	// Make sure all the items can be searched.
	db.tb.DebugFlush()
	b.ResetTimer()

	benchSearch := func(b *testing.B, tfs *TagFilters, expectedMetricIDs int) {
		tfss := []*TagFilters{tfs}
		for range b.N {
			// Use special globalIndexDate to instruct indexDB to search global
			// index instead of per-day index.
			metricIDs, err := db.searchMetricIDsByDateAndFiltersSlow(noDeadlineContext, nil, globalIndexDate, tfss, 2e9)
			if err != nil {
				b.Fatalf("unexpected error in searchMetricIDs: %s", err)
			}
			if metricIDs.Len() != expectedMetricIDs {
				b.Fatalf("unexpected metricIDs found; got %d; want %d", metricIDs.Len(), expectedMetricIDs)
			}
		}
	}
	addTagFilter := func(tfs *TagFilters, key, value string, isNegative, isRegexp bool) {
		if err := tfs.Add([]byte(key), []byte(value), isNegative, isRegexp); err != nil {
			b.Fatalf("cannot add tag filter %q=%q, isNegative=%v, isRegexp=%v", key, value, isNegative, isRegexp)
		}
	}

	b.Run(`n="1"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "n", "1", false, false)
		benchSearch(b, tfs, 2e5)
	})
	b.Run(`n="1",j="foo"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "n", "1", false, false)
		addTagFilter(tfs, "j", "foo", false, false)
		benchSearch(b, tfs, 1e5)
	})
	b.Run(`j="foo",n="1"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "j", "foo", false, false)
		addTagFilter(tfs, "n", "1", false, false)
		benchSearch(b, tfs, 1e5)
	})
	b.Run(`n="1",j!="foo"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "n", "1", false, false)
		addTagFilter(tfs, "j", "foo", true, false)
		benchSearch(b, tfs, 1e5)
	})
	b.Run(`i=~".*"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "i", ".*", false, true)
		benchSearch(b, tfs, 0)
	})
	b.Run(`i=~".+"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "i", ".+", false, true)
		benchSearch(b, tfs, 5e6)
	})
	b.Run(`i=~""`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "i", "", false, true)
		benchSearch(b, tfs, 0)
	})
	b.Run(`i!=""`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "i", "", true, false)
		benchSearch(b, tfs, 5e6)
	})
	b.Run(`n="1",i=~".*",j="foo"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "n", "1", false, false)
		addTagFilter(tfs, "i", ".*", false, true)
		addTagFilter(tfs, "j", "foo", false, false)
		benchSearch(b, tfs, 1e5)
	})
	b.Run(`n="1",i=~".*",i!="2",j="foo"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "n", "1", false, false)
		addTagFilter(tfs, "i", ".*", false, true)
		addTagFilter(tfs, "i", "2", true, false)
		addTagFilter(tfs, "j", "foo", false, false)
		benchSearch(b, tfs, 1e5-1)
	})
	b.Run(`n="1",i!=""`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "n", "1", false, false)
		addTagFilter(tfs, "i", "", true, false)
		benchSearch(b, tfs, 2e5)
	})
	b.Run(`n="1",i!="",j="foo"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "n", "1", false, false)
		addTagFilter(tfs, "i", "", true, false)
		addTagFilter(tfs, "j", "foo", false, false)
		benchSearch(b, tfs, 1e5)
	})
	b.Run(`n="1",i=~".+",j="foo"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "n", "1", false, false)
		addTagFilter(tfs, "i", ".+", false, true)
		addTagFilter(tfs, "j", "foo", false, false)
		benchSearch(b, tfs, 1e5)
	})
	b.Run(`n="1",i=~"1.+",j="foo"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "n", "1", false, false)
		addTagFilter(tfs, "i", "1.+", false, true)
		addTagFilter(tfs, "j", "foo", false, false)
		benchSearch(b, tfs, 11110)
	})
	b.Run(`n="1",i=~".+",i!="2",j="foo"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "n", "1", false, false)
		addTagFilter(tfs, "i", ".+", false, true)
		addTagFilter(tfs, "i", "2", true, false)
		addTagFilter(tfs, "j", "foo", false, false)
		benchSearch(b, tfs, 1e5-1)
	})
	b.Run(`n="1",i=~".+",i!~"2.*",j="foo"`, func(b *testing.B) {
		tfs := NewTagFilters()
		addTagFilter(tfs, "n", "1", false, false)
		addTagFilter(tfs, "i", ".+", false, true)
		addTagFilter(tfs, "i", "2.*", true, true)
		addTagFilter(tfs, "j", "foo", false, false)
		benchSearch(b, tfs, 88889)
	})

	s.tb.PutPartition(ptw)
	s.MustClose()
	fs.MustRemoveDir(path)
}

func BenchmarkIndexDBGetTSIDs(b *testing.B) {
	const path = "BenchmarkIndexDBGetTSIDs"
	timestamp := time.Date(2025, 3, 17, 0, 0, 0, 0, time.UTC).UnixMilli()
	s := MustOpenStorage(path, OpenOptions{})
	ptw := s.tb.MustGetPartition(timestamp)
	db := ptw.pt.idb

	const recordsPerLoop = 1000
	const recordsCount = 1e5

	// Fill the db with recordsCount records.
	var mn MetricName
	mn.MetricGroup = []byte("rps")
	for i := range 2 {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		mn.AddTag(key, value)
	}
	mn.sortTags()

	var tsid TSID
	date := uint64(timestamp) / msecPerDay

	for range int(recordsCount) {
		generateTSID(&tsid, &mn)
		createAllIndexesForMetricName(db, &mn, &tsid, date)
	}
	db.tb.DebugFlush()

	b.SetBytes(recordsPerLoop)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var tsidLocal TSID
		var metricNameLocal []byte
		var mnLocal MetricName
		mnLocal.CopyFrom(&mn)
		mnLocal.sortTags()
		for pb.Next() {
			is := db.getIndexSearch()
			for i := range recordsPerLoop {
				metricNameLocal = mnLocal.Marshal(metricNameLocal[:0])
				if !is.getTSIDByMetricName(&tsidLocal, metricNameLocal, date) {
					panic(fmt.Errorf("cannot obtain tsid for row %d", i))
				}
			}
			db.putIndexSearch(is)
		}
	})
	b.StopTimer()

	s.tb.PutPartition(ptw)
	s.MustClose()
	fs.MustRemoveDir(path)
}

const (
	churnMetricGroup   = "high_churn_metric"
	bigPartMetricGroup = "big_part_metric"
)

func BenchmarkIndexDBHighChurnTSIDMiss(b *testing.B) {
	benchmarkIndexDBHighChurnTSIDMiss(b, "BenchmarkIndexDBHighChurnTSIDMiss")
}

func BenchmarkIndexDBHighChurnTSIDMissLowIndexCache(b *testing.B) {
	// Keep the index-block cache deliberately smaller than a single useful
	// working set. This makes the baseline exercise index block reads instead
	// of measuring only an in-memory Seek. Restore the normal default for other
	// benchmarks in the same process.
	defaultCacheSize := int(0.10 * float64(memory.Allowed()))
	mergeset.SetIndexBlocksCacheSize(1)
	b.Cleanup(func() {
		mergeset.SetIndexBlocksCacheSize(defaultCacheSize)
	})
	benchmarkIndexDBHighChurnTSIDMiss(b, "BenchmarkIndexDBHighChurnTSIDMissLowIndexCache")
}

func BenchmarkIndexDBHighChurnTSIDMissHeatMix(b *testing.B) {
	// Keep enough cache for a repeated hot lookup while cold lookups churn the
	// remaining index blocks. The cold percentage is varied below, so the same
	// benchmark shows where the filter's avoided reads start to dominate its
	// fixed hash cost.
	defaultCacheSize := int(0.10 * float64(memory.Allowed()))
	mergeset.SetIndexBlocksCacheSize(64 << 10)
	b.Cleanup(func() {
		mergeset.SetIndexBlocksCacheSize(defaultCacheSize)
	})
	for _, coldPercent := range []int{0, 1, 5, 10, 25, 50, 75, 100} {
		b.Run(fmt.Sprintf("cold=%d%%", coldPercent), func(b *testing.B) {
			benchmarkIndexDBHighChurnTSIDMissParts(b, "BenchmarkIndexDBHighChurnTSIDMissHeatMix", maxStablePartsCount, coldPercent)
		})
	}
}

func benchmarkIndexDBHighChurnTSIDMiss(b *testing.B, benchmarkName string) {
	for _, partsCount := range []int{1, 4, maxStablePartsCount} {
		b.Run(fmt.Sprintf("parts=%d", partsCount), func(b *testing.B) {
			benchmarkIndexDBHighChurnTSIDMissParts(b, benchmarkName, partsCount, 100)
		})
	}
}

// maxStablePartsCount is the largest part count which is stable for the merge
// policy used by mergeset. Merging file parts requires the merge multiplier to
// reach maxPartsToMerge/2, so 8 or more equal-sized parts are merged by the
// background merger at any time. Fewer parts stay as they are, which keeps the
// reported part count reproducible.
const maxStablePartsCount = 7

func benchmarkIndexDBHighChurnTSIDMissParts(b *testing.B, benchmarkName string, partsCount, coldPercent int) {
	path := fmt.Sprintf("%s-%d-%d", benchmarkName, partsCount, coldPercent)
	fs.MustRemoveDir(path)
	timestamp := time.Date(2025, 3, 17, 0, 0, 0, 0, time.UTC).UnixMilli()
	s := MustOpenStorage(path, OpenOptions{})
	closed := false
	b.Cleanup(func() {
		if !closed {
			s.MustClose()
		}
		fs.MustRemoveDir(path)
	})
	ptw := s.tb.MustGetPartition(timestamp)
	db := ptw.pt.idb
	date := uint64(timestamp) / msecPerDay

	const recordsCount = 50000
	var mn MetricName
	mn.MetricGroup = []byte(churnMetricGroup)
	mn.AddTag("pod", "")
	mn.sortTags()
	var tsid TSID
	recordsPerPart := recordsCount / partsCount
	for part := range partsCount {
		for i := range recordsPerPart {
			metricID := part*recordsPerPart + i
			mn.Tags[0].Value = strconv.AppendInt(mn.Tags[0].Value[:0], int64(metricID*2), 10)
			generateTSID(&tsid, &mn)
			createAllIndexesForMetricName(db, &mn, &tsid, date)
		}
		// Persist each batch independently to model index parts created over
		// time by high-churn ingestion.
		db.tb.DebugFlush()
		s.tb.PutPartition(ptw)
		s.MustClose()
		s = MustOpenStorage(path, OpenOptions{})
		ptw = s.tb.MustGetPartition(timestamp)
		db = ptw.pt.idb
	}
	// The background merger runs concurrently with the setup above, so wait
	// until the number of file parts settles and require the requested count.
	// Otherwise the reported part count silently changes with the machine load,
	// and rows become incomparable.
	deadline := time.Now().Add(10 * time.Second)
	stablePartsCount := uint64(0)
	stableSince := time.Now()
	for {
		var tm mergeset.TableMetrics
		db.tb.UpdateMetrics(&tm)
		if tm.FilePartsCount != stablePartsCount {
			stablePartsCount = tm.FilePartsCount
			stableSince = time.Now()
		} else if time.Since(stableSince) >= 250*time.Millisecond {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var tableMetrics mergeset.TableMetrics
	db.tb.UpdateMetrics(&tableMetrics)
	if tableMetrics.FilePartsCount != uint64(partsCount) {
		s.tb.PutPartition(ptw)
		s.MustClose()
		closed = true
		b.Fatalf("expected %d file parts; got %d. Adjust maxStablePartsCount or the merge policy assumptions", partsCount, tableMetrics.FilePartsCount)
	}
	if tableMetrics.TotalItemsCount() == 0 || tableMetrics.FilePartsCount == 0 {
		s.tb.PutPartition(ptw)
		s.MustClose()
		closed = true
		b.Fatalf("expected searchable file parts; got %d items and %d file parts", tableMetrics.TotalItemsCount(), tableMetrics.FilePartsCount)
	}
	b.Run("part-filter", func(b *testing.B) {
		benchmarkTSIDMissLookups(b, db, date, churnMetricGroup, recordsCount, coldPercent, &tableMetrics, true)
	})
	b.Run("index-scan", func(b *testing.B) {
		benchmarkTSIDMissLookups(b, db, date, churnMetricGroup, recordsCount, coldPercent, &tableMetrics, false)
	})

	s.tb.PutPartition(ptw)
	s.MustClose()
	closed = true
}

func BenchmarkStorageAddRowsTSIDLookup(b *testing.B) {
	for _, disableFilter := range []bool{false, true} {
		filterName := "part-filter"
		if disableFilter {
			filterName = "index-scan"
		}
		for _, seriesName := range []string{"new-series", "existing-series-cache-hit", "existing-series-cache-miss"} {
			b.Run(filterName+"/"+seriesName, func(b *testing.B) {
				benchmarkStorageAddRowsTSIDLookup(b, disableFilter, seriesName)
			})
		}
	}
}

func benchmarkStorageAddRowsTSIDLookup(b *testing.B, disableFilter bool, seriesName string) {
	existingSeries := seriesName != "new-series"
	existingSeriesCacheMiss := seriesName == "existing-series-cache-miss"
	path := b.Name()
	fs.MustRemoveDir(path)
	s := MustOpenStorage(path, OpenOptions{DisableTSIDLookupBloomFilter: disableFilter})
	b.Cleanup(func() {
		s.MustClose()
		fs.MustRemoveDir(path)
	})
	timestamp := time.Now().UnixMilli()
	ptw := s.tb.MustGetPartition(timestamp)
	db := ptw.pt.idb
	date := uint64(timestamp) / msecPerDay

	const initialSeriesCount = 20000
	const cacheMissBatchSize = 1000
	var mn MetricName
	mn.MetricGroup = []byte("high_churn_write")
	mn.AddTag("pod", "")
	mn.sortTags()
	var tsid TSID
	seedMetricNames := make([][]byte, cacheMissBatchSize)
	for i := range initialSeriesCount {
		mn.Tags[0].Value = strconv.AppendInt(mn.Tags[0].Value[:0], int64(i), 10)
		generateTSID(&tsid, &mn)
		createAllIndexesForMetricName(db, &mn, &tsid, date)
		if i < cacheMissBatchSize {
			seedMetricNames[i] = mn.marshalRaw(nil)
		}
	}
	db.tb.DebugFlush()

	if seriesName == "existing-series-cache-hit" {
		mn.Tags[0].Value = append(mn.Tags[0].Value[:0], "0"...)
	} else if !existingSeries {
		mn.Tags[0].Value = mn.Tags[0].Value[:0]
	}
	var row MetricRow
	row.Timestamp = timestamp
	row.Value = 1
	if seriesName == "existing-series-cache-hit" {
		row.MetricNameRaw = mn.marshalRaw(nil)
	} else if existingSeriesCacheMiss {
		row.MetricNameRaw = seedMetricNames[0]
	}
	// Reopen the storage so lookups use persisted index parts. Keep the same
	// workload split between new series, cache hits, and cache misses.
	s.tb.PutPartition(ptw)
	s.MustClose()
	s = MustOpenStorage(path, OpenOptions{DisableTSIDLookupBloomFilter: disableFilter})
	ptw = s.tb.MustGetPartition(timestamp)
	db = ptw.pt.idb
	if seriesName == "existing-series-cache-hit" {
		// Prime the TSID cache so this measures the common existing-series path.
		s.AddRows([]MetricRow{row}, defaultPrecisionBits)
	} else if existingSeriesCacheMiss {
		s.tsidCache.Reset()
	}

	s.rowsAddedTotal.Store(0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !existingSeries {
			mn.Tags[0].Value = strconv.AppendInt(mn.Tags[0].Value[:0], int64(initialSeriesCount+i), 10)
			row.MetricNameRaw = mn.marshalRaw(row.MetricNameRaw[:0])
		} else if existingSeriesCacheMiss {
			if i > 0 && i%cacheMissBatchSize == 0 {
				b.StopTimer()
				s.tsidCache.Reset()
				b.StartTimer()
			}
			row.MetricNameRaw = seedMetricNames[i%cacheMissBatchSize]
		}
		s.AddRows([]MetricRow{row}, defaultPrecisionBits)
	}
	b.StopTimer()
	b.ReportMetric(float64(s.rowsAddedTotal.Load())/float64(b.N), "rows/op")
	if !disableFilter && (!existingSeries || existingSeriesCacheMiss) {
		skipped := db.tsidLookupFilterSkipped.Load()
		falsePositives := db.tsidLookupFilterFalsePositives.Load()
		b.ReportMetric(float64(skipped)/float64(b.N), "filter-skips/op")
		possible := db.tsidLookupFilterPossible.Load()
		b.ReportMetric(float64(possible)/float64(b.N), "filter-possible/op")
		b.ReportMetric(float64(falsePositives)/float64(b.N), "filter-false-positives/op")
		b.ReportMetric(float64(b.N-int(skipped))/float64(b.N), "index-lookups/op")
	} else if !existingSeries || existingSeriesCacheMiss {
		b.ReportMetric(1, "index-lookups/op")
	}
	s.tb.PutPartition(ptw)
}

// benchmarkTSIDMissLookups measures lookups of absent metric names either with the part
// filter or with the regular index search.
//
// tableMetrics must contain the metrics snapshot taken before the lookup phase.
func benchmarkTSIDMissLookups(b *testing.B, db *indexDB, date uint64, metricGroup string, recordsCount uint64, coldPercent int, tableMetrics *mergeset.TableMetrics, useFilter bool) {
	b.Helper()
	db.s.disableTSIDLookupBloomFilter = !useFilter
	b.ReportAllocs()
	// Setup and the metric snapshots below walk all the parts, so keep them out
	// of the measured lookup time.
	b.StopTimer()
	b.ReportMetric(float64(tableMetrics.FilePartsCount), "parts")
	b.ReportMetric(float64(coldPercent), "cold-percent")
	b.ReportMetric(float64(tableMetrics.PartFilterFileSizeBytes), "filter-disk-bytes")
	filterSkippedBefore := db.tsidLookupFilterSkipped.Load()
	filterPossibleBefore := db.tsidLookupFilterPossible.Load()
	filterFalsePositiveBefore := db.tsidLookupFilterFalsePositives.Load()
	filterFallbackBefore := db.tsidLookupFilterFallbackUnfiltered.Load() + db.tsidLookupFilterFallbackBudget.Load() + db.tsidLookupFilterFallbackRead.Load() + db.tsidLookupFilterFallbackVerify.Load()
	var tableMetricsBefore mergeset.TableMetrics
	db.tb.UpdateMetrics(&tableMetricsBefore)
	b.StartTimer()
	// Give every worker a distinct sequence: sharing one would make the workers
	// repeat the same cold metric names and warm the shared caches for each other.
	var workerSeq atomic.Uint64
	b.RunParallel(func(pb *testing.PB) {
		is := db.getIndexSearch()
		defer db.putIndexSearch(is)
		var mnLocal MetricName
		mnLocal.MetricGroup = []byte(metricGroup)
		mnLocal.AddTag("pod", "")
		mnLocal.sortTags()
		var metricName []byte
		var tsidLocal TSID
		sequence := workerSeq.Add(1) * 0x9E3779B97F4A7C15
		iteration := 0
		for pb.Next() {
			iteration++
			metricID := uint64(1)
			if iteration%100 < coldPercent {
				// Use a deterministic pseudo-random sequence so the reusable
				// partSearch cursor cannot satisfy every cold lookup via tryFastSeek.
				sequence = sequence*6364136223846793005 + 1442695040888963407
				metricID = sequence % recordsCount
			}
			mnLocal.Tags[0].Value = strconv.AppendUint(mnLocal.Tags[0].Value[:0], metricID*2+1, 10)
			metricName = mnLocal.Marshal(metricName[:0])
			if is.getTSIDByMetricName(&tsidLocal, metricName, date) {
				panic("unexpected TSID found for absent metric name")
			}
		}
	})
	b.StopTimer()
	if useFilter && b.N > 0 {
		skipped := db.tsidLookupFilterSkipped.Load() - filterSkippedBefore
		b.ReportMetric(float64(skipped)/float64(b.N), "filter-skips/op")
		b.ReportMetric(float64(b.N-int(skipped))/float64(b.N), "index-lookups/op")
	} else if b.N > 0 {
		b.ReportMetric(1, "index-lookups/op")
	}
	b.ReportMetric(float64(db.tsidLookupFilterPossible.Load()-filterPossibleBefore)/float64(b.N), "filter-possible/op")
	b.ReportMetric(float64(db.tsidLookupFilterFalsePositives.Load()-filterFalsePositiveBefore)/float64(b.N), "filter-false-positives/op")
	b.ReportMetric(float64(db.tsidLookupFilterFallbackUnfiltered.Load()+db.tsidLookupFilterFallbackBudget.Load()+db.tsidLookupFilterFallbackRead.Load()+db.tsidLookupFilterFallbackVerify.Load()-filterFallbackBefore)/float64(b.N), "filter-fallbacks/op")
	var tableMetricsAfter mergeset.TableMetrics
	db.tb.UpdateMetrics(&tableMetricsAfter)
	b.ReportMetric(float64(tableMetricsAfter.IndexBlocksCacheRequests-tableMetricsBefore.IndexBlocksCacheRequests)/float64(b.N), "index-block-requests/op")
	b.ReportMetric(float64(tableMetricsAfter.IndexBlocksCacheMisses-tableMetricsBefore.IndexBlocksCacheMisses)/float64(b.N), "index-block-misses/op")
	b.ReportMetric(float64(tableMetricsAfter.PartFilterCacheRequests-tableMetricsBefore.PartFilterCacheRequests)/float64(b.N), "filter-cache-requests/op")
	b.ReportMetric(float64(tableMetricsAfter.PartFilterCacheMisses-tableMetricsBefore.PartFilterCacheMisses)/float64(b.N), "filter-cache-misses/op")
	b.ReportMetric(float64(tableMetricsAfter.PartFilterCacheSizeBytes), "filter-cache-bytes")
	b.ReportMetric(float64(tableMetricsAfter.PartFilterCacheSizeMaxBytes), "filter-cache-max-bytes")
}

// BenchmarkIndexDBBigPartTSIDMissHeatMix measures how the part filter size limit affects
// lookups of absent metric names in a part with millions of series, where the limit bites.
func BenchmarkIndexDBBigPartTSIDMissHeatMix(b *testing.B) {
	for _, tc := range []struct {
		name        string
		maxBytes    uint64
		maxSegments int
	}{
		{name: "limit=4MiB", maxBytes: 4 << 20, maxSegments: 64},
		{name: "limit=8MiB", maxBytes: 8 << 20, maxSegments: 128},
		{name: "limit=16MiB", maxBytes: 16 << 20, maxSegments: 256},
	} {
		b.Run(tc.name, func(b *testing.B) {
			restore := mergeset.SetPartFilterSizeLimit(tc.maxBytes, tc.maxSegments)
			defer restore()
			benchmarkIndexDBBigPartTSIDMiss(b, tc.name)
		})
	}
}

// bigPartRecordsCount is the number of series in the single part used by
// BenchmarkIndexDBBigPartTSIDMissHeatMix. It is the size of the biggest index part
// in the reported production deployment, where the previous 4 MiB filter limit
// dropped the number of bits per key.
const bigPartRecordsCount = 4_220_000

func benchmarkIndexDBBigPartTSIDMiss(b *testing.B, benchmarkName string) {
	// Keep the index-block cache much smaller than the part index, so the index scan
	// actually reads and decompresses index blocks for the cold lookups. Restore the
	// normal default for the other benchmarks in the same process.
	defaultCacheSize := int(0.10 * float64(memory.Allowed()))
	mergeset.SetIndexBlocksCacheSize(64 << 10)
	b.Cleanup(func() {
		mergeset.SetIndexBlocksCacheSize(defaultCacheSize)
	})

	path := fmt.Sprintf("%s-%d", benchmarkName, bigPartRecordsCount)
	fs.MustRemoveDir(path)
	timestamp := time.Date(2025, 3, 17, 0, 0, 0, 0, time.UTC).UnixMilli()
	s := MustOpenStorage(path, OpenOptions{})
	closed := false
	b.Cleanup(func() {
		if !closed {
			s.MustClose()
		}
		fs.MustRemoveDir(path)
	})
	ptw := s.tb.MustGetPartition(timestamp)
	db := ptw.pt.idb
	date := uint64(timestamp) / msecPerDay

	// Fill a single part, so its filter is limited by partFilterMaxBytes.
	var mn MetricName
	mn.MetricGroup = []byte(bigPartMetricGroup)
	mn.AddTag("pod", "")
	mn.sortTags()
	var tsid TSID
	for i := uint64(0); i < bigPartRecordsCount; i++ {
		mn.Tags[0].Value = strconv.AppendUint(mn.Tags[0].Value[:0], i*2, 10)
		generateTSID(&tsid, &mn)
		createAllIndexesForMetricName(db, &mn, &tsid, date)
	}
	db.tb.DebugFlush()
	s.tb.PutPartition(ptw)
	s.MustClose()
	s = MustOpenStorage(path, OpenOptions{})
	ptw = s.tb.MustGetPartition(timestamp)
	db = ptw.pt.idb

	// Wait until the background merger settles, so the reported part count and the
	// filter sizes are stable.
	deadline := time.Now().Add(30 * time.Second)
	stablePartsCount := uint64(0)
	stableSince := time.Now()
	for {
		var tm mergeset.TableMetrics
		db.tb.UpdateMetrics(&tm)
		if tm.FilePartsCount != stablePartsCount {
			stablePartsCount = tm.FilePartsCount
			stableSince = time.Now()
		} else if time.Since(stableSince) >= 500*time.Millisecond {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	var tableMetrics mergeset.TableMetrics
	db.tb.UpdateMetrics(&tableMetrics)
	if tableMetrics.FilePartsCount == 0 || tableMetrics.TotalItemsCount() == 0 {
		s.tb.PutPartition(ptw)
		s.MustClose()
		closed = true
		b.Fatalf("expected searchable file parts; got %d items and %d file parts", tableMetrics.TotalItemsCount(), tableMetrics.FilePartsCount)
	}

	for _, coldPercent := range []int{1, 5, 10, 100} {
		b.Run(fmt.Sprintf("cold=%d%%", coldPercent), func(b *testing.B) {
			b.Run("part-filter", func(b *testing.B) {
				benchmarkTSIDMissLookups(b, db, date, bigPartMetricGroup, bigPartRecordsCount, coldPercent, &tableMetrics, true)
			})
			b.Run("index-scan", func(b *testing.B) {
				benchmarkTSIDMissLookups(b, db, date, bigPartMetricGroup, bigPartRecordsCount, coldPercent, &tableMetrics, false)
			})
		})
	}

	s.tb.PutPartition(ptw)
	s.MustClose()
	closed = true
}
