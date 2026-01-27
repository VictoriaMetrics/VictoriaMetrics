package storage

import (
	"bytes"
	"fmt"
	"math/rand"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
)

func TestTagFiltersToMetricIDsCache(t *testing.T) {
	f := func(want []uint64) {
		t.Helper()

		path := t.Name()
		defer fs.MustRemoveDir(path)

		s := MustOpenStorage(path, OpenOptions{})
		defer s.MustClose()

		ptw := s.tb.MustGetPartition(time.Now().UnixMilli())
		idb := ptw.pt.idb
		defer s.tb.PutPartition(ptw)

		key := []byte("key")
		wantSet := &uint64set.Set{}
		wantSet.AddMulti(want)
		idb.putMetricIDsToTagFiltersCache(nil, wantSet, key)
		gotSet, ok := idb.getMetricIDsFromTagFiltersCache(nil, key)
		if !ok {
			t.Fatalf("expected metricIDs to be found in cache but they weren't: %v", want)
		}
		got := gotSet.AppendTo(nil)
		slices.Sort(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected metricIDs in cache: got %v, want %v", got, want)
		}
	}

	f([]uint64{0})
	f([]uint64{1})
	f([]uint64{1234, 678932943, 843289893843})
	f([]uint64{1, 2, 3, 4, 5, 6, 8989898, 823849234, 1<<64 - 1, 1<<32 - 1, 0})
}

func TestTagFiltersToMetricIDsCache_EmptyMetricIDList(t *testing.T) {
	path := t.Name()
	defer fs.MustRemoveDir(path)
	s := MustOpenStorage(path, OpenOptions{})
	defer s.MustClose()
	ptw := s.tb.MustGetPartition(time.Now().UnixMilli())
	idb := ptw.pt.idb
	defer s.tb.PutPartition(ptw)

	key := []byte("key")
	idb.putMetricIDsToTagFiltersCache(nil, nil, key)
	got, ok := idb.getMetricIDsFromTagFiltersCache(nil, key)
	if !ok {
		t.Fatalf("expected empty metricID list to be found in cache but it wasn't")
	}
	if got.Len() > 0 {
		t.Fatalf("unexpected found metricID list to be empty but got %v", got.AppendTo(nil))
	}

}

func TestReverseBytes(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		result := reverseBytes(nil, []byte(s))
		if string(result) != resultExpected {
			t.Fatalf("unexpected result for reverseBytes(%q); got %q; want %q", s, result, resultExpected)
		}
	}
	f("", "")
	f("a", "a")
	f("av", "va")
	f("foo.bar", "rab.oof")
}

