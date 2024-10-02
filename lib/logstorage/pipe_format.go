package logstorage

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/valyala/quicktemplate"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pipeFormat processes '| format ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#format-pipe
type pipeFormat struct {
	formatStr string
	steps     []patternStep

	resultField string

	keepOriginalFields bool
	skipEmptyResults   bool

	// iff is an optional filter for skipping the format func
	iff *ifFilter
}

func (pf *pipeFormat) String() string {
	s := "format"
	if pf.iff != nil {
		s += " " + pf.iff.String()
	}
	s += " " + quoteTokenIfNeeded(pf.formatStr)
	if !isMsgFieldName(pf.resultField) {
		s += " as " + quoteTokenIfNeeded(pf.resultField)
	}
	if pf.keepOriginalFields {
		s += " keep_original_fields"
	}
	if pf.skipEmptyResults {
		s += " skip_empty_results"
	}
	return s
}

func (pf *pipeFormat) canLiveTail() bool {
	return true
}

func (pf *pipeFormat) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.isEmpty() {
		if pf.iff != nil {
			neededFields.addFields(pf.iff.neededFields)
		}
		return
	}

	if neededFields.contains("*") {
		if !unneededFields.contains(pf.resultField) {
			if !pf.keepOriginalFields && !pf.skipEmptyResults {
				unneededFields.add(pf.resultField)
			}
			if pf.iff != nil {
				unneededFields.removeFields(pf.iff.neededFields)
			}
			for _, step := range pf.steps {
				if step.field != "" {
					unneededFields.remove(step.field)
				}
			}
		}
	} else {
		if neededFields.contains(pf.resultField) {
			if !pf.keepOriginalFields && !pf.skipEmptyResults {
				neededFields.remove(pf.resultField)
			}
			if pf.iff != nil {
				neededFields.addFields(pf.iff.neededFields)
			}
			for _, step := range pf.steps {
				if step.field != "" {
					neededFields.add(step.field)
				}
			}
		}
	}
}

func (pf *pipeFormat) optimize() {
	pf.iff.optimizeFilterIn()
}

func (pf *pipeFormat) hasFilterInWithQuery() bool {
	return pf.iff.hasFilterInWithQuery()
}

func (pf *pipeFormat) initFilterInValues(cache map[string][]string, getFieldValuesFunc getFieldValuesFunc) (pipe, error) {
	iffNew, err := pf.iff.initFilterInValues(cache, getFieldValuesFunc)
	if err != nil {
		return nil, err
	}
	pfNew := *pf
	pfNew.iff = iffNew
	return &pfNew, nil
}

func (pf *pipeFormat) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeFormatProcessor{
		pf:     pf,
		ppNext: ppNext,

		shards: make([]pipeFormatProcessorShard, workersCount),
	}
}

type pipeFormatProcessor struct {
	pf     *pipeFormat
	ppNext pipeProcessor

	shards []pipeFormatProcessorShard
}

type pipeFormatProcessorShard struct {
	pipeFormatProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeFormatProcessorShardNopad{})%128]byte
}

type pipeFormatProcessorShardNopad struct {
	bm bitmap

	a  arena
	rc resultColumn
}

func (pfp *pipeFormatProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := &pfp.shards[workerID]
	pf := pfp.pf

	bm := &shard.bm
	bm.init(br.rowsLen)
	bm.setBits()
	if iff := pf.iff; iff != nil {
		iff.f.applyToBlockResult(br, bm)
		if bm.isZero() {
			pfp.ppNext.writeBlock(workerID, br)
			return
		}
	}

	shard.rc.name = pf.resultField

	resultColumn := br.getColumnByName(pf.resultField)
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		v := ""
		if bm.isSetBit(rowIdx) {
			v = shard.formatRow(pf, br, rowIdx)
			if v == "" && pf.skipEmptyResults || pf.keepOriginalFields {
				if vOrig := resultColumn.getValueAtRow(br, rowIdx); vOrig != "" {
					v = vOrig
				}
			}
		} else {
			v = resultColumn.getValueAtRow(br, rowIdx)
		}
		shard.rc.addValue(v)
	}

	br.addResultColumn(&shard.rc)
	pfp.ppNext.writeBlock(workerID, br)

	shard.a.reset()
	shard.rc.reset()
}

func (pfp *pipeFormatProcessor) flush() error {
	return nil
}

func (shard *pipeFormatProcessorShard) formatRow(pf *pipeFormat, br *blockResult, rowIdx int) string {
	b := shard.a.b
	bLen := len(b)
	for _, step := range pf.steps {
		b = append(b, step.prefix...)
		if step.field != "" {
			c := br.getColumnByName(step.field)
			v := c.getValueAtRow(br, rowIdx)
			switch step.fieldOpt {
			case "q":
				b = quicktemplate.AppendJSONString(b, v, true)
			case "time":
				nsecs, ok := tryParseInt64(v)
				if !ok {
					b = append(b, v...)
					continue
				}
				b = marshalTimestampRFC3339NanoString(b, nsecs)
			case "duration":
				nsecs, ok := tryParseInt64(v)
				if !ok {
					b = append(b, v...)
					continue
				}
				b = marshalDurationString(b, nsecs)
			case "ipv4":
				ipNum, ok := tryParseUint64(v)
				if !ok || ipNum > math.MaxUint32 {
					b = append(b, v...)
					continue
				}
				b = marshalIPv4String(b, uint32(ipNum))
			default:
				b = append(b, v...)
			}
		}
	}
	shard.a.b = b

	return bytesutil.ToUnsafeString(b[bLen:])
}

func parsePipeFormat(lex *lexer) (*pipeFormat, error) {
	if !lex.isKeyword("format") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "format")
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

	// parse format
	formatStr, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot read 'format': %w", err)
	}
	steps, err := parsePatternSteps(formatStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'pattern' %q: %w", formatStr, err)
	}

	// parse optional 'as ...` part
	resultField := "_msg"
	if lex.isKeyword("as") {
		lex.nextToken()
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result field after 'format %q as': %w", formatStr, err)
		}
		resultField = field
	}

	keepOriginalFields := false
	skipEmptyResults := false
	switch {
	case lex.isKeyword("keep_original_fields"):
		lex.nextToken()
		keepOriginalFields = true
	case lex.isKeyword("skip_empty_results"):
		lex.nextToken()
		skipEmptyResults = true
	}

	pf := &pipeFormat{
		formatStr:          formatStr,
		steps:              steps,
		resultField:        resultField,
		keepOriginalFields: keepOriginalFields,
		skipEmptyResults:   skipEmptyResults,
		iff:                iff,
	}

	return pf, nil
}
