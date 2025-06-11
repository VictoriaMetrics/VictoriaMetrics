package logstorage

import (
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

type statsRate struct {
	// stepSeconds must be updated by the caller before calling newStatsProcessor().
	stepSeconds float64
}

func (sr *statsRate) String() string {
	return "rate()"
}

func (sr *statsRate) updateNeededFields(_ *prefixfilter.Filter) {
	// There is no need in fetching any columns for rate() - the number of matching rows can be calculated as blockResult.rowsLen
}

func (sr *statsRate) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsRateProcessor()
}

type statsRateProcessor struct {
	rowsCount uint64
}

func (srp *statsRateProcessor) updateStatsForAllRows(_ statsFunc, br *blockResult) int {
	srp.rowsCount += uint64(br.rowsLen)
	return 0
}

func (srp *statsRateProcessor) updateStatsForRow(_ statsFunc, _ *blockResult, _ int) int {
	srp.rowsCount++
	return 0
}

func (srp *statsRateProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsRateProcessor)
	srp.rowsCount += src.rowsCount
}

func (srp *statsRateProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	return encoding.MarshalVarUint64(dst, srp.rowsCount)
}

func (srp *statsRateProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	rowsCount, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot unmarshal rowsCount")
	}
	src = src[n:]

	srp.rowsCount = rowsCount

	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected non-empty tail left; len(tail)=%d", len(src))
	}

	return 0, nil
}

func (srp *statsRateProcessor) finalizeStats(sf statsFunc, dst []byte, _ <-chan struct{}) []byte {
	sr := sf.(*statsRate)
	rate := float64(srp.rowsCount)
	if sr.stepSeconds > 0 {
		rate /= sr.stepSeconds
	}
	return strconv.AppendFloat(dst, rate, 'f', -1, 64)
}

func parseStatsRate(lex *lexer) (*statsRate, error) {
	fields, err := parseStatsFuncFields(lex, "rate")
	if err != nil {
		return nil, err
	}
	if len(fields) > 0 {
		return nil, fmt.Errorf("unexpected non-empty args for 'rate()' function: %q", fields)
	}
	sr := &statsRate{}
	return sr, nil
}
