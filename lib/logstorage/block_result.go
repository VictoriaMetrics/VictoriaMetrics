package logstorage

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fastnum"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// blockResult holds results for a single block of log entries.
//
// It is expected that its contents is accessed only from a single goroutine at a time.
type blockResult struct {
	// rowsLen is the number of rows in the given blockResult.
	rowsLen int

	// bs is the associated blockSearch for the given blockResult.
	//
	// bs is nil for the blockResult constructed by pipes.
	bs *blockSearch

	// bm is the associated bitmap for the given blockResult.
	//
	// bm is nil for the blockResult constructed by pipes.
	bm *bitmap

	// a holds all the bytes behind the requested column values in the block.
	a arena

	// valuesBuf holds all the requested column values in the block.
	valuesBuf []string

	// timestampsBuf contains cached timestamps for the selected log entries in the block.
	//
	// timestamps must be obtained via blockResult.getTimestamps() call.
	timestampsBuf []int64

	// csBuf contains requested columns.
	csBuf []blockResultColumn

	// csEmpty contains non-existing columns, which were referenced via getColumnByName()
	csEmpty []blockResultColumn

	// cs contains cached pointers to requested columns returned from getColumns() if csInitialized=true.
	cs []*blockResultColumn

	// csInitialized is set to true if cs is properly initialized and can be returned from getColumns().
	csInitialized bool

	fvecs []filteredValuesEncodedCreator
	svecs []searchValuesEncodedCreator
}

func (br *blockResult) reset() {
	br.rowsLen = 0

	br.cs = nil
	br.bm = nil

	br.a.reset()

	clear(br.valuesBuf)
	br.valuesBuf = br.valuesBuf[:0]

	br.timestampsBuf = br.timestampsBuf[:0]

	clear(br.csBuf)
	br.csBuf = br.csBuf[:0]

	clear(br.csEmpty)
	br.csEmpty = br.csEmpty[:0]

	clear(br.cs)
	br.cs = br.cs[:0]

	br.csInitialized = false

	clear(br.fvecs)
	br.fvecs = br.fvecs[:0]

	clear(br.svecs)
	br.svecs = br.svecs[:0]
}

// clone returns a clone of br, which owns its own data.
func (br *blockResult) clone() *blockResult {
	brNew := &blockResult{}

	brNew.rowsLen = br.rowsLen

	// do not clone br.cs, since it may be updated at any time.
	// do not clone br.bm, since it may be updated at any time.

	cs := br.getColumns()

	// Pre-populate values in every column in order to properly calculate the needed backing buffer size below.
	for _, c := range cs {
		_ = c.getValues(br)
	}

	// Calculate the backing buffer size needed for cloning column values.
	bufLen := 0
	for _, c := range cs {
		bufLen += c.neededBackingBufLen()
	}
	brNew.a.preallocate(bufLen)

	valuesBufLen := 0
	for _, c := range cs {
		valuesBufLen += c.neededBackingValuesBufLen()
	}
	brNew.valuesBuf = make([]string, 0, valuesBufLen)

	srcTimestamps := br.getTimestamps()
	brNew.timestampsBuf = make([]int64, len(srcTimestamps))
	copy(brNew.timestampsBuf, srcTimestamps)
	brNew.checkTimestampsLen()

	csNew := make([]blockResultColumn, len(cs))
	for i, c := range cs {
		csNew[i] = c.clone(brNew)
	}
	brNew.csBuf = csNew

	// do not clone br.csEmpty - it will be populated by the caller via getColumnByName().

	// do not clone br.fvecs and br.svecs, since they may point to external data.

	return brNew
}

// initFromFilterAllColumns initializes br from brSrc by copying rows identified by set bits at bm.
//
// The br is valid until brSrc or bm is updated.
func (br *blockResult) initFromFilterAllColumns(brSrc *blockResult, bm *bitmap) {
	br.reset()

	srcTimestamps := brSrc.getTimestamps()
	dstTimestamps := br.timestampsBuf[:0]
	bm.forEachSetBitReadonly(func(idx int) {
		dstTimestamps = append(dstTimestamps, srcTimestamps[idx])
	})
	br.timestampsBuf = dstTimestamps
	br.rowsLen = len(br.timestampsBuf)

	for _, cSrc := range brSrc.getColumns() {
		br.appendFilteredColumn(brSrc, cSrc, bm)
	}
}

// appendFilteredColumn adds cSrc with the given bm filter to br.
//
// the br is valid until brSrc, cSrc or bm is updated.
func (br *blockResult) appendFilteredColumn(brSrc *blockResult, cSrc *blockResultColumn, bm *bitmap) {
	if br.rowsLen == 0 {
		return
	}
	cDst := blockResultColumn{
		name: cSrc.name,
	}

	if cSrc.isConst {
		cDst.isConst = true
		cDst.valuesEncoded = cSrc.valuesEncoded
	} else if cSrc.isTime {
		cDst.isTime = true
	} else {
		cDst.valueType = cSrc.valueType
		cDst.minValue = cSrc.minValue
		cDst.maxValue = cSrc.maxValue
		cDst.dictValues = cSrc.dictValues
		br.fvecs = append(br.fvecs, filteredValuesEncodedCreator{
			br: brSrc,
			c:  cSrc,
			bm: bm,
		})
		cDst.valuesEncodedCreator = &br.fvecs[len(br.fvecs)-1]
	}

	br.csBuf = append(br.csBuf, cDst)
	br.csInitialized = false
}

type filteredValuesEncodedCreator struct {
	br *blockResult
	c  *blockResultColumn
	bm *bitmap
}

func (fvec *filteredValuesEncodedCreator) newValuesEncoded(br *blockResult) []string {
	valuesEncodedSrc := fvec.c.getValuesEncoded(fvec.br)

	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	fvec.bm.forEachSetBitReadonly(func(idx int) {
		valuesBuf = append(valuesBuf, valuesEncodedSrc[idx])
	})
	br.valuesBuf = valuesBuf

	return valuesBuf[valuesBufLen:]
}

// cloneValues clones the given values into br and returns the cloned values.
func (br *blockResult) cloneValues(values []string) []string {
	if values == nil {
		return nil
	}

	valuesBufLen := len(br.valuesBuf)
	for _, v := range values {
		br.addValue(v)
	}
	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) addValue(v string) {
	valuesBuf := br.valuesBuf
	if len(valuesBuf) > 0 && v == valuesBuf[len(valuesBuf)-1] {
		v = valuesBuf[len(valuesBuf)-1]
	} else {
		v = br.a.copyString(v)
	}
	br.valuesBuf = append(br.valuesBuf, v)
}

