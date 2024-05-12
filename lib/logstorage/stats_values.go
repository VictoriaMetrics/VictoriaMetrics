package logstorage

import (
	"fmt"
	"slices"
	"strings"
	"unsafe"
)

type statsValues struct {
	fields       []string
	containsStar bool
	limit        uint64
}

func (sv *statsValues) String() string {
	s := "values(" + fieldNamesString(sv.fields) + ")"
	if sv.limit > 0 {
		s += fmt.Sprintf(" limit %d", sv.limit)
	}
	return s
}

func (sv *statsValues) neededFields() []string {
	return sv.fields
}

func (sv *statsValues) newStatsProcessor() (statsProcessor, int) {
	svp := &statsValuesProcessor{
		sv: sv,
	}
	return svp, int(unsafe.Sizeof(*svp))
}

type statsValuesProcessor struct {
	sv *statsValues

	values []string
}

func (svp *statsValuesProcessor) updateStatsForAllRows(br *blockResult) int {
	if svp.limitReached() {
		// Limit on the number of unique values has been reached
		return 0
	}

	stateSizeIncrease := 0
	if svp.sv.containsStar {
		for _, c := range br.getColumns() {
			stateSizeIncrease += svp.updateStatsForAllRowsColumn(c, br)
		}
	} else {
		for _, field := range svp.sv.fields {
			c := br.getColumnByName(field)
			stateSizeIncrease += svp.updateStatsForAllRowsColumn(c, br)
		}
	}
	return stateSizeIncrease
}

func (svp *statsValuesProcessor) updateStatsForAllRowsColumn(c *blockResultColumn, br *blockResult) int {
	stateSizeIncrease := 0
	if c.isConst {
		v := strings.Clone(c.encodedValues[0])
		stateSizeIncrease += len(v)

		values := svp.values
		for range br.timestamps {
			values = append(values, v)
		}
		svp.values = values

		stateSizeIncrease += len(br.timestamps) * int(unsafe.Sizeof(values[0]))
		return stateSizeIncrease
	}
	if c.valueType == valueTypeDict {
		dictValues := make([]string, len(c.dictValues))
		for i, v := range c.dictValues {
			dictValues[i] = strings.Clone(v)
			stateSizeIncrease += len(v)
		}

		values := svp.values
		for _, encodedValue := range c.encodedValues {
			idx := encodedValue[0]
			values = append(values, dictValues[idx])
		}
		svp.values = values

		stateSizeIncrease += len(br.timestamps) * int(unsafe.Sizeof(values[0]))
		return stateSizeIncrease
	}

	values := svp.values
	for _, v := range c.getValues(br) {
		if len(values) == 0 || values[len(values)-1] != v {
			v = strings.Clone(v)
			stateSizeIncrease += len(v)
		}
		values = append(values, v)
	}
	svp.values = values

	stateSizeIncrease += len(br.timestamps) * int(unsafe.Sizeof(values[0]))
	return stateSizeIncrease
}

func (svp *statsValuesProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	if svp.limitReached() {
		// Limit on the number of unique values has been reached
		return 0
	}

	stateSizeIncrease := 0
	if svp.sv.containsStar {
		for _, c := range br.getColumns() {
			stateSizeIncrease += svp.updateStatsForRowColumn(c, br, rowIdx)
		}
	} else {
		for _, field := range svp.sv.fields {
			c := br.getColumnByName(field)
			stateSizeIncrease += svp.updateStatsForRowColumn(c, br, rowIdx)
		}
	}
	return stateSizeIncrease
}

func (svp *statsValuesProcessor) updateStatsForRowColumn(c *blockResultColumn, br *blockResult, rowIdx int) int {
	stateSizeIncrease := 0
	if c.isConst {
		v := strings.Clone(c.encodedValues[0])
		stateSizeIncrease += len(v)

		svp.values = append(svp.values, v)
		stateSizeIncrease += int(unsafe.Sizeof(svp.values[0]))

		return stateSizeIncrease
	}
	if c.valueType == valueTypeDict {
		// collect unique non-zero c.dictValues
		dictIdx := c.encodedValues[rowIdx][0]
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

func (svp *statsValuesProcessor) mergeState(sfp statsProcessor) {
	if svp.limitReached() {
		return
	}

	src := sfp.(*statsValuesProcessor)
	svp.values = append(svp.values, src.values...)
}

func (svp *statsValuesProcessor) finalizeStats() string {
	items := svp.values
	if len(items) == 0 {
		return "[]"
	}

	if limit := svp.sv.limit; limit > 0 && uint64(len(items)) > limit {
		items = items[:limit]
	}

	return marshalJSONArray(items)
}

func (svp *statsValuesProcessor) limitReached() bool {
	limit := svp.sv.limit
	return limit > 0 && uint64(len(svp.values)) >= limit
}

func parseStatsValues(lex *lexer) (*statsValues, error) {
	fields, err := parseFieldNamesForStatsFunc(lex, "values")
	if err != nil {
		return nil, err
	}
	sv := &statsValues{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
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
