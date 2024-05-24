package logstorage

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pipeReplace processes '| replace ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#replace-pipe
type pipeReplace struct {
	srcField  string
	oldSubstr string
	newSubstr string

	// limit limits the number of replacements, which can be performed
	limit uint64

	// iff is an optional filter for skipping the replace operation
	iff *ifFilter
}

func (pr *pipeReplace) String() string {
	s := "replace"
	if pr.iff != nil {
		s += " " + pr.iff.String()
	}
	s += fmt.Sprintf(" (%s, %s)", quoteTokenIfNeeded(pr.oldSubstr), quoteTokenIfNeeded(pr.newSubstr))
	if pr.srcField != "_msg" {
		s += " at " + quoteTokenIfNeeded(pr.srcField)
	}
	if pr.limit > 0 {
		s += fmt.Sprintf(" limit %d", pr.limit)
	}
	return s
}

func (pr *pipeReplace) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		if !unneededFields.contains(pr.srcField) && pr.iff != nil {
			unneededFields.removeFields(pr.iff.neededFields)
		}
	} else {
		if neededFields.contains(pr.srcField) && pr.iff != nil {
			neededFields.addFields(pr.iff.neededFields)
		}
	}
}

func (pr *pipeReplace) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &pipeReplaceProcessor{
		pr:     pr,
		ppBase: ppBase,

		shards: make([]pipeReplaceProcessorShard, workersCount),
	}
}

type pipeReplaceProcessor struct {
	pr     *pipeReplace
	ppBase pipeProcessor

	shards []pipeReplaceProcessorShard
}

type pipeReplaceProcessorShard struct {
	pipeReplaceProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeReplaceProcessorShardNopad{})%128]byte
}

type pipeReplaceProcessorShardNopad struct {
	bm bitmap

	uctx fieldsUnpackerContext
	wctx pipeUnpackWriteContext
}

func (prp *pipeReplaceProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &prp.shards[workerID]
	shard.wctx.init(workerID, prp.ppBase, false, false, br)
	shard.uctx.init(workerID, "")

	pr := prp.pr
	bm := &shard.bm
	bm.init(len(br.timestamps))
	bm.setBits()
	if iff := pr.iff; iff != nil {
		iff.f.applyToBlockResult(br, bm)
		if bm.isZero() {
			prp.ppBase.writeBlock(workerID, br)
			return
		}
	}

	c := br.getColumnByName(pr.srcField)
	values := c.getValues(br)

	bb := bbPool.Get()
	vPrev := ""
	shard.uctx.addField(pr.srcField, "")
	for rowIdx, v := range values {
		if bm.isSetBit(rowIdx) {
			if vPrev != v {
				bb.B = appendReplace(bb.B[:0], v, pr.oldSubstr, pr.newSubstr, pr.limit)
				s := bytesutil.ToUnsafeString(bb.B)
				shard.uctx.resetFields()
				shard.uctx.addField(pr.srcField, s)
				vPrev = v
			}
			shard.wctx.writeRow(rowIdx, shard.uctx.fields)
		} else {
			shard.wctx.writeRow(rowIdx, nil)
		}
	}
	bbPool.Put(bb)

	shard.wctx.flush()
	shard.wctx.reset()
	shard.uctx.reset()
}

func (prp *pipeReplaceProcessor) flush() error {
	return nil
}

func parsePipeReplace(lex *lexer) (*pipeReplace, error) {
	if !lex.isKeyword("replace") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "replace")
	}
	lex.nextToken()

	// parse optional if (...)
	var iff *ifFilter
	if lex.isKeyword("if") {
		f, err := parseIfFilter(lex)
		if err != nil {
			return nil, err
		}
		iff = f
	}

	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing '(' after 'replace'")
	}
	lex.nextToken()

	oldSubstr, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse oldSubstr in 'replace': %w", err)
	}
	if !lex.isKeyword(",") {
		return nil, fmt.Errorf("missing ',' after 'replace(%q'", oldSubstr)
	}
	lex.nextToken()

	newSubstr, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse newSubstr in 'replace': %w", err)
	}

	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("missing ')' after 'replace(%q, %q'", oldSubstr, newSubstr)
	}
	lex.nextToken()

	srcField := "_msg"
	if lex.isKeyword("at") {
		lex.nextToken()
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'at' field after 'replace(%q, %q)': %w", oldSubstr, newSubstr, err)
		}
		srcField = f
	}

	limit := uint64(0)
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s' in 'replace'", lex.token)
		}
		lex.nextToken()
		limit = n
	}

	pr := &pipeReplace{
		srcField:  srcField,
		oldSubstr: oldSubstr,
		newSubstr: newSubstr,
		limit:     limit,
		iff:       iff,
	}

	return pr, nil
}

func appendReplace(dst []byte, s, oldSubstr, newSubstr string, limit uint64) []byte {
	if len(s) == 0 {
		return dst
	}
	if len(oldSubstr) == 0 {
		return append(dst, s...)
	}

	replacements := uint64(0)
	for {
		n := strings.Index(s, oldSubstr)
		if n < 0 {
			return append(dst, s...)
		}
		dst = append(dst, s[:n]...)
		dst = append(dst, newSubstr...)
		s = s[n+len(oldSubstr):]
		replacements++
		if limit > 0 && replacements >= limit {
			return append(dst, s...)
		}
	}
}
