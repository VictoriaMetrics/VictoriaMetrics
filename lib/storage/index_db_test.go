package storage

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
)

func TestRemoveDuplicateMetricIDs(t *testing.T) {
	f := func(metricIDs, expectedMetricIDs []uint64) {
		a := removeDuplicateMetricIDs(metricIDs)
		if !reflect.DeepEqual(a, expectedMetricIDs) {
			t.Fatalf("unexpected result from removeDuplicateMetricIDs:\ngot\n%d\nwant\n%d", a, expectedMetricIDs)
		}
	}
	f(nil, nil)
	f([]uint64{123}, []uint64{123})
	f([]uint64{123, 123}, []uint64{123})
	f([]uint64{123, 123, 123}, []uint64{123})
	f([]uint64{0, 1, 1, 2}, []uint64{0, 1, 2})
	f([]uint64{0, 0, 0, 1, 1, 2}, []uint64{0, 1, 2})
	f([]uint64{0, 1, 1, 2, 2}, []uint64{0, 1, 2})
	f([]uint64{0, 1, 2, 2}, []uint64{0, 1, 2})
}

func TestMarshalUnmarshalTSIDs(t *testing.T) {
	f := func(tsids []TSID) {
		t.Helper()
		value := marshalTSIDs(nil, tsids)
		tsidsGot, err := unmarshalTSIDs(nil, value)
		if err != nil {
			t.Fatalf("cannot unmarshal tsids: %s", err)
		}
		if len(tsids) == 0 && len(tsidsGot) != 0 || len(tsids) > 0 && !reflect.DeepEqual(tsids, tsidsGot) {
			t.Fatalf("unexpected tsids unmarshaled\ngot\n%+v\nwant\n%+v", tsidsGot, tsids)
		}

		// Try marshlaing with prefix
		prefix := []byte("prefix")
		valueExt := marshalTSIDs(prefix, tsids)
		if !bytes.Equal(valueExt[:len(prefix)], prefix) {
			t.Fatalf("unexpected prefix after marshaling;\ngot\n%X\nwant\n%X", valueExt[:len(prefix)], prefix)
		}
		if !bytes.Equal(valueExt[len(prefix):], value) {
			t.Fatalf("unexpected prefixed marshaled value;\ngot\n%X\nwant\n%X", valueExt[len(prefix):], value)
		}

		// Try unmarshaling with prefix
		tsidPrefix := []TSID{{MetricID: 123}, {JobID: 456}}
		tsidsGot, err = unmarshalTSIDs(tsidPrefix, value)
		if err != nil {
			t.Fatalf("cannot unmarshal prefixed tsids: %s", err)
		}
		if !reflect.DeepEqual(tsidsGot[:len(tsidPrefix)], tsidPrefix) {
			t.Fatalf("unexpected tsid prefix\ngot\n%+v\nwant\n%+v", tsidsGot[:len(tsidPrefix)], tsidPrefix)
		}
		if len(tsids) == 0 && len(tsidsGot) != len(tsidPrefix) || len(tsids) > 0 && !reflect.DeepEqual(tsidsGot[len(tsidPrefix):], tsids) {
			t.Fatalf("unexpected prefixed tsids unmarshaled\ngot\n%+v\nwant\n%+v", tsidsGot[len(tsidPrefix):], tsids)
		}
	}

	f(nil)
	f([]TSID{{MetricID: 123}})
	f([]TSID{{JobID: 34}, {MetricID: 2343}, {InstanceID: 243321}})
}

func TestIndexDBOpenClose(t *testing.T) {
	metricIDCache := workingsetcache.New(1234, time.Hour)
	metricNameCache := workingsetcache.New(1234, time.Hour)
	defer metricIDCache.Stop()
	defer metricNameCache.Stop()

	var hmCurr atomic.Value
	hmCurr.Store(&hourMetricIDs{})
	var hmPrev atomic.Value
	hmPrev.Store(&hourMetricIDs{})

	for i := 0; i < 5; i++ {
		db, err := openIndexDB("test-index-db", metricIDCache, metricNameCache, &hmCurr, &hmPrev)
		if err != nil {
			t.Fatalf("cannot open indexDB: %s", err)
		}
		db.MustClose()
	}
	if err := os.RemoveAll("test-index-db"); err != nil {
		t.Fatalf("cannot remove indexDB: %s", err)
	}
}

