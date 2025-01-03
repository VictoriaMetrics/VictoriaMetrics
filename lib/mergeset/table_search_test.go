package mergeset

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync/atomic"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	n := m.Run()
	os.Exit(n)
}

func TestTableSearchSerial(t *testing.T) {
	const path = "TestTableSearchSerial"
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

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
		tb := MustOpenTable(path, nil, nil, &isReadOnly)
		defer tb.MustClose()
		if err := testTableSearchSerial(tb, items); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}()
}

func TestTableSearchConcurrent(t *testing.T) {
	const path = "TestTableSearchConcurrent"
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

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
		tb := MustOpenTable(path, nil, nil, &isReadOnly)
		defer tb.MustClose()
		if err := testTableSearchConcurrent(tb, items); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}()
}

func testTableSearchConcurrent(tb *Table, items []string) error {
	const goroutines = 5
	ch := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			ch <- testTableSearchSerial(tb, items)
		}()
	}
	for i := 0; i < goroutines; i++ {
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
	ts.Init(tb)
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
	tb := MustOpenTable(path, flushCallback, nil, &isReadOnly)
	items := make([]string, itemsCount)
	for i := 0; i < itemsCount; i++ {
		item := fmt.Sprintf("%d:%d", r.Intn(1e9), i)
		tb.AddItems([][]byte{[]byte(item)})
		items[i] = item
	}
	tb.DebugFlush()
	if itemsCount > 0 && flushes.Load() == 0 {
		return nil, nil, fmt.Errorf("unexpeted zero flushes for itemsCount=%d", itemsCount)
	}

	sort.Strings(items)
	return tb, items, nil
}
