package logstorage

import (
	"path/filepath"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
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
	columnNamesWriter        writerWithStats
	columnIdxsWriter         writerWithStats
	metaindexWriter          writerWithStats
	indexWriter              writerWithStats
	columnsHeaderIndexWriter writerWithStats
	columnsHeaderWriter      writerWithStats
	timestampsWriter         writerWithStats

	messageBloomValuesWriter bloomValuesWriter

	bloomValuesShards       []bloomValuesWriter
	createBloomValuesWriter func(shardIdx uint64) bloomValuesStreamWriter
	maxShards               uint64

	// columnNameIDGenerator is used for generating columnName->id mapping for all the columns seen in bsw
	columnNameIDGenerator columnNameIDGenerator

	columnIdxs    map[uint64]uint64
	nextColumnIdx uint64
}

type bloomValuesWriter struct {
	bloom  writerWithStats
	values writerWithStats
}

func (w *bloomValuesWriter) reset() {
	w.bloom.reset()
	w.values.reset()
}

func (w *bloomValuesWriter) init(sw bloomValuesStreamWriter) {
	w.bloom.init(sw.bloom)
	w.values.init(sw.values)
}

func (w *bloomValuesWriter) totalBytesWritten() uint64 {
	return w.bloom.bytesWritten + w.values.bytesWritten
}

func (w *bloomValuesWriter) appendClosers(dst []fs.MustCloser) []fs.MustCloser {
	dst = append(dst, &w.bloom)
	dst = append(dst, &w.values)
	return dst
}

type bloomValuesStreamWriter struct {
	bloom  filestream.WriteCloser
	values filestream.WriteCloser
}

func (sw *streamWriters) reset() {
	sw.columnNamesWriter.reset()
	sw.columnIdxsWriter.reset()
	sw.metaindexWriter.reset()
	sw.indexWriter.reset()
	sw.columnsHeaderIndexWriter.reset()
	sw.columnsHeaderWriter.reset()
	sw.timestampsWriter.reset()

	sw.messageBloomValuesWriter.reset()
	for i := range sw.bloomValuesShards {
		sw.bloomValuesShards[i].reset()
	}
	sw.bloomValuesShards = sw.bloomValuesShards[:0]

	sw.createBloomValuesWriter = nil
	sw.maxShards = 0

	sw.columnNameIDGenerator.reset()
	sw.columnIdxs = nil
	sw.nextColumnIdx = 0
}

func (sw *streamWriters) init(columnNamesWriter, columnIdxsWriter, metaindexWriter, indexWriter,
	columnsHeaderIndexWriter, columnsHeaderWriter, timestampsWriter filestream.WriteCloser,
	messageBloomValuesWriter bloomValuesStreamWriter, createBloomValuesWriter func(shardIdx uint64) bloomValuesStreamWriter, maxShards uint64,
) {
	sw.columnNamesWriter.init(columnNamesWriter)
	sw.columnIdxsWriter.init(columnIdxsWriter)
	sw.metaindexWriter.init(metaindexWriter)
	sw.indexWriter.init(indexWriter)
	sw.columnsHeaderIndexWriter.init(columnsHeaderIndexWriter)
	sw.columnsHeaderWriter.init(columnsHeaderWriter)
	sw.timestampsWriter.init(timestampsWriter)

	sw.messageBloomValuesWriter.init(messageBloomValuesWriter)

	sw.createBloomValuesWriter = createBloomValuesWriter
	sw.maxShards = maxShards
}

func (sw *streamWriters) totalBytesWritten() uint64 {
	n := uint64(0)

	n += sw.columnNamesWriter.bytesWritten
	n += sw.columnIdxsWriter.bytesWritten
	n += sw.metaindexWriter.bytesWritten
	n += sw.indexWriter.bytesWritten
	n += sw.columnsHeaderIndexWriter.bytesWritten
	n += sw.columnsHeaderWriter.bytesWritten
	n += sw.timestampsWriter.bytesWritten

	n += sw.messageBloomValuesWriter.totalBytesWritten()
	for i := range sw.bloomValuesShards {
		n += sw.bloomValuesShards[i].totalBytesWritten()
	}

	return n
}

func (sw *streamWriters) MustClose() {
	// Flush and close files in parallel in order to reduce the time needed for this operation
	// on high-latency storage systems such as NFS or Ceph.
	cs := []fs.MustCloser{
		&sw.columnNamesWriter,
		&sw.columnIdxsWriter,
		&sw.metaindexWriter,
		&sw.indexWriter,
		&sw.columnsHeaderIndexWriter,
		&sw.columnsHeaderWriter,
		&sw.timestampsWriter,
	}

	cs = sw.messageBloomValuesWriter.appendClosers(cs)
	for i := range sw.bloomValuesShards {
		cs = sw.bloomValuesShards[i].appendClosers(cs)
	}

	fs.MustCloseParallel(cs)
}

