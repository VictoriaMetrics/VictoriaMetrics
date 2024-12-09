package logstorage

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
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

	// the maximum unique values to track per each field.
	maxValuesPerField uint64

	// fields with values longer than maxValueLen are ignored, since it is hard to use them in faceted search.
	maxValueLen uint64
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
	return s
}

func (pf *pipeFacets) canLiveTail() bool {
	return false
}

func (pf *pipeFacets) updateNeededFields(neededFields, unneededFields fieldsSet) {
	neededFields.add("*")
	unneededFields.reset()
}

func (pf *pipeFacets) hasFilterInWithQuery() bool {
	return false
}

func (pf *pipeFacets) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pf, nil
}

func (pf *pipeFacets) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.2)

	shards := make([]pipeFacetsProcessorShard, workersCount)
	for i := range shards {
		shards[i] = pipeFacetsProcessorShard{
			pipeFacetsProcessorShardNopad: pipeFacetsProcessorShardNopad{
				pf: pf,
			},
		}
	}

	pfp := &pipeFacetsProcessor{
		pf:     pf,
		stopCh: stopCh,
		cancel: cancel,
		ppNext: ppNext,

		shards: shards,

		maxStateSize: maxStateSize,
	}
	pfp.stateSizeBudget.Store(maxStateSize)

	return pfp
}

type pipeFacetsProcessor struct {
	pf     *pipeFacets
	stopCh <-chan struct{}
	cancel func()
	ppNext pipeProcessor

	shards []pipeFacetsProcessorShard

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

type pipeFacetsProcessorShard struct {
	pipeFacetsProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeFacetsProcessorShardNopad{})%128]byte
}

type pipeFacetsProcessorShardNopad struct {
	// pf points to the parent pipeFacets.
	pf *pipeFacets

	// m holds hits per every field=value pair.
	m map[string]*pipeFacetsFieldHits

	// stateSizeBudget is the remaining budget for the whole state size for the shard.
	// The per-shard budget is provided in chunks from the parent pipeTopProcessor.
	stateSizeBudget int
}

type pipeFacetsFieldHits struct {
	m          map[string]*uint64
	mustIgnore bool
}

func (fhs *pipeFacetsFieldHits) enableIgnoreField(shard *pipeFacetsProcessorShard) {
	mLen := len(fhs.m)
	fhs.m = nil
	shard.stateSizeBudget += mLen * 8
	fhs.mustIgnore = true
}

// writeBlock writes br to shard.
func (shard *pipeFacetsProcessorShard) writeBlock(br *blockResult) {
	cs := br.getColumns()
	for _, c := range cs {
		shard.updateFacetsForColumn(br, c)
	}
}

func (shard *pipeFacetsProcessorShard) updateFacetsForColumn(br *blockResult, c *blockResultColumn) {
	fhs := shard.getFieldHits(c.name)
	if fhs.mustIgnore {
		return
	}
	if c.isConst {
		v := c.valuesEncoded[0]
		shard.updateState(fhs, v, uint64(br.rowsLen))
		return
	}
	if c.valueType == valueTypeDict {
		c.forEachDictValueWithHits(br, func(v string, hits uint64) {
			shard.updateState(fhs, v, hits)
		})
		return
	}

	for i := 0; i < br.rowsLen; i++ {
		v := c.getValueAtRow(br, i)
		shard.updateState(fhs, v, 1)
	}
}

func (shard *pipeFacetsProcessorShard) updateState(fhs *pipeFacetsFieldHits, v string, hits uint64) {
	if fhs.mustIgnore {
		return
	}
	if len(v) == 0 {
		// It is impossible to calculate properly the number of hits
		// for all empty per-field values - the final number will be misleading,
		// since it doesn't include blocks without the given field.
		// So it is better ignoring empty values.
		return
	}
	if uint64(len(v)) > shard.pf.maxValueLen {
		// Ignore fields with too long values, since they are hard to use in faceted search.
		fhs.enableIgnoreField(shard)
		return
	}

	pHits := fhs.m[v]
	if pHits == nil {
		if uint64(len(fhs.m)) >= shard.pf.maxValuesPerField {
			// Ignore fields with too many unique values
			fhs.enableIgnoreField(shard)
			return
		}
		vCopy := strings.Clone(v)
		hits := uint64(0)
		pHits = &hits
		fhs.m[vCopy] = pHits
		shard.stateSizeBudget -= len(vCopy) + int(unsafe.Sizeof(vCopy)+unsafe.Sizeof(hits)+unsafe.Sizeof(pHits))
	}
	*pHits += hits
}

func (shard *pipeFacetsProcessorShard) getFieldHits(fieldName string) *pipeFacetsFieldHits {
	if shard.m == nil {
		shard.m = make(map[string]*pipeFacetsFieldHits)
	}
	fhs, ok := shard.m[fieldName]
	if !ok {
		fhs = &pipeFacetsFieldHits{
			m: make(map[string]*uint64),
		}
		fieldNameCopy := strings.Clone(fieldName)
		shard.m[fieldNameCopy] = fhs
		shard.stateSizeBudget -= len(fieldNameCopy) + int(unsafe.Sizeof(*fhs))
	}
	return fhs
}

func (pfp *pipeFacetsProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := &pfp.shards[workerID]

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
	m := make(map[string]map[string]*uint64)
	for _, shard := range pfp.shards {
		if needStop(pfp.stopCh) {
			return nil
		}
		for fieldName, fhs := range shard.m {
			if fhs.mustIgnore {
				continue
			}
			vs, ok := m[fieldName]
			if !ok {
				m[fieldName] = fhs.m
				continue
			}
			for v, pHits := range fhs.m {
				ph, ok := vs[v]
				if !ok {
					vs[v] = pHits
				} else {
					*ph += *pHits
				}
			}
		}
	}

	// sort fieldNames
	fieldNames := make([]string, 0, len(m))
	for fieldName := range m {
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
		values := m[fieldName]
		if uint64(len(values)) > pfp.pf.maxValuesPerField {
			continue
		}

		vs := make([]pipeTopEntry, 0, len(values))
		for k, pHits := range values {
			vs = append(vs, pipeTopEntry{
				k:    k,
				hits: *pHits,
			})
		}
		sort.Slice(vs, func(i, j int) bool {
			return vs[i].hits > vs[j].hits
		})
		if uint64(len(vs)) > limit {
			vs = vs[:limit]
		}
		for _, v := range vs {
			wctx.writeRow(fieldName, v.k, v.hits)
		}
	}
	wctx.flush()

	return nil
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
	if wctx.valuesLen >= 1_000_000 {
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

	maxValuesPerField := uint64(pipeFacetsDefaultMaxValuesPerField)
	if lex.isKeyword("max_values_per_field") {
		lex.nextToken()
		n, s, err := parseNumber(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse max_values_per_field: %w", err)
		}
		if n < 1 {
			return nil, fmt.Errorf("max_value_per_field must be integer bigger than 0; got %s", s)
		}
		maxValuesPerField = uint64(n)
	}

	maxValueLen := uint64(pipeFacetsDefaultMaxValueLen)
	if lex.isKeyword("max_value_len") {
		lex.nextToken()
		n, s, err := parseNumber(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse max_value_len: %w", err)
		}
		if n < 1 {
			return nil, fmt.Errorf("max_value_len must be integer bigger than 0; got %s", s)
		}
		maxValueLen = uint64(n)
	}

	pf := &pipeFacets{
		limit:             limit,
		maxValuesPerField: maxValuesPerField,
		maxValueLen:       maxValueLen,
	}
	return pf, nil
}
