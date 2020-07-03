package storage

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// blockStreamReader represents block stream reader.
type blockStreamReader struct {
	// Currently active block.
	Block Block

	// Filesystem path to the stream reader.
	//
	// Is empty for inmemory stream readers.
	path string

	ph partHeader

	// Use io.Reader type for timestampsReader and valuesReader
	// in order to remove I2I conversion in readBlock
	// when passing them to fs.ReadFullData
	timestampsReader io.Reader
	valuesReader     io.Reader

	indexReader filestream.ReadCloser

	mrs []metaindexRow

	// Points the current mr from mrs.
	mr *metaindexRow

	// The total number of rows read so far.
	rowsCount uint64

	// The total number of blocks read so far.
	blocksCount uint64

	// The number of block headers in the current index block.
	indexBlockHeadersCount uint32

	timestampsBlockOffset uint64
	valuesBlockOffset     uint64
	indexBlockOffset      uint64

	indexData           []byte
	compressedIndexData []byte

	// Cursor to indexData.
	indexCursor []byte

	err error
}

func (bsr *blockStreamReader) assertWriteClosers() {
	_ = bsr.timestampsReader.(filestream.ReadCloser)
	_ = bsr.valuesReader.(filestream.ReadCloser)
}

func (bsr *blockStreamReader) reset() {
	bsr.Block.Reset()

	bsr.path = ""

	bsr.ph.Reset()

	bsr.timestampsReader = nil
	bsr.valuesReader = nil
	bsr.indexReader = nil

	bsr.mrs = bsr.mrs[:0]
	bsr.mr = nil

	bsr.rowsCount = 0
	bsr.blocksCount = 0
	bsr.indexBlockHeadersCount = 0

	bsr.timestampsBlockOffset = 0
	bsr.valuesBlockOffset = 0
	bsr.indexBlockOffset = 0

	bsr.indexData = bsr.indexData[:0]
	bsr.compressedIndexData = bsr.compressedIndexData[:0]

	bsr.indexCursor = nil

	bsr.err = nil
}

// String returns human-readable representation of bsr.
func (bsr *blockStreamReader) String() string {
	if len(bsr.path) > 0 {
		return bsr.path
	}
	return bsr.ph.String()
}

// InitFromInmemoryPart initializes bsr from the given mp.
func (bsr *blockStreamReader) InitFromInmemoryPart(mp *inmemoryPart) {
	bsr.reset()

	bsr.ph = mp.ph
	bsr.timestampsReader = mp.timestampsData.NewReader()
	bsr.valuesReader = mp.valuesData.NewReader()
	bsr.indexReader = mp.indexData.NewReader()

	var err error
	bsr.mrs, err = unmarshalMetaindexRows(bsr.mrs[:0], mp.metaindexData.NewReader())
	if err != nil {
		logger.Panicf("BUG: cannot unmarshal metaindex rows from inmemoryPart: %s", err)
	}

	bsr.assertWriteClosers()
}

// InitFromFilePart initializes bsr from a file-based part on the given path.
//
// Files in the part are always read without OS cache pollution,
// since they are usually deleted after the merge.
func (bsr *blockStreamReader) InitFromFilePart(path string) error {
	bsr.reset()

	path = filepath.Clean(path)

	if err := bsr.ph.ParseFromPath(path); err != nil {
		return fmt.Errorf("cannot parse path to part: %w", err)
	}

	timestampsPath := path + "/timestamps.bin"
	timestampsFile, err := filestream.Open(timestampsPath, true)
	if err != nil {
		return fmt.Errorf("cannot open timestamps file in stream mode: %w", err)
	}

	valuesPath := path + "/values.bin"
	valuesFile, err := filestream.Open(valuesPath, true)
	if err != nil {
		timestampsFile.MustClose()
		return fmt.Errorf("cannot open values file in stream mode: %w", err)
	}

	indexPath := path + "/index.bin"
	indexFile, err := filestream.Open(indexPath, true)
	if err != nil {
		timestampsFile.MustClose()
		valuesFile.MustClose()
		return fmt.Errorf("cannot open index file in stream mode: %w", err)
	}

	metaindexPath := path + "/metaindex.bin"
	metaindexFile, err := filestream.Open(metaindexPath, true)
	if err != nil {
		timestampsFile.MustClose()
		valuesFile.MustClose()
		indexFile.MustClose()
		return fmt.Errorf("cannot open metaindex file in stream mode: %w", err)
	}
	mrs, err := unmarshalMetaindexRows(bsr.mrs[:0], metaindexFile)
	metaindexFile.MustClose()
	if err != nil {
		timestampsFile.MustClose()
		valuesFile.MustClose()
		indexFile.MustClose()
		return fmt.Errorf("cannot unmarshal metaindex rows from inmemoryPart: %w", err)
	}

	bsr.path = path
	bsr.timestampsReader = timestampsFile
	bsr.valuesReader = valuesFile
	bsr.indexReader = indexFile
	bsr.mrs = mrs

	bsr.assertWriteClosers()

	return nil
}

