package mergeset

import (
	"fmt"
	"math/rand"
	"sort"
	"sync/atomic"
	"testing"
	"time"
)

func TestPartSearch(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	p, items, err := newTestPart(r, 10, 4000)
	if err != nil {
		t.Fatalf("cannot create test part: %s", err)
	}

	t.Run("serial", func(t *testing.T) {
		if err := testPartSearchSerial(r, p, items); err != nil {
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
		go func(n int) {
			rLocal := rand.New(rand.NewSource(int64(n)))
			ch <- testPartSearchSerial(rLocal, p, items)
		}(i)
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

func testPartSearchSerial(r *rand.Rand, p *part, items []string) error {
	var ps partSearch

	ps.Init(p, true)
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
		idx := r.Intn(len(items))
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

func newTestPart(r *rand.Rand, blocksCount, maxItemsPerBlock int) (*part, []string, error) {
	bsrs, items := newTestInmemoryBlockStreamReaders(r, blocksCount, maxItemsPerBlock)

	var itemsMerged atomic.Uint64
	var ip inmemoryPart
	var bsw blockStreamWriter
	bsw.MustInitFromInmemoryPart(&ip, -3)
	if err := mergeBlockStreams(&ip.ph, &bsw, bsrs, nil, nil, &itemsMerged); err != nil {
		return nil, nil, fmt.Errorf("cannot merge blocks: %w", err)
	}
	if n := itemsMerged.Load(); n != uint64(len(items)) {
		return nil, nil, fmt.Errorf("unexpected itemsMerged; got %d; want %d", n, len(items))
	}
	size := ip.size()
	p := newPart(&ip.ph, "partName", size, ip.metaindexData.NewReader(), &ip.indexData, &ip.itemsData, &ip.lensData)
	return p, items, nil
}

func TestGetInmemoryBlockWithZeroSizeBlock(t *testing.T) {
	var ph partHeader
	var ip inmemoryPart
	var bsw blockStreamWriter
	bsw.MustInitFromInmemoryPart(&ip, -3)

	buildBlock := func(items ...string) inmemoryBlock {
		var ib inmemoryBlock
		for _, item := range items {
			if !ib.Add([]byte(item)) {
				t.Fatalf("cannot add item %q", item)
			}
		}
		ib.SortItems()
		return ib
	}

	writeBlock := func(ib inmemoryBlock) {
		if len(ib.items) == 0 {
			t.Fatalf("block must contain items")
		}
		data := ib.data
		ph.itemsCount += uint64(len(ib.items))
		if ph.blocksCount == 0 {
			ph.firstItem = append(ph.firstItem[:0], ib.items[0].Bytes(data)...)
		}
		ph.lastItem = append(ph.lastItem[:0], ib.items[len(ib.items)-1].Bytes(data)...)
		ph.blocksCount++
		bsw.WriteBlock(&ib)
	}

	writeBlock(buildBlock("a"))
	writeBlock(buildBlock("b0", "b1"))
	bsw.MustClose()

	p := newPart(&ph, "test", ip.size(), ip.metaindexData.NewReader(), &ip.indexData, &ip.itemsData, &ip.lensData)
	defer p.MustClose()

	var ps partSearch
	ps.Init(p, false)
	ps.mrs = p.mrs
	if err := ps.nextBHS(); err != nil {
		t.Fatalf("cannot read block headers: %s", err)
	}
	if len(ps.bhs) != 2 {
		t.Fatalf("unexpected block headers count: %d", len(ps.bhs))
	}
	if ps.bhs[0].itemsBlockOffset != ps.bhs[1].itemsBlockOffset {
		t.Fatalf("blocks must share itemsBlockOffset for the test: %d vs %d", ps.bhs[0].itemsBlockOffset, ps.bhs[1].itemsBlockOffset)
	}
	if ps.bhs[0].itemsBlockSize != 0 {
		t.Fatalf("the first block must have zero itemsBlockSize; got %d", ps.bhs[0].itemsBlockSize)
	}

	for i := range 4 {
		if _, err := ps.getInmemoryBlock(&ps.bhs[1]); err != nil {
			t.Fatalf("cannot load non-empty block at iteration %d: %s", i, err)
		}
	}
	ibZero, err := ps.getInmemoryBlock(&ps.bhs[0])
	if err != nil {
		t.Fatalf("cannot load zero-size block: %s", err)
	}
	if len(ibZero.items) != int(ps.bhs[0].itemsCount) {
		t.Fatalf("unexpected items count in zero-size block; got %d; want %d", len(ibZero.items), ps.bhs[0].itemsCount)
	}
	if got := string(ibZero.items[0].Bytes(ibZero.data)); got != "a" {
		t.Fatalf("unexpected item in zero-size block; got %q; want %q", got, "a")
	}
}
