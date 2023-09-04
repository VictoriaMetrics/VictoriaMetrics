package logstorage

import (
	"fmt"
	"math"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// blockHeader contains information about a single block.
//
// blockHeader is stored in the indexFilename file.
type blockHeader struct {
	// streamID is a stream id for entries in the block
	streamID streamID

	// uncompressedSizeBytes is the original (uncompressed) size of log entries stored in the block
	uncompressedSizeBytes uint64

	// rowsCount is the number of log entries stored in the block
	rowsCount uint64

	// timestampsHeader contains information about timestamps for log entries in the block
	timestampsHeader timestampsHeader

	// columnsHeaderOffset is the offset of columnsHeader at columnsHeaderFilename
	columnsHeaderOffset uint64

	// columnsHeaderSize is the size of columnsHeader at columnsHeaderFilename
	columnsHeaderSize uint64
}

// reset resets bh, so it can be re-used.
func (bh *blockHeader) reset() {
	bh.streamID.reset()
	bh.uncompressedSizeBytes = 0
	bh.rowsCount = 0
	bh.timestampsHeader.reset()
	bh.columnsHeaderOffset = 0
	bh.columnsHeaderSize = 0
}

func (bh *blockHeader) copyFrom(src *blockHeader) {
	bh.reset()

	bh.streamID = src.streamID
	bh.uncompressedSizeBytes = src.uncompressedSizeBytes
	bh.rowsCount = src.rowsCount
	bh.timestampsHeader.copyFrom(&src.timestampsHeader)
	bh.columnsHeaderOffset = src.columnsHeaderOffset
	bh.columnsHeaderSize = src.columnsHeaderSize
}

// marshal appends the marshaled bh to dst and returns the result.
func (bh *blockHeader) marshal(dst []byte) []byte {
	// Do not store the version used for encoding directly in the block header, since:
	// - all the block headers in the same part use the same encoding
	// - the block header encoding version can be put in metadata file for the part (aka metadataFilename)

	dst = bh.streamID.marshal(dst)
	dst = encoding.MarshalVarUint64(dst, bh.uncompressedSizeBytes)
	dst = encoding.MarshalVarUint64(dst, bh.rowsCount)
	dst = bh.timestampsHeader.marshal(dst)
	dst = encoding.MarshalVarUint64(dst, bh.columnsHeaderOffset)
	dst = encoding.MarshalVarUint64(dst, bh.columnsHeaderSize)

	return dst
}

// unmarshal unmarshals bh from src and returns the remaining tail.
func (bh *blockHeader) unmarshal(src []byte) ([]byte, error) {
	bh.reset()

	srcOrig := src

	// unmarshal bh.streamID
	tail, err := bh.streamID.unmarshal(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal streamID: %w", err)
	}
	src = tail

	// unmarshal bh.uncompressedSizeBytes
	tail, n, err := encoding.UnmarshalVarUint64(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal uncompressedSizeBytes: %w", err)
	}
	bh.uncompressedSizeBytes = n
	src = tail

	// unmarshal bh.rowsCount
	tail, n, err = encoding.UnmarshalVarUint64(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal rowsCount: %w", err)
	}
	if n > maxRowsPerBlock {
		return srcOrig, fmt.Errorf("too big value for rowsCount: %d; mustn't exceed %d", n, maxRowsPerBlock)
	}
	bh.rowsCount = n
	src = tail

	// unmarshal bh.timestampsHeader
	tail, err = bh.timestampsHeader.unmarshal(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal timestampsHeader: %w", err)
	}
	src = tail

	// unmarshal columnsHeaderOffset
	tail, n, err = encoding.UnmarshalVarUint64(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal columnsHeaderOffset: %w", err)
	}
	bh.columnsHeaderOffset = n
	src = tail

	// unmarshal columnsHeaderSize
	tail, n, err = encoding.UnmarshalVarUint64(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal columnsHeaderSize: %w", err)
	}
	if n > maxColumnsHeaderSize {
		return srcOrig, fmt.Errorf("too big value for columnsHeaderSize: %d; mustn't exceed %d", n, maxColumnsHeaderSize)
	}
	bh.columnsHeaderSize = n
	src = tail

	return src, nil
}

func getBlockHeader() *blockHeader {
	v := blockHeaderPool.Get()
	if v == nil {
		return &blockHeader{}
	}
	return v.(*blockHeader)
}

func putBlockHeader(bh *blockHeader) {
	bh.reset()
	blockHeaderPool.Put(bh)
}

var blockHeaderPool sync.Pool

// unmarshalBlockHeaders appends unmarshaled from src blockHeader entries to dst and returns the result.
func unmarshalBlockHeaders(dst []blockHeader, src []byte) ([]blockHeader, error) {
	dstOrig := dst
	for len(src) > 0 {
		if len(dst) < cap(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, blockHeader{})
		}
		bh := &dst[len(dst)-1]
		tail, err := bh.unmarshal(src)
		if err != nil {
			return dstOrig, fmt.Errorf("cannot unmarshal blockHeader entries: %w", err)
		}
		src = tail
	}
	if err := validateBlockHeaders(dst[len(dstOrig):]); err != nil {
		return dstOrig, err
	}
	return dst, nil
}

