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
	nameCopy := ""
	if fieldPrefix != "" {
		nameBuf := uctx.a.newBytes(len(fieldPrefix) + len(name))
		copy(nameBuf, fieldPrefix)
		copy(nameBuf[len(fieldPrefix):], name)
		nameCopy = bytesutil.ToUnsafeString(nameBuf)
	} else {
		nameCopy = uctx.a.copyString(name)
	}

	valueCopy := uctx.a.copyString(value)

	uctx.fields = append(uctx.fields, Field{
		Name:  nameCopy,
		Value: valueCopy,
	})
}

func newPipeUnpackProcessor(workersCount int, unpackFunc func(uctx *fieldsUnpackerContext, s, fieldPrefix string), ppBase pipeProcessor,
	fromField, fieldPrefix string, iff *ifFilter) *pipeUnpackProcessor {
	return &pipeUnpackProcessor{
		unpackFunc: unpackFunc,
		ppBase:     ppBase,

		shards: make([]pipeUnpackProcessorShard, workersCount),

		fromField:   fromField,
		fieldPrefix: fieldPrefix,
		iff:         iff,
	}
}

type pipeUnpackProcessor struct {
	unpackFunc func(uctx *fieldsUnpackerContext, s, fieldPrefix string)
	ppBase     pipeProcessor

	shards []pipeUnpackProcessorShard

	fromField   string
	fieldPrefix string

	iff *ifFilter
}

type pipeUnpackProcessorShard struct {
	pipeUnpackProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeUnpackProcessorShardNopad{})%128]byte
}

type pipeUnpackProcessorShardNopad struct {
	bm bitmap

	uctx fieldsUnpackerContext
	wctx pipeUnpackWriteContext
}

func (pup *pipeUnpackProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &pup.shards[workerID]
	shard.wctx.init(br, pup.ppBase)

	bm := &shard.bm
	bm.init(len(br.timestamps))
	bm.setBits()
	if pup.iff != nil {
		pup.iff.f.applyToBlockResult(br, bm)
		if bm.isZero() {
			pup.ppBase.writeBlock(workerID, br)
			return
		}
	}

	c := br.getColumnByName(pup.fromField)
	if c.isConst {
		v := c.valuesEncoded[0]
		shard.uctx.resetFields()
		pup.unpackFunc(&shard.uctx, v, pup.fieldPrefix)
		for rowIdx := range br.timestamps {
			if bm.isSetBit(rowIdx) {
				shard.wctx.writeRow(rowIdx, shard.uctx.fields)
			} else {
				shard.wctx.writeRow(rowIdx, nil)
			}
		}
	} else {
		values := c.getValues(br)
		vPrevApplied := ""
		for i, v := range values {
			if bm.isSetBit(i) {
				if vPrevApplied != v {
					shard.uctx.resetFields()
					pup.unpackFunc(&shard.uctx, v, pup.fieldPrefix)
					vPrevApplied = v
				}
				shard.wctx.writeRow(i, shard.uctx.fields)
			} else {
				shard.wctx.writeRow(i, nil)
			}
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

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
}

func (wctx *pipeUnpackWriteContext) reset() {
	wctx.brSrc = nil
	wctx.csSrc = nil
	wctx.ppBase = nil

	rcs := wctx.rcs
	for i := range rcs {
		rcs[i].reset()
	}
	wctx.rcs = rcs[:0]

	wctx.rowsCount = 0
	wctx.valuesLen = 0
}

func (wctx *pipeUnpackWriteContext) init(brSrc *blockResult, ppBase pipeProcessor) {
	wctx.reset()

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
		// send the current block to ppBase and construct a block with new set of columns
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

	wctx.rowsCount++
	if wctx.valuesLen >= 1_000_000 {
		wctx.flush()
	}
}

func (wctx *pipeUnpackWriteContext) flush() {
	rcs := wctx.rcs

	wctx.valuesLen = 0

	// Flush rcs to ppBase
	br := &wctx.br
	br.setResultColumns(rcs, wctx.rowsCount)
	wctx.rowsCount = 0
	wctx.ppBase.writeBlock(0, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetValues()
	}
}