func TestIndexDB(t *testing.T) {
	const accountsCount = 3
	const projectsCount = 2
	const metricGroups = 10

	t.Run("serial", func(t *testing.T) {
		metricIDCache := workingsetcache.New(1234, time.Hour)
		metricNameCache := workingsetcache.New(1234, time.Hour)
		defer metricIDCache.Stop()
		defer metricNameCache.Stop()

		var hmCurr atomic.Value
		hmCurr.Store(&hourMetricIDs{})
		var hmPrev atomic.Value
		hmPrev.Store(&hourMetricIDs{})

		dbName := "test-index-db-serial"
		db, err := openIndexDB(dbName, metricIDCache, metricNameCache, &hmCurr, &hmPrev)
		if err != nil {
			t.Fatalf("cannot open indexDB: %s", err)
		}
		defer func() {
			db.MustClose()
			if err := os.RemoveAll(dbName); err != nil {
				t.Fatalf("cannot remove indexDB: %s", err)
			}
		}()

		if err := testIndexDBBigMetricName(db); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		mns, tsids, err := testIndexDBGetOrCreateTSIDByName(db, accountsCount, projectsCount, metricGroups)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := testIndexDBBigMetricName(db); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := testIndexDBCheckTSIDByName(db, mns, tsids, false); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := testIndexDBBigMetricName(db); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Re-open the db and verify it works as expected.
		db.MustClose()
		db, err = openIndexDB(dbName, metricIDCache, metricNameCache, &hmCurr, &hmPrev)
		if err != nil {
			t.Fatalf("cannot open indexDB: %s", err)
		}
		if err := testIndexDBBigMetricName(db); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := testIndexDBCheckTSIDByName(db, mns, tsids, false); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := testIndexDBBigMetricName(db); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	})

	t.Run("concurrent", func(t *testing.T) {
		metricIDCache := workingsetcache.New(1234, time.Hour)
		metricNameCache := workingsetcache.New(1234, time.Hour)
		defer metricIDCache.Stop()
		defer metricNameCache.Stop()

		var hmCurr atomic.Value
		hmCurr.Store(&hourMetricIDs{})
		var hmPrev atomic.Value
		hmPrev.Store(&hourMetricIDs{})

		dbName := "test-index-db-concurrent"
		db, err := openIndexDB(dbName, metricIDCache, metricNameCache, &hmCurr, &hmPrev)
		if err != nil {
			t.Fatalf("cannot open indexDB: %s", err)
		}
		defer func() {
			db.MustClose()
			if err := os.RemoveAll(dbName); err != nil {
				t.Fatalf("cannot remove indexDB: %s", err)
			}
		}()

		ch := make(chan error, 3)
		for i := 0; i < cap(ch); i++ {
			go func() {
				if err := testIndexDBBigMetricName(db); err != nil {
					ch <- err
					return
				}
				mns, tsid, err := testIndexDBGetOrCreateTSIDByName(db, accountsCount, projectsCount, metricGroups)
				if err != nil {
					ch <- err
					return
				}
				if err := testIndexDBBigMetricName(db); err != nil {
					ch <- err
					return
				}
				if err := testIndexDBCheckTSIDByName(db, mns, tsid, true); err != nil {
					ch <- err
					return
				}
				if err := testIndexDBBigMetricName(db); err != nil {
					ch <- err
					return
				}
				ch <- nil
			}()
		}
		var errors []error
		for i := 0; i < cap(ch); i++ {
			select {
			case err := <-ch:
				if err != nil {
					errors = append(errors, fmt.Errorf("unexpected error: %s", err))
				}
			case <-time.After(30 * time.Second):
				t.Fatalf("timeout")
			}
		}
		if len(errors) > 0 {
			t.Fatal(errors[0])
		}
	})
}

