package logstorage

import (
	"path/filepath"
	"sync"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
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
	columnNamesWriter        writerWithStats
	metaindexWriter          writerWithStats
	indexWriter              writerWithStats
	columnsHeaderIndexWriter writerWithStats
	columnsHeaderWriter      writerWithStats
	timestampsWriter         writerWithStats

	messageBloomValuesWriter bloomValuesWriter
	bloomValuesShards        [bloomValuesShardsCount]bloomValuesWriter
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

func (w *bloomValuesWriter) MustClose() {
	w.bloom.MustClose()
	w.values.MustClose()
}

type bloomValuesStreamWriter struct {
	bloom  filestream.WriteCloser
	values filestream.WriteCloser
}

func (sw *streamWriters) reset() {
	sw.columnNamesWriter.reset()
	sw.metaindexWriter.reset()
	sw.indexWriter.reset()
	sw.columnsHeaderIndexWriter.reset()
	sw.columnsHeaderWriter.reset()
	sw.timestampsWriter.reset()

	sw.messageBloomValuesWriter.reset()
	for i := range sw.bloomValuesShards[:] {
		sw.bloomValuesShards[i].reset()
	}
}

func (sw *streamWriters) init(columnNamesWriter, metaindexWriter, indexWriter, columnsHeaderIndexWriter, columnsHeaderWriter, timestampsWriter filestream.WriteCloser,
	messageBloomValuesWriter bloomValuesStreamWriter, bloomValuesShards [bloomValuesShardsCount]bloomValuesStreamWriter,
) {
	sw.columnNamesWriter.init(columnNamesWriter)
	sw.metaindexWriter.init(metaindexWriter)
	sw.indexWriter.init(indexWriter)
	sw.columnsHeaderIndexWriter.init(columnsHeaderIndexWriter)
	sw.columnsHeaderWriter.init(columnsHeaderWriter)
	sw.timestampsWriter.init(timestampsWriter)

	sw.messageBloomValuesWriter.init(messageBloomValuesWriter)
	for i := range sw.bloomValuesShards[:] {
		sw.bloomValuesShards[i].init(bloomValuesShards[i])
	}
}

func (sw *streamWriters) totalBytesWritten() uint64 {
	n := uint64(0)

	n += sw.columnNamesWriter.bytesWritten
	n += sw.metaindexWriter.bytesWritten
	n += sw.indexWriter.bytesWritten
	n += sw.columnsHeaderIndexWriter.bytesWritten
	n += sw.columnsHeaderWriter.bytesWritten
	n += sw.timestampsWriter.bytesWritten

	n += sw.messageBloomValuesWriter.totalBytesWritten()
	for i := range sw.bloomValuesShards[:] {
		n += sw.bloomValuesShards[i].totalBytesWritten()
	}

	return n
}

func (sw *streamWriters) MustClose() {
	sw.columnNamesWriter.MustClose()
	sw.metaindexWriter.MustClose()
	sw.indexWriter.MustClose()
	sw.columnsHeaderIndexWriter.MustClose()
	sw.columnsHeaderWriter.MustClose()
	sw.timestampsWriter.MustClose()

	sw.messageBloomValuesWriter.MustClose()
	for i := range sw.bloomValuesShards[:] {
		sw.bloomValuesShards[i].MustClose()
	}
}

func (sw *streamWriters) getBloomValuesWriterForColumnName(name string) *bloomValuesWriter {
	if name == "" {
		return &sw.messageBloomValuesWriter
	}

	h := xxhash.Sum64(bytesutil.ToUnsafeBytes(name))
	idx := h % uint64(len(sw.bloomValuesShards))
	return &sw.bloomValuesShards[idx]
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

	// columnNameIDGenerator is used for generating columnName->id mapping for all the columns seen in bsw
	columnNameIDGenerator columnNameIDGenerator
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

	bsw.columnNameIDGenerator.reset()
}

// MustInitForInmemoryPart initializes bsw from mp
func (bsw *blockStreamWriter) MustInitForInmemoryPart(mp *inmemoryPart) {
	bsw.reset()

	messageBloomValues := mp.messageBloomValues.NewStreamWriter()

	var bloomValuesShards [bloomValuesShardsCount]bloomValuesStreamWriter
	for i := range bloomValuesShards[:] {
		bloomValuesShards[i] = mp.bloomValuesShards[i].NewStreamWriter()
	}

	bsw.streamWriters.init(&mp.columnNames, &mp.metaindex, &mp.index, &mp.columnsHeaderIndex, &mp.columnsHeader, &mp.timestamps, messageBloomValues, bloomValuesShards)
}

// MustInitForFilePart initializes bsw for writing data to file part located at path.
//
// if nocache is true, then the written data doesn't go to OS page cache.
func (bsw *blockStreamWriter) MustInitForFilePart(path string, nocache bool) {
	bsw.reset()

	fs.MustMkdirFailIfExist(path)

	columnNamesPath := filepath.Join(path, columnNamesFilename)
	metaindexPath := filepath.Join(path, metaindexFilename)
	indexPath := filepath.Join(path, indexFilename)
	columnsHeaderIndexPath := filepath.Join(path, columnsHeaderIndexFilename)
	columnsHeaderPath := filepath.Join(path, columnsHeaderFilename)
	timestampsPath := filepath.Join(path, timestampsFilename)

	// Always cache columnNames files, since it is re-read immediately after part creation
	columnNamesWriter := filestream.MustCreate(columnNamesPath, false)

	// Always cache metaindex file, since it is re-read immediately after part creation
	metaindexWriter := filestream.MustCreate(metaindexPath, false)

	indexWriter := filestream.MustCreate(indexPath, nocache)
	columnsHeaderIndexWriter := filestream.MustCreate(columnsHeaderIndexPath, nocache)
	columnsHeaderWriter := filestream.MustCreate(columnsHeaderPath, nocache)
	timestampsWriter := filestream.MustCreate(timestampsPath, nocache)

	messageBloomFilterPath := filepath.Join(path, messageBloomFilename)
	messageValuesPath := filepath.Join(path, messageValuesFilename)
	messageBloomValuesWriter := bloomValuesStreamWriter{
		bloom:  filestream.MustCreate(messageBloomFilterPath, nocache),
		values: filestream.MustCreate(messageValuesPath, nocache),
	}

	var bloomValuesShards [bloomValuesShardsCount]bloomValuesStreamWriter
	for i := range bloomValuesShards[:] {
		shard := &bloomValuesShards[i]

		bloomPath := getBloomFilePath(path, uint64(i))
		shard.bloom = filestream.MustCreate(bloomPath, nocache)

		valuesPath := getValuesFilePath(path, uint64(i))
		shard.values = filestream.MustCreate(valuesPath, nocache)
	}

	bsw.streamWriters.init(columnNamesWriter, metaindexWriter, indexWriter, columnsHeaderIndexWriter, columnsHeaderWriter, timestampsWriter, messageBloomValuesWriter, bloomValuesShards)
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
		b.mustWriteTo(sid, bh, &bsw.streamWriters, &bsw.columnNameIDGenerator)
	} else {
		bd.mustWriteTo(bh, &bsw.streamWriters, &bsw.columnNameIDGenerator)
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
	ph.FormatVersion = partFormatLatestVersion
	ph.UncompressedSizeBytes = bsw.globalUncompressedSizeBytes
	ph.RowsCount = bsw.globalRowsCount
	ph.BlocksCount = bsw.globalBlocksCount
	ph.MinTimestamp = bsw.globalMinTimestamp
	ph.MaxTimestamp = bsw.globalMaxTimestamp

	bsw.mustFlushIndexBlock(bsw.indexBlockData)

	// Write columnNames data
	mustWriteColumnNames(&bsw.streamWriters.columnNamesWriter, bsw.columnNameIDGenerator.columnNames)

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
