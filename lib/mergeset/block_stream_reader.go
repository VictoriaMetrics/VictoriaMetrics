package mergeset

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

type blockStreamReader struct {
	// Block contains the current block if Next returned true.
	Block inmemoryBlock

	// isInmemoryBlock is set to true if bsr was initialized with MustInitFromInmemoryBlock().
	isInmemoryBlock bool

	// The index of the current item in the Block, which is returned from CurrItem()
	currItemIdx int

	path string

	// ph contains partHeader for the read part.
	ph partHeader

	// All the metaindexRows.
	// The blockStreamReader doesn't own mrs - it must be alive
	// during the read.
	mrs []metaindexRow

	// The index for the currently processed metaindexRow from mrs.
	mrIdx int

	// Currently processed blockHeaders.
	bhs []blockHeader

	// The index of the currently processed blockHeader.
	bhIdx int

	indexReader filestream.ReadCloser
	itemsReader filestream.ReadCloser
	lensReader  filestream.ReadCloser

	// Contains the current blockHeader.
	bh *blockHeader

	// Contains the current storageBlock.
	sb storageBlock

	// The number of items read so far.
	itemsRead uint64

	// The number of blocks read so far.
	blocksRead uint64

	// Whether the first item in the reader checked with ph.firstItem.
	firstItemChecked bool

	packedBuf   []byte
	unpackedBuf []byte

	// The last error.
	err error
}

func (bsr *blockStreamReader) reset() {
	bsr.Block.Reset()
	bsr.isInmemoryBlock = false
	bsr.currItemIdx = 0
	bsr.path = ""
	bsr.ph.Reset()
	bsr.mrs = nil
	bsr.mrIdx = 0
	bsr.bhs = bsr.bhs[:0]
	bsr.bhIdx = 0

	bsr.indexReader = nil
	bsr.itemsReader = nil
	bsr.lensReader = nil

	bsr.bh = nil
	bsr.sb.Reset()

	bsr.itemsRead = 0
	bsr.blocksRead = 0
	bsr.firstItemChecked = false

	bsr.packedBuf = bsr.packedBuf[:0]
	bsr.unpackedBuf = bsr.unpackedBuf[:0]

	bsr.err = nil
}

func (bsr *blockStreamReader) String() string {
	if len(bsr.path) > 0 {
		return bsr.path
	}
	return bsr.ph.String()
}

// MustInitFromInmemoryBlock initializes bsr from the given ib.
func (bsr *blockStreamReader) MustInitFromInmemoryBlock(ib *inmemoryBlock) {
	bsr.reset()
	bsr.Block.CopyFrom(ib)
	bsr.Block.SortItems()
	bsr.isInmemoryBlock = true
}

// MustInitFromInmemoryPart initializes bsr from the given mp.
func (bsr *blockStreamReader) MustInitFromInmemoryPart(mp *inmemoryPart) {
	bsr.reset()

	var err error
	bsr.mrs, err = unmarshalMetaindexRows(bsr.mrs[:0], mp.metaindexData.NewReader())
	if err != nil {
		logger.Panicf("BUG: cannot unmarshal metaindex rows from inmemory part: %s", err)
	}

	bsr.ph.CopyFrom(&mp.ph)
	bsr.indexReader = mp.indexData.NewReader()
	bsr.itemsReader = mp.itemsData.NewReader()
	bsr.lensReader = mp.lensData.NewReader()

	if bsr.ph.itemsCount <= 0 {
		logger.Panicf("BUG: source inmemoryPart must contain at least a single item")
	}
	if bsr.ph.blocksCount <= 0 {
		logger.Panicf("BUG: source inmemoryPart must contain at least a single block")
	}
}

// MustInitFromFilePart initializes bsr from a file-based part on the given path.
//
// Part files are read without OS cache pollution, since the part is usually
// deleted after the merge.
func (bsr *blockStreamReader) MustInitFromFilePart(path string) {
	bsr.reset()

	path = filepath.Clean(path)

	bsr.ph.MustReadMetadata(path)

	metaindexPath := filepath.Join(path, metaindexFilename)
	metaindexFile := filestream.MustOpen(metaindexPath, true)

	var err error
	bsr.mrs, err = unmarshalMetaindexRows(bsr.mrs[:0], metaindexFile)
	metaindexFile.MustClose()
	if err != nil {
		logger.Panicf("FATAL: cannot unmarshal metaindex rows from file %q: %s", metaindexPath, err)
	}

	indexPath := filepath.Join(path, indexFilename)
	indexFile := filestream.MustOpen(indexPath, true)

	itemsPath := filepath.Join(path, itemsFilename)
	itemsFile := filestream.MustOpen(itemsPath, true)

	lensPath := filepath.Join(path, lensFilename)
	lensFile := filestream.MustOpen(lensPath, true)

	bsr.path = path
	bsr.indexReader = indexFile
	bsr.itemsReader = itemsFile
	bsr.lensReader = lensFile
}

