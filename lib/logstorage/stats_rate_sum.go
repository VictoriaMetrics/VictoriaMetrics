package logstorage

import (
	"strconv"
)

type statsRateSum struct {
	ss *statsSum

	// stepSeconds must be updated by the caller before calling newStatsProcessor().
	stepSeconds float64
}

func (sr *statsRateSum) String() string {
	return "rate_sum(" + statsFuncFieldsToString(sr.ss.fields) + ")"
}

func (sr *statsRateSum) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, sr.ss.fields)
}

func (sr *statsRateSum) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	srp := a.newStatsRateSumProcessor()
	srp.sr = sr
	srp.ssp = a.newStatsSumProcessor()
	srp.ssp.ss = sr.ss
	srp.ssp.sum = nan
	return srp
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

func (srp *statsRateSumProcessor) finalizeStats(dst []byte, _ <-chan struct{}) []byte {
	rate := srp.ssp.sum
	if srp.sr.stepSeconds > 0 {
		rate /= srp.sr.stepSeconds
	}
	return strconv.AppendFloat(dst, rate, 'f', -1, 64)
}

func parseStatsRateSum(lex *lexer) (*statsRateSum, error) {
	fields, err := parseStatsFuncFields(lex, "rate_sum")
	if err != nil {
		return nil, err
	}
	sr := &statsRateSum{
		ss: &statsSum{
			fields: fields,
		},
	}
	return sr, nil
}
