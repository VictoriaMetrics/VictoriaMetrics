package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsAny struct {
	fieldName string
}

func (sa *statsAny) String() string {
	return "any(" + quoteTokenIfNeeded(sa.fieldName) + ")"
}

func (sa *statsAny) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(sa.fieldName)
}

func (sa *statsAny) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsAnyProcessor()
}

type statsAnyProcessor struct {
	value string
}

func (sap *statsAnyProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sa := sf.(*statsAny)
	if sap.value != "" {
		return 0
	}

	return sap.updateState(sa, br, 0)
}

func (sap *statsAnyProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sa := sf.(*statsAny)
	if sap.value != "" {
		return 0
	}

	return sap.updateState(sa, br, rowIdx)
}

func (sap *statsAnyProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsAnyProcessor)
	if sap.value == "" {
		sap.value = src.value
	}
}

func (sap *statsAnyProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(sap.value))
	return dst
}

func (sap *statsAnyProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	value, n := encoding.UnmarshalBytes(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot unmarshal value")
	}
	src = src[n:]
	sap.value = string(value)

	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected non-empty tail left; len(tail)=%d", len(src))
	}

	stateSize := len(sap.value)

	return stateSize, nil
}

func (sap *statsAnyProcessor) updateState(sa *statsAny, br *blockResult, rowIdx int) int {
	c := br.getColumnByName(sa.fieldName)
	v := c.getValueAtRow(br, rowIdx)
	if v != "" {
		sap.value = strings.Clone(v)
		return len(v)
	}

	return 0
}

func (sap *statsAnyProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	return append(dst, sap.value...)
}

func parseStatsAny(lex *lexer) (statsFunc, error) {
	args, err := parseStatsFuncArgs(lex, "any")
	if err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args for 'any' function; got %d; want 1; args: %q", len(args), args)
	}

	sa := &statsAny{
		fieldName: args[0],
	}
	return sa, nil
}
