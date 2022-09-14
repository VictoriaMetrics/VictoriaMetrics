package storage

import (
	"fmt"
	"io"
	"sort"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// metaindexRow is a single metaindex row.
//
// The row points to a single index block containing block headers.
type metaindexRow struct {
	// TSID is the first TSID in the corresponding index block.
	TSID TSID

	// MinTimestamp is the minimum timestamp in the given index block.
	MinTimestamp int64

	// MaxTimestamp is the maximum timestamp in the given index block.
	MaxTimestamp int64

	// IndexBlockOffset is the offset of index block.
	IndexBlockOffset uint64

	// BlockHeadersCount is the number of block headers
	// in the given index block.
	BlockHeadersCount uint32

	// IndexBlockSize is the size of compressed index block.
	IndexBlockSize uint32
}

// Reset resets the mr using the given tsid.
func (mr *metaindexRow) Reset() {
	mr.TSID = TSID{}
	mr.BlockHeadersCount = 0
	mr.MinTimestamp = (1 << 63) - 1
	mr.MaxTimestamp = -1 << 63
	mr.IndexBlockOffset = 0
	mr.IndexBlockSize = 0
}

// RegisterBlockHeader registers the given bh in the mr.
func (mr *metaindexRow) RegisterBlockHeader(bh *blockHeader) {
	mr.BlockHeadersCount++
	if mr.BlockHeadersCount == 1 {
		mr.TSID = bh.TSID
		mr.MinTimestamp = bh.MinTimestamp
		mr.MaxTimestamp = bh.MaxTimestamp
		return
	}

	if bh.MinTimestamp < mr.MinTimestamp {
		mr.MinTimestamp = bh.MinTimestamp
	}
	if bh.MaxTimestamp > mr.MaxTimestamp {
		mr.MaxTimestamp = bh.MaxTimestamp
	}
}

// Marshal appends marshaled mr to dst and returns the result.
func (mr *metaindexRow) Marshal(dst []byte) []byte {
	dst = mr.TSID.Marshal(dst)
	dst = encoding.MarshalUint32(dst, mr.BlockHeadersCount)
	dst = encoding.MarshalInt64(dst, mr.MinTimestamp)
	dst = encoding.MarshalInt64(dst, mr.MaxTimestamp)
	dst = encoding.MarshalUint64(dst, mr.IndexBlockOffset)
	dst = encoding.MarshalUint32(dst, mr.IndexBlockSize)
	return dst
}

// Unmarshal unmarshals mr from src and returns the tail of src.
func (mr *metaindexRow) Unmarshal(src []byte) ([]byte, error) {
	// Unmarshal TSID
	tail, err := mr.TSID.Unmarshal(src)
	if err != nil {
		return src, fmt.Errorf("cannot unmarshal TSID: %w", err)
	}
	src = tail

	// Unmarshal BlockHeadersCount
	if len(src) < 4 {
		return src, fmt.Errorf("cannot unmarshal BlockHeadersCount from %d bytes; want at least %d bytes", len(src), 4)
	}
	mr.BlockHeadersCount = encoding.UnmarshalUint32(src)
	src = src[4:]

	// Unmarshal MinTimestamp
	if len(src) < 8 {
		return src, fmt.Errorf("cannot unmarshal MinTimestamp from %d bytes; want at least %d bytes", len(src), 8)
	}
	mr.MinTimestamp = encoding.UnmarshalInt64(src)
	src = src[8:]

	// Unmarshal MaxTimestamp
	if len(src) < 8 {
		return src, fmt.Errorf("cannot unmarshal MaxTimestamp from %d bytes; want at least %d bytes", len(src), 8)
	}
	mr.MaxTimestamp = encoding.UnmarshalInt64(src)
	src = src[8:]

	// Unmarshal IndexBlockOffset
	if len(src) < 8 {
		return src, fmt.Errorf("cannot unmarshal IndexBlockOffset from %d bytes; want at least %d bytes", len(src), 8)
	}
	mr.IndexBlockOffset = encoding.UnmarshalUint64(src)
	src = src[8:]

	// Unmarshal IndexBlockSize
	if len(src) < 4 {
		return src, fmt.Errorf("cannot unmarshal IndexBlockSize from %d bytes; want at least %d bytes", len(src), 4)
	}
	mr.IndexBlockSize = encoding.UnmarshalUint32(src)
	src = src[4:]

	// Validate unmarshaled data.
	if mr.BlockHeadersCount <= 0 {
		return src, fmt.Errorf("BlockHeadersCount must be greater than 0")
	}
	if mr.IndexBlockSize > 2*maxBlockSize {
		return src, fmt.Errorf("too big IndexBlockSize; got %d; cannot exceed %d", mr.IndexBlockSize, 2*maxBlockSize)
	}

	return src, nil
}

func unmarshalMetaindexRows(dst []metaindexRow, r io.Reader) ([]metaindexRow, error) {
	compressedData, err := io.ReadAll(r)
	if err != nil {
		return dst, fmt.Errorf("cannot read metaindex rows: %w", err)
	}
	data, err := encoding.DecompressZSTD(nil, compressedData)
	if err != nil {
		return dst, fmt.Errorf("cannot decompress metaindex rows: %w", err)
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

	// Make sure metaindex rows are sorted by tsid
	tmp := dst[dstLen:]
	ok := sort.SliceIsSorted(tmp, func(i, j int) bool { return tmp[i].TSID.Less(&tmp[j].TSID) })
	if !ok {
		return dst, fmt.Errorf("metaindexRow values must be sorted by TSID; got %+v", tmp)
	}

	return dst, nil
}
