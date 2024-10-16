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

type readerWithStats struct {
	r         filestream.ReadCloser
	bytesRead uint64
}

func (r *readerWithStats) reset() {
	r.r = nil
	r.bytesRead = 0
}

func (r *readerWithStats) init(rc filestream.ReadCloser) {
	r.reset()

	r.r = rc
}

// Path returns the path to r file
func (r *readerWithStats) Path() string {
	return r.r.Path()
}

// MustReadFull reads len(data) to r.
func (r *readerWithStats) MustReadFull(data []byte) {
	fs.MustReadData(r.r, data)
	r.bytesRead += uint64(len(data))
}

func (r *readerWithStats) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.bytesRead += uint64(n)
	return n, err
}

func (r *readerWithStats) MustClose() {
	if r.r != nil {
		r.r.MustClose()
		r.r = nil
	}
}

// streamReaders contains readers for blockStreamReader
type streamReaders struct {
	columnNamesReader        readerWithStats
	metaindexReader          readerWithStats
	indexReader              readerWithStats
	columnsHeaderIndexReader readerWithStats
	columnsHeaderReader      readerWithStats
	timestampsReader         readerWithStats

	messageBloomValuesReader bloomValuesReader
	oldBloomValuesReader     bloomValuesReader
	bloomValuesShards        [bloomValuesShardsCount]bloomValuesReader
}

type bloomValuesReader struct {
	bloom  readerWithStats
	values readerWithStats
}

func (r *bloomValuesReader) reset() {
	r.bloom.reset()
	r.values.reset()
}

func (r *bloomValuesReader) init(sr bloomValuesStreamReader) {
	r.bloom.init(sr.bloom)
	r.values.init(sr.values)
}

func (r *bloomValuesReader) totalBytesRead() uint64 {
	return r.bloom.bytesRead + r.values.bytesRead
}

func (r *bloomValuesReader) MustClose() {
	r.bloom.MustClose()
	r.values.MustClose()
}

type bloomValuesStreamReader struct {
	bloom  filestream.ReadCloser
	values filestream.ReadCloser
}

func (sr *streamReaders) reset() {
	sr.columnNamesReader.reset()
	sr.metaindexReader.reset()
	sr.indexReader.reset()
	sr.columnsHeaderIndexReader.reset()
	sr.columnsHeaderReader.reset()
	sr.timestampsReader.reset()

	sr.messageBloomValuesReader.reset()
	sr.oldBloomValuesReader.reset()
	for i := range sr.bloomValuesShards[:] {
		sr.bloomValuesShards[i].reset()
	}
}

func (sr *streamReaders) init(columnNamesReader, metaindexReader, indexReader, columnsHeaderIndexReader, columnsHeaderReader, timestampsReader filestream.ReadCloser,
	messageBloomValuesReader, oldBloomValuesReader bloomValuesStreamReader, bloomValuesShards [bloomValuesShardsCount]bloomValuesStreamReader,
) {
	sr.columnNamesReader.init(columnNamesReader)
	sr.metaindexReader.init(metaindexReader)
	sr.indexReader.init(indexReader)
	sr.columnsHeaderIndexReader.init(columnsHeaderIndexReader)
	sr.columnsHeaderReader.init(columnsHeaderReader)
	sr.timestampsReader.init(timestampsReader)

	sr.messageBloomValuesReader.init(messageBloomValuesReader)
	sr.oldBloomValuesReader.init(oldBloomValuesReader)
	for i := range sr.bloomValuesShards[:] {
		sr.bloomValuesShards[i].init(bloomValuesShards[i])
	}
}

func (sr *streamReaders) totalBytesRead() uint64 {
	n := uint64(0)

	n += sr.columnNamesReader.bytesRead
	n += sr.metaindexReader.bytesRead
	n += sr.indexReader.bytesRead
	n += sr.columnsHeaderIndexReader.bytesRead
	n += sr.columnsHeaderReader.bytesRead
	n += sr.timestampsReader.bytesRead

	n += sr.messageBloomValuesReader.totalBytesRead()
	n += sr.oldBloomValuesReader.totalBytesRead()
	for i := range sr.bloomValuesShards[:] {
		n += sr.bloomValuesShards[i].totalBytesRead()
	}

	return n
}

