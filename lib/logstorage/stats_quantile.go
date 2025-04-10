package logstorage

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unsafe"

	"github.com/valyala/fastrand"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type statsQuantile struct {
	fields []string

	phi    float64
	phiStr string
}

func (sq *statsQuantile) String() string {
	s := "quantile(" + sq.phiStr
	if len(sq.fields) > 0 {
		s += ", " + fieldNamesString(sq.fields)
	}
	s += ")"
	return s
}

func (sq *statsQuantile) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, sq.fields)
}

func (sq *statsQuantile) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsQuantileProcessor()
}

type statsQuantileProcessor struct {
	h histogram
}

func (sqp *statsQuantileProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sq := sf.(*statsQuantile)
	stateSizeIncrease := 0

	fields := sq.fields
	if len(fields) == 0 {
		for _, c := range br.getColumns() {
			stateSizeIncrease += sqp.updateStateForColumn(br, c)
		}
	} else {
		for _, field := range fields {
			c := br.getColumnByName(field)
			stateSizeIncrease += sqp.updateStateForColumn(br, c)
		}
	}

	return stateSizeIncrease
}

func (sqp *statsQuantileProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sq := sf.(*statsQuantile)
	h := &sqp.h
	stateSizeIncrease := 0

	fields := sq.fields
	if len(fields) == 0 {
		for _, c := range br.getColumns() {
			v := c.getValueAtRow(br, rowIdx)
			stateSizeIncrease += h.update(v)
		}
	} else {
		for _, field := range fields {
			c := br.getColumnByName(field)
			v := c.getValueAtRow(br, rowIdx)
			stateSizeIncrease += h.update(v)
		}
	}

	return stateSizeIncrease
}

