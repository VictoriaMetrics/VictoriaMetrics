package mergeset

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"
)

func TestPartSearch(t *testing.T) {
	p, items, err := newTestPart(10, 4000)
	if err != nil {
		t.Fatalf("cannot create test part: %s", err)
	}

	t.Run("serial", func(t *testing.T) {
		if err := testPartSearchSerial(p, items); err != nil {
			t.Fatalf("error in serial part search test: %s", err)
		}
	})

	t.Run("concurrent", func(t *testing.T) {
		if err := testPartSearchConcurrent(p, items); err != nil {
			t.Fatalf("error in concurrent part search test: %s", err)
		}
	})
}

func testPartSearchConcurrent(p *part, items []string) error {
	const goroutinesCount = 5
	ch := make(chan error, goroutinesCount)
	for i := 0; i < goroutinesCount; i++ {
		go func() {
			ch <- testPartSearchSerial(p, items)
		}()
	}
	for i := 0; i < goroutinesCount; i++ {
		select {
		case err := <-ch:
			if err != nil {
				return err
			}
		case <-time.After(time.Second * 5):
			return fmt.Errorf("timeout")
		}
	}
	return nil
}

func testPartSearchSerial(p *part, items []string) error {
	var ps partSearch

	ps.Init(p, nil)
	var k []byte

	// Search for the item smaller than the items[0]
	k = append(k[:0], items[0]...)
	if len(k) > 0 {
		k = k[:len(k)-1]
	}
	ps.Seek(k)
	for i, item := range items {
		if !ps.NextItem() {
			return fmt.Errorf("missing item at position %d", i)
		}
		if string(ps.Item) != item {
			return fmt.Errorf("unexpected item found at position %d; got %X; want %X", i, ps.Item, item)
		}
	}
	if ps.NextItem() {
		return fmt.Errorf("unexpected item found past the end of all the items: %X", ps.Item)
	}
	if err := ps.Error(); err != nil {
		return fmt.Errorf("unexpected error: %w", err)
	}

	// Search for the item bigger than the items[len(items)-1]
	k = append(k[:0], items[len(items)-1]...)
	k = append(k, "tail"...)
	ps.Seek(k)
	if ps.NextItem() {
		return fmt.Errorf("unexpected item found: %X; want nothing", ps.Item)
	}
	if err := ps.Error(); err != nil {
		return fmt.Errorf("unexpected error when searching past the last item: %w", err)
	}

	// Search for inner items
	for loop := 0; loop < 100; loop++ {
		idx := rand.Intn(len(items))
		k = append(k[:0], items[idx]...)
		ps.Seek(k)
		n := sort.Search(len(items), func(i int) bool {
			return string(k) <= string(items[i])
		})
		for i := n; i < len(items); i++ {
			if !ps.NextItem() {
				return fmt.Errorf("missing item at position %d for idx %d on the loop %d", i, n, loop)
			}
			if string(ps.Item) != items[i] {
				return fmt.Errorf("unexpected item found at position %d for idx %d out of %d items; loop %d; key=%X; got %X; want %X",
					i, n, len(items), loop, k, ps.Item, items[i])
			}
		}
		if ps.NextItem() {
			return fmt.Errorf("unexpected item found past the end of all the items for idx %d out of %d items; loop %d: got %X", n, len(items), loop, ps.Item)
		}
		if err := ps.Error(); err != nil {
			return fmt.Errorf("unexpected error on loop %d: %w", loop, err)
		}
	}

	// Search for sorted items
	for i, item := range items {
		ps.Seek([]byte(item))
		if !ps.NextItem() {
			return fmt.Errorf("cannot find items[%d]=%X", i, item)
		}
		if string(ps.Item) != item {
			return fmt.Errorf("unexpected item found at position %d: got %X; want %X", i, ps.Item, item)
		}
		if err := ps.Error(); err != nil {
			return fmt.Errorf("unexpected error when searching for items[%d]=%X: %w", i, item, err)
		}
	}

	// Search for reversely sorted items
	for i := 0; i < len(items); i++ {
		item := items[len(items)-i-1]
		ps.Seek([]byte(item))
		if !ps.NextItem() {
			return fmt.Errorf("cannot find items[%d]=%X", i, item)
		}
		if string(ps.Item) != item {
			return fmt.Errorf("unexpected item found at position %d: got %X; want %X", i, ps.Item, item)
		}
		if err := ps.Error(); err != nil {
			return fmt.Errorf("unexpected error when searching for items[%d]=%X: %w", i, item, err)
		}
	}

	return nil
}

func newTestPart(blocksCount, maxItemsPerBlock int) (*part, []string, error) {
	bsrs, items := newTestInmemoryBlockStreamReaders(blocksCount, maxItemsPerBlock)

	var itemsMerged uint64
	var ip inmemoryPart
	var bsw blockStreamWriter
	bsw.InitFromInmemoryPart(&ip)
	if err := mergeBlockStreams(&ip.ph, &bsw, bsrs, nil, nil, &itemsMerged); err != nil {
		return nil, nil, fmt.Errorf("cannot merge blocks: %w", err)
	}
	if itemsMerged != uint64(len(items)) {
		return nil, nil, fmt.Errorf("unexpected itemsMerged; got %d; want %d", itemsMerged, len(items))
	}
	size := ip.size()
	p, err := newPart(&ip.ph, "partName", size, ip.metaindexData.NewReader(), &ip.indexData, &ip.itemsData, &ip.lensData)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create part: %w", err)
	}
	return p, items, nil
}