// MustClose closes the bsr.
//
// It closes *Reader files passed to Init.
func (bsr *blockStreamReader) MustClose() {
	bsr.timestampsReader.(filestream.ReadCloser).MustClose()
	bsr.valuesReader.(filestream.ReadCloser).MustClose()
	bsr.indexReader.MustClose()

	bsr.reset()
}

// Error returns the last error.
func (bsr *blockStreamReader) Error() error {
	if bsr.err == nil || bsr.err == io.EOF {
		return nil
	}
	return fmt.Errorf("error when reading part %q: %w", bsr, bsr.err)
}

// NextBlock advances bsr to the next block.
func (bsr *blockStreamReader) NextBlock() bool {
	if bsr.err != nil {
		return false
	}

	bsr.Block.Reset()

	err := bsr.readBlock()
	if err == nil {
		if bsr.Block.bh.RowsCount > 0 {
			return true
		}
		bsr.err = fmt.Errorf("invalid block read with zero rows; block=%+v", &bsr.Block)
		return false
	}
	if err == io.EOF {
		bsr.err = io.EOF
		return false
	}

	bsr.err = fmt.Errorf("cannot read next block: %w", err)
	return false
}

func (bsr *blockStreamReader) readBlock() error {
	if len(bsr.indexCursor) == 0 {
		if bsr.mr != nil && bsr.indexBlockHeadersCount != bsr.mr.BlockHeadersCount {
			return fmt.Errorf("invalid number of block headers in the previous index block at offset %d; got %d; want %d",
				bsr.prevIndexBlockOffset(), bsr.indexBlockHeadersCount, bsr.mr.BlockHeadersCount)
		}
		bsr.indexBlockHeadersCount = 0
		if err := bsr.readIndexBlock(); err != nil {
			if err == io.EOF {
				return io.EOF
			}
			return fmt.Errorf("cannot read index block from index data: %w", err)
		}
	}

	// Read block header.
	if len(bsr.indexCursor) < marshaledBlockHeaderSize {
		return fmt.Errorf("too short index data for reading block header at offset %d; got %d bytes; want %d bytes",
			bsr.prevIndexBlockOffset(), len(bsr.indexCursor), marshaledBlockHeaderSize)
	}
	bsr.Block.headerData = append(bsr.Block.headerData[:0], bsr.indexCursor[:marshaledBlockHeaderSize]...)
	bsr.indexCursor = bsr.indexCursor[marshaledBlockHeaderSize:]
	tail, err := bsr.Block.bh.Unmarshal(bsr.Block.headerData)
	if err != nil {
		return fmt.Errorf("cannot parse block header read from index data at offset %d: %w", bsr.prevIndexBlockOffset(), err)
	}
	if len(tail) > 0 {
		return fmt.Errorf("non-empty tail left after parsing block header at offset %d: %x", bsr.prevIndexBlockOffset(), tail)
	}

	bsr.blocksCount++
	if bsr.blocksCount > bsr.ph.BlocksCount {
		return fmt.Errorf("too many blocks found in the block stream; got %d; cannot be bigger than %d", bsr.blocksCount, bsr.ph.BlocksCount)
	}

	// Validate block header.
	bsr.rowsCount += uint64(bsr.Block.bh.RowsCount)
	if bsr.rowsCount > bsr.ph.RowsCount {
		return fmt.Errorf("too many rows found in the block stream; got %d; cannot be bigger than %d", bsr.rowsCount, bsr.ph.RowsCount)
	}
	if bsr.Block.bh.MinTimestamp < bsr.ph.MinTimestamp {
		return fmt.Errorf("invalid MinTimestamp at block header at offset %d; got %d; cannot be smaller than %d",
			bsr.prevIndexBlockOffset(), bsr.Block.bh.MinTimestamp, bsr.ph.MinTimestamp)
	}
	if bsr.Block.bh.MaxTimestamp > bsr.ph.MaxTimestamp {
		return fmt.Errorf("invalid MaxTimestamp at block header at offset %d; got %d; cannot be bigger than %d",
			bsr.prevIndexBlockOffset(), bsr.Block.bh.MaxTimestamp, bsr.ph.MaxTimestamp)
	}
	if bsr.Block.bh.TimestampsBlockOffset != bsr.timestampsBlockOffset {
		return fmt.Errorf("invalid TimestampsBlockOffset at block header at offset %d; got %d; want %d",
			bsr.prevIndexBlockOffset(), bsr.Block.bh.TimestampsBlockOffset, bsr.timestampsBlockOffset)
	}
	if bsr.Block.bh.ValuesBlockOffset != bsr.valuesBlockOffset {
		return fmt.Errorf("invalid ValuesBlockOffset at block header at offset %d; got %d; want %d",
			bsr.prevIndexBlockOffset(), bsr.Block.bh.ValuesBlockOffset, bsr.valuesBlockOffset)
	}

	// Read timestamps data.
	bsr.Block.timestampsData = bytesutil.Resize(bsr.Block.timestampsData, int(bsr.Block.bh.TimestampsBlockSize))
	if err := fs.ReadFullData(bsr.timestampsReader, bsr.Block.timestampsData); err != nil {
		return fmt.Errorf("cannot read timestamps block at offset %d: %w", bsr.timestampsBlockOffset, err)
	}

	// Read values data.
	bsr.Block.valuesData = bytesutil.Resize(bsr.Block.valuesData, int(bsr.Block.bh.ValuesBlockSize))
	if err := fs.ReadFullData(bsr.valuesReader, bsr.Block.valuesData); err != nil {
		return fmt.Errorf("cannot read values block at offset %d: %w", bsr.valuesBlockOffset, err)
	}

	// Update offsets.
	bsr.timestampsBlockOffset += uint64(bsr.Block.bh.TimestampsBlockSize)
	bsr.valuesBlockOffset += uint64(bsr.Block.bh.ValuesBlockSize)
	bsr.indexBlockHeadersCount++

	return nil
}

