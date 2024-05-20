package logstorage

import (
	"fmt"
	"unsafe"
)

// pipeUnpackJSON processes '| unpack_json ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_json-pipe
type pipeUnpackJSON struct {
	fromField string

	resultPrefix string
}

func (pu *pipeUnpackJSON) String() string {
	s := "unpack_json"
	if !isMsgFieldName(pu.fromField) {
		s += " from " + quoteTokenIfNeeded(pu.fromField)
	}
	if pu.resultPrefix != "" {
		s += " result_prefix " + quoteTokenIfNeeded(pu.resultPrefix)
	}
	return s
}

func (pu *pipeUnpackJSON) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		unneededFields.remove(pu.fromField)
	} else {
		neededFields.add(pu.fromField)
	}
}

func (pu *pipeUnpackJSON) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	shards := make([]pipeUnpackJSONProcessorShard, workersCount)

	pup := &pipeUnpackJSONProcessor{
		pu:     pu,
		ppBase: ppBase,

		shards: shards,
	}
	return pup
}

type pipeUnpackJSONProcessor struct {
	pu     *pipeUnpackJSON
	ppBase pipeProcessor

	shards []pipeUnpackJSONProcessorShard
}

type pipeUnpackJSONProcessorShard struct {
	pipeUnpackJSONProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeUnpackJSONProcessorShardNopad{})%128]byte
}

type pipeUnpackJSONProcessorShardNopad struct {
	jsonParser JSONParser

	rcs []resultColumn
	br  blockResult

	valuesLen int
}

func (shard *pipeUnpackJSONProcessorShard) writeRow(ppBase pipeProcessor, br *blockResult, cs []*blockResultColumn, rowIdx int, extraFields []Field) {
	rcs := shard.rcs

	areEqualColumns := len(rcs) == len(cs)+len(extraFields)
	if areEqualColumns {
		for i, f := range extraFields {
			if rcs[len(cs)+i].name != f.Name {
				areEqualColumns = false
				break
			}
		}
	}
	if !areEqualColumns {
		// send the current block to bbBase and construct a block with new set of columns
		shard.flush(ppBase)

		rcs = shard.rcs[:0]
		for _, c := range cs {
			rcs = appendResultColumnWithName(rcs, c.name)
		}
		for _, f := range extraFields {
			rcs = appendResultColumnWithName(rcs, f.Name)
		}
		shard.rcs = rcs
	}

	for i, c := range cs {
		v := c.getValueAtRow(br, rowIdx)
		rcs[i].addValue(v)
		shard.valuesLen += len(v)
	}
	for i, f := range extraFields {
		v := f.Value
		rcs[len(cs)+i].addValue(v)
		shard.valuesLen += len(v)
	}
	if shard.valuesLen >= 1_000_000 {
		shard.flush(ppBase)
	}
}

func (shard *pipeUnpackJSONProcessorShard) flush(ppBase pipeProcessor) {
	rcs := shard.rcs

	shard.valuesLen = 0

	if len(rcs) == 0 {
		return
	}

	// Flush rcs to ppBase
	br := &shard.br
	br.setResultColumns(rcs)
	ppBase.writeBlock(0, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetValues()
	}
}

func (shard *pipeUnpackJSONProcessorShard) parseJSON(v, resultPrefix string) []Field {
	if len(v) == 0 || v[0] != '{' {
		// This isn't a JSON object
		return nil
	}
	if err := shard.jsonParser.ParseLogMessageNoResetBuf(v, resultPrefix); err != nil {
		// Cannot parse v
		return nil
	}
	return shard.jsonParser.Fields
}

func (pup *pipeUnpackJSONProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	resultPrefix := pup.pu.resultPrefix
	shard := &pup.shards[workerID]

	cs := br.getColumns()
	c := br.getColumnByName(pup.pu.fromField)
	if c.isConst {
		v := c.valuesEncoded[0]
		extraFields := shard.parseJSON(v, resultPrefix)
		for rowIdx := range br.timestamps {
			shard.writeRow(pup.ppBase, br, cs, rowIdx, extraFields)
		}
	} else {
		values := c.getValues(br)
		var extraFields []Field
		for i, v := range values {
			if i == 0 || values[i-1] != v {
				extraFields = shard.parseJSON(v, resultPrefix)
			}
			shard.writeRow(pup.ppBase, br, cs, i, extraFields)
		}
	}

	shard.flush(pup.ppBase)
	shard.jsonParser.reset()
}

func (pup *pipeUnpackJSONProcessor) flush() error {
	return nil
}

func parsePipeUnpackJSON(lex *lexer) (*pipeUnpackJSON, error) {
	if !lex.isKeyword("unpack_json") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "unpack_json")
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

	pu := &pipeUnpackJSON{
		fromField:    fromField,
		resultPrefix: resultPrefix,
	}
	return pu, nil
}
