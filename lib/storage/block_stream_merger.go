package storage

import (
	"container/heap"
	"fmt"
	"io"
)

// blockStreamMerger is used for merging block streams.
type blockStreamMerger struct {
	// The current block to work with.
	Block *Block

	bsrHeap blockStreamReaderHeap

	// Whether the call to NextBlock must be no-op.
	nextBlockNoop bool

	// The last error
	err error
}

func (bsm *blockStreamMerger) reset() {
	bsm.Block = nil
	for i := range bsm.bsrHeap {
		bsm.bsrHeap[i] = nil
	}
	bsm.bsrHeap = bsm.bsrHeap[:0]
	bsm.nextBlockNoop = false
	bsm.err = nil
}

// Init initializes bsm with the given bsrs.
func (bsm *blockStreamMerger) Init(bsrs []*blockStreamReader) {
	bsm.reset()
	for _, bsr := range bsrs {
		if bsr.NextBlock() {
			bsm.bsrHeap = append(bsm.bsrHeap, bsr)
			continue
		}
		if err := bsr.Error(); err != nil {
			bsm.err = fmt.Errorf("cannot obtain the next block to merge: %w", err)
			return
		}
	}

	if len(bsm.bsrHeap) == 0 {
		bsm.err = io.EOF
		return
	}

	heap.Init(&bsm.bsrHeap)
	bsm.Block = &bsm.bsrHeap[0].Block
	bsm.nextBlockNoop = true
}

// NextBlock stores the next block in bsm.Block.
//
// The blocks are sorted by (TDIS, MinTimestamp). Two subsequent blocks
// for the same TSID may contain overlapped time ranges.
func (bsm *blockStreamMerger) NextBlock() bool {
	if bsm.err != nil {
		return false
	}
	if bsm.nextBlockNoop {
		bsm.nextBlockNoop = false
		return true
	}

	bsm.err = bsm.nextBlock()
	switch bsm.err {
	case nil:
		return true
	case io.EOF:
		return false
	default:
		bsm.err = fmt.Errorf("cannot obtain the next block to merge: %w", bsm.err)
		return false
	}
}

func (bsm *blockStreamMerger) nextBlock() error {
	bsrMin := bsm.bsrHeap[0]
	if bsrMin.NextBlock() {
		heap.Fix(&bsm.bsrHeap, 0)
		bsm.Block = &bsm.bsrHeap[0].Block
		return nil
	}

	if err := bsrMin.Error(); err != nil {
		return err
	}

	heap.Pop(&bsm.bsrHeap)

	if len(bsm.bsrHeap) == 0 {
		return io.EOF
	}

	bsm.Block = &bsm.bsrHeap[0].Block
	return nil
}

func (bsm *blockStreamMerger) Error() error {
	if bsm.err == io.EOF {
		return nil
	}
	return bsm.err
}

type blockStreamReaderHeap []*blockStreamReader

func (bsrh *blockStreamReaderHeap) Len() int {
	return len(*bsrh)
}

func (bsrh *blockStreamReaderHeap) Less(i, j int) bool {
	x := *bsrh
	a, b := &x[i].Block.bh, &x[j].Block.bh
	if a.TSID.MetricID == b.TSID.MetricID {
		// Fast path for identical TSID values.
		return a.MinTimestamp < b.MinTimestamp
	}
	// Slow path for distinct TSID values.
	return a.TSID.Less(&b.TSID)
}

func (bsrh *blockStreamReaderHeap) Swap(i, j int) {
	x := *bsrh
	x[i], x[j] = x[j], x[i]
}

func (bsrh *blockStreamReaderHeap) Push(x interface{}) {
	*bsrh = append(*bsrh, x.(*blockStreamReader))
}

func (bsrh *blockStreamReaderHeap) Pop() interface{} {
	a := *bsrh
	v := a[len(a)-1]
	*bsrh = a[:len(a)-1]
	return v
}
