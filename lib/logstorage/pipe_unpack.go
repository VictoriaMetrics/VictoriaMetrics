package logstorage

import (
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

func updateNeededFieldsForUnpackPipe(fromField string, outFields []string, keepOriginalFields, skipEmptyResults bool, iff *ifFilter, neededFields, unneededFields fieldsSet) {
	if neededFields.isEmpty() {
		if iff != nil {
			neededFields.addFields(iff.neededFields)
		}
		return
	}

	if neededFields.contains("*") {
		unneededFieldsOrig := unneededFields.clone()
		unneededFieldsCount := 0
		if len(outFields) > 0 {
			for _, f := range outFields {
				if unneededFieldsOrig.contains(f) {
					unneededFieldsCount++
				}
				if !keepOriginalFields && !skipEmptyResults {
					unneededFields.add(f)
				}
			}
		}
		if len(outFields) == 0 || unneededFieldsCount < len(outFields) {
			unneededFields.remove(fromField)
			if iff != nil {
				unneededFields.removeFields(iff.neededFields)
			}
		}
	} else {
		neededFieldsOrig := neededFields.clone()
		needFromField := len(outFields) == 0
		if len(outFields) > 0 {
			needFromField = false
			for _, f := range outFields {
				if neededFieldsOrig.contains(f) {
					needFromField = true
				}
				if !keepOriginalFields && !skipEmptyResults {
					neededFields.remove(f)
				}
			}
		}
		if needFromField {
			neededFields.add(fromField)
			if iff != nil {
				neededFields.addFields(iff.neededFields)
			}
		}
	}
}

type fieldsUnpackerContext struct {
	fieldPrefix string

	fields []Field
	a      arena
}

func (uctx *fieldsUnpackerContext) reset() {
	uctx.fieldPrefix = ""
	uctx.resetFields()
	uctx.a.reset()
}

func (uctx *fieldsUnpackerContext) resetFields() {
	clear(uctx.fields)
	uctx.fields = uctx.fields[:0]
}

func (uctx *fieldsUnpackerContext) init(fieldPrefix string) {
	uctx.reset()

	uctx.fieldPrefix = fieldPrefix
}

func (uctx *fieldsUnpackerContext) addField(name, value string) {
	nameCopy := ""
	fieldPrefix := uctx.fieldPrefix
	if fieldPrefix != "" {
		b := uctx.a.b
		bLen := len(b)
		b = append(b, fieldPrefix...)
		b = append(b, name...)
		uctx.a.b = b
		nameCopy = bytesutil.ToUnsafeString(b[bLen:])
	} else {
		nameCopy = uctx.a.copyString(name)
	}

	valueCopy := uctx.a.copyString(value)

	uctx.fields = append(uctx.fields, Field{
		Name:  nameCopy,
		Value: valueCopy,
	})
}

func newPipeUnpackProcessor(workersCount int, unpackFunc func(uctx *fieldsUnpackerContext, s string), ppNext pipeProcessor,
	fromField string, fieldPrefix string, keepOriginalFields, skipEmptyResults bool, iff *ifFilter) *pipeUnpackProcessor {

	return &pipeUnpackProcessor{
		unpackFunc: unpackFunc,
		ppNext:     ppNext,

		shards: make([]pipeUnpackProcessorShard, workersCount),

		fromField:          fromField,
		fieldPrefix:        fieldPrefix,
		keepOriginalFields: keepOriginalFields,
		skipEmptyResults:   skipEmptyResults,
		iff:                iff,
	}
}

type pipeUnpackProcessor struct {
	unpackFunc func(uctx *fieldsUnpackerContext, s string)
	ppNext     pipeProcessor

	shards []pipeUnpackProcessorShard

	fromField          string
	fieldPrefix        string
	keepOriginalFields bool
	skipEmptyResults   bool

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
	if br.rowsLen == 0 {
		return
	}

	shard := &pup.shards[workerID]
	shard.wctx.init(workerID, pup.ppNext, pup.keepOriginalFields, pup.skipEmptyResults, br)
	shard.uctx.init(pup.fieldPrefix)

	bm := &shard.bm
	bm.init(br.rowsLen)
	bm.setBits()
	if pup.iff != nil {
		pup.iff.f.applyToBlockResult(br, bm)
		if bm.isZero() {
			pup.ppNext.writeBlock(workerID, br)
			return
		}
	}

	c := br.getColumnByName(pup.fromField)
	if c.isConst {
		v := c.valuesEncoded[0]
		shard.uctx.resetFields()
		pup.unpackFunc(&shard.uctx, v)
		for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
			if bm.isSetBit(rowIdx) {
				shard.wctx.writeRow(rowIdx, shard.uctx.fields)
			} else {
				shard.wctx.writeRow(rowIdx, nil)
			}
		}
	} else {
		values := c.getValues(br)
		vPrev := ""
		hadUnpacks := false
		for i, v := range values {
			if bm.isSetBit(i) {
				if !hadUnpacks || vPrev != v {
					vPrev = v
					hadUnpacks = true

					shard.uctx.resetFields()
					pup.unpackFunc(&shard.uctx, v)
				}
				shard.wctx.writeRow(i, shard.uctx.fields)
			} else {
				shard.wctx.writeRow(i, nil)
			}
		}
	}

	shard.wctx.flush()
	shard.wctx.reset()
	shard.uctx.reset()
}

func (pup *pipeUnpackProcessor) flush() error {
	return nil
}

type pipeUnpackWriteContext struct {
	workerID           uint
	ppNext             pipeProcessor
	keepOriginalFields bool
	skipEmptyResults   bool

	brSrc *blockResult
	csSrc []*blockResultColumn

	rcs []resultColumn
	br  blockResult

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
}

func (wctx *pipeUnpackWriteContext) reset() {
	wctx.workerID = 0
	wctx.ppNext = nil
	wctx.keepOriginalFields = false
	wctx.skipEmptyResults = false

	wctx.brSrc = nil
	wctx.csSrc = nil

	rcs := wctx.rcs
	for i := range rcs {
		rcs[i].reset()
	}
	wctx.rcs = rcs[:0]

	wctx.rowsCount = 0
	wctx.valuesLen = 0
}

func (wctx *pipeUnpackWriteContext) init(workerID uint, ppNext pipeProcessor, keepOriginalFields, skipEmptyResults bool, brSrc *blockResult) {
	wctx.reset()

	wctx.workerID = workerID
	wctx.ppNext = ppNext
	wctx.keepOriginalFields = keepOriginalFields
	wctx.skipEmptyResults = skipEmptyResults

	wctx.brSrc = brSrc
	wctx.csSrc = brSrc.getColumns()
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
		// send the current block to ppNext and construct a block with new set of columns
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
		if v == "" && wctx.skipEmptyResults || wctx.keepOriginalFields {
			idx := getBlockResultColumnIdxByName(csSrc, f.Name)
			if idx >= 0 {
				vOrig := csSrc[idx].getValueAtRow(brSrc, rowIdx)
				if vOrig != "" {
					v = vOrig
				}
			}
		}
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

	// Flush rcs to ppNext
	br := &wctx.br
	br.setResultColumns(rcs, wctx.rowsCount)
	wctx.rowsCount = 0
	wctx.ppNext.writeBlock(wctx.workerID, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetValues()
	}
}
