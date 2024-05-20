package logstorage

import (
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type fieldsUnpackerContext struct {
	fields []Field
	a      arena
}

func (uctx *fieldsUnpackerContext) reset() {
	uctx.resetFields()
	uctx.a.reset()
}

func (uctx *fieldsUnpackerContext) resetFields() {
	clear(uctx.fields)
	uctx.fields = uctx.fields[:0]
}

func (uctx *fieldsUnpackerContext) addField(name, value, fieldPrefix string) {
	nameBuf := uctx.a.newBytes(len(fieldPrefix) + len(name))
	copy(nameBuf, fieldPrefix)
	copy(nameBuf[len(fieldPrefix):], name)
	nameCopy := bytesutil.ToUnsafeString(nameBuf)

	valueCopy := uctx.a.copyString(value)

	uctx.fields = append(uctx.fields, Field{
		Name:  nameCopy,
		Value: valueCopy,
	})
}

func newPipeUnpackProcessor(workersCount int, unpackFunc func(uctx *fieldsUnpackerContext, s, fieldPrefix string), ppBase pipeProcessor, fromField, fieldPrefix string) *pipeUnpackProcessor {
	return &pipeUnpackProcessor{
		unpackFunc: unpackFunc,
		ppBase:     ppBase,

		shards: make([]pipeUnpackProcessorShard, workersCount),

		fromField:   fromField,
		fieldPrefix: fieldPrefix,
	}
}

type pipeUnpackProcessor struct {
	unpackFunc func(uctx *fieldsUnpackerContext, s, fieldPrefix string)
	ppBase     pipeProcessor

	shards []pipeUnpackProcessorShard

	fromField   string
	fieldPrefix string
}

type pipeUnpackProcessorShard struct {
	pipeUnpackProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeUnpackProcessorShardNopad{})%128]byte
}

type pipeUnpackProcessorShardNopad struct {
	uctx fieldsUnpackerContext
	wctx pipeUnpackWriteContext
}

func (pup *pipeUnpackProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &pup.shards[workerID]
	shard.wctx.init(br, pup.ppBase)

	c := br.getColumnByName(pup.fromField)
	if c.isConst {
		v := c.valuesEncoded[0]
		shard.uctx.resetFields()
		pup.unpackFunc(&shard.uctx, v, pup.fieldPrefix)
		for rowIdx := range br.timestamps {
			shard.wctx.writeRow(rowIdx, shard.uctx.fields)
		}
	} else {
		values := c.getValues(br)
		for i, v := range values {
			if i == 0 || values[i-1] != v {
				shard.uctx.resetFields()
				pup.unpackFunc(&shard.uctx, v, pup.fieldPrefix)
			}
			shard.wctx.writeRow(i, shard.uctx.fields)
		}
	}

	shard.wctx.flush()
	shard.uctx.reset()
}

func (pup *pipeUnpackProcessor) flush() error {
	return nil
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
