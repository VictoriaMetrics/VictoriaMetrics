package mergeset

import (
	"container/heap"
	"fmt"
	"sync"
	"sync/atomic"
)

// mergeBlockStreams merges bsrs and writes result to bsw.
//
// It also fills ph.
//
// The function immediately returns when stopCh is closed.
//
// It also atomically adds the number of items merged to itemsMerged.
func mergeBlockStreams(ph *partHeader, bsw *blockStreamWriter, bsrs []*blockStreamReader, stopCh <-chan struct{}, itemsMerged *uint64) error {
	bsm := bsmPool.Get().(*blockStreamMerger)
	if err := bsm.Init(bsrs); err != nil {
		return fmt.Errorf("cannot initialize blockStreamMerger: %s", err)
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
	return fmt.Errorf("cannot merge %d block streams: %s: %s", len(bsrs), bsrs, err)
}

var bsmPool = &sync.Pool{
	New: func() interface{} {
		return &blockStreamMerger{}
	},
}

type blockStreamMerger struct {
	bsrHeap bsrHeap

	// ib is a scratch block with pending items.
	ib inmemoryBlock

	phFirstItemCaught bool
}

func (bsm *blockStreamMerger) reset() {
	for i := range bsm.bsrHeap {
		bsm.bsrHeap[i] = nil
	}
	bsm.bsrHeap = bsm.bsrHeap[:0]
	bsm.ib.Reset()

	bsm.phFirstItemCaught = false
}

func (bsm *blockStreamMerger) Init(bsrs []*blockStreamReader) error {
	bsm.reset()
	for _, bsr := range bsrs {
		if bsr.Next() {
			bsm.bsrHeap = append(bsm.bsrHeap, bsr)
		}

		if err := bsr.Error(); err != nil {
			return fmt.Errorf("cannot obtain the next block from blockStreamReader %q: %s", bsr.path, err)
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

	if !bsm.phFirstItemCaught {
		ph.firstItem = append(ph.firstItem[:0], bsr.Block.items[0]...)
		bsm.phFirstItemCaught = true
	}

	var nextItem []byte
	hasNextItem := false
	if len(bsm.bsrHeap) > 0 {
		nextItem = bsm.bsrHeap[0].bh.firstItem
		hasNextItem = true
	}
	for bsr.blockItemIdx < len(bsr.Block.items) && (!hasNextItem || string(bsr.Block.items[bsr.blockItemIdx]) <= string(nextItem)) {
		if bsm.ib.Add(bsr.Block.items[bsr.blockItemIdx]) {
			bsr.blockItemIdx++
			continue
		}

		// The bsm.ib is full. Flush it to bsw and continue.
		bsm.flushIB(bsw, ph, itemsMerged)
	}
	if bsr.blockItemIdx == len(bsr.Block.items) {
		// bsr.Block is fully read. Proceed to the next block.
		if bsr.Next() {
			heap.Push(&bsm.bsrHeap, bsr)
			goto again
		}
		if err := bsr.Error(); err != nil {
			return fmt.Errorf("cannot read storageBlock: %s", err)
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
	itemsCount := uint64(len(bsm.ib.items))
	ph.itemsCount += itemsCount
	atomic.AddUint64(itemsMerged, itemsCount)
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
