package logstorage

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"unsafe"

	"github.com/valyala/fastrand"
)

type statsQuantile struct {
	fields       []string
	containsStar bool

	phi float64
}

func (sq *statsQuantile) String() string {
	return fmt.Sprintf("quantile(%g, %s)", sq.phi, fieldNamesString(sq.fields))
}

func (sq *statsQuantile) neededFields() []string {
	return sq.fields
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
	h := &sqp.h
	stateSizeIncrease := 0

	if sqp.sq.containsStar {
		for _, c := range br.getColumns() {
			for _, v := range c.getValues(br) {
				f, ok := tryParseFloat64(v)
				if ok {
					stateSizeIncrease += h.update(f)
				}
			}
		}
	} else {
		for _, field := range sqp.sq.fields {
			c := br.getColumnByName(field)
			for _, v := range c.getValues(br) {
				f, ok := tryParseFloat64(v)
				if ok {
					stateSizeIncrease += h.update(f)
				}
			}
		}
	}

	return stateSizeIncrease
}

func (sqp *statsQuantileProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	h := &sqp.h
	stateSizeIncrease := 0

	if sqp.sq.containsStar {
		for _, c := range br.getColumns() {
			f := c.getFloatValueAtRow(rowIdx)
			if !math.IsNaN(f) {
				stateSizeIncrease += h.update(f)
			}
		}
	} else {
		for _, field := range sqp.sq.fields {
			c := br.getColumnByName(field)
			f := c.getFloatValueAtRow(rowIdx)
			if !math.IsNaN(f) {
				stateSizeIncrease += h.update(f)
			}
		}
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
	if len(fields) < 2 {
		return nil, fmt.Errorf("'quantile' must have at least two args: phi and field name")
	}

	// Parse phi
	phi, ok := tryParseFloat64(fields[0])
	if !ok {
		return nil, fmt.Errorf("phi arg in 'quantile' must be floating point number; got %q", fields[0])
	}
	if phi < 0 || phi > 1 {
		return nil, fmt.Errorf("phi arg in 'quantile' must be in the range [0..1]; got %q", fields[0])
	}

	// Parse fields
	fields = fields[1:]
	if slices.Contains(fields, "*") {
		fields = []string{"*"}
	}

	sq := &statsQuantile{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),

		phi: phi,
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