// sizeBytes returns the size of br in bytes.
func (br *blockResult) sizeBytes() int {
	n := int(unsafe.Sizeof(*br))

	n += br.a.sizeBytes()
	n += cap(br.valuesBuf) * int(unsafe.Sizeof(br.valuesBuf[0]))
	n += cap(br.timestampsBuf) * int(unsafe.Sizeof(br.timestampsBuf[0]))
	n += cap(br.csBuf) * int(unsafe.Sizeof(br.csBuf[0]))
	n += cap(br.cs) * int(unsafe.Sizeof(br.cs[0]))

	return n
}

// setResultColumns sets the given rcs as br columns.
//
// The br is valid only until rcs are modified.
func (br *blockResult) setResultColumns(rcs []resultColumn, rowsLen int) {
	br.reset()

	br.rowsLen = rowsLen

	for i := range rcs {
		br.addResultColumn(&rcs[i])
	}
}

func (br *blockResult) addResultColumn(rc *resultColumn) {
	if len(rc.values) != br.rowsLen {
		logger.Panicf("BUG: column %q must contain %d rows, but it contains %d rows", rc.name, br.rowsLen, len(rc.values))
	}
	if areConstValues(rc.values) {
		// This optimization allows reducing memory usage after br cloning
		br.csBuf = append(br.csBuf, blockResultColumn{
			name:          rc.name,
			isConst:       true,
			valuesEncoded: rc.values[:1],
		})
	} else {
		br.csBuf = append(br.csBuf, blockResultColumn{
			name:          rc.name,
			valueType:     valueTypeString,
			valuesEncoded: rc.values,
		})
	}
	br.csInitialized = false
}

// initAllColumns initializes all the columns in br.
func (br *blockResult) initAllColumns() {
	unneededColumnNames := br.bs.bsw.so.unneededColumnNames

	if !slices.Contains(unneededColumnNames, "_time") {
		// Add _time column
		br.addTimeColumn()
	}

	if !slices.Contains(unneededColumnNames, "_stream_id") {
		// Add _stream_id column
		br.addStreamIDColumn()
	}

	if !slices.Contains(unneededColumnNames, "_stream") {
		// Add _stream column
		if !br.addStreamColumn() {
			// Skip the current block, since the associated stream tags are missing
			br.reset()
			return
		}
	}

	if !slices.Contains(unneededColumnNames, "_msg") {
		// Add _msg column
		csh := br.bs.getColumnsHeader()
		v := csh.getConstColumnValue("_msg")
		if v != "" {
			br.addConstColumn("_msg", v)
		} else if ch := csh.getColumnHeader("_msg"); ch != nil {
			br.addColumn(ch)
		} else {
			br.addConstColumn("_msg", "")
		}
	}

	// Add other const columns
	csh := br.bs.getColumnsHeader()
	for _, cc := range csh.constColumns {
		if isMsgFieldName(cc.Name) {
			continue
		}
		if !slices.Contains(unneededColumnNames, cc.Name) {
			br.addConstColumn(cc.Name, cc.Value)
		}
	}

	// Add other non-const columns
	chs := csh.columnHeaders
	for i := range chs {
		ch := &chs[i]
		if isMsgFieldName(ch.name) {
			continue
		}
		if !slices.Contains(unneededColumnNames, ch.name) {
			br.addColumn(ch)
		}
	}

	br.csInitFast()
}

// initRequestedColumns initialized only requested columns in br.
func (br *blockResult) initRequestedColumns() {
	for _, columnName := range br.bs.bsw.so.neededColumnNames {
		switch columnName {
		case "_stream_id":
			br.addStreamIDColumn()
		case "_stream":
			if !br.addStreamColumn() {
				// Skip the current block, since the associated stream tags are missing.
				br.reset()
				return
			}
		case "_time":
			br.addTimeColumn()
		default:
			csh := br.bs.getColumnsHeader()
			v := csh.getConstColumnValue(columnName)
			if v != "" {
				br.addConstColumn(columnName, v)
			} else if ch := csh.getColumnHeader(columnName); ch != nil {
				br.addColumn(ch)
			} else {
				br.addConstColumn(columnName, "")
			}
		}
	}

	br.csInitFast()
}

// mustInit initializes br with the given bs and bm.
//
// br is valid until bs or bm changes.
func (br *blockResult) mustInit(bs *blockSearch, bm *bitmap) {
	br.reset()

	br.rowsLen = bm.onesCount()
	if br.rowsLen == 0 {
		return
	}

	br.bs = bs
	br.bm = bm
}

// intersectsTimeRange returns true if br timestamps intersect (minTimestamp .. maxTimestamp) time range.
func (br *blockResult) intersectsTimeRange(minTimestamp, maxTimestamp int64) bool {
	return minTimestamp < br.getMaxTimestamp(minTimestamp) && maxTimestamp > br.getMinTimestamp(maxTimestamp)
}

func (br *blockResult) getMinTimestamp(minTimestamp int64) int64 {
	if br.bs != nil {
		bh := &br.bs.bsw.bh
		if bh.rowsCount == uint64(br.rowsLen) {
			return min(minTimestamp, bh.timestampsHeader.minTimestamp)
		}
		if minTimestamp <= bh.timestampsHeader.minTimestamp {
			return minTimestamp
		}
	}

	// Slow path - need to scan timestamps
	timestamps := br.getTimestamps()
	for _, timestamp := range timestamps {
		if timestamp < minTimestamp {
			minTimestamp = timestamp
		}
	}
	return minTimestamp
}

func (br *blockResult) getMaxTimestamp(maxTimestamp int64) int64 {
	if br.bs != nil {
		bh := &br.bs.bsw.bh
		if bh.rowsCount == uint64(br.rowsLen) {
			return max(maxTimestamp, bh.timestampsHeader.maxTimestamp)
		}
		if maxTimestamp >= bh.timestampsHeader.maxTimestamp {
			return maxTimestamp
		}
	}

	// Slow path - need to scan timestamps
	timestamps := br.getTimestamps()
	for i := len(timestamps) - 1; i >= 0; i-- {
		if timestamps[i] > maxTimestamp {
			maxTimestamp = timestamps[i]
		}
	}
	return maxTimestamp
}

func (br *blockResult) getTimestamps() []int64 {
	if br.rowsLen > 0 && len(br.timestampsBuf) == 0 {
		br.initTimestamps()
	}
	return br.timestampsBuf
}

func (br *blockResult) initTimestamps() {
	if br.bs == nil {
		br.timestampsBuf = fastnum.AppendInt64Zeros(br.timestampsBuf[:0], br.rowsLen)
		return
	}

	srcTimestamps := br.bs.getTimestamps()
	if br.bm.areAllBitsSet() {
		// Fast path - all the rows in the block are selected, so copy all the timestamps without any filtering.
		br.timestampsBuf = append(br.timestampsBuf[:0], srcTimestamps...)
		br.checkTimestampsLen()
		return
	}

	// Slow path - copy only the needed timestamps to br according to filter results.
	dstTimestamps := br.timestampsBuf[:0]
	br.bm.forEachSetBitReadonly(func(idx int) {
		ts := srcTimestamps[idx]
		dstTimestamps = append(dstTimestamps, ts)
	})
	br.timestampsBuf = dstTimestamps
	br.checkTimestampsLen()
}

