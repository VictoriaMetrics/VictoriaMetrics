package logstorage

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// filterIn matches any exact value from the values map.
//
// Example LogsQL: `fieldName:in("foo", "bar baz")`
type filterIn struct {
	fieldName string
	values    []string

	// needeExecuteQuery is set to true if q must be executed for populating values before filter execution.
	needExecuteQuery bool

	// If q is non-nil, then values must be populated from q before filter execution.
	q *Query

	// qFieldName must be set to field name for obtaining values from if q is non-nil.
	qFieldName string

	tokenSetsOnce sync.Once
	tokenSets     [][]string

	stringValuesOnce sync.Once
	stringValues     map[string]struct{}

	uint8ValuesOnce sync.Once
	uint8Values     map[string]struct{}

	uint16ValuesOnce sync.Once
	uint16Values     map[string]struct{}

	uint32ValuesOnce sync.Once
	uint32Values     map[string]struct{}

	uint64ValuesOnce sync.Once
	uint64Values     map[string]struct{}

	float64ValuesOnce sync.Once
	float64Values     map[string]struct{}

	ipv4ValuesOnce sync.Once
	ipv4Values     map[string]struct{}

	timestampISO8601ValuesOnce sync.Once
	timestampISO8601Values     map[string]struct{}
}

func (fi *filterIn) String() string {
	args := ""
	if fi.q != nil {
		args = fi.q.String()
	} else {
		values := fi.values
		a := make([]string, len(values))
		for i, value := range values {
			a[i] = quoteTokenIfNeeded(value)
		}
		args = strings.Join(a, ",")
	}
	return fmt.Sprintf("%sin(%s)", quoteFieldNameIfNeeded(fi.fieldName), args)
}

func (fi *filterIn) updateNeededFields(neededFields fieldsSet) {
	neededFields.add(fi.fieldName)
}

func (fi *filterIn) getTokenSets() [][]string {
	fi.tokenSetsOnce.Do(fi.initTokenSets)
	return fi.tokenSets
}

// It is faster to match every row in the block instead of checking too big number of tokenSets against bloom filter.
const maxTokenSetsToInit = 1000

func (fi *filterIn) initTokenSets() {
	values := fi.values
	tokenSetsLen := len(values)
	if tokenSetsLen > maxTokenSetsToInit {
		tokenSetsLen = maxTokenSetsToInit
	}
	tokenSets := make([][]string, 0, tokenSetsLen+1)
	for _, v := range values {
		tokens := tokenizeStrings(nil, []string{v})
		tokenSets = append(tokenSets, tokens)
		if len(tokens) > maxTokenSetsToInit {
			break
		}
	}
	fi.tokenSets = tokenSets
}

func (fi *filterIn) getStringValues() map[string]struct{} {
	fi.stringValuesOnce.Do(fi.initStringValues)
	return fi.stringValues
}

func (fi *filterIn) initStringValues() {
	values := fi.values
	m := make(map[string]struct{}, len(values))
	for _, v := range values {
		m[v] = struct{}{}
	}
	fi.stringValues = m
}

func (fi *filterIn) getUint8Values() map[string]struct{} {
	fi.uint8ValuesOnce.Do(fi.initUint8Values)
	return fi.uint8Values
}

