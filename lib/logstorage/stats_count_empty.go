package logstorage

import (
	"slices"
	"strconv"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type statsCountEmpty struct {
	fields []string
}

func (sc *statsCountEmpty) String() string {
	return "count_empty(" + statsFuncFieldsToString(sc.fields) + ")"
}

func (sc *statsCountEmpty) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, sc.fields)
}

func (sc *statsCountEmpty) newStatsProcessor() (statsProcessor, int) {
	scp := &statsCountEmptyProcessor{
		sc: sc,
	}
	return scp, int(unsafe.Sizeof(*scp))
}

type statsCountEmptyProcessor struct {
	sc *statsCountEmpty

	rowsCount uint64
}

func (scp *statsCountEmptyProcessor) updateStatsForAllRows(br *blockResult) int {
	fields := scp.sc.fields
	if len(fields) == 0 {
		bm := getBitmap(br.rowsLen)
		bm.setBits()
		for _, c := range br.getColumns() {
			values := c.getValues(br)
			bm.forEachSetBit(func(idx int) bool {
				return values[idx] == ""
			})
		}
		scp.rowsCount += uint64(bm.onesCount())
		putBitmap(bm)
		return 0
	}
	if len(fields) == 1 {
		// Fast path for count_empty(single_column)
		c := br.getColumnByName(fields[0])
		if c.isConst {
			if c.valuesEncoded[0] == "" {
				scp.rowsCount += uint64(br.rowsLen)
			}
			return 0
		}
		if c.isTime {
			return 0
		}
		switch c.valueType {
		case valueTypeString:
			for _, v := range c.getValuesEncoded(br) {
				if v == "" {
					scp.rowsCount++
				}
			}
			return 0
		case valueTypeDict:
			zeroDictIdx := slices.Index(c.dictValues, "")
			if zeroDictIdx < 0 {
				return 0
			}
			for _, v := range c.getValuesEncoded(br) {
				if int(v[0]) == zeroDictIdx {
					scp.rowsCount++
				}
			}
			return 0
		case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64, valueTypeFloat64, valueTypeIPv4, valueTypeTimestampISO8601:
			return 0
		default:
			logger.Panicf("BUG: unknown valueType=%d", c.valueType)
			return 0
		}
	}

	// Slow path - count rows containing empty value for all the fields enumerated inside count_empty().
	bm := getBitmap(br.rowsLen)
	defer putBitmap(bm)

	bm.setBits()
	for _, f := range fields {
		c := br.getColumnByName(f)
		if c.isConst {
			if c.valuesEncoded[0] != "" {
				return 0
			}
			continue
		}
		if c.isTime {
			return 0
		}
		switch c.valueType {
		case valueTypeString:
			valuesEncoded := c.getValuesEncoded(br)
			bm.forEachSetBit(func(i int) bool {
				return valuesEncoded[i] == ""
			})
		case valueTypeDict:
			if !slices.Contains(c.dictValues, "") {
				return 0
			}
			valuesEncoded := c.getValuesEncoded(br)
			bm.forEachSetBit(func(i int) bool {
				dictIdx := valuesEncoded[i][0]
				return c.dictValues[dictIdx] == ""
			})
		case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64, valueTypeFloat64, valueTypeIPv4, valueTypeTimestampISO8601:
			return 0
		default:
			logger.Panicf("BUG: unknown valueType=%d", c.valueType)
			return 0
		}
	}

	scp.rowsCount += uint64(bm.onesCount())
	return 0
}

func (scp *statsCountEmptyProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	fields := scp.sc.fields
	if len(fields) == 0 {
		for _, c := range br.getColumns() {
			if v := c.getValueAtRow(br, rowIdx); v != "" {
				return 0
			}
		}
		scp.rowsCount++
		return 0
	}
	if len(fields) == 1 {
		// Fast path for count_empty(single_column)
		c := br.getColumnByName(fields[0])
		if c.isConst {
			if c.valuesEncoded[0] == "" {
				scp.rowsCount++
			}
			return 0
		}
		if c.isTime {
			return 0
		}
		switch c.valueType {
		case valueTypeString:
			valuesEncoded := c.getValuesEncoded(br)
			if v := valuesEncoded[rowIdx]; v == "" {
				scp.rowsCount++
			}
			return 0
		case valueTypeDict:
			valuesEncoded := c.getValuesEncoded(br)
			dictIdx := valuesEncoded[rowIdx][0]
			if v := c.dictValues[dictIdx]; v == "" {
				scp.rowsCount++
			}
			return 0
		case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64, valueTypeFloat64, valueTypeIPv4, valueTypeTimestampISO8601:
			return 0
		default:
			logger.Panicf("BUG: unknown valueType=%d", c.valueType)
			return 0
		}
	}

	// Slow path - count the row at rowIdx if at least a single field enumerated inside count() is non-empty
	for _, f := range fields {
		c := br.getColumnByName(f)
		if v := c.getValueAtRow(br, rowIdx); v != "" {
			return 0
		}
	}
	scp.rowsCount++
	return 0
}

func (scp *statsCountEmptyProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsCountEmptyProcessor)
	scp.rowsCount += src.rowsCount
}

func (scp *statsCountEmptyProcessor) finalizeStats() string {
	return strconv.FormatUint(scp.rowsCount, 10)
}

func parseStatsCountEmpty(lex *lexer) (*statsCountEmpty, error) {
	fields, err := parseStatsFuncFields(lex, "count_empty")
	if err != nil {
		return nil, err
	}
	sc := &statsCountEmpty{
		fields: fields,
	}
	return sc, nil
}
