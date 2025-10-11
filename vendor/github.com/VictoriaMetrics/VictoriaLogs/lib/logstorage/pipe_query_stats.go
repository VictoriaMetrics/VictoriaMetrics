package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeQueryStats implements '| query_stats' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#query_stats-pipe
type pipeQueryStats struct {
}

func (ps *pipeQueryStats) String() string {
	return "query_stats"
}

func (ps *pipeQueryStats) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	psLocal := &pipeQueryStatsLocal{}
	return ps, []pipe{psLocal}
}

func (ps *pipeQueryStats) canLiveTail() bool {
	return false
}

func (ps *pipeQueryStats) canReturnLastNResults() bool {
	return false
}

func (ps *pipeQueryStats) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter("*")
}

func (ps *pipeQueryStats) hasFilterInWithQuery() bool {
	return false
}

func (ps *pipeQueryStats) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return ps, nil
}

func (ps *pipeQueryStats) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (ps *pipeQueryStats) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	psp := &pipeQueryStatsProcessor{
		ps:     ps,
		ppNext: ppNext,
	}
	return psp
}

type pipeQueryStatsProcessor struct {
	ps     *pipeQueryStats
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeQueryStatsProcessorShard]

	// qs must be set via setQueryStats() before flush() call.
	qs *QueryStats

	// queryDurationNsecs must be set via setQueryStats() before flush() call.
	queryDurationNsecs int64
}

type pipeQueryStatsProcessorShard struct {
	// sink is used for preventing from the elimination of the loop inside writeBlock by too smart compiler
	sink int
}

func (psp *pipeQueryStatsProcessor) setQueryStats(qs *QueryStats, queryDurationNsecs int64) {
	psp.qs = qs
	psp.queryDurationNsecs = queryDurationNsecs
}

func (psp *pipeQueryStatsProcessor) writeBlock(workerID uint, br *blockResult) {
	// Read all the data from br in order to emulate the default behaviour
	// when this data is returned back to the client if there is no query_stats pipe at the end of the query.
	shard := psp.shards.Get(workerID)

	cs := br.getColumns()
	for _, c := range cs {
		values := c.getValues(br)
		shard.sink += len(values)
	}
}

func (psp *pipeQueryStatsProcessor) flush() error {
	psp.qs.writeToPipeProcessor(psp.ppNext, psp.queryDurationNsecs)
	return nil
}

func parsePipeQueryStats(lex *lexer) (pipe, error) {
	if !lex.isKeyword("query_stats") {
		return nil, fmt.Errorf("expecting 'query_stats'; got %q", lex.token)
	}
	lex.nextToken()

	ps := &pipeQueryStats{}

	return ps, nil
}
