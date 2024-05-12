package logstorage

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type statsUniqValues struct {
	fields       []string
	containsStar bool
	limit        uint64
}

func (su *statsUniqValues) String() string {
	s := "uniq_values(" + fieldNamesString(su.fields) + ")"
	if su.limit > 0 {
		s += fmt.Sprintf(" limit %d", su.limit)
	}
	return s
}

func (su *statsUniqValues) neededFields() []string {
	return su.fields
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
	if sup.su.containsStar {
		for _, c := range br.getColumns() {
			stateSizeIncrease += sup.updateStatsForAllRowsColumn(c, br)
		}
	} else {
		for _, field := range sup.su.fields {
			c := br.getColumnByName(field)
			stateSizeIncrease += sup.updateStatsForAllRowsColumn(c, br)
		}
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesProcessor) updateStatsForAllRowsColumn(c *blockResultColumn, br *blockResult) int {
	m := sup.m
	stateSizeIncrease := 0
	if c.isConst {
		// collect unique const values
		v := c.encodedValues[0]
		if v == "" {
			// skip empty values
			return stateSizeIncrease
		}
		if _, ok := m[v]; !ok {
			vCopy := strings.Clone(v)
			m[vCopy] = struct{}{}
			stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
		}
		return stateSizeIncrease
	}
	if c.valueType == valueTypeDict {
		// collect unique non-zero c.dictValues
		for _, v := range c.dictValues {
			if v == "" {
				// skip empty values
				continue
			}
			if _, ok := m[v]; !ok {
				vCopy := strings.Clone(v)
				m[vCopy] = struct{}{}
				stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
			}
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
		if _, ok := m[v]; !ok {
			vCopy := strings.Clone(v)
			m[vCopy] = struct{}{}
			stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
		}
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	if sup.limitReached() {
		// Limit on the number of unique values has been reached
		return 0
	}

	stateSizeIncrease := 0
	if sup.su.containsStar {
		for _, c := range br.getColumns() {
			stateSizeIncrease += sup.updateStatsForRowColumn(c, br, rowIdx)
		}
	} else {
		for _, field := range sup.su.fields {
			c := br.getColumnByName(field)
			stateSizeIncrease += sup.updateStatsForRowColumn(c, br, rowIdx)
		}
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesProcessor) updateStatsForRowColumn(c *blockResultColumn, br *blockResult, rowIdx int) int {
	m := sup.m
	stateSizeIncrease := 0
	if c.isConst {
		// collect unique const values
		v := c.encodedValues[0]
		if v == "" {
			// skip empty values
			return stateSizeIncrease
		}
		if _, ok := m[v]; !ok {
			vCopy := strings.Clone(v)
			m[vCopy] = struct{}{}
			stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
		}
		return stateSizeIncrease
	}
	if c.valueType == valueTypeDict {
		// collect unique non-zero c.dictValues
		dictIdx := c.encodedValues[rowIdx][0]
		v := c.dictValues[dictIdx]
		if v == "" {
			// skip empty values
			return stateSizeIncrease
		}
		if _, ok := m[v]; !ok {
			vCopy := strings.Clone(v)
			m[vCopy] = struct{}{}
			stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
		}
		return stateSizeIncrease
	}

	// collect unique values for the given rowIdx.
	v := c.getValueAtRow(br, rowIdx)
	if v == "" {
		// skip empty values
		return stateSizeIncrease
	}
	if _, ok := m[v]; !ok {
		vCopy := strings.Clone(v)
		m[vCopy] = struct{}{}
		stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesProcessor) mergeState(sfp statsProcessor) {
	if sup.limitReached() {
		return
	}

	src := sfp.(*statsUniqValuesProcessor)
	m := sup.m
	for k := range src.m {
		if _, ok := m[k]; !ok {
			m[k] = struct{}{}
		}
	}
}

func (sup *statsUniqValuesProcessor) finalizeStats() string {
	if len(sup.m) == 0 {
		return "[]"
	}

	// Sort unique items
	items := make([]string, 0, len(sup.m))
	for k := range sup.m {
		items = append(items, k)
	}
	slices.SortFunc(items, compareValues)

	if limit := sup.su.limit; limit > 0 && uint64(len(items)) > limit {
		items = items[:limit]
	}

	return marshalJSONArray(items)
}

func (sup *statsUniqValuesProcessor) limitReached() bool {
	limit := sup.su.limit
	return limit > 0 && uint64(len(sup.m)) >= limit
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
	b = strconv.AppendQuote(b, items[0])
	for _, item := range items[1:] {
		b = append(b, ',')
		b = strconv.AppendQuote(b, item)
	}
	b = append(b, ']')

	return bytesutil.ToUnsafeString(b)
}

func compareValues(a, b string) int {
	fA, okA := tryParseFloat64(a)
	fB, okB := tryParseFloat64(b)
	if okA && okB {
		if fA == fB {
			return 0
		}
		if fA < fB {
			return -1
		}
		return 1
	}
	if okA {
		return -1
	}
	if okB {
		return 1
	}
	return strings.Compare(a, b)
}

func parseStatsUniqValues(lex *lexer) (*statsUniqValues, error) {
	fields, err := parseFieldNamesForStatsFunc(lex, "uniq_values")
	if err != nil {
		return nil, err
	}
	su := &statsUniqValues{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
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
