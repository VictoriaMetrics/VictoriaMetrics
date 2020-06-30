package mergeset

import (
	"container/heap"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// PrepareBlockCallback can transform the passed items allocated at the given data.
//
// The callback is called during merge before flushing full block of the given items
// to persistent storage.
//
// The callback must return sorted items. The first and the last item must be unchanged.
// The callback can re-use data and items for storing the result.
type PrepareBlockCallback func(data []byte, items [][]byte) ([]byte, [][]byte)

// mergeBlockStreams merges bsrs and writes result to bsw.
//
// It also fills ph.
//
// prepareBlock is optional.
//
// The function immediately returns when stopCh is closed.
//
// It also atomically adds the number of items merged to itemsMerged.
func mergeBlockStreams(ph *partHeader, bsw *blockStreamWriter, bsrs []*blockStreamReader, prepareBlock PrepareBlockCallback, stopCh <-chan struct{}, itemsMerged *uint64) error {
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
	if err == errForciblyStopped {
		return err
	}
	return fmt.Errorf("cannot merge %d block streams: %s: %w", len(bsrs), bsrs, err)
}

var bsmPool = &sync.Pool{
	New: func() interface{} {
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

func (bsm *blockStreamMerger) Merge(bsw *blockStreamWriter, ph *partHeader, stopCh <-chan struct{}, itemsMerged *uint64) error {
again:
	if len(bsm.bsrHeap) == 0 {
		// Write the last (maybe incomplete) inmemoryBlock to bsw.
		bsm.flushIB(bsw, ph, itemsMerged)
		return nil
	}

	select {
	case <-stopCh:
		return errForciblyStopped
	default:
	}

	bsr := heap.Pop(&bsm.bsrHeap).(*blockStreamReader)

	var nextItem []byte
	hasNextItem := false
	if len(bsm.bsrHeap) > 0 {
		nextItem = bsm.bsrHeap[0].bh.firstItem
		hasNextItem = true
	}
	for bsr.blockItemIdx < len(bsr.Block.items) {
		item := bsr.Block.items[bsr.blockItemIdx]
		if hasNextItem && string(item) > string(nextItem) {
			break
		}
		if !bsm.ib.Add(item) {
			// The bsm.ib is full. Flush it to bsw and continue.
			bsm.flushIB(bsw, ph, itemsMerged)
			continue
		}
		bsr.blockItemIdx++
	}
	if bsr.blockItemIdx == len(bsr.Block.items) {
		// bsr.Block is fully read. Proceed to the next block.
		if bsr.Next() {
			heap.Push(&bsm.bsrHeap, bsr)
			goto again
		}
		if err := bsr.Error(); err != nil {
			return fmt.Errorf("cannot read storageBlock: %w", err)
		}
		goto again
	}

	// The next item in the bsr.Block exceeds nextItem.
	// Adjust bsr.bh.firstItem and return bsr to heap.
	bsr.bh.firstItem = append(bsr.bh.firstItem[:0], bsr.Block.items[bsr.blockItemIdx]...)
	heap.Push(&bsm.bsrHeap, bsr)
	goto again
}

func (bsm *blockStreamMerger) flushIB(bsw *blockStreamWriter, ph *partHeader, itemsMerged *uint64) {
	if len(bsm.ib.items) == 0 {
		// Nothing to flush.
		return
	}
	atomic.AddUint64(itemsMerged, uint64(len(bsm.ib.items)))
	if bsm.prepareBlock != nil {
		bsm.firstItem = append(bsm.firstItem[:0], bsm.ib.items[0]...)
		bsm.lastItem = append(bsm.lastItem[:0], bsm.ib.items[len(bsm.ib.items)-1]...)
		bsm.ib.data, bsm.ib.items = bsm.prepareBlock(bsm.ib.data, bsm.ib.items)
		if len(bsm.ib.items) == 0 {
			// Nothing to flush
			return
		}
		// Consistency checks after prepareBlock call.
		firstItem := bsm.ib.items[0]
		if string(firstItem) != string(bsm.firstItem) {
			logger.Panicf("BUG: prepareBlock must return first item equal to the original first item;\ngot\n%X\nwant\n%X", firstItem, bsm.firstItem)
		}
		lastItem := bsm.ib.items[len(bsm.ib.items)-1]
		if string(lastItem) != string(bsm.lastItem) {
			logger.Panicf("BUG: prepareBlock must return last item equal to the original last item;\ngot\n%X\nwant\n%X", lastItem, bsm.lastItem)
		}
		// Verify whether the bsm.ib.items are sorted only in tests, since this
		// can be expensive check in prod for items with long common prefix.
		if isInTest && !bsm.ib.isSorted() {
			logger.Panicf("BUG: prepareBlock must return sorted items;\ngot\n%s", bsm.ib.debugItemsString())
		}
	}
	ph.itemsCount += uint64(len(bsm.ib.items))
	if !bsm.phFirstItemCaught {
		ph.firstItem = append(ph.firstItem[:0], bsm.ib.items[0]...)
		bsm.phFirstItemCaught = true
	}
	ph.lastItem = append(ph.lastItem[:0], bsm.ib.items[len(bsm.ib.items)-1]...)
	bsw.WriteBlock(&bsm.ib)
	bsm.ib.Reset()
	ph.blocksCount++
}

type bsrHeap []*blockStreamReader

func (bh *bsrHeap) Len() int {
	return len(*bh)
}

func (bh *bsrHeap) Swap(i, j int) {
	x := *bh
	x[i], x[j] = x[j], x[i]
}

func (bh *bsrHeap) Less(i, j int) bool {
	x := *bh
	return string(x[i].bh.firstItem) < string(x[j].bh.firstItem)
}

func (bh *bsrHeap) Pop() interface{} {
	a := *bh
	v := a[len(a)-1]
	*bh = a[:len(a)-1]
	return v
}

func (bh *bsrHeap) Push(x interface{}) {
	v := x.(*blockStreamReader)
	*bh = append(*bh, v)
}
