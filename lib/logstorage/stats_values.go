package logstorage

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

type statsValues struct {
	fieldFilters []string
	limit        uint64
}

func (sv *statsValues) String() string {
	s := "values(" + fieldNamesString(sv.fieldFilters) + ")"
	if sv.limit > 0 {
		s += fmt.Sprintf(" limit %d", sv.limit)
	}
	return s
}

func (sv *statsValues) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sv.fieldFilters)
}

func (sv *statsValues) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsValuesProcessor()
}

type statsValuesProcessor struct {
	values []string
}

func (svp *statsValuesProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sv := sf.(*statsValues)
	if svp.limitReached(sv) {
		// Limit on the number of unique values has been reached
		return 0
	}

	stateSizeIncrease := 0

	mc := getMatchingColumns(br, sv.fieldFilters)
	for _, c := range mc.cs {
		stateSizeIncrease += svp.updateStatsForAllRowsColumn(c, br)
	}
	putMatchingColumns(mc)

	return stateSizeIncrease
}

func (svp *statsValuesProcessor) updateStatsForAllRowsColumn(c *blockResultColumn, br *blockResult) int {
	stateSizeIncrease := 0
	if c.isConst {
		v := strings.Clone(c.valuesEncoded[0])
		stateSizeIncrease += len(v)

		values := svp.values
		for i := 0; i < br.rowsLen; i++ {
			values = append(values, v)
		}
		svp.values = values

		stateSizeIncrease += br.rowsLen * int(unsafe.Sizeof(values[0]))
		return stateSizeIncrease
	}
	if c.valueType == valueTypeDict {
		dictValues := make([]string, len(c.dictValues))
		for i, v := range c.dictValues {
			dictValues[i] = strings.Clone(v)
			stateSizeIncrease += len(v)
		}

		values := svp.values
		for _, encodedValue := range c.getValuesEncoded(br) {
			idx := encodedValue[0]
			values = append(values, dictValues[idx])
		}
		svp.values = values

		stateSizeIncrease += br.rowsLen * int(unsafe.Sizeof(values[0]))
		return stateSizeIncrease
	}

	values := svp.values
	vPrev := ""
	for _, v := range c.getValues(br) {
		if len(values) == 0 || v != vPrev {
			vPrev = strings.Clone(v)
			stateSizeIncrease += len(vPrev)
		}
		values = append(values, vPrev)
	}
	svp.values = values

	stateSizeIncrease += br.rowsLen * int(unsafe.Sizeof(values[0]))
	return stateSizeIncrease
}

func (svp *statsValuesProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sv := sf.(*statsValues)
	if svp.limitReached(sv) {
		// Limit on the number of unique values has been reached
		return 0
	}

	stateSizeIncrease := 0

	mc := getMatchingColumns(br, sv.fieldFilters)
	for _, c := range mc.cs {
		stateSizeIncrease += svp.updateStatsForRowColumn(c, br, rowIdx)
	}
	putMatchingColumns(mc)

	return stateSizeIncrease
}

func (svp *statsValuesProcessor) updateStatsForRowColumn(c *blockResultColumn, br *blockResult, rowIdx int) int {
	stateSizeIncrease := 0
	if c.isConst {
		v := strings.Clone(c.valuesEncoded[0])
		stateSizeIncrease += len(v)

		svp.values = append(svp.values, v)
		stateSizeIncrease += int(unsafe.Sizeof(svp.values[0]))

		return stateSizeIncrease
	}
	if c.valueType == valueTypeDict {
		// collect unique non-zero c.dictValues
		valuesEncoded := c.getValuesEncoded(br)
		dictIdx := valuesEncoded[rowIdx][0]
		v := strings.Clone(c.dictValues[dictIdx])
		stateSizeIncrease += len(v)

		svp.values = append(svp.values, v)
		stateSizeIncrease += int(unsafe.Sizeof(svp.values[0]))

		return stateSizeIncrease
	}

	// collect unique values for the given rowIdx.
	v := c.getValueAtRow(br, rowIdx)
	v = strings.Clone(v)
	stateSizeIncrease += len(v)

	svp.values = append(svp.values, v)
	stateSizeIncrease += int(unsafe.Sizeof(svp.values[0]))

	return stateSizeIncrease
}

func (svp *statsValuesProcessor) mergeState(_ *chunkedAllocator, sf statsFunc, sfp statsProcessor) {
	sv := sf.(*statsValues)
	if svp.limitReached(sv) {
		return
	}

	src := sfp.(*statsValuesProcessor)
	svp.values = append(svp.values, src.values...)
}

func (svp *statsValuesProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	dst = encoding.MarshalVarUint64(dst, uint64(len(svp.values)))
	for _, v := range svp.values {
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(v))
	}
	return dst
}

func (svp *statsValuesProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	valuesLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot unmarshal valuesLen")
	}
	src = src[n:]

	values := make([]string, valuesLen)
	stateSize := int(unsafe.Sizeof(values[0])) * len(values)
	for i := range values {
		v, n := encoding.UnmarshalBytes(src)
		if n <= 0 {
			return 0, fmt.Errorf("cannot unmarshal value")
		}
		src = src[n:]

		values[i] = string(v)
		stateSize += len(v)
	}
	if len(values) == 0 {
		values = nil
	}
	svp.values = values

	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected non-empty tail left; len(tail)=%d", len(src))
	}

	return stateSize, nil
}

func (svp *statsValuesProcessor) finalizeStats(sf statsFunc, dst []byte, _ <-chan struct{}) []byte {
	sv := sf.(*statsValues)
	items := svp.values
	if len(items) == 0 {
		return append(dst, "[]"...)
	}

	if limit := sv.limit; limit > 0 && uint64(len(items)) > limit {
		items = items[:limit]
	}

	return marshalJSONArray(dst, items)
}

func (svp *statsValuesProcessor) limitReached(sv *statsValues) bool {
	limit := sv.limit
	return limit > 0 && uint64(len(svp.values)) > limit
}

func parseStatsValues(lex *lexer) (*statsValues, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "values")
	if err != nil {
		return nil, err
	}
	sv := &statsValues{
		fieldFilters: fieldFilters,
	}
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s' for 'values': %w", lex.token, err)
		}
		lex.nextToken()
		sv.limit = n
	}
	return sv, nil
}
