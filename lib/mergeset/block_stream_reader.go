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

	blockItemIdx int

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
	bsr.blockItemIdx = 0
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

// InitFromInmemoryPart initializes bsr from the given ip.
func (bsr *blockStreamReader) InitFromInmemoryPart(ip *inmemoryPart) {
	bsr.reset()

	var err error
	bsr.mrs, err = unmarshalMetaindexRows(bsr.mrs[:0], ip.metaindexData.NewReader())
	if err != nil {
		logger.Panicf("BUG: cannot unmarshal metaindex rows from inmemory part: %s", err)
	}

	bsr.ph.CopyFrom(&ip.ph)
	bsr.indexReader = ip.indexData.NewReader()
	bsr.itemsReader = ip.itemsData.NewReader()
	bsr.lensReader = ip.lensData.NewReader()

	if bsr.ph.itemsCount <= 0 {
		logger.Panicf("BUG: source inmemoryPart must contain at least a single item")
	}
	if bsr.ph.blocksCount <= 0 {
		logger.Panicf("BUG: source inmemoryPart must contain at least a single block")
	}
}

// InitFromFilePart initializes bsr from a file-based part on the given path.
//
// Part files are read without OS cache pollution, since the part is usually
// deleted after the merge.
func (bsr *blockStreamReader) InitFromFilePart(path string) error {
	bsr.reset()

	path = filepath.Clean(path)

	if err := bsr.ph.ParseFromPath(path); err != nil {
		return fmt.Errorf("cannot parse partHeader data from %q: %w", path, err)
	}

	metaindexPath := path + "/metaindex.bin"
	metaindexFile, err := filestream.Open(metaindexPath, true)
	if err != nil {
		return fmt.Errorf("cannot open metaindex file in stream mode: %w", err)
	}
	bsr.mrs, err = unmarshalMetaindexRows(bsr.mrs[:0], metaindexFile)
	metaindexFile.MustClose()
	if err != nil {
		return fmt.Errorf("cannot unmarshal metaindex rows from file %q: %w", metaindexPath, err)
	}

	indexPath := path + "/index.bin"
	indexFile, err := filestream.Open(indexPath, true)
	if err != nil {
		return fmt.Errorf("cannot open index file in stream mode: %w", err)
	}

	itemsPath := path + "/items.bin"
	itemsFile, err := filestream.Open(itemsPath, true)
	if err != nil {
		indexFile.MustClose()
		return fmt.Errorf("cannot open items file in stream mode: %w", err)
	}

	lensPath := path + "/lens.bin"
	lensFile, err := filestream.Open(lensPath, true)
	if err != nil {
		indexFile.MustClose()
		itemsFile.MustClose()
		return fmt.Errorf("cannot open lens file in stream mode: %w", err)
	}

	bsr.path = path
	bsr.indexReader = indexFile
	bsr.itemsReader = itemsFile
	bsr.lensReader = lensFile

	return nil
}

// MustClose closes the bsr.
//
// It closes *Reader files passed to Init.
func (bsr *blockStreamReader) MustClose() {
	bsr.indexReader.MustClose()
	bsr.itemsReader.MustClose()
	bsr.lensReader.MustClose()

	bsr.reset()
}

func (bsr *blockStreamReader) Next() bool {
	if bsr.err != nil {
		return false
	}

	if bsr.bhIdx >= len(bsr.bhs) {
		// The current index block is over. Try reading the next index block.
		if err := bsr.readNextBHS(); err != nil {
			if err == io.EOF {
				// Check the last item.
				lastItem := bsr.Block.items[len(bsr.Block.items)-1]
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

	bsr.sb.itemsData = bytesutil.Resize(bsr.sb.itemsData, int(bsr.bh.itemsBlockSize))
	if err := fs.ReadFullData(bsr.itemsReader, bsr.sb.itemsData); err != nil {
		bsr.err = fmt.Errorf("cannot read compressed items block with size %d: %w", bsr.bh.itemsBlockSize, err)
		return false
	}

	bsr.sb.lensData = bytesutil.Resize(bsr.sb.lensData, int(bsr.bh.lensBlockSize))
	if err := fs.ReadFullData(bsr.lensReader, bsr.sb.lensData); err != nil {
		bsr.err = fmt.Errorf("cannot read compressed lens block with size %d: %w", bsr.bh.lensBlockSize, err)
		return false
	}

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
	bsr.blockItemIdx = 0
	bsr.itemsRead += uint64(len(bsr.Block.items))
	if bsr.itemsRead > bsr.ph.itemsCount {
		bsr.err = fmt.Errorf("too many items read: %d; must be smaller than partHeader.itemsCount %d", bsr.itemsRead, bsr.ph.itemsCount)
		return false
	}
	if !bsr.firstItemChecked {
		bsr.firstItemChecked = true
		if string(bsr.ph.firstItem) != string(bsr.Block.items[0]) {
			bsr.err = fmt.Errorf("unexpected first item; got %X; want %X", bsr.Block.items[0], bsr.ph.firstItem)
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
	bsr.packedBuf = bytesutil.Resize(bsr.packedBuf, int(mr.indexBlockSize))
	if err := fs.ReadFullData(bsr.indexReader, bsr.packedBuf); err != nil {
		return fmt.Errorf("cannot read compressed index block with size %d: %w", mr.indexBlockSize, err)
	}

	// Unpack the compressed index block.
	var err error
	bsr.unpackedBuf, err = encoding.DecompressZSTD(bsr.unpackedBuf[:0], bsr.packedBuf)
	if err != nil {
		return fmt.Errorf("cannot decompress index block with size %d: %w", mr.indexBlockSize, err)
	}

	// Unmarshal the unpacked index block into bsr.bhs.
	if n := int(mr.blockHeadersCount) - cap(bsr.bhs); n > 0 {
		bsr.bhs = append(bsr.bhs[:cap(bsr.bhs)], make([]blockHeader, n)...)
	}
	bsr.bhs = bsr.bhs[:mr.blockHeadersCount]
	bsr.bhIdx = 0
	b := bsr.unpackedBuf
	for i := 0; i < int(mr.blockHeadersCount); i++ {
		tail, err := bsr.bhs[i].Unmarshal(b)
		if err != nil {
			return fmt.Errorf("cannot unmarshal blockHeader #%d in the index block #%d: %w", len(bsr.bhs), bsr.mrIdx, err)
		}
		b = tail
	}
	if len(b) > 0 {
		return fmt.Errorf("unexpected non-empty tail left after unmarshaling block headers; len(tail)=%d", len(b))
	}
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
