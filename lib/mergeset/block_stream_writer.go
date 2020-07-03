package mergeset

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

type blockStreamWriter struct {
	compressLevel int
	path          string

	metaindexWriter filestream.WriteCloser
	indexWriter     filestream.WriteCloser
	itemsWriter     filestream.WriteCloser
	lensWriter      filestream.WriteCloser

	sb storageBlock
	bh blockHeader
	mr metaindexRow

	unpackedIndexBlockBuf []byte
	packedIndexBlockBuf   []byte

	unpackedMetaindexBuf []byte
	packedMetaindexBuf   []byte

	itemsBlockOffset uint64
	lensBlockOffset  uint64
	indexBlockOffset uint64

	// whether the first item for mr has been caught.
	mrFirstItemCaught bool
}

func (bsw *blockStreamWriter) reset() {
	bsw.compressLevel = 0
	bsw.path = ""

	bsw.metaindexWriter = nil
	bsw.indexWriter = nil
	bsw.itemsWriter = nil
	bsw.lensWriter = nil

	bsw.sb.Reset()
	bsw.bh.Reset()
	bsw.mr.Reset()

	bsw.unpackedIndexBlockBuf = bsw.unpackedIndexBlockBuf[:0]
	bsw.packedIndexBlockBuf = bsw.packedIndexBlockBuf[:0]

	bsw.unpackedMetaindexBuf = bsw.unpackedMetaindexBuf[:0]
	bsw.packedMetaindexBuf = bsw.packedMetaindexBuf[:0]

	bsw.itemsBlockOffset = 0
	bsw.lensBlockOffset = 0
	bsw.indexBlockOffset = 0

	bsw.mrFirstItemCaught = false
}

func (bsw *blockStreamWriter) InitFromInmemoryPart(ip *inmemoryPart) {
	bsw.reset()

	// Use the minimum compression level for in-memory blocks,
	// since they are going to be re-compressed during the merge into file-based blocks.
	bsw.compressLevel = -5 // See https://github.com/facebook/zstd/releases/tag/v1.3.4

	bsw.metaindexWriter = &ip.metaindexData
	bsw.indexWriter = &ip.indexData
	bsw.itemsWriter = &ip.itemsData
	bsw.lensWriter = &ip.lensData
}

// InitFromFilePart initializes bsw from a file-based part on the given path.
//
// The bsw doesn't pollute OS page cache if nocache is set.
func (bsw *blockStreamWriter) InitFromFilePart(path string, nocache bool, compressLevel int) error {
	path = filepath.Clean(path)

	// Create the directory
	if err := fs.MkdirAllFailIfExist(path); err != nil {
		return fmt.Errorf("cannot create directory %q: %w", path, err)
	}

	// Create part files in the directory.

	// Always cache metaindex file in OS page cache, since it is immediately
	// read after the merge.
	metaindexPath := path + "/metaindex.bin"
	metaindexFile, err := filestream.Create(metaindexPath, false)
	if err != nil {
		fs.MustRemoveAll(path)
		return fmt.Errorf("cannot create metaindex file: %w", err)
	}

	indexPath := path + "/index.bin"
	indexFile, err := filestream.Create(indexPath, nocache)
	if err != nil {
		metaindexFile.MustClose()
		fs.MustRemoveAll(path)
		return fmt.Errorf("cannot create index file: %w", err)
	}

	itemsPath := path + "/items.bin"
	itemsFile, err := filestream.Create(itemsPath, nocache)
	if err != nil {
		metaindexFile.MustClose()
		indexFile.MustClose()
		fs.MustRemoveAll(path)
		return fmt.Errorf("cannot create items file: %w", err)
	}

	lensPath := path + "/lens.bin"
	lensFile, err := filestream.Create(lensPath, nocache)
	if err != nil {
		metaindexFile.MustClose()
		indexFile.MustClose()
		itemsFile.MustClose()
		fs.MustRemoveAll(path)
		return fmt.Errorf("cannot create lens file: %w", err)
	}

	bsw.reset()
	bsw.compressLevel = compressLevel
	bsw.path = path

	bsw.metaindexWriter = metaindexFile
	bsw.indexWriter = indexFile
	bsw.itemsWriter = itemsFile
	bsw.lensWriter = lensFile

	return nil
}