func validateBlockHeaders(bhs []blockHeader) error {
	for i := 1; i < len(bhs); i++ {
		bhCurr := &bhs[i]
		bhPrev := &bhs[i-1]
		if bhCurr.streamID.less(&bhPrev.streamID) {
			return fmt.Errorf("unexpected blockHeader with smaller streamID=%s after bigger streamID=%s at position %d", &bhCurr.streamID, &bhPrev.streamID, i)
		}
		if !bhCurr.streamID.equal(&bhPrev.streamID) {
			continue
		}
		thCurr := bhCurr.timestampsHeader
		thPrev := bhPrev.timestampsHeader
		if thCurr.minTimestamp < thPrev.minTimestamp {
			return fmt.Errorf("unexpected blockHeader with smaller timestamp=%d after bigger timestamp=%d at position %d", thCurr.minTimestamp, thPrev.minTimestamp, i)
		}
	}
	return nil
}

func resetBlockHeaders(bhs []blockHeader) []blockHeader {
	for i := range bhs {
		bhs[i].reset()
	}
	return bhs[:0]
}

func getColumnsHeader() *columnsHeader {
	v := columnsHeaderPool.Get()
	if v == nil {
		return &columnsHeader{}
	}
	return v.(*columnsHeader)
}

func putColumnsHeader(csh *columnsHeader) {
	csh.reset()
	columnsHeaderPool.Put(csh)
}

var columnsHeaderPool sync.Pool

// columnsHeader contains information about columns in a single block.
//
// columnsHeader is stored in the columnsHeaderFilename file.
type columnsHeader struct {
	// columnHeaders contains the information about every column seen in the block.
	columnHeaders []columnHeader

	// constColumns contain fields with constant values across all the block entries.
	constColumns []Field
}

func (csh *columnsHeader) reset() {
	chs := csh.columnHeaders
	for i := range chs {
		chs[i].reset()
	}
	csh.columnHeaders = chs[:0]

	ccs := csh.constColumns
	for i := range ccs {
		ccs[i].Reset()
	}
	csh.constColumns = ccs[:0]
}

func (csh *columnsHeader) getConstColumnValue(name string) string {
	if name == "_msg" {
		name = ""
	}
	ccs := csh.constColumns
	for i := range ccs {
		cc := &ccs[i]
		if cc.Name == name {
			return cc.Value
		}
	}
	return ""
}

func (csh *columnsHeader) getColumnHeader(name string) *columnHeader {
	if name == "_msg" {
		name = ""
	}
	chs := csh.columnHeaders
	for i := range chs {
		ch := &chs[i]
		if ch.name == name {
			return ch
		}
	}
	return nil
}

