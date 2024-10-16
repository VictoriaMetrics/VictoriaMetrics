package logstorage

import (
	"fmt"
)

type pipe interface {
	// String returns string representation of the pipe.
	String() string

	// canLiveTail must return true if the given pipe can be used in live tailing
	//
	// See https://docs.victoriametrics.com/victorialogs/querying/#live-tailing
	canLiveTail() bool

	// updateNeededFields must update neededFields and unneededFields with fields it needs and not needs at the input.
	updateNeededFields(neededFields, unneededFields fieldsSet)

	// newPipeProcessor must return new pipeProcessor, which writes data to the given ppNext.
	//
	// workersCount is the number of goroutine workers, which will call writeBlock() method.
	//
	// If stopCh is closed, the returned pipeProcessor must stop performing CPU-intensive tasks which take more than a few milliseconds.
	// It is OK to continue processing pipeProcessor calls if they take less than a few milliseconds.
	//
	// The returned pipeProcessor may call cancel() at any time in order to notify the caller to stop sending new data to it.
	newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor

	// optimize must optimize the pipe
	optimize()

	// hasFilterInWithQuery must return true of pipe contains 'in(subquery)' filter (recursively).
	hasFilterInWithQuery() bool

	// initFilterInValues must return new pipe with the initialized values for 'in(subquery)' filters (recursively).
	//
	// It is OK to return the pipe itself if it doesn't contain 'in(subquery)' filters.
	initFilterInValues(cache map[string][]string, getFieldValuesFunc getFieldValuesFunc) (pipe, error)
}

// pipeProcessor must process a single pipe.
type pipeProcessor interface {
	// writeBlock must write the given block of data to the given pipeProcessor.
	//
	// writeBlock is called concurrently from worker goroutines.
	// The workerID is the id of the worker goroutine, which calls the writeBlock.
	// It is in the range 0 ... workersCount-1 .
	//
	// It is OK to modify br contents inside writeBlock. The caller mustn't rely on br contents after writeBlock call.
	// It is forbidden to hold references to br after returning from writeBlock, since the caller may re-use it.
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

type defaultPipeProcessor func(workerID uint, br *blockResult)

func newDefaultPipeProcessor(writeBlock func(workerID uint, br *blockResult)) pipeProcessor {
	return defaultPipeProcessor(writeBlock)
}

func (dpp defaultPipeProcessor) writeBlock(workerID uint, br *blockResult) {
	dpp(workerID, br)
}

func (dpp defaultPipeProcessor) flush() error {
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
	case lex.isKeyword("blocks_count"):
		pc, err := parsePipeBlocksCount(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'blocks_count' pipe: %w", err)
		}
		return pc, nil
	case lex.isKeyword("copy", "cp"):
		pc, err := parsePipeCopy(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'copy' pipe: %w", err)
		}
		return pc, nil
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
	case lex.isKeyword("field_names"):
		pf, err := parsePipeFieldNames(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'field_names' pipe: %w", err)
		}
		return pf, nil
	case lex.isKeyword("field_values"):
		pf, err := parsePipeFieldValues(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot pase 'field_values' pipe: %w", err)
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
	case lex.isKeyword("format"):
		pf, err := parsePipeFormat(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'format' pipe: %w", err)
		}
		return pf, nil
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
		pp, err := parsePackJSON(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'pack_json' pipe: %w", err)
		}
		return pp, nil
	case lex.isKeyword("pack_logfmt"):
		pp, err := parsePackLogfmt(lex)
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
	case lex.isKeyword("sort"), lex.isKeyword("order"):
		ps, err := parsePipeSort(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'sort' pipe: %w", err)
		}
		return ps, nil
	case lex.isKeyword("stats"):
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

var pipeNames = func() map[string]struct{} {
	a := []string{
		"blocks_count",
		"copy", "cp",
		"delete", "del", "rm", "drop",
		"drop_empty_fields",
		"extract",
		"extract_regexp",
		"field_names",
		"field_values",
		"fields", "keep",
		"filter", "where",
		"format",
		"len",
		"limit", "head",
		"math", "eval",
		"offset", "skip",
		"pack_json",
		"pack_logmft",
		"rename", "mv",
		"replace",
		"replace_regexp",
		"sort", "order",
		"stats", "by",
		"stream_context",
		"top",
		"uniq",
		"unpack_json",
		"unpack_logfmt",
		"unpack_syslog",
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
