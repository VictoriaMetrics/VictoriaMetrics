package mergeset

import (
	"path/filepath"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type inmemoryPart struct {
	ph partHeader
	bh blockHeader
	mr metaindexRow

	metaindexData bytesutil.ByteBuffer
	indexData     bytesutil.ByteBuffer
	itemsData     bytesutil.ByteBuffer
	lensData      bytesutil.ByteBuffer
}

func (mp *inmemoryPart) Reset() {
	mp.ph.Reset()
	mp.bh.Reset()
	mp.mr.Reset()

	mp.metaindexData.Reset()
	mp.indexData.Reset()
	mp.itemsData.Reset()
	mp.lensData.Reset()
}

// MustStoreToDisk stores mp to the given path on disk.
func (mp *inmemoryPart) MustStoreToDisk(path string) {
	fs.MustMkdirFailIfExist(path)

	metaindexPath := filepath.Join(path, metaindexFilename)
	fs.MustWriteSync(metaindexPath, mp.metaindexData.B)

	indexPath := filepath.Join(path, indexFilename)
	fs.MustWriteSync(indexPath, mp.indexData.B)

	itemsPath := filepath.Join(path, itemsFilename)
	fs.MustWriteSync(itemsPath, mp.itemsData.B)

	lensPath := filepath.Join(path, lensFilename)
	fs.MustWriteSync(lensPath, mp.lensData.B)

	mp.ph.MustWriteMetadata(path)

	fs.MustSyncPath(path)
	// Do not sync parent directory - it must be synced by the caller.
}

// Init initializes mp from ib.
func (mp *inmemoryPart) Init(ib *inmemoryBlock) {
	mp.Reset()

	// Re-use mp.itemsData and mp.lensData in sb.
	// This eliminates copying itemsData and lensData from sb to mp later.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2247
	sb := &storageBlock{}
	sb.itemsData = mp.itemsData.B[:0]
	sb.lensData = mp.lensData.B[:0]

	// Use the minimum possible compressLevel for compressing inmemoryPart,
	// since it will be merged into file part soon.
	// See https://github.com/facebook/zstd/releases/tag/v1.3.4 for details about negative compression level
	compressLevel := -5
	mp.bh.firstItem, mp.bh.commonPrefix, mp.bh.itemsCount, mp.bh.marshalType = ib.MarshalUnsortedData(sb, mp.bh.firstItem[:0], mp.bh.commonPrefix[:0], compressLevel)

	mp.ph.itemsCount = uint64(len(ib.items))
	mp.ph.blocksCount = 1
	mp.ph.firstItem = append(mp.ph.firstItem[:0], ib.items[0].String(ib.data)...)
	mp.ph.lastItem = append(mp.ph.lastItem[:0], ib.items[len(ib.items)-1].String(ib.data)...)

	mp.itemsData.B = sb.itemsData
	mp.bh.itemsBlockOffset = 0
	mp.bh.itemsBlockSize = uint32(len(mp.itemsData.B))

	mp.lensData.B = sb.lensData
	mp.bh.lensBlockOffset = 0
	mp.bh.lensBlockSize = uint32(len(mp.lensData.B))

	bb := inmemoryPartBytePool.Get()
	bb.B = mp.bh.Marshal(bb.B[:0])
	if len(bb.B) > 3*maxIndexBlockSize {
		// marshaled blockHeader can exceed indexBlockSize when firstItem and commonPrefix sizes are close to indexBlockSize
		logger.Panicf("BUG: too big index block: %d bytes; mustn't exceed %d bytes", len(bb.B), 3*maxIndexBlockSize)
	}
	mp.indexData.B = encoding.CompressZSTDLevel(mp.indexData.B[:0], bb.B, compressLevel)

	mp.mr.firstItem = append(mp.mr.firstItem[:0], mp.bh.firstItem...)
	mp.mr.blockHeadersCount = 1
	mp.mr.indexBlockOffset = 0
	mp.mr.indexBlockSize = uint32(len(mp.indexData.B))
	bb.B = mp.mr.Marshal(bb.B[:0])
	mp.metaindexData.B = encoding.CompressZSTDLevel(mp.metaindexData.B[:0], bb.B, compressLevel)
	inmemoryPartBytePool.Put(bb)
}

var inmemoryPartBytePool bytesutil.ByteBufferPool

// It is safe calling NewPart multiple times.
// It is unsafe re-using mp while the returned part is in use.
func (mp *inmemoryPart) NewPart() *part {
	size := mp.size()
	p := newPart(&mp.ph, "", size, mp.metaindexData.NewReader(), &mp.indexData, &mp.itemsData, &mp.lensData)
	return p
}

func (mp *inmemoryPart) size() uint64 {
	return uint64(cap(mp.metaindexData.B) + cap(mp.indexData.B) + cap(mp.itemsData.B) + cap(mp.lensData.B))
}
