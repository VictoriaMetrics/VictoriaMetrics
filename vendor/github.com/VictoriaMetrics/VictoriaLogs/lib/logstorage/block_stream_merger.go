package logstorage

import (
	"container/heap"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// mustMergeBlockStreams merges bsrs to bsw and updates ph accordingly.
//
// if dropFilter is non-nil, then rows matching dropFilter are dropped during the merge.
//
// Finalize() is guaranteed to be called on bsw before returning from the func.
// MustClose() is guatanteed to be called on bsrs before returning from the func.
func mustMergeBlockStreams(ph *partHeader, idb *indexdb, bsw *blockStreamWriter, bsrs []*blockStreamReader, dropFilter *partitionSearchOptions, stopCh <-chan struct{}) {
	bsm := getBlockStreamMerger()
	bsm.mustInit(idb, bsw, bsrs, dropFilter)
	for len(bsm.readersHeap) > 0 {
		if needStop(stopCh) {
			break
		}
		bsr := bsm.readersHeap[0]
		bsm.mustWriteBlock(&bsr.blockData)
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
	// idb is indexdb for the current partition.
	//
	// It is used for filling up streamBuf and streamIDBuf.
	idb *indexdb

	// bsw is the block stream writer to write the merged blocks to.
	bsw *blockStreamWriter

	// bsrs contains the original readers passed to mustInit().
	// They are used by ReadersPaths()
	bsrs []*blockStreamReader

	// dropFilter is an optional filter for dropping matching rows during the merge.
	dropFilter *partitionSearchOptions

	// dropFilterFields contains the list of fields needed by dropFilter.
	dropFilterFields prefixfilter.Filter

	// readersHeap contains a heap of readers to read blocks to merge.
	readersHeap blockStreamReadersHeap

	// streamID is the stream ID for the pending data.
	streamID streamID

	// streamBuf is _stream field value for the current stream.
	//
	// It is used when dropFilter is set.
	streamBuf []byte

	// streamIDBuf is _stream_id field value for the current stream.
	//
	// It is used when dropFilter is set.
	streamIDBuf []byte

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
}

func (bsm *blockStreamMerger) reset() {
	bsm.idb = nil
	bsm.bsw = nil
	bsm.bsrs = nil
	bsm.dropFilter = nil
	bsm.dropFilterFields.Reset()

	rhs := bsm.readersHeap
	for i := range rhs {
		rhs[i] = nil
	}
	bsm.readersHeap = rhs[:0]

	bsm.streamID.reset()
	bsm.streamBuf = bsm.streamBuf[:0]
	bsm.streamIDBuf = bsm.streamIDBuf[:0]
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
}

func (bsm *blockStreamMerger) assertNoRows() {
	if bsm.bd.rowsCount > 0 {
		logger.Panicf("BUG: bsm.bd must be empty; got %d rows", bsm.bd.rowsCount)
	}
	if len(bsm.a.b) > 0 {
		logger.Panicf("BUG: bsm.a must be empty; got %d bytes", len(bsm.a.b))
	}
	if len(bsm.rows.timestamps) > 0 {
		logger.Panicf("BUG: bsm.rows must be empty; got %d rows", len(bsm.rows.timestamps))
	}
	if len(bsm.rowsTmp.timestamps) > 0 {
		logger.Panicf("BUG: bsm.rowsTmp must be empty; got %d rows", len(bsm.rowsTmp.timestamps))
	}
	if bsm.uncompressedRowsSizeBytes != 0 {
		logger.Panicf("BUG: bsm.uncompressedRowsSizeBytes must be 0; got %d", bsm.uncompressedRowsSizeBytes)
	}
}

func (bsm *blockStreamMerger) mustInit(idb *indexdb, bsw *blockStreamWriter, bsrs []*blockStreamReader, dropFilter *partitionSearchOptions) {
	bsm.reset()

	bsm.idb = idb
	bsm.bsw = bsw
	bsm.bsrs = bsrs

	bsm.dropFilter = dropFilter
	if dropFilter != nil {
		dropFilter.filter.updateNeededFields(&bsm.dropFilterFields)
	}

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
func (bsm *blockStreamMerger) mustWriteBlock(bd *blockData) {
	bsm.checkNextBlock(bd)
	switch {
	case !bd.streamID.equal(&bsm.streamID):
		// The bd contains another streamID.
		// Write the bsm logs under the current streamID, then process the bd.
		bsm.mustFlushRows()
		bsm.setStreamID(bd.streamID)
		bsm.mustWriteBlockData(bd)
	case bsm.uncompressedRowsSizeBytes == 0 && bsm.bd.rowsCount == 0 && bd.uncompressedSizeBytes >= maxUncompressedBlockSize:
		// The bsm is empty and the bd is full. Just write db to the output without spending CPU time on re-compression.
		bsm.mustWriteBlockData(bd)
	case bsm.uncompressedRowsSizeBytes+bsm.bd.uncompressedSizeBytes+bd.uncompressedSizeBytes >= 2*maxUncompressedBlockSize:
		// The bd cannot be merged with bsm, since the final block size will be too big.
		// Write the bsm logs, then process the bd.
		bsm.mustFlushRows()
		bsm.mustWriteBlockData(bd)
	default:
		// The bd contains the same streamID and the summary size of bsm logs and bd doesn't exceed the maximum allowed.
		// Merge them.
		bsm.mustMergeRows(bd)
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

// mustWriteBlockData writes bd to bsm.
func (bsm *blockStreamMerger) mustWriteBlockData(bd *blockData) {
	bsm.assertNoRows()

	td := &bd.timestampsData
	if bsm.needDropRows(&bd.streamID, td.minTimestamp, td.maxTimestamp) {
		if _, ok := bsm.dropFilter.filter.(*filterNoop); ok {
			// Fast path - drop the whole bd.
			// This path occurs when the dropFilter contains only stream filter - '{...}'.
			// The stream filter goes to dropFilter.streamFilter, while dropFilter.filter becomes noop.
			return
		}
		// Slow path - unpack bd and drop the needed rows before the merge.
		bsm.mustMergeRows(bd)
		return
	}

	if bd.uncompressedSizeBytes >= maxUncompressedBlockSize {
		// Fast path - write full bd to the output without extracting log entries from it.
		bsm.bsw.MustWriteBlockData(bd)
		return
	}

	bsm.bd.copyFrom(&bsm.a, bd)
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

	td := &bd.timestampsData
	if bsm.needDropRows(&bd.streamID, td.minTimestamp, td.maxTimestamp) {
		stream, streamID := bsm.getStreamAndStreamID()
		bsm.rows.skipRowsByDropFilter(bsm.dropFilter, &bsm.dropFilterFields, rowsLen, stream, streamID)
	}

	bsm.uncompressedRowsSizeBytes += uncompressedRowsSizeBytes(bsm.rows.rows[rowsLen:])
}

func (bsm *blockStreamMerger) needDropRows(sid *streamID, minTimestamp, maxTimestamp int64) bool {
	return bsm.dropFilter != nil && bsm.dropFilter.matchStreamID(sid) && bsm.dropFilter.matchTimeRange(minTimestamp, maxTimestamp)
}

func (bsm *blockStreamMerger) setStreamID(sid streamID) {
	bsm.streamID = sid

	if bsm.needDropRows(&bsm.streamID, math.MinInt64, math.MaxInt64) {
		bsm.streamBuf = bsm.idb.appendStreamString(bsm.streamBuf[:0], &bsm.streamID)
		bsm.streamIDBuf = sid.marshalString(bsm.streamIDBuf[:0])
	}
}

func (bsm *blockStreamMerger) getStreamAndStreamID() (string, string) {
	return bytesutil.ToUnsafeString(bsm.streamBuf), bytesutil.ToUnsafeString(bsm.streamIDBuf)
}

func (bsm *blockStreamMerger) mustFlushRows() {
	if len(bsm.rows.timestamps) == 0 {
		bsm.bsw.MustWriteBlockData(&bsm.bd)
	} else if bsm.rows.hasNonEmptyRows() {
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
