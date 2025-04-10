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
	srp.ssp.sum = nan
	return srp
}

type statsRateSumProcessor struct {
	ssp statsSumProcessor
}

func (srp *statsRateSumProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	ss := sf.(*statsRateSum)
	return srp.ssp.updateStatsForAllRows(ss.ss, br)
}

func (srp *statsRateSumProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	ss := sf.(*statsRateSum)
	return srp.ssp.updateStatsForRow(ss.ss, br, rowIdx)
}

func (srp *statsRateSumProcessor) mergeState(a *chunkedAllocator, sf statsFunc, sfp statsProcessor) {
	ss := sf.(*statsRateSum)
	src := sfp.(*statsRateSumProcessor)
	srp.ssp.mergeState(a, ss.ss, &src.ssp)
}

func (srp *statsRateSumProcessor) exportState(dst []byte, stopCh <-chan struct{}) []byte {
	return srp.ssp.exportState(dst, stopCh)
}

func (srp *statsRateSumProcessor) importState(src []byte, stopCh <-chan struct{}) (int, error) {
	return srp.ssp.importState(src, stopCh)
}

func (srp *statsRateSumProcessor) finalizeStats(sf statsFunc, dst []byte, _ <-chan struct{}) []byte {
	sr := sf.(*statsRateSum)
	rate := srp.ssp.sum
	if sr.stepSeconds > 0 {
		rate /= sr.stepSeconds
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
