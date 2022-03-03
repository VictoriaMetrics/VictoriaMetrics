package mergeset

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type inmemoryPart struct {
	ph partHeader
	bh blockHeader
	mr metaindexRow

	unpackedIndexBlockBuf []byte
	packedIndexBlockBuf   []byte

	unpackedMetaindexBuf []byte
	packedMetaindexBuf   []byte

	metaindexData bytesutil.ByteBuffer
	indexData     bytesutil.ByteBuffer
	itemsData     bytesutil.ByteBuffer
	lensData      bytesutil.ByteBuffer
}

func (mp *inmemoryPart) Reset() {
	mp.ph.Reset()
	mp.bh.Reset()
	mp.mr.Reset()

	mp.unpackedIndexBlockBuf = mp.unpackedIndexBlockBuf[:0]
	mp.packedIndexBlockBuf = mp.packedIndexBlockBuf[:0]

	mp.unpackedMetaindexBuf = mp.unpackedMetaindexBuf[:0]
	mp.packedMetaindexBuf = mp.packedMetaindexBuf[:0]

	mp.metaindexData.Reset()
	mp.indexData.Reset()
	mp.itemsData.Reset()
	mp.lensData.Reset()
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

	mp.unpackedIndexBlockBuf = mp.bh.Marshal(mp.unpackedIndexBlockBuf[:0])
	mp.packedIndexBlockBuf = encoding.CompressZSTDLevel(mp.packedIndexBlockBuf[:0], mp.unpackedIndexBlockBuf, 0)
	fs.MustWriteData(&mp.indexData, mp.packedIndexBlockBuf)

	mp.mr.firstItem = append(mp.mr.firstItem[:0], mp.bh.firstItem...)
	mp.mr.blockHeadersCount = 1
	mp.mr.indexBlockOffset = 0
	mp.mr.indexBlockSize = uint32(len(mp.packedIndexBlockBuf))
	mp.unpackedMetaindexBuf = mp.mr.Marshal(mp.unpackedMetaindexBuf[:0])
	mp.packedMetaindexBuf = encoding.CompressZSTDLevel(mp.packedMetaindexBuf[:0], mp.unpackedMetaindexBuf, 0)
	fs.MustWriteData(&mp.metaindexData, mp.packedMetaindexBuf)
}

// It is safe calling NewPart multiple times.
// It is unsafe re-using mp while the returned part is in use.
func (mp *inmemoryPart) NewPart() *part {
	ph := mp.ph
	size := mp.size()
	p, err := newPart(&ph, "", size, mp.metaindexData.NewReader(), &mp.indexData, &mp.itemsData, &mp.lensData)
	if err != nil {
		logger.Panicf("BUG: cannot create a part from inmemoryPart: %s", err)
	}
	return p
}

func (mp *inmemoryPart) size() uint64 {
	return uint64(len(mp.metaindexData.B) + len(mp.indexData.B) + len(mp.itemsData.B) + len(mp.lensData.B))
}

func getInmemoryPart() *inmemoryPart {
	v := inmemoryPartPool.Get()
	if v == nil {
		return &inmemoryPart{}
	}
	return v.(*inmemoryPart)
}

func putInmemoryPart(mp *inmemoryPart) {
	mp.Reset()
	inmemoryPartPool.Put(mp)
}

var inmemoryPartPool sync.Pool