func (br *blockResult) checkTimestampsLen() {
	if len(br.timestampsBuf) != br.rowsLen {
		logger.Panicf("BUG: unexpected number of timestamps; got %d; want %d", len(br.timestampsBuf), br.rowsLen)
	}
}

func (br *blockResult) newValuesEncodedFromColumnHeader(bs *blockSearch, bm *bitmap, ch *columnHeader) []string {
	valuesBufLen := len(br.valuesBuf)

	switch ch.valueType {
	case valueTypeString:
		visitValuesReadonly(bs, ch, bm, br.addValue)
	case valueTypeDict:
		visitValuesReadonly(bs, ch, bm, func(v string) {
			if len(v) != 1 {
				logger.Panicf("FATAL: %s: unexpected dict value size for column %q; got %d bytes; want 1 byte", bs.partPath(), ch.name, len(v))
			}
			dictIdx := v[0]
			if int(dictIdx) >= len(ch.valuesDict.values) {
				logger.Panicf("FATAL: %s: too big dict index for column %q: %d; should be smaller than %d", bs.partPath(), ch.name, dictIdx, len(ch.valuesDict.values))
			}
			br.addValue(v)
		})
	case valueTypeUint8:
		visitValuesReadonly(bs, ch, bm, func(v string) {
			if len(v) != 1 {
				logger.Panicf("FATAL: %s: unexpected size for uint8 column %q; got %d bytes; want 1 byte", bs.partPath(), ch.name, len(v))
			}
			br.addValue(v)
		})
	case valueTypeUint16:
		visitValuesReadonly(bs, ch, bm, func(v string) {
			if len(v) != 2 {
				logger.Panicf("FATAL: %s: unexpected size for uint16 column %q; got %d bytes; want 2 bytes", bs.partPath(), ch.name, len(v))
			}
			br.addValue(v)
		})
	case valueTypeUint32:
		visitValuesReadonly(bs, ch, bm, func(v string) {
			if len(v) != 4 {
				logger.Panicf("FATAL: %s: unexpected size for uint32 column %q; got %d bytes; want 4 bytes", bs.partPath(), ch.name, len(v))
			}
			br.addValue(v)
		})
	case valueTypeUint64:
		visitValuesReadonly(bs, ch, bm, func(v string) {
			if len(v) != 8 {
				logger.Panicf("FATAL: %s: unexpected size for uint64 column %q; got %d bytes; want 8 bytes", bs.partPath(), ch.name, len(v))
			}
			br.addValue(v)
		})
	case valueTypeFloat64:
		visitValuesReadonly(bs, ch, bm, func(v string) {
			if len(v) != 8 {
				logger.Panicf("FATAL: %s: unexpected size for float64 column %q; got %d bytes; want 8 bytes", bs.partPath(), ch.name, len(v))
			}
			br.addValue(v)
		})
	case valueTypeIPv4:
		visitValuesReadonly(bs, ch, bm, func(v string) {
			if len(v) != 4 {
				logger.Panicf("FATAL: %s: unexpected size for ipv4 column %q; got %d bytes; want 4 bytes", bs.partPath(), ch.name, len(v))
			}
			br.addValue(v)
		})
	case valueTypeTimestampISO8601:
		visitValuesReadonly(bs, ch, bm, func(v string) {
			if len(v) != 8 {
				logger.Panicf("FATAL: %s: unexpected size for timestmap column %q; got %d bytes; want 8 bytes", bs.partPath(), ch.name, len(v))
			}
			br.addValue(v)
		})
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d for column %q", bs.partPath(), ch.valueType, ch.name)
	}

	return br.valuesBuf[valuesBufLen:]
}

// addColumn adds column for the given ch to br.
//
// The added column is valid until ch is changed.
func (br *blockResult) addColumn(ch *columnHeader) {
	br.csBuf = append(br.csBuf, blockResultColumn{
		name:       getCanonicalColumnName(ch.name),
		valueType:  ch.valueType,
		minValue:   ch.minValue,
		maxValue:   ch.maxValue,
		dictValues: ch.valuesDict.values,
	})
	c := &br.csBuf[len(br.csBuf)-1]

	br.svecs = append(br.svecs, searchValuesEncodedCreator{
		bs: br.bs,
		bm: br.bm,
		ch: ch,
	})
	c.valuesEncodedCreator = &br.svecs[len(br.svecs)-1]
	br.csInitialized = false
}

type searchValuesEncodedCreator struct {
	bs *blockSearch
	bm *bitmap
	ch *columnHeader
}

func (svec *searchValuesEncodedCreator) newValuesEncoded(br *blockResult) []string {
	return br.newValuesEncodedFromColumnHeader(svec.bs, svec.bm, svec.ch)
}

func (br *blockResult) addTimeColumn() {
	br.csBuf = append(br.csBuf, blockResultColumn{
		name:   "_time",
		isTime: true,
	})
	br.csInitialized = false
}

func (br *blockResult) addStreamIDColumn() {
	bb := bbPool.Get()
	bb.B = br.bs.bsw.bh.streamID.marshalString(bb.B)
	br.addConstColumn("_stream_id", bytesutil.ToUnsafeString(bb.B))
	bbPool.Put(bb)
}

func (br *blockResult) addStreamColumn() bool {
	streamStr := br.bs.getStreamStr()
	if streamStr == "" {
		return false
	}
	br.addConstColumn("_stream", streamStr)
	return true
}

func (br *blockResult) addConstColumn(name, value string) {
	nameCopy := br.a.copyString(name)

	valuesBufLen := len(br.valuesBuf)
	br.addValue(value)
	valuesEncoded := br.valuesBuf[valuesBufLen:]

	br.csBuf = append(br.csBuf, blockResultColumn{
		name:          nameCopy,
		isConst:       true,
		valuesEncoded: valuesEncoded,
	})
	br.csInitialized = false
}

func (br *blockResult) newValuesBucketedForColumn(c *blockResultColumn, bf *byStatsField) []string {
	if c.isConst {
		v := c.valuesEncoded[0]
		return br.getBucketedConstValues(v, bf)
	}
	if c.isTime {
		return br.getBucketedTimestampValues(bf)
	}

	valuesEncoded := c.getValuesEncoded(br)

	switch c.valueType {
	case valueTypeString:
		return br.getBucketedStringValues(valuesEncoded, bf)
	case valueTypeDict:
		return br.getBucketedDictValues(valuesEncoded, c.dictValues, bf)
	case valueTypeUint8:
		return br.getBucketedUint8Values(valuesEncoded, bf)
	case valueTypeUint16:
		return br.getBucketedUint16Values(valuesEncoded, bf)
	case valueTypeUint32:
		return br.getBucketedUint32Values(valuesEncoded, bf)
	case valueTypeUint64:
		return br.getBucketedUint64Values(valuesEncoded, bf)
	case valueTypeFloat64:
		return br.getBucketedFloat64Values(valuesEncoded, bf)
	case valueTypeIPv4:
		return br.getBucketedIPv4Values(valuesEncoded, bf)
	case valueTypeTimestampISO8601:
		return br.getBucketedTimestampISO8601Values(valuesEncoded, bf)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return nil
	}
}