func (sqp *statsQuantileProcessor) updateStateForColumn(br *blockResult, c *blockResultColumn) int {
	h := &sqp.h
	stateSizeIncrease := 0

	if c.isConst {
		v := c.valuesEncoded[0]
		for i := 0; i < br.rowsLen; i++ {
			stateSizeIncrease += h.update(v)
		}
		return stateSizeIncrease
	}
	if c.isTime {
		timestamps := br.getTimestamps()
		bb := bbPool.Get()
		for _, ts := range timestamps {
			bb.B = marshalTimestampRFC3339NanoString(bb.B[:0], ts)
			stateSizeIncrease += h.update(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
		return stateSizeIncrease
	}

	switch c.valueType {
	case valueTypeString:
		for _, v := range c.getValues(br) {
			stateSizeIncrease += h.update(v)
		}
	case valueTypeDict:
		dictValues := c.dictValues
		for _, ve := range c.getValuesEncoded(br) {
			idx := ve[0]
			v := dictValues[idx]
			stateSizeIncrease += h.update(v)
		}
	case valueTypeUint8:
		bb := bbPool.Get()
		for _, v := range c.getValuesEncoded(br) {
			n := unmarshalUint8(v)
			bb.B = marshalUint8String(bb.B[:0], n)
			stateSizeIncrease += h.update(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeUint16:
		bb := bbPool.Get()
		for _, v := range c.getValuesEncoded(br) {
			n := unmarshalUint16(v)
			bb.B = marshalUint16String(bb.B[:0], n)
			stateSizeIncrease += h.update(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeUint32:
		bb := bbPool.Get()
		for _, v := range c.getValuesEncoded(br) {
			n := unmarshalUint32(v)
			bb.B = marshalUint32String(bb.B[:0], n)
			stateSizeIncrease += h.update(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeUint64:
		bb := bbPool.Get()
		for _, v := range c.getValuesEncoded(br) {
			n := unmarshalUint64(v)
			bb.B = marshalUint64String(bb.B[:0], n)
			stateSizeIncrease += h.update(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeInt64:
		bb := bbPool.Get()
		for _, v := range c.getValuesEncoded(br) {
			n := unmarshalInt64(v)
			bb.B = marshalInt64String(bb.B[:0], n)
			stateSizeIncrease += h.update(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeFloat64:
		bb := bbPool.Get()
		for _, v := range c.getValuesEncoded(br) {
			f := unmarshalFloat64(v)
			bb.B = marshalFloat64String(bb.B[:0], f)
			stateSizeIncrease += h.update(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeIPv4:
		bb := bbPool.Get()
		for _, v := range c.getValuesEncoded(br) {
			n := unmarshalIPv4(v)
			bb.B = marshalIPv4String(bb.B[:0], n)
			stateSizeIncrease += h.update(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	case valueTypeTimestampISO8601:
		bb := bbPool.Get()
		for _, v := range c.getValuesEncoded(br) {
			n := unmarshalTimestampISO8601(v)
			bb.B = marshalTimestampISO8601String(bb.B[:0], n)
			stateSizeIncrease += h.update(bytesutil.ToUnsafeString(bb.B))
		}
		bbPool.Put(bb)
	default:
		logger.Panicf("BUG: unexpected valueType=%d", c.valueType)
	}

	return stateSizeIncrease
}

func (sqp *statsQuantileProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsQuantileProcessor)
	sqp.h.mergeState(&src.h)
}

func (sqp *statsQuantileProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	return sqp.h.exportState(dst)
}

func (sqp *statsQuantileProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	return sqp.h.importState(src)
}

func (sqp *statsQuantileProcessor) finalizeStats(sf statsFunc, dst []byte, _ <-chan struct{}) []byte {
	sq := sf.(*statsQuantile)
	q := sqp.h.quantile(sq.phi)
	return append(dst, q...)
}

func parseStatsQuantile(lex *lexer) (*statsQuantile, error) {
	if !lex.isKeyword("quantile") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "quantile")
	}
	lex.nextToken()

	fields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'quantile' args: %w", err)
	}
	if len(fields) < 1 {
		return nil, fmt.Errorf("'quantile' must have at least phi arg")
	}

	// Parse phi
	phiStr := fields[0]
	phi, ok := tryParseFloat64(phiStr)
	if !ok {
		return nil, fmt.Errorf("phi arg in 'quantile' must be floating point number; got %q", phiStr)
	}
	if phi < 0 || phi > 1 {
		return nil, fmt.Errorf("phi arg in 'quantile' must be in the range [0..1]; got %q", phiStr)
	}

	// Parse fields
	fields = fields[1:]
	if slices.Contains(fields, "*") {
		fields = nil
	}

	sq := &statsQuantile{
		fields: fields,

		phi:    phi,
		phiStr: phiStr,
	}
	return sq, nil
}

type histogram struct {
	a     []string
	min   string
	max   string
	count uint64

	rng fastrand.RNG
}

func (h *histogram) exportState(dst []byte) []byte {
	dst = encoding.MarshalVarUint64(dst, uint64(len(h.a)))
	for _, v := range h.a {
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(v))
	}

	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(h.min))
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(h.max))
	dst = encoding.MarshalVarUint64(dst, h.count)

	return dst
}

func (h *histogram) importState(src []byte) (int, error) {
	// read h.a
	itemsLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot read itemsLen")
	}
	src = src[n:]

	a := make([]string, itemsLen)
	stateSize := int(unsafe.Sizeof(a[0])) * len(a)
	for i := range a {
		value, n := encoding.UnmarshalBytes(src)
		if n <= 0 {
			return 0, fmt.Errorf("cannot read value")
		}
		src = src[n:]

		a[i] = string(value)
		stateSize += len(value)
	}
	if len(a) == 0 {
		a = nil
	}
	h.a = a

	// read h.min
	value, n := encoding.UnmarshalBytes(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot read min value")
	}
	src = src[n:]

	h.min = string(value)
	stateSize += len(value)

	// read h.max
	value, n = encoding.UnmarshalBytes(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot read max value")
	}
	src = src[n:]

	h.max = string(value)
	stateSize += len(value)

	// read h.count
	count, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot read count")
	}
	src = src[n:]

	h.count = count

	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected non-empty tail left; len(tail)=%d", len(src))
	}

	return stateSize, nil
}

func (h *histogram) update(v string) int {
	if h.count == 0 || lessString(v, h.min) {
		h.min = strings.Clone(v)
	}
	if h.count == 0 || lessString(h.max, v) {
		h.max = strings.Clone(v)
	}

	h.count++
	if len(h.a) < maxHistogramSamples {
		if len(h.a) > 0 && v == h.a[len(h.a)-1] {
			h.a = append(h.a, h.a[len(h.a)-1])
			return int(unsafe.Sizeof(v))
		}
		vCopy := strings.Clone(v)
		h.a = append(h.a, vCopy)
		return len(vCopy) + int(unsafe.Sizeof(vCopy))
	}

	if n := h.rng.Uint32n(uint32(h.count)); n < uint32(len(h.a)) {
		vPrev := h.a[n]
		if vPrev != v {
			vCopy := strings.Clone(v)
			h.a[n] = vCopy
			return len(vCopy) - len(vPrev)
		}
	}
	return 0
}

const maxHistogramSamples = 10_000

func (h *histogram) mergeState(src *histogram) {
	if src.count == 0 {
		// Nothing to merge
		return
	}
	if h.count == 0 {
		h.a = append(h.a, src.a...)
		h.min = src.min
		h.max = src.max
		h.count = src.count
		return
	}

	h.a = append(h.a, src.a...)
	if lessString(src.min, h.min) {
		h.min = src.min
	}
	if lessString(h.max, src.max) {
		h.max = src.max
	}
	h.count += src.count
}

func (h *histogram) quantile(phi float64) string {
	if len(h.a) == 0 {
		return ""
	}
	if len(h.a) == 1 {
		return h.a[0]
	}
	if phi <= 0 {
		return h.min
	}
	if phi >= 1 {
		return h.max
	}

	sort.Slice(h.a, func(i, j int) bool {
		return lessString(h.a[i], h.a[j])
	})
	idx := int(phi * float64(len(h.a)))
	if idx == len(h.a) {
		return h.max
	}
	return h.a[idx]
}
