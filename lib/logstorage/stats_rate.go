package logstorage

import (
	"fmt"
	"strconv"
	"unsafe"
)

type statsRate struct {
	// stepSeconds must be updated by the caller before calling newStatsProcessor().
	stepSeconds float64
}

func (sr *statsRate) String() string {
	return "rate()"
}

func (sr *statsRate) updateNeededFields(_ fieldsSet) {
	// There is no need in fetching any columns for rate() - the number of matching rows can be calculated as blockResult.rowsLen
}

func (sr *statsRate) newStatsProcessor() (statsProcessor, int) {
	srp := &statsRateProcessor{
		sr: sr,
	}
	return srp, int(unsafe.Sizeof(*srp))
}

type statsRateProcessor struct {
	sr *statsRate

	rowsCount uint64
}

func (srp *statsRateProcessor) updateStatsForAllRows(br *blockResult) int {
	srp.rowsCount += uint64(br.rowsLen)
	return 0
}

func (srp *statsRateProcessor) updateStatsForRow(_ *blockResult, _ int) int {
	srp.rowsCount++
	return 0
}

func (srp *statsRateProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsRateProcessor)
	srp.rowsCount += src.rowsCount
}

func (srp *statsRateProcessor) finalizeStats() string {
	rate := float64(srp.rowsCount)
	if srp.sr.stepSeconds > 0 {
		rate /= srp.sr.stepSeconds
	}
	return strconv.FormatFloat(rate, 'f', -1, 64)
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
