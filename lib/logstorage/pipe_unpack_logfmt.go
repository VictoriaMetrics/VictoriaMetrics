package logstorage

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pipeUnpackLogfmt processes '| unpack_logfmt ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_logfmt-pipe
type pipeUnpackLogfmt struct {
	fromField string

	resultPrefix string
}

func (pu *pipeUnpackLogfmt) String() string {
	s := "unpack_logfmt"
	if !isMsgFieldName(pu.fromField) {
		s += " from " + quoteTokenIfNeeded(pu.fromField)
	}
	if pu.resultPrefix != "" {
		s += " result_prefix " + quoteTokenIfNeeded(pu.resultPrefix)
	}
	return s
}

func (pu *pipeUnpackLogfmt) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		unneededFields.remove(pu.fromField)
	} else {
		neededFields.add(pu.fromField)
	}
}

func (pu *pipeUnpackLogfmt) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	shards := make([]pipeUnpackLogfmtProcessorShard, workersCount)

	pup := &pipeUnpackLogfmtProcessor{
		pu:     pu,
		ppBase: ppBase,

		shards: shards,
	}
	return pup
}

type pipeUnpackLogfmtProcessor struct {
	pu     *pipeUnpackLogfmt
	ppBase pipeProcessor

	shards []pipeUnpackLogfmtProcessorShard
}

type pipeUnpackLogfmtProcessorShard struct {
	pipeUnpackLogfmtProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeUnpackLogfmtProcessorShardNopad{})%128]byte
}

type pipeUnpackLogfmtProcessorShardNopad struct {
	p logfmtParser

	wctx pipeUnpackWriteContext
}

func (pup *pipeUnpackLogfmtProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	resultPrefix := pup.pu.resultPrefix
	shard := &pup.shards[workerID]
	wctx := &shard.wctx
	wctx.init(br, pup.ppBase)

	c := br.getColumnByName(pup.pu.fromField)
	if c.isConst {
		v := c.valuesEncoded[0]
		extraFields := shard.p.parse(v, resultPrefix)
		for rowIdx := range br.timestamps {
			wctx.writeRow(rowIdx, extraFields)
		}
	} else {
		values := c.getValues(br)
		var extraFields []Field
		for i, v := range values {
			if i == 0 || values[i-1] != v {
				extraFields = shard.p.parse(v, resultPrefix)
			}
			wctx.writeRow(i, extraFields)
		}
	}

	wctx.flush()
	shard.p.reset()
}

func (pup *pipeUnpackLogfmtProcessor) flush() error {
	return nil
}

func parsePipeUnpackLogfmt(lex *lexer) (*pipeUnpackLogfmt, error) {
	if !lex.isKeyword("unpack_logfmt") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "unpack_logfmt")
	}
	lex.nextToken()

	fromField := "_msg"
	if lex.isKeyword("from") {
		lex.nextToken()
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'from' field name: %w", err)
		}
		fromField = f
	}

	resultPrefix := ""
	if lex.isKeyword("result_prefix") {
		lex.nextToken()
		p, err := getCompoundToken(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'result_prefix': %w", err)
		}
		resultPrefix = p
	}

	pu := &pipeUnpackLogfmt{
		fromField:    fromField,
		resultPrefix: resultPrefix,
	}
	return pu, nil
}

type pipeUnpackWriteContext struct {
	brSrc  *blockResult
	csSrc  []*blockResultColumn
	ppBase pipeProcessor

	rcs []resultColumn
	br  blockResult

	valuesLen int
}

func (wctx *pipeUnpackWriteContext) init(brSrc *blockResult, ppBase pipeProcessor) {
	wctx.brSrc = brSrc
	wctx.csSrc = brSrc.getColumns()
	wctx.ppBase = ppBase
}

func (wctx *pipeUnpackWriteContext) writeRow(rowIdx int, extraFields []Field) {
	csSrc := wctx.csSrc
	rcs := wctx.rcs

	areEqualColumns := len(rcs) == len(csSrc)+len(extraFields)
	if areEqualColumns {
		for i, f := range extraFields {
			if rcs[len(csSrc)+i].name != f.Name {
				areEqualColumns = false
				break
			}
		}
	}
	if !areEqualColumns {
		// send the current block to bbBase and construct a block with new set of columns
		wctx.flush()

		rcs = wctx.rcs[:0]
		for _, c := range csSrc {
			rcs = appendResultColumnWithName(rcs, c.name)
		}
		for _, f := range extraFields {
			rcs = appendResultColumnWithName(rcs, f.Name)
		}
		wctx.rcs = rcs
	}

	brSrc := wctx.brSrc
	for i, c := range csSrc {
		v := c.getValueAtRow(brSrc, rowIdx)
		rcs[i].addValue(v)
		wctx.valuesLen += len(v)
	}
	for i, f := range extraFields {
		v := f.Value
		rcs[len(csSrc)+i].addValue(v)
		wctx.valuesLen += len(v)
	}
	if wctx.valuesLen >= 1_000_000 {
		wctx.flush()
	}
}

func (wctx *pipeUnpackWriteContext) flush() {
	rcs := wctx.rcs

	wctx.valuesLen = 0

	if len(rcs) == 0 {
		return
	}

	// Flush rcs to ppBase
	br := &wctx.br
	br.setResultColumns(rcs)
	wctx.ppBase.writeBlock(0, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetValues()
	}
}

type logfmtParser struct {
	Fields []Field

	buf []byte
}

func (p *logfmtParser) reset() {
	clear(p.Fields)
	p.Fields = p.Fields[:0]

	p.buf = p.buf[:0]
}

func (p *logfmtParser) parse(s, resultPrefix string) []Field {
	clear(p.Fields)
	p.Fields = p.Fields[:0]

	for {
		// Search for field name
		n := strings.IndexByte(s, '=')
		if n < 0 {
			// field name couldn't be read
			return p.Fields
		}

		name := strings.TrimSpace(s[:n])
		s = s[n+1:]
		if len(s) == 0 {
			p.addField(name, "", resultPrefix)
			return p.Fields
		}

		// Search for field value
		value, nOffset := tryUnquoteString(s)
		if nOffset >= 0 {
			p.addField(name, value, resultPrefix)
			s = s[nOffset:]
			if len(s) == 0 {
				return p.Fields
			}
			if s[0] != ' ' {
				return p.Fields
			}
			s = s[1:]
		} else {
			n := strings.IndexByte(s, ' ')
			if n < 0 {
				p.addField(name, s, resultPrefix)
				return p.Fields
			}
			p.addField(name, s[:n], resultPrefix)
			s = s[n+1:]
		}
	}
}

func (p *logfmtParser) addField(name, value, resultPrefix string) {
	if resultPrefix != "" {
		buf := p.buf
		bufLen := len(buf)
		buf = append(buf, resultPrefix...)
		buf = append(buf, name...)
		p.buf = buf

		name = bytesutil.ToUnsafeString(buf[bufLen:])
	}
	p.Fields = append(p.Fields, Field{
		Name:  name,
		Value: value,
	})
}
