package storage

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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
	s := MustOpenStorage(path, retentionMax, 0, 0)
	db := s.idb()

	const recordsPerLoop = 1e3

	b.ReportAllocs()
	b.SetBytes(recordsPerLoop)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var mn MetricName
		var genTSID generationTSID

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
			benchmarkIndexDBAddTSIDs(db, &genTSID, &mn, startOffset, recordsPerLoop)
			startOffset += recordsPerLoop
		}
	})
	b.StopTimer()

	s.MustClose()
	fs.MustRemoveAll(path)
}

func benchmarkIndexDBAddTSIDs(db *indexDB, genTSID *generationTSID, mn *MetricName, startOffset, recordsPerLoop int) {
	date := uint64(0)
	is := db.getIndexSearch(noDeadline)
	defer db.putIndexSearch(is)
	for i := 0; i < recordsPerLoop; i++ {
		mn.MetricGroup = strconv.AppendUint(mn.MetricGroup[:0], uint64(i+startOffset), 10)
		for j := range mn.Tags {
			mn.Tags[j].Value = strconv.AppendUint(mn.Tags[j].Value[:0], uint64(i*j), 16)
		}
		mn.sortTags()

		generateTSID(&genTSID.TSID, mn)
		createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
	}
}

func BenchmarkHeadPostingForMatchers(b *testing.B) {
	// This benchmark is equivalent to https://github.com/prometheus/prometheus/blob/23c0299d85bfeb5d9b59e994861553a25ca578e5/tsdb/head_bench_test.go#L52
	// See https://www.robustperception.io/evaluating-performance-and-correctness for more details.
	const path = "BenchmarkHeadPostingForMatchers"
	s := MustOpenStorage(path, retentionMax, 0, 0)
	db := s.idb()

	// Fill the db with data as in https://github.com/prometheus/prometheus/blob/23c0299d85bfeb5d9b59e994861553a25ca578e5/tsdb/head_bench_test.go#L66
	is := db.getIndexSearch(noDeadline)
	defer db.putIndexSearch(is)
	var mn MetricName
	var genTSID generationTSID
	date := uint64(0)
	addSeries := func(kvs ...string) {
		mn.Reset()
		for i := 0; i < len(kvs); i += 2 {
			mn.AddTag(kvs[i], kvs[i+1])
		}
		mn.sortTags()
		generateTSID(&genTSID.TSID, &mn)
		createAllIndexesForMetricName(is, &mn, &genTSID.TSID, date)
	}
	for n := 0; n < 10; n++ {
		ns := strconv.Itoa(n)
		for i := 0; i < 100000; i++ {
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
	db.s.DebugFlush()
	b.ResetTimer()

	benchSearch := func(b *testing.B, tfs *TagFilters, expectedMetricIDs int) {
		tfss := []*TagFilters{tfs}
		tr := TimeRange{
			MinTimestamp: 0,
			MaxTimestamp: timestampFromTime(time.Now()),
		}
		for i := 0; i < b.N; i++ {
			is := db.getIndexSearch(noDeadline)
			metricIDs, err := is.searchMetricIDs(nil, tfss, tr, 2e9)
			db.putIndexSearch(is)
			if err != nil {
				b.Fatalf("unexpected error in searchMetricIDs: %s", err)
			}
			if len(metricIDs) != expectedMetricIDs {
				b.Fatalf("unexpected metricIDs found; got %d; want %d", len(metricIDs), expectedMetricIDs)
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

	s.MustClose()
	fs.MustRemoveAll(path)
}

func BenchmarkIndexDBGetTSIDs(b *testing.B) {
	const path = "BenchmarkIndexDBGetTSIDs"
	s := MustOpenStorage(path, retentionMax, 0, 0)
	db := s.idb()

	const recordsPerLoop = 1000
	const recordsCount = 1e5

	// Fill the db with recordsCount records.
	var mn MetricName
	mn.MetricGroup = []byte("rps")
	for i := 0; i < 2; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		mn.AddTag(key, value)
	}
	mn.sortTags()

	var genTSID generationTSID
	date := uint64(12345)

	is := db.getIndexSearch(noDeadline)
	defer db.putIndexSearch(is)

	for i := 0; i < recordsCount; i++ {
		generateTSID(&genTSID.TSID, &mn)
		createAllIndexesForMetricName(is, &mn, &genTSID.TSID, date)
	}
	db.s.DebugFlush()

	b.SetBytes(recordsPerLoop)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var genTSIDLocal generationTSID
		var metricNameLocal []byte
		var mnLocal MetricName
		mnLocal.CopyFrom(&mn)
		mnLocal.sortTags()
		for pb.Next() {
			is := db.getIndexSearch(noDeadline)
			for i := 0; i < recordsPerLoop; i++ {
				metricNameLocal = mnLocal.Marshal(metricNameLocal[:0])
				if !is.getTSIDByMetricName(&genTSIDLocal, metricNameLocal, date) {
					panic(fmt.Errorf("cannot obtain tsid for row %d", i))
				}
			}
			db.putIndexSearch(is)
		}
	})
	b.StopTimer()

	s.MustClose()
	fs.MustRemoveAll(path)
}

func BenchmarkMarshalUnmarshalMetricIDs(b *testing.B) {
	rng := rand.New(rand.NewSource(1))

	f := func(b *testing.B, numMetricIDs int) {
		metricIDs := make([]uint64, numMetricIDs)
		// metric IDs need to be sorted.
		ts := uint64(time.Now().UnixNano())
		for i := range numMetricIDs {
			metricIDs[i] = ts + uint64(rng.Intn(100))
		}

		var marshalledLen int
		b.ResetTimer()
		for range b.N {
			marshalled := marshalMetricIDs(nil, metricIDs)
			marshalledLen = len(marshalled)
			_ = mustUnmarshalMetricIDs(nil, marshalled)
		}
		b.StopTimer()
		compressionRate := float64(numMetricIDs*8) / float64(marshalledLen)
		b.ReportMetric(compressionRate, "compression-rate")
	}

	for _, n := range []int{0, 1, 10, 100, 1e3, 1e4, 1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("numMetricIDs-%d", n), func(b *testing.B) {
			f(b, n)
		})
	}
}
