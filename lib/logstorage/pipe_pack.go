package logstorage

import (
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

func updateNeededFieldsForPipePack(neededFields, unneededFields fieldsSet, resultField string, fields []string) {
	if neededFields.contains("*") {
		if !unneededFields.contains(resultField) {
			if len(fields) > 0 {
				unneededFields.removeFields(fields)
			} else {
				unneededFields.reset()
			}
		}
	} else {
		if neededFields.contains(resultField) {
			neededFields.remove(resultField)
			if len(fields) > 0 {
				neededFields.addFields(fields)
			} else {
				neededFields.add("*")
			}
		}
	}
}

func newPipePackProcessor(workersCount int, ppNext pipeProcessor, resultField string, fields []string, marshalFields func(dst []byte, fields []Field) []byte) pipeProcessor {
	return &pipePackProcessor{
		ppNext:        ppNext,
		resultField:   resultField,
		fields:        fields,
		marshalFields: marshalFields,

		shards: make([]pipePackProcessorShard, workersCount),
	}
}

type pipePackProcessor struct {
	ppNext        pipeProcessor
	resultField   string
	fields        []string
	marshalFields func(dst []byte, fields []Field) []byte

	shards []pipePackProcessorShard
}

type pipePackProcessorShard struct {
	pipePackProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipePackProcessorShardNopad{})%128]byte
}

type pipePackProcessorShardNopad struct {
	rc resultColumn

	buf    []byte
	fields []Field

	cs []*blockResultColumn
}

func (ppp *pipePackProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := &ppp.shards[workerID]

	shard.rc.name = ppp.resultField

	cs := shard.cs[:0]
	if len(ppp.fields) == 0 {
		csAll := br.getColumns()
		cs = append(cs, csAll...)
	} else {
		for _, f := range ppp.fields {
			c := br.getColumnByName(f)
			cs = append(cs, c)
		}
	}
	shard.cs = cs

	buf := shard.buf[:0]
	fields := shard.fields
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		fields = fields[:0]
		for _, c := range cs {
			v := c.getValueAtRow(br, rowIdx)
			fields = append(fields, Field{
				Name:  c.name,
				Value: v,
			})
		}

		bufLen := len(buf)
		buf = ppp.marshalFields(buf, fields)
		v := bytesutil.ToUnsafeString(buf[bufLen:])
		shard.rc.addValue(v)
	}
	shard.fields = fields

	br.addResultColumn(&shard.rc)
	ppp.ppNext.writeBlock(workerID, br)

	shard.rc.reset()
}

func (ppp *pipePackProcessor) flush() error {
	return nil
}
