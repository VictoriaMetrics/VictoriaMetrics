package mergeset

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"
	"time"
)

func TestMergeBlockStreams(t *testing.T) {
	for _, blocksToMerge := range []int{1, 2, 3, 4, 5, 10, 20} {
		t.Run(fmt.Sprintf("blocks-%d", blocksToMerge), func(t *testing.T) {
			for _, maxItemsPerBlock := range []int{1, 2, 10, 100, 1000, 10000} {
				t.Run(fmt.Sprintf("maxItemsPerBlock-%d", maxItemsPerBlock), func(t *testing.T) {
					testMergeBlockStreams(t, blocksToMerge, maxItemsPerBlock)
				})
			}
		})
	}
}

func TestMultilevelMerge(t *testing.T) {
	r := rand.New(rand.NewSource(1))

	// Prepare blocks to merge.
	bsrs, items := newTestInmemoryBlockStreamReaders(r, 10, 4000)
	var itemsMerged atomic.Uint64

	// First level merge
	var dstIP1 inmemoryPart
	var bsw1 blockStreamWriter
	bsw1.MustInitFromInmemoryPart(&dstIP1, -5)
	if err := mergeBlockStreams(&dstIP1.ph, &bsw1, bsrs[:5], nil, nil, &itemsMerged); err != nil {
		t.Fatalf("cannot merge first level part 1: %s", err)
	}

	var dstIP2 inmemoryPart
	var bsw2 blockStreamWriter
	bsw2.MustInitFromInmemoryPart(&dstIP2, -5)
	if err := mergeBlockStreams(&dstIP2.ph, &bsw2, bsrs[5:], nil, nil, &itemsMerged); err != nil {
		t.Fatalf("cannot merge first level part 2: %s", err)
	}

	if n := itemsMerged.Load(); n != uint64(len(items)) {
		t.Fatalf("unexpected itemsMerged; got %d; want %d", n, len(items))
	}

	// Second level merge (aka final merge)
	itemsMerged.Store(0)
	var dstIP inmemoryPart
	var bsw blockStreamWriter
	bsrsTop := []*blockStreamReader{
		newTestBlockStreamReader(&dstIP1),
		newTestBlockStreamReader(&dstIP2),
	}
	bsw.MustInitFromInmemoryPart(&dstIP, 1)
	if err := mergeBlockStreams(&dstIP.ph, &bsw, bsrsTop, nil, nil, &itemsMerged); err != nil {
		t.Fatalf("cannot merge second level: %s", err)
	}
	if n := itemsMerged.Load(); n != uint64(len(items)) {
		t.Fatalf("unexpected itemsMerged after final merge; got %d; want %d", n, len(items))
	}

	// Verify the resulting part (dstIP) contains all the items
	// in the correct order.
	if err := testCheckItems(&dstIP, items); err != nil {
		t.Fatalf("error checking items: %s", err)
	}
}

func TestMergeForciblyStop(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	bsrs, _ := newTestInmemoryBlockStreamReaders(r, 20, 4000)
	var dstIP inmemoryPart
	var bsw blockStreamWriter
	bsw.MustInitFromInmemoryPart(&dstIP, 1)
	ch := make(chan struct{})
	var itemsMerged atomic.Uint64
	close(ch)
	if err := mergeBlockStreams(&dstIP.ph, &bsw, bsrs, nil, ch, &itemsMerged); !errors.Is(err, errForciblyStopped) {
		t.Fatalf("unexpected error during merge: got %v; want %v", err, errForciblyStopped)
	}
	if n := itemsMerged.Load(); n != 0 {
		t.Fatalf("unexpected itemsMerged; got %d; want %d", n, 0)
	}
}

func testMergeBlockStreams(t *testing.T, blocksToMerge, maxItemsPerBlock int) {
	t.Helper()

	r := rand.New(rand.NewSource(1))
	if err := testMergeBlockStreamsSerial(r, blocksToMerge, maxItemsPerBlock); err != nil {
		t.Fatalf("unexpected error in serial test: %s", err)
	}

	const concurrency = 3
	ch := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func(n int) {
			rLocal := rand.New(rand.NewSource(int64(n)))
			ch <- testMergeBlockStreamsSerial(rLocal, blocksToMerge, maxItemsPerBlock)
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error in concurrent test: %s", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout in concurrent test")
		}
	}
}

