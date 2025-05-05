package mergeset

import (
	"fmt"
	"sort"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

type blockHeader struct {
	// common prefix for all the items in the block.
	commonPrefix []byte

	// The first item.
	firstItem []byte

	// Whether commonPrefix and firstItem point to external data.
	noCopy bool

	// Marshal type used for block compression.
	marshalType marshalType

	// The number of items in the block, excluding the first item.
	itemsCount uint32

	// The offset of the items block.
	itemsBlockOffset uint64

	// The offset of the lens block.
	lensBlockOffset uint64

	// The size of the items block.
	itemsBlockSize uint32

	// The size of the lens block.
	lensBlockSize uint32
}

func (bh *blockHeader) SizeBytes() int {
	return int(unsafe.Sizeof(*bh)) + cap(bh.commonPrefix) + cap(bh.firstItem)
}

func (bh *blockHeader) Reset() {
	if bh.noCopy {
		bh.commonPrefix = nil
		bh.firstItem = nil
	} else {
		bh.commonPrefix = bh.commonPrefix[:0]
		bh.firstItem = bh.firstItem[:0]
	}
	bh.marshalType = marshalTypePlain
	bh.itemsCount = 0
	bh.itemsBlockOffset = 0
	bh.lensBlockOffset = 0
	bh.itemsBlockSize = 0
	bh.lensBlockSize = 0
}

func (bh *blockHeader) Marshal(dst []byte) []byte {
	dst = encoding.MarshalBytes(dst, bh.commonPrefix)
	dst = encoding.MarshalBytes(dst, bh.firstItem)
	dst = append(dst, byte(bh.marshalType))
	dst = encoding.MarshalUint32(dst, bh.itemsCount)
	dst = encoding.MarshalUint64(dst, bh.itemsBlockOffset)
	dst = encoding.MarshalUint64(dst, bh.lensBlockOffset)
	dst = encoding.MarshalUint32(dst, bh.itemsBlockSize)
	dst = encoding.MarshalUint32(dst, bh.lensBlockSize)
	return dst
}

// UnmarshalNoCopy unmarshals bh from src without copying the data from src.
//
// The src must remain unchanged while bh is in use.
func (bh *blockHeader) UnmarshalNoCopy(src []byte) ([]byte, error) {
	bh.noCopy = true
	// Unmarshal commonPrefix
	cp, nSize := encoding.UnmarshalBytes(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal commonPrefix")
	}
	src = src[nSize:]
	bh.commonPrefix = cp[:len(cp):len(cp)]

	// Unmarshal firstItem
	fi, nSize := encoding.UnmarshalBytes(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal firstItem")
	}
	src = src[nSize:]
	bh.firstItem = fi[:len(fi):len(fi)]

	// Unmarshal marshalType
	if len(src) == 0 {
		return src, fmt.Errorf("cannot unmarshal marshalType from zero bytes")
	}
	bh.marshalType = marshalType(src[0])
	src = src[1:]
	if err := checkMarshalType(bh.marshalType); err != nil {
		return src, fmt.Errorf("unexpected marshalType: %w", err)
	}

	// Unmarshal itemsCount
	if len(src) < 4 {
		return src, fmt.Errorf("cannot unmarshal itemsCount from %d bytes; need at least %d bytes", len(src), 4)
	}
	bh.itemsCount = encoding.UnmarshalUint32(src)
	src = src[4:]

	// Unmarshal itemsBlockOffset
	if len(src) < 8 {
		return src, fmt.Errorf("cannot unmarshal itemsBlockOffset from %d bytes; need at least %d bytes", len(src), 8)
	}
	bh.itemsBlockOffset = encoding.UnmarshalUint64(src)
	src = src[8:]

	// Unmarshal lensBlockOffset
	if len(src) < 8 {
		return src, fmt.Errorf("cannot unmarshal lensBlockOffset from %d bytes; need at least %d bytes", len(src), 8)
	}
	bh.lensBlockOffset = encoding.UnmarshalUint64(src)
	src = src[8:]

	// Unmarshal itemsBlockSize
	if len(src) < 4 {
		return src, fmt.Errorf("cannot unmarshal itemsBlockSize from %d bytes; need at least %d bytes", len(src), 4)
	}
	bh.itemsBlockSize = encoding.UnmarshalUint32(src)
	src = src[4:]

	// Unmarshal lensBlockSize
	if len(src) < 4 {
		return src, fmt.Errorf("cannot unmarshal lensBlockSize from %d bytes; need at least %d bytes", len(src), 4)
	}
	bh.lensBlockSize = encoding.UnmarshalUint32(src)
	src = src[4:]

	if bh.itemsCount <= 0 {
		return src, fmt.Errorf("itemsCount must be bigger than 0; got %d", bh.itemsCount)
	}
	if bh.itemsBlockSize > 2*maxInmemoryBlockSize {
		return src, fmt.Errorf("too big itemsBlockSize; got %d; cannot exceed %d", bh.itemsBlockSize, 2*maxInmemoryBlockSize)
	}
	if bh.lensBlockSize > 2*8*maxInmemoryBlockSize {
		return src, fmt.Errorf("too big lensBlockSize; got %d; cannot exceed %d", bh.lensBlockSize, 2*8*maxInmemoryBlockSize)
	}

	return src, nil
}

// unmarshalBlockHeadersNoCopy unmarshals all the block headers from src,
// appends them to dst and returns the appended result.
//
// Block headers must be sorted by bh.firstItem.
//
// It is expected that src remains unchanged while rhe returned blocks are in use.
func unmarshalBlockHeadersNoCopy(dst []blockHeader, src []byte, blockHeadersCount int) ([]blockHeader, error) {
	if blockHeadersCount <= 0 {
		logger.Panicf("BUG: blockHeadersCount must be greater than 0; got %d", blockHeadersCount)
	}
	dstLen := len(dst)
	dst = slicesutil.SetLength(dst, dstLen+blockHeadersCount)
	for i := 0; i < blockHeadersCount; i++ {
		tail, err := dst[dstLen+i].UnmarshalNoCopy(src)
		if err != nil {
			return dst, fmt.Errorf("cannot unmarshal block header #%d out of %d: %w", i, blockHeadersCount, err)
		}
		src = tail
	}
	if len(src) > 0 {
		return dst, fmt.Errorf("unexpected non-zero tail left after unmarshaling %d block headers; len(tail)=%d", blockHeadersCount, len(src))
	}
	newBHS := dst[dstLen:]

	// Verify that block headers are sorted by firstItem.
	if !sort.SliceIsSorted(newBHS, func(i, j int) bool { return string(newBHS[i].firstItem) < string(newBHS[j].firstItem) }) {
		return dst, fmt.Errorf("block headers must be sorted by firstItem; unmarshaled unsorted block headers: %#v", newBHS)
	}

	return dst, nil
}