func (csh *columnsHeader) resizeConstColumns(columnsLen int) []Field {
	ccs := csh.constColumns
	if n := columnsLen - cap(ccs); n > 0 {
		ccs = append(ccs[:cap(ccs)], make([]Field, n)...)
	}
	ccs = ccs[:columnsLen]
	csh.constColumns = ccs
	return ccs
}

func (csh *columnsHeader) resizeColumnHeaders(columnHeadersLen int) []columnHeader {
	chs := csh.columnHeaders
	if n := columnHeadersLen - cap(chs); n > 0 {
		chs = append(chs[:cap(chs)], make([]columnHeader, n)...)
	}
	chs = chs[:columnHeadersLen]
	csh.columnHeaders = chs
	return chs
}

func (csh *columnsHeader) marshal(dst []byte) []byte {
	chs := csh.columnHeaders
	dst = encoding.MarshalVarUint64(dst, uint64(len(chs)))
	for i := range chs {
		dst = chs[i].marshal(dst)
	}

	ccs := csh.constColumns
	dst = encoding.MarshalVarUint64(dst, uint64(len(ccs)))
	for i := range ccs {
		dst = ccs[i].marshal(dst)
	}

	return dst
}

func (csh *columnsHeader) unmarshal(src []byte) error {
	csh.reset()

	// unmarshal columnHeaders
	tail, n, err := encoding.UnmarshalVarUint64(src)
	if err != nil {
		return fmt.Errorf("cannot unmarshal columnHeaders len: %w", err)
	}
	if n > maxColumnsPerBlock {
		return fmt.Errorf("too many column headers: %d; mustn't exceed %d", n, maxColumnsPerBlock)
	}
	src = tail
	chs := csh.resizeColumnHeaders(int(n))
	for i := range chs {
		tail, err = chs[i].unmarshal(src)
		if err != nil {
			return fmt.Errorf("cannot unmarshal columnHeader %d out of %d columnHeaders: %w", i, len(chs), err)
		}
		src = tail
	}
	csh.columnHeaders = chs

	// unmarshal constColumns
	tail, n, err = encoding.UnmarshalVarUint64(src)
	if err != nil {
		return fmt.Errorf("cannot unmarshal constColumns len: %w", err)
	}
	if n+uint64(len(csh.columnHeaders)) > maxColumnsPerBlock {
		return fmt.Errorf("too many columns: %d; mustn't exceed %d", n+uint64(len(csh.columnHeaders)), maxColumnsPerBlock)
	}
	src = tail
	ccs := csh.resizeConstColumns(int(n))
	for i := range ccs {
		tail, err = ccs[i].unmarshal(src)
		if err != nil {
			return fmt.Errorf("cannot unmarshal constColumn %d out of %d columns: %w", i, len(ccs), err)
		}
		src = tail
	}

	// Verify that the src is empty
	if len(src) > 0 {
		return fmt.Errorf("unexpected non-empty tail left after unmarshaling columnsHeader: len(tail)=%d", len(src))
	}

	return nil
}

