package logstorage

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsCountEmpty struct {
	fieldFilters []string
}

func (sc *statsCountEmpty) String() string {
	return "count_empty(" + fieldNamesString(sc.fieldFilters) + ")"
}

func (sc *statsCountEmpty) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sc.fieldFilters)
}

func (sc *statsCountEmpty) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsCountEmptyProcessor()
}

type statsCountEmptyProcessor struct {
	rowsCount uint64
}

func (scp *statsCountEmptyProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sc := sf.(*statsCountEmpty)

	if isSingleField(sc.fieldFilters) {
		// Fast path for count_empty(single_column)
		c := br.getColumnByName(sc.fieldFilters[0])
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
		case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64, valueTypeInt64,
			valueTypeFloat64, valueTypeIPv4, valueTypeTimestampISO8601:
		default:
			logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		}
		return 0
	}

	// Slow path - count rows containing empty value for all the fields enumerated inside count_empty().

	bm := getBitmap(br.rowsLen)
	bm.setBits()
	defer putBitmap(bm)

	cs := br.getColumns()
	for _, c := range cs {
		if !prefixfilter.MatchFilters(sc.fieldFilters, c.name) {
			continue
		}

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
			zeroDictIdx := slices.Index(c.dictValues, "")
			if zeroDictIdx < 0 {
				return 0
			}
			valuesEncoded := c.getValuesEncoded(br)
			bm.forEachSetBit(func(i int) bool {
				return int(valuesEncoded[i][0]) == zeroDictIdx
			})
		case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64, valueTypeInt64,
			valueTypeFloat64, valueTypeIPv4, valueTypeTimestampISO8601:
			return 0
		default:
			logger.Panicf("BUG: unknown valueType=%d", c.valueType)
			return 0
		}
	}

	scp.rowsCount += uint64(bm.onesCount())
	return 0
}

func (scp *statsCountEmptyProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sc := sf.(*statsCountEmpty)

	if isSingleField(sc.fieldFilters) {
		// Fast path for count_empty(single_column)
		c := br.getColumnByName(sc.fieldFilters[0])
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
		case valueTypeDict:
			valuesEncoded := c.getValuesEncoded(br)
			dictIdx := valuesEncoded[rowIdx][0]
			if v := c.dictValues[dictIdx]; v == "" {
				scp.rowsCount++
			}
		case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64, valueTypeInt64,
			valueTypeFloat64, valueTypeIPv4, valueTypeTimestampISO8601:
		default:
			logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		}
		return 0
	}

	// Slow path - count the row at rowIdx if at least a single field enumerated inside count() is non-empty
	cs := br.getColumns()
	for _, c := range cs {
		if !prefixfilter.MatchFilters(sc.fieldFilters, c.name) {
			continue
		}
		if v := c.getValueAtRow(br, rowIdx); v != "" {
			return 0
		}
	}
	scp.rowsCount++
	return 0
}

func (scp *statsCountEmptyProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsCountEmptyProcessor)
	scp.rowsCount += src.rowsCount
}

func (scp *statsCountEmptyProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	return encoding.MarshalVarUint64(dst, scp.rowsCount)
}

func (scp *statsCountEmptyProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	rowsCount, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot unmarshal rowsCount")
	}
	src = src[n:]

	scp.rowsCount = rowsCount

	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected non-empty tail left; len(tail)=%d", len(src))
	}

	return 0, nil
}

func (scp *statsCountEmptyProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	return strconv.AppendUint(dst, scp.rowsCount, 10)
}

func parseStatsCountEmpty(lex *lexer) (statsFunc, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "count_empty")
	if err != nil {
		return nil, err
	}
	sc := &statsCountEmpty{
		fieldFilters: fieldFilters,
	}
	return sc, nil
}
