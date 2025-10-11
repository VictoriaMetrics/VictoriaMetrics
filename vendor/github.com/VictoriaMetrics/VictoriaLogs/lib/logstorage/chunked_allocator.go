package logstorage

import (
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// chunkedAllocator reduces memory fragmentation when allocating pre-defined structs in a scoped fashion.
//
// It also reduces the number of memory allocations by amortizing them into 64Kb slice allocations.
//
// chunkedAllocator cannot be used from concurrently running goroutines.
type chunkedAllocator struct {
	avgProcessors              []statsAvgProcessor
	countProcessors            []statsCountProcessor
	countEmptyProcessors       []statsCountEmptyProcessor
	countUniqProcessors        []statsCountUniqProcessor
	countUniqHashProcessors    []statsCountUniqHashProcessor
	histogramProcessors        []statsHistogramProcessor
	jsonValuesProcessors       []statsJSONValuesProcessor
	jsonValuesSortedProcessors []statsJSONValuesSortedProcessor
	jsonValuesTopkProcessors   []statsJSONValuesTopkProcessor
	maxProcessors              []statsMaxProcessor
	medianProcessors           []statsMedianProcessor
	minProcessors              []statsMinProcessor
	quantileProcessors         []statsQuantileProcessor
	rateProcessors             []statsRateProcessor
	rateSumProcessors          []statsRateSumProcessor
	rowAnyProcessors           []statsRowAnyProcessor
	rowMaxProcessors           []statsRowMaxProcessor
	rowMinProcessors           []statsRowMinProcessor
	sumProcessors              []statsSumProcessor
	sumLenProcessors           []statsSumLenProcessor
	uniqValuesProcessors       []statsUniqValuesProcessor
	valuesProcessors           []statsValuesProcessor

	pipeStatsGroups         []pipeStatsGroup
	pipeStatsGroupMapShards []pipeStatsGroupMapShard

	statsProcessors []statsProcessor

	statsCountUniqSets     []statsCountUniqSet
	statsCountUniqHashSets []statsCountUniqHashSet

	hitsMapShards []hitsMapShard

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

func (a *chunkedAllocator) newStatsJSONValuesProcessor() (p *statsJSONValuesProcessor) {
	return addNewItem(&a.jsonValuesProcessors, a)
}

func (a *chunkedAllocator) newStatsJSONValuesSortedProcessor() (p *statsJSONValuesSortedProcessor) {
	return addNewItem(&a.jsonValuesSortedProcessors, a)
}

func (a *chunkedAllocator) newStatsJSONValuesTopkProcessor() (p *statsJSONValuesTopkProcessor) {
	return addNewItem(&a.jsonValuesTopkProcessors, a)
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

func (a *chunkedAllocator) newPipeStatsGroupMapShards(itemsLen uint) []pipeStatsGroupMapShard {
	return addNewItems(&a.pipeStatsGroupMapShards, itemsLen, a)
}

func (a *chunkedAllocator) newStatsProcessors(itemsLen uint) []statsProcessor {
	return addNewItems(&a.statsProcessors, itemsLen, a)
}

func (a *chunkedAllocator) newStatsCountUniqSets(itemsLen uint) []statsCountUniqSet {
	return addNewItems(&a.statsCountUniqSets, itemsLen, a)
}

func (a *chunkedAllocator) newStatsCountUniqHashSets(itemsLen uint) []statsCountUniqHashSet {
	return addNewItems(&a.statsCountUniqHashSets, itemsLen, a)
}

func (a *chunkedAllocator) newHitsMapShards(itemsLen uint) []hitsMapShard {
	return addNewItems(&a.hitsMapShards, itemsLen, a)
}

func (a *chunkedAllocator) newUint64() (p *uint64) {
	return addNewItem(&a.u64Buf, a)
}

func (a *chunkedAllocator) cloneBytesToString(b []byte) string {
	return a.cloneString(bytesutil.ToUnsafeString(b))
}

func (a *chunkedAllocator) cloneString(s string) string {
	xs := addNewItems(&a.stringsBuf, uint(len(s)), a)
	copy(xs, s)
	return bytesutil.ToUnsafeString(xs)
}

func addNewItem[T any](dstPtr *[]T, a *chunkedAllocator) *T {
	xs := addNewItems(dstPtr, 1, a)
	return &xs[0]
}

func addNewItems[T any](dstPtr *[]T, itemsLen uint, a *chunkedAllocator) []T {
	dst := *dstPtr
	var maxItems = (64 * 1024) / uint(unsafe.Sizeof(dst[0]))
	if itemsLen > maxItems {
		return make([]T, itemsLen)
	}
	if dst != nil && uint(len(dst))+itemsLen > maxItems {
		dst = nil
	}
	if dst == nil {
		dst = make([]T, 0, maxItems)
		a.bytesAllocated += int(maxItems * uint(unsafe.Sizeof(dst[0])))
	}
	dstLen := uint(len(dst))
	dst = dst[:dstLen+itemsLen]
	xs := dst[dstLen : dstLen+itemsLen : dstLen+itemsLen]
	*dstPtr = dst
	return xs
}
