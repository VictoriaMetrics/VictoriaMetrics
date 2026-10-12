package mergeset

import (
	"bytes"
	"container/heap"
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// TableSearch is a reusable cursor used for searching in the Table.
type TableSearch struct {
	// Item contains the next item after successful NextItem
	// or FirstItemWithPrefix call.
	//
	// Item contents breaks after the next call to NextItem.
	Item []byte

	tb *Table

	pws []*partWrapper

	psPool []partSearch
	psHeap partSearchHeap

	err error

	nextItemNoop bool
	needClosing  bool
}

func (ts *TableSearch) reset() {
	ts.Item = nil
	ts.tb = nil

	for i := range ts.pws {
		ts.pws[i] = nil
	}
	ts.pws = ts.pws[:0]

	for i := range ts.psPool {
		ts.psPool[i].reset()
	}
	ts.psPool = ts.psPool[:0]

	for i := range ts.psHeap {
		ts.psHeap[i] = nil
	}
	ts.psHeap = ts.psHeap[:0]

	ts.err = nil

	ts.nextItemNoop = false
	ts.needClosing = false
}

// Init initializes ts for searching in the tb.
//
// MustClose must be called when the ts is no longer needed.
func (ts *TableSearch) Init(tb *Table, sparse bool) {
	if ts.needClosing {
		logger.Panicf("BUG: missing MustClose call before the next call to Init")
	}

	ts.reset()

	ts.tb = tb
	ts.needClosing = true

	ts.pws = ts.tb.getParts(ts.pws[:0])

	// Initialize the psPool.
	ts.psPool = slicesutil.SetLength(ts.psPool, len(ts.pws))
	for i, pw := range ts.pws {
		ts.psPool[i].Init(pw.p, sparse)
	}
}

// Seek seeks for the first item greater or equal to k in the ts.
func (ts *TableSearch) Seek(k []byte) {
	ts.seek(k, false)
}

// PartFilterSearchStats describes the per-part filter decisions made by
// SeekWithPartFilter.
type PartFilterSearchStats struct {
	SkippedParts    uint64
	PossibleParts   uint64
	UnfilteredParts uint64

	BudgetFallbackParts uint64
	ReadFallbackParts   uint64
	VerifyFallbackParts uint64
}

// HasFallbacks reports whether at least one part couldn't be filtered, so it
// must be searched.
func (stats *PartFilterSearchStats) HasFallbacks() bool {
	return stats.UnfilteredParts > 0 || stats.BudgetFallbackParts > 0 ||
		stats.ReadFallbackParts > 0 || stats.VerifyFallbackParts > 0
}

// SeekWithPartFilter seeks in parts whose filter may contain k.
//
// Parts without a filter, parts with an unusable filter and parts which may
// contain k are searched, so the returned item set is the same as the one
// returned by Seek for the searched parts.
func (ts *TableSearch) SeekWithPartFilter(k []byte) PartFilterSearchStats {
	return ts.seek(k, true)
}

func (ts *TableSearch) seek(k []byte, usePartFilter bool) PartFilterSearchStats {
	var stats PartFilterSearchStats
	if err := ts.Error(); err != nil {
		// Do nothing on unrecoverable error.
		return stats
	}
	ts.err = nil

	// Initialize the psHeap.
	ts.psHeap = ts.psHeap[:0]
	var filterH, filterHash1, filterHash2 uint64
	if usePartFilter {
		// The hashes are calculated only once per SeekWithPartFilter call and
		// are re-used for all the parts.
		filterH, filterHash1, filterHash2 = partFilterHashes(k)
	}
	for i := range ts.psPool {
		ps := &ts.psPool[i]
		if usePartFilter {
			if bytes.Compare(k, ps.p.ph.lastItem) > 0 {
				// The key sorts after every item in the part, so this part cannot
				// contain the key and doesn't need a filter check or index seek.
				stats.SkippedParts++
				continue
			}
			switch ps.p.filterDecision(filterH, filterHash1, filterHash2) {
			case partFilterNegative:
				stats.SkippedParts++
				continue
			case partFilterPossible:
				stats.PossibleParts++
			case partFilterUnfiltered:
				stats.UnfilteredParts++
			case partFilterFallbackBudget:
				stats.BudgetFallbackParts++
			case partFilterFallbackRead:
				stats.ReadFallbackParts++
			case partFilterFallbackVerify:
				stats.VerifyFallbackParts++
			default:
				logger.Panicf("BUG: unexpected part filter decision for the part %q", ps.p.path)
			}
		}
		ps.Seek(k)
		if !ps.NextItem() {
			if err := ps.Error(); err != nil {
				// Return only the first error, since it has no sense in returning all errors.
				ts.err = fmt.Errorf("cannot seek %q: %w", k, err)
				return stats
			}
			continue
		}
		ts.psHeap = append(ts.psHeap, ps)
	}
	if len(ts.psHeap) == 0 {
		ts.err = io.EOF
		return stats
	}
	heap.Init(&ts.psHeap)
	ts.Item = ts.psHeap[0].Item
	ts.nextItemNoop = true
	return stats
}

// FirstItemWithPrefix seeks for the first item with the given prefix in the ts.
//
// It returns io.EOF if such an item doesn't exist.
func (ts *TableSearch) FirstItemWithPrefix(prefix []byte) error {
	ts.Seek(prefix)
	if !ts.NextItem() {
		if err := ts.Error(); err != nil {
			return err
		}
		return io.EOF
	}
	if err := ts.Error(); err != nil {
		return err
	}
	if !bytes.HasPrefix(ts.Item, prefix) {
		return io.EOF
	}
	return nil
}

// NextItem advances to the next item.
func (ts *TableSearch) NextItem() bool {
	if ts.err != nil {
		return false
	}
	if ts.nextItemNoop {
		ts.nextItemNoop = false
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

func (ts *TableSearch) nextBlock() error {
	psMin := ts.psHeap[0]
	if psMin.NextItem() {
		heap.Fix(&ts.psHeap, 0)
		ts.Item = ts.psHeap[0].Item
		return nil
	}

	if err := psMin.Error(); err != nil {
		return err
	}

	heap.Pop(&ts.psHeap)

	if len(ts.psHeap) == 0 {
		return io.EOF
	}

	ts.Item = ts.psHeap[0].Item
	return nil
}

// Error returns the last error in ts.
func (ts *TableSearch) Error() error {
	if ts.err == io.EOF {
		return nil
	}
	return ts.err
}

// MustClose closes the ts.
func (ts *TableSearch) MustClose() {
	if !ts.needClosing {
		logger.Panicf("BUG: missing Init call before MustClose call")
	}
	ts.tb.putParts(ts.pws)
	ts.reset()
}

type partSearchHeap []*partSearch

func (psh *partSearchHeap) Len() int {
	return len(*psh)
}

func (psh *partSearchHeap) Less(i, j int) bool {
	x := *psh
	return string(x[i].Item) < string(x[j].Item)
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