func TestMergeTagToMetricIDsRows(t *testing.T) {
	f := func(items []string, expectedItems []string) {
		t.Helper()
		var data []byte
		var itemsB []mergeset.Item
		for _, item := range items {
			data = append(data, item...)
			itemsB = append(itemsB, mergeset.Item{
				Start: uint32(len(data) - len(item)),
				End:   uint32(len(data)),
			})
		}
		if !checkItemsSorted(data, itemsB) {
			t.Fatalf("source items aren't sorted; items:\n%q", itemsB)
		}
		resultData, resultItemsB := mergeTagToMetricIDsRows(data, itemsB)
		if len(resultItemsB) != len(expectedItems) {
			t.Fatalf("unexpected len(resultItemsB); got %d; want %d", len(resultItemsB), len(expectedItems))
		}
		if !checkItemsSorted(resultData, resultItemsB) {
			t.Fatalf("result items aren't sorted; items:\n%q", resultItemsB)
		}
		buf := resultData
		for i, it := range resultItemsB {
			item := it.Bytes(resultData)
			if !bytes.HasPrefix(buf, item) {
				t.Fatalf("unexpected prefix for resultData #%d;\ngot\n%X\nwant\n%X", i, buf, item)
			}
			buf = buf[len(item):]
		}
		if len(buf) != 0 {
			t.Fatalf("unexpected tail left in resultData: %X", buf)
		}
		var resultItems []string
		for _, it := range resultItemsB {
			resultItems = append(resultItems, string(it.Bytes(resultData)))
		}
		if !reflect.DeepEqual(expectedItems, resultItems) {
			t.Fatalf("unexpected items;\ngot\n%X\nwant\n%X", resultItems, expectedItems)
		}
	}
	xy := func(nsPrefix byte, key, value string, metricIDs []uint64) string {
		dst := marshalCommonPrefix(nil, nsPrefix)
		if nsPrefix == nsPrefixDateTagToMetricIDs {
			dst = encoding.MarshalUint64(dst, 1234567901233)
		}
		t := &Tag{
			Key:   []byte(key),
			Value: []byte(value),
		}
		dst = t.Marshal(dst)
		for _, metricID := range metricIDs {
			dst = encoding.MarshalUint64(dst, metricID)
		}
		return string(dst)
	}
	x := func(key, value string, metricIDs []uint64) string {
		return xy(nsPrefixTagToMetricIDs, key, value, metricIDs)
	}
	y := func(key, value string, metricIDs []uint64) string {
		return xy(nsPrefixDateTagToMetricIDs, key, value, metricIDs)
	}

	f(nil, nil)
	f([]string{}, nil)
	f([]string{"foo"}, []string{"foo"})
	f([]string{"a", "b", "c", "def"}, []string{"a", "b", "c", "def"})
	f([]string{"\x00", "\x00b", "\x00c", "\x00def"}, []string{"\x00", "\x00b", "\x00c", "\x00def"})
	f([]string{
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
	}, []string{
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
	})
	f([]string{
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		y("", "", []uint64{0}),
		y("", "", []uint64{0}),
		y("", "", []uint64{0}),
	}, []string{
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		y("", "", []uint64{0}),
		y("", "", []uint64{0}),
	})
	f([]string{
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		"xyz",
	}, []string{
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		"xyz",
	})
	f([]string{
		"\x00asdf",
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
	}, []string{
		"\x00asdf",
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
	})
	f([]string{
		"\x00asdf",
		y("", "", []uint64{0}),
		y("", "", []uint64{0}),
		y("", "", []uint64{0}),
		y("", "", []uint64{0}),
	}, []string{
		"\x00asdf",
		y("", "", []uint64{0}),
		y("", "", []uint64{0}),
	})
	f([]string{
		"\x00asdf",
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		"xyz",
	}, []string{
		"\x00asdf",
		x("", "", []uint64{0}),
		"xyz",
	})
	f([]string{
		"\x00asdf",
		x("", "", []uint64{0}),
		x("", "", []uint64{0}),
		y("", "", []uint64{0}),
		y("", "", []uint64{0}),
		"xyz",
	}, []string{
		"\x00asdf",
		x("", "", []uint64{0}),
		y("", "", []uint64{0}),
		"xyz",
	})
	f([]string{
		"\x00asdf",
		x("", "", []uint64{1}),
		x("", "", []uint64{2}),
		x("", "", []uint64{3}),
		x("", "", []uint64{4}),
		"xyz",
	}, []string{
		"\x00asdf",
		x("", "", []uint64{1, 2, 3, 4}),
		"xyz",
	})
	f([]string{
		"\x00asdf",
		x("", "", []uint64{1}),
		x("", "", []uint64{2}),
		x("", "", []uint64{3}),
		x("", "", []uint64{4}),
	}, []string{
		"\x00asdf",
		x("", "", []uint64{1, 2, 3}),
		x("", "", []uint64{4}),
	})
	f([]string{
		"\x00asdf",
		x("", "", []uint64{1}),
		x("", "", []uint64{2, 3, 4}),
		x("", "", []uint64{2, 3, 4, 5}),
		x("", "", []uint64{3, 5}),
		"foo",
	}, []string{
		"\x00asdf",
		x("", "", []uint64{1, 2, 3, 4, 5}),
		"foo",
	})
	f([]string{
		"\x00asdf",
		x("", "", []uint64{1}),
		x("", "a", []uint64{2, 3, 4}),
		x("", "a", []uint64{2, 3, 4, 5}),
		x("", "b", []uint64{3, 5}),
		"foo",
	}, []string{
		"\x00asdf",
		x("", "", []uint64{1}),
		x("", "a", []uint64{2, 3, 4, 5}),
		x("", "b", []uint64{3, 5}),
		"foo",
	})
	f([]string{
		"\x00asdf",
		x("", "", []uint64{1}),
		x("x", "a", []uint64{2, 3, 4}),
		x("y", "", []uint64{2, 3, 4, 5}),
		x("y", "x", []uint64{3, 5}),
		"foo",
	}, []string{
		"\x00asdf",
		x("", "", []uint64{1}),
		x("x", "a", []uint64{2, 3, 4}),
		x("y", "", []uint64{2, 3, 4, 5}),
		x("y", "x", []uint64{3, 5}),
		"foo",
	})
	f([]string{
		"\x00asdf",
		x("sdf", "aa", []uint64{1, 1, 3}),
		x("sdf", "aa", []uint64{1, 2}),
		"foo",
	}, []string{
		"\x00asdf",
		x("sdf", "aa", []uint64{1, 2, 3}),
		"foo",
	})
	f([]string{
		"\x00asdf",
		x("sdf", "aa", []uint64{1, 2, 2, 4}),
		x("sdf", "aa", []uint64{1, 2, 3}),
		"foo",
	}, []string{
		"\x00asdf",
		x("sdf", "aa", []uint64{1, 2, 3, 4}),
		"foo",
	})

	// Construct big source chunks
	var metricIDs []uint64

	metricIDs = metricIDs[:0]
	for i := 0; i < maxMetricIDsPerRow-1; i++ {
		metricIDs = append(metricIDs, uint64(i))
	}
	f([]string{
		"\x00aa",
		x("foo", "bar", metricIDs),
		x("foo", "bar", metricIDs),
		y("foo", "bar", metricIDs),
		y("foo", "bar", metricIDs),
		"x",
	}, []string{
		"\x00aa",
		x("foo", "bar", metricIDs),
		y("foo", "bar", metricIDs),
		"x",
	})

	metricIDs = metricIDs[:0]
	for i := 0; i < maxMetricIDsPerRow; i++ {
		metricIDs = append(metricIDs, uint64(i))
	}
	f([]string{
		"\x00aa",
		x("foo", "bar", metricIDs),
		x("foo", "bar", metricIDs),
		"x",
	}, []string{
		"\x00aa",
		x("foo", "bar", metricIDs),
		x("foo", "bar", metricIDs),
		"x",
	})

	metricIDs = metricIDs[:0]
	for i := 0; i < 3*maxMetricIDsPerRow; i++ {
		metricIDs = append(metricIDs, uint64(i))
	}
	f([]string{
		"\x00aa",
		x("foo", "bar", metricIDs),
		x("foo", "bar", metricIDs),
		"x",
	}, []string{
		"\x00aa",
		x("foo", "bar", metricIDs),
		x("foo", "bar", metricIDs),
		"x",
	})
	f([]string{
		"\x00aa",
		x("foo", "bar", []uint64{0, 0, 1, 2, 3}),
		x("foo", "bar", metricIDs),
		x("foo", "bar", metricIDs),
		"x",
	}, []string{
		"\x00aa",
		x("foo", "bar", []uint64{0, 1, 2, 3}),
		x("foo", "bar", metricIDs),
		x("foo", "bar", metricIDs),
		"x",
	})

	// Check for duplicate metricIDs removal
	metricIDs = metricIDs[:0]
	for i := 0; i < maxMetricIDsPerRow-1; i++ {
		metricIDs = append(metricIDs, 123)
	}
	f([]string{
		"\x00aa",
		x("foo", "bar", metricIDs),
		x("foo", "bar", metricIDs),
		y("foo", "bar", metricIDs),
		"x",
	}, []string{
		"\x00aa",
		x("foo", "bar", []uint64{123}),
		y("foo", "bar", []uint64{123}),
		"x",
	})

	// Check fallback to the original items after merging, which result in incorrect ordering.
	metricIDs = metricIDs[:0]
	for i := 0; i < maxMetricIDsPerRow-3; i++ {
		metricIDs = append(metricIDs, uint64(123))
	}
	f([]string{
		"\x00aa",
		x("foo", "bar", metricIDs),
		x("foo", "bar", []uint64{123, 123, 125}),
		x("foo", "bar", []uint64{123, 124}),
		"x",
	}, []string{
		"\x00aa",
		x("foo", "bar", metricIDs),
		x("foo", "bar", []uint64{123, 123, 125}),
		x("foo", "bar", []uint64{123, 124}),
		"x",
	})
	f([]string{
		"\x00aa",
		x("foo", "bar", metricIDs),
		x("foo", "bar", []uint64{123, 123, 125}),
		x("foo", "bar", []uint64{123, 124}),
		y("foo", "bar", []uint64{123, 124}),
	}, []string{
		"\x00aa",
		x("foo", "bar", metricIDs),
		x("foo", "bar", []uint64{123, 123, 125}),
		x("foo", "bar", []uint64{123, 124}),
		y("foo", "bar", []uint64{123, 124}),
	})
	f([]string{
		x("foo", "bar", metricIDs),
		x("foo", "bar", []uint64{123, 123, 125}),
		x("foo", "bar", []uint64{123, 124}),
	}, []string{
		x("foo", "bar", metricIDs),
		x("foo", "bar", []uint64{123, 123, 125}),
		x("foo", "bar", []uint64{123, 124}),
	})
}

func TestRemoveDuplicateMetricIDs(t *testing.T) {
	f := func(metricIDs, expectedMetricIDs []uint64) {
		t.Helper()
		a := removeDuplicateMetricIDs(metricIDs)
		if !reflect.DeepEqual(a, expectedMetricIDs) {
			t.Fatalf("unexpected result from removeDuplicateMetricIDs:\ngot\n%d\nwant\n%d", a, expectedMetricIDs)
		}
	}
	f(nil, nil)
	f([]uint64{123}, []uint64{123})
	f([]uint64{123, 123}, []uint64{123})
	f([]uint64{123, 123, 123}, []uint64{123})
	f([]uint64{123, 1234, 1235}, []uint64{123, 1234, 1235})
	f([]uint64{0, 1, 1, 2}, []uint64{0, 1, 2})
	f([]uint64{0, 0, 0, 1, 1, 2}, []uint64{0, 1, 2})
	f([]uint64{0, 1, 1, 2, 2}, []uint64{0, 1, 2})
	f([]uint64{0, 1, 2, 2}, []uint64{0, 1, 2})
}

func TestIndexDBOpenClose(t *testing.T) {
	defer testRemoveAll(t)

	var s Storage
	path := filepath.Join(t.Name(), "2025_01")
	for i := 0; i < 5; i++ {
		var isReadOnly atomic.Bool
		db := mustOpenIndexDB(123, TimeRange{}, "name", path, &s, &isReadOnly, false)
		db.MustClose()
	}
}

