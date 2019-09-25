package mergeset

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestTableOpenClose(t *testing.T) {
	const path = "TestTableOpenClose"
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

	// Create a new table
	tb, err := OpenTable(path, nil, nil)
	if err != nil {
		t.Fatalf("cannot create new table: %s", err)
	}

	// Close it
	tb.MustClose()

	// Re-open created table multiple times.
	for i := 0; i < 10; i++ {
		tb, err := OpenTable(path, nil, nil)
		if err != nil {
			t.Fatalf("cannot open created table: %s", err)
		}
		tb.MustClose()
	}
}

func TestTableOpenMultipleTimes(t *testing.T) {
	const path = "TestTableOpenMultipleTimes"
	defer func() {
		_ = os.RemoveAll(path)
	}()

	tb1, err := OpenTable(path, nil, nil)
	if err != nil {
		t.Fatalf("cannot open table: %s", err)
	}
	defer tb1.MustClose()

	for i := 0; i < 10; i++ {
		tb2, err := OpenTable(path, nil, nil)
		if err == nil {
			tb2.MustClose()
			t.Fatalf("expecting non-nil error when opening already opened table")
		}
	}
}

func TestTableAddItemSerial(t *testing.T) {
	const path = "TestTableAddItemSerial"
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

	var flushes uint64
	flushCallback := func() {
		atomic.AddUint64(&flushes, 1)
	}
	tb, err := OpenTable(path, flushCallback, nil)
	if err != nil {
		t.Fatalf("cannot open %q: %s", path, err)
	}

	const itemsCount = 1e5
	testAddItemsSerial(tb, itemsCount)

	// Verify items count after pending items flush.
	tb.DebugFlush()
	if atomic.LoadUint64(&flushes) == 0 {
		t.Fatalf("unexpected zero flushes")
	}

	var m TableMetrics
	tb.UpdateMetrics(&m)
	if m.ItemsCount != itemsCount {
		t.Fatalf("unexpected itemsCount; got %d; want %v", m.ItemsCount, itemsCount)
	}

	tb.MustClose()

	// Re-open the table and make sure ItemsCount remains the same.
	testReopenTable(t, path, itemsCount)

	// Add more items in order to verify merge between inmemory parts and file-based parts.
	tb, err = OpenTable(path, nil, nil)
	if err != nil {
		t.Fatalf("cannot open %q: %s", path, err)
	}
	const moreItemsCount = itemsCount * 3
	testAddItemsSerial(tb, moreItemsCount)
	tb.MustClose()

	// Re-open the table and verify ItemsCount again.
	testReopenTable(t, path, itemsCount+moreItemsCount)
}

func testAddItemsSerial(tb *Table, itemsCount int) {
	for i := 0; i < itemsCount; i++ {
		item := getRandomBytes()
		if len(item) > maxInmemoryBlockSize {
			item = item[:maxInmemoryBlockSize]
		}
		if err := tb.AddItems([][]byte{item}); err != nil {
			logger.Panicf("BUG: cannot add item to table: %s", err)
		}
	}
}

func TestTableCreateSnapshotAt(t *testing.T) {
	const path = "TestTableCreateSnapshotAt"
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

	tb, err := OpenTable(path, nil, nil)
	if err != nil {
		t.Fatalf("cannot open %q: %s", path, err)
	}
	defer tb.MustClose()

	// Write a lot of items into the table, so background merges would start.
	const itemsCount = 3e5
	for i := 0; i < itemsCount; i++ {
		item := []byte(fmt.Sprintf("item %d", i))
		if err := tb.AddItems([][]byte{item}); err != nil {
			t.Fatalf("cannot add item to table: %s", err)
		}
	}
	tb.DebugFlush()

	// Create multiple snapshots.
	snapshot1 := path + "-test-snapshot1"
	if err := tb.CreateSnapshotAt(snapshot1); err != nil {
		t.Fatalf("cannot create snapshot1: %s", err)
	}
	snapshot2 := path + "-test-snapshot2"
	if err := tb.CreateSnapshotAt(snapshot2); err != nil {
		t.Fatalf("cannot create snapshot2: %s", err)
	}
	defer func() {
		_ = os.RemoveAll(snapshot1)
		_ = os.RemoveAll(snapshot2)
	}()

	// Verify snapshots contain all the data.
	tb1, err := OpenTable(snapshot1, nil, nil)
	if err != nil {
		t.Fatalf("cannot open %q: %s", path, err)
	}
	defer tb1.MustClose()

	tb2, err := OpenTable(snapshot2, nil, nil)
	if err != nil {
		t.Fatalf("cannot open %q: %s", path, err)
	}
	defer tb2.MustClose()

	var ts, ts1, ts2 TableSearch
	ts.Init(tb, nil)
	ts1.Init(tb1, nil)
	defer ts1.MustClose()
	ts2.Init(tb2, nil)
	defer ts2.MustClose()
	for i := 0; i < itemsCount; i++ {
		key := []byte(fmt.Sprintf("item %d", i))
		if err := ts.FirstItemWithPrefix(key); err != nil {
			t.Fatalf("cannot find item[%d]=%q in the original table: %s", i, key, err)
		}
		if !bytes.Equal(key, ts.Item) {
			t.Fatalf("unexpected item found for key=%q in the original table; got %q", key, ts.Item)
		}
		if err := ts1.FirstItemWithPrefix(key); err != nil {
			t.Fatalf("cannot find item[%d]=%q in snapshot1: %s", i, key, err)
		}
		if !bytes.Equal(key, ts1.Item) {
			t.Fatalf("unexpected item found for key=%q in snapshot1; got %q", key, ts1.Item)
		}
		if err := ts2.FirstItemWithPrefix(key); err != nil {
			t.Fatalf("cannot find item[%d]=%q in snapshot2: %s", i, key, err)
		}
		if !bytes.Equal(key, ts2.Item) {
			t.Fatalf("unexpected item found for key=%q in snapshot2; got %q", key, ts2.Item)
		}
	}
}

