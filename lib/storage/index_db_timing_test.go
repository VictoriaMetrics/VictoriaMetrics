package storage

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/VictoriaMetrics/fastcache"
)

func BenchmarkIndexDBAddTSIDs(b *testing.B) {
	const recordsPerLoop = 1e3

	metricIDCache := fastcache.New(1234)
	metricNameCache := fastcache.New(1234)
	defer metricIDCache.Reset()
	defer metricNameCache.Reset()
	const dbName = "bench-index-db-add-tsids"
	db, err := openIndexDB(dbName, metricIDCache, metricNameCache, nil, nil)
	if err != nil {
		b.Fatalf("cannot open indexDB: %s", err)
	}
	defer func() {
		db.MustClose()
		if err := os.RemoveAll(dbName); err != nil {
			b.Fatalf("cannot remove indexDB: %s", err)
		}
	}()

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
			benchmarkIndexDBAddTSIDs(db, &tsid, &mn, startOffset, recordsPerLoop)
			startOffset += recordsPerLoop
		}
	})
	b.StopTimer()
}

func benchmarkIndexDBAddTSIDs(db *indexDB, tsid *TSID, mn *MetricName, startOffset, recordsPerLoop int) {
	var metricName []byte
	is := db.getIndexSearch()
	defer db.putIndexSearch(is)
	for i := 0; i < recordsPerLoop; i++ {
		mn.MetricGroup = strconv.AppendUint(mn.MetricGroup[:0], uint64(i+startOffset), 10)
		for j := range mn.Tags {
			mn.Tags[j].Value = strconv.AppendUint(mn.Tags[j].Value[:0], uint64(i*j), 16)
		}
		mn.sortTags()
		metricName = mn.Marshal(metricName[:0])
		if err := is.GetOrCreateTSIDByName(tsid, metricName); err != nil {
			panic(fmt.Errorf("cannot insert record: %s", err))
		}
	}
}

func BenchmarkIndexDBSearchTSIDs(b *testing.B) {
	metricIDCache := fastcache.New(1234)
	metricNameCache := fastcache.New(1234)
	defer metricIDCache.Reset()
	defer metricNameCache.Reset()
	const dbName = "bench-index-db-search-tsids"
	db, err := openIndexDB(dbName, metricIDCache, metricNameCache, nil, nil)
	if err != nil {
		b.Fatalf("cannot open indexDB: %s", err)
	}
	defer func() {
		db.MustClose()
		if err := os.RemoveAll(dbName); err != nil {
			b.Fatalf("cannot remove indexDB: %s", err)
		}
	}()

	const accountsCount = 111
	const projectsCount = 33333
	const recordsCount = 1e5

	// Fill the db with recordsCount records.
	var mn MetricName
	mn.MetricGroup = []byte("rps")
	for i := 0; i < 2; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		mn.AddTag(key, value)
	}
	var tsid TSID
	var metricName []byte
	is := db.getIndexSearch()
	defer db.putIndexSearch(is)
	for i := 0; i < recordsCount; i++ {
		mn.sortTags()
		metricName = mn.Marshal(metricName[:0])
		if err := is.GetOrCreateTSIDByName(&tsid, metricName); err != nil {
			b.Fatalf("cannot insert record: %s", err)
		}
	}

	b.SetBytes(1)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		tags := []Tag{
			{[]byte("key_0"), []byte("value_0")},
			{[]byte("key_1"), []byte("value_1")},
		}
		var tfs TagFilters
		tfss := []*TagFilters{&tfs}
		i := 0
		for pb.Next() {
			tfs.Reset()
			for j := range tags {
				if err := tfs.Add(tags[j].Key, tags[j].Value, false, false); err != nil {
					panic(fmt.Errorf("BUG: unexpected error: %s", err))
				}
			}
			tsids, err := db.searchTSIDs(tfss, TimeRange{}, 1e5)
			if err != nil {
				panic(fmt.Errorf("unexpected error in search for tfs=%s: %s", &tfs, err))
			}
			if len(tsids) == 0 && i < recordsCount {
				panic(fmt.Errorf("zero tsids found for tfs=%s", &tfs))
			}
			i++
		}
	})
}

func BenchmarkIndexDBGetTSIDs(b *testing.B) {
	metricIDCache := fastcache.New(1234)
	metricNameCache := fastcache.New(1234)
	defer metricIDCache.Reset()
	defer metricNameCache.Reset()
	const dbName = "bench-index-db-get-tsids"
	db, err := openIndexDB(dbName, metricIDCache, metricNameCache, nil, nil)
	if err != nil {
		b.Fatalf("cannot open indexDB: %s", err)
	}
	defer func() {
		db.MustClose()
		if err := os.RemoveAll(dbName); err != nil {
			b.Fatalf("cannot remove indexDB: %s", err)
		}
	}()

	const recordsPerLoop = 1000
	const accountsCount = 111
	const projectsCount = 33333
	const recordsCount = 1e5

	// Fill the db with recordsCount records.
	var mn MetricName
	mn.MetricGroup = []byte("rps")
	for i := 0; i < 2; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		mn.AddTag(key, value)
	}
	var tsid TSID
	var metricName []byte

	is := db.getIndexSearch()
	defer db.putIndexSearch(is)
	for i := 0; i < recordsCount; i++ {
		mn.sortTags()
		metricName = mn.Marshal(metricName[:0])
		if err := is.GetOrCreateTSIDByName(&tsid, metricName); err != nil {
			b.Fatalf("cannot insert record: %s", err)
		}
	}

	b.SetBytes(recordsPerLoop)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var tsidLocal TSID
		var metricNameLocal []byte
		mnLocal := mn
		is := db.getIndexSearch()
		defer db.putIndexSearch(is)
		for pb.Next() {
			for i := 0; i < recordsPerLoop; i++ {
				mnLocal.sortTags()
				metricNameLocal = mnLocal.Marshal(metricNameLocal[:0])
				if err := is.GetOrCreateTSIDByName(&tsidLocal, metricNameLocal); err != nil {
					panic(fmt.Errorf("cannot obtain tsid: %s", err))
				}
			}
		}
	})
	b.StopTimer()
}
