package logstorage

import (
	"fmt"

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

type noopPipeProcessor func(workerID uint, br *blockResult)

func newNoopPipeProcessor(writeBlock func(workerID uint, br *blockResult)) pipeProcessor {
	return noopPipeProcessor(writeBlock)
}

func (npp noopPipeProcessor) writeBlock(workerID uint, br *blockResult) {
	npp(workerID, br)
}

func (npp noopPipeProcessor) flush() error {
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
	switch {
	case lex.isKeyword("block_stats"):
		ps, err := parsePipeBlockStats(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'block_stats' pipe: %w", err)
		}
		return ps, nil
	case lex.isKeyword("blocks_count"):
		pc, err := parsePipeBlocksCount(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'blocks_count' pipe: %w", err)
		}
		return pc, nil
	case lex.isKeyword("collapse_nums"):
		pc, err := parsePipeCollapseNums(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'collapse_nums' pipe: %w", err)
		}
		return pc, nil
	case lex.isKeyword("copy", "cp"):
		pc, err := parsePipeCopy(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'copy' pipe: %w", err)
		}
		return pc, nil
	case lex.isKeyword("decolorize"):
		pd, err := parsePipeDecolorize(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'decolorize' pipe: %w", err)
		}
		return pd, nil
	case lex.isKeyword("delete", "del", "rm", "drop"):
		pd, err := parsePipeDelete(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'delete' pipe: %w", err)
		}
		return pd, nil
	case lex.isKeyword("drop_empty_fields"):
		pd, err := parsePipeDropEmptyFields(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'drop_empty_fields' pipe: %w", err)
		}
		return pd, nil
	case lex.isKeyword("extract"):
		pe, err := parsePipeExtract(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'extract' pipe: %w", err)
		}
		return pe, nil
	case lex.isKeyword("extract_regexp"):
		pe, err := parsePipeExtractRegexp(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'extract_regexp' pipe: %w", err)
		}
		return pe, nil
	case lex.isKeyword("facets"):
		pf, err := parsePipeFacets(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'facets' pipe: %w", err)
		}
		return pf, nil
	case lex.isKeyword("field_names"):
		pf, err := parsePipeFieldNames(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'field_names' pipe: %w", err)
		}
		return pf, nil
	case lex.isKeyword("field_values"):
		pf, err := parsePipeFieldValues(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'field_values' pipe: %w", err)
		}
		return pf, nil
	case lex.isKeyword("fields", "keep"):
		pf, err := parsePipeFields(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'fields' pipe: %w", err)
		}
		return pf, nil
	case lex.isKeyword("filter", "where"):
		pf, err := parsePipeFilter(lex, true)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'filter' pipe: %w", err)
		}
		return pf, nil
	case lex.isKeyword("first"):
		pf, err := parsePipeFirst(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'first' pipe: %w", err)
		}
		return pf, nil
	case lex.isKeyword("format"):
		pf, err := parsePipeFormat(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'format' pipe: %w", err)
		}
		return pf, nil
	case lex.isKeyword("join"):
		pj, err := parsePipeJoin(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'join' pipe: %w", err)
		}
		return pj, nil
	case lex.isKeyword("json_array_len"):
		pl, err := parsePipeJSONArrayLen(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'json_array_len' pipe: %w", err)
		}
		return pl, nil
	case lex.isKeyword("hash"):
		ph, err := parsePipeHash(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'hash' pipe: %w", err)
		}
		return ph, nil
	case lex.isKeyword("last"):
		pl, err := parsePipeLast(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'last' pipe: %w", err)
		}
		return pl, nil
	case lex.isKeyword("len"):
		pl, err := parsePipeLen(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'len' pipe: %w", err)
		}
		return pl, nil
	case lex.isKeyword("limit", "head"):
		pl, err := parsePipeLimit(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'limit' pipe: %w", err)
		}
		return pl, nil
	case lex.isKeyword("math", "eval"):
		pm, err := parsePipeMath(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'math' pipe: %w", err)
		}
		return pm, nil
	case lex.isKeyword("offset", "skip"):
		ps, err := parsePipeOffset(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'offset' pipe: %w", err)
		}
		return ps, nil
	case lex.isKeyword("pack_json"):
		pp, err := parsePipePackJSON(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'pack_json' pipe: %w", err)
		}
		return pp, nil
	case lex.isKeyword("pack_logfmt"):
		pp, err := parsePipePackLogfmt(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'pack_logfmt' pipe: %w", err)
		}
		return pp, nil
	case lex.isKeyword("rename", "mv"):
		pr, err := parsePipeRename(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'rename' pipe: %w", err)
		}
		return pr, nil
	case lex.isKeyword("replace"):
		pr, err := parsePipeReplace(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'replace' pipe: %w", err)
		}
		return pr, nil
	case lex.isKeyword("replace_regexp"):
		pr, err := parsePipeReplaceRegexp(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'replace_regexp' pipe: %w", err)
		}
		return pr, nil
	case lex.isKeyword("sample"):
		ps, err := parsePipeSample(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'sample' pipe: %w", err)
		}
		return ps, nil
	case lex.isKeyword("sort", "order"):
		ps, err := parsePipeSort(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'sort' pipe: %w", err)
		}
		return ps, nil
	case lex.isKeyword("stats", "stats_remote"):
		ps, err := parsePipeStats(lex, true)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'stats' pipe: %w", err)
		}
		return ps, nil
	case lex.isKeyword("stream_context"):
		pc, err := parsePipeStreamContext(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'stream_context' pipe: %w", err)
		}
		return pc, nil
	case lex.isKeyword("top"):
		pt, err := parsePipeTop(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'top' pipe: %w", err)
		}
		return pt, nil
	case lex.isKeyword("union"):
		pu, err := parsePipeUnion(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'union' pipe: %w", err)
		}
		return pu, nil
	case lex.isKeyword("uniq"):
		pu, err := parsePipeUniq(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'uniq' pipe: %w", err)
		}
		return pu, nil
	case lex.isKeyword("unpack_json"):
		pu, err := parsePipeUnpackJSON(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'unpack_json' pipe: %w", err)
		}
		return pu, nil
	case lex.isKeyword("unpack_logfmt"):
		pu, err := parsePipeUnpackLogfmt(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'unpack_logfmt' pipe: %w", err)
		}
		return pu, nil
	case lex.isKeyword("unpack_syslog"):
		pu, err := parsePipeUnpackSyslog(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'unpack_syslog' pipe: %w", err)
		}
		return pu, nil
	case lex.isKeyword("unpack_words"):
		pu, err := parsePipeUnpackWords(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'unpack_words' pipe: %w", err)
		}
		return pu, nil
	case lex.isKeyword("unroll"):
		pu, err := parsePipeUnroll(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'unroll' pipe: %w", err)
		}
		return pu, nil
	default:
		lexState := lex.backupState()

		// Try parsing stats pipe without 'stats' keyword
		ps, err := parsePipeStats(lex, false)
		if err == nil {
			return ps, nil
		}
		lex.restoreState(lexState)

		// Try parsing filter pipe without 'filter' keyword
		pf, err := parsePipeFilter(lex, false)
		if err == nil {
			return pf, nil
		}
		lex.restoreState(lexState)

		return nil, fmt.Errorf("unexpected pipe %q", lex.token)
	}
}

