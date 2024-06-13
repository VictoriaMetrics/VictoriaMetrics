package logstorage

import (
	"unsafe"
)

type statsMedian struct {
	fields []string
}

func (sm *statsMedian) String() string {
	return "median(" + statsFuncFieldsToString(sm.fields) + ")"
}

func (sm *statsMedian) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, sm.fields)
}

func (sm *statsMedian) newStatsProcessor() (statsProcessor, int) {
	smp := &statsMedianProcessor{
		sqp: &statsQuantileProcessor{
			sq: &statsQuantile{
				fields: sm.fields,
				phi:    0.5,
			},
		},
	}
	return smp, int(unsafe.Sizeof(*smp)) + int(unsafe.Sizeof(*smp.sqp)) + int(unsafe.Sizeof(*smp.sqp.sq))
}

type statsMedianProcessor struct {
	sqp *statsQuantileProcessor
}

func (smp *statsMedianProcessor) updateStatsForAllRows(br *blockResult) int {
	return smp.sqp.updateStatsForAllRows(br)
}

func (smp *statsMedianProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	return smp.sqp.updateStatsForRow(br, rowIdx)
}

func (smp *statsMedianProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsMedianProcessor)
	smp.sqp.mergeState(src.sqp)
}

func (smp *statsMedianProcessor) finalizeStats() string {
	return smp.sqp.finalizeStats()
}

func parseStatsMedian(lex *lexer) (*statsMedian, error) {
	fields, err := parseStatsFuncFields(lex, "median")
	if err != nil {
		return nil, err
	}
	sm := &statsMedian{
		fields: fields,
	}
	return sm, nil
}