func (fi *filterIn) initUint8Values() {
	values := fi.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*1)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok || n >= (1<<8) {
			continue
		}
		bufLen := len(buf)
		buf = append(buf, byte(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	fi.uint8Values = m
}

func (fi *filterIn) getUint16Values() map[string]struct{} {
	fi.uint16ValuesOnce.Do(fi.initUint16Values)
	return fi.uint16Values
}

func (fi *filterIn) initUint16Values() {
	values := fi.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*2)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok || n >= (1<<16) {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint16(buf, uint16(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	fi.uint16Values = m
}

func (fi *filterIn) getUint32Values() map[string]struct{} {
	fi.uint32ValuesOnce.Do(fi.initUint32Values)
	return fi.uint32Values
}

func (fi *filterIn) initUint32Values() {
	values := fi.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*4)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok || n >= (1<<32) {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint32(buf, uint32(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	fi.uint32Values = m
}

func (fi *filterIn) getUint64Values() map[string]struct{} {
	fi.uint64ValuesOnce.Do(fi.initUint64Values)
	return fi.uint64Values
}

func (fi *filterIn) initUint64Values() {
	values := fi.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*8)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint64(buf, n)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	fi.uint64Values = m
}

func (fi *filterIn) getFloat64Values() map[string]struct{} {
	fi.float64ValuesOnce.Do(fi.initFloat64Values)
	return fi.float64Values
}

func (fi *filterIn) initFloat64Values() {
	values := fi.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*8)
	for _, v := range values {
		f, ok := tryParseFloat64(v)
		if !ok {
			continue
		}
		n := math.Float64bits(f)
		bufLen := len(buf)
		buf = encoding.MarshalUint64(buf, n)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	fi.float64Values = m
}

func (fi *filterIn) getIPv4Values() map[string]struct{} {
	fi.ipv4ValuesOnce.Do(fi.initIPv4Values)
	return fi.ipv4Values
}

func (fi *filterIn) initIPv4Values() {
	values := fi.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*4)
	for _, v := range values {
		n, ok := tryParseIPv4(v)
		if !ok {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint32(buf, uint32(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	fi.ipv4Values = m
}

func (fi *filterIn) getTimestampISO8601Values() map[string]struct{} {
	fi.timestampISO8601ValuesOnce.Do(fi.initTimestampISO8601Values)
	return fi.timestampISO8601Values
}

func (fi *filterIn) initTimestampISO8601Values() {
	values := fi.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*8)
	for _, v := range values {
		n, ok := tryParseTimestampISO8601(v)
		if !ok {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint64(buf, uint64(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	fi.timestampISO8601Values = m
}

func (fi *filterIn) applyToBlockResult(br *blockResult, bm *bitmap) {
	if len(fi.values) == 0 {
		bm.resetBits()
		return
	}

	c := br.getColumnByName(fi.fieldName)
	if c.isConst {
		stringValues := fi.getStringValues()
		v := c.valuesEncoded[0]
		if _, ok := stringValues[v]; !ok {
			bm.resetBits()
		}
		return
	}
	if c.isTime {
		fi.matchColumnByStringValues(br, bm, c)
		return
	}

	switch c.valueType {
	case valueTypeString:
		fi.matchColumnByStringValues(br, bm, c)
	case valueTypeDict:
		stringValues := fi.getStringValues()
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if _, ok := stringValues[v]; ok {
				c = 1
			}
			bb.B = append(bb.B, c)
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			n := valuesEncoded[idx][0]
			return bb.B[n] == 1
		})
		bbPool.Put(bb)
	case valueTypeUint8:
		binValues := fi.getUint8Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeUint16:
		binValues := fi.getUint16Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeUint32:
		binValues := fi.getUint32Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeUint64:
		binValues := fi.getUint64Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeFloat64:
		binValues := fi.getFloat64Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeIPv4:
		binValues := fi.getIPv4Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeTimestampISO8601:
		binValues := fi.getTimestampISO8601Values()
		matchColumnByBinValues(br, bm, c, binValues)
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func (fi *filterIn) matchColumnByStringValues(br *blockResult, bm *bitmap, c *blockResultColumn) {
	stringValues := fi.getStringValues()
	values := c.getValues(br)
	bm.forEachSetBit(func(idx int) bool {
		v := values[idx]
		_, ok := stringValues[v]
		return ok
	})
}

func matchColumnByBinValues(br *blockResult, bm *bitmap, c *blockResultColumn, binValues map[string]struct{}) {
	if len(binValues) == 0 {
		bm.resetBits()
		return
	}
	valuesEncoded := c.getValuesEncoded(br)
	bm.forEachSetBit(func(idx int) bool {
		v := valuesEncoded[idx]
		_, ok := binValues[v]
		return ok
	})
}

func (fi *filterIn) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fi.fieldName

	if len(fi.values) == 0 {
		bm.resetBits()
		return
	}

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		stringValues := fi.getStringValues()
		if _, ok := stringValues[v]; !ok {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty phrase.
		stringValues := fi.getStringValues()
		if _, ok := stringValues[""]; !ok {
			bm.resetBits()
		}
		return
	}

	tokenSets := fi.getTokenSets()

	switch ch.valueType {
	case valueTypeString:
		stringValues := fi.getStringValues()
		matchAnyValue(bs, ch, bm, stringValues, tokenSets)
	case valueTypeDict:
		stringValues := fi.getStringValues()
		matchValuesDictByAnyValue(bs, ch, bm, stringValues)
	case valueTypeUint8:
		binValues := fi.getUint8Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeUint16:
		binValues := fi.getUint16Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeUint32:
		binValues := fi.getUint32Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeUint64:
		binValues := fi.getUint64Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeFloat64:
		binValues := fi.getFloat64Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeIPv4:
		binValues := fi.getIPv4Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeTimestampISO8601:
		binValues := fi.getTimestampISO8601Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchAnyValue(bs *blockSearch, ch *columnHeader, bm *bitmap, values map[string]struct{}, tokenSets [][]string) {
	if len(values) == 0 {
		bm.resetBits()
		return
	}
	if !matchBloomFilterAnyTokenSet(bs, ch, tokenSets) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		_, ok := values[v]
		return ok
	})
}

func matchBloomFilterAnyTokenSet(bs *blockSearch, ch *columnHeader, tokenSets [][]string) bool {
	if len(tokenSets) == 0 {
		return false
	}
	if len(tokenSets) > maxTokenSetsToInit || uint64(len(tokenSets)) > 10*bs.bsw.bh.rowsCount {
		// It is faster to match every row in the block against all the values
		// instead of using bloom filter for too big number of tokenSets.
		return true
	}
	bf := bs.getBloomFilterForColumn(ch)
	for _, tokens := range tokenSets {
		if bf.containsAll(tokens) {
			return true
		}
	}
	return false
}

func matchValuesDictByAnyValue(bs *blockSearch, ch *columnHeader, bm *bitmap, values map[string]struct{}) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if _, ok := values[v]; ok {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}
