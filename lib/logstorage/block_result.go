package logstorage

import (
	"encoding/binary"
	"math"
	"slices"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fastnum"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// blockResult holds results for a single block of log entries.
//
// It is expected that its contents is accessed only from a single goroutine at a time.
type blockResult struct {
	// buf holds all the bytes behind the requested column values in the block.
	buf []byte

	// values holds all the requested column values in the block.
	valuesBuf []string

	// streamID is streamID for the given blockResult.
	streamID streamID

	// timestamps contain timestamps for the selected log entries in the block.
	timestamps []int64

	// csOffset contains cs offset for the requested columns.
	//
	// columns with indexes below csOffset are ignored.
	// This is needed for simplifying data transformations at pipe stages.
	csOffset int

	// cs contains requested columns.
	cs []blockResultColumn
}

func (br *blockResult) reset() {
	br.buf = br.buf[:0]

	clear(br.valuesBuf)
	br.valuesBuf = br.valuesBuf[:0]

	br.streamID.reset()

	br.timestamps = br.timestamps[:0]

	br.csOffset = 0

	clear(br.cs)
	br.cs = br.cs[:0]
}

// setResultColumns sets the given rcs as br columns.
//
// The returned result is valid only until rcs are modified.
func (br *blockResult) setResultColumns(rcs []resultColumn) {
	br.reset()
	if len(rcs) == 0 {
		return
	}

	br.timestamps = fastnum.AppendInt64Zeros(br.timestamps[:0], len(rcs[0].values))

	cs := br.cs
	for _, rc := range rcs {
		cs = append(cs, blockResultColumn{
			name:          rc.name,
			valueType:     valueTypeString,
			encodedValues: rc.values,
		})
	}
	br.cs = cs
}

func (br *blockResult) fetchAllColumns(bs *blockSearch, bm *bitmap) {
	if !br.addStreamColumn(bs) {
		// Skip the current block, since the associated stream tags are missing.
		br.reset()
		return
	}

	br.addTimeColumn()

	// Add _msg column
	v := bs.csh.getConstColumnValue("_msg")
	if v != "" {
		br.addConstColumn("_msg", v)
	} else if ch := bs.csh.getColumnHeader("_msg"); ch != nil {
		br.addColumn(bs, ch, bm)
	} else {
		br.addConstColumn("_msg", "")
	}

	// Add other const columns
	for _, cc := range bs.csh.constColumns {
		if isMsgFieldName(cc.Name) {
			continue
		}
		br.addConstColumn(cc.Name, cc.Value)
	}

	// Add other non-const columns
	chs := bs.csh.columnHeaders
	for i := range chs {
		ch := &chs[i]
		if isMsgFieldName(ch.name) {
			continue
		}
		br.addColumn(bs, ch, bm)
	}
}

func (br *blockResult) fetchRequestedColumns(bs *blockSearch, bm *bitmap) {
	for _, columnName := range bs.bsw.so.neededColumnNames {
		switch columnName {
		case "_stream":
			if !br.addStreamColumn(bs) {
				// Skip the current block, since the associated stream tags are missing.
				br.reset()
				return
			}
		case "_time":
			br.addTimeColumn()
		default:
			v := bs.csh.getConstColumnValue(columnName)
			if v != "" {
				br.addConstColumn(columnName, v)
			} else if ch := bs.csh.getColumnHeader(columnName); ch != nil {
				br.addColumn(bs, ch, bm)
			} else {
				br.addConstColumn(columnName, "")
			}
		}
	}
}

func (br *blockResult) mustInit(bs *blockSearch, bm *bitmap) {
	br.reset()

	br.streamID = bs.bsw.bh.streamID

	if bm.isZero() {
		// Nothing to initialize for zero matching log entries in the block.
		return
	}

	// Initialize timestamps, since they are required for all the further work with br.

	srcTimestamps := bs.getTimestamps()
	if bm.areAllBitsSet() {
		// Fast path - all the rows in the block are selected, so copy all the timestamps without any filtering.
		br.timestamps = append(br.timestamps[:0], srcTimestamps...)
		return
	}

	// Slow path - copy only the needed timestamps to br according to filter results.
	dstTimestamps := br.timestamps[:0]
	bm.forEachSetBit(func(idx int) bool {
		ts := srcTimestamps[idx]
		dstTimestamps = append(dstTimestamps, ts)
		return true
	})
	br.timestamps = dstTimestamps
}

func (br *blockResult) addColumn(bs *blockSearch, ch *columnHeader, bm *bitmap) {
	buf := br.buf
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	var dictValues []string

	appendValue := func(v string) {
		bufLen := len(buf)
		buf = append(buf, v...)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		valuesBuf = append(valuesBuf, s)
	}

	switch ch.valueType {
	case valueTypeString:
		visitValues(bs, ch, bm, func(v string) bool {
			appendValue(v)
			return true
		})
	case valueTypeDict:
		dictValues = ch.valuesDict.values
		visitValues(bs, ch, bm, func(v string) bool {
			if len(v) != 1 {
				logger.Panicf("FATAL: %s: unexpected dict value size for column %q; got %d bytes; want 1 byte", bs.partPath(), ch.name, len(v))
			}
			dictIdx := v[0]
			if int(dictIdx) >= len(dictValues) {
				logger.Panicf("FATAL: %s: too big dict index for column %q: %d; should be smaller than %d", bs.partPath(), ch.name, dictIdx, len(dictValues))
			}
			appendValue(v)
			return true
		})
	case valueTypeUint8:
		visitValues(bs, ch, bm, func(v string) bool {
			if len(v) != 1 {
				logger.Panicf("FATAL: %s: unexpected size for uint8 column %q; got %d bytes; want 1 byte", bs.partPath(), ch.name, len(v))
			}
			appendValue(v)
			return true
		})
	case valueTypeUint16:
		visitValues(bs, ch, bm, func(v string) bool {
			if len(v) != 2 {
				logger.Panicf("FATAL: %s: unexpected size for uint16 column %q; got %d bytes; want 2 bytes", bs.partPath(), ch.name, len(v))
			}
			appendValue(v)
			return true
		})
	case valueTypeUint32:
		visitValues(bs, ch, bm, func(v string) bool {
			if len(v) != 4 {
				logger.Panicf("FATAL: %s: unexpected size for uint32 column %q; got %d bytes; want 4 bytes", bs.partPath(), ch.name, len(v))
			}
			appendValue(v)
			return true
		})
	case valueTypeUint64:
		visitValues(bs, ch, bm, func(v string) bool {
			if len(v) != 8 {
				logger.Panicf("FATAL: %s: unexpected size for uint64 column %q; got %d bytes; want 8 bytes", bs.partPath(), ch.name, len(v))
			}
			appendValue(v)
			return true
		})
	case valueTypeFloat64:
		visitValues(bs, ch, bm, func(v string) bool {
			if len(v) != 8 {
				logger.Panicf("FATAL: %s: unexpected size for float64 column %q; got %d bytes; want 8 bytes", bs.partPath(), ch.name, len(v))
			}
			appendValue(v)
			return true
		})
	case valueTypeIPv4:
		visitValues(bs, ch, bm, func(v string) bool {
			if len(v) != 4 {
				logger.Panicf("FATAL: %s: unexpected size for ipv4 column %q; got %d bytes; want 4 bytes", bs.partPath(), ch.name, len(v))
			}
			appendValue(v)
			return true
		})
	case valueTypeTimestampISO8601:
		visitValues(bs, ch, bm, func(v string) bool {
			if len(v) != 8 {
				logger.Panicf("FATAL: %s: unexpected size for timestmap column %q; got %d bytes; want 8 bytes", bs.partPath(), ch.name, len(v))
			}
			appendValue(v)
			return true
		})
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d for column %q", bs.partPath(), ch.valueType, ch.name)
	}

	encodedValues := valuesBuf[valuesBufLen:]

	valuesBufLen = len(valuesBuf)
	for _, v := range dictValues {
		appendValue(v)
	}
	dictValues = valuesBuf[valuesBufLen:]

	name := getCanonicalColumnName(ch.name)
	br.cs = append(br.cs, blockResultColumn{
		name:          name,
		valueType:     ch.valueType,
		dictValues:    dictValues,
		encodedValues: encodedValues,
	})
	br.buf = buf
	br.valuesBuf = valuesBuf
}

func (br *blockResult) addTimeColumn() {
	br.cs = append(br.cs, blockResultColumn{
		name:   "_time",
		isTime: true,
	})
}

func (br *blockResult) addStreamColumn(bs *blockSearch) bool {
	bb := bbPool.Get()
	defer bbPool.Put(bb)

	bb.B = bs.bsw.p.pt.appendStreamTagsByStreamID(bb.B[:0], &br.streamID)
	if len(bb.B) == 0 {
		// Couldn't find stream tags by streamID. This may be the case when the corresponding log stream
		// was recently registered and its tags aren't visible to search yet.
		// The stream tags must become visible in a few seconds.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6042
		return false
	}

	st := GetStreamTags()
	mustUnmarshalStreamTags(st, bb.B)
	bb.B = st.marshalString(bb.B[:0])
	PutStreamTags(st)

	s := bytesutil.ToUnsafeString(bb.B)
	br.addConstColumn("_stream", s)
	return true
}

func (br *blockResult) addConstColumn(name, value string) {
	buf := br.buf
	bufLen := len(buf)
	buf = append(buf, value...)
	s := bytesutil.ToUnsafeString(buf[bufLen:])
	br.buf = buf

	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)
	valuesBuf = append(valuesBuf, s)
	br.valuesBuf = valuesBuf

	br.cs = append(br.cs, blockResultColumn{
		name:          name,
		isConst:       true,
		encodedValues: valuesBuf[valuesBufLen:],
	})
}

