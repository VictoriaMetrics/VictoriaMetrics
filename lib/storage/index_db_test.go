package storage

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
	"github.com/VictoriaMetrics/fastcache"
)

func TestMarshalUnmarshalMetricIDs(t *testing.T) {
	f := func(metricIDs []uint64) {
		t.Helper()

		// Try marshaling and unmarshaling to an empty dst
		data := marshalMetricIDs(nil, metricIDs)
		result := mustUnmarshalMetricIDs(nil, data)
		if !reflect.DeepEqual(result, metricIDs) {
			t.Fatalf("unexpected metricIDs after unmarshaling;\ngot\n%d\nwant\n%d", result, metricIDs)
		}

		// Try marshaling and unmarshaling to non-empty dst
		dataPrefix := []byte("prefix")
		data = marshalMetricIDs(dataPrefix, metricIDs)
		if len(data) < len(dataPrefix) {
			t.Fatalf("too short len(data)=%d; must be at least len(dataPrefix)=%d", len(data), len(dataPrefix))
		}
		if string(data[:len(dataPrefix)]) != string(dataPrefix) {
			t.Fatalf("unexpected prefix; got %q; want %q", data[:len(dataPrefix)], dataPrefix)
		}
		data = data[len(dataPrefix):]

		resultPrefix := []uint64{889432422, 89243, 9823}
		result = mustUnmarshalMetricIDs(resultPrefix, data)
		if len(result) < len(resultPrefix) {
			t.Fatalf("too short result returned; len(result)=%d; must be at least len(resultPrefix)=%d", len(result), len(resultPrefix))
		}
		if !reflect.DeepEqual(result[:len(resultPrefix)], resultPrefix) {
			t.Fatalf("unexpected result prefix; got %d; want %d", result[:len(resultPrefix)], resultPrefix)
		}
		result = result[len(resultPrefix):]
		if (len(metricIDs) > 0 || len(result) > 0) && !reflect.DeepEqual(result, metricIDs) {
			t.Fatalf("unexpected metricIDs after unmarshaling from prefix;\ngot\n%d\nwant\n%d", result, metricIDs)
		}
	}

	f(nil)
	f([]uint64{0})
	f([]uint64{1})
	f([]uint64{1234, 678932943, 843289893843})
	f([]uint64{1, 2, 3, 4, 5, 6, 8989898, 823849234, 1<<64 - 1, 1<<32 - 1, 0})
}

