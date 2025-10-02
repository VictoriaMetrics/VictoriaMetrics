package logstorage

import (
	"fmt"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsJSONValues struct {
	fieldFilters []string
	limit        uint64
}

func (sv *statsJSONValues) String() string {
	s := "json_values(" + fieldNamesString(sv.fieldFilters) + ")"
	if sv.limit > 0 {
		s += fmt.Sprintf(" limit %d", sv.limit)
	}
	return s
}

func (sv *statsJSONValues) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sv.fieldFilters)
}

func (sv *statsJSONValues) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	svp := a.newStatsJSONValuesProcessor()
	svp.a = a
	return svp
}

type statsJSONValuesProcessor struct {
	values []string

	a *chunkedAllocator

	fieldsBuf []Field
	buf       []byte
}

func (svp *statsJSONValuesProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sv := sf.(*statsJSONValues)
	if svp.limitReached(sv) {
		// Limit on the number of unique values has been reached
		return 0
	}

	stateSizeIncrease := 0

	mc := getMatchingColumns(br, sv.fieldFilters)
	mc.sort()
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		stateSizeIncrease += svp.updateStateForRow(br, mc.cs, rowIdx)
	}
	putMatchingColumns(mc)

	return stateSizeIncrease
}

func (svp *statsJSONValuesProcessor) updateStateForRow(br *blockResult, cs []*blockResultColumn, rowIdx int) int {
	bytesAllocated := svp.a.bytesAllocated
	fieldsBuf := svp.fieldsBuf[:0]
	for _, c := range cs {
		fieldsBuf = append(fieldsBuf, Field{
			Name:  c.name,
			Value: c.getValueAtRow(br, rowIdx),
		})
	}
	svp.buf = MarshalFieldsToJSON(svp.buf[:0], fieldsBuf)
	value := svp.a.cloneBytesToString(svp.buf)
	svp.values = append(svp.values, value)
	return (svp.a.bytesAllocated - bytesAllocated) + int(unsafe.Sizeof(value))
}

func (svp *statsJSONValuesProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sv := sf.(*statsJSONValues)
	if svp.limitReached(sv) {
		// Limit on the number of unique values has been reached
		return 0
	}

	mc := getMatchingColumns(br, sv.fieldFilters)
	mc.sort()
	stateSizeIncrease := svp.updateStateForRow(br, mc.cs, rowIdx)
	putMatchingColumns(mc)

	return stateSizeIncrease
}

func (svp *statsJSONValuesProcessor) mergeState(_ *chunkedAllocator, sf statsFunc, sfp statsProcessor) {
	sv := sf.(*statsJSONValues)
	if svp.limitReached(sv) {
		return
	}

	src := sfp.(*statsJSONValuesProcessor)
	svp.values = append(svp.values, src.values...)
}

func (svp *statsJSONValuesProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	dst = encoding.MarshalVarUint64(dst, uint64(len(svp.values)))
	for _, v := range svp.values {
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(v))
	}
	return dst
}

func (svp *statsJSONValuesProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	valuesLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot unmarshal valuesLen")
	}
	src = src[n:]

	values := make([]string, valuesLen)
	stateSizeIncrease := int(unsafe.Sizeof(values[0])) * len(values)
	for i := range values {
		v, n := encoding.UnmarshalBytes(src)
		if n <= 0 {
			return 0, fmt.Errorf("cannot unmarshal value")
		}
		src = src[n:]

		values[i] = svp.a.cloneBytesToString(v)

		stateSizeIncrease += len(v)
	}
	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected tail left after unmarshaling values; len(tail)=%d", len(src))
	}

	if len(values) == 0 {
		values = nil
	}
	svp.values = values

	return stateSizeIncrease, nil
}

func (svp *statsJSONValuesProcessor) finalizeStats(sf statsFunc, dst []byte, _ <-chan struct{}) []byte {
	sv := sf.(*statsJSONValues)
	items := svp.values
	if len(items) == 0 {
		return append(dst, "[]"...)
	}

	if limit := sv.limit; limit > 0 && uint64(len(items)) > limit {
		items = items[:limit]
	}

	return marshalJSONArray(dst, items)
}

func (svp *statsJSONValuesProcessor) limitReached(sv *statsJSONValues) bool {
	limit := sv.limit
	return limit > 0 && uint64(len(svp.values)) > limit
}

func parseStatsJSONValues(lex *lexer) (*statsJSONValues, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "json_values")
	if err != nil {
		return nil, err
	}
	sv := &statsJSONValues{
		fieldFilters: fieldFilters,
	}
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s' for 'json_values': %w", lex.token, err)
		}
		lex.nextToken()
		sv.limit = n
	}
	return sv, nil
}