func TestTableAddItemsConcurrent(t *testing.T) {
	const path = "TestTableAddItemsConcurrent"
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

	var flushes uint64
	flushCallback := func() {
		atomic.AddUint64(&flushes, 1)
	}
	var itemsMerged uint64
	prepareBlock := func(data []byte, items [][]byte) ([]byte, [][]byte) {
		atomic.AddUint64(&itemsMerged, uint64(len(items)))
		return data, items
	}
	tb, err := OpenTable(path, flushCallback, prepareBlock)
	if err != nil {
		t.Fatalf("cannot open %q: %s", path, err)
	}

	const itemsCount = 1e5
	testAddItemsConcurrent(tb, itemsCount)

	// Verify items count after pending items flush.
	tb.DebugFlush()
	if atomic.LoadUint64(&flushes) == 0 {
		t.Fatalf("unexpected zero flushes")
	}
	n := atomic.LoadUint64(&itemsMerged)
	if n < itemsCount {
		t.Fatalf("too low number of items merged; got %v; must be at least %v", n, itemsCount)
	}

	var m TableMetrics
	tb.UpdateMetrics(&m)
	if m.ItemsCount != itemsCount {
		t.Fatalf("unexpected itemsCount; got %d; want %v", m.ItemsCount, itemsCount)
	}

	tb.MustClose()

	// Re-open the table and make sure ItemsCount remains the same.
	testReopenTable(t, path, itemsCount)

	// Add more items in order to verify merge between inmemory parts and file-based parts.
	tb, err = OpenTable(path, nil, nil)
	if err != nil {
		t.Fatalf("cannot open %q: %s", path, err)
	}
	const moreItemsCount = itemsCount * 3
	testAddItemsConcurrent(tb, moreItemsCount)
	tb.MustClose()

	// Re-open the table and verify ItemsCount again.
	testReopenTable(t, path, itemsCount+moreItemsCount)
}

func testAddItemsConcurrent(tb *Table, itemsCount int) {
	const goroutinesCount = 6
	workCh := make(chan int, itemsCount)
	var wg sync.WaitGroup
	for i := 0; i < goroutinesCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range workCh {
				item := getRandomBytes()
				if len(item) > maxInmemoryBlockSize {
					item = item[:maxInmemoryBlockSize]
				}
				if err := tb.AddItems([][]byte{item}); err != nil {
					logger.Panicf("BUG: cannot add item to table: %s", err)
				}
			}
		}()
	}
	for i := 0; i < itemsCount; i++ {
		workCh <- i
	}
	close(workCh)
	wg.Wait()
}

func testReopenTable(t *testing.T, path string, itemsCount int) {
	t.Helper()

	for i := 0; i < 10; i++ {
		tb, err := OpenTable(path, nil, nil)
		if err != nil {
			t.Fatalf("cannot re-open %q: %s", path, err)
		}
		var m TableMetrics
		tb.UpdateMetrics(&m)
		if m.ItemsCount != uint64(itemsCount) {
			t.Fatalf("unexpected itemsCount after re-opening; got %d; want %v", m.ItemsCount, itemsCount)
		}
		tb.MustClose()
	}
}
