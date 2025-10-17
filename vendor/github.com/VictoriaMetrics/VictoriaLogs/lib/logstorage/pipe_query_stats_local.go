package logstorage

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeQueryStatsLocal processes local part of the pipeQueryStats in cluster.
type pipeQueryStatsLocal struct {
}

func (ps *pipeQueryStatsLocal) String() string {
	return "query_stats_local"
}

func (ps *pipeQueryStatsLocal) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	logger.Panicf("BUG: unexpected call for %T", ps)
	return nil, nil
}

func (ps *pipeQueryStatsLocal) canLiveTail() bool {
	return false
}

func (ps *pipeQueryStatsLocal) canReturnLastNResults() bool {
	return false
}

func (ps *pipeQueryStatsLocal) updateNeededFields(_ *prefixfilter.Filter) {
	// Nothing to do
}

func (ps *pipeQueryStatsLocal) hasFilterInWithQuery() bool {
	return false
}

func (ps *pipeQueryStatsLocal) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return ps, nil
}

func (ps *pipeQueryStatsLocal) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (ps *pipeQueryStatsLocal) newPipeProcessor(_ int, stopCh <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	psp := &pipeQueryStatsLocalProcessor{
		ppNext: ppNext,
	}
	return psp
}

type pipeQueryStatsLocalProcessor struct {
	ppNext pipeProcessor

	// qs must be set before flush() cal via setQueryStats()
	qs *QueryStats

	// queryDurationNsecs must be set before flush() call via setQueryStats()
	queryDurationNsecs int64
}

func (psp *pipeQueryStatsLocalProcessor) setQueryStats(qs *QueryStats, queryDurationNsecs int64) {
	psp.qs = qs
	psp.queryDurationNsecs = queryDurationNsecs
}

func (psp *pipeQueryStatsLocalProcessor) writeBlock(_ uint, _ *blockResult) {
	// Nothing to do - query stats is passed from the remote storage nodes via a side channel.
}

func (psp *pipeQueryStatsLocalProcessor) flush() error {
	psp.qs.writeToPipeProcessor(psp.ppNext, psp.queryDurationNsecs)
	return nil
}
