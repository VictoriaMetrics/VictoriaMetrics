package logstorage

import (
	"container/heap"
	"sort"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

type statsJSONValuesTopkProcessor struct {
	sortFieldsLen int

	h statsJSONValuesTopkHeap

	sortColumns   [][]string
	sortValuesBuf []string
}

func (svp *statsJSONValuesTopkProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sv := sf.(*statsJSONValues)

	svp.initSortColumns(br, sv.sortFields)

	stateSizeIncrease := 0
	mc := getMatchingColumns(br, sv.fieldFilters)
	mc.sort()
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		stateSizeIncrease += svp.updateStateForRow(sv, br, mc.cs, rowIdx)
	}
	putMatchingColumns(mc)

	return stateSizeIncrease
}

func (svp *statsJSONValuesTopkProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sv := sf.(*statsJSONValues)

	svp.initSortColumns(br, sv.sortFields)

	mc := getMatchingColumns(br, sv.fieldFilters)
	mc.sort()
	stateSizeIncrease := svp.updateStateForRow(sv, br, mc.cs, rowIdx)
	putMatchingColumns(mc)

	return stateSizeIncrease
}

func (svp *statsJSONValuesTopkProcessor) updateStateForRow(sv *statsJSONValues, br *blockResult, cs []*blockResultColumn, rowIdx int) int {
	svp.sortValuesBuf = slicesutil.SetLength(svp.sortValuesBuf, len(svp.sortColumns))
	for i, values := range svp.sortColumns {
		svp.sortValuesBuf[i] = values[rowIdx]
	}

	if uint64(len(svp.h.entries)) < sv.limit {
		e := newStatsJSONValuesSortedEntry(br, cs, svp.sortValuesBuf, rowIdx)
		svp.h.entries = append(svp.h.entries, e)
		heap.Fix(&svp.h, len(svp.h.entries)-1)

		return e.sizeBytes()
	}

	top := svp.h.entries[0]
	if !statsJSONValuesLess(sv.sortFields, svp.sortValuesBuf, top.sortValues) {
		// Fast path - the current entry is bigger than the biggest entry in the entries
		return 0
	}

	// Slow path - replace the top entry with the current entry
	e := newStatsJSONValuesSortedEntry(br, cs, svp.sortValuesBuf, rowIdx)
	bytesAllocated := e.sizeBytes() - top.sizeBytes()
	svp.h.entries[0] = e
	heap.Fix(&svp.h, 0)

	return bytesAllocated
}

func (svp *statsJSONValuesTopkProcessor) initSortColumns(br *blockResult, sortFields []*bySortField) {
	svp.sortColumns = svp.sortColumns[:0]
	for _, sf := range sortFields {
		c := br.getColumnByName(sf.name)
		values := c.getValues(br)
		svp.sortColumns = append(svp.sortColumns, values)
	}
}

func (svp *statsJSONValuesTopkProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsJSONValuesTopkProcessor)
	svp.h.entries = append(svp.h.entries, src.h.entries...)
}

func (svp *statsJSONValuesTopkProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	return statsJSONValuesSortedMarshalState(dst, svp.h.entries)
}

func (svp *statsJSONValuesTopkProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	entries, stateSizeIncrease, err := statsJSONValuesSortedUnmarshalState(src, svp.sortFieldsLen)
	if err != nil {
		return 0, err
	}
	svp.h.entries = entries

	return stateSizeIncrease, nil
}

func (svp *statsJSONValuesTopkProcessor) finalizeStats(sf statsFunc, dst []byte, _ <-chan struct{}) []byte {
	sv := sf.(*statsJSONValues)

	entries := svp.h.entries

	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		return statsJSONValuesLess(sv.sortFields, a.sortValues, b.sortValues)
	})
	if uint64(len(entries)) > sv.limit {
		entries = entries[:sv.limit]
	}

	values := make([]string, len(entries))
	for i := range entries {
		values[i] = entries[i].value
	}

	return marshalJSONValues(dst, values)
}

type statsJSONValuesTopkHeap struct {
	sortFields []*bySortField

	entries []*statsJSONValuesSortedEntry
}

func (h *statsJSONValuesTopkHeap) Len() int {
	return len(h.entries)
}
func (h *statsJSONValuesTopkHeap) Swap(i, j int) {
	a := h.entries
	a[i], a[j] = a[j], a[i]
}
func (h *statsJSONValuesTopkHeap) Less(i, j int) bool {
	a := h.entries
	return !statsJSONValuesLess(h.sortFields, a[i].sortValues, a[j].sortValues)
}
func (h *statsJSONValuesTopkHeap) Push(v any) {
	e := v.(*statsJSONValuesSortedEntry)
	h.entries = append(h.entries, e)
}
func (h *statsJSONValuesTopkHeap) Pop() any {
	a := h.entries
	e := a[len(a)-1]
	h.entries = a[:len(a)-1]
	return e
}
