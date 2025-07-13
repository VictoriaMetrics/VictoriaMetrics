package logstorage

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeFacetsDefaultLimit is the default number of entries pipeFacets returns per each log field.
const pipeFacetsDefaultLimit = 10

// pipeFacetsDefaulatMaxValuesPerField is the default number of unique values to track per each field.
const pipeFacetsDefaultMaxValuesPerField = 1000

// pipeFacetsDefaultMaxValueLen is the default length of values in fields, which must be ignored when building facets.
const pipeFacetsDefaultMaxValueLen = 128

// pipeFacets processes '| facets ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#facets-pipe
type pipeFacets struct {
	// limit is the maximum number of values to return per each field with the maximum number of hits.
	limit uint64

	// the maximum unique values to track per each field. Fields with bigger number of unique values are ignored.
	maxValuesPerField uint64

	// fields with values longer than maxValueLen are ignored, since it is hard to use them in faceted search.
	maxValueLen uint64

	// keep facets for fields with const values over all the selected logs.
	//
	// by default such fields are skipped, since they do not help investigating the selected logs.
	keepConstFields bool
}

func (pf *pipeFacets) String() string {
	s := "facets"
	if pf.limit != pipeFacetsDefaultLimit {
		s += fmt.Sprintf(" %d", pf.limit)
	}
	if pf.maxValuesPerField != pipeFacetsDefaultMaxValuesPerField {
		s += fmt.Sprintf(" max_values_per_field %d", pf.maxValuesPerField)
	}
	if pf.maxValueLen != pipeFacetsDefaultMaxValueLen {
		s += fmt.Sprintf(" max_value_len %d", pf.maxValueLen)
	}
	if pf.keepConstFields {
		s += " keep_const_fields"
	}
	return s
}

func (pf *pipeFacets) splitToRemoteAndLocal(timestamp int64) (pipe, []pipe) {
	pRemote := *pf
	pRemote.limit = math.MaxUint64

	psLocalStr := fmt.Sprintf("stats by (field_name, field_value) sum(hits) as hits | sort by (hits desc) limit %d partition by (field_name) | sort by (field_name, hits desc)", pf.limit)
	psLocal := mustParsePipes(psLocalStr, timestamp)

	return &pRemote, psLocal
}

func (pf *pipeFacets) canLiveTail() bool {
	return false
}

func (pf *pipeFacets) updateNeededFields(f *prefixfilter.Filter) {
	f.AddAllowFilter("*")
}

func (pf *pipeFacets) hasFilterInWithQuery() bool {
	return false
}

func (pf *pipeFacets) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pf, nil
}

func (pf *pipeFacets) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pf *pipeFacets) newPipeProcessor(concurrency int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.2)

	pfp := &pipeFacetsProcessor{
		pf:          pf,
		concurrency: concurrency,
		stopCh:      stopCh,
		cancel:      cancel,
		ppNext:      ppNext,

		maxStateSize: maxStateSize,
	}
	pfp.shards.Init = func(shard *pipeFacetsProcessorShard) {
		shard.pfp = pfp
	}
	pfp.stateSizeBudget.Store(maxStateSize)

	return pfp
}

