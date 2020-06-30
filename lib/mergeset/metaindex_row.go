package mergeset

import (
	"fmt"
	"io"
	"io/ioutil"
	"sort"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// metaindexRow describes a block of blockHeaders aka index block.
type metaindexRow struct {
	// First item in the first block.
	// It is used for fast lookup of the required index block.
	firstItem []byte

	// The number of blockHeaders the block contains.
	blockHeadersCount uint32

	// The offset of the block in the index file.
	indexBlockOffset uint64

	// The size of the block in the index file.
	indexBlockSize uint32
}

func (mr *metaindexRow) Reset() {
	mr.firstItem = mr.firstItem[:0]
	mr.blockHeadersCount = 0
	mr.indexBlockOffset = 0
	mr.indexBlockSize = 0
}

func (mr *metaindexRow) Marshal(dst []byte) []byte {
	dst = encoding.MarshalBytes(dst, mr.firstItem)
	dst = encoding.MarshalUint32(dst, mr.blockHeadersCount)
	dst = encoding.MarshalUint64(dst, mr.indexBlockOffset)
	dst = encoding.MarshalUint32(dst, mr.indexBlockSize)
	return dst
}

func (mr *metaindexRow) Unmarshal(src []byte) ([]byte, error) {
	// Unmarshal firstItem
	tail, fi, err := encoding.UnmarshalBytes(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal firstItem: %w", err)
	}
	mr.firstItem = append(mr.firstItem[:0], fi...)
	src = tail

	// Unmarshal blockHeadersCount
	if len(src) < 4 {
		return src, fmt.Errorf("cannot unmarshal blockHeadersCount from %d bytes; need at least %d bytes", len(src), 4)
	}
	mr.blockHeadersCount = encoding.UnmarshalUint32(src)
	src = src[4:]

	// Unmarshal indexBlockOffset
	if len(src) < 8 {
		return src, fmt.Errorf("cannot unmarshal indexBlockOffset from %d bytes; need at least %d bytes", len(src), 8)
	}
	mr.indexBlockOffset = encoding.UnmarshalUint64(src)
	src = src[8:]

	// Unmarshal indexBlockSize
	if len(src) < 4 {
		return src, fmt.Errorf("cannot unmarshal indexBlockSize from %d bytes; need at least %d bytes", len(src), 4)
	}
	mr.indexBlockSize = encoding.UnmarshalUint32(src)
	src = src[4:]

	if mr.blockHeadersCount <= 0 {
		return src, fmt.Errorf("blockHeadersCount must be bigger than 0; got %d", mr.blockHeadersCount)
	}
	if mr.indexBlockSize > 2*maxIndexBlockSize {
		return src, fmt.Errorf("too big indexBlockSize: %d; cannot exceed %d", mr.indexBlockSize, 2*maxIndexBlockSize)
	}

	return src, nil
}

func unmarshalMetaindexRows(dst []metaindexRow, r io.Reader) ([]metaindexRow, error) {
	// It is ok to read all the metaindex in memory,
	// since it is quite small.
	compressedData, err := ioutil.ReadAll(r)
	if err != nil {
		return dst, fmt.Errorf("cannot read metaindex data: %w", err)
	}
	data, err := encoding.DecompressZSTD(nil, compressedData)
	if err != nil {
		return dst, fmt.Errorf("cannot decompress metaindex data with size %d bytes: %w", len(compressedData), err)
	}

	dstLen := len(dst)
	for len(data) > 0 {
		if len(dst) < cap(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, metaindexRow{})
		}
		mr := &dst[len(dst)-1]
		tail, err := mr.Unmarshal(data)
		if err != nil {
			return dst, fmt.Errorf("cannot unmarshal metaindexRow #%d from metaindex data: %w", len(dst)-dstLen, err)
		}
		data = tail
	}
	if dstLen == len(dst) {
		return dst, fmt.Errorf("expecting non-zero metaindex rows; got zero")
	}

	// Make sure metaindexRows are sorted by firstItem.
	tmp := dst[dstLen:]
	ok := sort.SliceIsSorted(tmp, func(i, j int) bool {
		return string(tmp[i].firstItem) < string(tmp[j].firstItem)
	})
	if !ok {
		return dst, fmt.Errorf("metaindex %d rows aren't sorted by firstItem", len(tmp))
	}

	return dst, nil
}
