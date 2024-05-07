package logstorage

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type statsUniqValues struct {
	fields       []string
	containsStar bool
}

func (su *statsUniqValues) String() string {
	return "uniq_values(" + fieldNamesString(su.fields) + ")"
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
	stateSizeIncrease := 0
	if sup.su.containsStar {
		columns := br.getColumns()
		for i := range columns {
			stateSizeIncrease += sup.updateStatsForAllRowsColumn(&columns[i], br)
		}
	} else {
		for _, field := range sup.su.fields {
			c := br.getColumnByName(field)
			stateSizeIncrease += sup.updateStatsForAllRowsColumn(&c, br)
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
	stateSizeIncrease := 0
	if sup.su.containsStar {
		columns := br.getColumns()
		for i := range columns {
			stateSizeIncrease += sup.updateStatsForRowColumn(&columns[i], br, rowIdx)
		}
	} else {
		for _, field := range sup.su.fields {
			c := br.getColumnByName(field)
			stateSizeIncrease += sup.updateStatsForRowColumn(&c, br, rowIdx)
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
	sort.Strings(items)

	// Marshal items into JSON array.

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

func parseStatsUniqValues(lex *lexer) (*statsUniqValues, error) {
	fields, err := parseFieldNamesForStatsFunc(lex, "uniq_values")
	if err != nil {
		return nil, err
	}
	su := &statsUniqValues{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
	}
	return su, nil
}