type pipeFacetsProcessor struct {
	pf          *pipeFacets
	concurrency int
	stopCh      <-chan struct{}
	cancel      func()
	ppNext      pipeProcessor

	shards atomicutil.Slice[pipeFacetsProcessorShard]

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

type pipeFacetsProcessorShard struct {
	// pfp points to the parent pipeFacetsProcessor.
	pfp *pipeFacetsProcessor

	// a is used for reducing memory allocations when counting facets over big number of unique fields
	a chunkedAllocator

	// m holds hits per every field=value pair.
	m map[string]*pipeFacetsFieldHits

	// rowsTotal contains the total number of selected logs.
	rowsTotal uint64

	// stateSizeBudget is the remaining budget for the whole state size for the shard.
	// The per-shard budget is provided in chunks from the parent pipeTopProcessor.
	stateSizeBudget int
}

type pipeFacetsFieldHits struct {
	m          hitsMapAdaptive
	mustIgnore bool
}

func (fhs *pipeFacetsFieldHits) enableIgnoreField() {
	fhs.m.clear()
	fhs.mustIgnore = true
}

// writeBlock writes br to shard.
func (shard *pipeFacetsProcessorShard) writeBlock(br *blockResult) {
	cs := br.getColumns()
	for _, c := range cs {
		shard.updateFacetsForColumn(br, c)
	}
	shard.rowsTotal += uint64(br.rowsLen)
}

func (shard *pipeFacetsProcessorShard) updateFacetsForColumn(br *blockResult, c *blockResultColumn) {
	fhs := shard.getFieldHits(c.name)
	if fhs.mustIgnore {
		return
	}
	if fhs.m.entriesCount() > shard.pfp.pf.maxValuesPerField {
		// Ignore fields with too many unique values
		fhs.enableIgnoreField()
		return
	}

	if c.isConst {
		v := c.valuesEncoded[0]
		shard.updateStateGeneric(fhs, v, uint64(br.rowsLen))
		return
	}

	switch c.valueType {
	case valueTypeDict:
		c.forEachDictValueWithHits(br, func(v string, hits uint64) {
			shard.updateStateGeneric(fhs, v, hits)
		})
	case valueTypeUint8:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint8(v)
			shard.updateStateUint64(fhs, uint64(n))
		}
	case valueTypeUint16:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint16(v)
			shard.updateStateUint64(fhs, uint64(n))
		}
	case valueTypeUint32:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint32(v)
			shard.updateStateUint64(fhs, uint64(n))
		}
	case valueTypeUint64:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint64(v)
			shard.updateStateUint64(fhs, n)
		}
	case valueTypeInt64:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalInt64(v)
			shard.updateStateInt64(fhs, n)
		}
	default:
		for i := 0; i < br.rowsLen; i++ {
			v := c.getValueAtRow(br, i)
			shard.updateStateGeneric(fhs, v, 1)
		}
	}
}

func (shard *pipeFacetsProcessorShard) updateStateInt64(fhs *pipeFacetsFieldHits, n int64) {
	if maxValueLen := shard.pfp.pf.maxValueLen; maxValueLen <= 21 && uint64(int64StringLen(n)) > maxValueLen {
		// Ignore fields with too long values, since they are hard to use in faceted search.
		fhs.enableIgnoreField()
		return
	}
	fhs.m.updateStateInt64(n, 1)
}

func (shard *pipeFacetsProcessorShard) updateStateUint64(fhs *pipeFacetsFieldHits, n uint64) {
	if maxValueLen := shard.pfp.pf.maxValueLen; maxValueLen <= 20 && uint64(uint64StringLen(n)) > maxValueLen {
		// Ignore fields with too long values, since they are hard to use in faceted search.
		fhs.enableIgnoreField()
		return
	}
	fhs.m.updateStateUint64(n, 1)
}

func int64StringLen(n int64) int {
	if n >= 0 {
		return uint64StringLen(uint64(n))
	}
	if n == -1<<63 {
		return 21
	}
	return 1 + uint64StringLen(uint64(-n))
}

func uint64StringLen(n uint64) int {
	if n < 10 {
		return 1
	}
	if n < 100 {
		return 2
	}
	if n < 1_000 {
		return 3
	}
	if n < 10_000 {
		return 4
	}
	if n < 100_000 {
		return 5
	}
	if n < 1_000_000 {
		return 6
	}
	if n < 10_000_000 {
		return 7
	}
	if n < 100_000_000 {
		return 8
	}
	if n < 1_000_000_000 {
		return 9
	}
	if n < 10_000_000_000 {
		return 10
	}
	return 20
}