func (sw *streamWriters) getBloomValuesWriterForColumnName(name string) *bloomValuesWriter {
	if name == "" {
		return &sw.messageBloomValuesWriter
	}

	columnID := sw.columnNameIDGenerator.getColumnNameID(name)
	shardIdx, ok := sw.columnIdxs[columnID]
	if ok {
		return &sw.bloomValuesShards[shardIdx]
	}

	shardIdx = sw.nextColumnIdx % sw.maxShards
	sw.nextColumnIdx++

	if sw.columnIdxs == nil {
		sw.columnIdxs = make(map[uint64]uint64)
	}
	sw.columnIdxs[columnID] = shardIdx

	if shardIdx >= uint64(len(sw.bloomValuesShards)) {
		if shardIdx > uint64(len(sw.bloomValuesShards)) {
			logger.Panicf("BUG: shardIdx must equal %d; got %d; maxShards=%d; columnIdxs=%v", len(sw.bloomValuesShards), shardIdx, sw.maxShards, sw.columnIdxs)
		}
		sws := sw.createBloomValuesWriter(shardIdx)
		sw.bloomValuesShards = slicesutil.SetLength(sw.bloomValuesShards, len(sw.bloomValuesShards)+1)
		sw.bloomValuesShards[len(sw.bloomValuesShards)-1].init(sws)
	}
	return &sw.bloomValuesShards[shardIdx]
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

// reset resets bsw for subsequent reuse.
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

	messageBloomValues := mp.messageBloomValues.NewStreamWriter()
	createBloomValuesWriter := func(_ uint64) bloomValuesStreamWriter {
		return mp.fieldBloomValues.NewStreamWriter()
	}

	bsw.streamWriters.init(&mp.columnNames, &mp.columnIdxs, &mp.metaindex, &mp.index, &mp.columnsHeaderIndex, &mp.columnsHeader, &mp.timestamps, messageBloomValues, createBloomValuesWriter, 1)
}

// MustInitForFilePart initializes bsw for writing data to file part located at path.
//
// if nocache is true, then the written data doesn't go to OS page cache.
func (bsw *blockStreamWriter) MustInitForFilePart(path string, nocache bool) {
	bsw.reset()

	fs.MustMkdirFailIfExist(path)

	// Open part files in parallel in order to minimze the time needed for this operation
	// on high-latency storage systems such as NFS and Ceph.

	columnNamesPath := filepath.Join(path, columnNamesFilename)
	columnIdxsPath := filepath.Join(path, columnIdxsFilename)
	metaindexPath := filepath.Join(path, metaindexFilename)
	indexPath := filepath.Join(path, indexFilename)
	columnsHeaderIndexPath := filepath.Join(path, columnsHeaderIndexFilename)
	columnsHeaderPath := filepath.Join(path, columnsHeaderFilename)
	timestampsPath := filepath.Join(path, timestampsFilename)

	var pfc filestream.ParallelFileCreator

	// Always cache columnNames file, since it is re-read immediately after part creation
	var columnNamesWriter filestream.WriteCloser
	pfc.Add(columnNamesPath, &columnNamesWriter, false)

	// Always cache columnIdxs file, since it is re-read immediately after part creation
	var columnIdxsWriter filestream.WriteCloser
	pfc.Add(columnIdxsPath, &columnIdxsWriter, false)

	// Always cache metaindex file, since it is re-read immediately after part creation
	var metaindexWriter filestream.WriteCloser
	pfc.Add(metaindexPath, &metaindexWriter, false)

	var indexWriter filestream.WriteCloser
	pfc.Add(indexPath, &indexWriter, nocache)

	var columnsHeaderIndexWriter filestream.WriteCloser
	pfc.Add(columnsHeaderIndexPath, &columnsHeaderIndexWriter, nocache)

	var columnsHeaderWriter filestream.WriteCloser
	pfc.Add(columnsHeaderPath, &columnsHeaderWriter, nocache)

	var timestampsWriter filestream.WriteCloser
	pfc.Add(timestampsPath, &timestampsWriter, nocache)

	messageBloomFilterPath := filepath.Join(path, messageBloomFilename)
	messageValuesPath := filepath.Join(path, messageValuesFilename)
	var messageBloomValuesWriter bloomValuesStreamWriter
	pfc.Add(messageBloomFilterPath, &messageBloomValuesWriter.bloom, nocache)
	pfc.Add(messageValuesPath, &messageBloomValuesWriter.values, nocache)

	pfc.Run()

	createBloomValuesWriter := func(shardIdx uint64) bloomValuesStreamWriter {
		bloomPath := getBloomFilePath(path, shardIdx)
		valuesPath := getValuesFilePath(path, shardIdx)

		var bvsw bloomValuesStreamWriter
		bvsw.bloom = filestream.MustCreate(bloomPath, nocache)
		bvsw.values = filestream.MustCreate(valuesPath, nocache)

		return bvsw
	}

	bsw.streamWriters.init(columnNamesWriter, columnIdxsWriter, metaindexWriter, indexWriter,
		columnsHeaderIndexWriter, columnsHeaderWriter, timestampsWriter, messageBloomValuesWriter,
		createBloomValuesWriter, bloomValuesMaxShardsCount)
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
// bsw can be reused after calling Finalize().
func (bsw *blockStreamWriter) Finalize(ph *partHeader) {
	ph.FormatVersion = partFormatLatestVersion
	ph.UncompressedSizeBytes = bsw.globalUncompressedSizeBytes
	ph.RowsCount = bsw.globalRowsCount
	ph.BlocksCount = bsw.globalBlocksCount
	ph.MinTimestamp = bsw.globalMinTimestamp
	ph.MaxTimestamp = bsw.globalMaxTimestamp
	ph.BloomValuesShardsCount = uint64(len(bsw.streamWriters.bloomValuesShards))

	bsw.mustFlushIndexBlock(bsw.indexBlockData)

	// Write columnNames data
	mustWriteColumnNames(&bsw.streamWriters.columnNamesWriter, bsw.streamWriters.columnNameIDGenerator.columnNames)

	// Write columnIdxs data
	mustWriteColumnIdxs(&bsw.streamWriters.columnIdxsWriter, bsw.streamWriters.columnIdxs)

	// Write metaindex data
	mustWriteIndexBlockHeaders(&bsw.streamWriters.metaindexWriter, bsw.metaindexData)

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