func TestTagFiltersToMetricIDsCache(t *testing.T) {
	f := func(want []uint64) {
		t.Helper()

		path := t.Name()
		defer fs.MustRemoveAll(path)

		s := MustOpenStorage(path, 0, 0, 0)
		defer s.MustClose()

		idb := s.idb()
		key := []byte("key")
		idb.putMetricIDsToTagFiltersCache(nil, want, key)
		got, ok := idb.getMetricIDsFromTagFiltersCache(nil, key)
		if !ok {
			t.Fatalf("expected metricIDs to be found in cache but they weren't: %v", want)
		}
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
	defer fs.MustRemoveAll(path)
	s := MustOpenStorage(path, 0, 0, 0)
	defer s.MustClose()
	idb := s.idb()

	key := []byte("key")
	emptyMetricIDs := []uint64(nil)
	idb.putMetricIDsToTagFiltersCache(nil, emptyMetricIDs, key)
	got, ok := idb.getMetricIDsFromTagFiltersCache(nil, key)
	if !ok {
		t.Fatalf("expected empty metricID list to be found in cache but it wasn't")
	}
	if len(got) > 0 {
		t.Fatalf("unexpected found metricID list to be empty but got %v", got)
	}

}

func TestMergeSortedMetricIDs(t *testing.T) {
	f := func(a, b []uint64) {
		t.Helper()
		m := make(map[uint64]bool)
		var resultExpected []uint64
		for _, v := range a {
			if !m[v] {
				m[v] = true
				resultExpected = append(resultExpected, v)
			}
		}
		for _, v := range b {
			if !m[v] {
				m[v] = true
				resultExpected = append(resultExpected, v)
			}
		}
		sort.Slice(resultExpected, func(i, j int) bool {
			return resultExpected[i] < resultExpected[j]
		})

		result := mergeSortedMetricIDs(a, b)
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result for mergeSortedMetricIDs(%d, %d); got\n%d\nwant\n%d", a, b, result, resultExpected)
		}
		result = mergeSortedMetricIDs(b, a)
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result for mergeSortedMetricIDs(%d, %d); got\n%d\nwant\n%d", b, a, result, resultExpected)
		}
	}
	f(nil, nil)
	f([]uint64{1}, nil)
	f(nil, []uint64{23})
	f([]uint64{1234}, []uint64{0})
	f([]uint64{1}, []uint64{1})
	f([]uint64{1}, []uint64{1, 2, 3})
	f([]uint64{1, 2, 3}, []uint64{1, 2, 3})
	f([]uint64{1, 2, 3}, []uint64{2, 3})
	f([]uint64{0, 1, 7, 8, 9, 13, 20}, []uint64{1, 2, 7, 13, 15})
	f([]uint64{0, 1, 2, 3, 4}, []uint64{5, 6, 7, 8})
	f([]uint64{0, 1, 2, 3, 4}, []uint64{4, 5, 6, 7, 8})
	f([]uint64{0, 1, 2, 3, 4}, []uint64{3, 4, 5, 6, 7, 8})
	f([]uint64{2, 3, 4}, []uint64{1, 5, 6, 7})
	f([]uint64{2, 3, 4}, []uint64{1, 2, 5, 6, 7})
	f([]uint64{2, 3, 4}, []uint64{1, 2, 4, 5, 6, 7})
	f([]uint64{2, 3, 4}, []uint64{1, 2, 3, 4, 5, 6, 7})
	f([]uint64{2, 3, 4, 6}, []uint64{1, 2, 3, 4, 5, 6, 7})
	f([]uint64{2, 3, 4, 6, 7}, []uint64{1, 2, 3, 4, 5, 6, 7})
	f([]uint64{2, 3, 4, 6, 7, 8}, []uint64{1, 2, 3, 4, 5, 6, 7})
	f([]uint64{2, 3, 4, 6, 7, 8, 9}, []uint64{1, 2, 3, 4, 5, 6, 7})
	f([]uint64{1, 2, 3, 4, 6, 7, 8, 9}, []uint64{1, 2, 3, 4, 5, 6, 7})
	f([]uint64{1, 2, 3, 4, 6, 7, 8, 9}, []uint64{2, 3, 4, 5, 6, 7})
	f([]uint64{}, []uint64{1, 2, 3})
	f([]uint64{0}, []uint64{1, 2, 3})
	f([]uint64{1}, []uint64{1, 2, 3})
	f([]uint64{1, 2}, []uint64{3, 4})
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
	var s Storage
	tableName := nextIndexDBTableName()
	for i := 0; i < 5; i++ {
		var isReadOnly atomic.Bool
		db := mustOpenIndexDB(tableName, &s, &isReadOnly)
		db.MustClose()
	}
	if err := os.RemoveAll(tableName); err != nil {
		t.Fatalf("cannot remove indexDB: %s", err)
	}
}