// columnHeaders contains information for values, which belong to a single label in a single block.
//
// The main column with an empty name is stored in messageValuesFilename,
// while the rest of columns are stored in fieldValuesFilename.
// This allows minimizing disk read IO when filtering by non-message columns.
//
// Every block column contains also a bloom filter for all the tokens stored in the column.
// This bloom filter is used for fast determining whether the given block may contain the given tokens.
//
// Tokens in bloom filter depend on valueType:
//
//   - valueTypeString stores lowercased tokens seen in all the values
//   - valueTypeDict doesn't store anything in the bloom filter, since all the encoded values
//     are available directly in the valuesDict field
//   - valueTypeUint8, valueTypeUint16, valueTypeUint32 and valueTypeUint64 stores encoded uint values
//   - valueTypeFloat64 stores encoded float64 values
//   - valueTypeIPv4 stores encoded into uint32 ips
//   - valueTypeTimestampISO8601 stores encoded into uint64 timestamps
//
// Bloom filters for main column with an empty name is stored in messageBloomFilename,
// while the rest of columns are stored in fieldBloomFilename.
type columnHeader struct {
	// name contains column name aka label name
	name string

	// valueType is the type of values stored in the block
	valueType valueType

	// minValue is the minimum encoded value for uint*, ipv4, timestamp and float64 value in the columnHeader
	//
	// It is used for fast detection of whether the given columnHeader contains values in the given range
	minValue uint64

	// maxValue is the maximum encoded value for uint*, ipv4, timestamp and float64 value in the columnHeader
	//
	// It is used for fast detection of whether the given columnHeader contains values in the given range
	maxValue uint64

	// valuesDict contains unique values for valueType = valueTypeDict
	valuesDict valuesDict

	// valuesOffset contains the offset of the block in either messageValuesFilename or fieldValuesFilename
	valuesOffset uint64

	// valuesSize contains the size of the block in either messageValuesFilename or fieldValuesFilename
	valuesSize uint64

	// bloomFilterOffset contains the offset of the bloom filter in either messageBloomFilename or fieldBloomFilename
	bloomFilterOffset uint64

	// bloomFilterSize contains the size of the bloom filter in either messageBloomFilename or fieldBloomFilename
	bloomFilterSize uint64
}

// reset resets ch
func (ch *columnHeader) reset() {
	ch.name = ""
	ch.valueType = 0

	ch.minValue = 0
	ch.maxValue = 0
	ch.valuesDict.reset()

	ch.valuesOffset = 0
	ch.valuesSize = 0

	ch.bloomFilterOffset = 0
	ch.bloomFilterSize = 0
}

// marshal appends marshaled ch to dst and returns the result.
func (ch *columnHeader) marshal(dst []byte) []byte {
	// check minValue/maxValue
	if ch.valueType == valueTypeFloat64 {
		minValue := math.Float64frombits(ch.minValue)
		maxValue := math.Float64frombits(ch.maxValue)
		if minValue > maxValue {
			logger.Panicf("BUG: minValue=%g must be smaller than maxValue=%g", minValue, maxValue)
		}
	} else {
		if ch.minValue > ch.maxValue {
			logger.Panicf("BUG: minValue=%d must be smaller than maxValue=%d", ch.minValue, ch.maxValue)
		}
	}

	// Encode common fields - ch.name and ch.valueType
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(ch.name))
	dst = append(dst, byte(ch.valueType))

	// Encode other fields depending on ch.valueType
	switch ch.valueType {
	case valueTypeString:
		dst = ch.marshalValuesAndBloomFilters(dst)
	case valueTypeDict:
		dst = ch.valuesDict.marshal(dst)
		dst = ch.marshalValues(dst)
	case valueTypeUint8:
		dst = append(dst, byte(ch.minValue))
		dst = append(dst, byte(ch.maxValue))
		dst = ch.marshalValuesAndBloomFilters(dst)
	case valueTypeUint16:
		dst = encoding.MarshalUint16(dst, uint16(ch.minValue))
		dst = encoding.MarshalUint16(dst, uint16(ch.maxValue))
		dst = ch.marshalValuesAndBloomFilters(dst)
	case valueTypeUint32:
		dst = encoding.MarshalUint32(dst, uint32(ch.minValue))
		dst = encoding.MarshalUint32(dst, uint32(ch.maxValue))
		dst = ch.marshalValuesAndBloomFilters(dst)
	case valueTypeUint64:
		dst = encoding.MarshalUint64(dst, ch.minValue)
		dst = encoding.MarshalUint64(dst, ch.maxValue)
		dst = ch.marshalValuesAndBloomFilters(dst)
	case valueTypeFloat64:
		// float64 values are encoded as uint64 via math.Float64bits()
		dst = encoding.MarshalUint64(dst, ch.minValue)
		dst = encoding.MarshalUint64(dst, ch.maxValue)
		dst = ch.marshalValuesAndBloomFilters(dst)
	case valueTypeIPv4:
		dst = encoding.MarshalUint32(dst, uint32(ch.minValue))
		dst = encoding.MarshalUint32(dst, uint32(ch.maxValue))
		dst = ch.marshalValuesAndBloomFilters(dst)
	case valueTypeTimestampISO8601:
		// timestamps are encoded in nanoseconds
		dst = encoding.MarshalUint64(dst, ch.minValue)
		dst = encoding.MarshalUint64(dst, ch.maxValue)
		dst = ch.marshalValuesAndBloomFilters(dst)
	default:
		logger.Panicf("BUG: unknown valueType=%d", ch.valueType)
	}

	return dst
}

