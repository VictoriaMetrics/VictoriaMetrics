package logstorage

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// pipeExtract processes '| extract from <field> <pattern>' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe
type pipeExtract struct {
	fromField string
	steps     []extractFormatStep

	pattern string
}

func (pe *pipeExtract) String() string {
	s := "extract"
	if !isMsgFieldName(pe.fromField) {
		s += " from " + quoteTokenIfNeeded(pe.fromField)
	}
	s += " " + quoteTokenIfNeeded(pe.pattern)
	return s
}

func (pe *pipeExtract) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		unneededFieldsOrig := unneededFields.clone()
		needFromField := false
		for _, step := range pe.steps {
			if step.field != "" {
				if !unneededFieldsOrig.contains(step.field) {
					needFromField = true
				}
				unneededFields.add(step.field)
			}
		}
		if needFromField {
			unneededFields.remove(pe.fromField)
		} else {
			unneededFields.add(pe.fromField)
		}
	} else {
		needFromField := false
		for _, step := range pe.steps {
			if step.field != "" && neededFields.contains(step.field) {
				needFromField = true
				neededFields.remove(step.field)
			}
		}
		if needFromField {
			neededFields.add(pe.fromField)
		}
	}
}

func (pe *pipeExtract) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	shards := make([]pipeExtractProcessorShard, workersCount)
	for i := range shards {
		ef := newExtractFormat(pe.steps)
		rcs := make([]resultColumn, len(ef.fields))
		for j := range rcs {
			rcs[j].name = ef.fields[j].name
		}
		shards[i] = pipeExtractProcessorShard{
			pipeExtractProcessorShardNopad: pipeExtractProcessorShardNopad{
				ef:  ef,
				rcs: rcs,
			},
		}
	}

	pep := &pipeExtractProcessor{
		pe:     pe,
		ppBase: ppBase,

		shards: shards,
	}
	return pep
}

type pipeExtractProcessor struct {
	pe     *pipeExtract
	ppBase pipeProcessor

	shards []pipeExtractProcessorShard
}

type pipeExtractProcessorShard struct {
	pipeExtractProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeExtractProcessorShardNopad{})%128]byte
}

type pipeExtractProcessorShardNopad struct {
	ef *extractFormat

	rcs []resultColumn
}

func (pep *pipeExtractProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &pep.shards[workerID]
	ef := shard.ef
	rcs := shard.rcs

	c := br.getColumnByName(pep.pe.fromField)
	if c.isConst {
		v := c.valuesEncoded[0]
		ef.apply(v)
		for i, f := range ef.fields {
			fieldValue := *f.value
			rc := &rcs[i]
			for range br.timestamps {
				rc.addValue(fieldValue)
			}
		}
	} else {
		values := c.getValues(br)
		for i, v := range values {
			if i == 0 || values[i-1] != v {
				ef.apply(v)
			}
			for j, f := range ef.fields {
				rcs[j].addValue(*f.value)
			}
		}
	}

	br.addResultColumns(rcs)
	pep.ppBase.writeBlock(workerID, br)

	for i := range rcs {
		rcs[i].resetValues()
	}
}

func (pep *pipeExtractProcessor) flush() error {
	return nil
}

func parsePipeExtract(lex *lexer) (*pipeExtract, error) {
	if !lex.isKeyword("extract") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "extract")
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

	pattern, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot read 'pattern': %w", err)
	}
	steps, err := parseExtractFormatSteps(pattern)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'pattern' %q: %w", pattern, err)
	}

	pe := &pipeExtract{
		fromField: fromField,
		steps:     steps,
		pattern:   pattern,
	}
	return pe, nil
}

type extractFormat struct {
	// steps contains steps for extracting fields from string
	steps []extractFormatStep

	// matches contains matches for every step in steps
	matches []string

	// fields contains matches for non-empty fields
	fields []extractField
}

type extractField struct {
	name  string
	value *string
}

type extractFormatStep struct {
	prefix string
	field  string
}

func newExtractFormat(steps []extractFormatStep) *extractFormat {
	if len(steps) == 0 {
		logger.Panicf("BUG: steps cannot be empty")
	}

	matches := make([]string, len(steps))

	var fields []extractField
	for i, step := range steps {
		if step.field != "" {
			fields = append(fields, extractField{
				name:  step.field,
				value: &matches[i],
			})
		}
	}
	if len(fields) == 0 {
		logger.Panicf("BUG: fields cannot be empty")
	}

	ef := &extractFormat{
		steps:   steps,
		matches: matches,
		fields:  fields,
	}
	return ef
}

func (ef *extractFormat) apply(s string) {
	clear(ef.matches)

	steps := ef.steps

	if prefix := steps[0].prefix; prefix != "" {
		n := strings.Index(s, prefix)
		if n < 0 {
			// Mismatch
			return
		}
		s = s[n+len(prefix):]
	}

	matches := ef.matches
	for i := range steps {
		nextPrefix := ""
		if i+1 < len(steps) {
			nextPrefix = steps[i+1].prefix
		}

		us, nOffset := tryUnquoteString(s)
		if nOffset >= 0 {
			// Matched quoted string
			matches[i] = us
			s = s[nOffset:]
			if !strings.HasPrefix(s, nextPrefix) {
				// Mismatch
				return
			}
			s = s[len(nextPrefix):]
		} else {
			// Match unquoted string until the nextPrefix
			if nextPrefix == "" {
				matches[i] = s
				return
			}
			n := strings.Index(s, nextPrefix)
			if n < 0 {
				// Mismatch
				return
			}
			matches[i] = s[:n]
			s = s[n+len(nextPrefix):]
		}
	}
}

func tryUnquoteString(s string) (string, int) {
	if len(s) == 0 {
		return s, -1
	}
	if s[0] != '"' && s[0] != '`' {
		return s, -1
	}
	qp, err := strconv.QuotedPrefix(s)
	if err != nil {
		return s, -1
	}
	us, err := strconv.Unquote(qp)
	if err != nil {
		return s, -1
	}
	return us, len(qp)
}

func parseExtractFormatSteps(s string) ([]extractFormatStep, error) {
	var steps []extractFormatStep

	hasNamedField := false

	n := strings.IndexByte(s, '<')
	if n < 0 {
		return nil, fmt.Errorf("missing <...> fields")
	}
	prefix := s[:n]
	s = s[n+1:]
	for {
		n := strings.IndexByte(s, '>')
		if n < 0 {
			return nil, fmt.Errorf("missing '>' for <%s", s)
		}
		field := s[:n]
		s = s[n+1:]

		if field == "_" || field == "*" {
			field = ""
		}
		steps = append(steps, extractFormatStep{
			prefix: prefix,
			field:  field,
		})
		if !hasNamedField && field != "" {
			hasNamedField = true
		}
		if len(s) == 0 {
			break
		}

		n = strings.IndexByte(s, '<')
		if n < 0 {
			steps = append(steps, extractFormatStep{
				prefix: s,
			})
			break
		}
		if n == 0 {
			return nil, fmt.Errorf("missing delimiter after <%s>", field)
		}
		prefix = s[:n]
		s = s[n+1:]
	}

	if !hasNamedField {
		return nil, fmt.Errorf("missing named fields like <name>")
	}

	for i := range steps {
		step := &steps[i]
		step.prefix = html.UnescapeString(step.prefix)
	}

	return steps, nil
}