func TestIndexDB(t *testing.T) {
	const metricGroups = 10
	timestamp := time.Now().UnixMilli()

	t.Run("serial", func(t *testing.T) {
		const path = "TestIndexDB-serial"
		s := MustOpenStorage(path, OpenOptions{})
		ptw := s.tb.MustGetPartition(timestamp)
		db := ptw.pt.idb
		mns, tsids, err := testIndexDBGetOrCreateTSIDByName(db, metricGroups, timestamp)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := testIndexDBCheckTSIDByName(db, mns, tsids, timestamp, false); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Re-open the storage and verify it works as expected.
		s.tb.PutPartition(ptw)
		s.MustClose()
		s = MustOpenStorage(path, OpenOptions{})

		ptw = s.tb.MustGetPartition(timestamp)
		db = ptw.pt.idb
		if err := testIndexDBCheckTSIDByName(db, mns, tsids, timestamp, false); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		s.tb.PutPartition(ptw)
		s.MustClose()
		fs.MustRemoveDir(path)
	})

	t.Run("concurrent", func(t *testing.T) {
		const path = "TestIndexDB-concurrent"
		s := MustOpenStorage(path, OpenOptions{})
		ptw := s.tb.MustGetPartition(timestamp)
		db := ptw.pt.idb

		ch := make(chan error, 3)
		for i := 0; i < cap(ch); i++ {
			go func() {
				mns, tsid, err := testIndexDBGetOrCreateTSIDByName(db, metricGroups, timestamp)
				if err != nil {
					ch <- err
					return
				}
				if err := testIndexDBCheckTSIDByName(db, mns, tsid, timestamp, true); err != nil {
					ch <- err
					return
				}
				ch <- nil
			}()
		}
		deadlineCh := time.After(30 * time.Second)
		for i := 0; i < cap(ch); i++ {
			select {
			case err := <-ch:
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			case <-deadlineCh:
				t.Fatalf("timeout")
			}
		}

		s.tb.PutPartition(ptw)
		s.MustClose()
		fs.MustRemoveDir(path)
	})
}

func testIndexDBGetOrCreateTSIDByName(db *indexDB, metricGroups int, timestamp int64) ([]MetricName, []TSID, error) {
	r := rand.New(rand.NewSource(1))
	// Create tsids.
	var mns []MetricName
	var tsids []TSID

	is := db.getIndexSearch(noDeadline)

	date := uint64(timestamp) / msecPerDay

	var metricNameBuf []byte
	for i := 0; i < 401; i++ {
		var mn MetricName

		// Init MetricGroup.
		mn.MetricGroup = []byte(fmt.Sprintf("metricGroup.%d\x00\x01\x02", i%metricGroups))

		// Init other tags.
		tagsCount := r.Intn(10) + 1
		for j := 0; j < tagsCount; j++ {
			key := fmt.Sprintf("key\x01\x02\x00_%d_%d", i, j)
			value := fmt.Sprintf("val\x01_%d\x00_%d\x02", i, j)
			mn.AddTag(key, value)
		}
		mn.sortTags()
		metricNameBuf = mn.Marshal(metricNameBuf[:0])

		// Create tsid for the metricName.
		var tsid TSID
		if !is.getTSIDByMetricName(&tsid, metricNameBuf, date) {
			generateTSID(&tsid, &mn)
			createAllIndexesForMetricName(db, &mn, &tsid, date)
		}

		mns = append(mns, mn)
		tsids = append(tsids, tsid)
	}

	db.putIndexSearch(is)

	// Flush index to disk, so it becomes visible for search
	db.tb.DebugFlush()

	return mns, tsids, nil
}

