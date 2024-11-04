package mergeset

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"sync/atomic"
	"testing"
)

func BenchmarkTableSearch(b *testing.B) {
	for _, itemsCount := range []int{1e5, 1e6, 1e7} {
		b.Run(fmt.Sprintf("itemsCount-%d", itemsCount), func(b *testing.B) {
			benchmarkTableSearch(b, itemsCount)
		})
	}
}

func benchmarkTableSearch(b *testing.B, itemsCount int) {
	r := rand.New(rand.NewSource(1))

	path := fmt.Sprintf("BenchmarkTableSearch-%d", itemsCount)
	if err := os.RemoveAll(path); err != nil {
		b.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

	tb, items, err := newTestTable(r, path, itemsCount)
	if err != nil {
		panic(fmt.Errorf("cannot create test table at %q with %d items: %w", path, itemsCount, err))
	}

	// Force finishing pending merges
	tb.MustClose()
	var isReadOnly atomic.Bool
	tb = MustOpenTable(path, nil, nil, &isReadOnly)
	defer tb.MustClose()

	keys := make([][]byte, len(items))
	for i, item := range items {
		keys[i] = []byte(item)
	}

	b.Run("sequential-keys-exact", func(b *testing.B) {
		benchmarkTableSearchKeys(b, tb, keys, 0)
	})
	b.Run("sequential-keys-without-suffix", func(b *testing.B) {
		benchmarkTableSearchKeys(b, tb, keys, 4)
	})

	randKeys := append([][]byte{}, keys...)
	r.Shuffle(len(randKeys), func(i, j int) {
		randKeys[i], randKeys[j] = randKeys[j], randKeys[i]
	})
	b.Run("random-keys-exact", func(b *testing.B) {
		benchmarkTableSearchKeys(b, tb, randKeys, 0)
	})
	b.Run("random-keys-without-suffix", func(b *testing.B) {
		benchmarkTableSearchKeys(b, tb, randKeys, 4)
	})
}

func benchmarkTableSearchKeys(b *testing.B, tb *Table, keys [][]byte, stripSuffix int) {
	for _, rowsToScan := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("rowsToScan-%d", rowsToScan), func(b *testing.B) {
			benchmarkTableSearchKeysExt(b, tb, keys, stripSuffix, rowsToScan)
		})
	}
}

func benchmarkTableSearchKeysExt(b *testing.B, tb *Table, keys [][]byte, stripSuffix, rowsToScan int) {
	searchKeysCount := 1000
	if searchKeysCount >= len(keys) {
		searchKeysCount = len(keys) - 1
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(searchKeysCount * rowsToScan))
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(1))
		var ts TableSearch
		ts.Init(tb)
		defer ts.MustClose()
		for pb.Next() {
			startIdx := r.Intn(len(keys) - searchKeysCount)
			searchKeys := keys[startIdx : startIdx+searchKeysCount]
			for i, key := range searchKeys {
				searchKey := key
				if len(searchKey) < stripSuffix {
					searchKey = nil
				} else {
					searchKey = searchKey[:len(searchKey)-stripSuffix]
				}
				ts.Seek(searchKey)
				if !ts.NextItem() {
					panic(fmt.Errorf("BUG: NextItem must return true for searchKeys[%d]=%q; err=%v", i, searchKey, ts.Error()))
				}
				if !bytes.HasPrefix(ts.Item, searchKey) {
					panic(fmt.Errorf("BUG: unexpected item found for searchKey[%d]=%q; got %q; want %q", i, searchKey, ts.Item, key))
				}
				for j := 1; j < rowsToScan; j++ {
					if !ts.NextItem() {
						break
					}
				}
				if err := ts.Error(); err != nil {
					panic(fmt.Errorf("BUG: unexpected error for searchKeys[%d]=%q: %w", i, searchKey, err))
				}
			}
		}
	})
}
