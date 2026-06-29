package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeCoalesce implements '| coalesce (...) as ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#coalesce-pipe
type pipeCoalesce struct {
	srcFieldFilters []string
	dstField        string
	defaultValue    string
}

func (pc *pipeCoalesce) String() string {
	if len(pc.srcFieldFilters) == 0 {
		logger.Panicf("BUG: pipeCoalesce must contain at least one srcField")
	}

	s := "coalesce(" + fieldNamesString(pc.srcFieldFilters) + ")"
	if pc.defaultValue != "" {
		s += " default " + quoteTokenIfNeeded(pc.defaultValue)
	}
	if pc.dstField != "_msg" {
		s += " as " + quoteTokenIfNeeded(pc.dstField)
	}
	return s
}

func (pc *pipeCoalesce) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pc, nil
}

func (pc *pipeCoalesce) canLiveTail() bool {
	return true
}

func (pc *pipeCoalesce) canReturnLastNResults() bool {
	return pc.dstField != "_time"
}

func (pc *pipeCoalesce) isFixedOutputFieldsOrder() bool {
	return false
}

func (pc *pipeCoalesce) updateNeededFields(pf *prefixfilter.Filter) {
	if pf.MatchString(pc.dstField) {
		pf.AddDenyFilter(pc.dstField)
		pf.AddAllowFilters(pc.srcFieldFilters)
	}
}

func (pc *pipeCoalesce) hasFilterInWithQuery() bool {
	return false
}

func (pc *pipeCoalesce) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
	return pc, nil
}

func (pc *pipeCoalesce) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pc *pipeCoalesce) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeCoalesceProcessor{
		pc:     pc,
		ppNext: ppNext,
	}
}

// pipeCoalesceProcessor processes the coalesce pipe
type pipeCoalesceProcessor struct {
	pc     *pipeCoalesce
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeCoalesceProcessorShard]
}

type pipeCoalesceProcessorShard struct {
	rc resultColumn

	cs []*blockResultColumn
}

func (pcp *pipeCoalesceProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := pcp.shards.Get(workerID)
	pc := pcp.pc

	// Initialize shard.cs
	cs := br.getColumns()
	for _, ff := range pc.srcFieldFilters {
		if !prefixfilter.IsWildcardFilter(ff) {
			c := br.getColumnByName(ff)
			shard.addColumn(c)
			continue
		}
		for _, c := range cs {
			if prefixfilter.MatchFilter(ff, c.name) {
				shard.addColumn(c)
			}
		}
	}

	// Fill the shard.rc
	for rowIdx := range br.rowsLen {
		value := ""
		for _, c := range shard.cs {
			v := c.getValueAtRow(br, rowIdx)
			if v != "" {
				value = v
				break
			}
		}
		if value == "" {
			value = pc.defaultValue
		}

		shard.rc.addValue(value)
	}

	shard.rc.name = pc.dstField
	br.addResultColumn(shard.rc)
	pcp.ppNext.writeBlock(workerID, br)

	shard.rc.reset()

	clear(shard.cs)
	shard.cs = shard.cs[:0]
}

func (shard *pipeCoalesceProcessorShard) addColumn(c *blockResultColumn) {
	// verify whether the given column already exists in shard.cs
	for _, col := range shard.cs {
		if col.name == c.name {
			// Nothing to add - the column already exists
			return
		}
	}

	// Add the column to cs.
	shard.cs = append(shard.cs, c)
}

func (pcp *pipeCoalesceProcessor) flush() error {
	return nil
}

// parsePipeCoalesce parses '| coalesce(field1, field2, field3) default "default value" as result_field'
func parsePipeCoalesce(lex *lexer) (pipe, error) {
	if !lex.isKeyword("coalesce") {
		return nil, fmt.Errorf("expecting 'coalesce'; got %q", lex.token)
	}
	lex.nextToken()

	srcFieldFilters, err := parseFieldFiltersInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse field names: %w", err)
	}

	if len(srcFieldFilters) == 0 {
		return nil, fmt.Errorf("coalesce requires at least one field name")
	}

	// Parse optional 'default' keyword and value
	defaultValue := ""
	if lex.isKeyword("default") {
		lex.nextToken()
		v, err := lex.nextCompoundToken()
		if err != nil {
			return nil, fmt.Errorf("cannot parse default value: %w", err)
		}
		defaultValue = v
	}

	// Parse 'as' token
	dstField := "_msg"
	if lex.isKeyword("as") {
		lex.nextToken()
		v, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result field name: %w", err)
		}
		dstField = v
	}

	pc := &pipeCoalesce{
		srcFieldFilters: srcFieldFilters,
		dstField:        dstField,
		defaultValue:    defaultValue,
	}

	return pc, nil
}
