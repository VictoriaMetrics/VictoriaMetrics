package mergeset

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestTableSearchWithPartFilter(t *testing.T) {
	const path = "TestTableSearchWithPartFilter"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)

	filterKey := func(item []byte) []byte {
		n := bytes.IndexByte(item, ':')
		if n < 0 {
			return nil
		}
		return item[:n+1]
	}
	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	for i := range 500 {
		item := fmt.Appendf(nil, "series-%04d:value", i)
		tb.AddItems([][]byte{item})
	}
	tb.DebugFlush()
	for i := 500; i < 1000; i++ {
		item := fmt.Appendf(nil, "series-%04d:value", i)
		tb.AddItems([][]byte{item})
	}
	tb.DebugFlush()
	var tm TableMetrics
	tb.UpdateMetrics(&tm)
	if tm.InmemoryPartsCount < 2 {
		t.Fatalf("expected multiple parts before merge; got %d", tm.InmemoryPartsCount)
	}
	assertFilterSearch := func(tb *Table) {
		t.Helper()
		var ts TableSearch
		ts.Init(tb, false)
		stats := ts.SeekWithPartFilter([]byte("series-0500:"))
		if stats.PossibleParts == 0 {
			t.Fatalf("expected a part filter match; got %+v", stats)
		}
		if !ts.NextItem() || !bytes.HasPrefix(ts.Item, []byte("series-0500:")) {
			t.Fatalf("expected to find filtered item; got %q", ts.Item)
		}
		stats = ts.SeekWithPartFilter([]byte("missing:"))
		if stats.SkippedParts == 0 || stats.UnfilteredParts != 0 || ts.NextItem() {
			t.Fatalf("expected all parts to reject missing key; stats=%+v item=%q", stats, ts.Item)
		}
		ts.MustClose()
	}
	assertFilterSearch(tb)
	tb.MustClose()

	// The persisted filter must remain usable after reopening the table.
	tb = MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	defer tb.MustClose()
	assertFilterSearch(tb)
}

func TestMustOpenTableRejectsMultiplePartFilterKeys(t *testing.T) {
	var isReadOnly atomic.Bool
	filterKey := func(item []byte) []byte { return item }
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected MustOpenTable to reject multiple part filter callbacks")
		}
	}()
	MustOpenTable(t.Name(), 0, nil, 0, nil, &isReadOnly, filterKey, filterKey)
}

func TestTableSearchWithMissingPartFilterFallsBack(t *testing.T) {
	const path = "TestTableSearchWithMissingPartFilterFallsBack"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)

	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, 0, nil, 0, nil, &isReadOnly)
	tb.AddItems([][]byte{[]byte("series:value")})
	tb.MustClose()

	filterKey := func(item []byte) []byte { return item }
	tb = MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	defer tb.MustClose()
	var ts TableSearch
	ts.Init(tb, false)
	stats := ts.SeekWithPartFilter([]byte("missing"))
	if stats.UnfilteredParts == 0 || stats.SkippedParts != 0 {
		t.Fatalf("expected missing filter to force fallback; got %+v", stats)
	}
	if !ts.NextItem() || string(ts.Item) != "series:value" {
		t.Fatalf("expected fallback to find the existing item; got %q", ts.Item)
	}
	ts.MustClose()
}

func TestTableSearchWithCorruptPartFilterFallsBack(t *testing.T) {
	const path = "TestTableSearchWithCorruptPartFilterFallsBack"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)

	filterKey := func(item []byte) []byte { return item }
	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	tb.AddItems([][]byte{[]byte("series:value")})
	tb.MustClose()

	partNames := mustReadPartNames(filepath.Join(path, partsFilename), path)
	filterPath := filepath.Join(path, partNames[0], filterFilename)
	filterData, err := os.ReadFile(filterPath)
	if err != nil {
		t.Fatalf("cannot read part filter: %s", err)
	}
	// Corrupt the filter directory while keeping the file readable, so the filter is ignored on open.
	filterData[partFilterHeaderSize] ^= 1
	if err := os.WriteFile(filterPath, filterData, 0o600); err != nil {
		t.Fatalf("cannot corrupt part filter: %s", err)
	}

	tb = MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	defer tb.MustClose()
	var ts TableSearch
	ts.Init(tb, false)
	stats := ts.SeekWithPartFilter([]byte("missing"))
	if stats.UnfilteredParts == 0 || stats.SkippedParts != 0 || stats.HasFallbacks() != true {
		t.Fatalf("expected corrupt filter to force fallback; got %+v", stats)
	}
	if !ts.NextItem() || string(ts.Item) != "series:value" {
		t.Fatalf("expected fallback to find existing item; got %q", ts.Item)
	}
	ts.MustClose()
}