func testIndexDBBigMetricName(db *indexDB) error {
	var bigBytes []byte
	for i := 0; i < 128*1000; i++ {
		bigBytes = append(bigBytes, byte(i))
	}
	var mn MetricName
	var tsid TSID

	is := db.getIndexSearch()
	defer db.putIndexSearch(is)

	// Try creating too big metric group
	mn.Reset()
	mn.MetricGroup = append(mn.MetricGroup[:0], bigBytes...)
	mn.sortTags()
	metricName := mn.Marshal(nil)
	if err := is.GetOrCreateTSIDByName(&tsid, metricName); err == nil {
		return fmt.Errorf("expecting non-nil error on an attempt to insert metric with too big MetricGroup")
	}

	// Try creating too big tag key
	mn.Reset()
	mn.MetricGroup = append(mn.MetricGroup[:0], "xxx"...)
	mn.Tags = []Tag{{
		Key:   append([]byte(nil), bigBytes...),
		Value: []byte("foobar"),
	}}
	mn.sortTags()
	metricName = mn.Marshal(nil)
	if err := is.GetOrCreateTSIDByName(&tsid, metricName); err == nil {
		return fmt.Errorf("expecting non-nil error on an attempt to insert metric with too big tag key")
	}

	// Try creating too big tag value
	mn.Reset()
	mn.MetricGroup = append(mn.MetricGroup[:0], "xxx"...)
	mn.Tags = []Tag{{
		Key:   []byte("foobar"),
		Value: append([]byte(nil), bigBytes...),
	}}
	mn.sortTags()
	metricName = mn.Marshal(nil)
	if err := is.GetOrCreateTSIDByName(&tsid, metricName); err == nil {
		return fmt.Errorf("expecting non-nil error on an attempt to insert metric with too big tag value")
	}

	// Try creating metric name with too many tags
	mn.Reset()
	mn.MetricGroup = append(mn.MetricGroup[:0], "xxx"...)
	for i := 0; i < 60000; i++ {
		mn.Tags = append(mn.Tags, Tag{
			Key:   []byte(fmt.Sprintf("foobar %d", i)),
			Value: []byte(fmt.Sprintf("sdfjdslkfj %d", i)),
		})
	}
	mn.sortTags()
	metricName = mn.Marshal(nil)
	if err := is.GetOrCreateTSIDByName(&tsid, metricName); err == nil {
		return fmt.Errorf("expecting non-nil error on an attempt to insert metric with too many tags")
	}

	return nil
}

func testIndexDBGetOrCreateTSIDByName(db *indexDB, accountsCount, projectsCount, metricGroups int) ([]MetricName, []TSID, error) {
	// Create tsids.
	var mns []MetricName
	var tsids []TSID

	is := db.getIndexSearch()
	defer db.putIndexSearch(is)

	var metricNameBuf []byte
	for i := 0; i < 4e2+1; i++ {
		var mn MetricName
		mn.AccountID = uint32((i + 2) % accountsCount)
		mn.ProjectID = uint32((i + 1) % projectsCount)

		// Init MetricGroup.
		mn.MetricGroup = []byte(fmt.Sprintf("metricGroup_%d\x00\x01\x02", i%metricGroups))

		// Init other tags.
		tagsCount := rand.Intn(10) + 1
		for j := 0; j < tagsCount; j++ {
			key := fmt.Sprintf("key\x01\x02\x00_%d_%d", i, j)
			value := fmt.Sprintf("val\x01_%d\x00_%d\x02", i, j)
			mn.AddTag(key, value)
		}
		mn.sortTags()
		metricNameBuf = mn.Marshal(metricNameBuf[:0])

		// Create tsid for the metricName.
		var tsid TSID
		if err := is.GetOrCreateTSIDByName(&tsid, metricNameBuf); err != nil {
			return nil, nil, fmt.Errorf("unexpected error when creating tsid for mn:\n%s: %s", &mn, err)
		}
		if tsid.AccountID != mn.AccountID {
			return nil, nil, fmt.Errorf("unexpected TSID.AccountID; got %d; want %d; mn:\n%s\ntsid:\n%+v", tsid.AccountID, mn.AccountID, &mn, &tsid)
		}
		if tsid.ProjectID != mn.ProjectID {
			return nil, nil, fmt.Errorf("unexpected TSID.ProjectID; got %d; want %d; mn:\n%s\ntsid:\n%+v", tsid.ProjectID, mn.ProjectID, &mn, &tsid)
		}

		mns = append(mns, mn)
		tsids = append(tsids, tsid)
	}

	// fill Date -> MetricID cache
	date := uint64(timestampFromTime(time.Now())) / msecPerDay
	for i := range tsids {
		tsid := &tsids[i]
		if err := db.storeDateMetricID(date, tsid.MetricID, tsid.AccountID, tsid.ProjectID); err != nil {
			return nil, nil, fmt.Errorf("error in storeDateMetricID(%d, %d, %d, %d): %s", date, tsid.MetricID, tsid.AccountID, tsid.ProjectID, err)
		}
	}

	// Flush index to disk, so it becomes visible for search
	db.tb.DebugFlush()

	return mns, tsids, nil
}