func (shard *pipeFacetsProcessorShard) updateStateGeneric(fhs *pipeFacetsFieldHits, v string, hits uint64) {
	if len(v) == 0 {
		// It is impossible to calculate properly the number of hits
		// for all empty per-field values - the final number will be misleading,
		// since it doesn't include blocks without the given field.
		// So it is better ignoring empty values.
		return
	}
	if uint64(len(v)) > shard.pfp.pf.maxValueLen {
		// Ignore fields with too long values, since they are hard to use in faceted search.
		fhs.enableIgnoreField()
		return
	}
	fhs.m.updateStateGeneric(v, hits)
}

func (shard *pipeFacetsProcessorShard) getFieldHits(fieldName string) *pipeFacetsFieldHits {
	if shard.m == nil {
		shard.m = make(map[string]*pipeFacetsFieldHits)
	}
	fhs, ok := shard.m[fieldName]
	if !ok {
		fhs = &pipeFacetsFieldHits{}
		fhs.m.init(uint(shard.pfp.concurrency), &shard.stateSizeBudget)
		fieldNameCopy := shard.a.cloneString(fieldName)
		shard.m[fieldNameCopy] = fhs
		shard.stateSizeBudget -= len(fieldNameCopy) + int(unsafe.Sizeof(fhs)+unsafe.Sizeof(*fhs))
	}
	return fhs
}

func (pfp *pipeFacetsProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := pfp.shards.Get(workerID)

	for shard.stateSizeBudget < 0 {
		// steal some budget for the state size from the global budget.
		remaining := pfp.stateSizeBudget.Add(-stateSizeBudgetChunk)
		if remaining < 0 {
			// The state size is too big. Stop processing data in order to avoid OOM crash.
			if remaining+stateSizeBudgetChunk >= 0 {
				// Notify worker goroutines to stop calling writeBlock() in order to save CPU time.
				pfp.cancel()
			}
			return
		}
		shard.stateSizeBudget += stateSizeBudgetChunk
	}

	shard.writeBlock(br)
}

func (pfp *pipeFacetsProcessor) flush() error {
	if n := pfp.stateSizeBudget.Load(); n <= 0 {
		return fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", pfp.pf.String(), pfp.maxStateSize/(1<<20))
	}

	// merge state across shards
	shards := pfp.shards.All()
	if len(shards) == 0 {
		return nil
	}

	hmasByFieldName := make(map[string][]*hitsMapAdaptive)
	rowsTotal := uint64(0)
	for _, shard := range shards {
		if needStop(pfp.stopCh) {
			return nil
		}
		for fieldName, fhs := range shard.m {
			if fhs.mustIgnore {
				continue
			}
			hmasByFieldName[fieldName] = append(hmasByFieldName[fieldName], &fhs.m)
		}
		rowsTotal += shard.rowsTotal
	}

	// sort fieldNames
	fieldNames := make([]string, 0, len(hmasByFieldName))
	for fieldName := range hmasByFieldName {
		fieldNames = append(fieldNames, fieldName)
	}
	sort.Strings(fieldNames)

	// Leave only limit entries with the biggest number of hits per each field name
	wctx := &pipeFacetsWriteContext{
		pfp: pfp,
	}
	limit := pfp.pf.limit
	for _, fieldName := range fieldNames {
		if needStop(pfp.stopCh) {
			return nil
		}

		hmas := hmasByFieldName[fieldName]
		var hms []*hitsMap
		var hmsLock sync.Mutex
		hitsMapMergeParallel(hmas, pfp.stopCh, func(hm *hitsMap) {
			hmsLock.Lock()
			hms = append(hms, hm)
			hmsLock.Unlock()
		})

		entriesCount := uint64(0)
		for _, hm := range hms {
			entriesCount += hm.entriesCount()
		}
		if entriesCount > pfp.pf.maxValuesPerField {
			continue
		}

		vs := make([]pipeTopEntry, 0, entriesCount)
		for _, hm := range hms {
			vs = appendTopEntryFacets(vs, hm)
		}

		if len(vs) == 1 && vs[0].hits == rowsTotal && !wctx.pfp.pf.keepConstFields {
			// Skip field with constant value.
			continue
		}
		sort.Slice(vs, func(i, j int) bool {
			return vs[i].hits > vs[j].hits
		})
		if uint64(len(vs)) > limit {
			vs = vs[:limit]
		}
		for _, v := range vs {
			if needStop(pfp.stopCh) {
				return nil
			}
			wctx.writeRow(fieldName, v.k, v.hits)
		}
	}
	wctx.flush()

	return nil
}

