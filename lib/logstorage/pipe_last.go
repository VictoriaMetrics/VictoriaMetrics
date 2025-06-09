package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// pipeLast processes '| last ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#last-pipe
type pipeLast struct {
	ps *pipeSort
}

func (pl *pipeLast) String() string {
	return pipeLastFirstString(pl.ps)
}

func pipeLastFirstString(ps *pipeSort) string {
	s := "first"
	if ps.isDesc {
		s = "last"
	}
	if ps.limit != 1 {
		s += fmt.Sprintf(" %d", ps.limit)
	}
	if len(ps.byFields) > 0 {
		a := make([]string, len(ps.byFields))
		for i, bf := range ps.byFields {
			a[i] = bf.String()
		}
		s += " by (" + strings.Join(a, ", ") + ")"
	}
	if len(ps.partitionByFields) > 0 {
		s += " partition by (" + fieldNamesString(ps.partitionByFields) + ")"
	}
	if ps.rankFieldName != "" {
		s += rankFieldNameString(ps.rankFieldName)
	}
	return s
}

func (pl *pipeLast) splitToRemoteAndLocal(timestamp int64) (pipe, []pipe) {
	return pl.ps.splitToRemoteAndLocal(timestamp)
}

func (pl *pipeLast) canLiveTail() bool {
	return false
}

func (pl *pipeLast) updateNeededFields(pf *prefixfilter.Filter) {
	pl.ps.updateNeededFields(pf)
}

func (pl *pipeLast) hasFilterInWithQuery() bool {
	return false
}

func (pl *pipeLast) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pl, nil
}

func (pl *pipeLast) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pl *pipeLast) newPipeProcessor(_ int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	return newPipeTopkProcessor(pl.ps, stopCh, cancel, ppNext)
}

func (pl *pipeLast) addPartitionByTime(step int64) {
	pl.ps.addPartitionByTime(step)
}

func parsePipeLast(lex *lexer) (pipe, error) {
	if !lex.isKeyword("last") {
		return nil, fmt.Errorf("expecting 'last'; got %q", lex.token)
	}
	lex.nextToken()

	ps, err := parsePipeLastFirst(lex)
	if err != nil {
		return nil, err
	}
	ps.isDesc = true
	pl := &pipeLast{
		ps: ps,
	}
	return pl, nil
}

func parsePipeLastFirst(lex *lexer) (*pipeSort, error) {
	var ps pipeSort
	ps.limit = 1
	if !lex.isKeyword("by", "partition", "rank", "(", "|", ")", "") {
		s := lex.token
		n, ok := tryParseUint64(s)
		lex.nextToken()
		if !ok {
			return nil, fmt.Errorf("expecting number; got %q", s)
		}
		if n < 1 {
			return nil, fmt.Errorf("the number must be bigger than 0; got %d", n)
		}
		ps.limit = n
	}

	if lex.isKeyword("by", "(") {
		if lex.isKeyword("by") {
			lex.nextToken()
		}
		bfs, err := parseBySortFields(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'by' clause: %w", err)
		}
		ps.byFields = bfs
	}

	if lex.isKeyword("partition") {
		lex.nextToken()
		if lex.isKeyword("by") {
			lex.nextToken()
		}
		fields, err := parseFieldNamesInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'partition by' args: %w", err)
		}
		ps.partitionByFields = fields
	}

	if lex.isKeyword("rank") {
		rankFieldName, err := parseRankFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot read rank field name: %s", err)
		}
		ps.rankFieldName = rankFieldName
	}

	return &ps, nil
}
