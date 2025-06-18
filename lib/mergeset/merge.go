package mergeset

import (
	"container/heap"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// PrepareBlockCallback can transform the passed items allocated at the given data.
//
// The callback is called during merge before flushing full block of the given items
// to persistent storage.
//
// The callback must return sorted items. The first and the last item must be unchanged.
// The callback can reuse data and items for storing the result.
type PrepareBlockCallback func(data []byte, items []Item) ([]byte, []Item)

// mergeBlockStreams merges bsrs and writes result to bsw.
//
// It also fills ph.
//
// prepareBlock is optional.
//
// The function immediately returns when stopCh is closed.
//
// It also atomically adds the number of items merged to itemsMerged.
func mergeBlockStreams(ph *partHeader, bsw *blockStreamWriter, bsrs []*blockStreamReader, prepareBlock PrepareBlockCallback, stopCh <-chan struct{},
	itemsMerged *atomic.Uint64) error {
	bsm := bsmPool.Get().(*blockStreamMerger)
	if err := bsm.Init(bsrs, prepareBlock); err != nil {
		return fmt.Errorf("cannot initialize blockStreamMerger: %w", err)
	}
	err := bsm.Merge(bsw, ph, stopCh, itemsMerged)
	bsm.reset()
	bsmPool.Put(bsm)
	bsw.MustClose()
	if err == nil {
		return nil
	}
	return fmt.Errorf("cannot merge %d block streams: %s: %w", len(bsrs), bsrs, err)
}

var bsmPool = &sync.Pool{
	New: func() any {
		return &blockStreamMerger{}
	},
}

type blockStreamMerger struct {
	prepareBlock PrepareBlockCallback

	bsrHeap bsrHeap

	// ib is a scratch block with pending items.
	ib inmemoryBlock

	phFirstItemCaught bool

	// This are auxiliary buffers used in flushIB
	// for consistency checks after prepareBlock call.
	firstItem []byte
	lastItem  []byte
}

func (bsm *blockStreamMerger) reset() {
	bsm.prepareBlock = nil

	for i := range bsm.bsrHeap {
		bsm.bsrHeap[i] = nil
	}
	bsm.bsrHeap = bsm.bsrHeap[:0]
	bsm.ib.Reset()

	bsm.phFirstItemCaught = false
}

func (bsm *blockStreamMerger) Init(bsrs []*blockStreamReader, prepareBlock PrepareBlockCallback) error {
	bsm.reset()
	bsm.prepareBlock = prepareBlock
	for _, bsr := range bsrs {
		if bsr.Next() {
			bsm.bsrHeap = append(bsm.bsrHeap, bsr)
		}

		if err := bsr.Error(); err != nil {
			return fmt.Errorf("cannot obtain the next block from blockStreamReader %q: %w", bsr.path, err)
		}
	}
	heap.Init(&bsm.bsrHeap)

	if len(bsm.bsrHeap) == 0 {
		return fmt.Errorf("bsrHeap cannot be empty")
	}

	return nil
}

var errForciblyStopped = fmt.Errorf("forcibly stopped")

func (bsm *blockStreamMerger) Merge(bsw *blockStreamWriter, ph *partHeader, stopCh <-chan struct{}, itemsMerged *atomic.Uint64) error {
	// Use local variables for tracking the number of merged items
	// and periodically propagate the collected stats to the caller, so it could be reflected in the exposed metrics.
	//
	// This minimizes expensive updates of itemsMerged var from concurrently running goroutines,
	// and improves concurrent merge scalability on multi-CPU systems - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8682 .
	var updateStatsDeadline uint64
	var localItemsMerged uint64
	updateStats := func() {
		itemsMerged.Add(localItemsMerged)
		localItemsMerged = 0
	}
	defer updateStats()

again:
	ct := fasttime.UnixTimestamp()
	if ct > updateStatsDeadline {
		updateStats()
		// Update the external stats once per second
		updateStatsDeadline = ct + 1
	}

	if len(bsm.bsrHeap) == 0 {
		// Write the last (maybe incomplete) inmemoryBlock to bsw.
		bsm.flushIB(bsw, ph, &localItemsMerged)
		return nil
	}

	select {
	case <-stopCh:
		return errForciblyStopped
	default:
	}

	bsr := bsm.bsrHeap[0]

	var nextItem string
	hasNextItem := false
	if len(bsm.bsrHeap) > 1 {
		bsr := bsm.bsrHeap.getNextReader()
		nextItem = bsr.CurrItem()
		hasNextItem = true
	}
	items := bsr.Block.items
	data := bsr.Block.data
	compareEveryItem := true
	if bsr.currItemIdx < len(items) {
		// An optimization, which allows skipping costly comparison for every merged item in the loop below.
		// Thanks to @ahfuzhang for the suggestion at https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5651
		lastItem := items[len(items)-1].String(data)
		compareEveryItem = hasNextItem && lastItem > nextItem
	}
	for bsr.currItemIdx < len(items) {
		item := items[bsr.currItemIdx].Bytes(data)
		if compareEveryItem && string(item) > nextItem {
			break
		}
		if !bsm.ib.Add(item) {
			// The bsm.ib is full. Flush it to bsw and continue.
			bsm.flushIB(bsw, ph, &localItemsMerged)
			continue
		}
		bsr.currItemIdx++
	}
	if bsr.currItemIdx == len(items) {
		// bsr.Block is fully read. Proceed to the next block.
		if bsr.Next() {
			heap.Fix(&bsm.bsrHeap, 0)
			goto again
		}
		if err := bsr.Error(); err != nil {
			return fmt.Errorf("cannot read storageBlock: %w", err)
		}
		heap.Pop(&bsm.bsrHeap)
		goto again
	}

	// The next item in the bsr.Block exceeds nextItem.
	// Return bsr to heap.
	heap.Fix(&bsm.bsrHeap, 0)
	goto again
}

func (bsm *blockStreamMerger) flushIB(bsw *blockStreamWriter, ph *partHeader, itemsMerged *uint64) {
	items := bsm.ib.items
	data := bsm.ib.data
	if len(items) == 0 {
		// Nothing to flush.
		return
	}
	*itemsMerged += uint64(len(items))
	if bsm.prepareBlock != nil {
		bsm.firstItem = append(bsm.firstItem[:0], items[0].String(data)...)
		bsm.lastItem = append(bsm.lastItem[:0], items[len(items)-1].String(data)...)
		data, items = bsm.prepareBlock(data, items)
		bsm.ib.data = data
		bsm.ib.items = items
		if len(items) == 0 {
			// Nothing to flush
			return
		}
		// Consistency checks after prepareBlock call.
		firstItem := items[0].String(data)
		if firstItem < string(bsm.firstItem) {
			logger.Panicf("BUG: prepareBlock must return the first item bigger or equal to the original first item;\ngot\n%X\nwant\n%X", firstItem, bsm.firstItem)
		}
		lastItem := items[len(items)-1].String(data)
		if lastItem > string(bsm.lastItem) {
			logger.Panicf("BUG: prepareBlock must return the last item smaller or equal to the original last item;\ngot\n%X\nwant\n%X", lastItem, bsm.lastItem)
		}
		// Verify whether the bsm.ib.items are sorted only in tests, since this
		// can be expensive check in prod for items with long common prefix.
		if isInTest && !bsm.ib.isSorted() {
			logger.Panicf("BUG: prepareBlock must return sorted items;\ngot\n%s", bsm.ib.debugItemsString())
		}
	}
	ph.itemsCount += uint64(len(items))
	if !bsm.phFirstItemCaught {
		ph.firstItem = append(ph.firstItem[:0], items[0].String(data)...)
		bsm.phFirstItemCaught = true
	}
	ph.lastItem = append(ph.lastItem[:0], items[len(items)-1].String(data)...)
	bsw.WriteBlock(&bsm.ib)
	bsm.ib.Reset()
	ph.blocksCount++
}

type bsrHeap []*blockStreamReader

func (bh bsrHeap) getNextReader() *blockStreamReader {
	if len(bh) < 2 {
		return nil
	}
	if len(bh) < 3 {
		return bh[1]
	}
	a := bh[1]
	b := bh[2]
	if a.CurrItem() <= b.CurrItem() {
		return a
	}
	return b
}

func (bh *bsrHeap) Len() int {
	return len(*bh)
}

func (bh *bsrHeap) Swap(i, j int) {
	x := *bh
	x[i], x[j] = x[j], x[i]
}

func (bh *bsrHeap) Less(i, j int) bool {
	x := *bh
	return x[i].CurrItem() < x[j].CurrItem()
}

func (bh *bsrHeap) Pop() any {
	a := *bh
	v := a[len(a)-1]
	*bh = a[:len(a)-1]
	return v
}

func (bh *bsrHeap) Push(x any) {
	v := x.(*blockStreamReader)
	*bh = append(*bh, v)
}