func appendTopEntryFacets(dst []pipeTopEntry, hm *hitsMap) []pipeTopEntry {
	for n, pHits := range hm.u64 {
		dst = append(dst, pipeTopEntry{
			k:    string(marshalUint64String(nil, n)),
			hits: *pHits,
		})
	}
	for n, pHits := range hm.negative64 {
		dst = append(dst, pipeTopEntry{
			k:    string(marshalInt64String(nil, int64(n))),
			hits: *pHits,
		})
	}
	for k, pHits := range hm.strings {
		dst = append(dst, pipeTopEntry{
			k:    k,
			hits: *pHits,
		})
	}
	return dst
}

type pipeFacetsWriteContext struct {
	pfp *pipeFacetsProcessor
	rcs []resultColumn
	br  blockResult

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
}

func (wctx *pipeFacetsWriteContext) writeRow(fieldName, fieldValue string, hits uint64) {
	rcs := wctx.rcs

	if len(rcs) == 0 {
		rcs = appendResultColumnWithName(rcs, "field_name")
		rcs = appendResultColumnWithName(rcs, "field_value")
		rcs = appendResultColumnWithName(rcs, "hits")
		wctx.rcs = rcs
	}

	rcs[0].addValue(fieldName)
	wctx.valuesLen += len(fieldName)

	rcs[1].addValue(fieldValue)
	wctx.valuesLen += len(fieldValue)

	hitsStr := string(marshalUint64String(nil, hits))
	rcs[2].addValue(hitsStr)
	wctx.valuesLen += len(hitsStr)

	wctx.rowsCount++

	// The 64_000 limit provides the best performance results.
	if wctx.valuesLen >= 64_000 {
		wctx.flush()
	}
}

func (wctx *pipeFacetsWriteContext) flush() {
	rcs := wctx.rcs
	br := &wctx.br

	wctx.valuesLen = 0

	// Flush rcs to ppNext
	br.setResultColumns(rcs, wctx.rowsCount)
	wctx.rowsCount = 0
	wctx.pfp.ppNext.writeBlock(0, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetValues()
	}
}

func parsePipeFacets(lex *lexer) (pipe, error) {
	if !lex.isKeyword("facets") {
		return nil, fmt.Errorf("expecting 'facets'; got %q", lex.token)
	}
	lex.nextToken()

	limit := uint64(pipeFacetsDefaultLimit)
	if isNumberPrefix(lex.token) {
		limitF, s, err := parseNumber(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse N in 'facets': %w", err)
		}
		if limitF < 1 {
			return nil, fmt.Errorf("N in 'facets %s' must be integer bigger than 0", s)
		}
		limit = uint64(limitF)
	}

	pf := &pipeFacets{
		limit:             limit,
		maxValuesPerField: pipeFacetsDefaultMaxValuesPerField,
		maxValueLen:       pipeFacetsDefaultMaxValueLen,
	}
	for {
		switch {
		case lex.isKeyword("max_values_per_field"):
			lex.nextToken()
			n, s, err := parseNumber(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse max_values_per_field: %w", err)
			}
			if n < 1 {
				return nil, fmt.Errorf("max_value_per_field must be integer bigger than 0; got %s", s)
			}
			pf.maxValuesPerField = uint64(n)
		case lex.isKeyword("max_value_len"):
			lex.nextToken()
			n, s, err := parseNumber(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse max_value_len: %w", err)
			}
			if n < 1 {
				return nil, fmt.Errorf("max_value_len must be integer bigger than 0; got %s", s)
			}
			pf.maxValueLen = uint64(n)
		case lex.isKeyword("keep_const_fields"):
			lex.nextToken()
			pf.keepConstFields = true
		default:
			return pf, nil
		}
	}
}
