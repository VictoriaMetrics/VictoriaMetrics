package mergeset

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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
	for i := range goroutinesCount {
		go func(n int) {
			rLocal := rand.New(rand.NewSource(int64(n)))
			ch <- testPartSearchSerial(rLocal, p, items)
		}(i)
	}
	for range goroutinesCount {
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
	for loop := range 100 {
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
	for i := range items {
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

	// iterate 4 times in order to place block into the cache
	// storage caches it after 2 missed requests according to the flag blockcache.missesBeforeCaching=2
	for i := range 4 {
		if _, err := ps.getInmemoryBlock(&ps.bhs[1]); err != nil {
			t.Fatalf("cannot load non-empty block at iteration %d: %s", i, err)
		}
	}
	assertBlockAt := func(bhIdx int, wantFirstItemValue string) {
		block, err := ps.getInmemoryBlock(&ps.bhs[bhIdx])
		if err != nil {
			t.Fatalf("cannot block=%d : %s", bhIdx, err)
		}
		if len(block.items) != int(ps.bhs[bhIdx].itemsCount) {
			t.Fatalf("unexpected items count in block=%d; got %d; want %d", bhIdx, len(block.items), ps.bhs[bhIdx].itemsCount)
		}
		if got := string(block.items[0].Bytes(block.data)); got != wantFirstItemValue {
			t.Fatalf("unexpected item in block=%d; got %q; want %q", bhIdx, got, wantFirstItemValue)
		}
	}
	assertBlockAt(0, "a")
	assertBlockAt(1, "b0")
}

type countingReadAtCloser struct {
	fs.MustReadAtCloser
	offsets []int64
}

func (c *countingReadAtCloser) MustReadAt(p []byte, off int64) {
	c.offsets = append(c.offsets, off)
	c.MustReadAtCloser.MustReadAt(p, off)
}

func (c *countingReadAtCloser) reset() {
	c.offsets = c.offsets[:0]
}

func newTestPartFromBlocks(t *testing.T, blocks [][]string) (*part, *countingReadAtCloser, []string) {
	t.Helper()

	var ph partHeader
	var ip inmemoryPart
	var bsw blockStreamWriter
	bsw.MustInitFromInmemoryPart(&ip, -3)

	var allItems []string
	for _, blockItems := range blocks {
		var ib inmemoryBlock
		for _, item := range blockItems {
			if !ib.Add([]byte(item)) {
				t.Fatalf("cannot add item %q", item)
			}
			allItems = append(allItems, item)
		}
		ib.SortItems()
		data := ib.data
		ph.itemsCount += uint64(len(ib.items))
		if ph.blocksCount == 0 {
			ph.firstItem = append(ph.firstItem[:0], ib.items[0].Bytes(data)...)
		}
		ph.lastItem = append(ph.lastItem[:0], ib.items[len(ib.items)-1].Bytes(data)...)
		ph.blocksCount++
		bsw.WriteBlock(&ib)
	}
	bsw.MustClose()

	counter := &countingReadAtCloser{MustReadAtCloser: &ip.itemsData}
	p := newPart(&ph, "test", ip.size(), ip.metaindexData.NewReader(), &ip.indexData, counter, &ip.lensData)
	return p, counter, allItems
}

func TestPartSearchSkipNonMatchingBlocks(t *testing.T) {
	blocks := [][]string{
		{"aaa/0", "aaa/1", "aaa/2"},
		{"ccc/0", "ccc/1", "ccc/2"},
		{"ddd/0"},
		{"fff/0", "fff/1", "fff/2"},
		{"ggg/0", "iii/9"},
		{"kkk/0", "kkk/1", "kkk/2"},
		{"mmm/0", "mmm/1"},
		{"mmm/1", "mmm/2"},
		{"xxx/0", "xxx/1"},
		{"yyy/0", "yyy/0"},
	}
	p, counter, allItems := newTestPartFromBlocks(t, blocks)
	defer p.MustClose()

	if len(p.mrs) != 1 {
		t.Fatalf("expected single metaindexRow; got %d", len(p.mrs))
	}

	f := func(key, wantFirst string, wantReads int) {
		t.Helper()
		ibCache.RemoveBlocksForPart(p)
		ibSparseCache.RemoveBlocksForPart(p)
		counter.reset()

		var ps partSearch
		ps.Init(p, true)
		ps.Seek([]byte(key))
		if len(counter.offsets) != wantReads {
			t.Fatalf("Seek(%q): unexpected block reads count; got %d (offsets=%v); want %d",
				key, len(counter.offsets), counter.offsets, wantReads)
		}

		// Verify full iteration from Seek(key) matches ground-truth sorted items.
		wantIdx := sort.Search(len(allItems), func(i int) bool {
			return key <= allItems[i]
		})
		if wantIdx == len(allItems) {
			if wantFirst != "" {
				t.Fatalf("Seek(%q): test bug: expected no item, wantFirst=%q", key, wantFirst)
			}
			if ps.NextItem() {
				t.Fatalf("Seek(%q): unexpected item %q; want EOF", key, ps.Item)
			}
			if err := ps.Error(); err != nil {
				t.Fatalf("Seek(%q): unexpected error: %s", key, err)
			}
			return
		}
		if allItems[wantIdx] != wantFirst {
			t.Fatalf("Seek(%q): test bug: ground truth first item %q != wantFirst %q", key, allItems[wantIdx], wantFirst)
		}
		for i := wantIdx; i < len(allItems); i++ {
			if !ps.NextItem() {
				t.Fatalf("Seek(%q): missing item %q at position %d", key, allItems[i], i)
			}
			if string(ps.Item) != allItems[i] {
				t.Fatalf("Seek(%q): unexpected item at position %d; got %q; want %q", key, i, ps.Item, allItems[i])
			}
		}
		if ps.NextItem() {
			t.Fatalf("Seek(%q): unexpected trailing item %q", key, ps.Item)
		}
		if err := ps.Error(); err != nil {
			t.Fatalf("Seek(%q): unexpected error: %s", key, err)
		}
	}

	// Non-matching prefix between block 0 ("aaa/") and block 1 ("ccc/"):
	// block 0 must be skipped, reading only block 1.
	f("bbb/0", "ccc/0", 1)

	// Exact match on block 1's firstItem ("ccc/0") when block 0 has smaller commonPrefix ("aaa/"):
	// block 0 must be skipped, reading only block 1.
	f("ccc/0", "ccc/0", 1)

	// Key shorter than block 0's commonPrefix ("aaa/") and strictly greater ("ab" > "aaa/"):
	// block 0 must be skipped, reading only block 1.
	f("ab", "ccc/0", 1)

	// Key sharing commonPrefix with block 0 ("aaa/"):
	// must read block 0 (1 read when found inside block 0, 2 reads when past last item in block 0).
	f("aaa/1", "aaa/1", 1)
	f("aaa/9", "ccc/0", 2)

	// Previous block is a single-item block ("ddd/0"): must skip it and read only block 3 ("fff/").
	f("eee/0", "fff/0", 1)
	f("fff/0", "fff/0", 1)

	// Previous block has empty commonPrefix (["ggg/0", "iii/9"]): must NOT skip it.
	f("hhh/0", "iii/9", 1)
	f("jjj/0", "kkk/0", 2)

	// Duplicate item across block boundary with matching commonPrefix ("mmm/"):
	// must NOT skip block 6, so both "mmm/1" items are returned.
	f("mmm/1", "mmm/1", 1)

	// Duplicate item in next block with disjoint commonPrefix ("xxx/" vs "yyy/"):
	// block 8 ("xxx/") must be skipped, returning both "yyy/0" items with 1 read.
	f("yyy/0", "yyy/0", 1)

	// Verify single-item block ("ddd/0") is skipped without decoding into ps.tmpIB
	// when the target block ("fff/") is already cached.
	ibCache.RemoveBlocksForPart(p)
	var psWarm partSearch
	for range 4 {
		psWarm.Init(p, false)
		psWarm.Seek([]byte("fff/1"))
	}
	counter.reset()
	var psSingle partSearch
	psSingle.Init(p, false)
	psSingle.Seek([]byte("eee/0"))
	if len(counter.offsets) != 0 {
		t.Fatalf("expected cached block read for fff/; got %d disk reads", len(counter.offsets))
	}
	if len(psSingle.tmpIB.items) != 0 {
		t.Fatalf("single-item block ddd/0 was unnecessarily decoded into tmpIB (%d items)", len(psSingle.tmpIB.items))
	}
	if !psSingle.NextItem() || string(psSingle.Item) != "fff/0" {
		t.Fatalf("unexpected item after skipping single-item block; got %q; want %q", psSingle.Item, "fff/0")
	}
}

func TestPartSearchSkipNonMatchingBlocksAcrossMetaindexRows(t *testing.T) {
	// Create enough blocks with ~500-byte keys so blockStreamWriter flushes multiple metaindexRows
	// (maxIndexBlockSize is 64KB, so ~150 block headers with 500-byte firstItem + commonPrefix exceed 64KB).
	const blocksCount = 200
	pad := strings.Repeat("p", 400)
	blocks := make([][]string, 0, blocksCount)
	for i := range blocksCount {
		// Use even prefix indices ("b0000_", "b0002_", ...) so odd keys fall strictly between blocks.
		prefix := fmt.Sprintf("b%04d_%s/", i*2, pad)
		blocks = append(blocks, []string{prefix + "0", prefix + "1"})
	}
	p, counter, allItems := newTestPartFromBlocks(t, blocks)
	defer p.MustClose()

	if len(p.mrs) < 2 {
		t.Fatalf("expected at least 2 metaindexRows; got %d", len(p.mrs))
	}

	// Find the boundary between metaindexRow 0 and metaindexRow 1.
	boundaryBlockIdx := int(p.mrs[0].blockHeadersCount)
	lastPrefixMR0 := fmt.Sprintf("b%04d_%s/", (boundaryBlockIdx-1)*2, pad)
	gapKeyBetweenMRs := fmt.Sprintf("b%04d_%s/0", (boundaryBlockIdx-1)*2+1, pad)
	firstItemMR1 := string(p.mrs[1].firstItem)

	f := func(key, wantFirst string, wantReads int) {
		t.Helper()
		ibCache.RemoveBlocksForPart(p)
		ibSparseCache.RemoveBlocksForPart(p)
		counter.reset()

		var ps partSearch
		ps.Init(p, true)
		ps.Seek([]byte(key))
		if len(counter.offsets) != wantReads {
			t.Fatalf("Seek(%q): unexpected block reads count; got %d; want %d",
				key[:16], len(counter.offsets), wantReads)
		}
		if !ps.NextItem() {
			t.Fatalf("Seek(%q): missing expected item %q (err=%s)", key[:16], wantFirst[:16], ps.Error())
		}
		if string(ps.Item) != wantFirst {
			t.Fatalf("Seek(%q): unexpected first item; got %q; want %q", key[:16], ps.Item[:min(len(ps.Item), 16)], wantFirst[:16])
		}
		// Verify next item also advances cleanly across the metaindexRow boundary.
		wantIdx := sort.Search(len(allItems), func(i int) bool {
			return key <= allItems[i]
		})
		if wantIdx+1 < len(allItems) {
			if !ps.NextItem() {
				t.Fatalf("Seek(%q): missing second item", key[:16])
			}
			if string(ps.Item) != allItems[wantIdx+1] {
				t.Fatalf("Seek(%q): unexpected second item; got %q; want %q", key[:16], ps.Item[:min(len(ps.Item), 16)], allItems[wantIdx+1][:16])
			}
		}
	}

	// Gap key between last block of mrs[0] and first block of mrs[1]:
	// must skip last block of mrs[0] and read only first block of mrs[1].
	f(gapKeyBetweenMRs, firstItemMR1, 1)

	// Exact match on first item of mrs[1]:
	// must skip last block of mrs[0] and read only first block of mrs[1].
	f(firstItemMR1, firstItemMR1, 1)

	// Key sharing commonPrefix with last block of mrs[0] but greater than its last item:
	// must read last block of mrs[0], find nothing, then read first block of mrs[1] (2 reads).
	f(lastPrefixMR0+"9", firstItemMR1, 2)

	// Exhaustive seek verification across all blocks and metaindexRow boundaries
	// in ascending, descending, and gap-key orders (also exercising tryFastSeek).
	var ps partSearch
	ps.Init(p, true)
	verifySeek := func(key string) {
		t.Helper()
		ps.Seek([]byte(key))
		wantIdx := sort.Search(len(allItems), func(i int) bool {
			return key <= allItems[i]
		})
		if wantIdx == len(allItems) {
			if ps.NextItem() {
				t.Fatalf("Seek(%q): unexpected item %q; want EOF", key[:min(len(key), 16)], ps.Item[:min(len(ps.Item), 16)])
			}
			if err := ps.Error(); err != nil {
				t.Fatalf("Seek(%q): unexpected error: %s", key[:min(len(key), 16)], err)
			}
			return
		}
		if !ps.NextItem() {
			t.Fatalf("Seek(%q): missing expected item (err=%s)", key[:min(len(key), 16)], ps.Error())
		}
		if string(ps.Item) != allItems[wantIdx] {
			t.Fatalf("Seek(%q): got %q; want %q", key[:min(len(key), 16)], ps.Item[:min(len(ps.Item), 16)], allItems[wantIdx][:min(len(allItems[wantIdx]), 16)])
		}
	}
	for i := range blocksCount {
		// Exact first item, exact second item, prefix-matching past-end key, and disjoint gap key.
		prefix := fmt.Sprintf("b%04d_%s/", i*2, pad)
		verifySeek(prefix + "0")
		verifySeek(prefix + "1")
		verifySeek(prefix + "2")
		verifySeek(fmt.Sprintf("b%04d_%s/0", i*2+1, pad))
	}
	for i := blocksCount - 1; i >= 0; i-- {
		verifySeek(fmt.Sprintf("b%04d_%s/0", i*2+1, pad))
		verifySeek(fmt.Sprintf("b%04d_%s/0", i*2, pad))
	}
}

func TestCanBlockContainItemGE(t *testing.T) {
	f := func(itemsCount uint32, commonPrefix, k string, want bool) {
		t.Helper()
		bh := &blockHeader{
			itemsCount:   itemsCount,
			commonPrefix: []byte(commonPrefix),
		}
		got := canBlockContainItemGE(bh, []byte(k))
		if got != want {
			t.Fatalf("canBlockContainItemGE(itemsCount=%d, cp=%q, k=%q) = %v; want %v",
				itemsCount, commonPrefix, k, got, want)
		}
	}

	// Single-item block only has firstItem (< k), so it can always be skipped.
	f(1, "", "b", false)
	f(1, "a", "a1", false)

	// Empty commonPrefix on multi-item block: cannot prove upper bound.
	f(2, "", "", true)
	f(2, "", "zzz", true)

	// Key starts with commonPrefix: block may contain items >= k.
	f(2, "abc", "abc", true)
	f(2, "abc", "abcd", true)
	f(2, "abc", "abc\xff", true)

	// Key strictly greater than commonPrefix (shorter, equal, and longer):
	// every item starting with commonPrefix is strictly less than k.
	f(2, "abc", "b", false)
	f(2, "abc", "ac", false)
	f(2, "abc", "abd", false)
	f(2, "abc", "abde", false)
	f(2, "abc\x00", "abc\x01", false)

	// Key smaller than commonPrefix (including proper prefix of commonPrefix):
	// never skip (also safe on corrupted headers where commonPrefix > firstItem).
	f(2, "abc", "", true)
	f(2, "abc", "a", true)
	f(2, "abc", "ab", true)
	f(2, "abc", "abb", true)
	f(2, "abc", "aaaa", true)
}
