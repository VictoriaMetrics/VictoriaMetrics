package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeGenerateSequence implements '| generate_sequence' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#generate_sequence-pipe
type pipeGenerateSequence struct {
	// n is the number of rows to generate in the sequence
	n uint64
}

func (pg *pipeGenerateSequence) String() string {
	return fmt.Sprintf("generate_sequence %d", pg.n)
}

func (pg *pipeGenerateSequence) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return nil, []pipe{pg}
}

func (pg *pipeGenerateSequence) canLiveTail() bool {
	return false
}

func (pg *pipeGenerateSequence) canReturnLastNResults() bool {
	return false
}

func (pg *pipeGenerateSequence) updateNeededFields(pf *prefixfilter.Filter) {
	pf.Reset()
}

func (pg *pipeGenerateSequence) hasFilterInWithQuery() bool {
	return false
}

func (pg *pipeGenerateSequence) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pg, nil
}

func (pg *pipeGenerateSequence) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pg *pipeGenerateSequence) newPipeProcessor(_ int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	pgp := &pipeGenerateSequenceProcessor{
		pg:     pg,
		stopCh: stopCh,
		cancel: cancel,
		ppNext: ppNext,
	}
	return pgp
}

type pipeGenerateSequenceProcessor struct {
	pg     *pipeGenerateSequence
	cancel func()
	stopCh <-chan struct{}
	ppNext pipeProcessor
}

func (pgp *pipeGenerateSequenceProcessor) writeBlock(_ uint, _ *blockResult) {
	// Notify the caller it must stop sending new data blocks here.
	// The requested sequence is generated in full in the flush() call.
	pgp.cancel()
}

func (pgp *pipeGenerateSequenceProcessor) flush() error {
	rcs := make([]resultColumn, 1)
	rc := &rcs[0]
	rc.name = "_msg"

	var br blockResult
	var buf []byte

	for i := uint64(0); i < pgp.pg.n; i++ {
		if needStop(pgp.stopCh) {
			return nil
		}

		bufLen := len(buf)
		buf = marshalUint64String(buf, i)
		v := bytesutil.ToUnsafeString(buf[bufLen:])
		rc.addValue(v)

		if len(buf) >= 64*1024-20 {
			// Flush the generated sequence to the next pipe
			br.setResultColumns(rcs, len(rc.values))
			pgp.ppNext.writeBlock(0, &br)
			rc.resetValues()
			buf = buf[:0]
		}
	}

	if len(buf) > 0 {
		br.setResultColumns(rcs, len(rc.values))
		pgp.ppNext.writeBlock(0, &br)
	}

	return nil
}

func parsePipeGenerateSequence(lex *lexer) (pipe, error) {
	if !lex.isKeyword("generate_sequence") {
		return nil, fmt.Errorf("expecting 'generate_sequence'; got %q", lex.token)
	}
	lex.nextToken()

	if !isNumberPrefix(lex.token) {
		return nil, fmt.Errorf("expecting the number of items to generate in 'generate_sequence' pipe; got %q", lex.token)
	}
	nF, s, err := parseNumber(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse N in 'generate_sequence': %w", err)
	}
	if nF < 1 {
		return nil, fmt.Errorf("value N in 'generate_sequence %s' must be integer bigger than 0", s)
	}
	n := uint64(nF)

	pg := &pipeGenerateSequence{
		n: n,
	}
	return pg, nil
}
