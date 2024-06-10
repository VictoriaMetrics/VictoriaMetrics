package logstorage

import (
	"path/filepath"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// writerWithStats writes data to w and tracks the total amounts of data written at bytesWritten.
type writerWithStats struct {
	w            filestream.WriteCloser
	bytesWritten uint64
}

func (w *writerWithStats) reset() {
	w.w = nil
	w.bytesWritten = 0
}

func (w *writerWithStats) init(wc filestream.WriteCloser) {
	w.reset()

	w.w = wc
}

func (w *writerWithStats) Path() string {
	return w.w.Path()
}

func (w *writerWithStats) MustWrite(data []byte) {
	fs.MustWriteData(w.w, data)
	w.bytesWritten += uint64(len(data))
}

// MustClose closes the underlying w.
func (w *writerWithStats) MustClose() {
	w.w.MustClose()
}

// streamWriters contain writers for blockStreamWriter
type streamWriters struct {
	metaindexWriter          writerWithStats
	indexWriter              writerWithStats
	columnsHeaderWriter      writerWithStats
	timestampsWriter         writerWithStats
	fieldValuesWriter        writerWithStats
	fieldBloomFilterWriter   writerWithStats
	messageValuesWriter      writerWithStats
	messageBloomFilterWriter writerWithStats
}

func (sw *streamWriters) reset() {
	sw.metaindexWriter.reset()
	sw.indexWriter.reset()
	sw.columnsHeaderWriter.reset()
	sw.timestampsWriter.reset()
	sw.fieldValuesWriter.reset()
	sw.fieldBloomFilterWriter.reset()
	sw.messageValuesWriter.reset()
	sw.messageBloomFilterWriter.reset()
}

func (sw *streamWriters) init(metaindexWriter, indexWriter, columnsHeaderWriter, timestampsWriter, fieldValuesWriter, fieldBloomFilterWriter,
	messageValuesWriter, messageBloomFilterWriter filestream.WriteCloser,
) {
	sw.metaindexWriter.init(metaindexWriter)
	sw.indexWriter.init(indexWriter)
	sw.columnsHeaderWriter.init(columnsHeaderWriter)
	sw.timestampsWriter.init(timestampsWriter)
	sw.fieldValuesWriter.init(fieldValuesWriter)
	sw.fieldBloomFilterWriter.init(fieldBloomFilterWriter)
	sw.messageValuesWriter.init(messageValuesWriter)
	sw.messageBloomFilterWriter.init(messageBloomFilterWriter)
}

func (sw *streamWriters) totalBytesWritten() uint64 {
	n := uint64(0)
	n += sw.metaindexWriter.bytesWritten
	n += sw.indexWriter.bytesWritten
	n += sw.columnsHeaderWriter.bytesWritten
	n += sw.timestampsWriter.bytesWritten
	n += sw.fieldValuesWriter.bytesWritten
	n += sw.fieldBloomFilterWriter.bytesWritten
	n += sw.messageValuesWriter.bytesWritten
	n += sw.messageBloomFilterWriter.bytesWritten
	return n
}

func (sw *streamWriters) MustClose() {
	sw.metaindexWriter.MustClose()
	sw.indexWriter.MustClose()
	sw.columnsHeaderWriter.MustClose()
	sw.timestampsWriter.MustClose()
	sw.fieldValuesWriter.MustClose()
	sw.fieldBloomFilterWriter.MustClose()
	sw.messageValuesWriter.MustClose()
	sw.messageBloomFilterWriter.MustClose()
}

// blockStreamWriter is used for writing blocks into the underlying storage in streaming manner.
type blockStreamWriter struct {
	// streamWriters contains writer for block data
	streamWriters streamWriters

	// sidLast is the streamID for the last written block
	sidLast streamID

	// sidFirst is the streamID for the first block in the current indexBlock
	sidFirst streamID

	// minTimestampLast is the minimum timestamp seen for the last written block
	minTimestampLast int64

	// minTimestamp is the minimum timestamp seen across written blocks for the current indexBlock
	minTimestamp int64

	// maxTimestamp is the maximum timestamp seen across written blocks for the current indexBlock
	maxTimestamp int64

	// hasWrittenBlocks is set to true if at least a single block is written to the current indexBlock
	hasWrittenBlocks bool

	// globalUncompressedSizeBytes is the total size of all the log entries written via bsw
	globalUncompressedSizeBytes uint64

	// globalRowsCount is the total number of log entries written via bsw
	globalRowsCount uint64

	// globalBlocksCount is the total number of blocks written to bsw
	globalBlocksCount uint64

	// globalMinTimestamp is the minimum timestamp seen across all the blocks written to bsw
	globalMinTimestamp int64

	// globalMaxTimestamp is the maximum timestamp seen across all the blocks written to bsw
	globalMaxTimestamp int64

	// indexBlockData contains marshaled blockHeader data, which isn't written yet to indexFilename
	indexBlockData []byte

	// metaindexData contains marshaled indexBlockHeader data, which isn't written yet to metaindexFilename
	metaindexData []byte

	// indexBlockHeader is used for marshaling the data to metaindexData
	indexBlockHeader indexBlockHeader
}

// reset resets bsw for subsequent re-use.
func (bsw *blockStreamWriter) reset() {
	bsw.streamWriters.reset()
	bsw.sidLast.reset()
	bsw.sidFirst.reset()
	bsw.minTimestampLast = 0
	bsw.minTimestamp = 0
	bsw.maxTimestamp = 0
	bsw.hasWrittenBlocks = false
	bsw.globalUncompressedSizeBytes = 0
	bsw.globalRowsCount = 0
	bsw.globalBlocksCount = 0
	bsw.globalMinTimestamp = 0
	bsw.globalMaxTimestamp = 0
	bsw.indexBlockData = bsw.indexBlockData[:0]

	if len(bsw.metaindexData) > 1024*1024 {
		// The length of bsw.metaindexData is unbound, so drop too long buffer
		// in order to conserve memory.
		bsw.metaindexData = nil
	} else {
		bsw.metaindexData = bsw.metaindexData[:0]
	}

	bsw.indexBlockHeader.reset()
}

// MustInitForInmemoryPart initializes bsw from mp
func (bsw *blockStreamWriter) MustInitForInmemoryPart(mp *inmemoryPart) {
	bsw.reset()
	bsw.streamWriters.init(&mp.metaindex, &mp.index, &mp.columnsHeader, &mp.timestamps, &mp.fieldValues, &mp.fieldBloomFilter, &mp.messageValues, &mp.messageBloomFilter)
}

// MustInitForFilePart initializes bsw for writing data to file part located at path.
//
// if nocache is true, then the written data doesn't go to OS page cache.
func (bsw *blockStreamWriter) MustInitForFilePart(path string, nocache bool) {
	bsw.reset()

	fs.MustMkdirFailIfExist(path)

	metaindexPath := filepath.Join(path, metaindexFilename)
	indexPath := filepath.Join(path, indexFilename)
	columnsHeaderPath := filepath.Join(path, columnsHeaderFilename)
	timestampsPath := filepath.Join(path, timestampsFilename)
	fieldValuesPath := filepath.Join(path, fieldValuesFilename)
	fieldBloomFilterPath := filepath.Join(path, fieldBloomFilename)
	messageValuesPath := filepath.Join(path, messageValuesFilename)
	messageBloomFilterPath := filepath.Join(path, messageBloomFilename)

	// Always cache metaindex file, since it it re-read immediately after part creation
	metaindexWriter := filestream.MustCreate(metaindexPath, false)

	indexWriter := filestream.MustCreate(indexPath, nocache)
	columnsHeaderWriter := filestream.MustCreate(columnsHeaderPath, nocache)
	timestampsWriter := filestream.MustCreate(timestampsPath, nocache)
	fieldValuesWriter := filestream.MustCreate(fieldValuesPath, nocache)
	fieldBloomFilterWriter := filestream.MustCreate(fieldBloomFilterPath, nocache)
	messageValuesWriter := filestream.MustCreate(messageValuesPath, nocache)
	messageBloomFilterWriter := filestream.MustCreate(messageBloomFilterPath, nocache)

	bsw.streamWriters.init(metaindexWriter, indexWriter, columnsHeaderWriter, timestampsWriter,
		fieldValuesWriter, fieldBloomFilterWriter, messageValuesWriter, messageBloomFilterWriter)
}

// MustWriteRows writes timestamps with rows under the given sid to bsw.
//
// timestamps must be sorted.
// sid must be bigger or equal to the sid for the previously written rs.
func (bsw *blockStreamWriter) MustWriteRows(sid *streamID, timestamps []int64, rows [][]Field) {
	if len(timestamps) == 0 {
		return
	}

	b := getBlock()
	b.MustInitFromRows(timestamps, rows)
	bsw.MustWriteBlock(sid, b)
	putBlock(b)
}

// MustWriteBlockData writes bd to bsw.
//
// The bd.streamID must be bigger or equal to the streamID for the previously written blocks.
func (bsw *blockStreamWriter) MustWriteBlockData(bd *blockData) {
	if bd.rowsCount == 0 {
		return
	}
	bsw.mustWriteBlockInternal(&bd.streamID, nil, bd)
}

// MustWriteBlock writes b under the given sid to bsw.
//
// The sid must be bigger or equal to the sid for the previously written blocks.
// The minimum timestamp in b must be bigger or equal to the minimum timestamp written to the same sid.
func (bsw *blockStreamWriter) MustWriteBlock(sid *streamID, b *block) {
	rowsCount := b.Len()
	if rowsCount == 0 {
		return
	}
	bsw.mustWriteBlockInternal(sid, b, nil)
}

func (bsw *blockStreamWriter) mustWriteBlockInternal(sid *streamID, b *block, bd *blockData) {
	if sid.less(&bsw.sidLast) {
		logger.Panicf("BUG: the sid=%s cannot be smaller than the previously written sid=%s", sid, &bsw.sidLast)
	}
	hasWrittenBlocks := bsw.hasWrittenBlocks
	if !hasWrittenBlocks {
		bsw.sidFirst = *sid
		bsw.hasWrittenBlocks = true
	}
	isSeenSid := sid.equal(&bsw.sidLast)
	bsw.sidLast = *sid

	bh := getBlockHeader()
	if b != nil {
		b.mustWriteTo(sid, bh, &bsw.streamWriters)
	} else {
		bd.mustWriteTo(bh, &bsw.streamWriters)
	}
	th := &bh.timestampsHeader
	if bsw.globalRowsCount == 0 || th.minTimestamp < bsw.globalMinTimestamp {
		bsw.globalMinTimestamp = th.minTimestamp
	}
	if bsw.globalRowsCount == 0 || th.maxTimestamp > bsw.globalMaxTimestamp {
		bsw.globalMaxTimestamp = th.maxTimestamp
	}
	if !hasWrittenBlocks || th.minTimestamp < bsw.minTimestamp {
		bsw.minTimestamp = th.minTimestamp
	}
	if !hasWrittenBlocks || th.maxTimestamp > bsw.maxTimestamp {
		bsw.maxTimestamp = th.maxTimestamp
	}
	if isSeenSid && th.minTimestamp < bsw.minTimestampLast {
		logger.Panicf("BUG: the block for sid=%s cannot contain timestamp smaller than %d, but it contains timestamp %d", sid, bsw.minTimestampLast, th.minTimestamp)
	}
	bsw.minTimestampLast = th.minTimestamp

	bsw.globalUncompressedSizeBytes += bh.uncompressedSizeBytes
	bsw.globalRowsCount += bh.rowsCount
	bsw.globalBlocksCount++

	// Marshal bh
	bsw.indexBlockData = bh.marshal(bsw.indexBlockData)
	putBlockHeader(bh)
	if len(bsw.indexBlockData) > maxUncompressedIndexBlockSize {
		bsw.mustFlushIndexBlock(bsw.indexBlockData)
		bsw.indexBlockData = bsw.indexBlockData[:0]
	}
}

func (bsw *blockStreamWriter) mustFlushIndexBlock(data []byte) {
	if len(data) > 0 {
		bsw.indexBlockHeader.mustWriteIndexBlock(data, bsw.sidFirst, bsw.minTimestamp, bsw.maxTimestamp, &bsw.streamWriters)
		bsw.metaindexData = bsw.indexBlockHeader.marshal(bsw.metaindexData)
	}
	bsw.hasWrittenBlocks = false
	bsw.minTimestamp = 0
	bsw.maxTimestamp = 0
	bsw.sidFirst.reset()
}

// Finalize() finalizes the data write process and updates ph with the finalized stats
//
// It closes the writers passed to MustInit().
//
// bsw can be re-used after calling Finalize().
func (bsw *blockStreamWriter) Finalize(ph *partHeader) {
	ph.UncompressedSizeBytes = bsw.globalUncompressedSizeBytes
	ph.RowsCount = bsw.globalRowsCount
	ph.BlocksCount = bsw.globalBlocksCount
	ph.MinTimestamp = bsw.globalMinTimestamp
	ph.MaxTimestamp = bsw.globalMaxTimestamp

	bsw.mustFlushIndexBlock(bsw.indexBlockData)

	// Write metaindex data
	bb := longTermBufPool.Get()
	bb.B = encoding.CompressZSTDLevel(bb.B[:0], bsw.metaindexData, 1)
	bsw.streamWriters.metaindexWriter.MustWrite(bb.B)
	if len(bb.B) < 1024*1024 {
		longTermBufPool.Put(bb)
	}

	ph.CompressedSizeBytes = bsw.streamWriters.totalBytesWritten()

	bsw.streamWriters.MustClose()
	bsw.reset()
}

var longTermBufPool bytesutil.ByteBufferPool

// getBlockStreamWriter returns new blockStreamWriter from the pool.
//
// Return back the blockStreamWriter to the pool when it is no longer needed by calling putBlockStreamWriter.
func getBlockStreamWriter() *blockStreamWriter {
	v := blockStreamWriterPool.Get()
	if v == nil {
		return &blockStreamWriter{}
	}
	return v.(*blockStreamWriter)
}

// putBlockStreamWriter returns bsw to the pool.
func putBlockStreamWriter(bsw *blockStreamWriter) {
	bsw.reset()
	blockStreamWriterPool.Put(bsw)
}

var blockStreamWriterPool sync.Pool
