package storage

import (
	"fmt"
	"math"
	"sort"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// blockHeader is a header for a time series block.
//
// Each block contains rows for a single time series. Rows are sorted
// by timestamp.
//
// A single time series may span multiple blocks.
type blockHeader struct {
	// TSID is the TSID for the block.
	// Multiple blocks may have the same TSID.
	TSID TSID

	// MinTimestamp is the minimum timestamp in the block.
	//
	// This is the first timestamp, since rows are sorted by timestamps.
	MinTimestamp int64

	// MaxTimestamp is the maximum timestamp in the block.
	//
	// This is the last timestamp, since rows are sorted by timestamps.
	MaxTimestamp int64

	// FirstValue is the first value in the block.
	//
	// It is stored here for better compression level, since usually
	// the first value significantly differs from subsequent values
	// which may be delta-encoded.
	FirstValue int64

	// TimestampsBlockOffset is the offset in bytes for a block
	// with timestamps in timestamps file.
	TimestampsBlockOffset uint64

	// ValuesBlockOffset is the offset in bytes for a block with values
	// in values file.
	ValuesBlockOffset uint64

	// TimestampsBlocksSize is the size in bytes for a block with timestamps.
	TimestampsBlockSize uint32

	// ValuesBlockSize is the size in bytes for a block with values.
	ValuesBlockSize uint32

	// RowsCount is the number of rows in the block.
	//
	// The block must contain at least one row.
	RowsCount uint32

	// Scale is the 10^Scale multiplier for values in the block.
	Scale int16

	// TimestampsMarshalType is the marshal type used for marshaling
	// a block with timestamps.
	TimestampsMarshalType encoding.MarshalType

	// ValuesMarshalType is the marshal type used for marshaling
	// a block with values.
	ValuesMarshalType encoding.MarshalType

	// PrecisionBits is the number of significant bits when using
	// MarshalTypeNearestDelta2 encoding.
	//
	// Possible values are in the range [1...64], where
	//     1 means max 50% error,
	//     2 means max 25% error,
	//     n means max 100/(2^n)% error,
	//    64 means exact values.
	//
	// Lower PrecisionBits give better block compression and speed.
	PrecisionBits uint8
}

// Less returns true if b is less than src.
func (bh *blockHeader) Less(src *blockHeader) bool {
	if bh.TSID.MetricID == src.TSID.MetricID {
		// Fast path for identical TSIDs.
		return bh.MinTimestamp < src.MinTimestamp
	}

	// Slow path for distinct TSIDs.
	return bh.TSID.Less(&src.TSID)
}

// marshaledBlockHeaderSize is the size of marshaled block header.
var marshaledBlockHeaderSize = func() int {
	var bh blockHeader
	data := bh.Marshal(nil)
	return len(data)
}()

// Marshal appends marshaled bh to dst and returns the result.
func (bh *blockHeader) Marshal(dst []byte) []byte {
	dst = bh.TSID.Marshal(dst)
	dst = encoding.MarshalInt64(dst, bh.MinTimestamp)
	dst = encoding.MarshalInt64(dst, bh.MaxTimestamp)
	dst = encoding.MarshalInt64(dst, bh.FirstValue)
	dst = encoding.MarshalUint64(dst, bh.TimestampsBlockOffset)
	dst = encoding.MarshalUint64(dst, bh.ValuesBlockOffset)
	dst = encoding.MarshalUint32(dst, bh.TimestampsBlockSize)
	dst = encoding.MarshalUint32(dst, bh.ValuesBlockSize)
	dst = encoding.MarshalUint32(dst, bh.RowsCount)
	dst = encoding.MarshalInt16(dst, bh.Scale)
	dst = append(dst, byte(bh.TimestampsMarshalType), byte(bh.ValuesMarshalType), bh.PrecisionBits)
	return dst
}

// Unmarshal unmarshals bh from src and returns the rest of src.
func (bh *blockHeader) Unmarshal(src []byte) ([]byte, error) {
	if len(src) < marshaledBlockHeaderSize {
		return src, fmt.Errorf("too short block header; got %d bytes; want %d bytes", len(src), marshaledBlockHeaderSize)
	}

	tail, err := bh.TSID.Unmarshal(src)
	if err != nil {
		return src, fmt.Errorf("cannot unmarshal TSID: %w", err)
	}
	src = tail

	bh.MinTimestamp = encoding.UnmarshalInt64(src)
	src = src[8:]
	bh.MaxTimestamp = encoding.UnmarshalInt64(src)
	src = src[8:]
	bh.FirstValue = encoding.UnmarshalInt64(src)
	src = src[8:]
	bh.TimestampsBlockOffset = encoding.UnmarshalUint64(src)
	src = src[8:]
	bh.ValuesBlockOffset = encoding.UnmarshalUint64(src)
	src = src[8:]
	bh.TimestampsBlockSize = encoding.UnmarshalUint32(src)
	src = src[4:]
	bh.ValuesBlockSize = encoding.UnmarshalUint32(src)
	src = src[4:]
	bh.RowsCount = encoding.UnmarshalUint32(src)
	src = src[4:]
	bh.Scale = encoding.UnmarshalInt16(src)
	src = src[2:]
	bh.TimestampsMarshalType = encoding.MarshalType(src[0])
	src = src[1:]
	bh.ValuesMarshalType = encoding.MarshalType(src[0])
	src = src[1:]
	bh.PrecisionBits = uint8(src[0])
	src = src[1:]

	err = bh.validate()
	return src, err
}

func (bh *blockHeader) marshalPortable(dst []byte) []byte {
	dst = encoding.MarshalVarInt64(dst, bh.MinTimestamp)
	dst = encoding.MarshalVarInt64(dst, bh.MaxTimestamp)
	dst = encoding.MarshalVarInt64(dst, bh.FirstValue)
	dst = encoding.MarshalVarUint64(dst, uint64(bh.RowsCount))
	dst = encoding.MarshalVarInt64(dst, int64(bh.Scale))
	dst = append(dst, byte(bh.TimestampsMarshalType), byte(bh.ValuesMarshalType), byte(bh.PrecisionBits))
	return dst
}

func (bh *blockHeader) unmarshalPortable(src []byte) ([]byte, error) {
	minTimestamp, nSize := encoding.UnmarshalVarInt64(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal firstTimestamp from varint")
	}
	src = src[nSize:]
	bh.MinTimestamp = minTimestamp

	maxTimestamp, nSize := encoding.UnmarshalVarInt64(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal firstTimestamp rom varint")
	}
	src = src[nSize:]
	bh.MaxTimestamp = maxTimestamp

	firstValue, nSize := encoding.UnmarshalVarInt64(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal firstValue from varint")
	}
	src = src[nSize:]
	bh.FirstValue = firstValue

	rowsCount, nSize := encoding.UnmarshalVarUint64(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal rowsCount from varuint")
	}
	src = src[nSize:]
	if rowsCount > math.MaxUint32 {
		return src, fmt.Errorf("got too big rowsCount=%d; it mustn't exceed %d", rowsCount, uint32(math.MaxUint32))
	}
	bh.RowsCount = uint32(rowsCount)

	scale, nSize := encoding.UnmarshalVarInt64(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal scale from varint")
	}
	src = src[nSize:]
	if scale < math.MinInt16 {
		return src, fmt.Errorf("got too small scale=%d; it mustn't be smaller than %d", scale, math.MinInt16)
	}
	if scale > math.MaxInt16 {
		return src, fmt.Errorf("got too big scale=%d; it mustn't exceed %d", scale, math.MaxInt16)
	}
	bh.Scale = int16(scale)
	if len(src) < 1 {
		return src, fmt.Errorf("cannot unmarshal marshalType for timestamps from %d bytes; need at least %d bytes", len(src), 1)
	}

	bh.TimestampsMarshalType = encoding.MarshalType(src[0])
	src = src[1:]
	if len(src) < 1 {
		return src, fmt.Errorf("cannot unmarshal marshalType for values from %d bytes; need at least %d bytes", len(src), 1)
	}
	bh.ValuesMarshalType = encoding.MarshalType(src[0])
	src = src[1:]
	if len(src) < 1 {
		return src, fmt.Errorf("cannot unmarshal precisionBits for values from %d bytes; need at least %d bytes", len(src), 1)
	}
	bh.PrecisionBits = uint8(src[0])
	src = src[1:]
	return src, nil
}

func (bh *blockHeader) validate() error {
	if bh.RowsCount == 0 {
		return fmt.Errorf("RowsCount in block header cannot be zero")
	}
	if bh.RowsCount > 2*maxRowsPerBlock {
		return fmt.Errorf("too big RowsCount; got %d; cannot exceed %d", bh.RowsCount, 2*maxRowsPerBlock)
	}
	if err := encoding.CheckMarshalType(bh.TimestampsMarshalType); err != nil {
		return fmt.Errorf("unsupported TimestampsMarshalType: %w", err)
	}
	if err := encoding.CheckMarshalType(bh.ValuesMarshalType); err != nil {
		return fmt.Errorf("unsupported ValuesMarshalType: %w", err)
	}
	if err := encoding.CheckPrecisionBits(bh.PrecisionBits); err != nil {
		return err
	}
	if bh.TimestampsBlockSize > 2*maxBlockSize {
		return fmt.Errorf("too big TimestampsBlockSize; got %d; cannot exceed %d", bh.TimestampsBlockSize, 2*maxBlockSize)
	}
	if bh.ValuesBlockSize > 2*maxBlockSize {
		return fmt.Errorf("too big ValuesBlockSize; got %d; cannot exceed %d", bh.ValuesBlockSize, 2*maxBlockSize)
	}
	return nil
}

// unmarshalBlockHeaders unmarshals all the block headers from src,
// appends them to dst and returns the appended result.
//
// Block headers must be sorted by bh.TSID.
func unmarshalBlockHeaders(dst []blockHeader, src []byte, blockHeadersCount int) ([]blockHeader, error) {
	if blockHeadersCount <= 0 {
		logger.Panicf("BUG: blockHeadersCount must be greater than zero; got %d", blockHeadersCount)
	}
	dstLen := len(dst)
	dst = slicesutil.ExtendCapacity(dst, blockHeadersCount)
	var bh blockHeader
	for len(src) > 0 {
		tmp, err := bh.Unmarshal(src)
		if err != nil {
			return dst, fmt.Errorf("cannot unmarshal block header: %w", err)
		}
		src = tmp
		dst = append(dst, bh)
	}

	newBHS := dst[dstLen:]

	// Verify the number of read block headers.
	if len(newBHS) != blockHeadersCount {
		return dst, fmt.Errorf("invalid number of block headers found: %d; want %d block headers", len(newBHS), blockHeadersCount)
	}

	// Verify that block headers are sorted by tsid.
	if !sort.SliceIsSorted(newBHS, func(i, j int) bool { return newBHS[i].TSID.Less(&newBHS[j].TSID) }) {
		return dst, fmt.Errorf("block headers must be sorted by tsid; unmarshaled unsorted block headers: %+v", newBHS)
	}

	return dst, nil
}
