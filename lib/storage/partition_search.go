package storage

import (
	"container/heap"
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// partitionSearch represents a search in the partition.
type partitionSearch struct {
	// BlockRef is the block found after NextBlock call.
	BlockRef *BlockRef

	// pt is a partition to search.
	pt *partition

	// pws hold parts snapshot for the given partition during Init call.
	// This snapshot is used for calling Part.PutParts on partitionSearch.MustClose.
	pws []*partWrapper

	psPool []partSearch
	psHeap partSearchHeap

	err error

	nextBlockNoop bool
	needClosing   bool
}

func (pts *partitionSearch) reset() {
	pts.BlockRef = nil
	pts.pt = nil

	for i := range pts.pws {
		pts.pws[i] = nil
	}
	pts.pws = pts.pws[:0]

	for i := range pts.psPool {
		pts.psPool[i].reset()
	}
	pts.psPool = pts.psPool[:0]

	for i := range pts.psHeap {
		pts.psHeap[i] = nil
	}
	pts.psHeap = pts.psHeap[:0]

	pts.err = nil
	pts.nextBlockNoop = false
	pts.needClosing = false
}

// Init initializes the search in the given partition for the given tsid and tr.
//
// tsids must be sorted.
// tsids cannot be modified after the Init call, since it is owned by pts.
//
// MustClose must be called when partition search is done.
func (pts *partitionSearch) Init(pt *partition, tsids []TSID, tr TimeRange) {
	if pts.needClosing {
		logger.Panicf("BUG: missing partitionSearch.MustClose call before the next call to Init")
	}

	pts.reset()
	pts.pt = pt
	pts.needClosing = true

	if len(tsids) == 0 {
		// Fast path - zero tsids.
		pts.err = io.EOF
		return
	}

	if pt.tr.MinTimestamp > tr.MaxTimestamp || pt.tr.MaxTimestamp < tr.MinTimestamp {
		// Fast path - the partition doesn't contain rows for the given time range.
		pts.err = io.EOF
		return
	}

	pts.pws = pt.GetParts(pts.pws[:0], true)

	// Initialize psPool.
	pts.psPool = slicesutil.SetLength(pts.psPool, len(pts.pws))
	for i, pw := range pts.pws {
		pts.psPool[i].Init(pw.p, tsids, tr)
	}

	// Initialize the psHeap.
	pts.psHeap = pts.psHeap[:0]
	for i := range pts.psPool {
		ps := &pts.psPool[i]
		if !ps.NextBlock() {
			if err := ps.Error(); err != nil {
				// Return only the first error, since it has no sense in returning all errors.
				pts.err = fmt.Errorf("cannot initialize partition search: %w", err)
				return
			}
			continue
		}
		pts.psHeap = append(pts.psHeap, ps)
	}
	if len(pts.psHeap) == 0 {
		pts.err = io.EOF
		return
	}
	heap.Init(&pts.psHeap)
	pts.BlockRef = &pts.psHeap[0].BlockRef
	pts.nextBlockNoop = true
}

// NextBlock advances to the next block.
//
// The blocks are sorted by (TDIS, MinTimestamp). Two subsequent blocks
// for the same TSID may contain overlapped time ranges.
func (pts *partitionSearch) NextBlock() bool {
	if pts.err != nil {
		return false
	}
	if pts.nextBlockNoop {
		pts.nextBlockNoop = false
		return true
	}

	pts.err = pts.nextBlock()
	if pts.err != nil {
		if pts.err != io.EOF {
			pts.err = fmt.Errorf("cannot obtain the next block to search in the partition: %w", pts.err)
		}
		return false
	}
	return true
}

func (pts *partitionSearch) nextBlock() error {
	psMin := pts.psHeap[0]
	if psMin.NextBlock() {
		heap.Fix(&pts.psHeap, 0)
		pts.BlockRef = &pts.psHeap[0].BlockRef
		return nil
	}

	if err := psMin.Error(); err != nil {
		return err
	}

	heap.Pop(&pts.psHeap)

	if len(pts.psHeap) == 0 {
		return io.EOF
	}

	pts.BlockRef = &pts.psHeap[0].BlockRef
	return nil
}

func (pts *partitionSearch) Error() error {
	if pts.err == io.EOF {
		return nil
	}
	return pts.err
}

// MustClose closes the pts.
func (pts *partitionSearch) MustClose() {
	if !pts.needClosing {
		logger.Panicf("BUG: missing Init call before the MustClose call")
	}

	pts.pt.PutParts(pts.pws)
	pts.reset()
}

type partSearchHeap []*partSearch

func (psh *partSearchHeap) Len() int {
	return len(*psh)
}

func (psh *partSearchHeap) Less(i, j int) bool {
	x := *psh
	return x[i].BlockRef.bh.Less(&x[j].BlockRef.bh)
}

func (psh *partSearchHeap) Swap(i, j int) {
	x := *psh
	x[i], x[j] = x[j], x[i]
}

func (psh *partSearchHeap) Push(x any) {
	*psh = append(*psh, x.(*partSearch))
}

func (psh *partSearchHeap) Pop() any {
	a := *psh
	v := a[len(a)-1]
	*psh = a[:len(a)-1]
	return v
}