func testMergeBlockStreamsSerial(r *rand.Rand, blocksToMerge, maxItemsPerBlock int) error {
	// Prepare blocks to merge.
	bsrs, items := newTestInmemoryBlockStreamReaders(r, blocksToMerge, maxItemsPerBlock)

	// Merge blocks.
	var itemsMerged atomic.Uint64
	var dstIP inmemoryPart
	var bsw blockStreamWriter
	bsw.MustInitFromInmemoryPart(&dstIP, -4)
	if err := mergeBlockStreams(&dstIP.ph, &bsw, bsrs, nil, nil, &itemsMerged); err != nil {
		return fmt.Errorf("cannot merge block streams: %w", err)
	}
	if n := itemsMerged.Load(); n != uint64(len(items)) {
		return fmt.Errorf("unexpected itemsMerged; got %d; want %d", n, len(items))
	}

	// Verify the resulting part (dstIP) contains all the items
	// in the correct order.
	if err := testCheckItems(&dstIP, items); err != nil {
		return fmt.Errorf("error checking items: %w", err)
	}
	return nil
}

func testCheckItems(dstIP *inmemoryPart, items []string) error {
	if int(dstIP.ph.itemsCount) != len(items) {
		return fmt.Errorf("unexpected number of items in the part; got %d; want %d", dstIP.ph.itemsCount, len(items))
	}
	if string(dstIP.ph.firstItem) != string(items[0]) {
		return fmt.Errorf("unexpected first item; got %q; want %q", dstIP.ph.firstItem, items[0])
	}
	if string(dstIP.ph.lastItem) != string(items[len(items)-1]) {
		return fmt.Errorf("unexpected last item; got %q; want %q", dstIP.ph.lastItem, items[len(items)-1])
	}

	var dstItems []string
	dstBsr := newTestBlockStreamReader(dstIP)
	for dstBsr.Next() {
		bh := dstBsr.bh
		if int(bh.itemsCount) != len(dstBsr.Block.items) {
			return fmt.Errorf("unexpected number of items in the block; got %d; want %d", len(dstBsr.Block.items), bh.itemsCount)
		}
		if bh.itemsCount <= 0 {
			return fmt.Errorf("unexpected empty block")
		}
		item := dstBsr.Block.items[0].Bytes(dstBsr.Block.data)
		if string(bh.firstItem) != string(item) {
			return fmt.Errorf("unexpected blockHeader.firstItem; got %q; want %q", bh.firstItem, item)
		}
		for _, it := range dstBsr.Block.items {
			item := it.Bytes(dstBsr.Block.data)
			dstItems = append(dstItems, string(item))
		}
	}
	if err := dstBsr.Error(); err != nil {
		return fmt.Errorf("unexpected error in dstBsr: %w", err)
	}
	if !reflect.DeepEqual(items, dstItems) {
		return fmt.Errorf("unequal items\ngot\n%q\nwant\n%q", dstItems, items)
	}
	return nil
}

func newTestInmemoryBlockStreamReaders(r *rand.Rand, blocksCount, maxItemsPerBlock int) ([]*blockStreamReader, []string) {
	var items []string
	var bsrs []*blockStreamReader
	for i := 0; i < blocksCount; i++ {
		var ib inmemoryBlock
		itemsPerBlock := r.Intn(maxItemsPerBlock) + 1
		for j := 0; j < itemsPerBlock; j++ {
			item := getRandomBytes(r)
			if !ib.Add(item) {
				break
			}
			items = append(items, string(item))
		}
		var ip inmemoryPart
		ip.Init(&ib)
		bsr := newTestBlockStreamReader(&ip)
		bsrs = append(bsrs, bsr)
	}
	sort.Strings(items)
	return bsrs, items
}

func newTestBlockStreamReader(ip *inmemoryPart) *blockStreamReader {
	var bsr blockStreamReader
	bsr.MustInitFromInmemoryPart(ip)
	return &bsr
}
