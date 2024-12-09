package logstorage

import (
	"strconv"
	"unsafe"
)

type statsRateSum struct {
	fields []string

	// stepSeconds must be updated by the caller before calling newStatsProcessor().
	stepSeconds float64
}

func (sr *statsRateSum) String() string {
	return "rate_sum(" + statsFuncFieldsToString(sr.fields) + ")"
}

func (sr *statsRateSum) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, sr.fields)
}

func (sr *statsRateSum) newStatsProcessor() (statsProcessor, int) {
	srp := &statsRateSumProcessor{
		sr: sr,
		ssp: &statsSumProcessor{
			ss: &statsSum{
				fields: sr.fields,
			},
			sum: nan,
		},
	}
	return srp, int(unsafe.Sizeof(*srp))
}

type statsRateSumProcessor struct {
	sr  *statsRateSum
	ssp *statsSumProcessor
}

func (srp *statsRateSumProcessor) updateStatsForAllRows(br *blockResult) int {
	return srp.ssp.updateStatsForAllRows(br)
}

func (srp *statsRateSumProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	return srp.ssp.updateStatsForRow(br, rowIdx)
}

func (srp *statsRateSumProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsRateSumProcessor)
	srp.ssp.mergeState(src.ssp)
}

func (srp *statsRateSumProcessor) finalizeStats() string {
	rate := srp.ssp.sum
	if srp.sr.stepSeconds > 0 {
		rate /= srp.sr.stepSeconds
	}
	return strconv.FormatFloat(rate, 'f', -1, 64)
}

func parseStatsRateSum(lex *lexer) (*statsRateSum, error) {
	fields, err := parseStatsFuncFields(lex, "rate_sum")
	if err != nil {
		return nil, err
	}
	sr := &statsRateSum{
		fields: fields,
	}
	return sr, nil
}