func (br *blockResult) getBucketedConstValues(v string, bf *byStatsField) []string {
	if v == "" {
		// Fast path - return a slice of empty strings without constructing the slice.
		return getEmptyStrings(br.rowsLen)
	}

	// Slower path - construct slice of identical values with the length equal to br.rowsLen

	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	v = br.getBucketedValue(v, bf)
	for i := 0; i < br.rowsLen; i++ {
		valuesBuf = append(valuesBuf, v)
	}

	br.valuesBuf = valuesBuf

	return valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedTimestampValues(bf *byStatsField) []string {
	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	timestamps := br.getTimestamps()
	var s string

	if !bf.hasBucketConfig() {
		for i := range timestamps {
			if i > 0 && timestamps[i-1] == timestamps[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			bufLen := len(buf)
			buf = marshalTimestampRFC3339NanoString(buf, timestamps[i])
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := int64(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffsetInt := int64(bf.bucketOffset)

		timestampPrev := int64(0)
		for i := range timestamps {
			if i > 0 && timestamps[i-1] == timestamps[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			timestamp := timestamps[i]
			timestamp -= bucketOffsetInt
			if bf.bucketSizeStr == "month" {
				timestamp = truncateTimestampToMonth(timestamp)
			} else if bf.bucketSizeStr == "year" {
				timestamp = truncateTimestampToYear(timestamp)
			} else {
				timestamp -= timestamp % bucketSizeInt
			}
			timestamp += bucketOffsetInt

			if i > 0 && timestampPrev == timestamp {
				valuesBuf = append(valuesBuf, s)
				continue
			}
			timestampPrev = timestamp

			bufLen := len(buf)
			buf = marshalTimestampRFC3339NanoString(buf, timestamp)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.a.b = buf
	br.valuesBuf = valuesBuf

	return valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedStringValues(values []string, bf *byStatsField) []string {
	if !bf.hasBucketConfig() {
		return values
	}

	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string
	for i := range values {
		if i > 0 && values[i-1] == values[i] {
			valuesBuf = append(valuesBuf, s)
			continue
		}

		s = br.getBucketedValue(values[i], bf)
		valuesBuf = append(valuesBuf, s)
	}

	br.valuesBuf = valuesBuf

	return valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedDictValues(valuesEncoded, dictValues []string, bf *byStatsField) []string {
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	dictValues = br.getBucketedStringValues(dictValues, bf)
	for _, v := range valuesEncoded {
		dictIdx := v[0]
		valuesBuf = append(valuesBuf, dictValues[dictIdx])
	}

	br.valuesBuf = valuesBuf

	return valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedUint8Values(valuesEncoded []string, bf *byStatsField) []string {
	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if !bf.hasBucketConfig() {
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := unmarshalUint8(v)
			bufLen := len(buf)
			buf = marshalUint8String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := uint64(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffsetInt := uint64(int64(bf.bucketOffset))

		nPrev := uint64(0)
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := uint64(unmarshalUint8(v))
			n -= bucketOffsetInt
			n -= n % bucketSizeInt
			n += bucketOffsetInt

			if i > 0 && nPrev == n {
				valuesBuf = append(valuesBuf, s)
				continue
			}
			nPrev = n

			bufLen := len(buf)
			buf = marshalUint64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedUint16Values(valuesEncoded []string, bf *byStatsField) []string {
	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if !bf.hasBucketConfig() {
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := unmarshalUint16(v)
			bufLen := len(buf)
			buf = marshalUint16String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := uint64(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffsetInt := uint64(int64(bf.bucketOffset))

		nPrev := uint64(0)
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := uint64(unmarshalUint16(v))
			n -= bucketOffsetInt
			n -= n % bucketSizeInt
			n += bucketOffsetInt

			if i > 0 && nPrev == n {
				valuesBuf = append(valuesBuf, s)
				continue
			}
			nPrev = n

			bufLen := len(buf)
			buf = marshalUint64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedUint32Values(valuesEncoded []string, bf *byStatsField) []string {
	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if !bf.hasBucketConfig() {
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := unmarshalUint32(v)
			bufLen := len(buf)
			buf = marshalUint32String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := uint64(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffsetInt := uint64(int64(bf.bucketOffset))

		nPrev := uint64(0)
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := uint64(unmarshalUint32(v))
			n -= bucketOffsetInt
			n -= n % bucketSizeInt
			n += bucketOffsetInt

			if i > 0 && nPrev == n {
				valuesBuf = append(valuesBuf, s)
				continue
			}
			nPrev = n

			bufLen := len(buf)
			buf = marshalUint64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedUint64Values(valuesEncoded []string, bf *byStatsField) []string {
	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if !bf.hasBucketConfig() {
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := unmarshalUint64(v)
			bufLen := len(buf)
			buf = marshalUint64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := uint64(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffsetInt := uint64(int64(bf.bucketOffset))

		nPrev := uint64(0)
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := unmarshalUint64(v)
			n -= bucketOffsetInt
			n -= n % bucketSizeInt
			n += bucketOffsetInt

			if i > 0 && nPrev == n {
				valuesBuf = append(valuesBuf, s)
				continue
			}
			nPrev = n

			bufLen := len(buf)
			buf = marshalUint64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedFloat64Values(valuesEncoded []string, bf *byStatsField) []string {
	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if !bf.hasBucketConfig() {
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			f := unmarshalFloat64(v)

			bufLen := len(buf)
			buf = marshalFloat64String(buf, f)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSize := bf.bucketSize
		if bucketSize <= 0 {
			bucketSize = 1
		}

		_, e := decimal.FromFloat(bucketSize)
		p10 := math.Pow10(int(-e))
		bucketSizeP10 := int64(bucketSize * p10)

		fPrev := float64(0)
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			f := unmarshalFloat64(v)

			f -= bf.bucketOffset

			// emulate f % bucketSize for float64 values
			fP10 := int64(f * p10)
			fP10 -= fP10 % bucketSizeP10
			f = float64(fP10) / p10

			f += bf.bucketOffset

			if fPrev == f {
				valuesBuf = append(valuesBuf, s)
				continue
			}
			fPrev = f

			bufLen := len(buf)
			buf = marshalFloat64String(buf, f)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedIPv4Values(valuesEncoded []string, bf *byStatsField) []string {
	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if !bf.hasBucketConfig() {
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			ip := unmarshalIPv4(v)
			bufLen := len(buf)
			buf = marshalIPv4String(buf, ip)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := uint32(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffsetInt := uint32(int32(bf.bucketOffset))

		nPrev := uint32(0)
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := unmarshalIPv4(v)
			n -= bucketOffsetInt
			n -= n % bucketSizeInt
			n += bucketOffsetInt

			if i > 0 && nPrev == n {
				valuesBuf = append(valuesBuf, s)
				continue
			}
			nPrev = n

			bufLen := len(buf)
			buf = marshalIPv4String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedTimestampISO8601Values(valuesEncoded []string, bf *byStatsField) []string {
	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if !bf.hasBucketConfig() {
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := unmarshalTimestampISO8601(v)

			bufLen := len(buf)
			buf = marshalTimestampISO8601String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := int64(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffsetInt := int64(bf.bucketOffset)

		timestampPrev := int64(0)
		bb := bbPool.Get()
		for i, v := range valuesEncoded {
			if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			timestamp := unmarshalTimestampISO8601(v)
			timestamp -= bucketOffsetInt
			if bf.bucketSizeStr == "month" {
				timestamp = truncateTimestampToMonth(timestamp)
			} else if bf.bucketSizeStr == "year" {
				timestamp = truncateTimestampToYear(timestamp)
			} else {
				timestamp -= timestamp % bucketSizeInt
			}
			timestamp -= timestamp % bucketSizeInt
			timestamp += bucketOffsetInt

			if timestampPrev == timestamp {
				valuesBuf = append(valuesBuf, s)
				continue
			}
			timestampPrev = timestamp

			bufLen := len(buf)
			buf = marshalTimestampISO8601String(buf, timestamp)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
		bbPool.Put(bb)
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return valuesBuf[valuesBufLen:]
}

// getBucketedValue returns bucketed s according to the given bf
func (br *blockResult) getBucketedValue(s string, bf *byStatsField) string {
	if !bf.hasBucketConfig() {
		return s
	}
	if len(s) == 0 {
		return s
	}

	c := s[0]
	if (c < '0' || c > '9') && c != '-' {
		// Fast path - the value cannot be bucketed, since it starts with unexpected chars.
		return s
	}

	if f, ok := tryParseFloat64(s); ok {
		bucketSize := bf.bucketSize
		if bucketSize <= 0 {
			bucketSize = 1
		}

		f -= bf.bucketOffset

		// emulate f % bucketSize for float64 values
		_, e := decimal.FromFloat(bucketSize)
		p10 := math.Pow10(int(-e))
		fP10 := int64(f * p10)
		fP10 -= fP10 % int64(bucketSize*p10)
		f = float64(fP10) / p10

		f += bf.bucketOffset

		buf := br.a.b
		bufLen := len(buf)
		buf = marshalFloat64String(buf, f)
		br.a.b = buf
		return bytesutil.ToUnsafeString(buf[bufLen:])
	}

	// There is no need in calling tryParseTimestampISO8601 here, since TryParseTimestampRFC3339Nano
	// should successfully parse ISO8601 timestamps.
	if timestamp, ok := TryParseTimestampRFC3339Nano(s); ok {
		bucketSizeInt := int64(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffset := int64(bf.bucketOffset)

		timestamp -= bucketOffset
		if bf.bucketSizeStr == "month" {
			timestamp = truncateTimestampToMonth(timestamp)
		} else if bf.bucketSizeStr == "year" {
			timestamp = truncateTimestampToYear(timestamp)
		} else {
			timestamp -= timestamp % bucketSizeInt
		}
		timestamp += bucketOffset

		buf := br.a.b
		bufLen := len(buf)
		buf = marshalTimestampRFC3339NanoString(buf, timestamp)
		br.a.b = buf
		return bytesutil.ToUnsafeString(buf[bufLen:])
	}

	if n, ok := tryParseIPv4(s); ok {
		bucketSizeInt := uint32(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffset := uint32(int32(bf.bucketOffset))

		n -= bucketOffset
		n -= n % bucketSizeInt
		n += bucketOffset

		buf := br.a.b
		bufLen := len(buf)
		buf = marshalIPv4String(buf, n)
		br.a.b = buf
		return bytesutil.ToUnsafeString(buf[bufLen:])
	}

	if nsecs, ok := tryParseDuration(s); ok {
		bucketSizeInt := int64(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffset := int64(bf.bucketOffset)

		nsecs -= bucketOffset
		nsecs -= nsecs % bucketSizeInt
		nsecs += bucketOffset

		buf := br.a.b
		bufLen := len(buf)
		buf = marshalDurationString(buf, nsecs)
		br.a.b = buf
		return bytesutil.ToUnsafeString(buf[bufLen:])
	}

	// Couldn't parse s, so return it as is.
	return s
}

// copyColumns copies columns from srcColumnNames to dstColumnNames.
func (br *blockResult) copyColumns(srcColumnNames, dstColumnNames []string) {
	for i, srcName := range srcColumnNames {
		br.copySingleColumn(srcName, dstColumnNames[i])
	}
}

func (br *blockResult) copySingleColumn(srcName, dstName string) {
	found := false
	cs := br.getColumns()
	csBufLen := len(br.csBuf)
	for _, c := range cs {
		if c.name != dstName {
			br.csBuf = append(br.csBuf, *c)
		}
		if c.name == srcName {
			cCopy := *c
			cCopy.name = dstName
			br.csBuf = append(br.csBuf, cCopy)
			found = true
		}
	}
	if !found {
		br.addConstColumn(dstName, "")
	}
	br.csBuf = append(br.csBuf[:0], br.csBuf[csBufLen:]...)
	br.csInitialized = false
}

// renameColumns renames columns from srcColumnNames to dstColumnNames.
func (br *blockResult) renameColumns(srcColumnNames, dstColumnNames []string) {
	for i, srcName := range srcColumnNames {
		br.renameSingleColumn(srcName, dstColumnNames[i])
	}
}

func (br *blockResult) renameSingleColumn(srcName, dstName string) {
	found := false
	cs := br.getColumns()
	csBufLen := len(br.csBuf)
	for _, c := range cs {
		if c.name == srcName {
			cCopy := *c
			cCopy.name = dstName
			br.csBuf = append(br.csBuf, cCopy)
			found = true
		} else if c.name != dstName {
			br.csBuf = append(br.csBuf, *c)
		}
	}
	if !found {
		br.addConstColumn(dstName, "")
	}
	br.csBuf = append(br.csBuf[:0], br.csBuf[csBufLen:]...)
	br.csInitialized = false
}

// deleteColumns deletes columns with the given columnNames.
func (br *blockResult) deleteColumns(columnNames []string) {
	if len(columnNames) == 0 {
		return
	}

	cs := br.getColumns()
	csBufLen := len(br.csBuf)
	for _, c := range cs {
		if !slices.Contains(columnNames, c.name) {
			br.csBuf = append(br.csBuf, *c)
		}
	}

	br.csBuf = append(br.csBuf[:0], br.csBuf[csBufLen:]...)
	br.csInitialized = false
}

// setColumns sets the resulting columns to the given columnNames.
func (br *blockResult) setColumns(columnNames []string) {
	if br.areSameColumns(columnNames) {
		// Fast path - nothing to change.
		return
	}

	// Slow path - construct the requested columns
	cs := br.getColumns()
	csBufLen := len(br.csBuf)
	for _, c := range cs {
		if slices.Contains(columnNames, c.name) {
			br.csBuf = append(br.csBuf, *c)
		}
	}

	for _, columnName := range columnNames {
		if idx := getBlockResultColumnIdxByName(cs, columnName); idx < 0 {
			br.addConstColumn(columnName, "")
		}
	}

	br.csBuf = append(br.csBuf[:0], br.csBuf[csBufLen:]...)
	br.csInitialized = false
}

func (br *blockResult) areSameColumns(columnNames []string) bool {
	cs := br.getColumns()
	if len(cs) != len(columnNames) {
		return false
	}
	for i, c := range cs {
		if c.name != columnNames[i] {
			return false
		}
	}
	return true
}

func (br *blockResult) getColumnByName(columnName string) *blockResultColumn {
	if columnName == "" {
		columnName = "_msg"
	}
	cs := br.getColumns()

	idx := getBlockResultColumnIdxByName(cs, columnName)
	if idx >= 0 {
		return cs[idx]
	}

	// Search for empty column with the given name
	csEmpty := br.csEmpty
	for i := range csEmpty {
		if csEmpty[i].name == columnName {
			return &csEmpty[i]
		}
	}

	// Create missing empty column
	br.csEmpty = append(br.csEmpty, blockResultColumn{
		name:          br.a.copyString(columnName),
		isConst:       true,
		valuesEncoded: getEmptyStrings(1),
	})
	return &br.csEmpty[len(br.csEmpty)-1]
}

func (br *blockResult) getColumns() []*blockResultColumn {
	if !br.csInitialized {
		br.csInit()
	}
	return br.cs
}

func (br *blockResult) csInit() {
	csBuf := br.csBuf
	clear(br.cs)
	cs := br.cs[:0]
	for i := range csBuf {
		c := &csBuf[i]
		idx := getBlockResultColumnIdxByName(cs, c.name)
		if idx >= 0 {
			cs[idx] = c
		} else {
			cs = append(cs, c)
		}
	}
	br.cs = cs
	br.csInitialized = true
}

func (br *blockResult) csInitFast() {
	csBuf := br.csBuf
	clear(br.cs)
	cs := slicesutil.SetLength(br.cs, len(csBuf))
	for i := range csBuf {
		cs[i] = &csBuf[i]
	}
	br.cs = cs
	br.csInitialized = true
}

func getBlockResultColumnIdxByName(cs []*blockResultColumn, name string) int {
	for i, c := range cs {
		if c.name == name {
			return i
		}
	}
	return -1
}

func (br *blockResult) skipRows(skipRows int) {
	timestamps := br.getTimestamps()
	br.timestampsBuf = append(br.timestampsBuf[:0], timestamps[skipRows:]...)
	br.rowsLen -= skipRows
	br.checkTimestampsLen()

	for _, c := range br.getColumns() {
		if c.values != nil {
			c.values = append(c.values[:0], c.values[skipRows:]...)
		}
		if c.isConst {
			continue
		}

		valuesEncoded := c.getValuesEncoded(br)
		if valuesEncoded != nil {
			c.valuesEncoded = append(valuesEncoded[:0], valuesEncoded[skipRows:]...)
		}
		if c.valuesBucketed != nil {
			c.valuesBucketed = append(c.valuesBucketed[:0], c.valuesBucketed[skipRows:]...)
		}
	}
}

func (br *blockResult) truncateRows(keepRows int) {
	timestamps := br.getTimestamps()
	br.timestampsBuf = append(br.timestampsBuf[:0], timestamps[:keepRows]...)
	br.rowsLen = keepRows
	br.checkTimestampsLen()

	for _, c := range br.getColumns() {
		if c.values != nil {
			c.values = c.values[:keepRows]
		}
		if c.isConst {
			continue
		}

		valuesEncoded := c.getValuesEncoded(br)
		if valuesEncoded != nil {
			c.valuesEncoded = valuesEncoded[:keepRows]
		}
		if c.valuesBucketed != nil {
			c.valuesBucketed = append(c.valuesBucketed[:0], c.valuesBucketed[keepRows:]...)
		}
	}
}

// blockResultColumn represents named column from blockResult.
//
// blockResultColumn doesn't own any referred data - all the referred data must be owned by blockResult.
// This simplifies copying, resetting and re-using of the struct.
type blockResultColumn struct {
	// name is column name
	name string

	// isConst is set to true if the column is const.
	//
	// The column value is stored in valuesEncoded[0]
	isConst bool

	// isTime is set to true if the column contains _time values.
	//
	// The column values are stored in blockResult.timestamps, while valuesEncoded is nil.
	isTime bool

	// valueType is the type of non-cost value
	valueType valueType

	// minValue is the minimum encoded value for uint*, ipv4, timestamp and float64 value
	//
	// It is used for fast detection of whether the given column contains values in the given range
	minValue uint64

	// maxValue is the maximum encoded value for uint*, ipv4, timestamp and float64 value
	//
	// It is used for fast detection of whether the given column contains values in the given range
	maxValue uint64

	// dictValues contains dict values for valueType=valueTypeDict.
	dictValues []string

	// valuesEncoded contains encoded values for non-const and non-time column after getValuesEncoded() call
	valuesEncoded []string

	// values contains decoded values after getValues() call
	values []string

	// valuesBucketed contains values after getValuesBucketed() call
	valuesBucketed []string

	// valuesEncodedCreator must return valuesEncoded.
	//
	// This interface must be set for non-const and non-time columns if valuesEncoded field isn't set.
	valuesEncodedCreator columnValuesEncodedCreator

	// bucketSizeStr contains bucketSizeStr for valuesBucketed
	bucketSizeStr string

	// bucketOffsetStr contains bucketOffset for valuesBucketed
	bucketOffsetStr string
}

// columnValuesEncodedCreator must return encoded values for the current column.
type columnValuesEncodedCreator interface {
	newValuesEncoded(br *blockResult) []string
}

// clone returns a clone of c backed by data from br.
//
// It is expected that c.valuesEncoded is already initialized for non-time column.
//
// The clone is valid until br is reset.
func (c *blockResultColumn) clone(br *blockResult) blockResultColumn {
	var cNew blockResultColumn

	cNew.name = br.a.copyString(c.name)

	cNew.isConst = c.isConst
	cNew.isTime = c.isTime
	cNew.valueType = c.valueType
	cNew.minValue = c.minValue
	cNew.maxValue = c.maxValue

	cNew.dictValues = br.cloneValues(c.dictValues)

	if !c.isTime && c.valuesEncoded == nil {
		logger.Panicf("BUG: valuesEncoded must be non-nil for non-time column %q; isConst=%v; valueType=%d", c.name, c.isConst, c.valueType)
	}
	cNew.valuesEncoded = br.cloneValues(c.valuesEncoded)

	if c.valueType != valueTypeString {
		cNew.values = br.cloneValues(c.values)
	}
	cNew.valuesBucketed = br.cloneValues(c.valuesBucketed)

	// Do not copy c.valuesEncodedCreator, since it may refer to data, which may change over time.
	// We already copied c.valuesEncoded, so cNew.valuesEncodedCreator must be nil.

	cNew.bucketSizeStr = c.bucketSizeStr
	cNew.bucketOffsetStr = c.bucketOffsetStr

	return cNew
}

func (c *blockResultColumn) neededBackingBufLen() int {
	n := len(c.name)
	n += valuesSizeBytes(c.dictValues)
	n += valuesSizeBytes(c.valuesEncoded)
	if c.valueType != valueTypeString {
		n += valuesSizeBytes(c.values)
	}
	n += valuesSizeBytes(c.valuesBucketed)
	return n
}

func (c *blockResultColumn) neededBackingValuesBufLen() int {
	n := 0
	n += len(c.dictValues)
	n += len(c.valuesEncoded)
	if c.valueType != valueTypeString {
		n += len(c.values)
	}
	n += len(c.valuesBucketed)
	return n
}

func valuesSizeBytes(values []string) int {
	n := 0
	for _, v := range values {
		n += len(v)
	}
	return n
}

// getValueAtRow returns value for the value at the given rowIdx.
//
// The returned value is valid until br.reset() is called.
func (c *blockResultColumn) getValueAtRow(br *blockResult, rowIdx int) string {
	if c.isConst {
		// Fast path for const column.
		return c.valuesEncoded[0]
	}
	if c.values != nil {
		// Fast path, which avoids call overhead for getValues().
		return c.values[rowIdx]
	}

	// Slow path - decode all the values and return the given value.
	values := c.getValues(br)
	return values[rowIdx]
}

// getValuesBucketed returns values for the given column, bucketed according to bf.
//
// The returned values are valid until br.reset() is called.
func (c *blockResultColumn) getValuesBucketed(br *blockResult, bf *byStatsField) []string {
	if !bf.hasBucketConfig() {
		return c.getValues(br)
	}
	if values := c.valuesBucketed; values != nil && c.bucketSizeStr == bf.bucketSizeStr && c.bucketOffsetStr == bf.bucketOffsetStr {
		return values
	}

	c.valuesBucketed = br.newValuesBucketedForColumn(c, bf)
	c.bucketSizeStr = bf.bucketSizeStr
	c.bucketOffsetStr = bf.bucketOffsetStr
	return c.valuesBucketed
}

// getValues returns values for the given column.
//
// The returned values are valid until br.reset() is called.
func (c *blockResultColumn) getValues(br *blockResult) []string {
	if values := c.values; values != nil {
		return values
	}

	c.values = br.newValuesBucketedForColumn(c, zeroByStatsField)
	return c.values
}

// getValuesEncoded returns encoded values for the given column.
//
// The returned values are valid until br.reset() is called.
func (c *blockResultColumn) getValuesEncoded(br *blockResult) []string {
	if c.isTime {
		return nil
	}

	if c.valuesEncoded == nil {
		c.valuesEncoded = c.valuesEncodedCreator.newValuesEncoded(br)
	}
	return c.valuesEncoded
}

// forEachDictValue calls f for every value in the column dictionary.
func (c *blockResultColumn) forEachDictValue(br *blockResult, f func(v string)) {
	if c.valueType != valueTypeDict {
		logger.Panicf("BUG: unexpected column valueType=%d; want %d", c.valueType, valueTypeDict)
	}
	if uint64(br.rowsLen) == br.bs.bsw.bh.rowsCount {
		// Fast path - there is no need in reading encoded values
		for _, v := range c.dictValues {
			f(v)
		}
		return
	}

	// Slow path - need to read encoded values in order filter not referenced columns.
	a := encoding.GetUint64s(len(c.dictValues))
	hits := a.A
	clear(hits)
	valuesEncoded := c.getValuesEncoded(br)
	for _, v := range valuesEncoded {
		idx := unmarshalUint8(v)
		hits[idx]++
	}
	for i, v := range c.dictValues {
		if h := hits[i]; h > 0 {
			f(v)
		}
	}
	encoding.PutUint64s(a)
}

// forEachDictValueWithHits calls f for every value in the column dictionary.
//
// hits is the number of rows with the given value v in the column.
func (c *blockResultColumn) forEachDictValueWithHits(br *blockResult, f func(v string, hits uint64)) {
	if c.valueType != valueTypeDict {
		logger.Panicf("BUG: unexpected column valueType=%d; want %d", c.valueType, valueTypeDict)
	}

	a := encoding.GetUint64s(len(c.dictValues))
	hits := a.A
	clear(hits)
	valuesEncoded := c.getValuesEncoded(br)
	for _, v := range valuesEncoded {
		idx := unmarshalUint8(v)
		hits[idx]++
	}
	for i, v := range c.dictValues {
		if h := hits[i]; h > 0 {
			f(v, h)
		}
	}
	encoding.PutUint64s(a)
}

func (c *blockResultColumn) getFloatValueAtRow(br *blockResult, rowIdx int) (float64, bool) {
	if c.isConst {
		v := c.valuesEncoded[0]
		return tryParseFloat64(v)
	}
	if c.isTime {
		return 0, false
	}

	valuesEncoded := c.getValuesEncoded(br)

	switch c.valueType {
	case valueTypeString:
		v := valuesEncoded[rowIdx]
		return tryParseFloat64(v)
	case valueTypeDict:
		dictIdx := valuesEncoded[rowIdx][0]
		v := c.dictValues[dictIdx]
		return tryParseFloat64(v)
	case valueTypeUint8:
		v := valuesEncoded[rowIdx]
		return float64(unmarshalUint8(v)), true
	case valueTypeUint16:
		v := valuesEncoded[rowIdx]
		return float64(unmarshalUint16(v)), true
	case valueTypeUint32:
		v := valuesEncoded[rowIdx]
		return float64(unmarshalUint32(v)), true
	case valueTypeUint64:
		v := valuesEncoded[rowIdx]
		return float64(unmarshalUint64(v)), true
	case valueTypeFloat64:
		v := valuesEncoded[rowIdx]
		f := unmarshalFloat64(v)
		return f, !math.IsNaN(f)
	case valueTypeIPv4:
		return 0, false
	case valueTypeTimestampISO8601:
		return 0, false
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return 0, false
	}
}

func (c *blockResultColumn) sumLenValues(br *blockResult) uint64 {
	if c.isConst {
		v := c.valuesEncoded[0]
		return uint64(len(v)) * uint64(br.rowsLen)
	}
	if c.isTime {
		return uint64(len(time.RFC3339Nano)) * uint64(br.rowsLen)
	}

	switch c.valueType {
	case valueTypeString:
		return c.sumLenStringValues(br)
	case valueTypeDict:
		n := uint64(0)
		dictValues := c.dictValues
		for _, v := range c.getValuesEncoded(br) {
			idx := v[0]
			v := dictValues[idx]
			n += uint64(len(v))
		}
		return n
	case valueTypeUint8:
		return c.sumLenStringValues(br)
	case valueTypeUint16:
		return c.sumLenStringValues(br)
	case valueTypeUint32:
		return c.sumLenStringValues(br)
	case valueTypeUint64:
		return c.sumLenStringValues(br)
	case valueTypeFloat64:
		return c.sumLenStringValues(br)
	case valueTypeIPv4:
		return c.sumLenStringValues(br)
	case valueTypeTimestampISO8601:
		return uint64(len(iso8601Timestamp)) * uint64(br.rowsLen)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return 0
	}
}

func (c *blockResultColumn) sumLenStringValues(br *blockResult) uint64 {
	n := uint64(0)
	for _, v := range c.getValues(br) {
		n += uint64(len(v))
	}
	return n
}

func (c *blockResultColumn) sumValues(br *blockResult) (float64, int) {
	if c.isConst {
		v := c.valuesEncoded[0]
		f, ok := tryParseFloat64(v)
		if !ok {
			return 0, 0
		}
		return f * float64(br.rowsLen), br.rowsLen
	}
	if c.isTime {
		return 0, 0
	}

	switch c.valueType {
	case valueTypeString:
		sum := float64(0)
		count := 0
		f := float64(0)
		ok := false
		values := c.getValuesEncoded(br)
		for i := range values {
			if i == 0 || values[i-1] != values[i] {
				f, ok = tryParseNumber(values[i])
			}
			if ok {
				sum += f
				count++
			}
		}
		return sum, count
	case valueTypeDict:
		dictValues := c.dictValues
		a := encoding.GetFloat64s(len(dictValues))
		dictValuesFloat := a.A
		for i, v := range dictValues {
			f, ok := tryParseNumber(v)
			if !ok {
				f = nan
			}
			dictValuesFloat[i] = f
		}
		sum := float64(0)
		count := 0
		for _, v := range c.getValuesEncoded(br) {
			dictIdx := v[0]
			f := dictValuesFloat[dictIdx]
			if !math.IsNaN(f) {
				sum += f
				count++
			}
		}
		encoding.PutFloat64s(a)
		return sum, count
	case valueTypeUint8:
		sum := uint64(0)
		for _, v := range c.getValuesEncoded(br) {
			sum += uint64(unmarshalUint8(v))
		}
		return float64(sum), br.rowsLen
	case valueTypeUint16:
		sum := uint64(0)
		for _, v := range c.getValuesEncoded(br) {
			sum += uint64(unmarshalUint16(v))
		}
		return float64(sum), br.rowsLen
	case valueTypeUint32:
		sum := uint64(0)
		for _, v := range c.getValuesEncoded(br) {
			sum += uint64(unmarshalUint32(v))
		}
		return float64(sum), br.rowsLen
	case valueTypeUint64:
		sum := float64(0)
		for _, v := range c.getValuesEncoded(br) {
			sum += float64(unmarshalUint64(v))
		}
		return sum, br.rowsLen
	case valueTypeFloat64:
		sum := float64(0)
		for _, v := range c.getValuesEncoded(br) {
			f := unmarshalFloat64(v)
			if !math.IsNaN(f) {
				sum += f
			}
		}
		return sum, br.rowsLen
	case valueTypeIPv4:
		return 0, 0
	case valueTypeTimestampISO8601:
		return 0, 0
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return 0, 0
	}
}

// resultColumn represents a column with result values.
//
// It doesn't own the result values.
type resultColumn struct {
	// name is column name.
	name string

	// values is the result values.
	values []string
}

func (rc *resultColumn) reset() {
	rc.name = ""
	rc.resetValues()
}

func (rc *resultColumn) resetValues() {
	clear(rc.values)
	rc.values = rc.values[:0]
}

func appendResultColumnWithName(dst []resultColumn, name string) []resultColumn {
	dst = slicesutil.SetLength(dst, len(dst)+1)
	rc := &dst[len(dst)-1]
	rc.name = name
	rc.resetValues()
	return dst
}

// addValue adds the given values v to rc.
//
// rc is valid until v is modified.
func (rc *resultColumn) addValue(v string) {
	rc.values = append(rc.values, v)
}

func truncateTimestampToMonth(timestamp int64) int64 {
	t := time.Unix(0, timestamp).UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).UnixNano()
}

func truncateTimestampToYear(timestamp int64) int64 {
	t := time.Unix(0, timestamp).UTC()
	return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano()
}

func getEmptyStrings(rowsLen int) []string {
	p := emptyStrings.Load()
	if p == nil {
		values := make([]string, rowsLen)
		emptyStrings.Store(&values)
		return values
	}
	values := *p
	return slicesutil.SetLength(values, rowsLen)
}

var emptyStrings atomic.Pointer[[]string]

func visitValuesReadonly(bs *blockSearch, ch *columnHeader, bm *bitmap, f func(value string)) {
	if bm.isZero() {
		// Fast path - nothing to visit
		return
	}
	values := bs.getValuesForColumn(ch)
	bm.forEachSetBitReadonly(func(idx int) {
		f(values[idx])
	})
}

func getCanonicalColumnName(columnName string) string {
	if columnName == "" {
		return "_msg"
	}
	return columnName
}

func tryParseNumber(s string) (float64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	f, ok := tryParseFloat64(s)
	if ok {
		return f, true
	}
	nsecs, ok := tryParseDuration(s)
	if ok {
		return float64(nsecs), true
	}
	bytes, ok := tryParseBytes(s)
	if ok {
		return float64(bytes), true
	}
	if isLikelyNumber(s) {
		f, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return f, true
		}
		n, err := strconv.ParseInt(s, 0, 64)
		if err == nil {
			return float64(n), true
		}
	}
	return 0, false
}

func isLikelyNumber(s string) bool {
	if !isNumberPrefix(s) {
		return false
	}
	if strings.Count(s, ".") > 1 {
		// This is likely IP address
		return false
	}
	if strings.IndexByte(s, ':') >= 0 || strings.Count(s, "-") > 2 {
		// This is likely a timestamp
		return false
	}
	return true
}

var nan = math.NaN()
var inf = math.Inf(1)
