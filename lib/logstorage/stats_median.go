package logstorage

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

type statsMedian struct {
	sq *statsQuantile
}

func (sm *statsMedian) String() string {
	return "median(" + fieldNamesString(sm.sq.fieldFilters) + ")"
}

func (sm *statsMedian) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sm.sq.fieldFilters)
}

func (sm *statsMedian) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsMedianProcessor()
}

type statsMedianProcessor struct {
	sqp statsQuantileProcessor
}

func (smp *statsMedianProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sm := sf.(*statsMedian)
	return smp.sqp.updateStatsForAllRows(sm.sq, br)
}

func (smp *statsMedianProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sm := sf.(*statsMedian)
	return smp.sqp.updateStatsForRow(sm.sq, br, rowIdx)
}

func (smp *statsMedianProcessor) mergeState(a *chunkedAllocator, sf statsFunc, sfp statsProcessor) {
	sm := sf.(*statsMedian)
	src := sfp.(*statsMedianProcessor)
	smp.sqp.mergeState(a, sm.sq, &src.sqp)
}

func (smp *statsMedianProcessor) exportState(dst []byte, stopCh <-chan struct{}) []byte {
	return smp.sqp.exportState(dst, stopCh)
}

func (smp *statsMedianProcessor) importState(src []byte, stopCh <-chan struct{}) (int, error) {
	return smp.sqp.importState(src, stopCh)
}

func (smp *statsMedianProcessor) finalizeStats(sf statsFunc, dst []byte, stopCh <-chan struct{}) []byte {
	sm := sf.(*statsMedian)
	return smp.sqp.finalizeStats(sm.sq, dst, stopCh)
}

func parseStatsMedian(lex *lexer) (*statsMedian, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "median")
	if err != nil {
		return nil, err
	}
	sm := &statsMedian{
		sq: &statsQuantile{
			fieldFilters: fieldFilters,
			phi:          0.5,
			phiStr:       "0.5",
		},
	}
	return sm, nil
}