func (br *blockResult) getBucketedColumnValues(c *blockResultColumn, bucketSize, bucketOffset float64) []string {
	if c.isConst {
		return br.getBucketedConstValues(c.encodedValues[0], bucketSize, bucketOffset)
	}
	if c.isTime {
		return br.getBucketedTimestampValues(bucketSize, bucketOffset)
	}

	switch c.valueType {
	case valueTypeString:
		return br.getBucketedStringValues(c.encodedValues, bucketSize, bucketOffset)
	case valueTypeDict:
		return br.getBucketedDictValues(c.encodedValues, c.dictValues, bucketSize, bucketOffset)
	case valueTypeUint8:
		return br.getBucketedUint8Values(c.encodedValues, bucketSize, bucketOffset)
	case valueTypeUint16:
		return br.getBucketedUint16Values(c.encodedValues, bucketSize, bucketOffset)
	case valueTypeUint32:
		return br.getBucketedUint32Values(c.encodedValues, bucketSize, bucketOffset)
	case valueTypeUint64:
		return br.getBucketedUint64Values(c.encodedValues, bucketSize, bucketOffset)
	case valueTypeFloat64:
		return br.getBucketedFloat64Values(c.encodedValues, bucketSize, bucketOffset)
	case valueTypeIPv4:
		return br.getBucketedIPv4Values(c.encodedValues, bucketSize, bucketOffset)
	case valueTypeTimestampISO8601:
		return br.getBucketedTimestampISO8601Values(c.encodedValues, bucketSize, bucketOffset)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return nil
	}
}

