package logstorage

import (
	"fmt"
	"slices"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pipePackJSON processes '| pack_json ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#pack_json-pipe
type pipePackJSON struct {
	resultField string

	fields []string
}

func (pp *pipePackJSON) String() string {
	s := "pack_json"
	if len(pp.fields) > 0 {
		s += " fields (" + fieldsToString(pp.fields) + ")"
	}
	if !isMsgFieldName(pp.resultField) {
		s += " as " + quoteTokenIfNeeded(pp.resultField)
	}
	return s
}

func (pp *pipePackJSON) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		if !unneededFields.contains(pp.resultField) {
			if len(pp.fields) > 0 {
				unneededFields.removeFields(pp.fields)
			} else {
				unneededFields.reset()
			}
		}
	} else {
		if neededFields.contains(pp.resultField) {
			if len(pp.fields) > 0 {
				neededFields.addFields(pp.fields)
			} else {
				neededFields.add("*")
			}
		}
	}
}

func (pp *pipePackJSON) optimize() {
	// nothing to do
}

func (pp *pipePackJSON) hasFilterInWithQuery() bool {
	return false
}

func (pp *pipePackJSON) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pp, nil
}

func (pp *pipePackJSON) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipePackJSONProcessor{
		pp:     pp,
		ppNext: ppNext,

		shards: make([]pipePackJSONProcessorShard, workersCount),
	}
}

type pipePackJSONProcessor struct {
	pp     *pipePackJSON
	ppNext pipeProcessor

	shards []pipePackJSONProcessorShard
}

type pipePackJSONProcessorShard struct {
	pipePackJSONProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipePackJSONProcessorShardNopad{})%128]byte
}

type pipePackJSONProcessorShardNopad struct {
	rc resultColumn

	buf    []byte
	fields []Field

	cs []*blockResultColumn
}

func (ppp *pipePackJSONProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &ppp.shards[workerID]

	shard.rc.name = ppp.pp.resultField

	cs := shard.cs[:0]
	if len(ppp.pp.fields) == 0 {
		csAll := br.getColumns()
		cs = append(cs, csAll...)
	} else {
		for _, f := range ppp.pp.fields {
			c := br.getColumnByName(f)
			cs = append(cs, c)
		}
	}
	shard.cs = cs

	buf := shard.buf[:0]
	fields := shard.fields
	for rowIdx := range br.timestamps {
		fields = fields[:0]
		for _, c := range cs {
			v := c.getValueAtRow(br, rowIdx)
			fields = append(fields, Field{
				Name:  c.name,
				Value: v,
			})
		}

		bufLen := len(buf)
		buf = MarshalFieldsToJSON(buf, fields)
		v := bytesutil.ToUnsafeString(buf[bufLen:])
		shard.rc.addValue(v)
	}
	shard.fields = fields

	br.addResultColumn(&shard.rc)
	ppp.ppNext.writeBlock(workerID, br)

	shard.rc.reset()
}

func (ppp *pipePackJSONProcessor) flush() error {
	return nil
}

func parsePackJSON(lex *lexer) (*pipePackJSON, error) {
	if !lex.isKeyword("pack_json") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "pack_json")
	}
	lex.nextToken()

	var fields []string
	if lex.isKeyword("fields") {
		lex.nextToken()
		fs, err := parseFieldNamesInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse fields: %w", err)
		}
		if slices.Contains(fs, "*") {
			fs = nil
		}
		fields = fs
	}

	// parse optional 'as ...` part
	resultField := "_msg"
	if lex.isKeyword("as") {
		lex.nextToken()
	}
	if !lex.isKeyword("|", ")", "") {
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result field for 'pack_json': %w", err)
		}
		resultField = field
	}

	pp := &pipePackJSON{
		resultField: resultField,
		fields:      fields,
	}

	return pp, nil
}
