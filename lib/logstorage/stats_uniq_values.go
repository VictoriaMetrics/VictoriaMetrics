package logstorage

import (
	"fmt"
	"slices"
	"strings"
	"unsafe"

	"github.com/valyala/quicktemplate"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
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
	stateSizeIncrease := 0
	if c.isConst {
		// collect unique const values
		v := c.valuesEncoded[0]
		if v == "" {
			// skip empty values
			return stateSizeIncrease
		}
		stateSizeIncrease += sup.updateState(v)
		return stateSizeIncrease
	}
	if c.valueType == valueTypeDict {
		// collect unique non-zero c.dictValues
		for _, v := range c.dictValues {
			if v == "" {
				// skip empty values
				continue
			}
			stateSizeIncrease += sup.updateState(v)
		}
		return stateSizeIncrease
	}

	// slow path - collect unique values across all rows
	values := c.getValues(br)
	for i, v := range values {
		if v == "" {
			// skip empty values
			continue
		}
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
	stateSizeIncrease := 0
	if c.isConst {
		// collect unique const values
		v := c.valuesEncoded[0]
		if v == "" {
			// skip empty values
			return stateSizeIncrease
		}
		stateSizeIncrease += sup.updateState(v)
		return stateSizeIncrease
	}
	if c.valueType == valueTypeDict {
		// collect unique non-zero c.dictValues
		valuesEncoded := c.getValuesEncoded(br)
		dictIdx := valuesEncoded[rowIdx][0]
		v := c.dictValues[dictIdx]
		if v == "" {
			// skip empty values
			return stateSizeIncrease
		}
		stateSizeIncrease += sup.updateState(v)
		return stateSizeIncrease
	}

	// collect unique values for the given rowIdx.
	v := c.getValueAtRow(br, rowIdx)
	if v == "" {
		// skip empty values
		return stateSizeIncrease
	}
	stateSizeIncrease += sup.updateState(v)
	return stateSizeIncrease
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

func (sup *statsUniqValuesProcessor) finalizeStats() string {
	if len(sup.m) == 0 {
		return "[]"
	}

	items := make([]string, 0, len(sup.m))
	for k := range sup.m {
		items = append(items, k)
	}
	sortStrings(items)

	if limit := sup.su.limit; limit > 0 && uint64(len(items)) > limit {
		items = items[:limit]
	}

	return marshalJSONArray(items)
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
	stateSizeIncrease := 0
	if _, ok := sup.m[v]; !ok {
		vCopy := strings.Clone(v)
		sup.m[vCopy] = struct{}{}
		stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesProcessor) limitReached() bool {
	limit := sup.su.limit
	return limit > 0 && uint64(len(sup.m)) > limit
}

func marshalJSONArray(items []string) string {
	// Pre-allocate buffer for serialized items.
	// Assume that there is no need in quoting items. Otherwise additional reallocations
	// for the allocated buffer are possible.
	bufSize := len(items) + 1
	for _, item := range items {
		bufSize += len(item)
	}
	b := make([]byte, 0, bufSize)

	b = append(b, '[')
	b = quicktemplate.AppendJSONString(b, items[0], true)
	for _, item := range items[1:] {
		b = append(b, ',')
		b = quicktemplate.AppendJSONString(b, item, true)
	}
	b = append(b, ']')

	return bytesutil.ToUnsafeString(b)
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
