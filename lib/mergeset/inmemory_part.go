package mergeset

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type inmemoryPart struct {
	ph partHeader
	sb storageBlock
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
	mp.sb.Reset()
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

	// Use the minimum possible compressLevel for compressing inmemoryPart,
	// since it will be merged into file part soon.
	compressLevel := 0
	mp.bh.firstItem, mp.bh.commonPrefix, mp.bh.itemsCount, mp.bh.marshalType = ib.MarshalUnsortedData(&mp.sb, mp.bh.firstItem[:0], mp.bh.commonPrefix[:0], compressLevel)

	mp.ph.itemsCount = uint64(len(ib.items))
	mp.ph.blocksCount = 1
	mp.ph.firstItem = append(mp.ph.firstItem[:0], ib.items[0].String(ib.data)...)
	mp.ph.lastItem = append(mp.ph.lastItem[:0], ib.items[len(ib.items)-1].String(ib.data)...)

	fs.MustWriteData(&mp.itemsData, mp.sb.itemsData)
	mp.bh.itemsBlockOffset = 0
	mp.bh.itemsBlockSize = uint32(len(mp.sb.itemsData))

	fs.MustWriteData(&mp.lensData, mp.sb.lensData)
	mp.bh.lensBlockOffset = 0
	mp.bh.lensBlockSize = uint32(len(mp.sb.lensData))

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
	select {
	case mp := <-mpPool:
		return mp
	default:
		return &inmemoryPart{}
	}
}

func putInmemoryPart(mp *inmemoryPart) {
	mp.Reset()
	select {
	case mpPool <- mp:
	default:
		// Drop mp in order to reduce memory usage.
	}
}

// Use chan instead of sync.Pool in order to reduce memory usage on systems with big number of CPU cores,
// since sync.Pool maintains per-CPU pool of inmemoryPart objects.
//
// The inmemoryPart object size can exceed 64KB, so it is better to use chan instead of sync.Pool for reducing memory usage.
var mpPool = make(chan *inmemoryPart, cgroup.AvailableCPUs())
