package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeJSONArrayConcat processes '| json_array_concat ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#json_array_concat-pipe
type pipeJSONArrayConcat struct {
	delimiter   string
	fromField   string
	resultField string
}

func (pc *pipeJSONArrayConcat) String() string {
	s := "json_array_concat"
	if pc.delimiter != "" {
		s += " " + quoteTokenIfNeeded(pc.delimiter)
	}
	if !isMsgFieldName(pc.fromField) {
		s += " from " + quoteTokenIfNeeded(pc.fromField)
	}
	if pc.resultField != pc.fromField {
		s += " as " + quoteTokenIfNeeded(pc.resultField)
	}
	return s
}

func (pc *pipeJSONArrayConcat) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pc, nil
}

func (pc *pipeJSONArrayConcat) canLiveTail() bool {
	return true
}

func (pc *pipeJSONArrayConcat) canReturnLastNResults() bool {
	return pc.resultField != "_time"
}

func (pc *pipeJSONArrayConcat) isFixedOutputFieldsOrder() bool {
	return false
}

func (pc *pipeJSONArrayConcat) updateNeededFields(pf *prefixfilter.Filter) {
	if pf.MatchString(pc.resultField) {
		pf.AddDenyFilter(pc.resultField)
		pf.AddAllowFilter(pc.fromField)
	}
}

func (pc *pipeJSONArrayConcat) hasFilterInWithQuery() bool {
	return false
}

func (pc *pipeJSONArrayConcat) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
	return pc, nil
}

func (pc *pipeJSONArrayConcat) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pc *pipeJSONArrayConcat) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	pcp := &pipeJSONArrayConcatProcessor{
		pc:     pc,
		ppNext: ppNext,
	}
	pcp.shards.Init = func(shard *pipeJSONArrayConcatProcessorShard) {
		shard.reset()
	}
	return pcp
}

type pipeJSONArrayConcatProcessor struct {
	pc     *pipeJSONArrayConcat
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeJSONArrayConcatProcessorShard]
}

func (pcp *pipeJSONArrayConcatProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := pcp.shards.Get(workerID)
	shard.rc.name = pcp.pc.resultField

	c := br.getColumnByName(pcp.pc.fromField)
	delimiter := pcp.pc.delimiter
	if c.isConst {
		// Fast path for const column
		v := c.valuesEncoded[0]
		out := shard.concat(v, delimiter)
		shard.rc.addValue(out)
		br.addResultColumnConst(shard.rc)
	} else {
		// Slow path for other columns
		values := c.getValues(br)
		prevOut := ""
		for rowIdx := range values {
			if rowIdx == 0 || values[rowIdx] != values[rowIdx-1] {
				prevOut = shard.concat(values[rowIdx], delimiter)
			}
			shard.rc.addValue(prevOut)
		}
		br.addResultColumn(shard.rc)
	}

	pcp.ppNext.writeBlock(workerID, br)

	shard.reset()
}

type pipeJSONArrayConcatProcessorShard struct {
	a  arena
	rc resultColumn

	tmpValues []string
}

func (shard *pipeJSONArrayConcatProcessorShard) reset() {
	shard.a.reset()
	shard.rc.reset()

	shard.tmpValues = shard.tmpValues[:0]
}

func (shard *pipeJSONArrayConcatProcessorShard) concat(arrayStr, delimiter string) string {
	shard.tmpValues = unpackJSONArray(shard.tmpValues[:0], &shard.a, arrayStr)

	bLen := len(shard.a.b)
	for i, v := range shard.tmpValues {
		if i > 0 {
			shard.a.b = append(shard.a.b, delimiter...)
		}
		shard.a.b = append(shard.a.b, v...)
	}
	return bytesutil.ToUnsafeString(shard.a.b[bLen:])
}

func (pcp *pipeJSONArrayConcatProcessor) flush() error {
	return nil
}

func parsePipeJSONArrayConcat(lex *lexer) (pipe, error) {
	if !lex.isKeyword("json_array_concat") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "json_array_concat")
	}
	lex.nextToken()

	delimiter := ""
	if !lex.isQueryPartTrailer() && !lex.isKeyword("from", "as") {
		s, err := lex.nextCompoundToken()
		if err != nil {
			return nil, fmt.Errorf("cannot parse delimiter for 'json_array_concat': %w", err)
		}
		delimiter = s
	}

	fromField := "_msg"
	if !lex.isQueryPartTrailer() && !lex.isKeyword("as") {
		if lex.isKeyword("from") {
			lex.nextToken()
		}
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'from' field for 'json_array_concat': %w", err)
		}
		fromField = f
	}

	resultField := fromField
	if !lex.isQueryPartTrailer() {
		if lex.isKeyword("as") {
			lex.nextToken()
		}
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result field for 'json_array_concat': %w", err)
		}
		resultField = f
	}

	return &pipeJSONArrayConcat{
		delimiter:   delimiter,
		fromField:   fromField,
		resultField: resultField,
	}, nil
}