// MustClose closes the bsw.
//
// It closes *Writer files passed to Init*.
func (bsw *blockStreamWriter) MustClose() {
	// Flush the remaining data.
	bsw.flushIndexData()

	// Compress and write metaindex.
	bsw.packedMetaindexBuf = encoding.CompressZSTDLevel(bsw.packedMetaindexBuf[:0], bsw.unpackedMetaindexBuf, bsw.compressLevel)
	fs.MustWriteData(bsw.metaindexWriter, bsw.packedMetaindexBuf)

	// Close all the writers.
	bsw.metaindexWriter.MustClose()
	bsw.indexWriter.MustClose()
	bsw.itemsWriter.MustClose()
	bsw.lensWriter.MustClose()

	// Sync bsw.path contents to make sure it doesn't disappear
	// after system crash or power loss.
	if bsw.path != "" {
		fs.MustSyncPath(bsw.path)
	}

	bsw.reset()
}

// WriteBlock writes ib to bsw.
//
// ib must be sorted.
func (bsw *blockStreamWriter) WriteBlock(ib *inmemoryBlock) {
	bsw.bh.firstItem, bsw.bh.commonPrefix, bsw.bh.itemsCount, bsw.bh.marshalType = ib.MarshalSortedData(&bsw.sb, bsw.bh.firstItem[:0], bsw.bh.commonPrefix[:0], bsw.compressLevel)

	if !bsw.mrFirstItemCaught {
		bsw.mr.firstItem = append(bsw.mr.firstItem[:0], bsw.bh.firstItem...)
		bsw.mrFirstItemCaught = true
	}

	// Write itemsData
	fs.MustWriteData(bsw.itemsWriter, bsw.sb.itemsData)
	bsw.bh.itemsBlockSize = uint32(len(bsw.sb.itemsData))
	bsw.bh.itemsBlockOffset = bsw.itemsBlockOffset
	bsw.itemsBlockOffset += uint64(bsw.bh.itemsBlockSize)

	// Write lensData
	fs.MustWriteData(bsw.lensWriter, bsw.sb.lensData)
	bsw.bh.lensBlockSize = uint32(len(bsw.sb.lensData))
	bsw.bh.lensBlockOffset = bsw.lensBlockOffset
	bsw.lensBlockOffset += uint64(bsw.bh.lensBlockSize)

	// Write blockHeader
	bsw.unpackedIndexBlockBuf = bsw.bh.Marshal(bsw.unpackedIndexBlockBuf)
	bsw.bh.Reset()
	bsw.mr.blockHeadersCount++
	if len(bsw.unpackedIndexBlockBuf) >= maxIndexBlockSize {
		bsw.flushIndexData()
	}
}

// The maximum size of index block with multiple blockHeaders.
const maxIndexBlockSize = 64 * 1024

func (bsw *blockStreamWriter) flushIndexData() {
	if len(bsw.unpackedIndexBlockBuf) == 0 {
		// Nothing to flush.
		return
	}

	// Write indexBlock.
	bsw.packedIndexBlockBuf = encoding.CompressZSTDLevel(bsw.packedIndexBlockBuf[:0], bsw.unpackedIndexBlockBuf, bsw.compressLevel)
	fs.MustWriteData(bsw.indexWriter, bsw.packedIndexBlockBuf)
	bsw.mr.indexBlockSize = uint32(len(bsw.packedIndexBlockBuf))
	bsw.mr.indexBlockOffset = bsw.indexBlockOffset
	bsw.indexBlockOffset += uint64(bsw.mr.indexBlockSize)
	bsw.unpackedIndexBlockBuf = bsw.unpackedIndexBlockBuf[:0]

	// Write metaindexRow.
	bsw.unpackedMetaindexBuf = bsw.mr.Marshal(bsw.unpackedMetaindexBuf)
	bsw.mr.Reset()

	// Notify that the next call to WriteBlock must catch the first item.
	bsw.mrFirstItemCaught = false
}

func getBlockStreamWriter() *blockStreamWriter {
	v := bswPool.Get()
	if v == nil {
		return &blockStreamWriter{}
	}
	return v.(*blockStreamWriter)
}

func putBlockStreamWriter(bsw *blockStreamWriter) {
	bsw.reset()
	bswPool.Put(bsw)
}

var bswPool sync.Pool
