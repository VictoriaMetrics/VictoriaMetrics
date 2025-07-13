package logstorage

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsCount struct {
	fieldFilters []string
}

func (sc *statsCount) String() string {
	return "count(" + fieldNamesString(sc.fieldFilters) + ")"
}

func (sc *statsCount) updateNeededFields(pf *prefixfilter.Filter) {
	if prefixfilter.MatchAll(sc.fieldFilters) {
		// Special case for count() - it doesn't need loading any additional fields
		return
	}

	pf.AddAllowFilters(sc.fieldFilters)
}

func (sc *statsCount) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsCountProcessor()
}

type statsCountProcessor struct {
	rowsCount uint64
}

func (scp *statsCountProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sc := sf.(*statsCount)

	if prefixfilter.MatchAll(sc.fieldFilters) {
		// Fast path - unconditionally count all the columns.
		scp.rowsCount += uint64(br.rowsLen)
		return 0
	}

	if isSingleField(sc.fieldFilters) {
		// Fast path for count(single_column)
		c := br.getColumnByName(sc.fieldFilters[0])
		if c.isConst {
			if c.valuesEncoded[0] != "" {
				scp.rowsCount += uint64(br.rowsLen)
			}
			return 0
		}
		if c.isTime {
			scp.rowsCount += uint64(br.rowsLen)
			return 0
		}
		switch c.valueType {
		case valueTypeString:
			for _, v := range c.getValuesEncoded(br) {
				if v != "" {
					scp.rowsCount++
				}
			}
		case valueTypeDict:
			zeroDictIdx := slices.Index(c.dictValues, "")
			if zeroDictIdx < 0 {
				scp.rowsCount += uint64(br.rowsLen)
				return 0
			}
			for _, v := range c.getValuesEncoded(br) {
				if int(v[0]) != zeroDictIdx {
					scp.rowsCount++
				}
			}
		case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64, valueTypeInt64,
			valueTypeFloat64, valueTypeIPv4, valueTypeTimestampISO8601:
			scp.rowsCount += uint64(br.rowsLen)
		default:
			logger.Panicf("BUG: unknown valueType=%d", c.valueType)
		}
		return 0
	}

	// Slow path - count rows containing at least a single non-empty value for the fields enumerated inside count().
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
				scp.rowsCount += uint64(br.rowsLen)
				return 0
			}
			continue
		}
		if c.isTime {
			scp.rowsCount += uint64(br.rowsLen)
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
				scp.rowsCount += uint64(br.rowsLen)
				return 0
			}
			valuesEncoded := c.getValuesEncoded(br)
			bm.forEachSetBit(func(i int) bool {
				return int(valuesEncoded[i][0]) == zeroDictIdx
			})
		case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64, valueTypeInt64,
			valueTypeFloat64, valueTypeIPv4, valueTypeTimestampISO8601:
			scp.rowsCount += uint64(br.rowsLen)
			return 0
		default:
			logger.Panicf("BUG: unknown valueType=%d", c.valueType)
			return 0
		}
	}

	scp.rowsCount += uint64(br.rowsLen)
	scp.rowsCount -= uint64(bm.onesCount())
	return 0
}

func (scp *statsCountProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sc := sf.(*statsCount)

	if prefixfilter.MatchAll(sc.fieldFilters) {
		// Fast path - unconditionally count the given column
		scp.rowsCount++
		return 0
	}

	if isSingleField(sc.fieldFilters) {
		// Fast path for count(single_column)
		c := br.getColumnByName(sc.fieldFilters[0])
		if c.isConst {
			if c.valuesEncoded[0] != "" {
				scp.rowsCount++
			}
			return 0
		}
		if c.isTime {
			scp.rowsCount++
			return 0
		}
		switch c.valueType {
		case valueTypeString:
			valuesEncoded := c.getValuesEncoded(br)
			if v := valuesEncoded[rowIdx]; v != "" {
				scp.rowsCount++
			}
		case valueTypeDict:
			valuesEncoded := c.getValuesEncoded(br)
			dictIdx := valuesEncoded[rowIdx][0]
			if v := c.dictValues[dictIdx]; v != "" {
				scp.rowsCount++
			}
		case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64, valueTypeInt64,
			valueTypeFloat64, valueTypeIPv4, valueTypeTimestampISO8601:
			scp.rowsCount++
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
			scp.rowsCount++
			return 0
		}
	}
	return 0
}

func (scp *statsCountProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsCountProcessor)
	scp.rowsCount += src.rowsCount
}

func (scp *statsCountProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	return encoding.MarshalVarUint64(dst, scp.rowsCount)
}

func (scp *statsCountProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
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

func (scp *statsCountProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	return strconv.AppendUint(dst, scp.rowsCount, 10)
}

func parseStatsCount(lex *lexer) (*statsCount, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "count")
	if err != nil {
		return nil, err
	}
	sc := &statsCount{
		fieldFilters: fieldFilters,
	}
	return sc, nil
}