func (ch *columnHeader) marshalValuesAndBloomFilters(dst []byte) []byte {
	dst = ch.marshalValues(dst)
	dst = ch.marshalBloomFilters(dst)
	return dst
}

func (ch *columnHeader) marshalValues(dst []byte) []byte {
	dst = encoding.MarshalVarUint64(dst, ch.valuesOffset)
	dst = encoding.MarshalVarUint64(dst, ch.valuesSize)
	return dst
}

func (ch *columnHeader) marshalBloomFilters(dst []byte) []byte {
	dst = encoding.MarshalVarUint64(dst, ch.bloomFilterOffset)
	dst = encoding.MarshalVarUint64(dst, ch.bloomFilterSize)
	return dst
}

// unmarshal unmarshals ch from src and returns the tail left after unmarshaling.
func (ch *columnHeader) unmarshal(src []byte) ([]byte, error) {
	ch.reset()

	srcOrig := src

	// Unmarshal column name
	tail, data, err := encoding.UnmarshalBytes(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal column name: %w", err)
	}
	// Do not use bytesutil.InternBytes(data) here, since it works slower than the string(data) in prod
	ch.name = string(data)
	src = tail

	// Unmarshal value type
	if len(src) < 1 {
		return srcOrig, fmt.Errorf("cannot unmarshal valueType from 0 bytes for column %q; need at least 1 byte", ch.name)
	}
	ch.valueType = valueType(src[0])
	src = src[1:]

	// Unmarshal the rest of data depending on valueType
	switch ch.valueType {
	case valueTypeString:
		tail, err = ch.unmarshalValuesAndBloomFilters(src)
		if err != nil {
			return srcOrig, fmt.Errorf("cannot unmarshal values and bloom filters at valueTypeString for column %q: %w", ch.name, err)
		}
		src = tail
	case valueTypeDict:
		tail, err = ch.valuesDict.unmarshal(src)
		if err != nil {
			return srcOrig, fmt.Errorf("cannot unmarshal dict at valueTypeDict for column %q: %w", ch.name, err)
		}
		src = tail

		tail, err = ch.unmarshalValues(src)
		if err != nil {
			return srcOrig, fmt.Errorf("cannot unmarshal values at valueTypeDict for column %q: %w", ch.name, err)
		}
		src = tail
	case valueTypeUint8:
		if len(src) < 2 {
			return srcOrig, fmt.Errorf("cannot unmarshal min/max values at valueTypeUint8 from %d bytes for column %q; need at least 2 bytes", len(src), ch.name)
		}
		ch.minValue = uint64(src[0])
		ch.maxValue = uint64(src[1])
		src = src[2:]

		tail, err = ch.unmarshalValuesAndBloomFilters(src)
		if err != nil {
			return srcOrig, fmt.Errorf("cannot unmarshal values and bloom filters at valueTypeUint8 for column %q: %w", ch.name, err)
		}
		src = tail
	case valueTypeUint16:
		if len(src) < 4 {
			return srcOrig, fmt.Errorf("cannot unmarshal min/max values at valueTypeUint16 from %d bytes for column %q; need at least 4 bytes", len(src), ch.name)
		}
		ch.minValue = uint64(encoding.UnmarshalUint16(src))
		ch.maxValue = uint64(encoding.UnmarshalUint16(src[2:]))
		src = src[4:]

		tail, err = ch.unmarshalValuesAndBloomFilters(src)
		if err != nil {
			return srcOrig, fmt.Errorf("cannot unmarshal values and bloom filters at valueTypeUint16 for column %q: %w", ch.name, err)
		}
		src = tail
	case valueTypeUint32:
		if len(src) < 8 {
			return srcOrig, fmt.Errorf("cannot unmarshal min/max values at valueTypeUint32 from %d bytes for column %q; need at least 8 bytes", len(src), ch.name)
		}
		ch.minValue = uint64(encoding.UnmarshalUint32(src))
		ch.maxValue = uint64(encoding.UnmarshalUint32(src[4:]))
		src = src[8:]

		tail, err = ch.unmarshalValuesAndBloomFilters(src)
		if err != nil {
			return srcOrig, fmt.Errorf("cannot unmarshal values and bloom filters at valueTypeUint32 for column %q: %w", ch.name, err)
		}
		src = tail
	case valueTypeUint64:
		if len(src) < 16 {
			return srcOrig, fmt.Errorf("cannot unmarshal min/max values at valueTypeUint64 from %d bytes for column %q; need at least 16 bytes", len(src), ch.name)
		}
		ch.minValue = encoding.UnmarshalUint64(src)
		ch.maxValue = encoding.UnmarshalUint64(src[8:])
		src = src[16:]

		tail, err = ch.unmarshalValuesAndBloomFilters(src)
		if err != nil {
			return srcOrig, fmt.Errorf("cannot unmarshal values and bloom filters at valueTypeUint64 for column %q: %w", ch.name, err)
		}
		src = tail
	case valueTypeFloat64:
		if len(src) < 16 {
			return srcOrig, fmt.Errorf("cannot unmarshal min/max values at valueTypeFloat64 from %d bytes for column %q; need at least 16 bytes", len(src), ch.name)
		}
		// min and max values must be converted to real values with math.Float64frombits() during querying.
		ch.minValue = encoding.UnmarshalUint64(src)
		ch.maxValue = encoding.UnmarshalUint64(src[8:])
		src = src[16:]

		tail, err = ch.unmarshalValuesAndBloomFilters(src)
		if err != nil {
			return srcOrig, fmt.Errorf("cannot unmarshal values and bloom filters at valueTypeFloat64 for column %q: %w", ch.name, err)
		}
		src = tail
	case valueTypeIPv4:
		if len(src) < 8 {
			return srcOrig, fmt.Errorf("cannot unmarshal min/max values at valueTypeIPv4 from %d bytes for column %q; need at least 8 bytes", len(src), ch.name)
		}
		ch.minValue = uint64(encoding.UnmarshalUint32(src))
		ch.maxValue = uint64(encoding.UnmarshalUint32(src[4:]))
		src = src[8:]

		tail, err = ch.unmarshalValuesAndBloomFilters(src)
		if err != nil {
			return srcOrig, fmt.Errorf("cannot unmarshal values and bloom filters at valueTypeIPv4 for column %q: %w", ch.name, err)
		}
		src = tail
	case valueTypeTimestampISO8601:
		if len(src) < 16 {
			return srcOrig, fmt.Errorf("cannot unmarshal min/max values at valueTypeTimestampISO8601 from %d bytes for column %q; need at least 16 bytes",
				len(src), ch.name)
		}
		ch.minValue = encoding.UnmarshalUint64(src)
		ch.maxValue = encoding.UnmarshalUint64(src[8:])
		src = src[16:]

		tail, err = ch.unmarshalValuesAndBloomFilters(src)
		if err != nil {
			return srcOrig, fmt.Errorf("cannot unmarshal values and bloom filters at valueTypeTimestampISO8601 for column %q: %w", ch.name, err)
		}
		src = tail
	default:
		return srcOrig, fmt.Errorf("unexpected valueType=%d for column %q", ch.valueType, ch.name)
	}

	return src, nil
}

