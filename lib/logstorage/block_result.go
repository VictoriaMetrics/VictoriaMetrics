package logstorage

import (
	"math"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

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

	cs := br.cs
	for i := range cs {
		cs[i].reset()
	}
	br.cs = cs[:0]
}

func (br *blockResult) resetRows() {
	br.buf = br.buf[:0]

	clear(br.valuesBuf)
	br.valuesBuf = br.valuesBuf[:0]

	br.timestamps = br.timestamps[:0]

	cs := br.getColumns()
	for i := range cs {
		cs[i].resetRows()
	}
}

func (br *blockResult) addRow(timestamp int64, values []string) {
	br.timestamps = append(br.timestamps, timestamp)

	cs := br.getColumns()
	if len(values) != len(cs) {
		logger.Panicf("BUG: unexpected number of values in a row; got %d; want %d", len(values), len(cs))
	}
	for i := range cs {
		cs[i].addValue(values[i])
	}
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
	for _, columnName := range bs.bsw.so.resultColumnNames {
		if columnName == "" {
			columnName = "_msg"
		}
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

	name := ch.name
	if name == "" {
		name = "_msg"
	}
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

func (br *blockResult) addEmptyStringColumn(columnName string) {
	br.cs = append(br.cs, blockResultColumn{
		name:      columnName,
		valueType: valueTypeString,
	})
}

func (br *blockResult) updateColumns(columnNames []string) {
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
	if columnName == "" {
		columnName = "_msg"
	}

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

func (br *blockResult) appendColumnValues(dst [][]string, columnNames []string) [][]string {
	for _, columnName := range columnNames {
		c := br.getColumnByName(columnName)
		values := c.getValues(br)
		dst = append(dst, values)
	}
	return dst
}

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

	// dictValues contain dictionary values for valueTypeDict column
	dictValues []string

	// encodedValues contain encoded values for non-const column
	encodedValues []string

	// values contain decoded values after getValues() call for the given column
	values []string

	// buf and valuesBuf are used by addValue() in order to re-use memory across resetRows().
	buf       []byte
	valuesBuf []string
}

func (c *blockResultColumn) reset() {
	c.name = ""
	c.isConst = false
	c.isTime = false
	c.valueType = valueTypeUnknown
	c.dictValues = nil
	c.encodedValues = nil
	c.values = nil

	c.buf = c.buf[:0]

	clear(c.valuesBuf)
	c.valuesBuf = c.valuesBuf[:0]
}

func (c *blockResultColumn) resetRows() {
	c.dictValues = nil
	c.encodedValues = nil
	c.values = nil

	c.buf = c.buf[:0]

	clear(c.valuesBuf)
	c.valuesBuf = c.valuesBuf[:0]
}

func (c *blockResultColumn) addValue(v string) {
	if c.valueType != valueTypeString {
		logger.Panicf("BUG: unexpected column type; got %d; want %d", c.valueType, valueTypeString)
	}

	bufLen := len(c.buf)
	c.buf = append(c.buf, v...)
	c.valuesBuf = append(c.valuesBuf, bytesutil.ToUnsafeString(c.buf[bufLen:]))

	c.encodedValues = c.valuesBuf
	c.values = c.valuesBuf
}

// getEncodedValues returns encoded values for the given column.
//
// The returned encoded values are valid until br.reset() is called.
func (c *blockResultColumn) getEncodedValues(br *blockResult) []string {
	if c.encodedValues != nil {
		return c.encodedValues
	}

	if !c.isTime {
		logger.Panicf("BUG: encodedValues may be missing only for _time column; got %q column", c.name)
	}

	buf := br.buf
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	for _, timestamp := range br.timestamps {
		bufLen := len(buf)
		buf = encoding.MarshalInt64(buf, timestamp)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		valuesBuf = append(valuesBuf, s)
	}

	c.encodedValues = valuesBuf[valuesBufLen:]

	br.valuesBuf = valuesBuf
	br.buf = buf

	return c.encodedValues
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

// getValues returns values for the given column.
//
// The returned values are valid until br.reset() is called.
func (c *blockResultColumn) getValues(br *blockResult) []string {
	if c.values != nil {
		return c.values
	}

	buf := br.buf
	valuesBuf := br.valuesBuf
	valuesBufLen := len(valuesBuf)

	if c.isConst {
		v := c.encodedValues[0]
		if v == "" {
			// Fast path - return a slice of empty strings without constructing it.
			c.values = getEmptyStrings(len(br.timestamps))
			return c.values
		}

		// Slower path - construct slice of identical values with the len(br.timestamps)
		for range br.timestamps {
			valuesBuf = append(valuesBuf, v)
		}
		c.values = valuesBuf[valuesBufLen:]
		br.valuesBuf = valuesBuf
		return c.values
	}
	if c.isTime {
		for _, timestamp := range br.timestamps {
			t := time.Unix(0, timestamp).UTC()
			bufLen := len(buf)
			buf = t.AppendFormat(buf, time.RFC3339Nano)
			s := bytesutil.ToUnsafeString(buf[bufLen:])
			valuesBuf = append(valuesBuf, s)
		}
		c.values = valuesBuf[valuesBufLen:]
		br.buf = buf
		br.valuesBuf = valuesBuf
		return c.values
	}

	appendValue := func(v string) {
		bufLen := len(buf)
		buf = append(buf, v...)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		valuesBuf = append(valuesBuf, s)
	}

	switch c.valueType {
	case valueTypeString:
		c.values = c.encodedValues
		return c.values
	case valueTypeDict:
		dictValues := c.dictValues
		for _, v := range c.encodedValues {
			dictIdx := v[0]
			appendValue(dictValues[dictIdx])
		}
	case valueTypeUint8:
		bb := bbPool.Get()
		for _, v := range c.encodedValues {
			n := uint64(v[0])
			bb.B = strconv.AppendUint(bb.B[:0], n, 10)
			appendValue(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeUint16:
		bb := bbPool.Get()
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			n := uint64(encoding.UnmarshalUint16(b))
			bb.B = strconv.AppendUint(bb.B[:0], n, 10)
			appendValue(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeUint32:
		bb := bbPool.Get()
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			n := uint64(encoding.UnmarshalUint32(b))
			bb.B = strconv.AppendUint(bb.B[:0], n, 10)
			appendValue(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeUint64:
		bb := bbPool.Get()
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			n := encoding.UnmarshalUint64(b)
			bb.B = strconv.AppendUint(bb.B[:0], n, 10)
			appendValue(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeFloat64:
		bb := bbPool.Get()
		for _, v := range c.encodedValues {
			bb.B = toFloat64String(bb.B[:0], v)
			appendValue(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeIPv4:
		bb := bbPool.Get()
		for _, v := range c.encodedValues {
			bb.B = toIPv4String(bb.B[:0], v)
			appendValue(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeTimestampISO8601:
		bb := bbPool.Get()
		for _, v := range c.encodedValues {
			bb.B = toTimestampISO8601String(bb.B[:0], v)
			appendValue(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
	}

	c.values = valuesBuf[valuesBufLen:]
	br.buf = buf
	br.valuesBuf = valuesBuf

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
		max := math.Inf(-1)
		f := float64(0)
		ok := false
		values := c.encodedValues
		for i := range values {
			if i == 0 || values[i-1] != values[i] {
				f, ok = tryParseFloat64(values[i])
			}
			if ok && f > max {
				max = f
			}
		}
		if math.IsInf(max, -1) {
			return nan
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
		max := math.Inf(-1)
		for _, v := range c.encodedValues {
			dictIdx := v[0]
			f := dictValuesFloat[dictIdx]
			if f > max {
				max = f
			}
		}
		encoding.PutFloat64s(a)
		if math.IsInf(max, -1) {
			return nan
		}
		return max
	case valueTypeUint8:
		max := math.Inf(-1)
		for _, v := range c.encodedValues {
			f := float64(v[0])
			if f > max {
				max = f
			}
		}
		return max
	case valueTypeUint16:
		max := math.Inf(-1)
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			f := float64(encoding.UnmarshalUint16(b))
			if f > max {
				max = f
			}
		}
		return max
	case valueTypeUint32:
		max := math.Inf(-1)
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			f := float64(encoding.UnmarshalUint32(b))
			if f > max {
				max = f
			}
		}
		return max
	case valueTypeUint64:
		max := math.Inf(-1)
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

func (c *blockResultColumn) sumValues(br *blockResult) float64 {
	if c.isConst {
		v := c.encodedValues[0]
		f, ok := tryParseFloat64(v)
		if !ok {
			return 0
		}
		return f * float64(len(br.timestamps))
	}
	if c.isTime {
		return 0
	}

	switch c.valueType {
	case valueTypeString:
		sum := float64(0)
		f := float64(0)
		ok := false
		values := c.encodedValues
		for i := range values {
			if i == 0 || values[i-1] != values[i] {
				f, ok = tryParseFloat64(values[i])
			}
			if ok {
				sum += f
			}
		}
		return sum
	case valueTypeDict:
		a := encoding.GetFloat64s(len(c.dictValues))
		dictValuesFloat := a.A
		for i, v := range c.dictValues {
			f, ok := tryParseFloat64(v)
			if !ok {
				f = 0
			}
			dictValuesFloat[i] = f
		}
		sum := float64(0)
		for _, v := range c.encodedValues {
			dictIdx := v[0]
			sum += dictValuesFloat[dictIdx]
		}
		encoding.PutFloat64s(a)
		return sum
	case valueTypeUint8:
		sum := uint64(0)
		for _, v := range c.encodedValues {
			sum += uint64(v[0])
		}
		return float64(sum)
	case valueTypeUint16:
		sum := uint64(0)
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			sum += uint64(encoding.UnmarshalUint16(b))
		}
		return float64(sum)
	case valueTypeUint32:
		sum := uint64(0)
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			sum += uint64(encoding.UnmarshalUint32(b))
		}
		return float64(sum)
	case valueTypeUint64:
		sum := float64(0)
		for _, v := range c.encodedValues {
			b := bytesutil.ToUnsafeBytes(v)
			sum += float64(encoding.UnmarshalUint64(b))
		}
		return sum
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
		return sum
	case valueTypeIPv4:
		return 0
	case valueTypeTimestampISO8601:
		return 0
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		return 0
	}
}

var nan = math.NaN()
