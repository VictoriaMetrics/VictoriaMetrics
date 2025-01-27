package logstorage

import (
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type chunkedAllocator struct {
	avgProcessors           []statsAvgProcessor
	countProcessors         []statsCountProcessor
	countEmptyProcessors    []statsCountEmptyProcessor
	countUniqProcessors     []statsCountUniqProcessor
	countUniqHashProcessors []statsCountUniqHashProcessor
	histogramProcessors     []statsHistogramProcessor
	maxProcessors           []statsMaxProcessor
	medianProcessors        []statsMedianProcessor
	minProcessors           []statsMinProcessor
	quantileProcessors      []statsQuantileProcessor
	rateProcessors          []statsRateProcessor
	rateSumProcessors       []statsRateSumProcessor
	rowAnyProcessors        []statsRowAnyProcessor
	rowMaxProcessors        []statsRowMaxProcessor
	rowMinProcessors        []statsRowMinProcessor
	sumProcessors           []statsSumProcessor
	sumLenProcessors        []statsSumLenProcessor
	uniqValuesProcessors    []statsUniqValuesProcessor
	valuesProcessors        []statsValuesProcessor

	pipeStatsGroups []pipeStatsGroup

	u64Buf []uint64

	stringsBuf []byte

	bytesAllocated int
}

func (a *chunkedAllocator) newStatsAvgProcessor() (p *statsAvgProcessor) {
	return addNewItem(&a.avgProcessors, a)
}

func (a *chunkedAllocator) newStatsCountProcessor() (p *statsCountProcessor) {
	return addNewItem(&a.countProcessors, a)
}

func (a *chunkedAllocator) newStatsCountEmptyProcessor() (p *statsCountEmptyProcessor) {
	return addNewItem(&a.countEmptyProcessors, a)
}

func (a *chunkedAllocator) newStatsCountUniqProcessor() (p *statsCountUniqProcessor) {
	return addNewItem(&a.countUniqProcessors, a)
}

func (a *chunkedAllocator) newStatsCountUniqHashProcessor() (p *statsCountUniqHashProcessor) {
	return addNewItem(&a.countUniqHashProcessors, a)
}

func (a *chunkedAllocator) newStatsHistogramProcessor() (p *statsHistogramProcessor) {
	return addNewItem(&a.histogramProcessors, a)
}

func (a *chunkedAllocator) newStatsMaxProcessor() (p *statsMaxProcessor) {
	return addNewItem(&a.maxProcessors, a)
}

func (a *chunkedAllocator) newStatsMedianProcessor() (p *statsMedianProcessor) {
	return addNewItem(&a.medianProcessors, a)
}

func (a *chunkedAllocator) newStatsMinProcessor() (p *statsMinProcessor) {
	return addNewItem(&a.minProcessors, a)
}

func (a *chunkedAllocator) newStatsQuantileProcessor() (p *statsQuantileProcessor) {
	return addNewItem(&a.quantileProcessors, a)
}

func (a *chunkedAllocator) newStatsRateProcessor() (p *statsRateProcessor) {
	return addNewItem(&a.rateProcessors, a)
}

func (a *chunkedAllocator) newStatsRateSumProcessor() (p *statsRateSumProcessor) {
	return addNewItem(&a.rateSumProcessors, a)
}

func (a *chunkedAllocator) newStatsRowAnyProcessor() (p *statsRowAnyProcessor) {
	return addNewItem(&a.rowAnyProcessors, a)
}

func (a *chunkedAllocator) newStatsRowMaxProcessor() (p *statsRowMaxProcessor) {
	return addNewItem(&a.rowMaxProcessors, a)
}

func (a *chunkedAllocator) newStatsRowMinProcessor() (p *statsRowMinProcessor) {
	return addNewItem(&a.rowMinProcessors, a)
}

func (a *chunkedAllocator) newStatsSumProcessor() (p *statsSumProcessor) {
	return addNewItem(&a.sumProcessors, a)
}

func (a *chunkedAllocator) newStatsSumLenProcessor() (p *statsSumLenProcessor) {
	return addNewItem(&a.sumLenProcessors, a)
}

func (a *chunkedAllocator) newStatsUniqValuesProcessor() (p *statsUniqValuesProcessor) {
	return addNewItem(&a.uniqValuesProcessors, a)
}

func (a *chunkedAllocator) newStatsValuesProcessor() (p *statsValuesProcessor) {
	return addNewItem(&a.valuesProcessors, a)
}

func (a *chunkedAllocator) newPipeStatsGroup() (p *pipeStatsGroup) {
	return addNewItem(&a.pipeStatsGroups, a)
}

func (a *chunkedAllocator) newUint64() (p *uint64) {
	return addNewItem(&a.u64Buf, a)
}

func (a *chunkedAllocator) cloneBytesToString(b []byte) string {
	return a.cloneString(bytesutil.ToUnsafeString(b))
}

func (a *chunkedAllocator) cloneString(s string) string {
	const maxChunkLen = 64 * 1024
	if a.stringsBuf != nil && len(a.stringsBuf)+len(s) > maxChunkLen {
		a.stringsBuf = nil
	}
	if a.stringsBuf == nil {
		a.stringsBuf = make([]byte, 0, maxChunkLen)
		a.bytesAllocated += maxChunkLen
	}

	sbLen := len(a.stringsBuf)
	a.stringsBuf = append(a.stringsBuf, s...)
	return bytesutil.ToUnsafeString(a.stringsBuf[sbLen:])
}

func addNewItem[T any](dstPtr *[]T, a *chunkedAllocator) *T {
	dst := *dstPtr
	var maxItems = (64 * 1024) / int(unsafe.Sizeof(dst[0]))
	if dst != nil && len(dst)+1 > maxItems {
		dst = nil
	}
	if dst == nil {
		dst = make([]T, 0, maxItems)
		a.bytesAllocated += maxItems * int(unsafe.Sizeof(dst[0]))
	}
	var x T
	dst = append(dst, x)
	item := &dst[len(dst)-1]
	*dstPtr = dst
	return item
}