func (ch *columnHeader) unmarshalValuesAndBloomFilters(src []byte) ([]byte, error) {
	srcOrig := src

	tail, err := ch.unmarshalValues(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal values: %w", err)
	}
	src = tail

	tail, err = ch.unmarshalBloomFilters(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal bloom filters: %w", err)
	}
	src = tail

	return src, nil
}

func (ch *columnHeader) unmarshalValues(src []byte) ([]byte, error) {
	srcOrig := src

	tail, n, err := encoding.UnmarshalVarUint64(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal valuesOffset: %w", err)
	}
	ch.valuesOffset = n
	src = tail

	tail, n, err = encoding.UnmarshalVarUint64(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal valuesSize: %w", err)
	}
	if n > maxValuesBlockSize {
		return srcOrig, fmt.Errorf("too big valuesSize: %d bytes; mustn't exceed %d bytes", n, maxValuesBlockSize)
	}
	ch.valuesSize = n
	src = tail

	return src, nil
}

func (ch *columnHeader) unmarshalBloomFilters(src []byte) ([]byte, error) {
	srcOrig := src

	tail, n, err := encoding.UnmarshalVarUint64(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal bloomFilterOffset: %w", err)
	}
	ch.bloomFilterOffset = n
	src = tail

	tail, n, err = encoding.UnmarshalVarUint64(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal bloomFilterSize: %w", err)
	}
	if n > maxBloomFilterBlockSize {
		return srcOrig, fmt.Errorf("too big bloomFilterSize: %d bytes; mustn't exceed %d bytes", n, maxBloomFilterBlockSize)
	}
	ch.bloomFilterSize = n
	src = tail

	return src, nil
}