// MustClose closes the bsr.
//
// It closes *Reader files passed to Init.
func (bsr *blockStreamReader) MustClose() {
	if !bsr.isInmemoryBlock {
		bsr.indexReader.MustClose()
		bsr.itemsReader.MustClose()
		bsr.lensReader.MustClose()
	}
	bsr.reset()
}

func (bsr *blockStreamReader) CurrItem() string {
	return bsr.Block.items[bsr.currItemIdx].String(bsr.Block.data)
}

func (bsr *blockStreamReader) Next() bool {
	if bsr.err != nil {
		return false
	}
	if bsr.isInmemoryBlock {
		bsr.err = io.EOF
		return true
	}

	if bsr.bhIdx >= len(bsr.bhs) {
		// The current index block is over. Try reading the next index block.
		if err := bsr.readNextBHS(); err != nil {
			if err == io.EOF {
				// Check the last item.
				b := &bsr.Block
				lastItem := b.items[len(b.items)-1].Bytes(b.data)
				if string(bsr.ph.lastItem) != string(lastItem) {
					err = fmt.Errorf("unexpected last item; got %X; want %X", lastItem, bsr.ph.lastItem)
				}
			} else {
				err = fmt.Errorf("cannot read the next index block: %w", err)
			}
			bsr.err = err
			return false
		}
	}

	bsr.bh = &bsr.bhs[bsr.bhIdx]
	bsr.bhIdx++

	bsr.sb.itemsData = bytesutil.ResizeNoCopyMayOverallocate(bsr.sb.itemsData, int(bsr.bh.itemsBlockSize))
	fs.MustReadData(bsr.itemsReader, bsr.sb.itemsData)

	bsr.sb.lensData = bytesutil.ResizeNoCopyMayOverallocate(bsr.sb.lensData, int(bsr.bh.lensBlockSize))
	fs.MustReadData(bsr.lensReader, bsr.sb.lensData)

	if err := bsr.Block.UnmarshalData(&bsr.sb, bsr.bh.firstItem, bsr.bh.commonPrefix, bsr.bh.itemsCount, bsr.bh.marshalType); err != nil {
		bsr.err = fmt.Errorf("cannot unmarshal inmemoryBlock from storageBlock with firstItem=%X, commonPrefix=%X, itemsCount=%d, marshalType=%d: %w",
			bsr.bh.firstItem, bsr.bh.commonPrefix, bsr.bh.itemsCount, bsr.bh.marshalType, err)
		return false
	}
	bsr.blocksRead++
	if bsr.blocksRead > bsr.ph.blocksCount {
		bsr.err = fmt.Errorf("too many blocks read: %d; must be smaller than partHeader.blocksCount %d", bsr.blocksRead, bsr.ph.blocksCount)
		return false
	}
	bsr.currItemIdx = 0
	bsr.itemsRead += uint64(len(bsr.Block.items))
	if bsr.itemsRead > bsr.ph.itemsCount {
		bsr.err = fmt.Errorf("too many items read: %d; must be smaller than partHeader.itemsCount %d", bsr.itemsRead, bsr.ph.itemsCount)
		return false
	}
	if !bsr.firstItemChecked {
		bsr.firstItemChecked = true
		b := &bsr.Block
		firstItem := b.items[0].Bytes(b.data)
		if string(bsr.ph.firstItem) != string(firstItem) {
			bsr.err = fmt.Errorf("unexpected first item; got %X; want %X", firstItem, bsr.ph.firstItem)
			return false
		}
	}
	return true
}

func (bsr *blockStreamReader) readNextBHS() error {
	if bsr.mrIdx >= len(bsr.mrs) {
		return io.EOF
	}

	mr := &bsr.mrs[bsr.mrIdx]
	bsr.mrIdx++

	// Read compressed index block.
	bsr.packedBuf = bytesutil.ResizeNoCopyMayOverallocate(bsr.packedBuf, int(mr.indexBlockSize))
	fs.MustReadData(bsr.indexReader, bsr.packedBuf)

	// Unpack the compressed index block.
	var err error
	bsr.unpackedBuf, err = encoding.DecompressZSTD(bsr.unpackedBuf[:0], bsr.packedBuf)
	if err != nil {
		return fmt.Errorf("cannot decompress index block: %w", err)
	}

	// Unmarshal the unpacked index block into bsr.bhs.
	bsr.bhs, err = unmarshalBlockHeadersNoCopy(bsr.bhs[:0], bsr.unpackedBuf, int(mr.blockHeadersCount))
	if err != nil {
		return fmt.Errorf("cannot unmarshal blockHeaders in the index block #%d: %w", bsr.mrIdx, err)
	}
	bsr.bhIdx = 0
	return nil
}

func (bsr *blockStreamReader) Error() error {
	if bsr.err == io.EOF {
		return nil
	}
	return bsr.err
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