func TestIndexDB(t *testing.T) {
	const metricGroups = 10

	t.Run("serial", func(t *testing.T) {
		const path = "TestIndexDB-serial"
		s := MustOpenStorage(path, retentionMax, 0, 0)

		db := s.idb()
		mns, tsids, err := testIndexDBGetOrCreateTSIDByName(db, metricGroups)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := testIndexDBCheckTSIDByName(db, mns, tsids, false); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Re-open the storage and verify it works as expected.
		s.MustClose()
		s = MustOpenStorage(path, retentionMax, 0, 0)

		db = s.idb()
		if err := testIndexDBCheckTSIDByName(db, mns, tsids, false); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		s.MustClose()
		fs.MustRemoveAll(path)
	})

	t.Run("concurrent", func(t *testing.T) {
		const path = "TestIndexDB-concurrent"
		s := MustOpenStorage(path, retentionMax, 0, 0)
		db := s.idb()

		ch := make(chan error, 3)
		for i := 0; i < cap(ch); i++ {
			go func() {
				mns, tsid, err := testIndexDBGetOrCreateTSIDByName(db, metricGroups)
				if err != nil {
					ch <- err
					return
				}
				if err := testIndexDBCheckTSIDByName(db, mns, tsid, true); err != nil {
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

		s.MustClose()
		fs.MustRemoveAll(path)
	})
}

func testIndexDBGetOrCreateTSIDByName(db *indexDB, metricGroups int) ([]MetricName, []TSID, error) {
	r := rand.New(rand.NewSource(1))
	// Create tsids.
	var mns []MetricName
	var tsids []TSID

	is := db.getIndexSearch(noDeadline)

	date := uint64(timestampFromTime(time.Now())) / msecPerDay

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
		var genTSID generationTSID
		if !is.getTSIDByMetricName(&genTSID, metricNameBuf, date) {
			generateTSID(&genTSID.TSID, &mn)
			createAllIndexesForMetricName(is, &mn, &genTSID.TSID, date)
		}

		mns = append(mns, mn)
		tsids = append(tsids, genTSID.TSID)
	}
	db.putIndexSearch(is)

	// Flush index to disk, so it becomes visible for search
	db.s.DebugFlush()

	return mns, tsids, nil
}

func testIndexDBCheckTSIDByName(db *indexDB, mns []MetricName, tsids []TSID, isConcurrent bool) error {
	hasValue := func(lvs []string, v []byte) bool {
		for _, lv := range lvs {
			if string(v) == lv {
				return true
			}
		}
		return false
	}

	currentTime := timestampFromTime(time.Now())
	timeseriesCounters := make(map[uint64]bool)
	var genTSID generationTSID
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
		if !is.getTSIDByMetricName(&genTSID, metricName, uint64(currentTime)/msecPerDay) {
			return fmt.Errorf("cannot obtain tsid #%d for mn %s", i, mn)
		}
		db.putIndexSearch(is)

		if isConcurrent {
			// Copy tsid.MetricID, since multiple TSIDs may match
			// the same mn in concurrent mode.
			genTSID.TSID.MetricID = tsid.MetricID
		}
		if !reflect.DeepEqual(tsid, &genTSID.TSID) {
			return fmt.Errorf("unexpected tsid for mn:\n%s\ngot\n%+v\nwant\n%+v", mn, &genTSID.TSID, tsid)
		}

		// Search for metric name for the given metricID.
		var ok bool
		metricNameCopy, ok = db.searchMetricNameWithCache(metricNameCopy[:0], genTSID.TSID.MetricID)
		if !ok {
			return fmt.Errorf("cannot find metricName for metricID=%d; i=%d", genTSID.TSID.MetricID, i)
		}
		if !bytes.Equal(metricName, metricNameCopy) {
			return fmt.Errorf("unexpected mn for metricID=%d;\ngot\n%q\nwant\n%q", genTSID.TSID.MetricID, metricNameCopy, metricName)
		}

		// Try searching metric name for non-existent MetricID.
		buf, found := db.searchMetricNameWithCache(nil, 1)
		if found {
			return fmt.Errorf("unexpected metricName found for non-existing metricID; got %X", buf)
		}
		if len(buf) > 0 {
			return fmt.Errorf("expecting empty buf when searching for non-existent metricID; got %X", buf)
		}

		// Test SearchLabelValuesWithFiltersOnTimeRange
		lvs, err := db.SearchLabelValuesWithFiltersOnTimeRange(nil, "__name__", nil, TimeRange{}, 1e5, 1e9, noDeadline)
		if err != nil {
			return fmt.Errorf("error in SearchLabelValuesWithFiltersOnTimeRange(labelName=%q): %w", "__name__", err)
		}
		if !hasValue(lvs, mn.MetricGroup) {
			return fmt.Errorf("SearchLabelValuesWithFiltersOnTimeRange(labelName=%q): couldn't find %q; found %q", "__name__", mn.MetricGroup, lvs)
		}
		for i := range mn.Tags {
			tag := &mn.Tags[i]
			lvs, err := db.SearchLabelValuesWithFiltersOnTimeRange(nil, string(tag.Key), nil, TimeRange{}, 1e5, 1e9, noDeadline)
			if err != nil {
				return fmt.Errorf("error in SearchLabelValuesWithFiltersOnTimeRange(labelName=%q): %w", tag.Key, err)
			}
			if !hasValue(lvs, tag.Value) {
				return fmt.Errorf("SearchLabelValuesWithFiltersOnTimeRange(labelName=%q): couldn't find %q; found %q", tag.Key, tag.Value, lvs)
			}
			allLabelNames[string(tag.Key)] = true
		}
	}

	// Test SearchLabelNamesWithFiltersOnTimeRange (empty filters, global time range)
	lns, err := db.SearchLabelNamesWithFiltersOnTimeRange(nil, nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelNamesWithFiltersOnTimeRange(empty filter, global time range): %w", err)
	}
	if !hasValue(lns, []byte("__name__")) {
		return fmt.Errorf("cannot find __name__ in %q", lns)
	}
	for labelName := range allLabelNames {
		if !hasValue(lns, []byte(labelName)) {
			return fmt.Errorf("cannot find %q in %q", labelName, lns)
		}
	}

	// Check timerseriesCounters only for serial test.
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
		MinTimestamp: currentTime - msecPerDay,
		MaxTimestamp: currentTime + msecPerDay,
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
		tsidsFound, err := searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
		if err != nil {
			return fmt.Errorf("cannot search by exact tag filter: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in exact tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s\ni=%d", tsid, tsidsFound, tfs, mn, i)
		}

		// Verify tag cache.
		tsidsCached, err := searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
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
		tsidsFound, err = searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
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
		tsidsFound, err = searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
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
		tsidsFound, err = searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
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
		tsidsFound, err = searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
		if err != nil {
			return fmt.Errorf("cannot search with multipel filters matching empty tags: %w", err)
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
		tsidsFound, err = searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
		if err != nil {
			return fmt.Errorf("cannot search by regexp tag filter: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing in regexp tsidsFound\ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}
		if err := tfs.Add(nil, mn.MetricGroup, true, true); err != nil {
			return fmt.Errorf("cannot add negative filter for zeroing search results: %w", err)
		}
		tsidsFound, err = searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
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
		tsidsFound, err = searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
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
		tsidsFound, err = searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
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
		tsidsFound, err = searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
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
		tsidsFound, err = searchTSIDsInTest(db, []*TagFilters{tfs1, tfs2}, tr)
		if err != nil {
			return fmt.Errorf("cannot search for empty metricGroup: %w", err)
		}
		if !testHasTSID(tsidsFound, tsid) {
			return fmt.Errorf("tsids is missing when searching for multiple tfss \ntsid=%+v\ntsidsFound=%+v\ntfs=%s\nmn=%s", tsid, tsidsFound, tfs, mn)
		}

		// Verify empty tfss
		tsidsFound, err = searchTSIDsInTest(db, nil, tr)
		if err != nil {
			return fmt.Errorf("cannot search for nil tfss: %w", err)
		}
		if len(tsidsFound) != 0 {
			return fmt.Errorf("unexpected non-empty tsids fround for nil tfss; found %d tsids", len(tsidsFound))
		}
	}

	return nil
}

func searchTSIDsInTest(db *indexDB, tfs []*TagFilters, tr TimeRange) ([]TSID, error) {
	metricIDs, err := db.searchMetricIDs(nil, tfs, tr, 1e5, noDeadline)
	if err != nil {
		return nil, err
	}
	return db.getTSIDsFromMetricIDs(nil, metricIDs, noDeadline)
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

func TestIndexDBRepopulateAfterRotation(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	path := "TestIndexRepopulateAfterRotation"
	s := MustOpenStorage(path, retention31Days, 1e5, 1e5)

	db := s.idb()
	if db.generation == 0 {
		t.Fatalf("expected indexDB generation to be not 0")
	}

	const metricRowsN = 1000

	currentDayTimestamp := (time.Now().UnixMilli() / msecPerDay) * msecPerDay
	timeMin := currentDayTimestamp - 24*3600*1000
	timeMax := currentDayTimestamp + 24*3600*1000
	mrs := testGenerateMetricRows(r, metricRowsN, timeMin, timeMax)
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()

	// verify the storage contains rows.
	var m Metrics
	s.UpdateMetrics(&m)
	if rowsCount := m.TableMetrics.TotalRowsCount(); rowsCount < uint64(metricRowsN) {
		t.Fatalf("expecting at least %d rows in the table; got %d", metricRowsN, rowsCount)
	}

	// check new series were registered in indexDB
	added := db.s.newTimeseriesCreated.Load()
	if added != metricRowsN {
		t.Fatalf("expected indexDB to contain %d rows; got %d", metricRowsN, added)
	}

	// check new series were added to cache
	var cs fastcache.Stats
	s.tsidCache.UpdateStats(&cs)
	if cs.EntriesCount != metricRowsN {
		t.Fatalf("expected tsidCache to contain %d rows; got %d", metricRowsN, cs.EntriesCount)
	}

	// check if cache entries do belong to current indexDB generation
	var genTSID generationTSID
	for _, mr := range mrs {
		s.getTSIDFromCache(&genTSID, mr.MetricNameRaw)
		if genTSID.generation != db.generation {
			t.Fatalf("expected all entries in tsidCache to have the same indexDB generation: %d;"+
				"got %d", db.generation, genTSID.generation)
		}
	}
	prevGeneration := db.generation

	// force index rotation
	s.mustRotateIndexDB(time.Now())

	// check tsidCache wasn't reset after the rotation
	var cs2 fastcache.Stats
	s.tsidCache.UpdateStats(&cs2)
	if cs.EntriesCount != metricRowsN {
		t.Fatalf("expected tsidCache after rotation to contain %d rows; got %d", metricRowsN, cs2.EntriesCount)
	}
	dbNew := s.idb()
	if dbNew.generation == 0 {
		t.Fatalf("expected new indexDB generation to be not 0")
	}
	if dbNew.generation == prevGeneration {
		t.Fatalf("expected new indexDB generation %d to be different from prev indexDB", dbNew.generation)
	}

	// Re-insert rows again and verify that all the entries belong to new generation
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()

	for _, mr := range mrs {
		s.getTSIDFromCache(&genTSID, mr.MetricNameRaw)
		if genTSID.generation != dbNew.generation {
			t.Fatalf("unexpected generation for data after rotation; got %d; want %d", genTSID.generation, dbNew.generation)
		}
	}

	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func TestSearchTSIDWithTimeRange(t *testing.T) {
	const path = "TestSearchTSIDWithTimeRange"
	s := MustOpenStorage(path, retentionMax, 0, 0)
	db := s.idb()

	is := db.getIndexSearch(noDeadline)

	// Create a bunch of per-day time series
	const days = 5
	const metricsPerDay = 1000
	theDay := time.Date(2019, time.October, 15, 5, 1, 0, 0, time.UTC)
	now := uint64(timestampFromTime(theDay))
	baseDate := now / msecPerDay
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
	for day := 0; day < days; day++ {
		date := baseDate - uint64(day)
		var metricIDs uint64set.Set
		for metric := 0; metric < metricsPerDay; metric++ {
			mn := newMN("testMetric", day, metric)
			metricNameBuf = mn.Marshal(metricNameBuf[:0])
			var genTSID generationTSID
			if !is.getTSIDByMetricName(&genTSID, metricNameBuf, date) {
				generateTSID(&genTSID.TSID, &mn)
				createAllIndexesForMetricName(is, &mn, &genTSID.TSID, date)
			}
			metricIDs.Add(genTSID.TSID.MetricID)
		}

		allMetricIDs.Union(&metricIDs)
		perDayMetricIDs[date] = &metricIDs
	}
	db.putIndexSearch(is)

	// Flush index to disk, so it becomes visible for search
	s.DebugFlush()

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
	var genTSID generationTSID
	if !is3.getTSIDByMetricName(&genTSID, metricNameBuf, date) {
		generateTSID(&genTSID.TSID, &mn)
		createAllIndexesForMetricName(is3, &mn, &genTSID.TSID, date)
	}
	// delete the added metric. It is expected it won't be returned during searches
	deletedSet := &uint64set.Set{}
	deletedSet.Add(genTSID.TSID.MetricID)
	s.setDeletedMetricIDs(deletedSet)
	db.putIndexSearch(is3)
	s.DebugFlush()

	// Check SearchLabelNamesWithFiltersOnTimeRange with the specified time range.
	tr := TimeRange{
		MinTimestamp: int64(now) - msecPerDay,
		MaxTimestamp: int64(now),
	}
	lns, err := db.SearchLabelNamesWithFiltersOnTimeRange(nil, nil, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelNamesWithFiltersOnTimeRange(timeRange=%s): %s", &tr, err)
	}
	sort.Strings(lns)
	if !reflect.DeepEqual(lns, labelNames) {
		t.Fatalf("unexpected labelNames; got\n%s\nwant\n%s", lns, labelNames)
	}

	// Check SearchLabelValuesWithFiltersOnTimeRange with the specified time range.
	lvs, err := db.SearchLabelValuesWithFiltersOnTimeRange(nil, "", nil, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelValuesWithFiltersOnTimeRange(timeRange=%s): %s", &tr, err)
	}
	sort.Strings(lvs)
	if !reflect.DeepEqual(lvs, labelValues) {
		t.Fatalf("unexpected labelValues; got\n%s\nwant\n%s", lvs, labelValues)
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

	// Perform a search within a day.
	// This should return the metrics for the day
	tr = TimeRange{
		MinTimestamp: int64(now - 2*msecPerHour - 1),
		MaxTimestamp: int64(now),
	}
	matchedTSIDs, err := searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
	if err != nil {
		t.Fatalf("error searching tsids: %v", err)
	}
	if len(matchedTSIDs) != metricsPerDay {
		t.Fatalf("expected %d time series for current day, got %d time series", metricsPerDay, len(matchedTSIDs))
	}

	// Check SearchLabelNamesWithFiltersOnTimeRange with the specified filter.
	lns, err = db.SearchLabelNamesWithFiltersOnTimeRange(nil, []*TagFilters{tfs}, TimeRange{}, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelNamesWithFiltersOnTimeRange(filters=%s): %s", tfs, err)
	}
	sort.Strings(lns)
	if !reflect.DeepEqual(lns, labelNames) {
		t.Fatalf("unexpected labelNames; got\n%s\nwant\n%s", lns, labelNames)
	}

	// Check SearchLabelNamesWithFiltersOnTimeRange with the specified filter and time range.
	lns, err = db.SearchLabelNamesWithFiltersOnTimeRange(nil, []*TagFilters{tfs}, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelNamesWithFiltersOnTimeRange(filters=%s, timeRange=%s): %s", tfs, &tr, err)
	}
	sort.Strings(lns)
	if !reflect.DeepEqual(lns, labelNames) {
		t.Fatalf("unexpected labelNames; got\n%s\nwant\n%s", lns, labelNames)
	}

	// Check SearchLabelNamesWithFiltersOnTimeRange with filters on metric name and time range.
	lns, err = db.SearchLabelNamesWithFiltersOnTimeRange(nil, []*TagFilters{tfsMetricName}, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelNamesWithFiltersOnTimeRange(filters=%s, timeRange=%s): %s", tfs, &tr, err)
	}
	sort.Strings(lns)
	if !reflect.DeepEqual(lns, labelNames) {
		t.Fatalf("unexpected labelNames; got\n%s\nwant\n%s", lns, labelNames)
	}

	// Check SearchLabelValuesWithFiltersOnTimeRange with the specified filter.
	lvs, err = db.SearchLabelValuesWithFiltersOnTimeRange(nil, "", []*TagFilters{tfs}, TimeRange{}, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelValuesWithFiltersOnTimeRange(filters=%s): %s", tfs, err)
	}
	sort.Strings(lvs)
	if !reflect.DeepEqual(lvs, labelValues) {
		t.Fatalf("unexpected labelValues; got\n%s\nwant\n%s", lvs, labelValues)
	}

	// Check SearchLabelValuesWithFiltersOnTimeRange with the specified filter and time range.
	lvs, err = db.SearchLabelValuesWithFiltersOnTimeRange(nil, "", []*TagFilters{tfs}, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelValuesWithFiltersOnTimeRange(filters=%s, timeRange=%s): %s", tfs, &tr, err)
	}
	sort.Strings(lvs)
	if !reflect.DeepEqual(lvs, labelValues) {
		t.Fatalf("unexpected labelValues; got\n%s\nwant\n%s", lvs, labelValues)
	}

	// Check SearchLabelValuesWithFiltersOnTimeRange with filters on metric name and time range.
	lvs, err = db.SearchLabelValuesWithFiltersOnTimeRange(nil, "", []*TagFilters{tfsMetricName}, tr, 10000, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("unexpected error in SearchLabelValuesWithFiltersOnTimeRange(filters=%s, timeRange=%s): %s", tfs, &tr, err)
	}
	sort.Strings(lvs)
	if !reflect.DeepEqual(lvs, labelValues) {
		t.Fatalf("unexpected labelValues; got\n%s\nwant\n%s", lvs, labelValues)
	}

	// Perform a search across all the days, should match all metrics
	tr = TimeRange{
		MinTimestamp: int64(now - msecPerDay*days),
		MaxTimestamp: int64(now),
	}

	matchedTSIDs, err = searchTSIDsInTest(db, []*TagFilters{tfs}, tr)
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

	s.MustClose()
	fs.MustRemoveAll(path)
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

		metricIDCache:     workingsetcache.New(1234),
		metricNameCache:   workingsetcache.New(1234),
		tsidCache:         workingsetcache.New(1234),
		dateMetricIDCache: newDateMetricIDCache(),
		retentionMsecs:    retentionMax.Milliseconds(),
	}
	s.setDeletedMetricIDs(&uint64set.Set{})
	return s
}

func stopTestStorage(s *Storage) {
	s.metricIDCache.Stop()
	s.metricNameCache.Stop()
	s.tsidCache.Stop()
	fs.MustRemoveDirAtomic(s.cachePath)
}
