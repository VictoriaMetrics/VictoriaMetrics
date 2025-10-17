package logstorage

import (
	"fmt"
	"sort"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeFieldValuesLocal processes local part of the pipeFieldValues in cluster.
type pipeFieldValuesLocal struct {
	pf *pipeFieldValues
}

func (pf *pipeFieldValuesLocal) String() string {
	s := "field_values_local " + quoteTokenIfNeeded(pf.pf.field)
	if pf.pf.limit > 0 {
		s += fmt.Sprintf(" limit %d", pf.pf.limit)
	}
	return s
}

func (pf *pipeFieldValuesLocal) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	logger.Panicf("BUG: unexpected call for %T", pf)
	return nil, nil
}

func (pf *pipeFieldValuesLocal) canLiveTail() bool {
	return false
}

func (pf *pipeFieldValuesLocal) canReturnLastNResults() bool {
	return false
}

func (pf *pipeFieldValuesLocal) updateNeededFields(f *prefixfilter.Filter) {
	f.Reset()

	f.AddAllowFilter(pf.pf.field)

	hitsFieldName := pf.pf.getHitsFieldName()
	f.AddAllowFilter(hitsFieldName)
}

func (pf *pipeFieldValuesLocal) hasFilterInWithQuery() bool {
	return false
}

func (pf *pipeFieldValuesLocal) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pf, nil
}

func (pf *pipeFieldValuesLocal) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pf *pipeFieldValuesLocal) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeFieldValuesLocalProcessor{
		pf:     pf,
		ppNext: ppNext,
	}
}

type pipeFieldValuesLocalProcessor struct {
	pf     *pipeFieldValuesLocal
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeFieldValuesLocalProcessorShard]
}

type pipeFieldValuesLocalProcessorShard struct {
	vhs []ValueWithHits
}

func (pfp *pipeFieldValuesLocalProcessor) writeBlock(workerID uint, br *blockResult) {
	shard := pfp.shards.Get(workerID)

	pf := pfp.pf.pf
	cValues := br.getColumnByName(pf.field)

	hitsFieldName := pf.getHitsFieldName()
	cHits := br.getColumnByName(hitsFieldName)

	values := cValues.getValues(br)
	hits := cHits.getValues(br)

	for i, value := range values {
		hits64, ok := tryParseUint64(hits[i])
		if !ok {
			logger.Panicf("BUG: unexpected hits received from the remote storage for %q: %q; it must be uint64", value, hits[i])
		}
		shard.vhs = append(shard.vhs, ValueWithHits{
			Value: strings.Clone(value),
			Hits:  hits64,
		})
	}
}

func (pfp *pipeFieldValuesLocalProcessor) flush() error {
	pf := pfp.pf.pf
	shards := pfp.shards.All()
	if len(shards) == 0 {
		return nil
	}

	a := make([][]ValueWithHits, len(shards))
	for i, shard := range shards {
		a[i] = shard.vhs
	}
	result := MergeValuesWithHits(a, pf.limit, true)

	hitsFieldName := pf.getHitsFieldName()
	fields := []string{pf.field, hitsFieldName}
	wctx := newPipeFixedFieldsWriteContext(pfp.ppNext, fields)

	rowValues := make([]string, 2)
	for i := range result {
		rowValues[0] = result[i].Value
		rowValues[1] = string(marshalUint64String(nil, result[i].Hits))
		wctx.writeRow(rowValues)
	}
	wctx.flush()

	return nil
}

// MergeValuesWithHits merges a entries and applies the given limit to the number of returned entries.
//
// If resetHitsOnLimitExceeded is set to true and the number of merged entries exceeds the given limit,
// then hits are zeroed in the returned response.
func MergeValuesWithHits(a [][]ValueWithHits, limit uint64, resetHitsOnLimitExceeded bool) []ValueWithHits {
	needResetHits := false
	m := make(map[string]uint64)
	for _, vhs := range a {
		if !needResetHits && hasZeroHits(vhs) {
			needResetHits = true
		}
		for _, vh := range vhs {
			m[vh.Value] += vh.Hits
		}
	}

	result := make([]ValueWithHits, 0, len(m))
	for value, hits := range m {
		result = append(result, ValueWithHits{
			Value: value,
			Hits:  hits,
		})
	}

	if needResetHits {
		resetHits(result)
	}

	sortValuesWithHits(result)

	if limit > 0 && uint64(len(result)) > limit {
		if resetHitsOnLimitExceeded {
			resetHits(result)
			sortValuesWithHits(result)
		}
		result = result[:limit]
	}

	return result
}

func sortValuesWithHits(vhs []ValueWithHits) {
	// Sort results in descending order of hits and ascending order of values for identical hits.
	sort.Slice(vhs, func(i, j int) bool {
		a, b := &vhs[i], &vhs[j]
		if a.Hits == b.Hits {
			return stringsutil.LessNatural(a.Value, b.Value)
		}
		return a.Hits > b.Hits
	})
}

func resetHits(vhs []ValueWithHits) {
	for i := range vhs {
		vhs[i].Hits = 0
	}
}

func hasZeroHits(vhs []ValueWithHits) bool {
	for i := range vhs {
		if vhs[i].Hits == 0 {
			return true
		}
	}
	return false
}
