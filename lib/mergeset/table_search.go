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
	if err := ts.Error(); err != nil {
		// Do nothing on unrecoverable error.
		return
	}
	ts.err = nil

	// Initialize the psHeap.
	ts.psHeap = ts.psHeap[:0]
	for i := range ts.psPool {
		ps := &ts.psPool[i]
		ps.Seek(k)
		if !ps.NextItem() {
			if err := ps.Error(); err != nil {
				// Return only the first error, since it has no sense in returning all errors.
				ts.err = fmt.Errorf("cannot seek %q: %w", k, err)
				return
			}
			continue
		}
		ts.psHeap = append(ts.psHeap, ps)
	}
	if len(ts.psHeap) == 0 {
		ts.err = io.EOF
		return
	}
	heap.Init(&ts.psHeap)
	ts.Item = ts.psHeap[0].Item
	ts.nextItemNoop = true
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