func (bsr *blockStreamReader) readIndexBlock() error {
	// Go to the next metaindex row.
	if len(bsr.mrs) == 0 {
		return io.EOF
	}
	bsr.mr = &bsr.mrs[0]
	bsr.mrs = bsr.mrs[1:]

	// Validate metaindex row.
	if bsr.indexBlockOffset != bsr.mr.IndexBlockOffset {
		return fmt.Errorf("invalid IndexBlockOffset in metaindex row; got %d; want %d", bsr.mr.IndexBlockOffset, bsr.indexBlockOffset)
	}
	if bsr.mr.MinTimestamp < bsr.ph.MinTimestamp {
		return fmt.Errorf("invalid MinTimesamp in metaindex row; got %d; cannot be smaller than %d", bsr.mr.MinTimestamp, bsr.ph.MinTimestamp)
	}
	if bsr.mr.MaxTimestamp > bsr.ph.MaxTimestamp {
		return fmt.Errorf("invalid MaxTimestamp in metaindex row; got %d; cannot be bigger than %d", bsr.mr.MaxTimestamp, bsr.ph.MaxTimestamp)
	}

	// Read index block.
	bsr.compressedIndexData = bytesutil.Resize(bsr.compressedIndexData, int(bsr.mr.IndexBlockSize))
	if err := fs.ReadFullData(bsr.indexReader, bsr.compressedIndexData); err != nil {
		return fmt.Errorf("cannot read index block from index data at offset %d: %w", bsr.indexBlockOffset, err)
	}
	tmpData, err := encoding.DecompressZSTD(bsr.indexData[:0], bsr.compressedIndexData)
	if err != nil {
		return fmt.Errorf("cannot decompress index block read at offset %d: %w", bsr.indexBlockOffset, err)
	}
	bsr.indexData = tmpData
	bsr.indexCursor = bsr.indexData

	// Update offsets.
	bsr.indexBlockOffset += uint64(bsr.mr.IndexBlockSize)

	return nil
}

func (bsr *blockStreamReader) prevIndexBlockOffset() uint64 {
	return bsr.indexBlockOffset - uint64(bsr.mr.IndexBlockSize)
}

func getBlockStreamReader() *blockStreamReader {
	v := bsrPool.Get()
	if v == nil {
		return &blockStreamReader{}
	}
	return v.(*blockStreamReader)
}

func putBlockStreamReader(bsr *blockStreamReader) {
	bsr.MustClose()
	bsrPool.Put(bsr)
}

var bsrPool sync.Pool