func (sr *streamReaders) MustClose() {
	sr.columnNamesReader.MustClose()
	sr.metaindexReader.MustClose()
	sr.indexReader.MustClose()
	sr.columnsHeaderIndexReader.MustClose()
	sr.columnsHeaderReader.MustClose()
	sr.timestampsReader.MustClose()

	sr.messageBloomValuesReader.MustClose()
	sr.oldBloomValuesReader.MustClose()
	for i := range sr.bloomValuesShards[:] {
		sr.bloomValuesShards[i].MustClose()
	}
}

func (sr *streamReaders) getBloomValuesReaderForColumnName(name string, partFormatVersion uint) *bloomValuesReader {
	if name == "" {
		return &sr.messageBloomValuesReader
	}
	if partFormatVersion < 1 {
		return &sr.oldBloomValuesReader
	}

	h := xxhash.Sum64(bytesutil.ToUnsafeBytes(name))
	idx := h % uint64(len(sr.bloomValuesShards))
	return &sr.bloomValuesShards[idx]
}

// blockStreamReader is used for reading blocks in streaming manner from a part.
type blockStreamReader struct {
	// blockData contains the data for the last read block
	blockData blockData

	// a contains data for blockData
	a arena

	// ph is the header for the part
	ph partHeader

	// streamReaders contains data readers in stream mode
	streamReaders streamReaders

	// columnNameIDs contains columnName->id mapping for all the column names seen in the part
	columnNameIDs map[string]uint64

	// columnNames constains id->columnName mapping for all the columns seen in the part
	columnNames []string

	// indexBlockHeaders contains the list of all the indexBlockHeader entries for the part
	indexBlockHeaders []indexBlockHeader

	// blockHeaders contains the list of blockHeader entries for the current indexBlockHeader pointed by nextIndexBlockIdx
	blockHeaders []blockHeader

	// nextIndexBlockIdx is the index of the next item to read from indexBlockHeaders
	nextIndexBlockIdx int

	// nextBlockIdx is the index of the next item to read from blockHeaders
	nextBlockIdx int

	// globalUncompressedSizeBytes is the total size of log entries seen in the part
	globalUncompressedSizeBytes uint64

	// globalRowsCount is the number of log entries seen in the part
	globalRowsCount uint64

	// globalBlocksCount is the number of blocks seen in the part
	globalBlocksCount uint64

	// sidLast is the stream id for the previously read block
	sidLast streamID

	// minTimestampLast is the minimum timestamp for the previously read block
	minTimestampLast int64
}

// reset resets bsr, so it can be re-used
func (bsr *blockStreamReader) reset() {
	bsr.blockData.reset()
	bsr.a.reset()
	bsr.ph.reset()
	bsr.streamReaders.reset()

	bsr.columnNameIDs = nil
	bsr.columnNames = nil

	ihs := bsr.indexBlockHeaders
	if len(ihs) > 10e3 {
		// The ihs len is unbound, so it is better to drop too long indexBlockHeaders in order to reduce memory usage
		ihs = nil
	}
	for i := range ihs {
		ihs[i].reset()
	}
	bsr.indexBlockHeaders = ihs[:0]

	bhs := bsr.blockHeaders
	for i := range bhs {
		bhs[i].reset()
	}
	bsr.blockHeaders = bhs[:0]

	bsr.nextIndexBlockIdx = 0
	bsr.nextBlockIdx = 0
	bsr.globalUncompressedSizeBytes = 0
	bsr.globalRowsCount = 0
	bsr.globalBlocksCount = 0

	bsr.sidLast.reset()
	bsr.minTimestampLast = 0
}

// Path returns part path for bsr (e.g. file path, url or in-memory reference)
func (bsr *blockStreamReader) Path() string {
	path := bsr.streamReaders.metaindexReader.Path()
	return filepath.Dir(path)
}

