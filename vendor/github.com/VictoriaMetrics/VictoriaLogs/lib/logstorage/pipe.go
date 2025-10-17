package logstorage

import (
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type pipe interface {
	// String returns string representation of the pipe.
	String() string

	// splitToRemoteAndLocal must return pipes for remote and local execution.
	//
	// The timestamp is the query execution timestamp.
	//
	// If the pipe can be executed remotely in full, then the returned local pipes must be empty.
	// If the pipe cannot be executed remotely, then the returned remote pipe must be nil.
	// If the pipe must be executed remotely and locally, then both returned remote and local pipes must be non-empty.
	splitToRemoteAndLocal(timestamp int64) (pipe, []pipe)

	// canLiveTail must return true if the given pipe can be used in live tailing
	//
	// See https://docs.victoriametrics.com/victorialogs/querying/#live-tailing
	canLiveTail() bool

	// canReturnLastNResults must return true if the given pipe can return last N results ordered by _time desc
	//
	// The pipe can return last N results if it doesn't modify the _time field.
	canReturnLastNResults() bool

	// updateNeededFields must update pf with fields it needs and not needs at the input.
	updateNeededFields(pf *prefixfilter.Filter)

	// newPipeProcessor must return new pipeProcessor, which writes data to the given ppNext.
	//
	// concurrency is the number of goroutines, which are allowed to run in parallel during pipe calculations.
	//
	// If stopCh is closed, the returned pipeProcessor must stop performing CPU-intensive tasks which take more than a few milliseconds.
	// It is OK to continue processing pipeProcessor calls if they take less than a few milliseconds.
	//
	// The returned pipeProcessor may call cancel() at any time in order to notify the caller to stop sending new data to it.
	newPipeProcessor(concurrency int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor

	// hasFilterInWithQuery must return true of pipe contains 'in(subquery)' filter (recursively).
	hasFilterInWithQuery() bool

	// initFilterInValues must return new pipe with the initialized values for 'in(subquery)' filters (recursively).
	//
	// If keepSubquery is false, then the returned pipe must completely replace subquery with the subquery results,
	// the the returned pipe is marshaled into `in(r1, ..., rN)` where r1, ..., rN are subquery results.
	//
	// It is OK to return the pipe itself if it doesn't contain 'in(subquery)' filters.
	initFilterInValues(cache *inValuesCache, getFieldValuesFunc getFieldValuesFunc, keepSubquery bool) (pipe, error)

	// visitSubqueries must call visitFunc for all the subqueries, which exist at the pipe (recursively).
	visitSubqueries(visitFunc func(q *Query))
}

// pipeProcessor must process a single pipe.
type pipeProcessor interface {
	// writeBlock must write the given block of data to the given pipeProcessor.
	//
	// writeBlock is called concurrently from worker goroutines.
	// The workerID is the id of the worker goroutine, which calls the writeBlock.
	// It is in the range 0 ... workersCount-1 , where workersCount is the number of worker goroutines.
	// The number of worker goroutines is unknown beforehand (but is usually limited by the number of CPU cores),
	// so the pipe must dynamically adapt to it. It is recommended using lib/atomicutil.Slice for maintaining per-worker state.
	//
	// It is OK to modify br contents inside writeBlock. The caller mustn't rely on br contents after writeBlock call.
	// It is forbidden to hold references to br after returning from writeBlock, since the caller may reuse it.
	//
	// If any error occurs at writeBlock, then cancel() must be called by pipeProcessor in order to notify worker goroutines
	// to stop sending new data. The occurred error must be returned from flush().
	//
	// cancel() may be called also when the pipeProcessor decides to stop accepting new data, even if there is no any error.
	writeBlock(workerID uint, br *blockResult)

	// flush must flush all the data accumulated in the pipeProcessor to the next pipeProcessor.
	//
	// flush is called after all the worker goroutines are stopped.
	//
	// It is guaranteed that flush() is called for every pipeProcessor returned from pipe.newPipeProcessor().
	flush() error
}

type noopPipeProcessor struct {
	stopCh          <-chan struct{}
	writeBlockFinal func(workerID uint, br *blockResult)
}

func newNoopPipeProcessor(stopCh <-chan struct{}, writeBlock func(workerID uint, br *blockResult)) pipeProcessor {
	return &noopPipeProcessor{
		stopCh:          stopCh,
		writeBlockFinal: writeBlock,
	}
}

func (npp *noopPipeProcessor) writeBlock(workerID uint, br *blockResult) {
	if needStop(npp.stopCh) {
		return
	}
	npp.writeBlockFinal(workerID, br)
}

func (npp *noopPipeProcessor) flush() error {
	logger.Panicf("BUG: mustn't be called!")
	return nil
}

func parsePipes(lex *lexer) ([]pipe, error) {
	var pipes []pipe
	for {
		p, err := parsePipe(lex)
		if err != nil {
			return nil, err
		}
		pipes = append(pipes, p)

		switch {
		case lex.isKeyword("|"):
			lex.nextToken()
		case lex.isKeyword(")", ""):
			return pipes, nil
		default:
			return nil, fmt.Errorf("unexpected token after [%s]: %q; expecting '|' or ')'", pipes[len(pipes)-1], lex.token)
		}
	}
}

func parsePipe(lex *lexer) (pipe, error) {
	pps := getPipeParsers()
	for pipeName, parseFunc := range pps {
		if !lex.isKeyword(pipeName) {
			continue
		}
		p, err := parseFunc(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q pipe: %w", pipeName, err)
		}
		return p, nil
	}

	lexState := lex.backupState()

	// Try parsing stats pipe without 'stats' keyword
	ps, err := parsePipeStatsNoStatsKeyword(lex)
	if err == nil {
		return ps, nil
	}
	lex.restoreState(lexState)

	// Try parsing filter pipe without 'filter' keyword
	pf, err := parsePipeFilterNoFilterKeyword(lex)
	if err == nil {
		return pf, nil
	}
	lex.restoreState(lexState)

	return nil, fmt.Errorf("unexpected pipe %q", lex.token)
}

var pipeParsers map[string]pipeParseFunc
var pipeParsersOnce sync.Once

type pipeParseFunc func(lex *lexer) (pipe, error)

func getPipeParsers() map[string]pipeParseFunc {
	pipeParsersOnce.Do(initPipeParsers)
	return pipeParsers
}

func initPipeParsers() {
	pipeParsers = map[string]pipeParseFunc{
		"block_stats":       parsePipeBlockStats,
		"blocks_count":      parsePipeBlocksCount,
		"collapse_nums":     parsePipeCollapseNums,
		"copy":              parsePipeCopy,
		"cp":                parsePipeCopy,
		"decolorize":        parsePipeDecolorize,
		"del":               parsePipeDelete,
		"delete":            parsePipeDelete,
		"drop":              parsePipeDelete,
		"drop_empty_fields": parsePipeDropEmptyFields,
		"extract":           parsePipeExtract,
		"extract_regexp":    parsePipeExtractRegexp,
		"eval":              parsePipeMath,
		"facets":            parsePipeFacets,
		"field_names":       parsePipeFieldNames,
		"field_values":      parsePipeFieldValues,
		"fields":            parsePipeFields,
		"filter":            parsePipeFilter,
		"first":             parsePipeFirst,
		"format":            parsePipeFormat,
		"generate_sequence": parsePipeGenerateSequence,
		"hash":              parsePipeHash,
		"join":              parsePipeJoin,
		"json_array_len":    parsePipeJSONArrayLen,
		"head":              parsePipeLimit,
		"keep":              parsePipeFields,
		"last":              parsePipeLast,
		"len":               parsePipeLen,
		"limit":             parsePipeLimit,
		"math":              parsePipeMath,
		"mv":                parsePipeRename,
		"offset":            parsePipeOffset,
		"order":             parsePipeSort,
		"pack_json":         parsePipePackJSON,
		"pack_logfmt":       parsePipePackLogfmt,
		"query_stats":       parsePipeQueryStats,
		"rename":            parsePipeRename,
		"replace":           parsePipeReplace,
		"replace_regexp":    parsePipeReplaceRegexp,
		"rm":                parsePipeDelete,
		"running_stats":     parsePipeRunningStats,
		"sample":            parsePipeSample,
		"set_stream_fields": parsePipeSetStreamFields,
		"skip":              parsePipeOffset,
		"sort":              parsePipeSort,
		"split":             parsePipeSplit,
		"stats":             parsePipeStats,
		"stats_remote":      parsePipeStats,
		"stream_context":    parsePipeStreamContext,
		"time_add":          parsePipeTimeAdd,
		"top":               parsePipeTop,
		"total_stats":       parsePipeTotalStats,
		"union":             parsePipeUnion,
		"uniq":              parsePipeUniq,
		"unpack_json":       parsePipeUnpackJSON,
		"unpack_logfmt":     parsePipeUnpackLogfmt,
		"unpack_syslog":     parsePipeUnpackSyslog,
		"unpack_words":      parsePipeUnpackWords,
		"unroll":            parsePipeUnroll,
		"where":             parsePipeFilter,
	}
}

func isPipeName(s string) bool {
	pps := getPipeParsers()
	sLower := strings.ToLower(s)
	return pps[sLower] != nil
}

func mustParsePipes(s string, timestamp int64) []pipe {
	lex := newLexer(s, timestamp)
	pipes, err := parsePipes(lex)
	if err != nil {
		logger.Panicf("BUG: cannot parse [%s]: %s", s, err)
	}
	if !lex.isEnd() {
		logger.Panicf("BUG: unexpected tail left after parsing [%s]: %s", s, lex.context())
	}
	return pipes
}

func mustParsePipe(s string, timestamp int64) pipe {
	lex := newLexer(s, timestamp)
	p, err := parsePipe(lex)
	if err != nil {
		logger.Panicf("BUG: cannot parse [%s]: %s", s, err)
	}
	if !lex.isEnd() {
		logger.Panicf("BUG: unexpected tail left after parsing [%s]: %s", s, lex.context())
	}
	return p
}
