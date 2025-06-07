package logstorage

import (
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

type statsAvg struct {
	fieldFilters []string
}

func (sa *statsAvg) String() string {
	return "avg(" + fieldNamesString(sa.fieldFilters) + ")"
}

func (sa *statsAvg) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sa.fieldFilters)
}

func (sa *statsAvg) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsAvgProcessor()
}

type statsAvgProcessor struct {
	sum   float64
	count uint64
}

func (sap *statsAvgProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sa := sf.(*statsAvg)

	mc := getMatchingColumns(br, sa.fieldFilters)
	for _, c := range mc.cs {
		f, count := c.sumValues(br)
		sap.sum += f
		sap.count += uint64(count)
	}
	putMatchingColumns(mc)

	return 0
}

func (sap *statsAvgProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sa := sf.(*statsAvg)

	mc := getMatchingColumns(br, sa.fieldFilters)
	for _, c := range mc.cs {
		f, ok := c.getFloatValueAtRow(br, rowIdx)
		if ok {
			sap.sum += f
			sap.count++
		}
	}
	putMatchingColumns(mc)

	return 0
}

func (sap *statsAvgProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsAvgProcessor)
	sap.sum += src.sum
	sap.count += src.count
}

func (sap *statsAvgProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	dst = marshalFloat64(dst, sap.sum)
	dst = encoding.MarshalVarUint64(dst, sap.count)
	return dst
}

func (sap *statsAvgProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	if len(src) < 8 {
		return 0, fmt.Errorf("cannot unmarshal sum from %d bytes; need 8 bytes", len(src))
	}
	sap.sum = unmarshalFloat64(bytesutil.ToUnsafeString(src))
	src = src[8:]

	count, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot unmarshal count")
	}
	sap.count = count
	src = src[n:]

	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected tail left; len(tail)=%d", len(src))
	}

	return 0, nil
}

func (sap *statsAvgProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	avg := sap.sum / float64(sap.count)
	return strconv.AppendFloat(dst, avg, 'f', -1, 64)
}

func parseStatsAvg(lex *lexer) (*statsAvg, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "avg")
	if err != nil {
		return nil, err
	}
	sa := &statsAvg{
		fieldFilters: fieldFilters,
	}
	return sa, nil
}

func parseStatsFuncFields(lex *lexer, funcName string) ([]string, error) {
	if !lex.isKeyword(funcName) {
		return nil, fmt.Errorf("unexpected func; got %q; want %q", lex.token, funcName)
	}
	lex.nextToken()
	fields, err := parseFieldFiltersInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q args: %w", funcName, err)
	}

	// Check that all the selected fields are real fields
	for _, f := range fields {
		if prefixfilter.IsWildcardFilter(f) {
			return nil, fmt.Errorf("unexpected wildcard filter %q inside %s()", f, funcName)
		}
	}

	return fields, nil
}

func parseStatsFuncFieldFilters(lex *lexer, funcName string) ([]string, error) {
	if !lex.isKeyword(funcName) {
		return nil, fmt.Errorf("unexpected func; got %q; want %q", lex.token, funcName)
	}
	lex.nextToken()
	fields, err := parseFieldFiltersInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q args: %w", funcName, err)
	}
	if len(fields) == 0 {
		fields = []string{"*"}
	}
	return fields, nil
}