// timestampsHeader contains the information about timestamps block.
type timestampsHeader struct {
	// blockOffset is an offset of timestamps block inside timestampsFilename file
	blockOffset uint64

	// blockSize is the size of the timestamps block inside timestampsFilename file
	blockSize uint64

	// minTimestamp is the mimumum timestamp seen in the block
	minTimestamp int64

	// maxTimestamp is the maximum timestamp seen in the block
	maxTimestamp int64

	// marshalType is the type used for encoding the timestamps block
	marshalType encoding.MarshalType
}

// reset resets th, so it can be reused
func (th *timestampsHeader) reset() {
	th.blockOffset = 0
	th.blockSize = 0
	th.minTimestamp = 0
	th.maxTimestamp = 0
	th.marshalType = 0
}

func (th *timestampsHeader) copyFrom(src *timestampsHeader) {
	th.blockOffset = src.blockOffset
	th.blockSize = src.blockSize
	th.minTimestamp = src.minTimestamp
	th.maxTimestamp = src.maxTimestamp
	th.marshalType = src.marshalType
}

// marshal appends marshaled th to dst and returns the result.
func (th *timestampsHeader) marshal(dst []byte) []byte {
	dst = encoding.MarshalUint64(dst, th.blockOffset)
	dst = encoding.MarshalUint64(dst, th.blockSize)
	dst = encoding.MarshalUint64(dst, uint64(th.minTimestamp))
	dst = encoding.MarshalUint64(dst, uint64(th.maxTimestamp))
	dst = append(dst, byte(th.marshalType))
	return dst
}

// unmarshal unmarshals th from src and returns the tail left after the unmarshaling.
func (th *timestampsHeader) unmarshal(src []byte) ([]byte, error) {
	th.reset()

	if len(src) < 33 {
		return src, fmt.Errorf("cannot unmarshal timestampsHeader from %d bytes; need at least 33 bytes", len(src))
	}

	th.blockOffset = encoding.UnmarshalUint64(src)
	th.blockSize = encoding.UnmarshalUint64(src[8:])
	th.minTimestamp = int64(encoding.UnmarshalUint64(src[16:]))
	th.maxTimestamp = int64(encoding.UnmarshalUint64(src[24:]))
	th.marshalType = encoding.MarshalType(src[32])

	return src[33:], nil
}
