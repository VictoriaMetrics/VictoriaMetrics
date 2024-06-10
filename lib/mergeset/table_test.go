package mergeset

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
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
	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, nil, nil, &isReadOnly)

	// Close it
	tb.MustClose()

	// Re-open created table multiple times.
	for i := 0; i < 4; i++ {
		tb := MustOpenTable(path, nil, nil, &isReadOnly)
		tb.MustClose()
	}
}

func TestTableAddItemsTooLongItem(t *testing.T) {
	const path = "TestTableAddItemsTooLongItem"
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}

	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, nil, nil, &isReadOnly)
	tb.AddItems([][]byte{make([]byte, maxInmemoryBlockSize+1)})
	tb.MustClose()
	_ = os.RemoveAll(path)
}

func TestTableAddItemsSerial(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	const path = "TestTableAddItemsSerial"
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

	var flushes atomic.Uint64
	flushCallback := func() {
		flushes.Add(1)
	}
	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, flushCallback, nil, &isReadOnly)

	const itemsCount = 10e3
	testAddItemsSerial(r, tb, itemsCount)

	// Verify items count after pending items flush.
	tb.DebugFlush()
	if flushes.Load() == 0 {
		t.Fatalf("unexpected zero flushes")
	}

	var m TableMetrics
	tb.UpdateMetrics(&m)
	if n := m.TotalItemsCount(); n != itemsCount {
		t.Fatalf("unexpected itemsCount; got %d; want %v", n, itemsCount)
	}

	tb.MustClose()

	// Re-open the table and make sure itemsCount remains the same.
	testReopenTable(t, path, itemsCount)

	// Add more items in order to verify merge between inmemory parts and file-based parts.
	tb = MustOpenTable(path, nil, nil, &isReadOnly)
	const moreItemsCount = itemsCount * 3
	testAddItemsSerial(r, tb, moreItemsCount)
	tb.MustClose()

	// Re-open the table and verify itemsCount again.
	testReopenTable(t, path, itemsCount+moreItemsCount)
}

func testAddItemsSerial(r *rand.Rand, tb *Table, itemsCount int) {
	for i := 0; i < itemsCount; i++ {
		item := getRandomBytes(r)
		if len(item) > maxInmemoryBlockSize {
			item = item[:maxInmemoryBlockSize]
		}
		tb.AddItems([][]byte{item})
	}
}

func TestTableCreateSnapshotAt(t *testing.T) {
	const path = "TestTableCreateSnapshotAt"
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}

	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, nil, nil, &isReadOnly)

	// Write a lot of items into the table, so background merges would start.
	const itemsCount = 3e5
	for i := 0; i < itemsCount; i++ {
		item := []byte(fmt.Sprintf("item %d", i))
		tb.AddItems([][]byte{item})
	}

	// Close and open the table in order to flush all the data to disk before creating snapshots.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4272#issuecomment-1550221840
	tb.MustClose()
	tb = MustOpenTable(path, nil, nil, &isReadOnly)

	// Create multiple snapshots.
	snapshot1 := path + "-test-snapshot1"
	if err := tb.CreateSnapshotAt(snapshot1); err != nil {
		t.Fatalf("cannot create snapshot1: %s", err)
	}
	snapshot2 := path + "-test-snapshot2"
	if err := tb.CreateSnapshotAt(snapshot2); err != nil {
		t.Fatalf("cannot create snapshot2: %s", err)
	}

	// Verify snapshots contain all the data.
	tb1 := MustOpenTable(snapshot1, nil, nil, &isReadOnly)
	tb2 := MustOpenTable(snapshot2, nil, nil, &isReadOnly)

	var ts, ts1, ts2 TableSearch
	ts.Init(tb)
	ts1.Init(tb1)
	ts2.Init(tb2)
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
	ts1.MustClose()
	ts2.MustClose()

	// Close and remove tables.
	tb2.MustClose()
	tb1.MustClose()
	tb.MustClose()

	_ = os.RemoveAll(snapshot2)
	_ = os.RemoveAll(snapshot1)
	_ = os.RemoveAll(path)
}