// MustInitFromInmemoryPart initializes bsr from mp.
func (bsr *blockStreamReader) MustInitFromInmemoryPart(mp *inmemoryPart) {
	bsr.reset()

	bsr.ph = mp.ph

	// Initialize streamReaders
	columnNamesReader := mp.columnNames.NewReader()
	metaindexReader := mp.metaindex.NewReader()
	indexReader := mp.index.NewReader()
	columnsHeaderIndexReader := mp.columnsHeaderIndex.NewReader()
	columnsHeaderReader := mp.columnsHeader.NewReader()
	timestampsReader := mp.timestamps.NewReader()

	messageBloomValuesReader := mp.messageBloomValues.NewStreamReader()
	var oldBloomValuesReader bloomValuesStreamReader
	var bloomValuesShards [bloomValuesShardsCount]bloomValuesStreamReader
	for i := range bloomValuesShards[:] {
		bloomValuesShards[i] = mp.bloomValuesShards[i].NewStreamReader()
	}

	bsr.streamReaders.init(columnNamesReader, metaindexReader, indexReader, columnsHeaderIndexReader, columnsHeaderReader, timestampsReader,
		messageBloomValuesReader, oldBloomValuesReader, bloomValuesShards)

	// Read columnNames data
	bsr.columnNames, bsr.columnNameIDs = mustReadColumnNames(&bsr.streamReaders.columnNamesReader)

	// Read metaindex data
	bsr.indexBlockHeaders = mustReadIndexBlockHeaders(bsr.indexBlockHeaders[:0], &bsr.streamReaders.metaindexReader)
}

// MustInitFromFilePart initializes bsr from file part at the given path.
func (bsr *blockStreamReader) MustInitFromFilePart(path string) {
	bsr.reset()

	// Files in the part are always read without OS cache pollution,
	// since they are usually deleted after the merge.
	const nocache = true

	bsr.ph.mustReadMetadata(path)

	columnNamesPath := filepath.Join(path, columnNamesFilename)
	metaindexPath := filepath.Join(path, metaindexFilename)
	indexPath := filepath.Join(path, indexFilename)
	columnsHeaderIndexPath := filepath.Join(path, columnsHeaderIndexFilename)
	columnsHeaderPath := filepath.Join(path, columnsHeaderFilename)
	timestampsPath := filepath.Join(path, timestampsFilename)

	// Open data readers
	var columnNamesReader filestream.ReadCloser
	if bsr.ph.FormatVersion >= 1 {
		columnNamesReader = filestream.MustOpen(columnNamesPath, nocache)
	}
	metaindexReader := filestream.MustOpen(metaindexPath, nocache)
	indexReader := filestream.MustOpen(indexPath, nocache)
	var columnsHeaderIndexReader filestream.ReadCloser
	if bsr.ph.FormatVersion >= 1 {
		columnsHeaderIndexReader = filestream.MustOpen(columnsHeaderIndexPath, nocache)
	}
	columnsHeaderReader := filestream.MustOpen(columnsHeaderPath, nocache)
	timestampsReader := filestream.MustOpen(timestampsPath, nocache)

	messageBloomFilterPath := filepath.Join(path, messageBloomFilename)
	messageValuesPath := filepath.Join(path, messageValuesFilename)
	messageBloomValuesReader := bloomValuesStreamReader{
		bloom:  filestream.MustOpen(messageBloomFilterPath, nocache),
		values: filestream.MustOpen(messageValuesPath, nocache),
	}
	var oldBloomValuesReader bloomValuesStreamReader
	var bloomValuesShards [bloomValuesShardsCount]bloomValuesStreamReader
	if bsr.ph.FormatVersion < 1 {
		bloomPath := filepath.Join(path, oldBloomFilename)
		oldBloomValuesReader.bloom = filestream.MustOpen(bloomPath, nocache)

		valuesPath := filepath.Join(path, oldValuesFilename)
		oldBloomValuesReader.values = filestream.MustOpen(valuesPath, nocache)
	} else {
		for i := range bloomValuesShards[:] {
			shard := &bloomValuesShards[i]

			bloomPath := getBloomFilePath(path, uint64(i))
			shard.bloom = filestream.MustOpen(bloomPath, nocache)

			valuesPath := getValuesFilePath(path, uint64(i))
			shard.values = filestream.MustOpen(valuesPath, nocache)
		}
	}

	// Initialize streamReaders
	bsr.streamReaders.init(columnNamesReader, metaindexReader, indexReader, columnsHeaderIndexReader, columnsHeaderReader, timestampsReader,
		messageBloomValuesReader, oldBloomValuesReader, bloomValuesShards)

	if bsr.ph.FormatVersion >= 1 {
		// Read columnNames data
		bsr.columnNames, bsr.columnNameIDs = mustReadColumnNames(&bsr.streamReaders.columnNamesReader)
	}

	// Read metaindex data
	bsr.indexBlockHeaders = mustReadIndexBlockHeaders(bsr.indexBlockHeaders[:0], &bsr.streamReaders.metaindexReader)
}

