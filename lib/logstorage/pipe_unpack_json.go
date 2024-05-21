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
	p JSONParser

	wctx pipeUnpackWriteContext
}

func (shard *pipeUnpackJSONProcessorShard) parseJSON(v, resultPrefix string) []Field {
	if len(v) == 0 || v[0] != '{' {
		// This isn't a JSON object
		return nil
	}
	if err := shard.p.ParseLogMessageNoResetBuf(v, resultPrefix); err != nil {
		// Cannot parse v
		return nil
	}
	return shard.p.Fields
}

func (pup *pipeUnpackJSONProcessor) writeBlock(workerID uint, br *blockResult) {
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
		extraFields := shard.parseJSON(v, resultPrefix)
		for rowIdx := range br.timestamps {
			wctx.writeRow(rowIdx, extraFields)
		}
	} else {
		values := c.getValues(br)
		var extraFields []Field
		for i, v := range values {
			if i == 0 || values[i-1] != v {
				extraFields = shard.parseJSON(v, resultPrefix)
			}
			wctx.writeRow(i, extraFields)
		}
	}

	wctx.flush()
	shard.p.reset()
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
