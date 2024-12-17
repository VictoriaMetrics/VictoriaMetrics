package logstorage

import (
	"fmt"
	"slices"
	"strings"
	"unsafe"

	"github.com/valyala/quicktemplate"
)

type statsUniqValues struct {
	fields []string
	limit  uint64
}

func (su *statsUniqValues) String() string {
	s := "uniq_values(" + statsFuncFieldsToString(su.fields) + ")"
	if su.limit > 0 {
		s += fmt.Sprintf(" limit %d", su.limit)
	}
	return s
}

func (su *statsUniqValues) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, su.fields)
}

func (su *statsUniqValues) newStatsProcessor() (statsProcessor, int) {
	sup := &statsUniqValuesProcessor{
		su: su,

		m: make(map[string]struct{}),
	}
	return sup, int(unsafe.Sizeof(*sup))
}

type statsUniqValuesProcessor struct {
	su *statsUniqValues

	m map[string]struct{}
}

func (sup *statsUniqValuesProcessor) updateStatsForAllRows(br *blockResult) int {
	if sup.limitReached() {
		// Limit on the number of unique values has been reached
		return 0
	}

	stateSizeIncrease := 0
	fields := sup.su.fields
	if len(fields) == 0 {
		for _, c := range br.getColumns() {
			stateSizeIncrease += sup.updateStatsForAllRowsColumn(c, br)
		}
	} else {
		for _, field := range fields {
			c := br.getColumnByName(field)
			stateSizeIncrease += sup.updateStatsForAllRowsColumn(c, br)
		}
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesProcessor) updateStatsForAllRowsColumn(c *blockResultColumn, br *blockResult) int {
	if c.isConst {
		// collect unique const values
		v := c.valuesEncoded[0]
		return sup.updateState(v)
	}

	stateSizeIncrease := 0
	if c.valueType == valueTypeDict {
		// collect unique non-zero c.dictValues
		c.forEachDictValue(br, func(v string) {
			stateSizeIncrease += sup.updateState(v)
		})
		return stateSizeIncrease
	}

	// slow path - collect unique values across all rows
	values := c.getValues(br)
	for i, v := range values {
		if i > 0 && values[i-1] == v {
			// This value has been already counted.
			continue
		}
		stateSizeIncrease += sup.updateState(v)
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	if sup.limitReached() {
		// Limit on the number of unique values has been reached
		return 0
	}

	stateSizeIncrease := 0
	fields := sup.su.fields
	if len(fields) == 0 {
		for _, c := range br.getColumns() {
			stateSizeIncrease += sup.updateStatsForRowColumn(c, br, rowIdx)
		}
	} else {
		for _, field := range fields {
			c := br.getColumnByName(field)
			stateSizeIncrease += sup.updateStatsForRowColumn(c, br, rowIdx)
		}
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesProcessor) updateStatsForRowColumn(c *blockResultColumn, br *blockResult, rowIdx int) int {
	if c.isConst {
		// collect unique const values
		v := c.valuesEncoded[0]
		return sup.updateState(v)
	}

	if c.valueType == valueTypeDict {
		// collect unique non-zero c.dictValues
		valuesEncoded := c.getValuesEncoded(br)
		dictIdx := valuesEncoded[rowIdx][0]
		v := c.dictValues[dictIdx]
		return sup.updateState(v)
	}

	// collect unique values for the given rowIdx.
	v := c.getValueAtRow(br, rowIdx)
	return sup.updateState(v)
}

func (sup *statsUniqValuesProcessor) mergeState(sfp statsProcessor) {
	if sup.limitReached() {
		return
	}

	src := sfp.(*statsUniqValuesProcessor)
	for k := range src.m {
		if _, ok := sup.m[k]; !ok {
			sup.m[k] = struct{}{}
		}
	}
}

func (sup *statsUniqValuesProcessor) finalizeStats(dst []byte) []byte {
	if len(sup.m) == 0 {
		return append(dst, "[]"...)
	}

	items := make([]string, 0, len(sup.m))
	for k := range sup.m {
		items = append(items, k)
	}
	sortStrings(items)

	if limit := sup.su.limit; limit > 0 && uint64(len(items)) > limit {
		items = items[:limit]
	}

	return marshalJSONArray(dst, items)
}

func sortStrings(a []string) {
	slices.SortFunc(a, func(x, y string) int {
		if x == y {
			return 0
		}
		if lessString(x, y) {
			return -1
		}
		return 1
	})
}

func (sup *statsUniqValuesProcessor) updateState(v string) int {
	if v == "" {
		// Skip empty values
		return 0
	}
	if _, ok := sup.m[v]; ok {
		return 0
	}
	vCopy := strings.Clone(v)
	sup.m[vCopy] = struct{}{}
	return len(vCopy) + int(unsafe.Sizeof(vCopy))
}

func (sup *statsUniqValuesProcessor) limitReached() bool {
	limit := sup.su.limit
	return limit > 0 && uint64(len(sup.m)) > limit
}

func marshalJSONArray(dst []byte, items []string) []byte {
	dst = append(dst, '[')
	dst = quicktemplate.AppendJSONString(dst, items[0], true)
	for _, item := range items[1:] {
		dst = append(dst, ',')
		dst = quicktemplate.AppendJSONString(dst, item, true)
	}
	dst = append(dst, ']')
	return dst
}

func parseStatsUniqValues(lex *lexer) (*statsUniqValues, error) {
	fields, err := parseStatsFuncFields(lex, "uniq_values")
	if err != nil {
		return nil, err
	}
	su := &statsUniqValues{
		fields: fields,
	}
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s' for 'uniq_values': %w", lex.token, err)
		}
		lex.nextToken()
		su.limit = n
	}
	return su, nil
}
