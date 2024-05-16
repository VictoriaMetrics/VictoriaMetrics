package logstorage

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type statsUniqValuesCount struct {
	fields       []string
	containsStar bool
	limit        uint64
}

func (su *statsUniqValuesCount) String() string {
	s := "uniq_values_count(" + fieldNamesString(su.fields) + ")"
	if su.limit > 0 {
		s += fmt.Sprintf(" limit %d", su.limit)
	}
	return s
}

func (su *statsUniqValuesCount) neededFields() []string {
	return su.fields
}

func (su *statsUniqValuesCount) newStatsProcessor() (statsProcessor, int) {
	sup := &statsUniqValuesCountProcessor{
		su: su,

		m: make(map[string]uint64),
	}
	return sup, int(unsafe.Sizeof(*sup))
}

type statsUniqValuesCountProcessor struct {
	su *statsUniqValuesCount

	m map[string]uint64
}

func (sup *statsUniqValuesCountProcessor) updateStatsForAllRows(br *blockResult) int {
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

func (sup *statsUniqValuesCountProcessor) updateStatsForAllRowsColumn(c *blockResultColumn, br *blockResult) int {
	m := sup.m
	stateSizeIncrease := 0
	if c.isConst {
		// collect unique const values
		v := strings.Clone(c.encodedValues[0])
		if v == "" {
			// skip empty values
			return stateSizeIncrease
		}
		if _, ok := m[v]; !ok {
			m[v] = 0
			stateSizeIncrease += len(v) + int(unsafe.Sizeof(v)) + 8
		}
		m[v]++
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
				m[vCopy] = 0
				stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy)) + 8
			}
			m[v]++
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
			// This value has been already stored.
			m[v]++
			continue
		}
		if _, ok := m[v]; !ok {
			vCopy := strings.Clone(v)
			m[vCopy] = 0
			stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy)) + 8
		}
		m[v]++
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesCountProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
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

func (sup *statsUniqValuesCountProcessor) updateStatsForRowColumn(c *blockResultColumn, br *blockResult, rowIdx int) int {
	m := sup.m
	stateSizeIncrease := 0
	if c.isConst {
		// collect unique const values
		v := strings.Clone(c.encodedValues[0])
		if v == "" {
			// skip empty values
			return stateSizeIncrease
		}
		if _, ok := m[v]; !ok {
			m[v] = 0
			stateSizeIncrease += len(v) + int(unsafe.Sizeof(v)) + 8
		}
		m[v]++
		return stateSizeIncrease
	}
	if c.valueType == valueTypeDict {
		// collect unique non-zero c.dictValues
		dictIdx := c.encodedValues[rowIdx][0]
		v := strings.Clone(c.dictValues[dictIdx])
		if v == "" {
			// skip empty values
			return stateSizeIncrease
		}
		if _, ok := m[v]; !ok {
			m[v] = 0
			stateSizeIncrease += len(v) + int(unsafe.Sizeof(v)) + 8
		}
		m[v]++
		return stateSizeIncrease
	}

	// collect unique values for the given rowIdx.
	v := strings.Clone(c.getValueAtRow(br, rowIdx))
	if v == "" {
		// skip empty values
		return stateSizeIncrease
	}
	if _, ok := m[v]; !ok {
		m[v] = 0
		stateSizeIncrease += len(v) + int(unsafe.Sizeof(v)) + 8
	}
	m[v]++
	return stateSizeIncrease
}

func (sup *statsUniqValuesCountProcessor) mergeState(sfp statsProcessor) {
	if sup.limitReached() {
		return
	}

	src := sfp.(*statsUniqValuesCountProcessor)
	m := sup.m
	for k, v := range src.m {
		if _, ok := m[k]; !ok {
			m[k] = 0
		}
		m[k] += v
	}
}

func (sup *statsUniqValuesCountProcessor) finalizeStats() string {
	if len(sup.m) == 0 {
		return "{}"
	}

	keys := make([]string, 0, len(sup.m))
	for k := range sup.m {
		keys = append(keys, k)
	}

	if limit := sup.su.limit; limit > 0 && uint64(len(keys)) > limit {
		keys = keys[:limit]
	}

	return marshalJSONObject(sup.m, keys)
}

func marshalJSONObject(m map[string]uint64, keys []string) string {
	// Pre-allocate buffer for serialized items.
	// Assume that there is no need in quoting items. Otherwise additional reallocations
	// for the allocated buffer are possible.
	bufSize := 2*len(keys) + 1
	for _, key := range keys {
		bufSize += len(key) + len(strconv.FormatUint(m[key], 10))
	}
	b := make([]byte, 0, bufSize)

	b = append(b, '{')
	b = strconv.AppendQuote(b, keys[0])
	b = append(b, ':')
	b = append(b, strconv.FormatUint(m[keys[0]], 10)...)
	for _, key := range keys[1:] {
		b = append(b, ',')
		b = strconv.AppendQuote(b, key)
		b = append(b, ':')
		b = append(b, strconv.FormatUint(m[key], 10)...)
	}
	b = append(b, '}')

	return bytesutil.ToUnsafeString(b)
}

func (sup *statsUniqValuesCountProcessor) limitReached() bool {
	limit := sup.su.limit
	return limit > 0 && uint64(len(sup.m)) >= limit
}

func parseStatsUniqValuesCount(lex *lexer) (*statsUniqValuesCount, error) {
	fields, err := parseFieldNamesForStatsFunc(lex, "uniq_values_count")
	if err != nil {
		return nil, err
	}
	su := &statsUniqValuesCount{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
	}
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s' for 'uniq_values_count': %w", lex.token, err)
		}
		lex.nextToken()
		su.limit = n
	}
	return su, nil
}
