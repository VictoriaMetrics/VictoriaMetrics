package logstorage

import (
	"container/heap"
	"fmt"
	"slices"
	"sync"
	"unsafe"

	"github.com/cespare/xxhash/v2"
	"github.com/valyala/quicktemplate"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type statsUniqValues struct {
	fields []string
	limit  uint64
}

func (su *statsUniqValues) String() string {
	s := "uniq_values(" + statsFuncFieldsToString(su.fields) + ")"
	if su.limit > 0 {
		s += fmt.Sprintf(" limit %d", su.limit)
	}
	return s
}

func (su *statsUniqValues) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, su.fields)
}

func (su *statsUniqValues) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	sup := a.newStatsUniqValuesProcessor()
	sup.a = a
	sup.m = make(map[string]struct{})
	return sup
}

type statsUniqValuesProcessor struct {
	a *chunkedAllocator

	// concurrency is the number of parallel workers to use when merging shards.
	//
	// this field must be updated by the caller before using statsUniqValuesProcessor.
	concurrency uint

	m  map[string]struct{}
	ms []map[string]struct{}
}

func (sup *statsUniqValuesProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	su := sf.(*statsUniqValues)
	if sup.limitReached(su) {
		// Limit on the number of unique values has been reached
		return 0
	}

	stateSizeIncrease := 0
	fields := su.fields
	if len(fields) == 0 {
		for _, c := range br.getColumns() {
			stateSizeIncrease += sup.updateStatsForAllRowsColumn(c, br)
		}
	} else {
		for _, field := range fields {
			c := br.getColumnByName(field)
			stateSizeIncrease += sup.updateStatsForAllRowsColumn(c, br)
		}
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesProcessor) updateStatsForAllRowsColumn(c *blockResultColumn, br *blockResult) int {
	if c.isConst {
		// collect unique const values
		v := c.valuesEncoded[0]
		return sup.updateState(v)
	}

	stateSizeIncrease := 0
	if c.valueType == valueTypeDict {
		// collect unique non-zero c.dictValues
		c.forEachDictValue(br, func(v string) {
			stateSizeIncrease += sup.updateState(v)
		})
		return stateSizeIncrease
	}

	// slow path - collect unique values across all rows
	values := c.getValues(br)
	for i, v := range values {
		if i > 0 && values[i-1] == v {
			// This value has been already counted.
			continue
		}
		stateSizeIncrease += sup.updateState(v)
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	su := sf.(*statsUniqValues)
	if sup.limitReached(su) {
		// Limit on the number of unique values has been reached
		return 0
	}

	stateSizeIncrease := 0
	fields := su.fields
	if len(fields) == 0 {
		for _, c := range br.getColumns() {
			stateSizeIncrease += sup.updateStatsForRowColumn(c, br, rowIdx)
		}
	} else {
		for _, field := range fields {
			c := br.getColumnByName(field)
			stateSizeIncrease += sup.updateStatsForRowColumn(c, br, rowIdx)
		}
	}
	return stateSizeIncrease
}

func (sup *statsUniqValuesProcessor) updateStatsForRowColumn(c *blockResultColumn, br *blockResult, rowIdx int) int {
	if c.isConst {
		// collect unique const values
		v := c.valuesEncoded[0]
		return sup.updateState(v)
	}

	if c.valueType == valueTypeDict {
		// collect unique non-zero c.dictValues
		valuesEncoded := c.getValuesEncoded(br)
		dictIdx := valuesEncoded[rowIdx][0]
		v := c.dictValues[dictIdx]
		return sup.updateState(v)
	}

	// collect unique values for the given rowIdx.
	v := c.getValueAtRow(br, rowIdx)
	return sup.updateState(v)
}

func (sup *statsUniqValuesProcessor) mergeState(_ *chunkedAllocator, sf statsFunc, sfp statsProcessor) {
	su := sf.(*statsUniqValues)
	if sup.limitReached(su) {
		return
	}

	src := sfp.(*statsUniqValuesProcessor)
	if len(src.m) > 100_000 {
		// Postpone merging too big number of items in parallel
		sup.ms = append(sup.ms, src.m)
		return
	}

	for k := range src.m {
		if _, ok := sup.m[k]; !ok {
			sup.m[k] = struct{}{}
		}
	}
}

func (sup *statsUniqValuesProcessor) finalizeStats(sf statsFunc, dst []byte, stopCh <-chan struct{}) []byte {
	su := sf.(*statsUniqValues)
	var items []string
	if len(sup.ms) > 0 {
		sup.ms = append(sup.ms, sup.m)
		items = mergeSetsParallel(sup.ms, sup.concurrency, stopCh)
	} else {
		items = setToSortedSlice(sup.m)
	}

	if limit := su.limit; limit > 0 && uint64(len(items)) > limit {
		items = items[:limit]
	}

	return marshalJSONArray(dst, items)
}

func mergeSetsParallel(ms []map[string]struct{}, concurrency uint, stopCh <-chan struct{}) []string {
	shardsLen := len(ms)
	cpusCount := concurrency

	var wg sync.WaitGroup
	msShards := make([][]map[string]struct{}, shardsLen)
	for i := range msShards {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			perCPU := make([]map[string]struct{}, cpusCount)
			for i := range perCPU {
				perCPU[i] = make(map[string]struct{})
			}

			for s := range ms[idx] {
				if needStop(stopCh) {
					return
				}
				h := xxhash.Sum64(bytesutil.ToUnsafeBytes(s))
				m := perCPU[h%uint64(len(perCPU))]
				m[s] = struct{}{}
			}

			msShards[idx] = perCPU
			ms[idx] = nil
		}(i)
	}
	wg.Wait()

	perCPUItems := make([][]string, cpusCount)
	for i := range perCPUItems {
		wg.Add(1)
		go func(cpuIdx int) {
			defer wg.Done()

			m := msShards[0][cpuIdx]
			for _, perCPU := range msShards[1:] {
				for s := range perCPU[cpuIdx] {
					if needStop(stopCh) {
						return
					}
					if _, ok := m[s]; !ok {
						m[s] = struct{}{}
					}
				}
				perCPU[cpuIdx] = nil
			}

			items := setToSortedSlice(m)
			perCPUItems[cpuIdx] = items
		}(i)
	}
	wg.Wait()

	itemsTotalLen := 0
	for _, items := range perCPUItems {
		itemsTotalLen += len(items)
	}
	itemsAll := make([]string, 0, itemsTotalLen)

	var h sortedStringsHeap
	for _, items := range perCPUItems {
		if len(items) > 0 {
			h = append(h, items)
		}
	}
	heap.Init(&h)
	for len(h) > 0 {
		if needStop(stopCh) {
			return nil
		}
		top := h[0]
		s := top[0]
		itemsAll = append(itemsAll, s)
		if len(top) == 1 {
			heap.Pop(&h)
			continue
		}
		h[0] = top[1:]
		heap.Fix(&h, 0)
	}
	return itemsAll
}

type sortedStringsHeap [][]string

func (h *sortedStringsHeap) Len() int {
	return len(*h)
}
func (h *sortedStringsHeap) Less(i, j int) bool {
	a := *h
	return lessString(a[i][0], a[j][0])
}
func (h *sortedStringsHeap) Swap(i, j int) {
	a := *h
	a[i], a[j] = a[j], a[i]
}
func (h *sortedStringsHeap) Push(x any) {
	ss := x.([]string)
	*h = append(*h, ss)
}
func (h *sortedStringsHeap) Pop() any {
	a := *h
	x := a[len(a)-1]
	a[len(a)-1] = nil
	*h = a[:len(a)-1]
	return x
}

func setToSortedSlice(m map[string]struct{}) []string {
	items := make([]string, 0, len(m))
	for k := range m {
		items = append(items, k)
	}
	sortStrings(items)
	return items
}

func sortStrings(a []string) {
	slices.SortFunc(a, func(x, y string) int {
		if x == y {
			return 0
		}
		if lessString(x, y) {
			return -1
		}
		return 1
	})
}

func (sup *statsUniqValuesProcessor) updateState(v string) int {
	if v == "" {
		// Skip empty values
		return 0
	}
	if _, ok := sup.m[v]; ok {
		return 0
	}
	vCopy := sup.a.cloneString(v)
	sup.m[vCopy] = struct{}{}
	return len(vCopy) + int(unsafe.Sizeof(vCopy))
}

func (sup *statsUniqValuesProcessor) limitReached(su *statsUniqValues) bool {
	limit := su.limit
	return limit > 0 && uint64(len(sup.m)) > limit
}

func marshalJSONArray(dst []byte, items []string) []byte {
	if len(items) == 0 {
		return append(dst, "[]"...)
	}
	dst = append(dst, '[')
	dst = quicktemplate.AppendJSONString(dst, items[0], true)
	for _, item := range items[1:] {
		dst = append(dst, ',')
		dst = quicktemplate.AppendJSONString(dst, item, true)
	}
	dst = append(dst, ']')
	return dst
}

func parseStatsUniqValues(lex *lexer) (*statsUniqValues, error) {
	fields, err := parseStatsFuncFields(lex, "uniq_values")
	if err != nil {
		return nil, err
	}
	su := &statsUniqValues{
		fields: fields,
	}
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s' for 'uniq_values': %w", lex.token, err)
		}
		lex.nextToken()
		su.limit = n
	}
	return su, nil
}
