package logstorage

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fastnum"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
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

	// brSrc points to the source blockResult if bs is nil and bm is non-nil.
	brSrc *blockResult

	// bm is an optional bitmap, which must be applied to bs or brSrc in order to obtain blockResult values.
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
}

func (br *blockResult) reset() {
	br.rowsLen = 0

	br.bs = nil
	br.brSrc = nil
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
}

// clone returns a clone of br, which owns its own data.
func (br *blockResult) clone() *blockResult {
	brNew := &blockResult{}

	brNew.rowsLen = br.rowsLen

	// do not clone br.cs, since it may be updated at any time.
	// do not clone br.brSrc, since it may be updated at any time.
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

	return brNew
}

// initFromFilterAllColumns initializes br from brSrc by copying rows identified by set bits at bm.
//
// The br is valid until brSrc or bm is updated.
func (br *blockResult) initFromFilterAllColumns(brSrc *blockResult, bm *bitmap) {
	br.reset()

	br.rowsLen = bm.onesCount()
	if br.rowsLen == 0 {
		return
	}

	br.brSrc = brSrc
	br.bm = bm

	cs := brSrc.getColumns()
	for _, c := range cs {
		br.appendFilteredColumn(c)
	}
}

// appendFilteredColumn adds cSrc to br.
//
// the br is valid until cSrc is updated.
func (br *blockResult) appendFilteredColumn(cSrc *blockResultColumn) {
	if br.rowsLen == 0 {
		logger.Panicf("BUG: br.rowsLen must be greater than 0")
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
		cDst.cSrc = cSrc
	}

	br.csAdd(cDst)
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

func (br *blockResult) addValues(values []string) {
	valuesBufLen := len(br.valuesBuf)
	br.valuesBuf = slicesutil.SetLength(br.valuesBuf, valuesBufLen+len(values))
	valuesBuf := br.valuesBuf[valuesBufLen:]
	_ = valuesBuf[len(values)-1]
	for i, v := range values {
		valuesBuf[i] = br.a.copyString(v)
	}
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

func (br *blockResult) initFromDataBlock(db *DataBlock) {
	br.reset()

	br.rowsLen = db.RowsCount()

	for i := range db.Columns {
		c := &db.Columns[i]
		if c.Name == "_time" {
			var ok bool
			br.timestampsBuf, ok = tryParseTimestamps(br.timestampsBuf[:0], c.Values)
			if ok {
				br.addTimeColumn()
				continue
			}
			br.timestampsBuf = br.timestampsBuf[:0]
		}
		br.addResultColumn(resultColumn{
			name:   c.Name,
			values: c.Values,
		})
	}
}

// setResultColumns sets the given rcs as br columns.
//
// The br is valid only until rcs are modified.
func (br *blockResult) setResultColumns(rcs []resultColumn, rowsLen int) {
	br.reset()

	br.rowsLen = rowsLen

	for i := range rcs {
		br.addResultColumn(rcs[i])
	}
}

func (br *blockResult) addResultColumnFloat64(rc resultColumn, minValue, maxValue float64) {
	if len(rc.values) != br.rowsLen {
		logger.Panicf("BUG: column %q must contain %d rows, but it contains %d rows", rc.name, br.rowsLen, len(rc.values))
	}
	br.csAdd(blockResultColumn{
		name:          rc.name,
		valueType:     valueTypeFloat64,
		minValue:      math.Float64bits(minValue),
		maxValue:      math.Float64bits(maxValue),
		valuesEncoded: rc.values,
	})
}

func (br *blockResult) addResultColumn(rc resultColumn) {
	if len(rc.values) != br.rowsLen {
		logger.Panicf("BUG: column %q must contain %d rows, but it contains %d rows", rc.name, br.rowsLen, len(rc.values))
	}
	if areConstValues(rc.values) {
		br.addResultColumnConst(rc)
	} else {
		br.csAdd(blockResultColumn{
			name:          rc.name,
			valueType:     valueTypeString,
			valuesEncoded: rc.values,
		})
	}
}

func (br *blockResult) addResultColumnConst(rc resultColumn) {
	br.valuesBuf = append(br.valuesBuf, rc.values[0])
	valuesEncoded := br.valuesBuf[len(br.valuesBuf)-1:]
	br.csAdd(blockResultColumn{
		name:          rc.name,
		isConst:       true,
		valuesEncoded: valuesEncoded,
	})
}

// initColumns initializes columns in br according to pf.
func (br *blockResult) initColumns(pf *prefixfilter.Filter) {
	fields, ok := pf.GetAllowStrings()
	if ok {
		// Fast path
		br.initColumnsByFields(fields)
	} else {
		// Slow path
		br.initColumnsByFilter(pf)
	}

	br.csInitFast()
}

func (br *blockResult) initColumnsByFields(fields []string) {
	for _, f := range fields {
		switch f {
		case "_time":
			br.addTimeColumn()
		case "_stream_id":
			br.addStreamIDColumn()
		case "_stream":
			if !br.addStreamColumn() {
				// Skip the current block, since the associated stream tags are missing
				br.reset()
				return
			}
		default:
			v := br.bs.getConstColumnValue(f)
			if v != "" {
				br.addConstColumn(f, v)
			} else if ch := br.bs.getColumnHeader(f); ch != nil {
				br.addColumn(ch)
			} else {
				br.addConstColumn(f, "")
			}
		}
	}
}

func (br *blockResult) initColumnsByFilter(pf *prefixfilter.Filter) {
	if pf.MatchString("_time") {
		br.addTimeColumn()
	}

	if pf.MatchString("_stream_id") {
		br.addStreamIDColumn()
	}

	if pf.MatchString("_stream") {
		if !br.addStreamColumn() {
			// Skip the current block, since the associated stream tags are missing
			br.reset()
			return
		}
	}

	if pf.MatchString("_msg") {
		// Add _msg column
		v := br.bs.getConstColumnValue("_msg")
		if v != "" {
			br.addConstColumn("_msg", v)
		} else if ch := br.bs.getColumnHeader("_msg"); ch != nil {
			br.addColumn(ch)
		} else {
			br.addConstColumn("_msg", "")
		}
	}

	// Add other const columns
	csh := br.bs.getColumnsHeader()
	for _, cc := range csh.constColumns {
		if cc.Name == "" {
			// We already added _msg column above
			continue
		}
		if pf.MatchString(cc.Name) {
			br.addConstColumn(cc.Name, cc.Value)
		}
	}

	// Add other non-const columns
	chs := csh.columnHeaders
	for i := range chs {
		ch := &chs[i]
		if ch.name == "" {
			// We already added _msg column above
			continue
		}
		if pf.MatchString(ch.name) {
			br.addColumn(ch)
		}
	}
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

// intersectsTimeRange returns true if br timestamps intersect [minTimestamp .. maxTimestamp] time range.
func (br *blockResult) intersectsTimeRange(minTimestamp, maxTimestamp int64) bool {
	return minTimestamp <= br.getMaxTimestamp(minTimestamp) && maxTimestamp >= br.getMinTimestamp(maxTimestamp)
}

func (br *blockResult) getMinTimestamp(minTimestamp int64) int64 {
	if br.bs != nil {
		th := &br.bs.bsw.bh.timestampsHeader
		if br.isFull() {
			return min(minTimestamp, th.minTimestamp)
		}
		if minTimestamp <= th.minTimestamp {
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
		th := &br.bs.bsw.bh.timestampsHeader
		if br.isFull() {
			return max(maxTimestamp, th.maxTimestamp)
		}
		if maxTimestamp >= th.maxTimestamp {
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
	if br.brSrc != nil {
		srcTimestamps := br.brSrc.getTimestamps()
		br.initTimestampsInternal(srcTimestamps)
		return
	}
	if br.bs != nil {
		srcTimestamps := br.bs.getTimestamps()
		br.initTimestampsInternal(srcTimestamps)
		return
	}

	// Try decoding timestamps from _time field
	c := br.getColumnByName("_time")
	timestampValues := c.getValues(br)
	var ok bool
	br.timestampsBuf, ok = tryParseTimestamps(br.timestampsBuf[:0], timestampValues)
	if !ok {
		br.timestampsBuf = fastnum.AppendInt64Zeros(br.timestampsBuf[:0], br.rowsLen)
	}
}

func (br *blockResult) initTimestampsInternal(srcTimestamps []int64) {
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

func tryParseTimestamps(dst []int64, src []string) ([]int64, bool) {
	dstLen := len(dst)
	dst = slicesutil.SetLength(dst, dstLen+len(src))
	timestamps := dst[dstLen:]
	for i, v := range src {
		ts, ok := TryParseTimestampRFC3339Nano(v)
		if !ok {
			return dst, false
		}
		timestamps[i] = ts
	}
	return dst, true
}

func (br *blockResult) checkTimestampsLen() {
	if len(br.timestampsBuf) != br.rowsLen {
		logger.Panicf("BUG: unexpected number of timestamps; got %d; want %d", len(br.timestampsBuf), br.rowsLen)
	}
}

func (br *blockResult) readValuesEncodedFromColumnHeader(ch *columnHeader) []string {
	valuesBufLen := len(br.valuesBuf)

	switch ch.valueType {
	case valueTypeString:
		visitValuesReadonly(br.bs, ch, br.bm, br.addValues)
	case valueTypeDict:
		visitValuesReadonly(br.bs, ch, br.bm, func(values []string) {
			checkValuesSize(br.bs, ch, values, 1, "dict")
			for _, v := range values {
				dictIdx := v[0]
				if int(dictIdx) >= len(ch.valuesDict.values) {
					logger.Panicf("FATAL: %s: too big dict index for column %q: %d; should be smaller than %d", br.bs.partPath(), ch.name, dictIdx, len(ch.valuesDict.values))
				}
			}
			br.addValues(values)
		})
	case valueTypeUint8:
		visitValuesReadonly(br.bs, ch, br.bm, func(values []string) {
			checkValuesSize(br.bs, ch, values, 1, "uint8")
			br.addValues(values)
		})
	case valueTypeUint16:
		visitValuesReadonly(br.bs, ch, br.bm, func(values []string) {
			checkValuesSize(br.bs, ch, values, 2, "uint16")
			br.addValues(values)
		})
	case valueTypeUint32:
		visitValuesReadonly(br.bs, ch, br.bm, func(values []string) {
			checkValuesSize(br.bs, ch, values, 4, "uint32")
			br.addValues(values)
		})
	case valueTypeUint64:
		visitValuesReadonly(br.bs, ch, br.bm, func(values []string) {
			checkValuesSize(br.bs, ch, values, 8, "uint64")
			br.addValues(values)
		})
	case valueTypeInt64:
		visitValuesReadonly(br.bs, ch, br.bm, func(values []string) {
			checkValuesSize(br.bs, ch, values, 8, "int64")
			br.addValues(values)
		})
	case valueTypeFloat64:
		visitValuesReadonly(br.bs, ch, br.bm, func(values []string) {
			checkValuesSize(br.bs, ch, values, 8, "float64")
			br.addValues(values)
		})
	case valueTypeIPv4:
		visitValuesReadonly(br.bs, ch, br.bm, func(values []string) {
			checkValuesSize(br.bs, ch, values, 4, "ipv4")
			br.addValues(values)
		})
	case valueTypeTimestampISO8601:
		visitValuesReadonly(br.bs, ch, br.bm, func(values []string) {
			checkValuesSize(br.bs, ch, values, 8, "iso8601")
			br.addValues(values)
		})
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d for column %q", br.bs.partPath(), ch.valueType, ch.name)
	}

	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) readValuesEncodedFromResultColumn(c *blockResultColumn) []string {
	valuesEncodedSrc := c.getValuesEncoded(br.brSrc)

	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	br.bm.forEachSetBitReadonly(func(idx int) {
		valuesBuf = append(valuesBuf, valuesEncodedSrc[idx])
	})
	br.valuesBuf = valuesBuf

	return valuesBuf[valuesBufLen:]
}

func checkValuesSize(bs *blockSearch, ch *columnHeader, values []string, sizeExpected int, typeStr string) {
	for _, v := range values {
		if len(v) != sizeExpected {
			logger.Panicf("FATAL: %s: unexpected size for %s column %q; got %d bytes; want %d bytes", typeStr, bs.partPath(), ch.name, len(v), sizeExpected)
		}
	}
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
		chSrc:      ch,
	})

	br.csInitialized = false
}

func (br *blockResult) addTimeColumn() {
	br.csAdd(blockResultColumn{
		name:   "_time",
		isTime: true,
	})
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

	br.csAdd(blockResultColumn{
		name:          nameCopy,
		isConst:       true,
		valuesEncoded: valuesEncoded,
	})
}

func (br *blockResult) newValuesForColumn(c *blockResultColumn) []string {
	if c.isConst {
		v := c.valuesEncoded[0]
		return br.getConstValues(v)
	}
	if c.isTime {
		return br.getTimestampValues()
	}

	switch c.valueType {
	case valueTypeString:
		return c.getValuesEncoded(br)
	case valueTypeDict:
		return br.getDictValues(c)
	case valueTypeUint8:
		return br.getUint8Values(c)
	case valueTypeUint16:
		return br.getUint16Values(c)
	case valueTypeUint32:
		return br.getUint32Values(c)
	case valueTypeUint64:
		return br.getUint64Values(c)
	case valueTypeInt64:
		return br.getInt64Values(c)
	case valueTypeFloat64:
		return br.getFloat64Values(c)
	case valueTypeIPv4:
		return br.getIPv4Values(c)
	case valueTypeTimestampISO8601:
		return br.getTimestampISO8601Values(c)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return nil
	}
}

func (br *blockResult) newValuesBucketedForColumn(c *blockResultColumn, bf *byStatsField) []string {
	if c.isConst {
		v := c.valuesEncoded[0]
		s := br.getBucketedValue(v, bf)
		return br.getConstValues(s)
	}
	if c.isTime {
		return br.getBucketedTimestampValues(bf)
	}

	switch c.valueType {
	case valueTypeString:
		valuesEncoded := c.getValuesEncoded(br)
		return br.getBucketedStrings(valuesEncoded, bf)
	case valueTypeDict:
		return br.getBucketedDictValues(c, bf)
	case valueTypeUint8:
		return br.getBucketedUint8Values(c, bf)
	case valueTypeUint16:
		return br.getBucketedUint16Values(c, bf)
	case valueTypeUint32:
		return br.getBucketedUint32Values(c, bf)
	case valueTypeUint64:
		return br.getBucketedUint64Values(c, bf)
	case valueTypeInt64:
		return br.getBucketedInt64Values(c, bf)
	case valueTypeFloat64:
		return br.getBucketedFloat64Values(c, bf)
	case valueTypeIPv4:
		return br.getBucketedIPv4Values(c, bf)
	case valueTypeTimestampISO8601:
		return br.getBucketedTimestampISO8601Values(c, bf)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return nil
	}
}

func (br *blockResult) getConstValues(s string) []string {
	if s == "" {
		// Fast path - return a slice of empty strings without constructing the slice.
		return getEmptyStrings(br.rowsLen)
	}

	// Slower path - construct slice of identical values with the length equal to br.rowsLen
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+br.rowsLen)
	values := valuesBuf[valuesBufLen:]

	for i := range values {
		values[i] = s
	}

	br.valuesBuf = valuesBuf

	return values
}

func (br *blockResult) getBucketedTimestampValues(bf *byStatsField) []string {
	bucketSizeInt := int64(bf.bucketSize)
	if bucketSizeInt <= 0 {
		bucketSizeInt = 1
	}
	bucketOffsetInt := int64(bf.bucketOffset)

	if br.bs != nil {
		th := &br.bs.bsw.bh.timestampsHeader
		tsMin := truncateTimestamp(th.minTimestamp, bucketSizeInt, bucketOffsetInt, bf.bucketSizeStr)
		tsMax := truncateTimestamp(th.maxTimestamp, bucketSizeInt, bucketOffsetInt, bf.bucketSizeStr)
		if tsMin == tsMax {
			// Fast path - all the timestamps in the block belong to the same bucket.
			buf := br.a.b
			bufLen := len(buf)
			buf = marshalTimestampRFC3339NanoString(buf, tsMin)
			s := bytesutil.ToUnsafeString(buf[bufLen:])
			br.a.b = buf

			return br.getConstValues(s)
		}
	}

	// Slow path - individually bucketize every timestamp in the block.
	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+br.rowsLen)
	values := valuesBuf[valuesBufLen:]

	var s string
	timestampTruncatedPrev := int64(0)
	timestamps := br.getTimestamps()
	for i := range timestamps {
		if i > 0 && timestamps[i-1] == timestamps[i] {
			values[i] = s
			continue
		}

		timestampTruncated := truncateTimestamp(timestamps[i], bucketSizeInt, bucketOffsetInt, bf.bucketSizeStr)

		if i == 0 || timestampTruncatedPrev != timestampTruncated {
			bufLen := len(buf)
			buf = marshalTimestampRFC3339NanoString(buf, timestampTruncated)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			timestampTruncatedPrev = timestampTruncated
		}
		values[i] = s
	}

	br.a.b = buf
	br.valuesBuf = valuesBuf

	return values
}

func truncateTimestamp(ts, bucketSizeInt, bucketOffsetInt int64, bucketSizeStr string) int64 {
	if bucketSizeStr == "week" {
		// Adjust the week to be started from Monday.
		bucketOffsetInt += 4 * nsecsPerDay
	}
	if bucketOffsetInt == 0 && bucketSizeStr != "month" && bucketSizeStr != "year" {
		// Fast path for timestamps without offsets
		r := ts % bucketSizeInt
		if r < 0 {
			r += bucketSizeInt
		}
		return ts - r
	}

	ts -= bucketOffsetInt
	if bucketSizeStr == "month" {
		ts = truncateTimestampToMonth(ts)
	} else if bucketSizeStr == "year" {
		ts = truncateTimestampToYear(ts)
	} else {
		r := ts % bucketSizeInt
		if r < 0 {
			r += bucketSizeInt
		}
		ts -= r
	}
	ts += bucketOffsetInt

	return ts
}

func (br *blockResult) getTimestampValues() []string {
	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+br.rowsLen)
	values := valuesBuf[valuesBufLen:]

	var s string
	timestamps := br.getTimestamps()
	for i := range timestamps {
		if i == 0 || timestamps[i-1] != timestamps[i] {
			bufLen := len(buf)
			buf = marshalTimestampRFC3339NanoString(buf, timestamps[i])
			s = bytesutil.ToUnsafeString(buf[bufLen:])
		}
		values[i] = s
	}

	br.a.b = buf
	br.valuesBuf = valuesBuf

	return values
}

func (br *blockResult) getBucketedStrings(valuesOrig []string, bf *byStatsField) []string {
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesOrig))
	values := valuesBuf[valuesBufLen:]

	var s string
	for i := range values {
		if i == 0 || valuesOrig[i-1] != valuesOrig[i] {
			s = br.getBucketedValue(valuesOrig[i], bf)
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf

	return values
}

func (br *blockResult) getBucketedDictValues(c *blockResultColumn, bf *byStatsField) []string {
	dictValuesBucketed := br.getBucketedStrings(c.dictValues, bf)
	if areConstValues(dictValuesBucketed) {
		// fast path - all the bucketed values in the block are the same
		return br.getConstValues(dictValuesBucketed[0])
	}

	valuesEncoded := c.getValuesEncoded(br)

	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	for i, v := range valuesEncoded {
		dictIdx := v[0]
		values[i] = dictValuesBucketed[dictIdx]
	}

	br.valuesBuf = valuesBuf

	return values
}

func (br *blockResult) getDictValues(c *blockResultColumn) []string {
	valuesEncoded := c.getValuesEncoded(br)

	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	for i, v := range valuesEncoded {
		dictIdx := v[0]
		values[i] = c.dictValues[dictIdx]
	}

	br.valuesBuf = valuesBuf

	return values
}

func (br *blockResult) getBucketedUint8Values(c *blockResultColumn, bf *byStatsField) []string {
	bucketSizeInt := uint64(bf.bucketSize)
	if bucketSizeInt <= 0 {
		bucketSizeInt = 1
	}
	bucketOffsetInt := uint64(int64(bf.bucketOffset))
	minValue := uint64(int64(c.minValue))
	maxValue := uint64(int64(c.maxValue))

	nMin := truncateUint64(minValue, bucketSizeInt, bucketOffsetInt)
	nMax := truncateUint64(maxValue, bucketSizeInt, bucketOffsetInt)
	if nMin == nMax {
		// fast path - all the truncated values in the block are the same
		buf := br.a.b
		bufLen := len(buf)
		buf = marshalUint64String(buf, nMin)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		br.a.b = buf

		return br.getConstValues(s)
	}

	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	nPrev := uint64(0)
	for i, v := range valuesEncoded {
		if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
			values[i] = s
			continue
		}

		n := uint64(unmarshalUint8(v))
		n = truncateUint64(n, bucketSizeInt, bucketOffsetInt)

		if i == 0 || nPrev != n {
			bufLen := len(buf)
			buf = marshalUint64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			nPrev = n
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func (br *blockResult) getUint8Values(c *blockResultColumn) []string {
	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	for i, v := range valuesEncoded {
		if i == 0 || valuesEncoded[i-1] != valuesEncoded[i] {
			n := unmarshalUint8(v)
			bufLen := len(buf)
			buf = marshalUint8String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func (br *blockResult) getBucketedUint16Values(c *blockResultColumn, bf *byStatsField) []string {
	bucketSizeInt := uint64(bf.bucketSize)
	if bucketSizeInt <= 0 {
		bucketSizeInt = 1
	}
	bucketOffsetInt := uint64(int64(bf.bucketOffset))
	minValue := uint64(int64(c.minValue))
	maxValue := uint64(int64(c.maxValue))

	nMin := truncateUint64(minValue, bucketSizeInt, bucketOffsetInt)
	nMax := truncateUint64(maxValue, bucketSizeInt, bucketOffsetInt)
	if nMin == nMax {
		// fast path - all the truncated values in the block are the same
		buf := br.a.b
		bufLen := len(buf)
		buf = marshalUint64String(buf, nMin)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		br.a.b = buf

		return br.getConstValues(s)
	}

	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	nPrev := uint64(0)
	for i, v := range valuesEncoded {
		if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
			values[i] = s
			continue
		}

		n := uint64(unmarshalUint16(v))
		n = truncateUint64(n, bucketSizeInt, bucketOffsetInt)

		if i == 0 || nPrev != n {
			bufLen := len(buf)
			buf = marshalUint64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			nPrev = n
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func (br *blockResult) getUint16Values(c *blockResultColumn) []string {
	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	for i, v := range valuesEncoded {
		if i == 0 || valuesEncoded[i-1] != valuesEncoded[i] {
			n := unmarshalUint16(v)
			bufLen := len(buf)
			buf = marshalUint16String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func (br *blockResult) getBucketedUint32Values(c *blockResultColumn, bf *byStatsField) []string {
	bucketSizeInt := uint64(bf.bucketSize)
	if bucketSizeInt <= 0 {
		bucketSizeInt = 1
	}
	bucketOffsetInt := uint64(int64(bf.bucketOffset))
	minValue := uint64(int64(c.minValue))
	maxValue := uint64(int64(c.maxValue))

	nMin := truncateUint64(minValue, bucketSizeInt, bucketOffsetInt)
	nMax := truncateUint64(maxValue, bucketSizeInt, bucketOffsetInt)
	if nMin == nMax {
		// fast path - all the truncated values in the block are the same
		buf := br.a.b
		bufLen := len(buf)
		buf = marshalUint64String(buf, nMin)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		br.a.b = buf

		return br.getConstValues(s)
	}

	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	nPrev := uint64(0)
	for i, v := range valuesEncoded {
		if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
			values[i] = s
			continue
		}

		n := uint64(unmarshalUint32(v))
		n = truncateUint64(n, bucketSizeInt, bucketOffsetInt)

		if i == 0 || nPrev != n {
			bufLen := len(buf)
			buf = marshalUint64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			nPrev = n
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func (br *blockResult) getUint32Values(c *blockResultColumn) []string {
	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	for i, v := range valuesEncoded {
		if i == 0 || valuesEncoded[i-1] != valuesEncoded[i] {
			n := unmarshalUint32(v)
			bufLen := len(buf)
			buf = marshalUint32String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func (br *blockResult) getBucketedUint64Values(c *blockResultColumn, bf *byStatsField) []string {
	bucketSizeInt := uint64(bf.bucketSize)
	if bucketSizeInt <= 0 {
		bucketSizeInt = 1
	}
	bucketOffsetInt := uint64(int64(bf.bucketOffset))
	minValue := uint64(int64(c.minValue))
	maxValue := uint64(int64(c.maxValue))

	nMin := truncateUint64(minValue, bucketSizeInt, bucketOffsetInt)
	nMax := truncateUint64(maxValue, bucketSizeInt, bucketOffsetInt)
	if nMin == nMax {
		// fast path - all the truncated values in the block are the same
		buf := br.a.b
		bufLen := len(buf)
		buf = marshalUint64String(buf, nMin)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		br.a.b = buf

		return br.getConstValues(s)
	}

	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	nPrev := uint64(0)
	for i, v := range valuesEncoded {
		if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
			values[i] = s
			continue
		}

		n := unmarshalUint64(v)
		n = truncateUint64(n, bucketSizeInt, bucketOffsetInt)

		if i == 0 || nPrev != n {
			bufLen := len(buf)
			buf = marshalUint64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			nPrev = n
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func truncateUint64(n, bucketSizeInt, bucketOffsetInt uint64) uint64 {
	if bucketOffsetInt == 0 {
		return n - n%bucketSizeInt
	}
	if bucketOffsetInt > n {
		return 0
	}

	n -= bucketOffsetInt
	n -= n % bucketSizeInt
	n += bucketOffsetInt
	return n
}

func (br *blockResult) getUint64Values(c *blockResultColumn) []string {
	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	for i, v := range valuesEncoded {
		if i == 0 || valuesEncoded[i-1] != valuesEncoded[i] {
			n := unmarshalUint64(v)
			bufLen := len(buf)
			buf = marshalUint64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func (br *blockResult) getBucketedInt64Values(c *blockResultColumn, bf *byStatsField) []string {
	bucketSizeInt := int64(bf.bucketSize)
	if bucketSizeInt == 0 {
		bucketSizeInt = 1
	}
	bucketOffsetInt := int64(bf.bucketOffset)
	minValue := int64(c.minValue)
	maxValue := int64(c.maxValue)

	nMin := truncateInt64(minValue, bucketSizeInt, bucketOffsetInt)
	nMax := truncateInt64(maxValue, bucketSizeInt, bucketOffsetInt)
	if nMin == nMax {
		// fast path - all the bucketed values in the block are the same
		buf := br.a.b
		bufLen := len(buf)
		buf = marshalInt64String(buf, nMin)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		br.a.b = buf

		return br.getConstValues(s)
	}

	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	nPrev := int64(0)
	for i, v := range valuesEncoded {
		if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
			values[i] = s
			continue
		}

		n := unmarshalInt64(v)
		n = truncateInt64(n, bucketSizeInt, bucketOffsetInt)

		if i == 0 || nPrev != n {
			bufLen := len(buf)
			buf = marshalInt64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			nPrev = n
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func truncateInt64(n, bucketSizeInt, bucketOffsetInt int64) int64 {
	if bucketOffsetInt == 0 {
		r := n % bucketSizeInt
		if r < 0 {
			r += bucketSizeInt
		}
		return n - r
	}

	n -= bucketOffsetInt
	r := n % bucketSizeInt
	if r < 0 {
		r += bucketSizeInt
	}
	n -= r
	n += bucketOffsetInt

	return n
}

func (br *blockResult) getInt64Values(c *blockResultColumn) []string {
	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	for i, v := range valuesEncoded {
		if i == 0 || valuesEncoded[i-1] != valuesEncoded[i] {
			n := unmarshalInt64(v)
			bufLen := len(buf)
			buf = marshalInt64String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func (br *blockResult) getBucketedFloat64Values(c *blockResultColumn, bf *byStatsField) []string {
	bucketSize := bf.bucketSize
	if bucketSize <= 0 {
		bucketSize = 1
	}

	_, e := decimal.FromFloat(bucketSize)
	p10 := math.Pow10(int(-e))
	bucketSizeP10 := int64(bucketSize * p10)
	minValue := math.Float64frombits(c.minValue)
	maxValue := math.Float64frombits(c.maxValue)

	fMin := truncateFloat64(minValue, p10, bucketSizeP10, bf.bucketOffset)
	fMax := truncateFloat64(maxValue, p10, bucketSizeP10, bf.bucketOffset)
	if fMin == fMax {
		// Fast path - all the truncated values in the block are the same.
		buf := br.a.b
		bufLen := len(buf)
		buf = marshalFloat64String(buf, fMin)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		br.a.b = buf

		return br.getConstValues(s)
	}

	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	fPrev := float64(0)
	for i, v := range valuesEncoded {
		if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
			values[i] = s
			continue
		}

		f := unmarshalFloat64(v)
		f = truncateFloat64(f, p10, bucketSizeP10, bf.bucketOffset)

		if i == 0 || fPrev != f {
			bufLen := len(buf)
			buf = marshalFloat64String(buf, f)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			fPrev = f
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func truncateFloat64(f float64, p10 float64, bucketSizeP10 int64, bucketOffset float64) float64 {
	if bucketOffset == 0 {
		fP10 := int64(math.Floor(f * p10))
		r := fP10 % bucketSizeP10
		fP10 -= r
		return float64(fP10) / p10
	}

	f -= bucketOffset

	fP10 := int64(math.Floor(f * p10))
	r := fP10 % bucketSizeP10
	fP10 -= r
	f = float64(fP10) / p10

	f += bucketOffset

	return f
}

func (br *blockResult) getFloat64Values(c *blockResultColumn) []string {
	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	for i, v := range valuesEncoded {
		if i == 0 || valuesEncoded[i-1] != valuesEncoded[i] {
			f := unmarshalFloat64(v)
			bufLen := len(buf)
			buf = marshalFloat64String(buf, f)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func (br *blockResult) getBucketedIPv4Values(c *blockResultColumn, bf *byStatsField) []string {
	bucketSizeInt := uint32(bf.bucketSize)
	if bucketSizeInt <= 0 {
		bucketSizeInt = 1
	}
	bucketOffsetInt := uint32(int32(bf.bucketOffset))
	minValue := uint32(int32(c.minValue))
	maxValue := uint32(int32(c.maxValue))

	ipMin := truncateUint32(minValue, bucketSizeInt, bucketOffsetInt)
	ipMax := truncateUint32(maxValue, bucketSizeInt, bucketOffsetInt)
	if ipMin == ipMax {
		// Fast path - all the ip values in the block belong to the same bucket.
		buf := br.a.b
		bufLen := len(buf)
		buf = marshalIPv4String(buf, ipMin)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		br.a.b = buf

		return br.getConstValues(s)
	}

	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	nPrev := uint32(0)
	for i, v := range valuesEncoded {
		if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
			values[i] = s
			continue
		}

		n := unmarshalIPv4(v)
		n = truncateUint32(n, bucketSizeInt, bucketOffsetInt)

		if i == 0 || nPrev != n {
			bufLen := len(buf)
			buf = marshalIPv4String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			nPrev = n
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func truncateUint32(n, bucketSizeInt, bucketOffsetInt uint32) uint32 {
	if bucketOffsetInt == 0 {
		return n - n%bucketSizeInt
	}
	if bucketOffsetInt > n {
		return 0
	}

	n -= bucketOffsetInt
	n -= n % bucketSizeInt
	n += bucketOffsetInt

	return n
}

func (br *blockResult) getIPv4Values(c *blockResultColumn) []string {
	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	for i, v := range valuesEncoded {
		if i == 0 || valuesEncoded[i-1] != valuesEncoded[i] {
			ip := unmarshalIPv4(v)
			bufLen := len(buf)
			buf = marshalIPv4String(buf, ip)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func (br *blockResult) getBucketedTimestampISO8601Values(c *blockResultColumn, bf *byStatsField) []string {
	bucketSizeInt := int64(bf.bucketSize)
	if bucketSizeInt <= 0 {
		bucketSizeInt = 1
	}
	bucketOffsetInt := int64(bf.bucketOffset)
	minValue := int64(c.minValue)
	maxValue := int64(c.maxValue)

	tsMin := truncateTimestamp(minValue, bucketSizeInt, bucketOffsetInt, bf.bucketSizeStr)
	tsMax := truncateTimestamp(maxValue, bucketSizeInt, bucketOffsetInt, bf.bucketSizeStr)
	if tsMin == tsMax {
		// Fast path - all the truncated values in the block have the same value
		buf := br.a.b
		bufLen := len(buf)
		buf = marshalTimestampISO8601String(buf, tsMin)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		br.a.b = buf

		return br.getConstValues(s)
	}

	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	timestampTruncatedPrev := int64(0)
	bb := bbPool.Get()
	for i, v := range valuesEncoded {
		if i > 0 && valuesEncoded[i-1] == valuesEncoded[i] {
			values[i] = s
			continue
		}

		timestamp := unmarshalTimestampISO8601(v)
		timestampTruncated := truncateTimestamp(timestamp, bucketSizeInt, bucketOffsetInt, bf.bucketSizeStr)

		if i == 0 || timestampTruncatedPrev != timestampTruncated {
			bufLen := len(buf)
			buf = marshalTimestampISO8601String(buf, timestampTruncated)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			timestampTruncatedPrev = timestampTruncated
		}
		values[i] = s
	}
	bbPool.Put(bb)

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

func (br *blockResult) getTimestampISO8601Values(c *blockResultColumn) []string {
	valuesEncoded := c.getValuesEncoded(br)

	buf := br.a.b
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+len(valuesEncoded))
	values := valuesBuf[valuesBufLen:]

	var s string
	for i, v := range valuesEncoded {
		if i == 0 || valuesEncoded[i-1] != valuesEncoded[i] {
			n := unmarshalTimestampISO8601(v)
			bufLen := len(buf)
			buf = marshalTimestampISO8601String(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
		}
		values[i] = s
	}

	br.valuesBuf = valuesBuf
	br.a.b = buf

	return values
}

// getBucketedValue returns bucketed s according to the given bf
func (br *blockResult) getBucketedValue(s string, bf *byStatsField) string {
	if len(s) == 0 {
		return ""
	}

	c := s[0]
	if (c < '0' || c > '9') && c != '-' {
		// Fast path - the value cannot be bucketed, since it starts with unexpected chars.
		return s
	}

	if n, ok := tryParseInt64(s); ok {
		bucketSizeInt := int64(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffsetInt := int64(bf.bucketOffset)

		nTruncated := truncateInt64(n, bucketSizeInt, bucketOffsetInt)

		buf := br.a.b
		bufLen := len(buf)
		buf = marshalInt64String(buf, nTruncated)
		br.a.b = buf
		return bytesutil.ToUnsafeString(buf[bufLen:])
	}

	if f, ok := tryParseFloat64(s); ok {
		bucketSize := bf.bucketSize
		if bucketSize <= 0 {
			bucketSize = 1
		}

		_, e := decimal.FromFloat(bucketSize)
		p10 := math.Pow10(int(-e))
		bucketSizeP10 := int64(bucketSize * p10)

		f = truncateFloat64(f, p10, bucketSizeP10, bf.bucketOffset)

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

		timestampTruncated := truncateTimestamp(timestamp, bucketSizeInt, bucketOffset, bf.bucketSizeStr)

		buf := br.a.b
		bufLen := len(buf)
		buf = marshalTimestampRFC3339NanoString(buf, timestampTruncated)
		br.a.b = buf
		return bytesutil.ToUnsafeString(buf[bufLen:])
	}

	if n, ok := tryParseIPv4(s); ok {
		bucketSizeInt := uint32(bf.bucketSize)
		if bucketSizeInt <= 0 {
			bucketSizeInt = 1
		}
		bucketOffset := uint32(int32(bf.bucketOffset))

		n = truncateUint32(n, bucketSizeInt, bucketOffset)

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

		nsecs = truncateInt64(nsecs, bucketSizeInt, bucketOffset)

		buf := br.a.b
		bufLen := len(buf)
		buf = marshalDurationString(buf, nsecs)
		br.a.b = buf
		return bytesutil.ToUnsafeString(buf[bufLen:])
	}

	// Couldn't parse s, so return it as is.
	return s
}

// copyColumnsByFilters copies columns from srcColumnFilters to dstColumnFilters.
//
// srcColumnFilters and dstColumnFilters may contain column names and column name prefixes ending with '*'.
func (br *blockResult) copyColumnsByFilters(srcColumnFilters, dstColumnFilters []string) {
	for i, srcFilter := range srcColumnFilters {
		dstFilter := dstColumnFilters[i]
		br.copyColumnsByFilter(srcFilter, dstFilter)
	}
}

func (br *blockResult) copyColumnsByFilter(srcFilter, dstFilter string) {
	found := false
	cs := br.getColumns()

	for _, c := range cs {
		if !prefixfilter.MatchFilter(srcFilter, c.name) {
			continue
		}

		aLen := len(br.a.b)
		br.a.b = prefixfilter.AppendReplace(br.a.b, srcFilter, dstFilter, c.name)
		fieldName := bytesutil.ToUnsafeString(br.a.b[aLen:])

		cCopy := *c
		cCopy.name = fieldName
		br.csAdd(cCopy)
		found = true
	}

	if !found && !prefixfilter.IsWildcardFilter(srcFilter) {
		br.addConstColumn(dstFilter, "")
	}
}

// renameColumnsByFilters renames columns from srcColumnFilters to dstColumnFilters.
//
// srcColumnFilters and dstColumnFilters may contain column names and column name prefixes ending with '*'.
func (br *blockResult) renameColumnsByFilters(srcColumnFilters, dstColumnFilters []string) {
	for i, srcFilter := range srcColumnFilters {
		dstFilter := dstColumnFilters[i]
		br.renameColumnsByFilter(srcFilter, dstFilter)
	}
}

func (br *blockResult) renameColumnsByFilter(srcFilter, dstFilter string) {
	found := false
	cs := br.getColumns()

	br.csInitialized = false
	csBuf := br.csBuf
	csBufLen := len(csBuf)

	for _, c := range cs {
		if !prefixfilter.MatchFilter(srcFilter, c.name) {
			csBuf = append(csBuf, *c)
		}
	}

	for _, c := range cs {
		if !prefixfilter.MatchFilter(srcFilter, c.name) {
			continue
		}

		aLen := len(br.a.b)
		br.a.b = prefixfilter.AppendReplace(br.a.b, srcFilter, dstFilter, c.name)

		c.name = bytesutil.ToUnsafeString(br.a.b[aLen:])
		csBuf = append(csBuf, *c)
		found = true
	}

	br.csBuf = csBuf
	br.csBuf = append(br.csBuf[:0], br.csBuf[csBufLen:]...)

	if !found && !prefixfilter.IsWildcardFilter(srcFilter) {
		br.addConstColumn(dstFilter, "")
	}
}

// deleteColumnsByFilters deletes columns with the given columnFilters.
//
// columnFilters may contain column names and column name prefixes ending with '*'.
func (br *blockResult) deleteColumnsByFilters(columnFilters []string) {
	if len(columnFilters) == 0 {
		return
	}

	cs := br.getColumns()

	br.csInitialized = false
	csBuf := br.csBuf
	csBufLen := len(csBuf)

	for _, c := range cs {
		if !prefixfilter.MatchFilters(columnFilters, c.name) {
			csBuf = append(csBuf, *c)
		}
	}

	br.csBuf = csBuf
	br.csBuf = append(br.csBuf[:0], br.csBuf[csBufLen:]...)
}

// setColumnFilters sets the resulting columns according to the given columnFilters.
func (br *blockResult) setColumnFilters(columnFilters []string) {
	if br.areSameColumns(columnFilters) {
		// Fast path - nothing to change.
		return
	}

	// Slow path - construct the requested columns
	cs := br.getColumns()

	br.csInitialized = false
	csBuf := br.csBuf
	csBufLen := len(csBuf)

	for _, c := range cs {
		if prefixfilter.MatchFilters(columnFilters, c.name) {
			csBuf = append(csBuf, *c)
		}
	}

	br.csBuf = csBuf
	br.csBuf = append(br.csBuf[:0], br.csBuf[csBufLen:]...)

	for _, columnFilter := range columnFilters {
		if prefixfilter.IsWildcardFilter(columnFilter) {
			continue
		}
		if idx := getBlockResultColumnIdxByName(cs, columnFilter); idx < 0 {
			br.addConstColumn(columnFilter, "")
		}
	}
}

func (br *blockResult) areSameColumns(columnFilters []string) bool {
	cs := br.getColumns()
	for _, c := range cs {
		if !prefixfilter.MatchFilters(columnFilters, c.name) {
			return false
		}
	}

	for _, columnFilter := range columnFilters {
		if prefixfilter.IsWildcardFilter(columnFilter) {
			continue
		}
		if idx := getBlockResultColumnIdxByName(cs, columnFilter); idx < 0 {
			return false
		}
	}

	return true
}

func getMatchingColumns(br *blockResult, filters []string) *matchingColumns {
	v := matchingColumnsPool.Get()
	if v == nil {
		v = &matchingColumns{}
	}
	mc := v.(*matchingColumns)

	if isSingleField(filters) {
		// Fast path - a single column is requested
		field := filters[0]
		c := br.getColumnByName(field)
		mc.cs = append(mc.cs[:0], c)
		return mc
	}

	// Slower path - multiple columns are requested
	mc.cs = br.getMatchingColumnsSlow(mc.cs[:0], filters)
	return mc
}

func putMatchingColumns(mc *matchingColumns) {
	mc.reset()
	matchingColumnsPool.Put(mc)
}

type matchingColumns struct {
	cs []*blockResultColumn
}

func (mc *matchingColumns) reset() {
	clear(mc.cs)
	mc.cs = mc.cs[:0]
}

func (mc *matchingColumns) sort() {
	if len(mc.cs) > 1 && !sort.IsSorted(mc) {
		sort.Sort(mc)
	}
}

func (mc *matchingColumns) Len() int {
	return len(mc.cs)
}
func (mc *matchingColumns) Less(i, j int) bool {
	cs := mc.cs
	return cs[i].name < cs[j].name
}
func (mc *matchingColumns) Swap(i, j int) {
	cs := mc.cs
	cs[i], cs[j] = cs[j], cs[i]
}

var matchingColumnsPool sync.Pool

func (br *blockResult) getMatchingColumnsSlow(dst []*blockResultColumn, filters []string) []*blockResultColumn {
	cs := br.getColumns()

	// Add columns matching the given filters
	for _, c := range cs {
		if prefixfilter.MatchFilters(filters, c.name) {
			dst = append(dst, c)
		}
	}

	// Add empty columns for non-wildcard filters, which do not match non-empty columns.
	for _, f := range filters {
		if prefixfilter.IsWildcardFilter(f) {
			continue
		}

		needEmptyField := true
		for _, c := range cs {
			if f == c.name {
				needEmptyField = false
				break
			}
		}

		if needEmptyField {
			c := br.getEmptyColumnByName(f)
			dst = append(dst, c)
		}
	}

	return dst
}

func isSingleField(filters []string) bool {
	return len(filters) == 1 && !prefixfilter.IsWildcardFilter(filters[0])
}

func (br *blockResult) getColumnByName(columnName string) *blockResultColumn {
	cs := br.getColumns()

	columnName = getCanonicalColumnName(columnName)
	idx := getBlockResultColumnIdxByName(cs, columnName)
	if idx >= 0 {
		return cs[idx]
	}

	return br.getEmptyColumnByName(columnName)
}

func (br *blockResult) getEmptyColumnByName(columnName string) *blockResultColumn {
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
	br.cs = br.cs[:0]
	for i := range csBuf {
		br.csAddOrReplace(&csBuf[i])
	}
	br.csInitialized = true
}

func (br *blockResult) csAdd(rc blockResultColumn) {
	br.csBuf = append(br.csBuf, rc)
	if !br.csInitialized {
		return
	}
	csBuf := br.csBuf
	br.csAddOrReplace(&csBuf[len(csBuf)-1])
}

func (br *blockResult) csAddOrReplace(c *blockResultColumn) {
	idx := getBlockResultColumnIdxByName(br.cs, c.name)
	if idx >= 0 {
		br.cs[idx] = c
	} else {
		br.cs = append(br.cs, c)
	}
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
// This simplifies copying, resetting and reusing of the struct.
type blockResultColumn struct {
	// name is column name
	name string

	// isConst is set to true if the column is const.
	//
	// The column value is stored in valuesEncoded[0]
	isConst bool

	// isTime is set to true if the column contains _time values.
	//
	// The column values are stored in blockResult.getTimestamps, while valuesEncoded is nil.
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

	// chSrc is an optional pointer to the source columnHeader. It is used for reading the values at readValuesEncoded()
	chSrc *columnHeader

	// cSrc is an optional pointer to the source blockResultColumn. It is used for reading the values at readValuesEncoded()
	cSrc *blockResultColumn

	// bucketSizeStr contains bucketSizeStr for valuesBucketed
	bucketSizeStr string

	// bucketOffsetStr contains bucketOffset for valuesBucketed
	bucketOffsetStr string
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

	// Do not copy c.chSrc and c.cSrc, since they may refer to data, which may change over time.
	// We already copied c.valuesEncoded, so c.chSrc and c.cSrc must be nil.

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
//
// See getValues for obtaining non-bucketed values.
func (c *blockResultColumn) getValuesBucketed(br *blockResult, bf *byStatsField) []string {
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
//
// See getValuesBucketed for obtaining bucketed values.
func (c *blockResultColumn) getValues(br *blockResult) []string {
	if values := c.values; values != nil {
		return values
	}

	c.values = br.newValuesForColumn(c)
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
		c.valuesEncoded = c.readValuesEncoded(br)
	}
	return c.valuesEncoded
}

func (c *blockResultColumn) readValuesEncoded(br *blockResult) []string {
	if br.bs != nil {
		return br.readValuesEncodedFromColumnHeader(c.chSrc)
	}
	return br.readValuesEncodedFromResultColumn(c.cSrc)
}

// forEachDictValue calls f for every matching value in the column dictionary.
//
// It properly skips non-matching dict values.
func (c *blockResultColumn) forEachDictValue(br *blockResult, f func(v string)) {
	if c.valueType != valueTypeDict {
		logger.Panicf("BUG: unexpected column valueType=%d; want %d", c.valueType, valueTypeDict)
	}
	if br.isFull() {
		// Fast path - there is no need in reading encoded values
		for _, v := range c.dictValues {
			f(v)
		}
		return
	}

	// Slow path - need to read encoded values in order to filter not referenced columns.
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

func (br *blockResult) isFull() bool {
	if br.bs == nil {
		return false
	}
	return br.bs.bsw.bh.rowsCount == uint64(br.rowsLen)
}

// forEachDictValueWithHits calls f for every matching value in the column dictionary.
//
// It properly skips non-matching dict values.
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
	case valueTypeInt64:
		v := valuesEncoded[rowIdx]
		return float64(unmarshalInt64(v)), true
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
	case valueTypeInt64:
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
	case valueTypeInt64:
		sum := float64(0)
		for _, v := range c.getValuesEncoded(br) {
			sum += float64(unmarshalInt64(v))
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
	// Do not call clear(rc.values), since it is slow when the query processes big number of columns.
	// It is OK if rc.values point to some old values - they will be eventually overwritten by new values.

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
	needStore := cap(values) < rowsLen
	values = slicesutil.SetLength(values, rowsLen)
	if needStore {
		valuesLocal := values
		emptyStrings.Store(&valuesLocal)
	}
	return values
}

var emptyStrings atomic.Pointer[[]string]

func visitValuesReadonly(bs *blockSearch, ch *columnHeader, bm *bitmap, f func(values []string)) {
	if bm.isZero() {
		// Fast path - nothing to visit
		return
	}
	values := bs.getValuesForColumn(ch)
	if bm.areAllBitsSet() {
		// Faster path - visit all the values
		f(values)
		return
	}

	// Slower path - visit only the selected values
	vb := getValuesBuf()
	bm.forEachSetBitReadonly(func(idx int) {
		vb.values = append(vb.values, values[idx])
	})
	f(vb.values)
	putValuesBuf(vb)
}

type valuesBuf struct {
	values []string
}

func getValuesBuf() *valuesBuf {
	v := valuesBufPool.Get()
	if v == nil {
		return &valuesBuf{}
	}
	return v.(*valuesBuf)
}

func putValuesBuf(vb *valuesBuf) {
	vb.values = vb.values[:0]
	valuesBufPool.Put(vb)
}

var valuesBufPool sync.Pool

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
