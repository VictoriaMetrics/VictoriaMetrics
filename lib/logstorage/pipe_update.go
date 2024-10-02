package logstorage

import (
	"unsafe"
)

func updateNeededFieldsForUpdatePipe(neededFields, unneededFields fieldsSet, field string, iff *ifFilter) {
	if neededFields.isEmpty() {
		if iff != nil {
			neededFields.addFields(iff.neededFields)
		}
		return
	}

	if neededFields.contains("*") {
		if !unneededFields.contains(field) && iff != nil {
			unneededFields.removeFields(iff.neededFields)
		}
	} else {
		if neededFields.contains(field) && iff != nil {
			neededFields.addFields(iff.neededFields)
		}
	}
}

func newPipeUpdateProcessor(workersCount int, updateFunc func(a *arena, v string) string, ppNext pipeProcessor, field string, iff *ifFilter) pipeProcessor {
	return &pipeUpdateProcessor{
		updateFunc: updateFunc,

		field: field,
		iff:   iff,

		ppNext: ppNext,

		shards: make([]pipeUpdateProcessorShard, workersCount),
	}
}

type pipeUpdateProcessor struct {
	updateFunc func(a *arena, v string) string

	field string
	iff   *ifFilter

	ppNext pipeProcessor

	shards []pipeUpdateProcessorShard
}

type pipeUpdateProcessorShard struct {
	pipeUpdateProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeUpdateProcessorShardNopad{})%128]byte
}

type pipeUpdateProcessorShardNopad struct {
	bm bitmap

	rc resultColumn
	a  arena
}

func (pup *pipeUpdateProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := &pup.shards[workerID]

	bm := &shard.bm
	bm.init(br.rowsLen)
	bm.setBits()
	if iff := pup.iff; iff != nil {
		iff.f.applyToBlockResult(br, bm)
		if bm.isZero() {
			pup.ppNext.writeBlock(workerID, br)
			return
		}
	}

	shard.rc.name = pup.field

	c := br.getColumnByName(pup.field)
	values := c.getValues(br)

	hadUpdates := false
	vPrev := ""
	for rowIdx, v := range values {
		if bm.isSetBit(rowIdx) {
			if !hadUpdates || vPrev != v {
				vPrev = v
				hadUpdates = true

				v = pup.updateFunc(&shard.a, v)
			}
		}
		shard.rc.addValue(v)
	}

	br.addResultColumn(&shard.rc)
	pup.ppNext.writeBlock(workerID, br)

	shard.rc.reset()
	shard.a.reset()
}

func (pup *pipeUpdateProcessor) flush() error {
	return nil
}