func testIndexDBCheckTSIDByName(db *indexDB, mns []MetricName, tsids []TSID, isConcurrent bool) error {
	hasValue := func(tvs []string, v []byte) bool {
		for _, tv := range tvs {
			if string(v) == tv {
				return true
			}
		}
		return false
	}

	type accountProjectKey struct {
		AccountID uint32
		ProjectID uint32
	}
	allKeys := make(map[accountProjectKey]map[string]bool)
	timeseriesCounters := make(map[accountProjectKey]map[uint64]bool)
	var tsidCopy TSID
	var metricNameCopy []byte
	for i := range mns {
		mn := &mns[i]
		tsid := &tsids[i]

		apKey := accountProjectKey{
			AccountID: tsid.AccountID,
			ProjectID: tsid.ProjectID,
		}
		tc := timeseriesCounters[apKey]
		if tc == nil {
			tc = make(map[uint64]bool)
			timeseriesCounters[apKey] = tc
		}
		tc[tsid.MetricID] = true

		mn.sortTags()
		metricName := mn.Marshal(nil)

		if err := db.getTSIDByNameNoCreate(&tsidCopy, metricName); err != nil {
			return fmt.Errorf("cannot obtain tsid #%d for mn %s: %s", i, mn, err)
		}
		if isConcurrent {
			// Copy tsid.MetricID, since multiple TSIDs may match
			// the same mn in concurrent mode.
			tsidCopy.MetricID = tsid.MetricID
		}
		if !reflect.DeepEqual(tsid, &tsidCopy) {
			return fmt.Errorf("unexpected tsid for mn:\n%s\ngot\n%+v\nwant\n%+v", mn, &tsidCopy, tsid)
		}

		// Search for metric name for the given metricID.
		var err error
		metricNameCopy, err = db.searchMetricName(metricNameCopy[:0], tsidCopy.MetricID, tsidCopy.AccountID, tsidCopy.ProjectID)
		if err != nil {
			return fmt.Errorf("error in searchMetricName for metricID=%d; i=%d: %s", tsidCopy.MetricID, i, err)
		}
		if !bytes.Equal(metricName, metricNameCopy) {
			return fmt.Errorf("unexpected mn for metricID=%d;\ngot\n%q\nwant\n%q", tsidCopy.MetricID, metricNameCopy, metricName)
		}

		// Try searching metric name for non-existent MetricID.
		buf, err := db.searchMetricName(nil, 1, mn.AccountID, mn.ProjectID)
		if err != io.EOF {
			return fmt.Errorf("expecting io.EOF error when searching for non-existing metricID; got %v", err)
		}
		if len(buf) > 0 {
			return fmt.Errorf("expecting empty buf when searching for non-existent metricID; got %X", buf)
		}

		// Test SearchTagValues
		tvs, err := db.SearchTagValues(mn.AccountID, mn.ProjectID, nil, 1e5)
		if err != nil {
			return fmt.Errorf("error in SearchTagValues for __name__: %s", err)
		}
		if !hasValue(tvs, mn.MetricGroup) {
			return fmt.Errorf("SearchTagValues couldn't find %q; found %q", mn.MetricGroup, tvs)
		}
		apKeys := allKeys[apKey]
		if apKeys == nil {
			apKeys = make(map[string]bool)
			allKeys[apKey] = apKeys
		}
		for i := range mn.Tags {
			tag := &mn.Tags[i]
			tvs, err := db.SearchTagValues(mn.AccountID, mn.ProjectID, tag.Key, 1e5)
			if err != nil {
				return fmt.Errorf("error in SearchTagValues for __name__: %s", err)
			}
			if !hasValue(tvs, tag.Value) {
				return fmt.Errorf("SearchTagValues couldn't find %q=%q; found %q", tag.Key, tag.Value, tvs)
			}
			apKeys[string(tag.Key)] = true
		}
	}

	// Test SearchTagKeys
	for k, apKeys := range allKeys {
		tks, err := db.SearchTagKeys(k.AccountID, k.ProjectID, 1e5)
		if err != nil {
			return fmt.Errorf("error in SearchTagKeys: %s", err)
		}
		if !hasValue(tks, nil) {
			return fmt.Errorf("cannot find __name__ in %q", tks)
		}
		for key := range apKeys {
			if !hasValue(tks, []byte(key)) {
				return fmt.Errorf("cannot find %q in %q", key, tks)
			}
		}
	}

	// Check timerseriesCounters only for serial test.
	// Concurrent test may create duplicate timeseries, so GetSeriesCount
	// would return more timeseries than needed.
	if !isConcurrent {
		for k, tc := range timeseriesCounters {
			n, err := db.GetSeriesCount(k.AccountID, k.ProjectID)
			if err != nil {
				return fmt.Errorf("unexpected error in GetSeriesCount(%v): %s", k, err)
			}
			if n != uint64(len(tc)) {
				return fmt.Errorf("unexpected GetSeriesCount(%v); got %d; want %d", k, n, uint64(len(tc)))
			}
		}
	}

	// Try tag filters.
	for i := range mns {
		mn := &mns[i]
		tsid := &tsids[i]

		// Search without regexps.
		tfs := NewTagFilters(mn.AccountID, mn.ProjectID)
		if err := tfs.Add(nil, mn.MetricGroup, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup: %s", err)
		}
		for j := 0; j < len(mn.Tags); j++ {
			t := &mn.Tags[j]
			if err := tfs.Add(t.Key, t.Value, false, false); err != nil {
				return fmt.Errorf("cannot create tag filter for tag: %s", err)
			}
		}
		if err := tfs.Add(nil, []byte("foobar"), true, false); err != nil {
			return fmt.Errorf("cannot add negative filter: %s", err)
		}
		if err := tfs.Add(nil, nil, true, false); err != nil {
			return fmt.Errorf("cannot add no-op negative filter: %s", err)
		}
		tsidsFound, err := db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search by exact tag filter: %s", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in exact tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s\ni=%d", tsid, tsidsFound, tfs, mn, i)
		}

		// Verify tag cache.
		tsidsCached, err := db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search by exact tag filter: %s", err)
		}
		if !reflect.DeepEqual(tsidsCached, tsidsFound) {
			return fmt.Errorf("unexpected tsids returned; got\n%+v; want\n%+v", tsidsCached, tsidsFound)
		}

		// Add negative filter for zeroing search results.
		if err := tfs.Add(nil, mn.MetricGroup, true, false); err != nil {
			return fmt.Errorf("cannot add negative filter for zeroing search results: %s", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search by exact tag filter with full negative: %s", err)
		}
		if testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("unexpected tsid found for exact negative filter\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with regexps.
		tfs.Reset(mn.AccountID, mn.ProjectID)
		if err := tfs.Add(nil, mn.MetricGroup, false, true); err != nil {
			return fmt.Errorf("cannot create regexp tag filter for MetricGroup: %s", err)
		}
		for j := 0; j < len(mn.Tags); j++ {
			t := &mn.Tags[j]
			if err := tfs.Add(t.Key, append(t.Value, "|foo*."...), false, true); err != nil {
				return fmt.Errorf("cannot create regexp tag filter for tag: %s", err)
			}
			if err := tfs.Add(t.Key, append(t.Value, "|aaa|foo|bar"...), false, true); err != nil {
				return fmt.Errorf("cannot create regexp tag filter for tag: %s", err)
			}
		}
		if err := tfs.Add(nil, []byte("^foobar$"), true, true); err != nil {
			return fmt.Errorf("cannot add negative filter with regexp: %s", err)
		}
		if err := tfs.Add(nil, nil, true, true); err != nil {
			return fmt.Errorf("cannot add no-op negative filter with regexp: %s", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search by regexp tag filter: %s", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in regexp tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}
		if err := tfs.Add(nil, mn.MetricGroup, true, true); err != nil {
			return fmt.Errorf("cannot add negative filter for zeroing search results: %s", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search by regexp tag filter with full negative: %s", err)
		}
		if testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("unexpected tsid found for regexp negative filter\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with filter matching zero results.
		tfs.Reset(mn.AccountID, mn.ProjectID)
		if err := tfs.Add([]byte("non-existing-key"), []byte("foobar"), false, false); err != nil {
			return fmt.Errorf("cannot add non-existing key: %s", err)
		}
		if err := tfs.Add(nil, mn.MetricGroup, false, true); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup matching zero results: %s", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search by non-existing tag filter: %s", err)
		}
		if len(tsidsFound) > 0 {
			return fmt.Errorf("non-zero tsidsFound for non-existing tag filter: %+v", tsidsFound)
		}

		if isConcurrent {
			// Skip empty filter search in concurrent mode, since it looks like
			// it has a lag preventing from correct work.
			continue
		}

		// Search with empty filter. It should match all the results for (accountID, projectID).
		tfs.Reset(mn.AccountID, mn.ProjectID)
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search for common prefix: %s", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in common prefix\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with empty metricGroup. It should match zero results.
		tfs.Reset(mn.AccountID, mn.ProjectID)
		if err := tfs.Add(nil, nil, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for empty metricGroup: %s", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search for empty metricGroup: %s", err)
		}
		if len(tsidsFound) != 0 {
			return fmt.Errorf("unexpected non-empty tsids found for empty metricGroup: %v", tsidsFound)
		}

		// Search with multiple tfss
		tfs1 := NewTagFilters(mn.AccountID, mn.ProjectID)
		if err := tfs1.Add(nil, nil, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for empty metricGroup: %s", err)
		}
		tfs2 := NewTagFilters(mn.AccountID, mn.ProjectID)
		if err := tfs2.Add(nil, mn.MetricGroup, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup: %s", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs1, tfs2}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search for empty metricGroup: %s", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing when searching for multiple tfss \ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Verify empty tfss
		tsidsFound, err = db.searchTSIDs(nil, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search for nil tfss: %s", err)
		}
		if len(tsidsFound) != 0 {
			return fmt.Errorf("unexpected non-empty tsids fround for nil tfss")
		}
	}

	return nil
}

func testHasTSID(tsids []TSID, tsid *TSID) bool {
	for i := range tsids {
		if tsids[i] == *tsid {
			return true
		}
	}
	return false
}

func TestMatchTagFilters(t *testing.T) {
	var mn MetricName
	mn.AccountID = 123
	mn.ProjectID = 456
	mn.MetricGroup = append(mn.MetricGroup, "foobar_metric"...)
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key %d", i)
		value := fmt.Sprintf("value %d", i)
		mn.AddTag(key, value)
	}
	var bb bytesutil.ByteBuffer

	// Verify tag filters for different account / project
	tfs := NewTagFilters(mn.AccountID, mn.ProjectID+1)
	if err := tfs.Add(nil, []byte("foobar_metric"), false, false); err != nil {
		t.Fatalf("cannot add filter: %s", err)
	}
	ok, err := matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Tag filters shouldn't match for invalid projectID")
	}
	tfs.Reset(mn.AccountID+1, mn.ProjectID)
	if err := tfs.Add(nil, []byte("foobar_metric"), false, false); err != nil {
		t.Fatalf("cannot add filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Tag filters shouldn't match for invalid accountID")
	}

	// Correct AccountID , ProjectID
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add(nil, []byte("foobar_metric"), false, false); err != nil {
		t.Fatalf("cannot add filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("should match")
	}

	// Empty tag filters should match.
	tfs.Reset(mn.AccountID, mn.ProjectID)
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("empty tag filters should match")
	}

	// Negative match by MetricGroup
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add(nil, []byte("foobar"), false, false); err != nil {
		t.Fatalf("cannot add no regexp, no negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add(nil, []byte("obar.+"), false, true); err != nil {
		t.Fatalf("cannot add regexp, no negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add(nil, []byte("foobar_metric"), true, false); err != nil {
		t.Fatalf("cannot add no regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add(nil, []byte("foob.+metric"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}

	// Positive match by MetricGroup
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add(nil, []byte("foobar_metric"), false, false); err != nil {
		t.Fatalf("cannot add no regexp, no negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add(nil, []byte("foobar.+etric"), false, true); err != nil {
		t.Fatalf("cannot add regexp, no negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add(nil, []byte("obar_metric"), true, false); err != nil {
		t.Fatalf("cannot add no regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add(nil, []byte("ob.+metric"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}

	// Negative match by non-existing tag
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("non-existing-tag"), []byte("foobar"), false, false); err != nil {
		t.Fatalf("cannot add no regexp, no negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("non-existing-tag"), []byte("obar.+"), false, true); err != nil {
		t.Fatalf("cannot add regexp, no negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("non-existing-tag"), []byte("foobar_metric"), true, false); err != nil {
		t.Fatalf("cannot add no regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("non-existing-tag"), []byte("foob.+metric"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("non-existing-tag"), []byte(".+"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}

	// Negative match by existing tag
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("key 0"), []byte("foobar"), false, false); err != nil {
		t.Fatalf("cannot add no regexp, no negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("key 1"), []byte("obar.+"), false, true); err != nil {
		t.Fatalf("cannot add regexp, no negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("key 2"), []byte("value 2"), true, false); err != nil {
		t.Fatalf("cannot add no regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("key 3"), []byte("v.+lue 3"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}

	// Positive match by existing tag
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("key 0"), []byte("value 0"), false, false); err != nil {
		t.Fatalf("cannot add no regexp, no negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("key 1"), []byte(".+lue 1"), false, true); err != nil {
		t.Fatalf("cannot add regexp, no negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("key 2"), []byte("value 3"), true, false); err != nil {
		t.Fatalf("cannot add no regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("key 3"), []byte("v.+lue 2"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("key 3"), []byte(""), true, false); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}

	// Positive match by multiple tags and MetricGroup
	tfs.Reset(mn.AccountID, mn.ProjectID)
	if err := tfs.Add([]byte("key 0"), []byte("value 0"), false, false); err != nil {
		t.Fatalf("cannot add no regexp, no negative filter: %s", err)
	}
	if err := tfs.Add([]byte("key 2"), []byte("value [0-9]"), false, true); err != nil {
		t.Fatalf("cannot add regexp, no negative filter: %s", err)
	}
	if err := tfs.Add([]byte("key 3"), []byte("value 23"), true, false); err != nil {
		t.Fatalf("cannt add no regexp, negative filter: %s", err)
	}
	if err := tfs.Add([]byte("key 2"), []byte("lue.+43"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	if err := tfs.Add(nil, []byte("foobar_metric"), false, false); err != nil {
		t.Fatalf("cannot add no regexp, no negative filter: %s", err)
	}
	if err := tfs.Add(nil, []byte("foo.+metric"), false, true); err != nil {
		t.Fatalf("cannot add regexp, no negative filter: %s", err)
	}
	if err := tfs.Add(nil, []byte("sdfdsf"), true, false); err != nil {
		t.Fatalf("cannot add no regexp, negative filter: %s", err)
	}
	if err := tfs.Add(nil, []byte("o.+metr"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}

	// Negative match by multiple tags and MetricGroup
	tfs.Reset(mn.AccountID, mn.ProjectID)
	// Positive matches
	if err := tfs.Add([]byte("key 0"), []byte("value 0"), false, false); err != nil {
		t.Fatalf("cannot add no regexp, no negative filter: %s", err)
	}
	if err := tfs.Add([]byte("key 2"), []byte("value [0-9]"), false, true); err != nil {
		t.Fatalf("cannot add regexp, no negative filter: %s", err)
	}
	if err := tfs.Add([]byte("key 3"), []byte("value 23"), true, false); err != nil {
		t.Fatalf("cannot add no regexp, negative filter: %s", err)
	}
	// Negative matches
	if err := tfs.Add([]byte("key 2"), []byte("v.+2"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	if err := tfs.Add(nil, []byte("obar_metric"), false, false); err != nil {
		t.Fatalf("cannot add no regexp, no negative filter: %s", err)
	}
	if err := tfs.Add(nil, []byte("oo.+metric"), false, true); err != nil {
		t.Fatalf("cannot add regexp, no negative filter: %s", err)
	}
	// Positive matches
	if err := tfs.Add(nil, []byte("sdfdsf"), true, false); err != nil {
		t.Fatalf("cannot add no regexp, negative filter: %s", err)
	}
	if err := tfs.Add(nil, []byte("o.+metr"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}
}

func toTFPointers(tfs []tagFilter) []*tagFilter {
	tfps := make([]*tagFilter, len(tfs))
	for i := range tfs {
		tfps[i] = &tfs[i]
	}
	return tfps
}
