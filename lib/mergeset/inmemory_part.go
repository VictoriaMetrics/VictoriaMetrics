package mergeset

import (
	"path/filepath"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/chunkedbuffer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type inmemoryPart struct {
	ph partHeader
	bh blockHeader
	mr metaindexRow

	metaindexData chunkedbuffer.Buffer
	indexData     chunkedbuffer.Buffer
	itemsData     chunkedbuffer.Buffer
	lensData      chunkedbuffer.Buffer
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
	indexPath := filepath.Join(path, indexFilename)
	itemsPath := filepath.Join(path, itemsFilename)
	lensPath := filepath.Join(path, lensFilename)

	var psw filestream.ParallelStreamWriter
	psw.Add(metaindexPath, &mp.metaindexData)
	psw.Add(indexPath, &mp.indexData)
	psw.Add(itemsPath, &mp.itemsData)
	psw.Add(lensPath, &mp.lensData)
	psw.Run()

	mp.ph.MustWriteMetadata(path)

	fs.MustSyncPathAndParentDir(path)
}

// Init initializes mp from ib.
func (mp *inmemoryPart) Init(ib *inmemoryBlock) {
	mp.Reset()

	sb := &storageBlock{}

	// Use the minimum possible compressLevel for compressing inmemoryPart,
	// since it will be merged into file part soon.
	// See https://github.com/facebook/zstd/releases/tag/v1.3.4 for details about negative compression level
	compressLevel := -5
	mp.bh.firstItem, mp.bh.commonPrefix, mp.bh.itemsCount, mp.bh.marshalType = ib.MarshalUnsortedData(sb, mp.bh.firstItem[:0], mp.bh.commonPrefix[:0], compressLevel)

	mp.ph.itemsCount = uint64(len(ib.items))
	mp.ph.blocksCount = 1
	mp.ph.firstItem = append(mp.ph.firstItem[:0], ib.items[0].String(ib.data)...)
	mp.ph.lastItem = append(mp.ph.lastItem[:0], ib.items[len(ib.items)-1].String(ib.data)...)

	mp.itemsData.MustWrite(sb.itemsData)
	mp.bh.itemsBlockOffset = 0
	mp.bh.itemsBlockSize = uint32(len(sb.itemsData))

	mp.lensData.MustWrite(sb.lensData)
	mp.bh.lensBlockOffset = 0
	mp.bh.lensBlockSize = uint32(len(sb.lensData))

	bb := inmemoryPartBytePool.Get()
	bb.B = mp.bh.Marshal(bb.B[:0])
	if len(bb.B) > 3*maxIndexBlockSize {
		// marshaled blockHeader can exceed indexBlockSize when firstItem and commonPrefix sizes are close to indexBlockSize
		logger.Panicf("BUG: too big index block: %d bytes; mustn't exceed %d bytes", len(bb.B), 3*maxIndexBlockSize)
	}
	bbLen := len(bb.B)
	bb.B = encoding.CompressZSTDLevel(bb.B, bb.B, compressLevel)
	mp.indexData.MustWrite(bb.B[bbLen:])

	mp.mr.firstItem = append(mp.mr.firstItem[:0], mp.bh.firstItem...)
	mp.mr.blockHeadersCount = 1
	mp.mr.indexBlockOffset = 0
	mp.mr.indexBlockSize = uint32(len(bb.B[bbLen:]))
	bb.B = mp.mr.Marshal(bb.B[:0])
	bbLen = len(bb.B)
	bb.B = encoding.CompressZSTDLevel(bb.B, bb.B, compressLevel)
	mp.metaindexData.MustWrite(bb.B[bbLen:])
	inmemoryPartBytePool.Put(bb)
}

var inmemoryPartBytePool bytesutil.ByteBufferPool

// It is safe calling NewPart multiple times.
// It is unsafe reusing mp while the returned part is in use.
func (mp *inmemoryPart) NewPart() *part {
	size := mp.size()
	p := newPart(&mp.ph, "", size, mp.metaindexData.NewReader(), &mp.indexData, &mp.itemsData, &mp.lensData)
	return p
}

func (mp *inmemoryPart) size() uint64 {
	return uint64(mp.metaindexData.SizeBytes() + mp.indexData.SizeBytes() + mp.itemsData.SizeBytes() + mp.lensData.SizeBytes())
}
