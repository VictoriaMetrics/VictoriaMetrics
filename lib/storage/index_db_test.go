package storage

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
)

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
		var itemsB [][]byte
		for _, item := range items {
			data = append(data, item...)
			itemsB = append(itemsB, data[len(data)-len(item):])
		}
		if !checkItemsSorted(itemsB) {
			t.Fatalf("source items aren't sorted; items:\n%q", itemsB)
		}
		resultData, resultItemsB := mergeTagToMetricIDsRows(data, itemsB)
		if len(resultItemsB) != len(expectedItems) {
			t.Fatalf("unexpected len(resultItemsB); got %d; want %d", len(resultItemsB), len(expectedItems))
		}
		if !checkItemsSorted(resultItemsB) {
			t.Fatalf("result items aren't sorted; items:\n%q", resultItemsB)
		}
		for i, item := range resultItemsB {
			if !bytes.HasPrefix(resultData, item) {
				t.Fatalf("unexpected prefix for resultData #%d;\ngot\n%X\nwant\n%X", i, resultData, item)
			}
			resultData = resultData[len(item):]
		}
		if len(resultData) != 0 {
			t.Fatalf("unexpected tail left in resultData: %X", resultData)
		}
		var resultItems []string
		for _, item := range resultItemsB {
			resultItems = append(resultItems, string(item))
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
		mns, tsids, err := testIndexDBGetOrCreateTSIDByName(db, metricGroups)
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
				mns, tsid, err := testIndexDBGetOrCreateTSIDByName(db, metricGroups)
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
					errors = append(errors, fmt.Errorf("unexpected error: %w", err))
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

func testIndexDBGetOrCreateTSIDByName(db *indexDB, metricGroups int) ([]MetricName, []TSID, error) {
	// Create tsids.
	var mns []MetricName
	var tsids []TSID

	is := db.getIndexSearch()
	defer db.putIndexSearch(is)

	var metricNameBuf []byte
	for i := 0; i < 4e2+1; i++ {
		var mn MetricName

		// Init MetricGroup.
		mn.MetricGroup = []byte(fmt.Sprintf("metricGroup.%d\x00\x01\x02", i%metricGroups))

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
			return nil, nil, fmt.Errorf("unexpected error when creating tsid for mn:\n%s: %w", &mn, err)
		}

		mns = append(mns, mn)
		tsids = append(tsids, tsid)
	}

	// fill Date -> MetricID cache
	date := uint64(timestampFromTime(time.Now())) / msecPerDay
	for i := range tsids {
		tsid := &tsids[i]
		if err := is.storeDateMetricID(date, tsid.MetricID); err != nil {
			return nil, nil, fmt.Errorf("error in storeDateMetricID(%d, %d): %w", date, tsid.MetricID, err)
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

	timeseriesCounters := make(map[uint64]bool)
	var tsidCopy TSID
	var metricNameCopy []byte
	allKeys := make(map[string]bool)
	for i := range mns {
		mn := &mns[i]
		tsid := &tsids[i]

		tc := timeseriesCounters
		tc[tsid.MetricID] = true

		mn.sortTags()
		metricName := mn.Marshal(nil)

		if err := db.getTSIDByNameNoCreate(&tsidCopy, metricName); err != nil {
			return fmt.Errorf("cannot obtain tsid #%d for mn %s: %w", i, mn, err)
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
		metricNameCopy, err = db.searchMetricName(metricNameCopy[:0], tsidCopy.MetricID)
		if err != nil {
			return fmt.Errorf("error in searchMetricName for metricID=%d; i=%d: %w", tsidCopy.MetricID, i, err)
		}
		if !bytes.Equal(metricName, metricNameCopy) {
			return fmt.Errorf("unexpected mn for metricID=%d;\ngot\n%q\nwant\n%q", tsidCopy.MetricID, metricNameCopy, metricName)
		}

		// Try searching metric name for non-existent MetricID.
		buf, err := db.searchMetricName(nil, 1)
		if err != io.EOF {
			return fmt.Errorf("expecting io.EOF error when searching for non-existing metricID; got %v", err)
		}
		if len(buf) > 0 {
			return fmt.Errorf("expecting empty buf when searching for non-existent metricID; got %X", buf)
		}

		// Test SearchTagValues
		tvs, err := db.SearchTagValues(nil, 1e5)
		if err != nil {
			return fmt.Errorf("error in SearchTagValues for __name__: %w", err)
		}
		if !hasValue(tvs, mn.MetricGroup) {
			return fmt.Errorf("SearchTagValues couldn't find %q; found %q", mn.MetricGroup, tvs)
		}
		for i := range mn.Tags {
			tag := &mn.Tags[i]
			tvs, err := db.SearchTagValues(tag.Key, 1e5)
			if err != nil {
				return fmt.Errorf("error in SearchTagValues for __name__: %w", err)
			}
			if !hasValue(tvs, tag.Value) {
				return fmt.Errorf("SearchTagValues couldn't find %q=%q; found %q", tag.Key, tag.Value, tvs)
			}
			allKeys[string(tag.Key)] = true
		}
	}

	// Test SearchTagKeys
	tks, err := db.SearchTagKeys(1e5)
	if err != nil {
		return fmt.Errorf("error in SearchTagKeys: %w", err)
	}
	if !hasValue(tks, nil) {
		return fmt.Errorf("cannot find __name__ in %q", tks)
	}
	for key := range allKeys {
		if !hasValue(tks, []byte(key)) {
			return fmt.Errorf("cannot find %q in %q", key, tks)
		}
	}

	// Check timerseriesCounters only for serial test.
	// Concurrent test may create duplicate timeseries, so GetSeriesCount
	// would return more timeseries than needed.
	if !isConcurrent {
		n, err := db.GetSeriesCount()
		if err != nil {
			return fmt.Errorf("unexpected error in GetSeriesCount(): %w", err)
		}
		if n != uint64(len(timeseriesCounters)) {
			return fmt.Errorf("unexpected GetSeriesCount(); got %d; want %d", n, uint64(len(timeseriesCounters)))
		}
	}

	// Try tag filters.
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
		tsidsFound, err := db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search by exact tag filter: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in exact tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s\ni=%d", tsid, tsidsFound, tfs, mn, i)
		}

		// Verify tag cache.
		tsidsCached, err := db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
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
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
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
		if tfsNew := tfs.Finalize(); len(tfsNew) > 0 {
			return fmt.Errorf("unexpected non-empty tag filters returned by TagFilters.Finalize: %v", tfsNew)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search by regexp tag filter for Graphite wildcard: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in regexp for Graphite wildcard tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
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
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search by regexp tag filter: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in regexp tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}
		if err := tfs.Add(nil, mn.MetricGroup, true, true); err != nil {
			return fmt.Errorf("cannot add negative filter for zeroing search results: %w", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
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
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
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
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
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
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, TimeRange{}, 1e5)
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
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs1, tfs2}, TimeRange{}, 1e5)
		if err != nil {
			return fmt.Errorf("cannot search for empty metricGroup: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing when searching for multiple tfss \ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Verify empty tfss
		tsidsFound, err = db.searchTSIDs(nil, TimeRange{}, 1e5)
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
	metricIDCache := workingsetcache.New(1234, time.Hour)
	metricNameCache := workingsetcache.New(1234, time.Hour)
	defer metricIDCache.Stop()
	defer metricNameCache.Stop()

	currMetricIDs := &hourMetricIDs{
		isFull: true,
		m:      &uint64set.Set{},
	}

	var hmCurr atomic.Value
	hmCurr.Store(currMetricIDs)

	prevMetricIDs := &hourMetricIDs{
		isFull: true,
		m:      &uint64set.Set{},
	}
	var hmPrev atomic.Value
	hmPrev.Store(prevMetricIDs)

	dbName := "test-index-db-ts-range"
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

	is := db.getIndexSearch()
	defer db.putIndexSearch(is)

	// Create a bunch of per-day time series
	const days = 5
	const metricsPerDay = 1000
	theDay := time.Date(2019, time.October, 15, 5, 1, 0, 0, time.UTC)
	now := uint64(timestampFromTime(theDay))
	currMetricIDs.hour = now / msecPerHour
	prevMetricIDs.hour = (now - msecPerHour) / msecPerHour
	baseDate := now / msecPerDay
	var metricNameBuf []byte
	for day := 0; day < days; day++ {
		var tsids []TSID
		for metric := 0; metric < metricsPerDay; metric++ {
			var mn MetricName
			mn.MetricGroup = []byte("testMetric")
			mn.AddTag(
				"constant",
				"const",
			)
			mn.AddTag(
				"day",
				fmt.Sprintf("%v", day),
			)
			mn.AddTag(
				"uniqueid",
				fmt.Sprintf("%v", metric),
			)
			mn.sortTags()

			metricNameBuf = mn.Marshal(metricNameBuf[:0])
			var tsid TSID
			if err := is.GetOrCreateTSIDByName(&tsid, metricNameBuf); err != nil {
				t.Fatalf("unexpected error when creating tsid for mn:\n%s: %s", &mn, err)
			}
			tsids = append(tsids, tsid)
		}

		// Add the metrics to the per-day stores
		date := baseDate - uint64(day*msecPerDay)
		for i := range tsids {
			tsid := &tsids[i]
			if err := is.storeDateMetricID(date, tsid.MetricID); err != nil {
				t.Fatalf("error in storeDateMetricID(%d, %d): %s", date, tsid.MetricID, err)
			}
		}

		// Add the the hour metrics caches
		if day == 0 {
			for i := 0; i < 256; i++ {
				prevMetricIDs.m.Add(tsids[i].MetricID)
				currMetricIDs.m.Add(tsids[i].MetricID)
			}
		}
	}

	// Flush index to disk, so it becomes visible for search
	db.tb.DebugFlush()

	// Create a filter that will match series that occur across multiple days
	tfs := NewTagFilters()
	if err := tfs.Add([]byte("constant"), []byte("const"), false, false); err != nil {
		t.Fatalf("cannot add filter: %s", err)
	}

	// Perform a search that can be fulfilled out of the hour metrics cache.
	// This should return the metrics in the hourly cache
	tr := TimeRange{
		MinTimestamp: int64(now - msecPerHour + 1),
		MaxTimestamp: int64(now),
	}
	matchedTSIDs, err := db.searchTSIDs([]*TagFilters{tfs}, tr, 10000)
	if err != nil {
		t.Fatalf("error searching tsids: %v", err)
	}
	if len(matchedTSIDs) != 256 {
		t.Fatal("Expected time series for current hour, got", len(matchedTSIDs))
	}

	// Perform a search within a day that falls out out of the hour metrics cache.
	// This should return the metrics for the day
	tr = TimeRange{
		MinTimestamp: int64(now - 2*msecPerHour - 1),
		MaxTimestamp: int64(now),
	}
	matchedTSIDs, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 10000)
	if err != nil {
		t.Fatalf("error searching tsids: %v", err)
	}
	if len(matchedTSIDs) != metricsPerDay {
		t.Fatal("Expected time series for current day, got", len(matchedTSIDs))
	}

	// Perform a search across all the days, should match all metrics
	tr = TimeRange{
		MinTimestamp: int64(now - msecPerDay*days),
		MaxTimestamp: int64(now),
	}

	matchedTSIDs, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 10000)
	if err != nil {
		t.Fatalf("error searching tsids: %v", err)
	}
	if len(matchedTSIDs) != metricsPerDay*days {
		t.Fatal("Expected time series for all days, got", len(matchedTSIDs))
	}

	// Check GetTSDBStatusForDate
	status, err := db.GetTSDBStatusForDate(baseDate, 5)
	if err != nil {
		t.Fatalf("error in GetTSDBStatusForDate: %s", err)
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
	expectedLabelValueCountByLabelName := []TopHeapEntry{
		{
			Name:  "uniqueid",
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
			Name:  "uniqueid=0",
			Count: 1,
		},
		{
			Name:  "uniqueid=1",
			Count: 1,
		},
	}
	if !reflect.DeepEqual(status.SeriesCountByLabelValuePair, expectedSeriesCountByLabelValuePair) {
		t.Fatalf("unexpected SeriesCountByLabelValuePair;\ngot\n%v\nwant\n%v", status.SeriesCountByLabelValuePair, expectedSeriesCountByLabelValuePair)
	}
}

func toTFPointers(tfs []tagFilter) []*tagFilter {
	tfps := make([]*tagFilter, len(tfs))
	for i := range tfs {
		tfps[i] = &tfs[i]
	}
	return tfps
}
