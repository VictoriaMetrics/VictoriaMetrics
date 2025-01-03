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
	sqp := a.newStatsQuantileProcessor()
	sqp.sq = sm.sq
	return sqp
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
