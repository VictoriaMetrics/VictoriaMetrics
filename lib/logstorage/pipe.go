package logstorage

import (
	"fmt"
)

type pipe interface {
	// String returns string representation of the pipe.
	String() string

	// updateNeededFields must update neededFields and unneededFields with fields it needs and not needs at the input.
	updateNeededFields(neededFields, unneededFields fieldsSet)

	// newPipeProcessor must return new pipeProcessor for the given ppBase.
	//
	// workersCount is the number of goroutine workers, which will call writeBlock() method.
	//
	// If stopCh is closed, the returned pipeProcessor must stop performing CPU-intensive tasks which take more than a few milliseconds.
	// It is OK to continue processing pipeProcessor calls if they take less than a few milliseconds.
	//
	// The returned pipeProcessor may call cancel() at any time in order to notify worker goroutines to stop sending new data to pipeProcessor.
	newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor
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

	// flush must flush all the data accumulated in the pipeProcessor to the base pipeProcessor.
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
	for !lex.isKeyword(")", "") {
		if !lex.isKeyword("|") {
			return nil, fmt.Errorf("expecting '|'")
		}
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing token after '|'")
		}
		switch {
		case lex.isKeyword("stats"):
			ps, err := parsePipeStats(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'stats' pipe: %w", err)
			}
			pipes = append(pipes, ps)
		case lex.isKeyword("sort"):
			ps, err := parsePipeSort(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'sort' pipe: %w", err)
			}
			pipes = append(pipes, ps)
		case lex.isKeyword("uniq"):
			pu, err := parsePipeUniq(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'uniq' pipe: %w", err)
			}
			pipes = append(pipes, pu)
		case lex.isKeyword("limit", "head"):
			pl, err := parsePipeLimit(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'limit' pipe: %w", err)
			}
			pipes = append(pipes, pl)
		case lex.isKeyword("offset", "skip"):
			ps, err := parsePipeOffset(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'offset' pipe: %w", err)
			}
			pipes = append(pipes, ps)
		case lex.isKeyword("fields"):
			pf, err := parsePipeFields(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'fields' pipe: %w", err)
			}
			pipes = append(pipes, pf)
		case lex.isKeyword("copy", "cp"):
			pc, err := parsePipeCopy(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'copy' pipe: %w", err)
			}
			pipes = append(pipes, pc)
		case lex.isKeyword("rename", "mv"):
			pr, err := parsePipeRename(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'rename' pipe: %w", err)
			}
			pipes = append(pipes, pr)
		case lex.isKeyword("delete", "del", "rm"):
			pd, err := parsePipeDelete(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'delete' pipe: %w", err)
			}
			pipes = append(pipes, pd)
		default:
			return nil, fmt.Errorf("unexpected pipe %q", lex.token)
		}
	}
	return pipes, nil
}
