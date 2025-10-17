package logstorage

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsJSONValues struct {
	// fieldFilters contains field filters for fields to select from logs.
	fieldFilters []string

	// sortFields contains optional fields for sorting the selected logs.
	//
	// if sortFields is empty, then the selected logs aren't sorted.
	sortFields []*bySortField

	// limit contains an optional limit on the number of logs to select.
	//
	// if limit==0, then all the logs are selected.
	limit uint64
}

func (sv *statsJSONValues) String() string {
	s := "json_values(" + fieldNamesString(sv.fieldFilters) + ")"

	if len(sv.sortFields) > 0 {
		a := make([]string, len(sv.sortFields))
		for i, sf := range sv.sortFields {
			a[i] = sf.String()
		}
		s += fmt.Sprintf(" sort by (%s)", strings.Join(a, ", "))
	}

	if sv.limit > 0 {
		s += fmt.Sprintf(" limit %d", sv.limit)
	}
	return s
}

func (sv *statsJSONValues) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sv.fieldFilters)

	for _, sf := range sv.sortFields {
		pf.AddAllowFilter(sf.name)
	}
}

func (sv *statsJSONValues) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	sortFieldsLen := len(sv.sortFields)

	if sortFieldsLen == 0 {
		return a.newStatsJSONValuesProcessor()
	}

	if sv.limit <= 0 {
		svp := a.newStatsJSONValuesSortedProcessor()
		svp.sortFieldsLen = sortFieldsLen
		return svp
	}

	svp := a.newStatsJSONValuesTopkProcessor()
	svp.sortFieldsLen = sortFieldsLen
	return svp
}

type statsJSONValuesProcessor struct {
	entries []string

	fieldsBuf []Field
}

func (svp *statsJSONValuesProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sv := sf.(*statsJSONValues)
	if svp.limitReached(sv) {
		// Limit on the number of entries has been reached
		return 0
	}

	stateSizeIncrease := 0
	mc := getMatchingColumns(br, sv.fieldFilters)
	mc.sort()
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		stateSizeIncrease += svp.updateStateForRow(br, mc.cs, rowIdx)
	}
	putMatchingColumns(mc)

	return stateSizeIncrease
}

func (svp *statsJSONValuesProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sv := sf.(*statsJSONValues)
	if svp.limitReached(sv) {
		// Limit on the number of entries has been reached
		return 0
	}

	mc := getMatchingColumns(br, sv.fieldFilters)
	mc.sort()
	stateSizeIncrease := svp.updateStateForRow(br, mc.cs, rowIdx)
	putMatchingColumns(mc)

	return stateSizeIncrease
}

func (svp *statsJSONValuesProcessor) updateStateForRow(br *blockResult, cs []*blockResultColumn, rowIdx int) int {
	svp.fieldsBuf = slicesutil.SetLength(svp.fieldsBuf, len(cs))
	for i, c := range cs {
		svp.fieldsBuf[i] = Field{
			Name:  c.name,
			Value: c.getValueAtRow(br, rowIdx),
		}
	}

	value := string(MarshalFieldsToJSON(nil, svp.fieldsBuf))
	svp.entries = append(svp.entries, value)
	return int(unsafe.Sizeof(value)) + len(value)
}

func (svp *statsJSONValuesProcessor) mergeState(_ *chunkedAllocator, sf statsFunc, sfp statsProcessor) {
	sv := sf.(*statsJSONValues)
	if svp.limitReached(sv) {
		return
	}

	src := sfp.(*statsJSONValuesProcessor)
	svp.entries = append(svp.entries, src.entries...)
}

func (svp *statsJSONValuesProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	dst = encoding.MarshalVarUint64(dst, uint64(len(svp.entries)))
	for _, v := range svp.entries {
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(v))
	}
	return dst
}

func (svp *statsJSONValuesProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	entriesLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot unmarshal entriesLen")
	}
	src = src[n:]

	entries := make([]string, entriesLen)
	stateSizeIncrease := int(unsafe.Sizeof(entries[0])) * len(entries)
	for i := range entries {
		v, n := encoding.UnmarshalBytes(src)
		if n <= 0 {
			return 0, fmt.Errorf("cannot unmarshal value")
		}
		src = src[n:]

		entries[i] = string(v)
		stateSizeIncrease += len(v)
	}
	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected tail left after unmarshaling entries; len(tail)=%d", len(src))
	}

	if len(entries) == 0 {
		entries = nil
	}
	svp.entries = entries

	return stateSizeIncrease, nil
}

func (svp *statsJSONValuesProcessor) finalizeStats(sf statsFunc, dst []byte, _ <-chan struct{}) []byte {
	sv := sf.(*statsJSONValues)

	entries := svp.entries

	if limit := sv.limit; limit > 0 && uint64(len(entries)) > limit {
		entries = entries[:limit]
	}

	return marshalJSONValues(dst, entries)
}

func (svp *statsJSONValuesProcessor) limitReached(sv *statsJSONValues) bool {
	limit := sv.limit
	return limit > 0 && uint64(len(svp.entries)) > limit
}

func parseStatsJSONValues(lex *lexer) (statsFunc, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "json_values")
	if err != nil {
		return nil, err
	}

	sv := &statsJSONValues{
		fieldFilters: fieldFilters,
	}

	if lex.isKeyword("sort", "order") {
		lex.nextToken()
		if lex.isKeyword("by") {
			lex.nextToken()
		}
		sfs, err := parseBySortFields(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'sort': %w", err)
		}
		sv.sortFields = sfs
	}

	if lex.isKeyword("limit") {
		n, err := parseLimit(lex)
		if err != nil {
			return nil, err
		}
		sv.limit = n
	}
	return sv, nil
}
