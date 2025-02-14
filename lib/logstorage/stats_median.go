package logstorage

type statsMedian struct {
	sq *statsQuantile
}

func (sm *statsMedian) String() string {
	return "median(" + statsFuncFieldsToString(sm.sq.fields) + ")"
}

func (sm *statsMedian) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, sm.sq.fields)
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

func (smp *statsMedianProcessor) finalizeStats(sf statsFunc, dst []byte, stopCh <-chan struct{}) []byte {
	sm := sf.(*statsMedian)
	return smp.sqp.finalizeStats(sm.sq, dst, stopCh)
}

func parseStatsMedian(lex *lexer) (*statsMedian, error) {
	fields, err := parseStatsFuncFields(lex, "median")
	if err != nil {
		return nil, err
	}
	sm := &statsMedian{
		sq: &statsQuantile{
			fields: fields,
			phi:    0.5,
			phiStr: "0.5",
		},
	}
	return sm, nil
}