func TestTableSearchWithCorruptPartFilterSegmentFallsBack(t *testing.T) {
	const path = "TestTableSearchWithCorruptPartFilterSegmentFallsBack"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)

	filterKey := func(item []byte) []byte { return item }
	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	tb.AddItems([][]byte{[]byte("series:value")})
	tb.MustClose()

	partNames := mustReadPartNames(filepath.Join(path, partsFilename), path)
	filterPath := filepath.Join(path, partNames[0], filterFilename)
	filterData, err := os.ReadFile(filterPath)
	if err != nil {
		t.Fatalf("cannot read part filter: %s", err)
	}
	// Corrupt the first filter segment, so its checksum stops matching.
	segmentsCount := int(binary.LittleEndian.Uint32(filterData[len(partFilterMagic):]))
	if segmentsCount != 1 {
		t.Fatalf("expected a single segment for a single item; got %d", segmentsCount)
	}
	segmentOffset := binary.LittleEndian.Uint64(filterData[partFilterHeaderSize:])
	segmentSize := binary.LittleEndian.Uint32(filterData[partFilterHeaderSize+8:])
	filterData[segmentOffset+uint64(segmentSize)-1] ^= 1
	if err := os.WriteFile(filterPath, filterData, 0o600); err != nil {
		t.Fatalf("cannot corrupt part filter: %s", err)
	}

	tb = MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	defer tb.MustClose()
	var ts TableSearch
	ts.Init(tb, false)
	stats := ts.SeekWithPartFilter([]byte("missing"))
	if stats.VerifyFallbackParts == 0 || stats.SkippedParts != 0 {
		t.Fatalf("expected the corrupted filter segment to force fallback; got %+v", stats)
	}
	if !ts.NextItem() || string(ts.Item) != "series:value" {
		t.Fatalf("expected fallback to find existing item; got %q", ts.Item)
	}
	ts.MustClose()
}

func TestTableSearchWithoutPartFilterCacheFallsBack(t *testing.T) {
	const path = "TestTableSearchWithoutPartFilterCacheFallsBack"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)

	filterKey := func(item []byte) []byte { return item }
	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	tb.AddItems([][]byte{[]byte("series:value")})
	tb.MustClose()

	cacheSizeBefore := getMaxPartFilterCacheSize()
	SetPartFilterCacheSize(1)
	defer SetPartFilterCacheSize(cacheSizeBefore)

	tb = MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	defer tb.MustClose()
	var ts TableSearch
	ts.Init(tb, false)
	stats := ts.SeekWithPartFilter([]byte("missing"))
	if stats.BudgetFallbackParts == 0 || stats.SkippedParts != 0 {
		t.Fatalf("expected the missing cache budget to force fallback; got %+v", stats)
	}
	if !ts.NextItem() || string(ts.Item) != "series:value" {
		t.Fatalf("expected fallback to find existing item; got %q", ts.Item)
	}
	ts.MustClose()
}

func TestTableSearchWithOversizedPartFilterFallsBack(t *testing.T) {
	const path = "TestTableSearchWithOversizedPartFilterFallsBack"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)

	filterKey := func(item []byte) []byte { return item }
	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	tb.AddItems([][]byte{[]byte("series:value")})
	tb.MustClose()

	partNames := mustReadPartNames(filepath.Join(path, partsFilename), path)
	filterPath := filepath.Join(path, partNames[0], filterFilename)
	if err := os.WriteFile(filterPath, make([]byte, partFilterMaxBytes*10), 0o600); err != nil {
		t.Fatalf("cannot write oversized part filter: %s", err)
	}

	tb = MustOpenTable(path, 0, nil, 0, nil, &isReadOnly, filterKey)
	defer tb.MustClose()
	var ts TableSearch
	ts.Init(tb, false)
	stats := ts.SeekWithPartFilter([]byte("missing"))
	if stats.UnfilteredParts == 0 || stats.SkippedParts != 0 {
		t.Fatalf("expected oversized filter to force fallback; got %+v", stats)
	}
	if !ts.NextItem() || string(ts.Item) != "series:value" {
		t.Fatalf("expected fallback to find existing item; got %q", ts.Item)
	}
	ts.MustClose()
}