// NextBlock reads the next block from bsr and puts it into bsr.blockData.
//
// false is returned if there are no other blocks.
//
// bsr.blockData is valid until the next call to NextBlock().
func (bsr *blockStreamReader) NextBlock() bool {
	for bsr.nextBlockIdx >= len(bsr.blockHeaders) {
		if !bsr.nextIndexBlock() {
			return false
		}
	}
	ih := &bsr.indexBlockHeaders[bsr.nextIndexBlockIdx-1]
	bh := &bsr.blockHeaders[bsr.nextBlockIdx]
	th := &bh.timestampsHeader

	// Validate bh
	if bh.streamID.less(&bsr.sidLast) {
		logger.Panicf("FATAL: %s: blockHeader.streamID=%s cannot be smaller than the streamID from the previously read block: %s", bsr.Path(), &bh.streamID, &bsr.sidLast)
	}
	if bh.streamID.equal(&bsr.sidLast) && th.minTimestamp < bsr.minTimestampLast {
		logger.Panicf("FATAL: %s: timestamps.minTimestamp=%d cannot be smaller than the minTimestamp for the previously read block for the same streamID: %d",
			bsr.Path(), th.minTimestamp, bsr.minTimestampLast)
	}
	bsr.minTimestampLast = th.minTimestamp
	bsr.sidLast = bh.streamID
	if th.minTimestamp < ih.minTimestamp {
		logger.Panicf("FATAL: %s: timestampsHeader.minTimestamp=%d cannot be smaller than indexBlockHeader.minTimestamp=%d", bsr.Path(), th.minTimestamp, ih.minTimestamp)
	}
	if th.maxTimestamp > ih.maxTimestamp {
		logger.Panicf("FATAL: %s: timestampsHeader.maxTimestamp=%d cannot be bigger than indexBlockHeader.maxTimestamp=%d", bsr.Path(), th.maxTimestamp, ih.minTimestamp)
	}

	// Read bsr.blockData
	bsr.a.reset()
	bsr.blockData.mustReadFrom(&bsr.a, bh, &bsr.streamReaders, bsr.ph.FormatVersion, bsr.columnNames)

	bsr.globalUncompressedSizeBytes += bh.uncompressedSizeBytes
	bsr.globalRowsCount += bh.rowsCount
	bsr.globalBlocksCount++
	if bsr.globalUncompressedSizeBytes > bsr.ph.UncompressedSizeBytes {
		logger.Panicf("FATAL: %s: too big size of entries read: %d; mustn't exceed partHeader.UncompressedSizeBytes=%d",
			bsr.Path(), bsr.globalUncompressedSizeBytes, bsr.ph.UncompressedSizeBytes)
	}
	if bsr.globalRowsCount > bsr.ph.RowsCount {
		logger.Panicf("FATAL: %s: too many log entries read so far: %d; mustn't exceed partHeader.RowsCount=%d", bsr.Path(), bsr.globalRowsCount, bsr.ph.RowsCount)
	}
	if bsr.globalBlocksCount > bsr.ph.BlocksCount {
		logger.Panicf("FATAL: %s: too many blocks read so far: %d; mustn't exceed partHeader.BlocksCount=%d", bsr.Path(), bsr.globalBlocksCount, bsr.ph.BlocksCount)
	}

	// The block has been sucessfully read
	bsr.nextBlockIdx++
	return true
}

