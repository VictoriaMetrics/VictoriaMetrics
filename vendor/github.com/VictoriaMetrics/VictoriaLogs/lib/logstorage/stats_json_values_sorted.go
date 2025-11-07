package logstorage

import (
	"fmt"
	"sort"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

type statsJSONValuesSortedProcessor struct {
	sortFieldsLen int

	entries []*statsJSONValuesSortedEntry

	sortColumns   [][]string
	sortValuesBuf []string
}

type statsJSONValuesSortedEntry struct {
	// The value itself
	value string

	// values for sortFields, which are used for sorting.
	sortValues []string
}

func newStatsJSONValuesSortedEntry(br *blockResult, cs []*blockResultColumn, sortValues []string, rowIdx int) *statsJSONValuesSortedEntry {
	fields := make([]Field, len(cs))
	for i, c := range cs {
		fields[i] = Field{
			Name:  c.name,
			Value: c.getValueAtRow(br, rowIdx),
		}
	}
	value := string(MarshalFieldsToJSON(nil, fields))

	sortValuesCopy := make([]string, len(sortValues))
	for i, v := range sortValues {
		sortValuesCopy[i] = strings.Clone(v)
	}

	return &statsJSONValuesSortedEntry{
		value:      value,
		sortValues: sortValuesCopy,
	}
}

func (e *statsJSONValuesSortedEntry) sizeBytes() int {
	return int(unsafe.Sizeof(*e)) + len(e.value) + int(unsafe.Sizeof(e.sortValues))*cap(e.sortValues)
}

func (svp *statsJSONValuesSortedProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sv := sf.(*statsJSONValues)

	svp.initSortColumns(br, sv.sortFields)

	stateSizeIncrease := 0
	mc := getMatchingColumns(br, sv.fieldFilters)
	mc.sort()
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		stateSizeIncrease += svp.updateStateForRow(br, mc.cs, rowIdx)
	}
	putMatchingColumns(mc)

	return stateSizeIncrease
}

func (svp *statsJSONValuesSortedProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sv := sf.(*statsJSONValues)

	svp.initSortColumns(br, sv.sortFields)

	mc := getMatchingColumns(br, sv.fieldFilters)
	mc.sort()
	stateSizeIncrease := svp.updateStateForRow(br, mc.cs, rowIdx)
	putMatchingColumns(mc)

	return stateSizeIncrease
}

func (svp *statsJSONValuesSortedProcessor) updateStateForRow(br *blockResult, cs []*blockResultColumn, rowIdx int) int {
	svp.sortValuesBuf = slicesutil.SetLength(svp.sortValuesBuf, len(svp.sortColumns))
	for i, values := range svp.sortColumns {
		svp.sortValuesBuf[i] = values[rowIdx]
	}

	e := newStatsJSONValuesSortedEntry(br, cs, svp.sortValuesBuf, rowIdx)

	svp.entries = append(svp.entries, e)

	return e.sizeBytes() + int(unsafe.Sizeof(svp.entries[0]))
}

func (svp *statsJSONValuesSortedProcessor) initSortColumns(br *blockResult, sortFields []*bySortField) {
	svp.sortColumns = svp.sortColumns[:0]
	for _, sf := range sortFields {
		c := br.getColumnByName(sf.name)
		values := c.getValues(br)
		svp.sortColumns = append(svp.sortColumns, values)
	}
}

func (svp *statsJSONValuesSortedProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsJSONValuesSortedProcessor)
	svp.entries = append(svp.entries, src.entries...)
}

func (svp *statsJSONValuesSortedProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	return statsJSONValuesSortedMarshalState(dst, svp.entries)
}

func (svp *statsJSONValuesSortedProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	entries, stateSizeIncrease, err := statsJSONValuesSortedUnmarshalState(src, svp.sortFieldsLen)
	if err != nil {
		return 0, err
	}
	svp.entries = entries

	return stateSizeIncrease, nil
}

func (svp *statsJSONValuesSortedProcessor) finalizeStats(sf statsFunc, dst []byte, _ <-chan struct{}) []byte {
	sv := sf.(*statsJSONValues)

	entries := svp.entries

	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		return statsJSONValuesLess(sv.sortFields, a.sortValues, b.sortValues)
	})

	values := make([]string, len(entries))
	for i := range entries {
		values[i] = entries[i].value
	}

	return marshalJSONValues(dst, values)
}

func statsJSONValuesLess(sfs []*bySortField, a, b []string) bool {
	for i, sf := range sfs {
		sA, sB := a[i], b[i]
		if sA == sB {
			continue
		}
		if lessString(sA, sB) {
			return !sf.isDesc
		}
		return sf.isDesc
	}
	return false
}

func statsJSONValuesSortedMarshalState(dst []byte, entries []*statsJSONValuesSortedEntry) []byte {
	dst = encoding.MarshalVarUint64(dst, uint64(len(entries)))
	for _, e := range entries {
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(e.value))
		for _, v := range e.sortValues {
			dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(v))
		}
	}
	return dst
}

func statsJSONValuesSortedUnmarshalState(src []byte, sortFieldsLen int) ([]*statsJSONValuesSortedEntry, int, error) {
	entriesLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return nil, 0, fmt.Errorf("cannot unmarshal entriesLen")
	}
	src = src[n:]

	entries := make([]*statsJSONValuesSortedEntry, entriesLen)
	stateSizeIncrease := int(unsafe.Sizeof(entries[0])) * len(entries)

	// Unmarshal values
	for i := range entries {
		v, n := encoding.UnmarshalBytes(src)
		if n <= 0 {
			return nil, 0, fmt.Errorf("cannot unmarshal value")
		}
		src = src[n:]

		value := string(v)

		sortValues := make([]string, sortFieldsLen)
		for j := range sortValues {
			v, n := encoding.UnmarshalBytes(src)
			if n <= 0 {
				return nil, 0, fmt.Errorf("cannot unmarshal sort value")
			}
			src = src[n:]

			sortValues[j] = string(v)
		}

		e := &statsJSONValuesSortedEntry{
			value:      value,
			sortValues: sortValues,
		}
		entries[i] = e

		stateSizeIncrease += e.sizeBytes()
	}

	if len(src) > 0 {
		return nil, 0, fmt.Errorf("unexpected tail left after unmarshaling values; len(tail)=%d", len(src))
	}

	if len(entries) == 0 {
		entries = nil
	}

	return entries, stateSizeIncrease, nil
}
