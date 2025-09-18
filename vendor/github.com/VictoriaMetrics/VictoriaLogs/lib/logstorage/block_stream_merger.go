package logstorage

import (
	"container/heap"
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// mustMergeBlockStreams merges bsrs to bsw and updates ph accordingly.
//
// Finalize() is guaranteed to be called on bsrs and bsw before returning from the func.
func mustMergeBlockStreams(ph *partHeader, bsw *blockStreamWriter, bsrs []*blockStreamReader, stopCh <-chan struct{}) {
	bsm := getBlockStreamMerger()
	bsm.mustInit(bsw, bsrs)
	for len(bsm.readersHeap) > 0 {
		if needStop(stopCh) {
			break
		}
		bsr := bsm.readersHeap[0]
		bsm.mustWriteBlock(&bsr.blockData, bsw)
		if bsr.NextBlock() {
			heap.Fix(&bsm.readersHeap, 0)
		} else {
			heap.Pop(&bsm.readersHeap)
		}
	}
	bsm.mustFlushRows()
	putBlockStreamMerger(bsm)

	bsw.Finalize(ph)
	mustCloseBlockStreamReaders(bsrs)
}

// blockStreamMerger merges block streams
type blockStreamMerger struct {
	// bsw is the block stream writer to write the merged blocks.
	bsw *blockStreamWriter

	// bsrs contains the original readers passed to mustInit().
	// They are used by ReadersPaths()
	bsrs []*blockStreamReader

	// readersHeap contains a heap of readers to read blocks to merge.
	readersHeap blockStreamReadersHeap

	// streamID is the stream ID for the pending data.
	streamID streamID

	// sbu is the unmarshaler for strings in rows and rowsTmp.
	sbu *stringsBlockUnmarshaler

	// vd is the decoder for unmarshaled strings.
	vd *valuesDecoder

	// bd is the pending blockData.
	// bd is unpacked into rows when needed.
	bd blockData

	// a holds bd data.
	a arena

	// rows is pending log entries.
	rows rows

	// rowsTmp is temporary storage for log entries during merge.
	rowsTmp rows

	// uncompressedRowsSizeBytes is the current size of uncompressed rows.
	//
	// It is used for flushing rows to blocks when their size reaches maxUncompressedBlockSize
	uncompressedRowsSizeBytes uint64

	// uniqueFields is an upper bound estimation for the number of unique fields in either rows or bd
	//
	// It is used for limiting the number of columns written per block
	uniqueFields int
}

func (bsm *blockStreamMerger) reset() {
	bsm.bsw = nil

	rhs := bsm.readersHeap
	for i := range rhs {
		rhs[i] = nil
	}
	bsm.readersHeap = rhs[:0]

	bsm.streamID.reset()
	bsm.resetRows()
}

func (bsm *blockStreamMerger) resetRows() {
	if bsm.sbu != nil {
		putStringsBlockUnmarshaler(bsm.sbu)
		bsm.sbu = nil
	}
	if bsm.vd != nil {
		putValuesDecoder(bsm.vd)
		bsm.vd = nil
	}
	bsm.bd.reset()
	bsm.a.reset()

	bsm.rows.reset()
	bsm.rowsTmp.reset()

	bsm.uncompressedRowsSizeBytes = 0
	bsm.uniqueFields = 0
}

func (bsm *blockStreamMerger) mustInit(bsw *blockStreamWriter, bsrs []*blockStreamReader) {
	bsm.reset()

	bsm.bsw = bsw
	bsm.bsrs = bsrs

	rsh := bsm.readersHeap[:0]
	for _, bsr := range bsrs {
		if bsr.NextBlock() {
			rsh = append(rsh, bsr)
		}
	}
	bsm.readersHeap = rsh
	heap.Init(&bsm.readersHeap)
}

// mustWriteBlock writes bd to bsm
func (bsm *blockStreamMerger) mustWriteBlock(bd *blockData, bsw *blockStreamWriter) {
	bsm.checkNextBlock(bd)
	uniqueFields := len(bd.columnsData) + len(bd.constColumns)
	switch {
	case !bd.streamID.equal(&bsm.streamID):
		// The bd contains another streamID.
		// Write the current log entries under the current streamID, then process the bd.
		bsm.mustFlushRows()
		bsm.streamID = bd.streamID
		if bd.uncompressedSizeBytes >= maxUncompressedBlockSize {
			// Fast path - write full bd to the output without extracting log entries from it.
			bsw.MustWriteBlockData(bd)
		} else {
			// Slow path - copy the bd to the curr bd.
			bsm.a.reset()
			bsm.bd.copyFrom(&bsm.a, bd)
			bsm.uniqueFields = uniqueFields
		}
	case bsm.uniqueFields+uniqueFields > maxColumnsPerBlock:
		// Cannot merge bd with bsm.rows, because too many columns will be created.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4762
		//
		// Flush bsm.rows and copy the bd to the curr bd.
		bsm.mustFlushRows()
		if uniqueFields >= maxColumnsPerBlock {
			bsw.MustWriteBlockData(bd)
		} else {
			bsm.a.reset()
			bsm.bd.copyFrom(&bsm.a, bd)
			bsm.uniqueFields = uniqueFields
		}
	case bd.uncompressedSizeBytes >= maxUncompressedBlockSize:
		// The bd contains the same streamID and it is full,
		// so it can be written next after the current log entries
		// without the need to merge the bd with the current log entries.
		// Write the current log entries and then the bd.
		bsm.mustFlushRows()
		bsw.MustWriteBlockData(bd)
	default:
		// The bd contains the same streamID and it isn't full,
		// so it must be merged with the current log entries.
		bsm.mustMergeRows(bd)
		bsm.uniqueFields += uniqueFields
	}
}

// checkNextBlock checks whether the bd can be written next after the current data.
func (bsm *blockStreamMerger) checkNextBlock(bd *blockData) {
	if len(bsm.rows.timestamps) > 0 && bsm.bd.rowsCount > 0 {
		logger.Panicf("BUG: bsm.bd must be empty when bsm.rows isn't empty! got %d log entries in bsm.bd", bsm.bd.rowsCount)
	}
	if bd.streamID.less(&bsm.streamID) {
		logger.Panicf("FATAL: cannot merge %s: the streamID=%s for the next block is smaller than the streamID=%s for the current block",
			bsm.ReadersPaths(), &bd.streamID, &bsm.streamID)
	}
	if !bd.streamID.equal(&bsm.streamID) {
		return
	}
	// streamID at bd equals streamID at bsm. Check that minTimestamp in bd is bigger or equal to the minTimestmap at bsm.
	if bd.rowsCount == 0 {
		return
	}
	nextMinTimestamp := bd.timestampsData.minTimestamp
	if len(bsm.rows.timestamps) == 0 {
		if bsm.bd.rowsCount == 0 {
			return
		}
		minTimestamp := bsm.bd.timestampsData.minTimestamp
		if nextMinTimestamp < minTimestamp {
			logger.Panicf("FATAL: cannot merge %s: the next block's minTimestamp=%d is smaller than the minTimestamp=%d for the current block",
				bsm.ReadersPaths(), nextMinTimestamp, minTimestamp)
		}
		return
	}
	minTimestamp := bsm.rows.timestamps[0]
	if nextMinTimestamp < minTimestamp {
		logger.Panicf("FATAL: cannot merge %s: the next block's minTimestamp=%d is smaller than the minTimestamp=%d for log entries for the current block",
			bsm.ReadersPaths(), nextMinTimestamp, minTimestamp)
	}
}

// ReadersPaths returns paths for input blockStreamReaders
func (bsm *blockStreamMerger) ReadersPaths() string {
	paths := make([]string, len(bsm.bsrs))
	for i, bsr := range bsm.bsrs {
		paths[i] = bsr.Path()
	}
	return fmt.Sprintf("[%s]", strings.Join(paths, ","))
}

// mustMergeRows merges the current log entries inside bsm with bd log entries.
func (bsm *blockStreamMerger) mustMergeRows(bd *blockData) {
	if bsm.bd.rowsCount > 0 {
		// Unmarshal log entries from bsm.bd
		bsm.mustUnmarshalRows(&bsm.bd)
		bsm.bd.reset()
		bsm.a.reset()
	}

	// Unmarshal log entries from bd
	rowsLen := len(bsm.rows.timestamps)
	bsm.mustUnmarshalRows(bd)

	// Merge unmarshaled log entries
	timestamps := bsm.rows.timestamps
	rows := bsm.rows.rows
	bsm.rowsTmp.mergeRows(timestamps[:rowsLen], timestamps[rowsLen:], rows[:rowsLen], rows[rowsLen:])
	bsm.rows, bsm.rowsTmp = bsm.rowsTmp, bsm.rows
	bsm.rowsTmp.reset()

	if bsm.uncompressedRowsSizeBytes >= maxUncompressedBlockSize {
		bsm.mustFlushRows()
	}
}

func (bsm *blockStreamMerger) mustUnmarshalRows(bd *blockData) {
	rowsLen := len(bsm.rows.timestamps)
	if bsm.sbu == nil {
		bsm.sbu = getStringsBlockUnmarshaler()
	}
	if bsm.vd == nil {
		bsm.vd = getValuesDecoder()
	}
	if err := bd.unmarshalRows(&bsm.rows, bsm.sbu, bsm.vd); err != nil {
		logger.Panicf("FATAL: cannot merge %s: cannot unmarshal log entries from blockData: %s", bsm.ReadersPaths(), err)
	}
	bsm.uncompressedRowsSizeBytes += uncompressedRowsSizeBytes(bsm.rows.rows[rowsLen:])
}

func (bsm *blockStreamMerger) mustFlushRows() {
	if len(bsm.rows.timestamps) == 0 {
		bsm.bsw.MustWriteBlockData(&bsm.bd)
	} else {
		bsm.bsw.MustWriteRows(&bsm.streamID, bsm.rows.timestamps, bsm.rows.rows)
	}
	bsm.resetRows()
}

func getBlockStreamMerger() *blockStreamMerger {
	v := blockStreamMergerPool.Get()
	if v == nil {
		return &blockStreamMerger{}
	}
	return v.(*blockStreamMerger)
}

func putBlockStreamMerger(bsm *blockStreamMerger) {
	bsm.reset()
	blockStreamMergerPool.Put(bsm)
}

var blockStreamMergerPool sync.Pool

type blockStreamReadersHeap []*blockStreamReader

func (h *blockStreamReadersHeap) Len() int {
	return len(*h)
}

func (h *blockStreamReadersHeap) Less(i, j int) bool {
	x := *h
	a := &x[i].blockData
	b := &x[j].blockData
	if !a.streamID.equal(&b.streamID) {
		return a.streamID.less(&b.streamID)
	}
	return a.timestampsData.minTimestamp < b.timestampsData.minTimestamp
}

func (h *blockStreamReadersHeap) Swap(i, j int) {
	x := *h
	x[i], x[j] = x[j], x[i]
}

func (h *blockStreamReadersHeap) Push(v any) {
	bsr := v.(*blockStreamReader)
	*h = append(*h, bsr)
}

func (h *blockStreamReadersHeap) Pop() any {
	x := *h
	bsr := x[len(x)-1]
	x[len(x)-1] = nil
	*h = x[:len(x)-1]
	return bsr
}