func testIndexDBCheckTSIDByName(db *indexDB, mns []MetricName, tsids []TSID, timestamp int64, isConcurrent bool) error {
	timeseriesCounters := make(map[uint64]bool)
	var tsidLocal TSID
	var metricNameCopy []byte
	allLabelNames := make(map[string]bool)
	for i := range mns {
		mn := &mns[i]
		tsid := &tsids[i]

		tc := timeseriesCounters
		tc[tsid.MetricID] = true

		mn.sortTags()
		metricName := mn.Marshal(nil)

		is := db.getIndexSearch(noDeadline)
		if !is.getTSIDByMetricName(&tsidLocal, metricName, uint64(timestamp)/msecPerDay) {
			return fmt.Errorf("cannot obtain tsid #%d for mn %s", i, mn)
		}
		db.putIndexSearch(is)

		if isConcurrent {
			// Copy tsid.MetricID, since multiple TSIDs may match
			// the same mn in concurrent mode.
			tsidLocal.MetricID = tsid.MetricID
		}
		if !reflect.DeepEqual(tsid, &tsidLocal) {
			return fmt.Errorf("unexpected tsid for mn:\n%s\ngot\n%+v\nwant\n%+v", mn, &tsidLocal, tsid)
		}

		// Search for metric name for the given metricID.
		var ok bool
		metricNameCopy, ok = db.searchMetricName(metricNameCopy[:0], tsidLocal.MetricID, false)
		if !ok {
			return fmt.Errorf("cannot find metricName for metricID=%d; i=%d", tsidLocal.MetricID, i)
		}
		if !bytes.Equal(metricName, metricNameCopy) {
			return fmt.Errorf("unexpected mn for metricID=%d;\ngot\n%q\nwant\n%q", tsidLocal.MetricID, metricNameCopy, metricName)
		}

		// Try searching metric name for non-existent MetricID.
		buf, found := db.searchMetricName(nil, 1, false)
		if found {
			return fmt.Errorf("unexpected metricName found for non-existing metricID; got %X", buf)
		}
		if len(buf) > 0 {
			return fmt.Errorf("expecting empty buf when searching for non-existent metricID; got %X", buf)
		}

		// Test SearchLabelValues
		lvs, err := db.SearchLabelValues(nil, "__name__", nil, TimeRange{}, 1e5, 1e9, noDeadline)
		if err != nil {
			return fmt.Errorf("error in SearchLabelValues(labelName=%q): %w", "__name__", err)
		}
		if _, ok := lvs[string(mn.MetricGroup)]; !ok {
			return fmt.Errorf("SearchLabelValues(labelName=%q): couldn't find %q; found %q", "__name__", mn.MetricGroup, lvs)
		}
		for i := range mn.Tags {
			tag := &mn.Tags[i]
			lvs, err := db.SearchLabelValues(nil, string(tag.Key), nil, TimeRange{}, 1e5, 1e9, noDeadline)
			if err != nil {
				return fmt.Errorf("error in SearchLabelValues(labelName=%q): %w", tag.Key, err)
			}
			if _, ok := lvs[string(tag.Value)]; !ok {
				return fmt.Errorf("SearchLabelValues(labelName=%q): couldn't find %q; found %q", tag.Key, tag.Value, lvs)
			}
			allLabelNames[string(tag.Key)] = true
		}
	}

	// Test SearchLabelNames (empty filters, global time range)
	lns, err := db.SearchLabelNames(nil, nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelNames(empty filter, global time range): %w", err)
	}
	if _, ok := lns["__name__"]; !ok {
		return fmt.Errorf("cannot find __name__ in %q", lns)
	}
	for labelName := range allLabelNames {
		if _, ok := lns[labelName]; !ok {
			return fmt.Errorf("cannot find %q in %q", labelName, lns)
		}
	}

	// Check timeseriesCounters only for serial test.
	// Concurrent test may create duplicate timeseries, so GetSeriesCount
	// would return more timeseries than needed.
	if !isConcurrent {
		n, err := db.GetSeriesCount(noDeadline)
		if err != nil {
			return fmt.Errorf("unexpected error in GetSeriesCount(): %w", err)
		}
		if n != uint64(len(timeseriesCounters)) {
			return fmt.Errorf("unexpected GetSeriesCount(); got %d; want %d", n, uint64(len(timeseriesCounters)))
		}
	}

	// Try tag filters.
	tr := TimeRange{
		MinTimestamp: timestamp - msecPerDay,
		MaxTimestamp: timestamp + msecPerDay,
	}
	for i := range mns {
		mn := &mns[i]
		tsid := &tsids[i]

		// Search without regexps.
		tfs := NewTagFilters()
		if err := tfs.Add(nil, mn.MetricGroup, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup: %w", err)
		}
		for j := 0; j < len(mn.Tags); j++ {
			t := &mn.Tags[j]
			if err := tfs.Add(t.Key, t.Value, false, false); err != nil {
				return fmt.Errorf("cannot create tag filter for tag: %w", err)
			}
		}
		if err := tfs.Add(nil, []byte("foobar"), true, false); err != nil {
			return fmt.Errorf("cannot add negative filter: %w", err)
		}
		if err := tfs.Add(nil, nil, true, false); err != nil {
			return fmt.Errorf("cannot add no-op negative filter: %w", err)
		}
		tsidsFound, err := db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by exact tag filter: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in exact tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s\ni=%d", tsid, tsidsFound, tfs, mn, i)
		}

		// Verify tag cache.
		tsidsCached, err := db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by exact tag filter: %w", err)
		}
		if !reflect.DeepEqual(tsidsCached, tsidsFound) {
			return fmt.Errorf("unexpected tsids returned; got\n%+v; want\n%+v", tsidsCached, tsidsFound)
		}

		// Add negative filter for zeroing search results.
		if err := tfs.Add(nil, mn.MetricGroup, true, false); err != nil {
			return fmt.Errorf("cannot add negative filter for zeroing search results: %w", err)
		}
		tsidsFound, err = db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by exact tag filter with full negative: %w", err)
		}
		if testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("unexpected tsid found for exact negative filter\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search for Graphite wildcard
		tfs.Reset()
		n := bytes.IndexByte(mn.MetricGroup, '.')
		if n < 0 {
			return fmt.Errorf("cannot find dot in MetricGroup %q", mn.MetricGroup)
		}
		re := "[^.]*" + regexp.QuoteMeta(string(mn.MetricGroup[n:]))
		if err := tfs.Add(nil, []byte(re), false, true); err != nil {
			return fmt.Errorf("cannot create regexp tag filter for Graphite wildcard")
		}
		tsidsFound, err = db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by regexp tag filter for Graphite wildcard: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in regexp for Graphite wildcard tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with a filter matching empty tag (a single filter)
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1601
		tfs.Reset()
		if err := tfs.Add(nil, mn.MetricGroup, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup: %w", err)
		}
		if err := tfs.Add([]byte("non-existent-tag"), []byte("foo|"), false, true); err != nil {
			return fmt.Errorf("cannot create regexp tag filter for non-existing tag: %w", err)
		}
		tsidsFound, err = db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search with a filter matching empty tag: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing when matching a filter with empty tag tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with filters matching empty tags (multiple filters)
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1601
		tfs.Reset()
		if err := tfs.Add(nil, mn.MetricGroup, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup: %w", err)
		}
		if err := tfs.Add([]byte("non-existent-tag1"), []byte("foo|"), false, true); err != nil {
			return fmt.Errorf("cannot create regexp tag filter for non-existing tag1: %w", err)
		}
		if err := tfs.Add([]byte("non-existent-tag2"), []byte("bar|"), false, true); err != nil {
			return fmt.Errorf("cannot create regexp tag filter for non-existing tag2: %w", err)
		}
		tsidsFound, err = db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search with multiple filters matching empty tags: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing when matching multiple filters with empty tags tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with regexps.
		tfs.Reset()
		if err := tfs.Add(nil, mn.MetricGroup, false, true); err != nil {
			return fmt.Errorf("cannot create regexp tag filter for MetricGroup: %w", err)
		}
		for j := 0; j < len(mn.Tags); j++ {
			t := &mn.Tags[j]
			if err := tfs.Add(t.Key, append(t.Value, "|foo*."...), false, true); err != nil {
				return fmt.Errorf("cannot create regexp tag filter for tag: %w", err)
			}
			if err := tfs.Add(t.Key, append(t.Value, "|aaa|foo|bar"...), false, true); err != nil {
				return fmt.Errorf("cannot create regexp tag filter for tag: %w", err)
			}
		}
		if err := tfs.Add(nil, []byte("^foobar$"), true, true); err != nil {
			return fmt.Errorf("cannot add negative filter with regexp: %w", err)
		}
		if err := tfs.Add(nil, nil, true, true); err != nil {
			return fmt.Errorf("cannot add no-op negative filter with regexp: %w", err)
		}
		tsidsFound, err = db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by regexp tag filter: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in regexp tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}
		if err := tfs.Add(nil, mn.MetricGroup, true, true); err != nil {
			return fmt.Errorf("cannot add negative filter for zeroing search results: %w", err)
		}
		tsidsFound, err = db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by regexp tag filter with full negative: %w", err)
		}
		if testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("unexpected tsid found for regexp negative filter\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with filter matching zero results.
		tfs.Reset()
		if err := tfs.Add([]byte("non-existing-key"), []byte("foobar"), false, false); err != nil {
			return fmt.Errorf("cannot add non-existing key: %w", err)
		}
		if err := tfs.Add(nil, mn.MetricGroup, false, true); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup matching zero results: %w", err)
		}
		tsidsFound, err = db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by non-existing tag filter: %w", err)
		}
		if len(tsidsFound) > 0 {
			return fmt.Errorf("non-zero tsidsFound for non-existing tag filter: %+v", tsidsFound)
		}

		if isConcurrent {
			// Skip empty filter search in concurrent mode, since it looks like
			// it has a lag preventing from correct work.
			continue
		}

		// Search with empty filter. It should match all the results.
		tfs.Reset()
		tsidsFound, err = db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search for common prefix: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in common prefix\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with empty metricGroup. It should match zero results.
		tfs.Reset()
		if err := tfs.Add(nil, nil, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for empty metricGroup: %w", err)
		}
		tsidsFound, err = db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search for empty metricGroup: %w", err)
		}
		if len(tsidsFound) != 0 {
			return fmt.Errorf("unexpected non-empty tsids found for empty metricGroup: %v", tsidsFound)
		}

		// Search with multiple tfss
		tfs1 := NewTagFilters()
		if err := tfs1.Add(nil, nil, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for empty metricGroup: %w", err)
		}
		tfs2 := NewTagFilters()
		if err := tfs2.Add(nil, mn.MetricGroup, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup: %w", err)
		}
		tsidsFound, err = db.SearchTSIDs(nil, []*TagFilters{tfs1, tfs2}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search for empty metricGroup: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing when searching for multiple tfss \ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Verify empty tfss
		tsidsFound, err = db.SearchTSIDs(nil, nil, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search for nil tfss: %w", err)
		}
		if len(tsidsFound) != 0 {
			return fmt.Errorf("unexpected non-empty tsids fround for nil tfss; found %d tsids", len(tsidsFound))
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

func TestGetRegexpForGraphiteNodeQuery(t *testing.T) {
	f := func(q, expectedRegexp string) {
		t.Helper()
		re, err := getRegexpForGraphiteQuery(q)
		if err != nil {
			t.Fatalf("unexpected error for query=%q: %s", q, err)
		}
		reStr := re.String()
		if reStr != expectedRegexp {
			t.Fatalf("unexpected regexp for query %q; got %q want %q", q, reStr, expectedRegexp)
		}
	}
	f(``, `^$`)
	f(`*`, `^[^.]*$`)
	f(`foo.`, `^foo\.$`)
	f(`foo.bar`, `^foo\.bar$`)
	f(`{foo,b*ar,b[a-z]}`, `^(?:foo|b[^.]*ar|b[a-z])$`)
	f(`[-a-zx.]`, `^[-a-zx.]$`)
	f(`**`, `^[^.]*[^.]*$`)
	f(`a*[de]{x,y}z`, `^a[^.]*[de](?:x|y)z$`)
	f(`foo{bar`, `^foo\{bar$`)
	f(`foo{ba,r`, `^foo\{ba,r$`)
	f(`foo[bar`, `^foo\[bar$`)
	f(`foo{bar}`, `^foobar$`)
	f(`foo{bar,,b{{a,b*},z},[x-y]*z}a`, `^foo(?:bar||b(?:(?:a|b[^.]*)|z)|[x-y][^.]*z)a$`)
}

func TestMatchTagFilters(t *testing.T) {
	var mn MetricName
	mn.MetricGroup = append(mn.MetricGroup, "foobar_metric"...)
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key %d", i)
		value := fmt.Sprintf("value %d", i)
		mn.AddTag(key, value)
	}
	var bb bytesutil.ByteBuffer

	var tfs TagFilters
	tfs.Reset()
	if err := tfs.Add(nil, []byte("foobar_metric"), false, false); err != nil {
		t.Fatalf("cannot add filter: %s", err)
	}
	ok, err := matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("should match")
	}

	// Empty tag filters should match.
	tfs.Reset()
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("empty tag filters should match")
	}

	// Negative match by MetricGroup
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
	if err := tfs.Add(nil, []byte(".+"), true, true); err != nil {
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
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
	if err := tfs.Add(nil, []byte(".+"), false, true); err != nil {
		t.Fatalf("cannot add regexp, positive filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}

	// Positive empty match by non-existing tag
	tfs.Reset()
	if err := tfs.Add([]byte("non-existing-tag"), []byte("foobar|"), false, true); err != nil {
		t.Fatalf("cannot add regexp, positive filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}

	// Negative match by non-existing tag
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
	if err := tfs.Add([]byte("non-existing-tag"), []byte(".+"), false, true); err != nil {
		t.Fatalf("cannot add regexp, non-negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}

	// Negative match by existing tag
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
	if err := tfs.Add([]byte("key 3"), []byte(".+"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}

	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/546
	tfs.Reset()
	if err := tfs.Add([]byte("key 3"), []byte("|value 3"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}
	tfs.Reset()
	if err := tfs.Add([]byte("key 3"), []byte("|value 2"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}

	// Positive match by existing tag
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
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
	tfs.Reset()
	if err := tfs.Add([]byte("key 3"), []byte("v.+lue 2|"), true, true); err != nil {
		t.Fatalf("cannot add regexp, negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}
	tfs.Reset()
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
	tfs.Reset()
	if err := tfs.Add([]byte("key 3"), []byte(".+"), false, true); err != nil {
		t.Fatalf("cannot add regexp, non-negative filter: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Should match")
	}

	// Positive match by multiple tags and MetricGroup
	tfs.Reset()
	if err := tfs.Add([]byte("key 0"), []byte("value 0"), false, false); err != nil {
		t.Fatalf("cannot add no regexp, no negative filter: %s", err)
	}
	if err := tfs.Add([]byte("key 2"), []byte("value [0-9]"), false, true); err != nil {
		t.Fatalf("cannot add regexp, no negative filter: %s", err)
	}
	if err := tfs.Add([]byte("key 3"), []byte("value 23"), true, false); err != nil {
		t.Fatalf("cannot add no regexp, negative filter: %s", err)
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
	tfs.Reset()
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

	// Negative match for multiple non-regexp positive filters
	tfs.Reset()
	if err := tfs.Add(nil, []byte("foobar_metric"), false, false); err != nil {
		t.Fatalf("cannot add non-regexp positive filter for MetricGroup: %s", err)
	}
	if err := tfs.Add([]byte("non-existing-metric"), []byte("foobar"), false, false); err != nil {
		t.Fatalf("cannot add non-regexp positive filter for non-existing tag: %s", err)
	}
	ok, err = matchTagFilters(&mn, toTFPointers(tfs.tfs), &bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Shouldn't match")
	}
}

func TestSearchTSIDWithTimeRange(t *testing.T) {
	// TODO: @f41gh7 refactor this test:
	// create a new test for LabelNames
	// move exist LabelVales tests into TestSearchLabelValues
	const path = "TestSearchTSIDWithTimeRange"
	// Create a bunch of per-day time series
	const days = 5
	const metricsPerDay = 1000
	timestamp := time.Date(2019, time.October, 15, 5, 1, 0, 0, time.UTC).UnixMilli()
	baseDate := uint64(timestamp) / msecPerDay
	var metricNameBuf []byte
	perDayMetricIDs := make(map[uint64]*uint64set.Set)
	var allMetricIDs uint64set.Set
	labelNames := []string{
		"__name__", "constant", "day", "UniqueId", "some_unique_id",
	}
	labelValues := []string{
		"testMetric",
	}
	sort.Strings(labelNames)

	newMN := func(name string, day, metric int) MetricName {
		var mn MetricName
		mn.MetricGroup = []byte(name)
		mn.AddTag(
			"constant",
			"const",
		)
		mn.AddTag(
			"day",
			fmt.Sprintf("%v", day),
		)
		mn.AddTag(
			"UniqueId",
			fmt.Sprintf("%v", metric),
		)
		mn.AddTag(
			"some_unique_id",
			fmt.Sprintf("%v", day),
		)
		mn.sortTags()
		return mn
	}

	s := MustOpenStorage(path, OpenOptions{})
	ptw := s.tb.MustGetPartition(timestamp)
	db := ptw.pt.idb
	is := db.getIndexSearch(noDeadline)

	for day := 0; day < days; day++ {
		date := baseDate - uint64(day)
		var metricIDs uint64set.Set
		for metric := 0; metric < metricsPerDay; metric++ {
			mn := newMN("testMetric", day, metric)
			metricNameBuf = mn.Marshal(metricNameBuf[:0])
			var tsid TSID
			if !is.getTSIDByMetricName(&tsid, metricNameBuf, date) {
				generateTSID(&tsid, &mn)
				createAllIndexesForMetricName(db, &mn, &tsid, date)
			}
			metricIDs.Add(tsid.MetricID)
		}

		allMetricIDs.Union(&metricIDs)
		perDayMetricIDs[date] = &metricIDs
	}
	db.putIndexSearch(is)

	// Flush index to disk, so it becomes visible for search
	db.tb.DebugFlush()

	is2 := db.getIndexSearch(noDeadline)

	// Check that all the metrics are found for all the days.
	for date := baseDate - days + 1; date <= baseDate; date++ {
		metricIDs, err := is2.getMetricIDsForDate(date, metricsPerDay)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !perDayMetricIDs[date].Equal(metricIDs) {
			t.Fatalf("unexpected metricIDs found;\ngot\n%d\nwant\n%d", metricIDs.AppendTo(nil), perDayMetricIDs[date].AppendTo(nil))
		}
	}

	// Check that all the metrics are found in global index
	metricIDs, err := is2.getMetricIDsForDate(0, metricsPerDay*days)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !allMetricIDs.Equal(metricIDs) {
		t.Fatalf("unexpected metricIDs found;\ngot\n%d\nwant\n%d", metricIDs.AppendTo(nil), allMetricIDs.AppendTo(nil))
	}
	db.putIndexSearch(is2)

	// add a metric that will be deleted shortly
	is3 := db.getIndexSearch(noDeadline)
	day := days
	date := baseDate - uint64(day)
	mn := newMN("deletedMetric", day, 999)
	mn.AddTag(
		"labelToDelete",
		fmt.Sprintf("%v", day),
	)
	mn.sortTags()
	metricNameBuf = mn.Marshal(metricNameBuf[:0])
	var tsid TSID
	if !is3.getTSIDByMetricName(&tsid, metricNameBuf, date) {
		generateTSID(&tsid, &mn)
		createAllIndexesForMetricName(db, &mn, &tsid, date)
	}
	// delete the added metric. It is expected it won't be returned during searches
	deletedSet := &uint64set.Set{}
	deletedSet.Add(tsid.MetricID)
	db.setDeletedMetricIDs(deletedSet)
	db.putIndexSearch(is3)
	db.tb.DebugFlush()

	// Check SearchLabelNames with the specified time range.
	tr := TimeRange{
		MinTimestamp: timestamp - msecPerDay,
		MaxTimestamp: timestamp,
	}
	lns, err := db.SearchLabelNames(nil, nil, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelNames(timeRange=%s): %s", &tr, err)
	}
	got := sortedSlice(lns)
	if !reflect.DeepEqual(got, labelNames) {
		t.Fatalf("unexpected labelNames; got\n%s\nwant\n%s", got, labelNames)
	}

	// Check SearchLabelValues with the specified time range.
	lvs, err := db.SearchLabelValues(nil, "", nil, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelValues(timeRange=%s): %s", &tr, err)
	}
	got = sortedSlice(lvs)
	if !reflect.DeepEqual(got, labelValues) {
		t.Fatalf("unexpected labelValues; got\n%s\nwant\n%s", got, labelValues)
	}

	// Create a filter that will match series that occur across multiple days
	tfs := NewTagFilters()
	if err := tfs.Add([]byte("constant"), []byte("const"), false, false); err != nil {
		t.Fatalf("cannot add filter: %s", err)
	}
	tfsMetricName := NewTagFilters()
	if err := tfsMetricName.Add([]byte("constant"), []byte("const"), false, false); err != nil {
		t.Fatalf("cannot add filter on label: %s", err)
	}
	if err := tfsMetricName.Add(nil, []byte("testMetric"), false, false); err != nil {
		t.Fatalf("cannot add filter on metric name: %s", err)
	}
	tfsComposite := NewTagFilters()
	if err := tfsComposite.Add(nil, []byte("testMetric"), false, false); err != nil {
		t.Fatalf("cannot add filter: %s", err)
	}

	// Perform a search within a day.
	// This should return the metrics for the day
	tr = TimeRange{
		MinTimestamp: timestamp - 2*msecPerHour - 1,
		MaxTimestamp: timestamp,
	}
	matchedTSIDs, err := db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
	if err != nil {
		t.Fatalf("error searching tsids: %v", err)
	}
	if len(matchedTSIDs) != metricsPerDay {
		t.Fatalf("expected %d time series for current day, got %d time series", metricsPerDay, len(matchedTSIDs))
	}

	// Check SearchLabelNames with the specified filter.
	lns, err = db.SearchLabelNames(nil, []*TagFilters{tfs}, TimeRange{}, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelNames(filters=%s): %s", tfs, err)
	}
	got = sortedSlice(lns)
	if !reflect.DeepEqual(got, labelNames) {
		t.Fatalf("unexpected labelNames; got\n%s\nwant\n%s", got, labelNames)
	}

	// Check SearchLabelNames with the specified filter and time range.
	lns, err = db.SearchLabelNames(nil, []*TagFilters{tfs}, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelNames(filters=%s, timeRange=%s): %s", tfs, &tr, err)
	}
	got = sortedSlice(lns)
	if !reflect.DeepEqual(got, labelNames) {
		t.Fatalf("unexpected labelNames; got\n%s\nwant\n%s", got, labelNames)
	}

	// Check SearchLabelNames with filters on metric name and time range.
	lns, err = db.SearchLabelNames(nil, []*TagFilters{tfsMetricName}, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelNames(filters=%s, timeRange=%s): %s", tfs, &tr, err)
	}
	got = sortedSlice(lns)
	if !reflect.DeepEqual(got, labelNames) {
		t.Fatalf("unexpected labelNames; got\n%s\nwant\n%s", got, labelNames)
	}

	// Check SearchLabelNames with filters on composite key and time range.
	lns, err = db.SearchLabelNames(nil, []*TagFilters{tfsComposite}, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelNames(filters=%s, timeRange=%s): %s", tfs, &tr, err)
	}
	got = sortedSlice(lns)
	if !reflect.DeepEqual(got, labelNames) {
		t.Fatalf("unexpected labelNames; got\n%s\nwant\n%s", got, labelNames)
	}

	// Check SearchLabelValues with the specified filter.
	lvs, err = db.SearchLabelValues(nil, "", []*TagFilters{tfs}, TimeRange{}, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelValues(filters=%s): %s", tfs, err)
	}
	got = sortedSlice(lvs)
	if !reflect.DeepEqual(got, labelValues) {
		t.Fatalf("unexpected labelValues; got\n%s\nwant\n%s", got, labelValues)
	}

	// Check SearchLabelValues with the specified filter and time range.
	lvs, err = db.SearchLabelValues(nil, "", []*TagFilters{tfs}, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelValues(filters=%s, timeRange=%s): %s", tfs, &tr, err)
	}
	got = sortedSlice(lvs)
	if !reflect.DeepEqual(got, labelValues) {
		t.Fatalf("unexpected labelValues; got\n%s\nwant\n%s", got, labelValues)
	}

	// Check SearchLabelValues with filters on metric name and time range.
	lvs, err = db.SearchLabelValues(nil, "", []*TagFilters{tfsMetricName}, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelValues(filters=%s, timeRange=%s): %s", tfs, &tr, err)
	}
	got = sortedSlice(lvs)
	if !reflect.DeepEqual(got, labelValues) {
		t.Fatalf("unexpected labelValues; got\n%s\nwant\n%s", got, labelValues)
	}

	// Check SearchLabelValues with filters on composite key and time range.
	lvs, err = db.SearchLabelValues(nil, "constant", []*TagFilters{tfsComposite}, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelValues(filters=%s, timeRange=%s): %s", tfs, &tr, err)
	}
	got = sortedSlice(lvs)
	labelValues = []string{"const"}
	if !reflect.DeepEqual(got, labelValues) {
		t.Fatalf("unexpected labelValues; got\n%s\nwant\n%s", got, labelValues)
	}

	// Perform a search across all the days, should match all metrics
	tr = TimeRange{
		MinTimestamp: timestamp - msecPerDay*days,
		MaxTimestamp: timestamp,
	}

	matchedTSIDs, err = db.SearchTSIDs(nil, []*TagFilters{tfs}, tr, 1e5, noDeadline)
	if err != nil {
		t.Fatalf("error searching tsids: %v", err)
	}
	if len(matchedTSIDs) != metricsPerDay*days {
		t.Fatalf("expected %d time series for all days, got %d time series", metricsPerDay*days, len(matchedTSIDs))
	}

	// Check GetTSDBStatus with nil filters.
	status, err := db.GetTSDBStatus(nil, nil, baseDate, "day", 5, 1e6, noDeadline)
	if err != nil {
		t.Fatalf("error in GetTSDBStatus with nil filters: %s", err)
	}
	if !status.hasEntries() {
		t.Fatalf("expecting non-empty TSDB status")
	}
	expectedSeriesCountByMetricName := []TopHeapEntry{
		{
			Name:  "testMetric",
			Count: 1000,
		},
	}
	if !reflect.DeepEqual(status.SeriesCountByMetricName, expectedSeriesCountByMetricName) {
		t.Fatalf("unexpected SeriesCountByMetricName;\ngot\n%v\nwant\n%v", status.SeriesCountByMetricName, expectedSeriesCountByMetricName)
	}
	expectedSeriesCountByLabelName := []TopHeapEntry{
		{
			Name:  "UniqueId",
			Count: 1000,
		},
		{
			Name:  "__name__",
			Count: 1000,
		},
		{
			Name:  "constant",
			Count: 1000,
		},
		{
			Name:  "day",
			Count: 1000,
		},
		{
			Name:  "some_unique_id",
			Count: 1000,
		},
	}
	if !reflect.DeepEqual(status.SeriesCountByLabelName, expectedSeriesCountByLabelName) {
		t.Fatalf("unexpected SeriesCountByLabelName;\ngot\n%v\nwant\n%v", status.SeriesCountByLabelName, expectedSeriesCountByLabelName)
	}
	expectedSeriesCountByFocusLabelValue := []TopHeapEntry{
		{
			Name:  "0",
			Count: 1000,
		},
	}
	if !reflect.DeepEqual(status.SeriesCountByFocusLabelValue, expectedSeriesCountByFocusLabelValue) {
		t.Fatalf("unexpected SeriesCountByFocusLabelValue;\ngot\n%v\nwant\n%v", status.SeriesCountByFocusLabelValue, expectedSeriesCountByFocusLabelValue)
	}
	expectedLabelValueCountByLabelName := []TopHeapEntry{
		{
			Name:  "UniqueId",
			Count: 1000,
		},
		{
			Name:  "__name__",
			Count: 1,
		},
		{
			Name:  "constant",
			Count: 1,
		},
		{
			Name:  "day",
			Count: 1,
		},
		{
			Name:  "some_unique_id",
			Count: 1,
		},
	}
	if !reflect.DeepEqual(status.LabelValueCountByLabelName, expectedLabelValueCountByLabelName) {
		t.Fatalf("unexpected LabelValueCountByLabelName;\ngot\n%v\nwant\n%v", status.LabelValueCountByLabelName, expectedLabelValueCountByLabelName)
	}
	expectedSeriesCountByLabelValuePair := []TopHeapEntry{
		{
			Name:  "__name__=testMetric",
			Count: 1000,
		},
		{
			Name:  "constant=const",
			Count: 1000,
		},
		{
			Name:  "day=0",
			Count: 1000,
		},
		{
			Name:  "some_unique_id=0",
			Count: 1000,
		},
		{
			Name:  "UniqueId=1",
			Count: 1,
		},
	}
	if !reflect.DeepEqual(status.SeriesCountByLabelValuePair, expectedSeriesCountByLabelValuePair) {
		t.Fatalf("unexpected SeriesCountByLabelValuePair;\ngot\n%v\nwant\n%v", status.SeriesCountByLabelValuePair, expectedSeriesCountByLabelValuePair)
	}
	expectedTotalSeries := uint64(1000)
	if status.TotalSeries != expectedTotalSeries {
		t.Fatalf("unexpected TotalSeries; got %d; want %d", status.TotalSeries, expectedTotalSeries)
	}
	expectedLabelValuePairs := uint64(5000)
	if status.TotalLabelValuePairs != expectedLabelValuePairs {
		t.Fatalf("unexpected TotalLabelValuePairs; got %d; want %d", status.TotalLabelValuePairs, expectedLabelValuePairs)
	}

	// Check GetTSDBStatus with non-nil filter, which matches all the series
	tfs = NewTagFilters()
	if err := tfs.Add([]byte("day"), []byte("0"), false, false); err != nil {
		t.Fatalf("cannot add filter: %s", err)
	}
	status, err = db.GetTSDBStatus(nil, []*TagFilters{tfs}, baseDate, "", 5, 1e6, noDeadline)
	if err != nil {
		t.Fatalf("error in GetTSDBStatus: %s", err)
	}
	if !status.hasEntries() {
		t.Fatalf("expecting non-empty TSDB status")
	}
	expectedSeriesCountByMetricName = []TopHeapEntry{
		{
			Name:  "testMetric",
			Count: 1000,
		},
	}
	if !reflect.DeepEqual(status.SeriesCountByMetricName, expectedSeriesCountByMetricName) {
		t.Fatalf("unexpected SeriesCountByMetricName;\ngot\n%v\nwant\n%v", status.SeriesCountByMetricName, expectedSeriesCountByMetricName)
	}
	expectedTotalSeries = 1000
	if status.TotalSeries != expectedTotalSeries {
		t.Fatalf("unexpected TotalSeries; got %d; want %d", status.TotalSeries, expectedTotalSeries)
	}
	expectedLabelValuePairs = 5000
	if status.TotalLabelValuePairs != expectedLabelValuePairs {
		t.Fatalf("unexpected TotalLabelValuePairs; got %d; want %d", status.TotalLabelValuePairs, expectedLabelValuePairs)
	}

	// Check GetTSDBStatus, which matches all the series on a global time range
	status, err = db.GetTSDBStatus(nil, nil, 0, "day", 5, 1e6, noDeadline)
	if err != nil {
		t.Fatalf("error in GetTSDBStatus: %s", err)
	}
	if !status.hasEntries() {
		t.Fatalf("expecting non-empty TSDB status")
	}
	expectedSeriesCountByMetricName = []TopHeapEntry{
		{
			Name:  "testMetric",
			Count: 5000,
		},
	}
	if !reflect.DeepEqual(status.SeriesCountByMetricName, expectedSeriesCountByMetricName) {
		t.Fatalf("unexpected SeriesCountByMetricName;\ngot\n%v\nwant\n%v", status.SeriesCountByMetricName, expectedSeriesCountByMetricName)
	}
	expectedTotalSeries = 5000
	if status.TotalSeries != expectedTotalSeries {
		t.Fatalf("unexpected TotalSeries; got %d; want %d", status.TotalSeries, expectedTotalSeries)
	}
	expectedLabelValuePairs = 25000
	if status.TotalLabelValuePairs != expectedLabelValuePairs {
		t.Fatalf("unexpected TotalLabelValuePairs; got %d; want %d", status.TotalLabelValuePairs, expectedLabelValuePairs)
	}
	expectedSeriesCountByFocusLabelValue = []TopHeapEntry{
		{
			Name:  "0",
			Count: 1000,
		},
		{
			Name:  "1",
			Count: 1000,
		},
		{
			Name:  "2",
			Count: 1000,
		},
		{
			Name:  "3",
			Count: 1000,
		},
		{
			Name:  "4",
			Count: 1000,
		},
	}
	if !reflect.DeepEqual(status.SeriesCountByFocusLabelValue, expectedSeriesCountByFocusLabelValue) {
		t.Fatalf("unexpected SeriesCountByFocusLabelValue;\ngot\n%v\nwant\n%v", status.SeriesCountByFocusLabelValue, expectedSeriesCountByFocusLabelValue)
	}

	// Check GetTSDBStatus with non-nil filter, which matches only 3 series
	tfs = NewTagFilters()
	if err := tfs.Add([]byte("UniqueId"), []byte("0|1|3"), false, true); err != nil {
		t.Fatalf("cannot add filter: %s", err)
	}
	status, err = db.GetTSDBStatus(nil, []*TagFilters{tfs}, baseDate, "", 5, 1e6, noDeadline)
	if err != nil {
		t.Fatalf("error in GetTSDBStatus: %s", err)
	}
	if !status.hasEntries() {
		t.Fatalf("expecting non-empty TSDB status")
	}
	expectedSeriesCountByMetricName = []TopHeapEntry{
		{
			Name:  "testMetric",
			Count: 3,
		},
	}
	if !reflect.DeepEqual(status.SeriesCountByMetricName, expectedSeriesCountByMetricName) {
		t.Fatalf("unexpected SeriesCountByMetricName;\ngot\n%v\nwant\n%v", status.SeriesCountByMetricName, expectedSeriesCountByMetricName)
	}
	expectedTotalSeries = 3
	if status.TotalSeries != expectedTotalSeries {
		t.Fatalf("unexpected TotalSeries; got %d; want %d", status.TotalSeries, expectedTotalSeries)
	}
	expectedLabelValuePairs = 15
	if status.TotalLabelValuePairs != expectedLabelValuePairs {
		t.Fatalf("unexpected TotalLabelValuePairs; got %d; want %d", status.TotalLabelValuePairs, expectedLabelValuePairs)
	}

	// Check GetTSDBStatus with non-nil filter on global time range, which matches only 15 series
	status, err = db.GetTSDBStatus(nil, []*TagFilters{tfs}, 0, "", 5, 1e6, noDeadline)
	if err != nil {
		t.Fatalf("error in GetTSDBStatus: %s", err)
	}
	if !status.hasEntries() {
		t.Fatalf("expecting non-empty TSDB status")
	}
	expectedSeriesCountByMetricName = []TopHeapEntry{
		{
			Name:  "testMetric",
			Count: 15,
		},
	}
	if !reflect.DeepEqual(status.SeriesCountByMetricName, expectedSeriesCountByMetricName) {
		t.Fatalf("unexpected SeriesCountByMetricName;\ngot\n%v\nwant\n%v", status.SeriesCountByMetricName, expectedSeriesCountByMetricName)
	}
	expectedTotalSeries = 15
	if status.TotalSeries != expectedTotalSeries {
		t.Fatalf("unexpected TotalSeries; got %d; want %d", status.TotalSeries, expectedTotalSeries)
	}
	expectedLabelValuePairs = 75
	if status.TotalLabelValuePairs != expectedLabelValuePairs {
		t.Fatalf("unexpected TotalLabelValuePairs; got %d; want %d", status.TotalLabelValuePairs, expectedLabelValuePairs)
	}

	s.tb.PutPartition(ptw)
	s.MustClose()
	fs.MustRemoveDir(path)
}

func toTFPointers(tfs []tagFilter) []*tagFilter {
	tfps := make([]*tagFilter, len(tfs))
	for i := range tfs {
		tfps[i] = &tfs[i]
	}
	return tfps
}

func newTestStorage() *Storage {
	s := &Storage{
		cachePath: "test-storage-cache",

		metricIDCache:   workingsetcache.New(1234),
		metricNameCache: workingsetcache.New(1234),
		tsidCache:       workingsetcache.New(1234),
		retentionMsecs:  retentionMax.Milliseconds(),
	}
	return s
}

func stopTestStorage(s *Storage) {
	s.metricIDCache.Stop()
	s.metricNameCache.Stop()
	s.tsidCache.Stop()
	fs.MustRemoveDir(s.cachePath)
}

func sortedSlice(m map[string]struct{}) []string {
	s := make([]string, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	slices.Sort(s)
	return s
}

func TestIndexSearchLegacyContainsTimeRange_Concurrent(t *testing.T) {
	defer testRemoveAll(t)

	// Create storage because indexDB depends on it.
	s := MustOpenStorage(filepath.Join(t.Name(), "storage"), OpenOptions{})
	defer s.MustClose()

	idbName := "test"
	idbPath := filepath.Join(t.Name(), indexdbDirname, idbName)
	var readOnly atomic.Bool
	readOnly.Store(true)
	noRegisterNewSeries := true
	idb := mustOpenIndexDB(123, TimeRange{}, idbName, idbPath, s, &readOnly, noRegisterNewSeries)
	defer idb.MustClose()

	minTimestamp := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	concurrency := int64(100)
	var wg sync.WaitGroup
	for i := range concurrency {
		ts := minTimestamp + msecPerDay*i
		wg.Go(func() {
			is := idb.getIndexSearch(noDeadline)
			_ = is.legacyContainsTimeRange(TimeRange{ts, ts})
			idb.putIndexSearch(is)
		})
	}
	wg.Wait()

	key := marshalCommonPrefix(nil, nsPrefixDateToMetricID)
	if got, want := idb.legacyMinMissingTimestampByKey[string(key)], minTimestamp; got != want {
		t.Fatalf("unexpected min timestamp: got %v, want %v", time.UnixMilli(got).UTC(), time.UnixMilli(want).UTC())
	}
}

func TestSearchLabelValues(t *testing.T) {
	const path = "TestSearchLabelValues"
	// Create a bunch of per-day time series
	const days = 5
	const metricsPerDay = 1000
	timestamp := time.Date(2019, time.October, 15, 5, 1, 0, 0, time.UTC).UnixMilli()
	baseDate := uint64(timestamp) / msecPerDay
	var metricNameBuf []byte
	perDayMetricIDs := make(map[uint64]*uint64set.Set)
	var allMetricIDs uint64set.Set
	uniqLabelNames := make(map[string]struct{})

	newMN := func(name string, day, metric int) MetricName {
		var mn MetricName
		metricName := fmt.Sprintf("%s_%d", name, metric)
		if _, ok := uniqLabelNames[metricName]; !ok {
			uniqLabelNames[metricName] = struct{}{}
		}
		mn.MetricGroup = []byte(metricName)
		mn.AddTag(
			"constant",
			"const",
		)
		mn.AddTag(
			"day",
			fmt.Sprintf("%v", day),
		)
		mn.AddTag(
			"UniqueId",
			fmt.Sprintf("%v", metric),
		)
		mn.AddTag(
			"some_unique_id",
			fmt.Sprintf("%v", day),
		)
		mn.sortTags()
		return mn
	}

	s := MustOpenStorage(path, OpenOptions{})
	ptw := s.tb.MustGetPartition(timestamp)
	db := ptw.pt.idb
	is := db.getIndexSearch(noDeadline)

	for day := range days {
		date := baseDate - uint64(day)
		var metricIDs uint64set.Set
		for metric := range metricsPerDay {
			mn := newMN("testMetric", day, metric)
			metricNameBuf = mn.Marshal(metricNameBuf[:0])
			var tsid TSID
			if !is.getTSIDByMetricName(&tsid, metricNameBuf, date) {
				generateTSID(&tsid, &mn)
				createAllIndexesForMetricName(db, &mn, &tsid, date)
			}
			metricIDs.Add(tsid.MetricID)
		}

		allMetricIDs.Union(&metricIDs)
		perDayMetricIDs[date] = &metricIDs
	}
	db.putIndexSearch(is)

	labelValues := sortedSlice(uniqLabelNames)

	// Flush index to disk, so it becomes visible for search
	db.tb.DebugFlush()

	is2 := db.getIndexSearch(noDeadline)

	// Check that all the metrics are found for all the days.
	for date := baseDate - days + 1; date <= baseDate; date++ {
		metricIDs, err := is2.getMetricIDsForDate(date, metricsPerDay)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !perDayMetricIDs[date].Equal(metricIDs) {
			t.Fatalf("unexpected metricIDs found;\ngot\n%d\nwant\n%d", metricIDs.AppendTo(nil), perDayMetricIDs[date].AppendTo(nil))
		}
	}

	// Check that all the metrics are found in global index
	metricIDs, err := is2.getMetricIDsForDate(0, metricsPerDay*days)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !allMetricIDs.Equal(metricIDs) {
		t.Fatalf("unexpected metricIDs found;\ngot\n%d\nwant\n%d", metricIDs.AppendTo(nil), allMetricIDs.AppendTo(nil))
	}
	db.putIndexSearch(is2)

	// Check SearchLabelNames with the specified time range.
	tr := TimeRange{
		MinTimestamp: timestamp - msecPerDay,
		MaxTimestamp: timestamp,
	}

	// Check SearchLabelValues with the specified time range.
	lvs, err := db.SearchLabelValues(nil, "", nil, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelValues(timeRange=%s): %s", &tr, err)
	}
	got := sortedSlice(lvs)
	if !reflect.DeepEqual(got, labelValues) {
		t.Fatalf("unexpected labelValues; got\n%s\nwant\n%s", got, labelValues)
	}

	tfsMetricNameRe := NewTagFilters()
	if err := tfsMetricNameRe.Add([]byte("constant"), []byte("const"), false, false); err != nil {
		t.Fatalf("cannot add filter on label: %s", err)
	}
	if err := tfsMetricNameRe.Add(nil, []byte("testMetric_99.*"), false, true); err != nil {
		t.Fatalf("cannot add filter on metric name: %s", err)
	}
	// Check SearchLabelValues with the specified time range and tfs matches correct results
	// if filter result exceeds quick search limit
	originValue := maxMetricIDsForDirectLabelsLookup
	maxMetricIDsForDirectLabelsLookup = 10
	defer func() {
		maxMetricIDsForDirectLabelsLookup = originValue
	}()
	lvs, err = db.SearchLabelValues(nil, "__name__", []*TagFilters{tfsMetricNameRe}, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelValues(timeRange=%s): %s", &tr, err)
	}
	got = sortedSlice(lvs)
	labelValuesReMatch := []string{"testMetric_99", "testMetric_990", "testMetric_991", "testMetric_992", "testMetric_993", "testMetric_994", "testMetric_995", "testMetric_996", "testMetric_997", "testMetric_998", "testMetric_999"}
	if !reflect.DeepEqual(got, labelValuesReMatch) {
		t.Fatalf("unexpected labelValues; got\n%s\nwant\n%s", got, labelValuesReMatch)
	}

	s.tb.PutPartition(ptw)
	s.MustClose()
	fs.MustRemoveDir(path)
}
