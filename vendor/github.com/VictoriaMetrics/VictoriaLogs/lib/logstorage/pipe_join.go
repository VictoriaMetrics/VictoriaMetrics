package logstorage

import (
	"fmt"
	"slices"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeJoin processes '| join ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#join-pipe
type pipeJoin struct {
	// byFields contains fields to use for join on q results
	byFields []string

	// q is a query for obtaining results for joining
	q *Query

	// The join is performed as INNER JOIN if isInner is set.
	// Otherwise the join is performed as LEFT JOIN.
	isInner bool

	// prefix is the prefix to add to log fields from q query
	prefix string

	// m contains results for joining. They are automatically initialized during query execution
	m map[string][][]Field
}

func (pj *pipeJoin) String() string {
	s := fmt.Sprintf("join by (%s) (%s)", fieldNamesString(pj.byFields), pj.q.String())
	if pj.isInner {
		s += " inner"
	}
	if pj.prefix != "" {
		s += " prefix " + quoteTokenIfNeeded(pj.prefix)
	}
	return s
}

func (pj *pipeJoin) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return nil, []pipe{pj}
}

func (pj *pipeJoin) canLiveTail() bool {
	return true
}

func (pj *pipeJoin) hasFilterInWithQuery() bool {
	// Do not check for in(...) filters at pj.q, since they are checked separately during pj.q execution.
	return false
}

func (pj *pipeJoin) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	// Do not init values for in(...) filters at pj.q, since they are initialized separately at initJoinMap.
	return pj, nil
}

func (pj *pipeJoin) visitSubqueries(visitFunc func(q *Query)) {
	pj.q.visitSubqueries(visitFunc)
}

func (pj *pipeJoin) initJoinMap(getJoinMapFunc getJoinMapFunc) (pipe, error) {
	m, err := getJoinMapFunc(pj.q, pj.byFields, pj.prefix)
	if err != nil {
		return nil, fmt.Errorf("cannot execute query at pipe [%s]: %w", pj, err)
	}
	pjNew := *pj
	pjNew.m = m
	return &pjNew, nil
}

func (pj *pipeJoin) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(pj.byFields)
}

func (pj *pipeJoin) newPipeProcessor(_ int, stopCh <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeJoinProcessor{
		pj:     pj,
		stopCh: stopCh,
		ppNext: ppNext,
	}
}

type pipeJoinProcessor struct {
	pj     *pipeJoin
	stopCh <-chan struct{}
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeJoinProcessorShard]
}

type pipeJoinProcessorShard struct {
	wctx pipeUnpackWriteContext

	byValues     []string
	byValuesIdxs []int
	tmpBuf       []byte
}

func (pjp *pipeJoinProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	pj := pjp.pj
	shard := pjp.shards.Get(workerID)
	shard.wctx.init(workerID, pjp.ppNext, true, true, br)

	shard.byValues = slicesutil.SetLength(shard.byValues, len(pj.byFields))
	byValues := shard.byValues

	cs := br.getColumns()
	shard.byValuesIdxs = slicesutil.SetLength(shard.byValuesIdxs, len(cs))
	byValuesIdxs := shard.byValuesIdxs
	for i := range cs {
		name := cs[i].name
		byValuesIdxs[i] = slices.Index(pj.byFields, name)

	}

	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		clear(byValues)
		for j := range cs {
			if cIdx := byValuesIdxs[j]; cIdx >= 0 {
				byValues[cIdx] = cs[j].getValueAtRow(br, rowIdx)
			}
		}

		shard.tmpBuf = marshalStrings(shard.tmpBuf[:0], byValues)
		matchingRows := pj.m[string(shard.tmpBuf)]

		if len(matchingRows) == 0 {
			if !pj.isInner {
				shard.wctx.writeRow(rowIdx, nil)
			}
			continue
		}
		for _, extraFields := range matchingRows {
			if needStop(pjp.stopCh) {
				return
			}
			shard.wctx.writeRow(rowIdx, extraFields)
		}
	}

	shard.wctx.flush()
	shard.wctx.reset()
}

func (pjp *pipeJoinProcessor) flush() error {
	return nil
}

func parsePipeJoin(lex *lexer) (pipe, error) {
	if !lex.isKeyword("join") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "join")
	}
	lex.nextToken()

	// parse by (...)
	if lex.isKeyword("by", "on") {
		lex.nextToken()
	}

	byFields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'by(...)' at 'join': %w", err)
	}
	if len(byFields) == 0 {
		return nil, fmt.Errorf("'by(...)' at 'join' must contain at least a single field")
	}
	if slices.Contains(byFields, "*") {
		return nil, fmt.Errorf("join by '*' isn't supported")
	}

	// Parse join query
	q, err := parseQueryInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse join(...) query: %w", err)
	}

	pj := &pipeJoin{
		byFields: byFields,
		q:        q,
	}

	if lex.isKeyword("inner") {
		lex.nextToken()
		pj.isInner = true
	}

	if lex.isKeyword("prefix") {
		lex.nextToken()
		prefix, err := getCompoundToken(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot read prefix for [%s]: %w", pj, err)
		}
		pj.prefix = prefix
	}

	return pj, nil
}