func (bsr *blockStreamReader) nextIndexBlock() bool {
	// Advance to the next indexBlockHeader
	if bsr.nextIndexBlockIdx >= len(bsr.indexBlockHeaders) {
		// No more blocks left
		// Validate bsr.ph
		totalBytesRead := bsr.streamReaders.totalBytesRead()
		if bsr.ph.CompressedSizeBytes != totalBytesRead {
			logger.Panicf("FATAL: %s: partHeader.CompressedSizeBytes=%d must match the size of data read: %d", bsr.Path(), bsr.ph.CompressedSizeBytes, totalBytesRead)
		}
		if bsr.ph.UncompressedSizeBytes != bsr.globalUncompressedSizeBytes {
			logger.Panicf("FATAL: %s: partHeader.UncompressedSizeBytes=%d must match the size of entries read: %d",
				bsr.Path(), bsr.ph.UncompressedSizeBytes, bsr.globalUncompressedSizeBytes)
		}
		if bsr.ph.RowsCount != bsr.globalRowsCount {
			logger.Panicf("FATAL: %s: partHeader.RowsCount=%d must match the number of log entries read: %d", bsr.Path(), bsr.ph.RowsCount, bsr.globalRowsCount)
		}
		if bsr.ph.BlocksCount != bsr.globalBlocksCount {
			logger.Panicf("FATAL: %s: partHeader.BlocksCount=%d must match the number of blocks read: %d", bsr.Path(), bsr.ph.BlocksCount, bsr.globalBlocksCount)
		}
		return false
	}
	ih := &bsr.indexBlockHeaders[bsr.nextIndexBlockIdx]

	// Validate ih
	metaindexReader := &bsr.streamReaders.metaindexReader
	if ih.minTimestamp < bsr.ph.MinTimestamp {
		logger.Panicf("FATAL: %s: indexBlockHeader.minTimestamp=%d cannot be smaller than partHeader.MinTimestamp=%d",
			metaindexReader.Path(), ih.minTimestamp, bsr.ph.MinTimestamp)
	}
	if ih.maxTimestamp > bsr.ph.MaxTimestamp {
		logger.Panicf("FATAL: %s: indexBlockHeader.maxTimestamp=%d cannot be bigger than partHeader.MaxTimestamp=%d",
			metaindexReader.Path(), ih.maxTimestamp, bsr.ph.MaxTimestamp)
	}

	// Read indexBlock for the given ih
	bb := longTermBufPool.Get()
	bb.B = ih.mustReadNextIndexBlock(bb.B[:0], &bsr.streamReaders)
	bsr.blockHeaders = resetBlockHeaders(bsr.blockHeaders)
	var err error
	bsr.blockHeaders, err = unmarshalBlockHeaders(bsr.blockHeaders[:0], bb.B, bsr.ph.FormatVersion)
	longTermBufPool.Put(bb)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot unmarshal blockHeader entries: %s", bsr.streamReaders.indexReader.Path(), err)
	}

	bsr.nextIndexBlockIdx++
	bsr.nextBlockIdx = 0
	return true
}

// MustClose closes bsr.
func (bsr *blockStreamReader) MustClose() {
	bsr.streamReaders.MustClose()
	bsr.reset()
}

// getBlockStreamReader returns blockStreamReader.
//
// The returned blockStreamReader must be initialized with MustInit().
// call putBlockStreamReader() when the retruend blockStreamReader is no longer needed.
func getBlockStreamReader() *blockStreamReader {
	v := blockStreamReaderPool.Get()
	if v == nil {
		v = &blockStreamReader{}
	}
	bsr := v.(*blockStreamReader)
	return bsr
}

// putBlockStreamReader returns bsr to the pool.
//
// bsr cannot be used after returning to the pool.
func putBlockStreamReader(bsr *blockStreamReader) {
	bsr.reset()
	blockStreamReaderPool.Put(bsr)
}

var blockStreamReaderPool sync.Pool

// mustCloseBlockStreamReaders calls MustClose() on the given bsrs.
func mustCloseBlockStreamReaders(bsrs []*blockStreamReader) {
	for _, bsr := range bsrs {
		bsr.MustClose()
	}
}