func mustParsePipes(s string, timestamp int64) []pipe {
	lex := newLexer(s, timestamp)
	pipes, err := parsePipes(lex)
	if err != nil {
		logger.Panicf("BUG: cannot parse [%s]: %s", s, err)
	}
	return pipes
}

func mustParsePipe(s string, timestamp int64) pipe {
	lex := newLexer(s, timestamp)
	p, err := parsePipe(lex)
	if err != nil {
		logger.Panicf("BUG: cannot parse [%s]: %s", s, err)
	}
	return p
}

var pipeNames = func() map[string]struct{} {
	a := []string{
		"block_stats",
		"blocks_count",
		"collapse_nums",
		"copy", "cp",
		"decolorize",
		"delete", "del", "rm", "drop",
		"drop_empty_fields",
		"extract",
		"extract_regexp",
		"facets",
		"field_names",
		"field_values",
		"fields", "keep",
		"filter", "where",
		"first",
		"format",
		"join",
		"json_array_len",
		"hash",
		"last",
		"len",
		"limit", "head",
		"math", "eval",
		"offset", "skip",
		"pack_json",
		"pack_logmft",
		"rename", "mv",
		"replace",
		"replace_regexp",
		"sample",
		"sort", "order",
		"stats", "stats_remote", "by",
		"stream_context",
		"top",
		"union",
		"uniq",
		"unpack_json",
		"unpack_logfmt",
		"unpack_syslog",
		"unpack_words",
		"unroll",
	}

	m := make(map[string]struct{}, len(a))
	for _, s := range a {
		m[s] = struct{}{}
	}

	// add stats names here, since they can be used without the initial `stats` keyword
	for _, s := range statsNames {
		m[s] = struct{}{}
	}
	return m
}()
