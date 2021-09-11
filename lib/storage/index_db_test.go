package storage

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
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
	xy := func(nsPrefix byte, accountID, projectID uint32, key, value string, metricIDs []uint64) string {
		dst := marshalCommonPrefix(nil, nsPrefix, accountID, projectID)
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
	x := func(accountID, projectID uint32, key, value string, metricIDs []uint64) string {
		return xy(nsPrefixTagToMetricIDs, accountID, projectID, key, value, metricIDs)
	}
	y := func(accountID, projectID uint32, key, value string, metricIDs []uint64) string {
		return xy(nsPrefixDateTagToMetricIDs, accountID, projectID, key, value, metricIDs)
	}

	f(nil, nil)
	f([]string{}, nil)
	f([]string{"foo"}, []string{"foo"})
	f([]string{"a", "b", "c", "def"}, []string{"a", "b", "c", "def"})
	f([]string{"\x00", "\x00b", "\x00c", "\x00def"}, []string{"\x00", "\x00b", "\x00c", "\x00def"})
	f([]string{
		x(0, 0, "", "", []uint64{0}),
		x(0, 0, "", "", []uint64{0}),
		x(0, 0, "", "", []uint64{0}),
		x(0, 0, "", "", []uint64{0}),
	}, []string{
		x(0, 0, "", "", []uint64{0}),
		x(0, 0, "", "", []uint64{0}),
		x(0, 0, "", "", []uint64{0}),
	})
	f([]string{
		x(0, 0, "", "", []uint64{0}),
		x(0, 0, "", "", []uint64{0}),
		x(0, 0, "", "", []uint64{0}),
		y(0, 0, "", "", []uint64{0}),
		y(0, 0, "", "", []uint64{0}),
		y(0, 0, "", "", []uint64{0}),
	}, []string{
		x(0, 0, "", "", []uint64{0}),
		x(0, 0, "", "", []uint64{0}),
		y(0, 0, "", "", []uint64{0}),
		y(0, 0, "", "", []uint64{0}),
	})
	f([]string{
		x(1, 2, "", "", []uint64{0}),
		x(1, 2, "", "", []uint64{0}),
		x(1, 2, "", "", []uint64{0}),
		x(1, 2, "", "", []uint64{0}),
		"xyz",
	}, []string{
		x(1, 2, "", "", []uint64{0}),
		x(1, 2, "", "", []uint64{0}),
		"xyz",
	})
	f([]string{
		"\x00asdf",
		x(1, 2, "", "", []uint64{0}),
		x(1, 2, "", "", []uint64{0}),
		x(1, 2, "", "", []uint64{0}),
		x(1, 2, "", "", []uint64{0}),
	}, []string{
		"\x00asdf",
		x(1, 2, "", "", []uint64{0}),
		x(1, 2, "", "", []uint64{0}),
	})
	f([]string{
		"\x00asdf",
		y(1, 2, "", "", []uint64{0}),
		y(1, 2, "", "", []uint64{0}),
		y(1, 2, "", "", []uint64{0}),
		y(1, 2, "", "", []uint64{0}),
	}, []string{
		"\x00asdf",
		y(1, 2, "", "", []uint64{0}),
		y(1, 2, "", "", []uint64{0}),
	})
	f([]string{
		"\x00asdf",
		x(3, 1, "", "", []uint64{0}),
		x(3, 1, "", "", []uint64{0}),
		x(3, 1, "", "", []uint64{0}),
		x(3, 1, "", "", []uint64{0}),
		"xyz",
	}, []string{
		"\x00asdf",
		x(3, 1, "", "", []uint64{0}),
		"xyz",
	})
	f([]string{
		"\x00asdf",
		x(3, 1, "", "", []uint64{0}),
		x(3, 1, "", "", []uint64{0}),
		y(3, 1, "", "", []uint64{0}),
		y(3, 1, "", "", []uint64{0}),
		"xyz",
	}, []string{
		"\x00asdf",
		x(3, 1, "", "", []uint64{0}),
		y(3, 1, "", "", []uint64{0}),
		"xyz",
	})
	f([]string{
		"\x00asdf",
		x(4, 2, "", "", []uint64{1}),
		x(4, 2, "", "", []uint64{2}),
		x(4, 2, "", "", []uint64{3}),
		x(4, 2, "", "", []uint64{4}),
		"xyz",
	}, []string{
		"\x00asdf",
		x(4, 2, "", "", []uint64{1, 2, 3, 4}),
		"xyz",
	})
	f([]string{
		"\x00asdf",
		x(1, 1, "", "", []uint64{1}),
		x(1, 1, "", "", []uint64{2}),
		x(1, 1, "", "", []uint64{3}),
		x(1, 1, "", "", []uint64{4}),
	}, []string{
		"\x00asdf",
		x(1, 1, "", "", []uint64{1, 2, 3}),
		x(1, 1, "", "", []uint64{4}),
	})
	f([]string{
		"\x00asdf",
		x(2, 2, "", "", []uint64{1}),
		x(2, 2, "", "", []uint64{2, 3, 4}),
		x(2, 2, "", "", []uint64{2, 3, 4, 5}),
		x(2, 2, "", "", []uint64{3, 5}),
		"foo",
	}, []string{
		"\x00asdf",
		x(2, 2, "", "", []uint64{1, 2, 3, 4, 5}),
		"foo",
	})
	f([]string{
		"\x00asdf",
		x(3, 3, "", "", []uint64{1}),
		x(3, 3, "", "a", []uint64{2, 3, 4}),
		x(3, 3, "", "a", []uint64{2, 3, 4, 5}),
		x(3, 3, "", "b", []uint64{3, 5}),
		"foo",
	}, []string{
		"\x00asdf",
		x(3, 3, "", "", []uint64{1}),
		x(3, 3, "", "a", []uint64{2, 3, 4, 5}),
		x(3, 3, "", "b", []uint64{3, 5}),
		"foo",
	})
	f([]string{
		"\x00asdf",
		x(2, 4, "", "", []uint64{1}),
		x(2, 4, "x", "a", []uint64{2, 3, 4}),
		x(2, 4, "y", "", []uint64{2, 3, 4, 5}),
		x(2, 4, "y", "x", []uint64{3, 5}),
		"foo",
	}, []string{
		"\x00asdf",
		x(2, 4, "", "", []uint64{1}),
		x(2, 4, "x", "a", []uint64{2, 3, 4}),
		x(2, 4, "y", "", []uint64{2, 3, 4, 5}),
		x(2, 4, "y", "x", []uint64{3, 5}),
		"foo",
	})
	f([]string{
		"\x00asdf",
		x(2, 4, "x", "a", []uint64{1}),
		x(2, 5, "x", "a", []uint64{2, 3, 4}),
		x(3, 4, "x", "a", []uint64{2, 3, 4, 5}),
		x(3, 4, "x", "b", []uint64{3, 5}),
		x(3, 4, "x", "b", []uint64{5, 6}),
		"foo",
	}, []string{
		"\x00asdf",
		x(2, 4, "x", "a", []uint64{1}),
		x(2, 5, "x", "a", []uint64{2, 3, 4}),
		x(3, 4, "x", "a", []uint64{2, 3, 4, 5}),
		x(3, 4, "x", "b", []uint64{3, 5, 6}),
		"foo",
	})
	f([]string{
		"\x00asdf",
		x(2, 2, "sdf", "aa", []uint64{1, 1, 3}),
		x(2, 2, "sdf", "aa", []uint64{1, 2}),
		"foo",
	}, []string{
		"\x00asdf",
		x(2, 2, "sdf", "aa", []uint64{1, 2, 3}),
		"foo",
	})
	f([]string{
		"\x00asdf",
		x(3, 2, "sdf", "aa", []uint64{1, 2, 2, 4}),
		x(3, 2, "sdf", "aa", []uint64{1, 2, 3}),
		"foo",
	}, []string{
		"\x00asdf",
		x(3, 2, "sdf", "aa", []uint64{1, 2, 3, 4}),
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
		x(3, 2, "foo", "bar", metricIDs),
		x(3, 2, "foo", "bar", metricIDs),
		y(2, 3, "foo", "bar", metricIDs),
		y(2, 3, "foo", "bar", metricIDs),
		"x",
	}, []string{
		"\x00aa",
		x(3, 2, "foo", "bar", metricIDs),
		y(2, 3, "foo", "bar", metricIDs),
		"x",
	})

	metricIDs = metricIDs[:0]
	for i := 0; i < maxMetricIDsPerRow; i++ {
		metricIDs = append(metricIDs, uint64(i))
	}
	f([]string{
		"\x00aa",
		x(3, 2, "foo", "bar", metricIDs),
		x(3, 2, "foo", "bar", metricIDs),
		"x",
	}, []string{
		"\x00aa",
		x(3, 2, "foo", "bar", metricIDs),
		x(3, 2, "foo", "bar", metricIDs),
		"x",
	})

	metricIDs = metricIDs[:0]
	for i := 0; i < 3*maxMetricIDsPerRow; i++ {
		metricIDs = append(metricIDs, uint64(i))
	}
	f([]string{
		"\x00aa",
		x(3, 2, "foo", "bar", metricIDs),
		x(3, 2, "foo", "bar", metricIDs),
		"x",
	}, []string{
		"\x00aa",
		x(3, 2, "foo", "bar", metricIDs),
		x(3, 2, "foo", "bar", metricIDs),
		"x",
	})
	f([]string{
		"\x00aa",
		x(3, 2, "foo", "bar", []uint64{0, 0, 1, 2, 3}),
		x(3, 2, "foo", "bar", metricIDs),
		x(3, 2, "foo", "bar", metricIDs),
		"x",
	}, []string{
		"\x00aa",
		x(3, 2, "foo", "bar", []uint64{0, 1, 2, 3}),
		x(3, 2, "foo", "bar", metricIDs),
		x(3, 2, "foo", "bar", metricIDs),
		"x",
	})

	// Check for duplicate metricIDs removal
	metricIDs = metricIDs[:0]
	for i := 0; i < maxMetricIDsPerRow-1; i++ {
		metricIDs = append(metricIDs, 123)
	}
	f([]string{
		"\x00aa",
		x(1, 2, "foo", "bar", metricIDs),
		x(1, 2, "foo", "bar", metricIDs),
		y(1, 1, "foo", "bar", metricIDs),
		"x",
	}, []string{
		"\x00aa",
		x(1, 2, "foo", "bar", []uint64{123}),
		y(1, 1, "foo", "bar", []uint64{123}),
		"x",
	})

	// Check fallback to the original items after merging, which result in incorrect ordering.
	metricIDs = metricIDs[:0]
	for i := 0; i < maxMetricIDsPerRow-3; i++ {
		metricIDs = append(metricIDs, uint64(123))
	}
	f([]string{
		"\x00aa",
		x(1, 2, "foo", "bar", metricIDs),
		x(1, 2, "foo", "bar", []uint64{123, 123, 125}),
		x(1, 2, "foo", "bar", []uint64{123, 124}),
		"x",
	}, []string{
		"\x00aa",
		x(1, 2, "foo", "bar", metricIDs),
		x(1, 2, "foo", "bar", []uint64{123, 123, 125}),
		x(1, 2, "foo", "bar", []uint64{123, 124}),
		"x",
	})
	f([]string{
		"\x00aa",
		x(1, 2, "foo", "bar", metricIDs),
		x(1, 2, "foo", "bar", []uint64{123, 123, 125}),
		x(1, 2, "foo", "bar", []uint64{123, 124}),
		y(1, 2, "foo", "bar", []uint64{123, 124}),
	}, []string{
		"\x00aa",
		x(1, 2, "foo", "bar", metricIDs),
		x(1, 2, "foo", "bar", []uint64{123, 123, 125}),
		x(1, 2, "foo", "bar", []uint64{123, 124}),
		y(1, 2, "foo", "bar", []uint64{123, 124}),
	})
	f([]string{
		x(1, 2, "foo", "bar", metricIDs),
		x(1, 2, "foo", "bar", []uint64{123, 123, 125}),
		x(1, 2, "foo", "bar", []uint64{123, 124}),
	}, []string{
		x(1, 2, "foo", "bar", metricIDs),
		x(1, 2, "foo", "bar", []uint64{123, 123, 125}),
		x(1, 2, "foo", "bar", []uint64{123, 124}),
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
	s := newTestStorage()
	defer stopTestStorage(s)

	for i := 0; i < 5; i++ {
		db, err := openIndexDB("test-index-db", s)
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
		s := newTestStorage()
		defer stopTestStorage(s)

		dbName := "test-index-db-serial"
		db, err := openIndexDB(dbName, s)
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
		db, err = openIndexDB(dbName, s)
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
		s := newTestStorage()
		defer stopTestStorage(s)

		dbName := "test-index-db-concurrent"
		db, err := openIndexDB(dbName, s)
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

	is := db.getIndexSearch(0, 0, noDeadline)
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

	is := db.getIndexSearch(0, 0, noDeadline)
	defer db.putIndexSearch(is)

	var metricNameBuf []byte
	for i := 0; i < 4e2+1; i++ {
		var mn MetricName
		mn.AccountID = uint32((i + 2) % accountsCount)
		mn.ProjectID = uint32((i + 1) % projectsCount)

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
		is.accountID = tsid.AccountID
		is.projectID = tsid.ProjectID
		if err := is.storeDateMetricID(date, tsid.MetricID, &mns[i]); err != nil {
			return nil, nil, fmt.Errorf("error in storeDateMetricID(%d, %d, %d, %d): %w", date, tsid.MetricID, tsid.AccountID, tsid.ProjectID, err)
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
		metricNameCopy, err = db.searchMetricNameWithCache(metricNameCopy[:0], tsidCopy.MetricID, tsidCopy.AccountID, tsidCopy.ProjectID)
		if err != nil {
			return fmt.Errorf("error in searchMetricNameWithCache for metricID=%d; i=%d: %w", tsidCopy.MetricID, i, err)
		}
		if !bytes.Equal(metricName, metricNameCopy) {
			return fmt.Errorf("unexpected mn for metricID=%d;\ngot\n%q\nwant\n%q", tsidCopy.MetricID, metricNameCopy, metricName)
		}

		// Try searching metric name for non-existent MetricID.
		buf, err := db.searchMetricNameWithCache(nil, 1, mn.AccountID, mn.ProjectID)
		if err != io.EOF {
			return fmt.Errorf("expecting io.EOF error when searching for non-existing metricID; got %v", err)
		}
		if len(buf) > 0 {
			return fmt.Errorf("expecting empty buf when searching for non-existent metricID; got %X", buf)
		}

		// Test SearchTagValues
		tvs, err := db.SearchTagValues(mn.AccountID, mn.ProjectID, nil, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("error in SearchTagValues for __name__: %w", err)
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
			tvs, err := db.SearchTagValues(mn.AccountID, mn.ProjectID, tag.Key, 1e5, noDeadline)
			if err != nil {
				return fmt.Errorf("error in SearchTagValues for __name__: %w", err)
			}
			if !hasValue(tvs, tag.Value) {
				return fmt.Errorf("SearchTagValues couldn't find %q=%q; found %q", tag.Key, tag.Value, tvs)
			}
			apKeys[string(tag.Key)] = true
		}
	}

	// Test SearchTagKeys
	for k, apKeys := range allKeys {
		tks, err := db.SearchTagKeys(k.AccountID, k.ProjectID, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("error in SearchTagKeys: %w", err)
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
			n, err := db.GetSeriesCount(k.AccountID, k.ProjectID, noDeadline)
			if err != nil {
				return fmt.Errorf("unexpected error in GetSeriesCount(%v): %w", k, err)
			}
			if n != uint64(len(tc)) {
				return fmt.Errorf("unexpected GetSeriesCount(%v); got %d; want %d", k, n, uint64(len(tc)))
			}
		}
	}

	// Try tag filters.
	currentTime := timestampFromTime(time.Now())
	tr := TimeRange{
		MinTimestamp: currentTime - msecPerDay,
		MaxTimestamp: currentTime + msecPerDay,
	}
	for i := range mns {
		mn := &mns[i]
		tsid := &tsids[i]

		// Search without regexps.
		tfs := NewTagFilters(mn.AccountID, mn.ProjectID)
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
		tsidsFound, err := db.searchTSIDs([]*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by exact tag filter: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in exact tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s\ni=%d", tsid, tsidsFound, tfs, mn, i)
		}

		// Verify tag cache.
		tsidsCached, err := db.searchTSIDs([]*TagFilters{tfs}, tr, 1e5, noDeadline)
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
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by exact tag filter with full negative: %w", err)
		}
		if testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("unexpected tsid found for exact negative filter\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search for Graphite wildcard
		tfs.Reset(mn.AccountID, mn.ProjectID)
		n := bytes.IndexByte(mn.MetricGroup, '.')
		if n < 0 {
			return fmt.Errorf("cannot find dot in MetricGroup %q", mn.MetricGroup)
		}
		re := "[^.]*" + regexp.QuoteMeta(string(mn.MetricGroup[n:]))
		if err := tfs.Add(nil, []byte(re), false, true); err != nil {
			return fmt.Errorf("cannot create regexp tag filter for Graphite wildcard")
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by regexp tag filter for Graphite wildcard: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in regexp for Graphite wildcard tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with a filter matching empty tag (a single filter)
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1601
		tfs.Reset(mn.AccountID, mn.ProjectID)
		if err := tfs.Add(nil, mn.MetricGroup, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup: %w", err)
		}
		if err := tfs.Add([]byte("non-existent-tag"), []byte("foo|"), false, true); err != nil {
			return fmt.Errorf("cannot create regexp tag filter for non-existing tag: %w", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search with a filter matching empty tag: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing when matching a filter with empty tag tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with filters matching empty tags (multiple filters)
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1601
		tfs.Reset(mn.AccountID, mn.ProjectID)
		if err := tfs.Add(nil, mn.MetricGroup, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup: %w", err)
		}
		if err := tfs.Add([]byte("non-existent-tag1"), []byte("foo|"), false, true); err != nil {
			return fmt.Errorf("cannot create regexp tag filter for non-existing tag1: %w", err)
		}
		if err := tfs.Add([]byte("non-existent-tag2"), []byte("bar|"), false, true); err != nil {
			return fmt.Errorf("cannot create regexp tag filter for non-existing tag2: %w", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search with multipel filters matching empty tags: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing when matching multiple filters with empty tags tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with regexps.
		tfs.Reset(mn.AccountID, mn.ProjectID)
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
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by regexp tag filter: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in regexp tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}
		if err := tfs.Add(nil, mn.MetricGroup, true, true); err != nil {
			return fmt.Errorf("cannot add negative filter for zeroing search results: %w", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search by regexp tag filter with full negative: %w", err)
		}
		if testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("unexpected tsid found for regexp negative filter\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with filter matching zero results.
		tfs.Reset(mn.AccountID, mn.ProjectID)
		if err := tfs.Add([]byte("non-existing-key"), []byte("foobar"), false, false); err != nil {
			return fmt.Errorf("cannot add non-existing key: %w", err)
		}
		if err := tfs.Add(nil, mn.MetricGroup, false, true); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup matching zero results: %w", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 1e5, noDeadline)
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

		// Search with empty filter. It should match all the results for (accountID, projectID).
		tfs.Reset(mn.AccountID, mn.ProjectID)
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search for common prefix: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in common prefix\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Search with empty metricGroup. It should match zero results.
		tfs.Reset(mn.AccountID, mn.ProjectID)
		if err := tfs.Add(nil, nil, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for empty metricGroup: %w", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search for empty metricGroup: %w", err)
		}
		if len(tsidsFound) != 0 {
			return fmt.Errorf("unexpected non-empty tsids found for empty metricGroup: %v", tsidsFound)
		}

		// Search with multiple tfss
		tfs1 := NewTagFilters(mn.AccountID, mn.ProjectID)
		if err := tfs1.Add(nil, nil, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for empty metricGroup: %w", err)
		}
		tfs2 := NewTagFilters(mn.AccountID, mn.ProjectID)
		if err := tfs2.Add(nil, mn.MetricGroup, false, false); err != nil {
			return fmt.Errorf("cannot create tag filter for MetricGroup: %w", err)
		}
		tsidsFound, err = db.searchTSIDs([]*TagFilters{tfs1, tfs2}, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search for empty metricGroup: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing when searching for multiple tfss \ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Verify empty tfss
		tsidsFound, err = db.searchTSIDs(nil, tr, 1e5, noDeadline)
		if err != nil {
			return fmt.Errorf("cannot search for nil tfss: %w", err)
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
	tfs.Reset(mn.AccountID, mn.ProjectID)
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
	tfs.Reset(mn.AccountID, mn.ProjectID)
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
	tfs.Reset(mn.AccountID, mn.ProjectID)
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
	tfs.Reset(mn.AccountID, mn.ProjectID)
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
	tfs.Reset(mn.AccountID, mn.ProjectID)
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
	tfs.Reset(mn.AccountID, mn.ProjectID)
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
	tfs.Reset(mn.AccountID, mn.ProjectID)
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

	// Negative match for multiple non-regexp positive filters
	tfs.Reset(mn.AccountID, mn.ProjectID)
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
	s := newTestStorage()
	defer stopTestStorage(s)

	dbName := "test-index-db-ts-range"
	db, err := openIndexDB(dbName, s)
	if err != nil {
		t.Fatalf("cannot open indexDB: %s", err)
	}
	defer func() {
		db.MustClose()
		if err := os.RemoveAll(dbName); err != nil {
			t.Fatalf("cannot remove indexDB: %s", err)
		}
	}()

	// Create a bunch of per-day time series
	const accountID = 12345
	const projectID = 85453
	is := db.getIndexSearch(accountID, projectID, noDeadline)
	defer db.putIndexSearch(is)
	const days = 5
	const metricsPerDay = 1000
	theDay := time.Date(2019, time.October, 15, 5, 1, 0, 0, time.UTC)
	now := uint64(timestampFromTime(theDay))
	baseDate := now / msecPerDay
	var metricNameBuf []byte
	perDayMetricIDs := make(map[uint64]*uint64set.Set)
	var allMetricIDs uint64set.Set
	tagKeys := []string{
		"", "constant", "day", "uniqueid",
	}
	tagValues := []string{
		"testMetric",
	}
	sort.Strings(tagKeys)
	for day := 0; day < days; day++ {
		var tsids []TSID
		var mns []MetricName
		for metric := 0; metric < metricsPerDay; metric++ {
			var mn MetricName
			mn.AccountID = accountID
			mn.ProjectID = projectID
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
			if tsid.AccountID != accountID {
				t.Fatalf("unexpected accountID; got %d; want %d", tsid.AccountID, accountID)
			}
			if tsid.ProjectID != projectID {
				t.Fatalf("unexpected accountID; got %d; want %d", tsid.ProjectID, projectID)
			}
			mns = append(mns, mn)
			tsids = append(tsids, tsid)
		}

		// Add the metrics to the per-day stores
		date := baseDate - uint64(day)
		var metricIDs uint64set.Set
		for i := range tsids {
			tsid := &tsids[i]
			metricIDs.Add(tsid.MetricID)
			if err := is.storeDateMetricID(date, tsid.MetricID, &mns[i]); err != nil {
				t.Fatalf("error in storeDateMetricID(%d, %d): %s", date, tsid.MetricID, err)
			}
		}
		allMetricIDs.Union(&metricIDs)
		perDayMetricIDs[date] = &metricIDs
	}

	// Flush index to disk, so it becomes visible for search
	db.tb.DebugFlush()

	is2 := db.getIndexSearch(accountID, projectID, noDeadline)
	defer db.putIndexSearch(is2)

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

	// Check SearchTagKeysOnTimeRange.
	tks, err := db.SearchTagKeysOnTimeRange(accountID, projectID, TimeRange{
		MinTimestamp: int64(now) - msecPerDay,
		MaxTimestamp: int64(now),
	}, 10000, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchTagKeysOnTimeRange: %s", err)
	}
	sort.Strings(tks)
	if !reflect.DeepEqual(tks, tagKeys) {
		t.Fatalf("unexpected tagKeys; got\n%s\nwant\n%s", tks, tagKeys)
	}

	// Check SearchTagValuesOnTimeRange.
	tvs, err := db.SearchTagValuesOnTimeRange(accountID, projectID, []byte(""), TimeRange{
		MinTimestamp: int64(now) - msecPerDay,
		MaxTimestamp: int64(now),
	}, 10000, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchTagValuesOnTimeRange: %s", err)
	}
	sort.Strings(tvs)
	if !reflect.DeepEqual(tvs, tagValues) {
		t.Fatalf("unexpected tagValues; got\n%s\nwant\n%s", tvs, tagValues)
	}

	// Create a filter that will match series that occur across multiple days
	tfs := NewTagFilters(accountID, projectID)
	if err := tfs.Add([]byte("constant"), []byte("const"), false, false); err != nil {
		t.Fatalf("cannot add filter: %s", err)
	}

	// Perform a search within a day.
	// This should return the metrics for the day
	tr := TimeRange{
		MinTimestamp: int64(now - 2*msecPerHour - 1),
		MaxTimestamp: int64(now),
	}
	matchedTSIDs, err := db.searchTSIDs([]*TagFilters{tfs}, tr, 10000, noDeadline)
	if err != nil {
		t.Fatalf("error searching tsids: %v", err)
	}
	if len(matchedTSIDs) != metricsPerDay {
		t.Fatalf("expected %d time series for current day, got %d time series", metricsPerDay, len(matchedTSIDs))
	}

	// Perform a search across all the days, should match all metrics
	tr = TimeRange{
		MinTimestamp: int64(now - msecPerDay*days),
		MaxTimestamp: int64(now),
	}

	matchedTSIDs, err = db.searchTSIDs([]*TagFilters{tfs}, tr, 10000, noDeadline)
	if err != nil {
		t.Fatalf("error searching tsids: %v", err)
	}
	if len(matchedTSIDs) != metricsPerDay*days {
		t.Fatalf("expected %d time series for all days, got %d time series", metricsPerDay*days, len(matchedTSIDs))
	}

	// Check GetTSDBStatusWithFiltersForDate with nil filters.
	status, err := db.GetTSDBStatusWithFiltersForDate(accountID, projectID, nil, baseDate, 5, noDeadline)
	if err != nil {
		t.Fatalf("error in GetTSDBStatusWithFiltersForDate with nil filters: %s", err)
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

	// Check GetTSDBStatusWithFiltersForDate
	tfs = NewTagFilters(accountID, projectID)
	if err := tfs.Add([]byte("day"), []byte("0"), false, false); err != nil {
		t.Fatalf("cannot add filter: %s", err)
	}
	status, err = db.GetTSDBStatusWithFiltersForDate(accountID, projectID, []*TagFilters{tfs}, baseDate, 5, noDeadline)
	if err != nil {
		t.Fatalf("error in GetTSDBStatusWithFiltersForDate: %s", err)
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

		metricIDCache:   workingsetcache.New(1234, time.Hour),
		metricNameCache: workingsetcache.New(1234, time.Hour),
		tsidCache:       workingsetcache.New(1234, time.Hour),
	}
	s.setDeletedMetricIDs(&uint64set.Set{})
	return s
}

func stopTestStorage(s *Storage) {
	s.metricIDCache.Stop()
	s.metricNameCache.Stop()
	s.tsidCache.Stop()
	fs.MustRemoveAll(s.cachePath)
}
