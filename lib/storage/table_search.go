package storage

import (
	"container/heap"
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// tableSearch performs searches in the table.
type tableSearch struct {
	BlockRef *BlockRef

	tb *table

	// ptws hold paritions snapshot for the given table during Init call.
	// This snapshot is used for calling table.PutPartitions on tableSearch.MustClose.
	ptws []*partitionWrapper

	ptsPool []partitionSearch
	ptsHeap partitionSearchHeap

	err error

	nextBlockNoop bool
	needClosing   bool
}

func (ts *tableSearch) reset() {
	ts.BlockRef = nil
	ts.tb = nil

	for i := range ts.ptws {
		ts.ptws[i] = nil
	}
	ts.ptws = ts.ptws[:0]

	for i := range ts.ptsPool {
		ts.ptsPool[i].reset()
	}
	ts.ptsPool = ts.ptsPool[:0]

	for i := range ts.ptsHeap {
		ts.ptsHeap[i] = nil
	}
	ts.ptsHeap = ts.ptsHeap[:0]

	ts.err = nil
	ts.nextBlockNoop = false
	ts.needClosing = false
}

// Init initializes the ts.
//
// tsids must be sorted.
// tsids cannot be modified after the Init call, since it is owned by ts.
//
// MustClose must be called then the tableSearch is done.
func (ts *tableSearch) Init(tb *table, tsids []TSID, tr TimeRange) {
	if ts.needClosing {
		logger.Panicf("BUG: missing MustClose call before the next call to Init")
	}

	// Adjust tr.MinTimestamp, so it doesn't obtain data older
	// than the tb retention.
	now := int64(fasttime.UnixTimestamp() * 1000)
	minTimestamp := now - tb.retentionMilliseconds
	if tr.MinTimestamp < minTimestamp {
		tr.MinTimestamp = minTimestamp
	}

	ts.reset()
	ts.tb = tb
	ts.needClosing = true

	if len(tsids) == 0 {
		// Fast path - zero tsids.
		ts.err = io.EOF
		return
	}

	ts.ptws = tb.GetPartitions(ts.ptws[:0])

	// Initialize the ptsPool.
	if n := len(ts.ptws) - cap(ts.ptsPool); n > 0 {
		ts.ptsPool = append(ts.ptsPool[:cap(ts.ptsPool)], make([]partitionSearch, n)...)
	}
	ts.ptsPool = ts.ptsPool[:len(ts.ptws)]
	for i, ptw := range ts.ptws {
		ts.ptsPool[i].Init(ptw.pt, tsids, tr)
	}

	// Initialize the ptsHeap.
	var errors []error
	ts.ptsHeap = ts.ptsHeap[:0]
	for i := range ts.ptsPool {
		pts := &ts.ptsPool[i]
		if !pts.NextBlock() {
			if err := pts.Error(); err != nil {
				errors = append(errors, err)
			}
			continue
		}
		ts.ptsHeap = append(ts.ptsHeap, pts)
	}
	if len(errors) > 0 {
		// Return only the first error, since it has no sense in returning all errors.
		ts.err = fmt.Errorf("cannot initialize table search: %w", errors[0])
		return
	}
	if len(ts.ptsHeap) == 0 {
		ts.err = io.EOF
		return
	}
	heap.Init(&ts.ptsHeap)
	ts.BlockRef = ts.ptsHeap[0].BlockRef
	ts.nextBlockNoop = true
}

// NextBlock advances to the next block.
//
// The blocks are sorted by (TSID, MinTimestamp). Two subsequent blocks
// for the same TSID may contain overlapped time ranges.
func (ts *tableSearch) NextBlock() bool {
	if ts.err != nil {
		return false
	}
	if ts.nextBlockNoop {
		ts.nextBlockNoop = false
		return true
	}

	ts.err = ts.nextBlock()
	if ts.err != nil {
		if ts.err != io.EOF {
			ts.err = fmt.Errorf("cannot obtain the next block to search in the table: %w", ts.err)
		}
		return false
	}
	return true
}

func (ts *tableSearch) nextBlock() error {
	ptsMin := ts.ptsHeap[0]
	if ptsMin.NextBlock() {
		heap.Fix(&ts.ptsHeap, 0)
		ts.BlockRef = ts.ptsHeap[0].BlockRef
		return nil
	}

	if err := ptsMin.Error(); err != nil {
		return err
	}

	heap.Pop(&ts.ptsHeap)

	if len(ts.ptsHeap) == 0 {
		return io.EOF
	}

	ts.BlockRef = ts.ptsHeap[0].BlockRef
	return nil
}

// Error returns the last error in the ts.
func (ts *tableSearch) Error() error {
	if ts.err == io.EOF {
		return nil
	}
	return ts.err
}

// MustClose closes the ts.
func (ts *tableSearch) MustClose() {
	if !ts.needClosing {
		logger.Panicf("BUG: missing Init call before MustClose call")
	}
	for i := range ts.ptsPool {
		ts.ptsPool[i].MustClose()
	}
	ts.tb.PutPartitions(ts.ptws)
	ts.reset()
}

type partitionSearchHeap []*partitionSearch

func (ptsh *partitionSearchHeap) Len() int {
	return len(*ptsh)
}

func (ptsh *partitionSearchHeap) Less(i, j int) bool {
	x := *ptsh
	return x[i].BlockRef.bh.Less(&x[j].BlockRef.bh)
}

func (ptsh *partitionSearchHeap) Swap(i, j int) {
	x := *ptsh
	x[i], x[j] = x[j], x[i]
}

func (ptsh *partitionSearchHeap) Push(x interface{}) {
	*ptsh = append(*ptsh, x.(*partitionSearch))
}

func (ptsh *partitionSearchHeap) Pop() interface{} {
	a := *ptsh
	v := a[len(a)-1]
	*ptsh = a[:len(a)-1]
	return v
}