func TestTableAddItemsConcurrentStress(t *testing.T) {
	const path = "TestTableAddItemsConcurrentStress"
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

	rawItemsShardsPerTableOrig := rawItemsShardsPerTable
	maxBlocksPerShardOrig := maxBlocksPerShard
	rawItemsShardsPerTable = 10
	maxBlocksPerShard = 3
	defer func() {
		rawItemsShardsPerTable = rawItemsShardsPerTableOrig
		maxBlocksPerShard = maxBlocksPerShardOrig
	}()

	var flushes atomic.Uint64
	flushCallback := func() {
		flushes.Add(1)
	}
	prepareBlock := func(data []byte, items []Item) ([]byte, []Item) {
		return data, items
	}

	blocksNeeded := rawItemsShardsPerTable * maxBlocksPerShard * 10
	testAddItems := func(tb *Table) {
		itemsBatch := make([][]byte, 0)

		for j := 0; j < blocksNeeded; j++ {
			item := bytes.Repeat([]byte{byte(j)}, maxInmemoryBlockSize-10)
			itemsBatch = append(itemsBatch, item)
		}
		tb.AddItems(itemsBatch)
	}

	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, flushCallback, prepareBlock, &isReadOnly)

	testAddItems(tb)

	// Verify items count after pending items flush.
	tb.DebugFlush()
	if flushes.Load() == 0 {
		t.Fatalf("unexpected zero flushes")
	}

	var m TableMetrics
	tb.UpdateMetrics(&m)
	if n := m.TotalItemsCount(); n != uint64(blocksNeeded) {
		t.Fatalf("unexpected itemsCount; got %d; want %v", n, blocksNeeded)
	}

	tb.MustClose()

	// Re-open the table and make sure itemsCount remains the same.
	testReopenTable(t, path, blocksNeeded)
}

func TestTableAddItemsConcurrent(t *testing.T) {
	const path = "TestTableAddItemsConcurrent"
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

	var flushes atomic.Uint64
	flushCallback := func() {
		flushes.Add(1)
	}
	prepareBlock := func(data []byte, items []Item) ([]byte, []Item) {
		return data, items
	}
	var isReadOnly atomic.Bool
	tb := MustOpenTable(path, flushCallback, prepareBlock, &isReadOnly)

	const itemsCount = 10e3
	testAddItemsConcurrent(tb, itemsCount)

	// Verify items count after pending items flush.
	tb.DebugFlush()
	if flushes.Load() == 0 {
		t.Fatalf("unexpected zero flushes")
	}

	var m TableMetrics
	tb.UpdateMetrics(&m)
	if n := m.TotalItemsCount(); n != itemsCount {
		t.Fatalf("unexpected itemsCount; got %d; want %v", n, itemsCount)
	}

	tb.MustClose()

	// Re-open the table and make sure itemsCount remains the same.
	testReopenTable(t, path, itemsCount)

	// Add more items in order to verify merge between inmemory parts and file-based parts.
	tb = MustOpenTable(path, nil, nil, &isReadOnly)
	const moreItemsCount = itemsCount * 3
	testAddItemsConcurrent(tb, moreItemsCount)
	tb.MustClose()

	// Re-open the table and verify itemsCount again.
	testReopenTable(t, path, itemsCount+moreItemsCount)
}

func testAddItemsConcurrent(tb *Table, itemsCount int) {
	const goroutinesCount = 6
	workCh := make(chan int, itemsCount)
	var wg sync.WaitGroup
	for i := 0; i < goroutinesCount; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(n)))
			for range workCh {
				item := getRandomBytes(r)
				if len(item) > maxInmemoryBlockSize {
					item = item[:maxInmemoryBlockSize]
				}
				tb.AddItems([][]byte{item})
			}
		}(i)
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
		var isReadOnly atomic.Bool
		tb := MustOpenTable(path, nil, nil, &isReadOnly)
		var m TableMetrics
		tb.UpdateMetrics(&m)
		if n := m.TotalItemsCount(); n != uint64(itemsCount) {
			t.Fatalf("unexpected itemsCount after re-opening; got %d; want %v", n, itemsCount)
		}
		tb.MustClose()
	}
}