func (br *blockResult) getBucketedConstValues(v string, bucketSize, bucketOffset float64) []string {
	if v == "" {
		// Fast path - return a slice of empty strings without constructing the slice.
		return getEmptyStrings(len(br.timestamps))
	}

	// Slower path - construct slice of identical values with the len(br.timestamps)

	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	v = br.getBucketedValue(v, bucketSize, bucketOffset)
	for range br.timestamps {
		valuesBuf = append(valuesBuf, v)
	}

	br.valuesBuf = valuesBuf

	return valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedTimestampValues(bucketSize, bucketOffset float64) []string {
	buf := br.buf
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	timestamps := br.timestamps
	var s string

	if bucketSize <= 1 && bucketOffset == 0 {
		for i := range timestamps {
			if i > 0 && timestamps[i-1] == timestamps[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			bufLen := len(buf)
			buf = marshalTimestampRFC3339Nano(buf, timestamps[i])
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := int64(bucketSize)
		bucketOffsetInt := int64(bucketOffset)
		var prevTimestamp int64
		for i := range timestamps {
			if i > 0 && timestamps[i-1] == timestamps[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			timestamp := timestamps[i]
			timestamp -= bucketOffsetInt
			timestamp -= timestamp % bucketSizeInt
			timestamp += bucketOffsetInt
			if i > 0 && timestamp == prevTimestamp {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			prevTimestamp = timestamp
			bufLen := len(buf)
			buf = marshalTimestampRFC3339Nano(buf, timestamp)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.buf = buf
	br.valuesBuf = valuesBuf

	return valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedStringValues(values []string, bucketSize, bucketOffset float64) []string {
	if bucketSize <= 0 && bucketOffset == 0 {
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

		s = br.getBucketedValue(values[i], bucketSize, bucketOffset)
		valuesBuf = append(valuesBuf, s)
	}

	br.valuesBuf = valuesBuf

	return valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedDictValues(encodedValues, dictValues []string, bucketSize, bucketOffset float64) []string {
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	dictValues = br.getBucketedStringValues(dictValues, bucketSize, bucketOffset)
	for _, v := range encodedValues {
		dictIdx := v[0]
		valuesBuf = append(valuesBuf, dictValues[dictIdx])
	}

	br.valuesBuf = valuesBuf

	return valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedUint8Values(encodedValues []string, bucketSize, bucketOffset float64) []string {
	buf := br.buf
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if (bucketSize <= 1 || bucketSize >= (1<<8)) && bucketOffset == 0 {
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := uint64(v[0])
			bufLen := len(buf)
			buf = marshalUint64(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := uint64(bucketSize)
		bucketOffsetInt := uint64(int64(bucketOffset))
		var nPrev uint64
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			n := uint64(v[0])
			n -= bucketOffsetInt
			n -= n % bucketSizeInt
			n += bucketOffsetInt
			if i > 0 && n == nPrev {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			nPrev = n
			bufLen := len(buf)
			buf = marshalUint64(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.buf = buf

	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedUint16Values(encodedValues []string, bucketSize, bucketOffset float64) []string {
	buf := br.buf
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if (bucketSize <= 1 || bucketSize >= (1<<16)) && bucketOffset == 0 {
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			b := bytesutil.ToUnsafeBytes(v)
			n := uint64(encoding.UnmarshalUint16(b))
			bufLen := len(buf)
			buf = marshalUint64(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := uint64(bucketSize)
		bucketOffsetInt := uint64(int64(bucketOffset))
		var nPrev uint64
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			b := bytesutil.ToUnsafeBytes(v)
			n := uint64(encoding.UnmarshalUint16(b))
			n -= bucketOffsetInt
			n -= n % bucketSizeInt
			n += bucketOffsetInt
			if i > 0 && n == nPrev {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			nPrev = n
			bufLen := len(buf)
			buf = marshalUint64(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.buf = buf

	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedUint32Values(encodedValues []string, bucketSize, bucketOffset float64) []string {
	buf := br.buf
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if (bucketSize <= 1 || bucketSize >= (1<<32)) && bucketOffset == 0 {
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			b := bytesutil.ToUnsafeBytes(v)
			n := uint64(encoding.UnmarshalUint32(b))
			bufLen := len(buf)
			buf = marshalUint64(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := uint64(bucketSize)
		bucketOffsetInt := uint64(int64(bucketOffset))
		var nPrev uint64
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			b := bytesutil.ToUnsafeBytes(v)
			n := uint64(encoding.UnmarshalUint32(b))
			n -= bucketOffsetInt
			n -= n % bucketSizeInt
			n += bucketOffsetInt
			if i > 0 && n == nPrev {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			nPrev = n
			bufLen := len(buf)
			buf = marshalUint64(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.buf = buf

	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedUint64Values(encodedValues []string, bucketSize, bucketOffset float64) []string {
	buf := br.buf
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if (bucketSize <= 1 || bucketSize >= (1<<64)) && bucketOffset == 0 {
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			b := bytesutil.ToUnsafeBytes(v)
			n := encoding.UnmarshalUint64(b)
			bufLen := len(buf)
			buf = marshalUint64(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := uint64(bucketSize)
		bucketOffsetInt := uint64(int64(bucketOffset))
		var nPrev uint64
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			b := bytesutil.ToUnsafeBytes(v)
			n := encoding.UnmarshalUint64(b)
			n -= bucketOffsetInt
			n -= n % bucketSizeInt
			n += bucketOffsetInt
			if i > 0 && n == nPrev {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			nPrev = n
			bufLen := len(buf)
			buf = marshalUint64(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.buf = buf

	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedFloat64Values(encodedValues []string, bucketSize, bucketOffset float64) []string {
	buf := br.buf
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if bucketSize <= 0 && bucketOffset == 0 {
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			b := bytesutil.ToUnsafeBytes(v)
			n := encoding.UnmarshalUint64(b)
			f := math.Float64frombits(n)

			bufLen := len(buf)
			buf = marshalFloat64(buf, f)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		_, e := decimal.FromFloat(bucketSize)
		p10 := math.Pow10(int(-e))
		bucketSizeP10 := int64(bucketSize * p10)
		var fPrev float64
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			b := bytesutil.ToUnsafeBytes(v)
			n := encoding.UnmarshalUint64(b)
			f := math.Float64frombits(n)

			f -= bucketOffset

			// emulate f % bucketSize for float64 values
			fP10 := int64(f * p10)
			fP10 -= fP10 % bucketSizeP10
			f = float64(fP10) / p10

			f += bucketOffset

			if i > 0 && f == fPrev {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			fPrev = f
			bufLen := len(buf)
			buf = marshalFloat64(buf, f)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.buf = buf

	return br.valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedIPv4Values(encodedValues []string, bucketSize, bucketOffset float64) []string {
	buf := br.buf
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if bucketSize <= 1 && bucketOffset == 0 {
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			bufLen := len(buf)
			buf = toIPv4String(buf, v)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := uint32(bucketSize)
		bucketOffsetInt := uint32(int32(bucketOffset))
		var nPrev uint32
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			b := bytesutil.ToUnsafeBytes(v)
			n := binary.BigEndian.Uint32(b)
			n -= bucketOffsetInt
			n -= n % bucketSizeInt
			n += bucketOffsetInt
			if i > 0 && n == nPrev {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			nPrev = n
			bufLen := len(buf)
			buf = marshalIPv4(buf, n)
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	}

	br.valuesBuf = valuesBuf
	br.buf = buf

	return valuesBuf[valuesBufLen:]
}

func (br *blockResult) getBucketedTimestampISO8601Values(encodedValues []string, bucketSize, bucketOffset float64) []string {
	buf := br.buf
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	var s string

	if bucketSize <= 1 && bucketOffset == 0 {
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			b := bytesutil.ToUnsafeBytes(v)
			n := encoding.UnmarshalUint64(b)

			bufLen := len(buf)
			buf = marshalTimestampISO8601(buf, int64(n))
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
	} else {
		bucketSizeInt := uint64(bucketSize)
		bucketOffsetInt := uint64(int64(bucketOffset))
		var nPrev uint64
		bb := bbPool.Get()
		for i, v := range encodedValues {
			if i > 0 && encodedValues[i-1] == encodedValues[i] {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			b := bytesutil.ToUnsafeBytes(v)
			n := encoding.UnmarshalUint64(b)
			n -= bucketOffsetInt
			n -= n % bucketSizeInt
			n += bucketOffsetInt
			if i > 0 && n == nPrev {
				valuesBuf = append(valuesBuf, s)
				continue
			}

			nPrev = n
			bufLen := len(buf)
			buf = marshalTimestampISO8601(buf, int64(n))
			s = bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
		bbPool.Put(bb)
	}

	br.valuesBuf = valuesBuf
	br.buf = buf

	return valuesBuf[valuesBufLen:]
}

// getBucketedValue returns bucketed s according to the given bucketSize and bucketOffset
func (br *blockResult) getBucketedValue(s string, bucketSize, bucketOffset float64) string {
	if bucketSize <= 0 && bucketOffset == 0 {
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
		f -= bucketOffset

		// emulate f % bucketSize for float64 values
		_, e := decimal.FromFloat(bucketSize)
		p10 := math.Pow10(int(-e))
		fP10 := int64(f * p10)
		fP10 -= fP10 % int64(bucketSize*p10)
		f = float64(fP10) / p10

		f += bucketOffset

		bufLen := len(br.buf)
		br.buf = marshalFloat64(br.buf, f)
		return bytesutil.ToUnsafeString(br.buf[bufLen:])
	}

	if nsecs, ok := tryParseTimestampISO8601(s); ok {
		nsecs -= int64(bucketOffset)
		nsecs -= nsecs % int64(bucketSize)
		nsecs += int64(bucketOffset)
		bufLen := len(br.buf)
		br.buf = marshalTimestampISO8601(br.buf, nsecs)
		return bytesutil.ToUnsafeString(br.buf[bufLen:])
	}

	if nsecs, ok := tryParseTimestampRFC3339Nano(s); ok {
		nsecs -= int64(bucketOffset)
		nsecs -= nsecs % int64(bucketSize)
		nsecs += int64(bucketOffset)
		bufLen := len(br.buf)
		br.buf = marshalTimestampRFC3339Nano(br.buf, nsecs)
		return bytesutil.ToUnsafeString(br.buf[bufLen:])
	}

	if n, ok := tryParseIPv4(s); ok {
		n -= uint32(int32(bucketOffset))
		n -= n % uint32(bucketSize)
		n += uint32(int32(bucketOffset))
		bufLen := len(br.buf)
		br.buf = marshalIPv4(br.buf, n)
		return bytesutil.ToUnsafeString(br.buf[bufLen:])
	}

	if nsecs, ok := tryParseDuration(s); ok {
		nsecs -= int64(bucketOffset)
		nsecs -= nsecs % int64(bucketSize)
		nsecs += int64(bucketOffset)
		bufLen := len(br.buf)
		br.buf = marshalDuration(br.buf, nsecs)
		return bytesutil.ToUnsafeString(br.buf[bufLen:])
	}

	// Couldn't parse s, so return it as is.
	return s
}

func (br *blockResult) addEmptyStringColumn(columnName string) {
	br.cs = append(br.cs, blockResultColumn{
		name:      columnName,
		valueType: valueTypeString,
	})
}

// copyColumns copies columns from srcColumnNames to dstColumnNames.
func (br *blockResult) copyColumns(srcColumnNames, dstColumnNames []string) {
	if len(srcColumnNames) == 0 {
		return
	}

	cs := br.cs
	csOffset := len(cs)
	for _, c := range br.getColumns() {
		if idx := slices.Index(srcColumnNames, c.name); idx >= 0 {
			c.name = dstColumnNames[idx]
			cs = append(cs, c)
			// continue is skipped intentionally in order to leave the original column in the columns list.
		}
		if !slices.Contains(dstColumnNames, c.name) {
			cs = append(cs, c)
		}
	}
	br.csOffset = csOffset
	br.cs = cs

	for _, dstColumnName := range dstColumnNames {
		br.createMissingColumnByName(dstColumnName)
	}
}

// renameColumns renames columns from srcColumnNames to dstColumnNames.
func (br *blockResult) renameColumns(srcColumnNames, dstColumnNames []string) {
	if len(srcColumnNames) == 0 {
		return
	}

	cs := br.cs
	csOffset := len(cs)
	for _, c := range br.getColumns() {
		if idx := slices.Index(srcColumnNames, c.name); idx >= 0 {
			c.name = dstColumnNames[idx]
			cs = append(cs, c)
			continue
		}
		if !slices.Contains(dstColumnNames, c.name) {
			cs = append(cs, c)
		}
	}
	br.csOffset = csOffset
	br.cs = cs

	for _, dstColumnName := range dstColumnNames {
		br.createMissingColumnByName(dstColumnName)
	}
}

// deleteColumns deletes columns with the given columnNames.
func (br *blockResult) deleteColumns(columnNames []string) {
	if len(columnNames) == 0 {
		return
	}

	cs := br.cs
	csOffset := len(cs)
	for _, c := range br.getColumns() {
		if !slices.Contains(columnNames, c.name) {
			cs = append(cs, c)
		}
	}
	br.csOffset = csOffset
	br.cs = cs
}

// setColumns sets the resulting columns to the given columnNames.
func (br *blockResult) setColumns(columnNames []string) {
	if br.areSameColumns(columnNames) {
		// Fast path - nothing to change.
		return
	}

	// Slow path - construct the requested columns
	cs := br.cs
	csOffset := len(cs)
	for _, columnName := range columnNames {
		c := br.getColumnByName(columnName)
		cs = append(cs, c)
	}
	br.csOffset = csOffset
	br.cs = cs
}

func (br *blockResult) areSameColumns(columnNames []string) bool {
	cs := br.getColumns()
	if len(cs) != len(columnNames) {
		return false
	}
	for i := range cs {
		if cs[i].name != columnNames[i] {
			return false
		}
	}
	return true
}

func (br *blockResult) getColumnByName(columnName string) blockResultColumn {
	cs := br.getColumns()
	for i := range cs {
		if cs[i].name == columnName {
			return cs[i]
		}
	}

	return blockResultColumn{
		name:          columnName,
		isConst:       true,
		encodedValues: getEmptyStrings(1),
	}
}

func (br *blockResult) createMissingColumnByName(columnName string) {
	cs := br.getColumns()
	for i := range cs {
		if cs[i].name == columnName {
			return
		}
	}

	br.cs = append(br.cs, blockResultColumn{
		name:          columnName,
		isConst:       true,
		encodedValues: getEmptyStrings(1),
	})
}

func (br *blockResult) getColumns() []blockResultColumn {
	return br.cs[br.csOffset:]
}

func (br *blockResult) skipRows(skipRows int) {
	br.timestamps = append(br.timestamps[:0], br.timestamps[skipRows:]...)
	cs := br.getColumns()
	for i := range cs {
		c := &cs[i]
		if c.values != nil {
			c.values = append(c.values[:0], c.values[skipRows:]...)
		}
		if c.isConst {
			continue
		}
		if c.encodedValues != nil {
			c.encodedValues = append(c.encodedValues[:0], c.encodedValues[skipRows:]...)
		}
	}
}

func (br *blockResult) truncateRows(keepRows int) {
	br.timestamps = br.timestamps[:keepRows]
	cs := br.getColumns()
	for i := range cs {
		c := &cs[i]
		if c.values != nil {
			c.values = c.values[:keepRows]
		}
		if c.isConst {
			continue
		}
		if c.encodedValues != nil {
			c.encodedValues = c.encodedValues[:keepRows]
		}
	}
}

// blockResultColumn represents named column from blockResult.
//
// blockResultColumn doesn't own any referred data - all the referred data must be owned by blockResult.
// This simplifies copying, resetting and re-using of the struct.
type blockResultColumn struct {
	// name is column name.
	name string

	// isConst is set to true if the column is const.
	//
	// The column value is stored in encodedValues[0]
	isConst bool

	// isTime is set to true if the column contains _time values.
	//
	// The column values are stored in blockResult.timestamps
	isTime bool

	// valueType is the type of non-cost value
	valueType valueType

	// dictValues contains dictionary values for valueTypeDict column
	dictValues []string

	// encodedValues contains encoded values for non-const column
	encodedValues []string

	// values contains decoded values after getValues() call
	values []string

	// bucketedValues contains values after getBucketedValues() call
	bucketedValues []string

	// bucketSize contains bucketSize for bucketedValues
	bucketSize float64

	// bucketOffset contains bucketOffset for bucketedValues
	bucketOffset float64
}

// getValueAtRow returns value for the value at the given rowIdx.
//
// The returned value is valid until br.reset() is called.
func (c *blockResultColumn) getValueAtRow(br *blockResult, rowIdx int) string {
	if c.isConst {
		// Fast path for const column.
		return c.encodedValues[0]
	}
	if c.values != nil {
		// Fast path, which avoids call overhead for getValues().
		return c.values[rowIdx]
	}

	// Slow path - decode all the values and return the given value.
	values := c.getValues(br)
	return values[rowIdx]
}

// getValues returns values for the given column, bucketed according to bucketSize and bucketOffset.
//
// The returned values are valid until br.reset() is called.
func (c *blockResultColumn) getBucketedValues(br *blockResult, bucketSize, bucketOffset float64) []string {
	if bucketSize <= 0 {
		return c.getValues(br)
	}
	if values := c.bucketedValues; values != nil && c.bucketSize == bucketSize && c.bucketOffset == bucketOffset {
		return values
	}

	c.bucketedValues = br.getBucketedColumnValues(c, bucketSize, bucketOffset)
	c.bucketSize = bucketSize
	c.bucketOffset = bucketOffset
	return c.bucketedValues
}

// getValues returns values for the given column.
//
// The returned values are valid until br.reset() is called.
func (c *blockResultColumn) getValues(br *blockResult) []string {
	if values := c.values; values != nil {
		return values
	}

	c.values = br.getBucketedColumnValues(c, 0, 0)
	return c.values
}

func (c *blockResultColumn) getFloatValueAtRow(rowIdx int) float64 {
	if c.isConst {
		v := c.encodedValues[0]
		f, ok := tryParseFloat64(v)
		if !ok {
			return nan
		}
		return f
	}
	if c.isTime {
		return nan
	}

	switch c.valueType {
	case valueTypeString:
		f, ok := tryParseFloat64(c.encodedValues[rowIdx])
		if !ok {
			return nan
		}
		return f
	case valueTypeDict:
		dictIdx := c.encodedValues[rowIdx][0]
		f, ok := tryParseFloat64(c.dictValues[dictIdx])
		if !ok {
			return nan
		}
		return f
	case valueTypeUint8:
		return float64(c.encodedValues[rowIdx][0])
	case valueTypeUint16:
		b := bytesutil.ToUnsafeBytes(c.encodedValues[rowIdx])
		return float64(encoding.UnmarshalUint16(b))
	case valueTypeUint32:
		b := bytesutil.ToUnsafeBytes(c.encodedValues[rowIdx])
		return float64(encoding.UnmarshalUint32(b))
	case valueTypeUint64:
		b := bytesutil.ToUnsafeBytes(c.encodedValues[rowIdx])
		return float64(encoding.UnmarshalUint64(b))
	case valueTypeFloat64:
		b := bytesutil.ToUnsafeBytes(c.encodedValues[rowIdx])
		n := encoding.UnmarshalUint64(b)
		return math.Float64frombits(n)
	case valueTypeIPv4:
		return nan
	case valueTypeTimestampISO8601:
		return nan
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return nan
	}
}

func (c *blockResultColumn) getMaxValue(br *blockResult) float64 {
	if c.isConst {
		v := c.encodedValues[0]
		f, ok := tryParseFloat64(v)
		if !ok {
			return nan
		}
		return f
	}
	if c.isTime {
		return nan
	}

	switch c.valueType {
	case valueTypeString:
		max := nan
		f := float64(0)
		ok := false
		values := c.encodedValues
		for i := range values {
			if i == 0 || values[i-1] != values[i] {
				f, ok = tryParseFloat64(values[i])
			}
			if ok && (f > max || math.IsNaN(max)) {
				max = f
			}
		}
		return max
	case valueTypeDict:
		a := encoding.GetFloat64s(len(c.dictValues))
		dictValuesFloat := a.A
		for i, v := range c.dictValues {
			f, ok := tryParseFloat64(v)
			if !ok {
				f = nan
			}
			dictValuesFloat[i] = f
		}
		max := nan
		for _, v := range c.encodedValues {
			dictIdx := v[0]
			f := dictValuesFloat[dictIdx]
			if f > max || math.IsNaN(max) {
				max = f
			}
		}
		encoding.PutFloat64s(a)
		return max
	case valueTypeUint8:
		max := -inf
		for _, v := range c.encodedValues {
			f := float64(v[0])
			if f > max {
				max = f
			}
		}
		return max
	case valueTypeUint16:
		max := -inf
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			f := float64(encoding.UnmarshalUint16(b))
			if f > max {
				max = f
			}
		}
		return max
	case valueTypeUint32:
		max := -inf
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			f := float64(encoding.UnmarshalUint32(b))
			if f > max {
				max = f
			}
		}
		return max
	case valueTypeUint64:
		max := -inf
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			f := float64(encoding.UnmarshalUint64(b))
			if f > max {
				max = f
			}
		}
		return max
	case valueTypeIPv4:
		return nan
	case valueTypeTimestampISO8601:
		return nan
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return nan
	}
}

func (c *blockResultColumn) getMinValue(br *blockResult) float64 {
	if c.isConst {
		v := c.encodedValues[0]
		f, ok := tryParseFloat64(v)
		if !ok {
			return nan
		}
		return f
	}
	if c.isTime {
		return nan
	}

	switch c.valueType {
	case valueTypeString:
		min := nan
		f := float64(0)
		ok := false
		values := c.encodedValues
		for i := range values {
			if i == 0 || values[i-1] != values[i] {
				f, ok = tryParseFloat64(values[i])
			}
			if ok && (f < min || math.IsNaN(min)) {
				min = f
			}
		}
		return min
	case valueTypeDict:
		a := encoding.GetFloat64s(len(c.dictValues))
		dictValuesFloat := a.A
		for i, v := range c.dictValues {
			f, ok := tryParseFloat64(v)
			if !ok {
				f = nan
			}
			dictValuesFloat[i] = f
		}
		min := nan
		for _, v := range c.encodedValues {
			dictIdx := v[0]
			f := dictValuesFloat[dictIdx]
			if f < min || math.IsNaN(min) {
				min = f
			}
		}
		encoding.PutFloat64s(a)
		return min
	case valueTypeUint8:
		min := inf
		for _, v := range c.encodedValues {
			f := float64(v[0])
			if f < min {
				min = f
			}
		}
		return min
	case valueTypeUint16:
		min := inf
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			f := float64(encoding.UnmarshalUint16(b))
			if f < min {
				min = f
			}
		}
		return min
	case valueTypeUint32:
		min := inf
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			f := float64(encoding.UnmarshalUint32(b))
			if f < min {
				min = f
			}
		}
		return min
	case valueTypeUint64:
		min := inf
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			f := float64(encoding.UnmarshalUint64(b))
			if f < min {
				min = f
			}
		}
		return min
	case valueTypeIPv4:
		return nan
	case valueTypeTimestampISO8601:
		return nan
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return nan
	}
}

func (c *blockResultColumn) sumValues(br *blockResult) (float64, int) {
	if c.isConst {
		v := c.encodedValues[0]
		f, ok := tryParseFloat64(v)
		if !ok {
			return 0, 0
		}
		return f * float64(len(br.timestamps)), len(br.timestamps)
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
		values := c.encodedValues
		for i := range values {
			if i == 0 || values[i-1] != values[i] {
				f, ok = tryParseFloat64(values[i])
			}
			if ok {
				sum += f
				count++
			}
		}
		return sum, count
	case valueTypeDict:
		a := encoding.GetFloat64s(len(c.dictValues))
		dictValuesFloat := a.A
		for i, v := range c.dictValues {
			f, ok := tryParseFloat64(v)
			if !ok {
				f = nan
			}
			dictValuesFloat[i] = f
		}
		sum := float64(0)
		count := 0
		for _, v := range c.encodedValues {
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
		for _, v := range c.encodedValues {
			sum += uint64(v[0])
		}
		return float64(sum), len(br.timestamps)
	case valueTypeUint16:
		sum := uint64(0)
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			sum += uint64(encoding.UnmarshalUint16(b))
		}
		return float64(sum), len(br.timestamps)
	case valueTypeUint32:
		sum := uint64(0)
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			sum += uint64(encoding.UnmarshalUint32(b))
		}
		return float64(sum), len(br.timestamps)
	case valueTypeUint64:
		sum := float64(0)
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			sum += float64(encoding.UnmarshalUint64(b))
		}
		return sum, len(br.timestamps)
	case valueTypeFloat64:
		sum := float64(0)
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			n := encoding.UnmarshalUint64(b)
			f := math.Float64frombits(n)
			if !math.IsNaN(f) {
				sum += f
			}
		}
		return sum, len(br.timestamps)
	case valueTypeIPv4:
		return 0, 0
	case valueTypeTimestampISO8601:
		return 0, 0
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return 0, 0
	}
}

// resultColumn represents a column with result values
type resultColumn struct {
	// name is column name.
	name string

	// buf contains values data.
	buf []byte

	// values is the result values. They are backed by data inside buf.
	values []string
}

func (rc *resultColumn) resetKeepName() {
	rc.buf = rc.buf[:0]

	clear(rc.values)
	rc.values = rc.values[:0]
}

// addValue adds the given values v to rc.
func (rc *resultColumn) addValue(v string) {
	bufLen := len(rc.buf)
	rc.buf = append(rc.buf, v...)
	rc.values = append(rc.values, bytesutil.ToUnsafeString(rc.buf[bufLen:]))
}

var nan = math.NaN()
var inf = math.Inf(1)
