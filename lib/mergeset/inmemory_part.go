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

func (ip *inmemoryPart) Reset() {
	ip.ph.Reset()
	ip.sb.Reset()
	ip.bh.Reset()
	ip.mr.Reset()

	ip.unpackedIndexBlockBuf = ip.unpackedIndexBlockBuf[:0]
	ip.packedIndexBlockBuf = ip.packedIndexBlockBuf[:0]

	ip.unpackedMetaindexBuf = ip.unpackedMetaindexBuf[:0]
	ip.packedMetaindexBuf = ip.packedMetaindexBuf[:0]

	ip.metaindexData.Reset()
	ip.indexData.Reset()
	ip.itemsData.Reset()
	ip.lensData.Reset()
}

// Init initializes ip from ib.
func (ip *inmemoryPart) Init(ib *inmemoryBlock) {
	ip.Reset()

	// Use the minimum possible compressLevel for compressing inmemoryPart,
	// since it will be merged into file part soon.
	compressLevel := 0
	ip.bh.firstItem, ip.bh.commonPrefix, ip.bh.itemsCount, ip.bh.marshalType = ib.MarshalUnsortedData(&ip.sb, ip.bh.firstItem[:0], ip.bh.commonPrefix[:0], compressLevel)

	ip.ph.itemsCount = uint64(len(ib.items))
	ip.ph.blocksCount = 1
	ip.ph.firstItem = append(ip.ph.firstItem[:0], ib.items[0]...)
	ip.ph.lastItem = append(ip.ph.lastItem[:0], ib.items[len(ib.items)-1]...)

	fs.MustWriteData(&ip.itemsData, ip.sb.itemsData)
	ip.bh.itemsBlockOffset = 0
	ip.bh.itemsBlockSize = uint32(len(ip.sb.itemsData))

	fs.MustWriteData(&ip.lensData, ip.sb.lensData)
	ip.bh.lensBlockOffset = 0
	ip.bh.lensBlockSize = uint32(len(ip.sb.lensData))

	ip.unpackedIndexBlockBuf = ip.bh.Marshal(ip.unpackedIndexBlockBuf[:0])
	ip.packedIndexBlockBuf = encoding.CompressZSTDLevel(ip.packedIndexBlockBuf[:0], ip.unpackedIndexBlockBuf, 0)
	fs.MustWriteData(&ip.indexData, ip.packedIndexBlockBuf)

	ip.mr.firstItem = append(ip.mr.firstItem[:0], ip.bh.firstItem...)
	ip.mr.blockHeadersCount = 1
	ip.mr.indexBlockOffset = 0
	ip.mr.indexBlockSize = uint32(len(ip.packedIndexBlockBuf))
	ip.unpackedMetaindexBuf = ip.mr.Marshal(ip.unpackedMetaindexBuf[:0])
	ip.packedMetaindexBuf = encoding.CompressZSTDLevel(ip.packedMetaindexBuf[:0], ip.unpackedMetaindexBuf, 0)
	fs.MustWriteData(&ip.metaindexData, ip.packedMetaindexBuf)
}

// It is safe calling NewPart multiple times.
// It is unsafe re-using ip while the returned part is in use.
func (ip *inmemoryPart) NewPart() *part {
	ph := ip.ph
	size := ip.size()
	p, err := newPart(&ph, "", size, ip.metaindexData.NewReader(), &ip.indexData, &ip.itemsData, &ip.lensData)
	if err != nil {
		logger.Panicf("BUG: cannot create a part from inmemoryPart: %s", err)
	}
	return p
}

func (ip *inmemoryPart) size() uint64 {
	return uint64(len(ip.metaindexData.B) + len(ip.indexData.B) + len(ip.itemsData.B) + len(ip.lensData.B))
}

func getInmemoryPart() *inmemoryPart {
	v := ipPool.Get()
	if v == nil {
		return &inmemoryPart{}
	}
	return v.(*inmemoryPart)
}

func putInmemoryPart(ip *inmemoryPart) {
	ip.Reset()
	ipPool.Put(ip)
}

var ipPool sync.Pool
