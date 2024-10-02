package logstorage

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"unsafe"

	"github.com/valyala/fastrand"

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

func (sq *statsQuantile) newStatsProcessor() (statsProcessor, int) {
	sqp := &statsQuantileProcessor{
		sq: sq,
	}
	return sqp, int(unsafe.Sizeof(*sqp))
}

type statsQuantileProcessor struct {
	sq *statsQuantile

	h histogram
}

func (sqp *statsQuantileProcessor) updateStatsForAllRows(br *blockResult) int {
	stateSizeIncrease := 0

	fields := sqp.sq.fields
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

func (sqp *statsQuantileProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	h := &sqp.h
	stateSizeIncrease := 0

	fields := sqp.sq.fields
	if len(fields) == 0 {
		for _, c := range br.getColumns() {
			f, ok := c.getFloatValueAtRow(br, rowIdx)
			if ok {
				stateSizeIncrease += h.update(f)
			}
		}
	} else {
		for _, field := range fields {
			c := br.getColumnByName(field)
			f, ok := c.getFloatValueAtRow(br, rowIdx)
			if ok {
				stateSizeIncrease += h.update(f)
			}
		}
	}

	return stateSizeIncrease
}

func (sqp *statsQuantileProcessor) updateStateForColumn(br *blockResult, c *blockResultColumn) int {
	h := &sqp.h
	stateSizeIncrease := 0

	if c.isConst {
		f, ok := tryParseFloat64(c.valuesEncoded[0])
		if ok {
			for i := 0; i < br.rowsLen; i++ {
				stateSizeIncrease += h.update(f)
			}
		}
		return stateSizeIncrease
	}
	if c.isTime {
		return 0
	}

	switch c.valueType {
	case valueTypeString:
		for _, v := range c.getValues(br) {
			f, ok := tryParseFloat64(v)
			if ok {
				stateSizeIncrease += h.update(f)
			}
		}
	case valueTypeDict:
		dictValues := c.dictValues
		a := encoding.GetFloat64s(len(dictValues))
		for i, v := range dictValues {
			f, ok := tryParseFloat64(v)
			if !ok {
				f = nan
			}
			a.A[i] = f
		}
		for _, v := range c.getValuesEncoded(br) {
			idx := v[0]
			f := a.A[idx]
			if !math.IsNaN(f) {
				h.update(f)
			}
		}
		encoding.PutFloat64s(a)
	case valueTypeUint8:
		for _, v := range c.getValuesEncoded(br) {
			n := unmarshalUint8(v)
			h.update(float64(n))
		}
	case valueTypeUint16:
		for _, v := range c.getValuesEncoded(br) {
			n := unmarshalUint16(v)
			h.update(float64(n))
		}
	case valueTypeUint32:
		for _, v := range c.getValuesEncoded(br) {
			n := unmarshalUint32(v)
			h.update(float64(n))
		}
	case valueTypeUint64:
		for _, v := range c.getValuesEncoded(br) {
			n := unmarshalUint64(v)
			h.update(float64(n))
		}
	case valueTypeFloat64:
		for _, v := range c.getValuesEncoded(br) {
			f := unmarshalFloat64(v)
			if !math.IsNaN(f) {
				h.update(f)
			}
		}
	case valueTypeIPv4:
	case valueTypeTimestampISO8601:
	default:
		logger.Panicf("BUG: unexpected valueType=%d", c.valueType)
	}

	return stateSizeIncrease
}

func (sqp *statsQuantileProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsQuantileProcessor)
	sqp.h.mergeState(&src.h)
}

func (sqp *statsQuantileProcessor) finalizeStats() string {
	q := sqp.h.quantile(sqp.sq.phi)
	return strconv.FormatFloat(q, 'f', -1, 64)
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
	a     []float64
	min   float64
	max   float64
	count uint64

	rng fastrand.RNG
}

func (h *histogram) update(f float64) int {
	if h.count == 0 || f < h.min {
		h.min = f
	}
	if h.count == 0 || f > h.max {
		h.max = f
	}

	h.count++
	if len(h.a) < maxHistogramSamples {
		h.a = append(h.a, f)
		return int(unsafe.Sizeof(f))
	}

	if n := h.rng.Uint32n(uint32(h.count)); n < uint32(len(h.a)) {
		h.a[n] = f
	}
	return 0
}

const maxHistogramSamples = 100_000

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
	if src.min < h.min {
		h.min = src.min
	}
	if src.max > h.max {
		h.max = src.max
	}
	h.count += src.count
}

func (h *histogram) quantile(phi float64) float64 {
	if len(h.a) == 0 {
		return nan
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

	slices.Sort(h.a)
	idx := int(phi * float64(len(h.a)))
	if idx == len(h.a) {
		return h.max
	}
	return h.a[idx]
}