func TestMain(m *testing.M) {
	n := m.Run()
	os.Exit(n)
}

func TestTableSearchSerial(t *testing.T) {
	const path = "TestTableSearchSerial"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)

	const itemsCount = 1e5

	items := func() []string {
		r := rand.New(rand.NewSource(1))
		tb, items, err := newTestTable(r, path, itemsCount)
		if err != nil {
			t.Fatalf("cannot create test table: %s", err)
		}
		defer tb.MustClose()
		if err := testTableSearchSerial(tb, items); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		return items
	}()

	func() {
		// Re-open the table and verify the search works.
		var isReadOnly atomic.Bool
		tb := MustOpenTable(path, 0, nil, 0, nil, &isReadOnly)
		defer tb.MustClose()
		if err := testTableSearchSerial(tb, items); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}()
}

func TestTableSearchConcurrent(t *testing.T) {
	const path = "TestTableSearchConcurrent"
	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)

	const itemsCount = 1e5
	items := func() []string {
		r := rand.New(rand.NewSource(2))
		tb, items, err := newTestTable(r, path, itemsCount)
		if err != nil {
			t.Fatalf("cannot create test table: %s", err)
		}
		defer tb.MustClose()
		if err := testTableSearchConcurrent(tb, items); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		return items
	}()

	// Re-open the table and verify the search works.
	func() {
		var isReadOnly atomic.Bool
		tb := MustOpenTable(path, 0, nil, 0, nil, &isReadOnly)
		defer tb.MustClose()
		if err := testTableSearchConcurrent(tb, items); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}()
}

func testTableSearchConcurrent(tb *Table, items []string) error {
	const goroutines = 5
	ch := make(chan error, goroutines)
	for range goroutines {
		go func() {
			ch <- testTableSearchSerial(tb, items)
		}()
	}
	for range goroutines {
		select {
		case err := <-ch:
			if err != nil {
				return fmt.Errorf("unexpected error: %w", err)
			}
		case <-time.After(time.Second * 5):
			return fmt.Errorf("timeout")
		}
	}
	return nil
}

func testTableSearchSerial(tb *Table, items []string) error {
	var ts TableSearch
	ts.Init(tb, false)
	for _, key := range []string{
		"",
		"123",
		"9",
		"892",
		"2384329",
		"fdsjflfdf",
		items[0],
		items[len(items)-1],
		items[len(items)/2],
	} {
		n := sort.Search(len(items), func(i int) bool {
			return key <= items[i]
		})
		ts.Seek([]byte(key))
		for n < len(items) {
			item := items[n]
			if !ts.NextItem() {
				return fmt.Errorf("missing item %q at position %d when searching for %q", item, n, key)
			}
			if string(ts.Item) != item {
				return fmt.Errorf("unexpected item found at position %d when searching for %q; got %q; want %q", n, key, ts.Item, item)
			}
			n++
		}
		if ts.NextItem() {
			return fmt.Errorf("superfluous item found at position %d when searching for %q: %q", n, key, ts.Item)
		}
		if err := ts.Error(); err != nil {
			return fmt.Errorf("unexpected error when searching for %q: %w", key, err)
		}
	}
	ts.MustClose()
	return nil
}

func newTestTable(r *rand.Rand, path string, itemsCount int) (*Table, []string, error) {
	var flushes atomic.Uint64
	flushCallback := func() {
		flushes.Add(1)
	}
	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, 0, flushCallback, 0, nil, &isReadOnly)
	items := make([]string, itemsCount)
	for i := range itemsCount {
		item := fmt.Sprintf("%d:%d", r.Intn(1e9), i)
		tb.AddItems([][]byte{[]byte(item)})
		items[i] = item
	}
	tb.DebugFlush()
	if itemsCount > 0 && flushes.Load() == 0 {
		return nil, nil, fmt.Errorf("unexpected zero flushes for itemsCount=%d", itemsCount)
	}

	sort.Strings(items)
	return tb, items, nil
}
