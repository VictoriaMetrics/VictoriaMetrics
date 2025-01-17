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
	a.avgProcessors, p = addNewItem(a.avgProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsCountProcessor() (p *statsCountProcessor) {
	a.countProcessors, p = addNewItem(a.countProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsCountEmptyProcessor() (p *statsCountEmptyProcessor) {
	a.countEmptyProcessors, p = addNewItem(a.countEmptyProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsCountUniqProcessor() (p *statsCountUniqProcessor) {
	a.countUniqProcessors, p = addNewItem(a.countUniqProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsCountUniqHashProcessor() (p *statsCountUniqHashProcessor) {
	a.countUniqHashProcessors, p = addNewItem(a.countUniqHashProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsHistogramProcessor() (p *statsHistogramProcessor) {
	a.histogramProcessors, p = addNewItem(a.histogramProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsMaxProcessor() (p *statsMaxProcessor) {
	a.maxProcessors, p = addNewItem(a.maxProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsMedianProcessor() (p *statsMedianProcessor) {
	a.medianProcessors, p = addNewItem(a.medianProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsMinProcessor() (p *statsMinProcessor) {
	a.minProcessors, p = addNewItem(a.minProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsQuantileProcessor() (p *statsQuantileProcessor) {
	a.quantileProcessors, p = addNewItem(a.quantileProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsRateProcessor() (p *statsRateProcessor) {
	a.rateProcessors, p = addNewItem(a.rateProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsRateSumProcessor() (p *statsRateSumProcessor) {
	a.rateSumProcessors, p = addNewItem(a.rateSumProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsRowAnyProcessor() (p *statsRowAnyProcessor) {
	a.rowAnyProcessors, p = addNewItem(a.rowAnyProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsRowMaxProcessor() (p *statsRowMaxProcessor) {
	a.rowMaxProcessors, p = addNewItem(a.rowMaxProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsRowMinProcessor() (p *statsRowMinProcessor) {
	a.rowMinProcessors, p = addNewItem(a.rowMinProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsSumProcessor() (p *statsSumProcessor) {
	a.sumProcessors, p = addNewItem(a.sumProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsSumLenProcessor() (p *statsSumLenProcessor) {
	a.sumLenProcessors, p = addNewItem(a.sumLenProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsUniqValuesProcessor() (p *statsUniqValuesProcessor) {
	a.uniqValuesProcessors, p = addNewItem(a.uniqValuesProcessors, a)
	return p
}

func (a *chunkedAllocator) newStatsValuesProcessor() (p *statsValuesProcessor) {
	a.valuesProcessors, p = addNewItem(a.valuesProcessors, a)
	return p
}

func (a *chunkedAllocator) newPipeStatsGroup() (p *pipeStatsGroup) {
	a.pipeStatsGroups, p = addNewItem(a.pipeStatsGroups, a)
	return p
}

func (a *chunkedAllocator) newUint64() (p *uint64) {
	a.u64Buf, p = addNewItem(a.u64Buf, a)
	return p
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

func addNewItem[T any](dst []T, a *chunkedAllocator) ([]T, *T) {
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
	return dst, &dst[len(dst)-1]
}
