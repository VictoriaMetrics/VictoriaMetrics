package logstorage

import (
	"fmt"
	"strconv"
	"sync"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

type statsCountUniq struct {
	fields []string
	limit  uint64
}

func (su *statsCountUniq) String() string {
	s := "count_uniq(" + statsFuncFieldsToString(su.fields) + ")"
	if su.limit > 0 {
		s += fmt.Sprintf(" limit %d", su.limit)
	}
	return s
}

func (su *statsCountUniq) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, su.fields)
}

func (su *statsCountUniq) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	sup := a.newStatsCountUniqProcessor()
	sup.a = a
	sup.m.init()
	return sup
}

type statsCountUniqProcessor struct {
	a *chunkedAllocator

	m  statsCountUniqSet
	ms []*statsCountUniqSet

	columnValues [][]string
	keyBuf       []byte
	tmpNum       int
}

type statsCountUniqSet struct {
	timestamps map[uint64]struct{}
	u64        map[uint64]struct{}
	negative64 map[uint64]struct{}
	strings    map[string]struct{}
}

func (sus *statsCountUniqSet) reset() {
	sus.timestamps = nil
	sus.u64 = nil
	sus.negative64 = nil
	sus.strings = nil
}

func (sus *statsCountUniqSet) init() {
	sus.timestamps = make(map[uint64]struct{})
	sus.u64 = make(map[uint64]struct{})
	sus.negative64 = make(map[uint64]struct{})
	sus.strings = make(map[string]struct{})
}

func (sus *statsCountUniqSet) entriesCount() uint64 {
	n := len(sus.timestamps) + len(sus.u64) + len(sus.negative64) + len(sus.strings)
	return uint64(n)
}

func (sus *statsCountUniqSet) updateStateTimestamp(ts int64) int {
	_, ok := sus.timestamps[uint64(ts)]
	if ok {
		return 0
	}
	sus.timestamps[uint64(ts)] = struct{}{}
	return 8
}

func (sus *statsCountUniqSet) updateStateUint64(n uint64) int {
	_, ok := sus.u64[n]
	if ok {
		return 0
	}
	sus.u64[n] = struct{}{}
	return 8
}

func (sus *statsCountUniqSet) updateStateInt64(n int64) int {
	if n >= 0 {
		return sus.updateStateUint64(uint64(n))
	}
	return sus.updateStateNegativeInt64(n)
}

func (sus *statsCountUniqSet) updateStateNegativeInt64(n int64) int {
	_, ok := sus.negative64[uint64(n)]
	if ok {
		return 0
	}
	sus.negative64[uint64(n)] = struct{}{}
	return 8
}

func (sus *statsCountUniqSet) updateStateGeneric(a *chunkedAllocator, v string) int {
	if n, ok := tryParseUint64(v); ok {
		return sus.updateStateUint64(n)
	}
	if len(v) > 0 && v[0] == '-' {
		if n, ok := tryParseInt64(v); ok {
			return sus.updateStateNegativeInt64(n)
		}
	}
	return sus.updateStateString(a, v)
}

func (sus *statsCountUniqSet) updateStateString(a *chunkedAllocator, v string) int {
	_, ok := sus.strings[v]
	if ok {
		return 0
	}
	vCopy := a.cloneString(v)
	sus.strings[vCopy] = struct{}{}
	return int(unsafe.Sizeof(v)) + len(v)
}

func (sus *statsCountUniqSet) mergeState(src *statsCountUniqSet, stopCh <-chan struct{}) {
	mergeUint64Set(sus.timestamps, src.timestamps, stopCh)
	mergeUint64Set(sus.u64, src.u64, stopCh)
	mergeUint64Set(sus.negative64, src.negative64, stopCh)

	for k := range src.strings {
		if needStop(stopCh) {
			return
		}
		if _, ok := sus.strings[k]; !ok {
			sus.strings[k] = struct{}{}
		}
	}
}

func mergeUint64Set(dst map[uint64]struct{}, src map[uint64]struct{}, stopCh <-chan struct{}) {
	for n := range src {
		if needStop(stopCh) {
			return
		}
		if _, ok := dst[n]; !ok {
			dst[n] = struct{}{}
		}
	}
}

func (sup *statsCountUniqProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	su := sf.(*statsCountUniq)
	if sup.limitReached(su) {
		return 0
	}

	fields := su.fields

	stateSizeIncrease := 0
	if len(fields) == 0 {
		// Count unique rows
		cs := br.getColumns()

		columnValues := sup.columnValues[:0]
		for _, c := range cs {
			values := c.getValues(br)
			columnValues = append(columnValues, values)
		}
		sup.columnValues = columnValues

		keyBuf := sup.keyBuf[:0]
		for i := 0; i < br.rowsLen; i++ {
			seenKey := true
			for _, values := range columnValues {
				if i == 0 || values[i-1] != values[i] {
					seenKey = false
					break
				}
			}
			if seenKey {
				// This key has been already counted.
				continue
			}

			allEmptyValues := true
			keyBuf = keyBuf[:0]
			for j, values := range columnValues {
				v := values[i]
				if v != "" {
					allEmptyValues = false
				}
				// Put column name into key, since every block can contain different set of columns for '*' selector.
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(cs[j].name))
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
			}
			if allEmptyValues {
				// Do not count empty values
				continue
			}
			stateSizeIncrease += sup.updateStateString(keyBuf)
		}
		sup.keyBuf = keyBuf
		return stateSizeIncrease
	}
	if len(fields) == 1 {
		// Fast path for a single column.
		return sup.updateStatsForAllRowsSingleColumn(br, fields[0])
	}

	// Slow path for multiple columns.

	// Pre-calculate column values for byFields in order to speed up building group key in the loop below.
	columnValues := sup.columnValues[:0]
	for _, f := range fields {
		c := br.getColumnByName(f)
		values := c.getValues(br)
		columnValues = append(columnValues, values)
	}
	sup.columnValues = columnValues

	keyBuf := sup.keyBuf[:0]
	for i := 0; i < br.rowsLen; i++ {
		seenKey := true
		for _, values := range columnValues {
			if i == 0 || values[i-1] != values[i] {
				seenKey = false
				break
			}
		}
		if seenKey {
			continue
		}

		allEmptyValues := true
		keyBuf = keyBuf[:0]
		for _, values := range columnValues {
			v := values[i]
			if v != "" {
				allEmptyValues = false
			}
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
		}
		if allEmptyValues {
			// Do not count empty values
			continue
		}
		stateSizeIncrease += sup.updateStateString(keyBuf)
	}
	sup.keyBuf = keyBuf
	return stateSizeIncrease
}

func (sup *statsCountUniqProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	su := sf.(*statsCountUniq)
	if sup.limitReached(su) {
		return 0
	}

	fields := su.fields

	if len(fields) == 0 {
		// Count unique rows
		allEmptyValues := true
		keyBuf := sup.keyBuf[:0]
		for _, c := range br.getColumns() {
			v := c.getValueAtRow(br, rowIdx)
			if v != "" {
				allEmptyValues = false
			}
			// Put column name into key, since every block can contain different set of columns for '*' selector.
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(c.name))
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
		}
		sup.keyBuf = keyBuf

		if allEmptyValues {
			// Do not count empty values
			return 0
		}
		return sup.updateStateString(keyBuf)
	}
	if len(fields) == 1 {
		// Fast path for a single column.
		return sup.updateStatsForRowSingleColumn(br, fields[0], rowIdx)
	}

	// Slow path for multiple columns.
	allEmptyValues := true
	keyBuf := sup.keyBuf[:0]
	for _, f := range fields {
		c := br.getColumnByName(f)
		v := c.getValueAtRow(br, rowIdx)
		if v != "" {
			allEmptyValues = false
		}
		keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
	}
	sup.keyBuf = keyBuf

	if allEmptyValues {
		// Do not count empty values
		return 0
	}
	return sup.updateStateString(keyBuf)
}

func (sup *statsCountUniqProcessor) updateStatsForAllRowsSingleColumn(br *blockResult, columnName string) int {
	stateSizeIncrease := 0
	c := br.getColumnByName(columnName)
	if c.isTime {
		// Count unique timestamps
		timestamps := br.getTimestamps()
		for i, timestamp := range timestamps {
			if i > 0 && timestamps[i-1] == timestamps[i] {
				// This timestamp has been already counted.
				continue
			}
			stateSizeIncrease += sup.m.updateStateTimestamp(timestamp)
		}
		return stateSizeIncrease
	}
	if c.isConst {
		// count unique const values
		v := c.valuesEncoded[0]
		if v == "" {
			// Do not count empty values
			return 0
		}
		return sup.updateStateGeneric(v)
	}

	switch c.valueType {
	case valueTypeDict:
		// count unique non-zero dict values for the selected logs
		sup.tmpNum = 0
		c.forEachDictValue(br, func(v string) {
			if v == "" {
				// Do not count empty values
				return
			}
			sup.tmpNum += sup.updateStateGeneric(v)
		})
		return sup.tmpNum
	case valueTypeUint8:
		values := c.getValuesEncoded(br)
		for i, v := range values {
			if i > 0 && values[i-1] == v {
				continue
			}
			n := unmarshalUint8(v)
			stateSizeIncrease += sup.m.updateStateUint64(uint64(n))
		}
		return stateSizeIncrease
	case valueTypeUint16:
		values := c.getValuesEncoded(br)
		for i, v := range values {
			if i > 0 && values[i-1] == v {
				continue
			}
			n := unmarshalUint16(v)
			stateSizeIncrease += sup.m.updateStateUint64(uint64(n))
		}
		return stateSizeIncrease
	case valueTypeUint32:
		values := c.getValuesEncoded(br)
		for i, v := range values {
			if i > 0 && values[i-1] == v {
				continue
			}
			n := unmarshalUint32(v)
			stateSizeIncrease += sup.m.updateStateUint64(uint64(n))
		}
		return stateSizeIncrease
	case valueTypeUint64:
		values := c.getValuesEncoded(br)
		for i, v := range values {
			if i > 0 && values[i-1] == v {
				continue
			}
			n := unmarshalUint64(v)
			stateSizeIncrease += sup.m.updateStateUint64(n)
		}
		return stateSizeIncrease
	case valueTypeInt64:
		values := c.getValuesEncoded(br)
		for i, v := range values {
			if i > 0 && values[i-1] == v {
				continue
			}
			n := unmarshalInt64(v)
			stateSizeIncrease += sup.m.updateStateInt64(n)
		}
		return stateSizeIncrease
	default:
		values := c.getValues(br)
		for i, v := range values {
			if v == "" {
				// Do not count empty values
				continue
			}
			if i > 0 && values[i-1] == v {
				// This value has been already counted.
				continue
			}
			stateSizeIncrease += sup.updateStateGeneric(v)
		}
		return stateSizeIncrease
	}
}

func (sup *statsCountUniqProcessor) updateStatsForRowSingleColumn(br *blockResult, columnName string, rowIdx int) int {
	c := br.getColumnByName(columnName)
	if c.isTime {
		// Count unique timestamps
		timestamps := br.getTimestamps()
		timestamp := timestamps[rowIdx]
		return sup.m.updateStateTimestamp(timestamp)
	}
	if c.isConst {
		// count unique const values
		v := c.valuesEncoded[0]
		if v == "" {
			// Do not count empty values
			return 0
		}
		return sup.updateStateGeneric(v)
	}

	switch c.valueType {
	case valueTypeDict:
		// count unique non-zero c.dictValues
		valuesEncoded := c.getValuesEncoded(br)
		dictIdx := valuesEncoded[rowIdx][0]
		v := c.dictValues[dictIdx]
		if v == "" {
			// Do not count empty values
			return 0
		}
		return sup.updateStateGeneric(v)
	case valueTypeUint8:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalUint8(v)
		return sup.m.updateStateUint64(uint64(n))
	case valueTypeUint16:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalUint16(v)
		return sup.m.updateStateUint64(uint64(n))
	case valueTypeUint32:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalUint32(v)
		return sup.m.updateStateUint64(uint64(n))
	case valueTypeUint64:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalUint64(v)
		return sup.m.updateStateUint64(n)
	case valueTypeInt64:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalInt64(v)
		return sup.m.updateStateInt64(n)
	default:
		// Count unique values for the given rowIdx
		v := c.getValueAtRow(br, rowIdx)
		if v == "" {
			// Do not count empty values
			return 0
		}
		return sup.updateStateGeneric(v)
	}
}

func (sup *statsCountUniqProcessor) mergeState(sf statsFunc, sfp statsProcessor) {
	su := sf.(*statsCountUniq)
	if sup.limitReached(su) {
		return
	}

	src := sfp.(*statsCountUniqProcessor)
	if src.m.entriesCount() > 100_000 {
		// Postpone merging too big number of items in parallel
		sup.ms = append(sup.ms, &src.m)
		return
	}

	sup.m.mergeState(&src.m, nil)
}

func (sup *statsCountUniqProcessor) finalizeStats(sf statsFunc, dst []byte, stopCh <-chan struct{}) []byte {
	n := sup.m.entriesCount()
	if len(sup.ms) > 0 {
		sup.ms = append(sup.ms, &sup.m)
		n = countUniqParallel(sup.ms, stopCh)
	}

	su := sf.(*statsCountUniq)
	if limit := su.limit; limit > 0 && n > limit {
		n = limit
	}
	return strconv.AppendUint(dst, n, 10)
}

func countUniqParallel(ms []*statsCountUniqSet, stopCh <-chan struct{}) uint64 {
	shardsLen := len(ms)
	cpusCount := cgroup.AvailableCPUs()

	var wg sync.WaitGroup
	msShards := make([][]statsCountUniqSet, shardsLen)
	for i := range msShards {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			perCPU := make([]statsCountUniqSet, cpusCount)
			for i := range perCPU {
				perCPU[i].init()
			}

			sus := ms[idx]

			for ts := range sus.timestamps {
				if needStop(stopCh) {
					return
				}
				k := unsafe.Slice((*byte)(unsafe.Pointer(&ts)), 8)
				h := xxhash.Sum64(k)
				cpuIdx := h % uint64(len(perCPU))
				perCPU[cpuIdx].timestamps[ts] = struct{}{}
			}
			for n := range sus.u64 {
				if needStop(stopCh) {
					return
				}
				k := unsafe.Slice((*byte)(unsafe.Pointer(&n)), 8)
				h := xxhash.Sum64(k)
				cpuIdx := h % uint64(len(perCPU))
				perCPU[cpuIdx].u64[n] = struct{}{}
			}
			for n := range sus.negative64 {
				if needStop(stopCh) {
					return
				}
				k := unsafe.Slice((*byte)(unsafe.Pointer(&n)), 8)
				h := xxhash.Sum64(k)
				cpuIdx := h % uint64(len(perCPU))
				perCPU[cpuIdx].negative64[n] = struct{}{}
			}
			for k := range sus.strings {
				if needStop(stopCh) {
					return
				}
				h := xxhash.Sum64(bytesutil.ToUnsafeBytes(k))
				cpuIdx := h % uint64(len(perCPU))
				perCPU[cpuIdx].strings[k] = struct{}{}
			}

			msShards[idx] = perCPU
			ms[idx].reset()
		}(i)
	}
	wg.Wait()

	perCPUCounts := make([]uint64, cpusCount)
	for i := range perCPUCounts {
		wg.Add(1)
		go func(cpuIdx int) {
			defer wg.Done()

			sus := &msShards[0][cpuIdx]
			for _, perCPU := range msShards[1:] {
				sus.mergeState(&perCPU[cpuIdx], stopCh)
				perCPU[cpuIdx].reset()
			}
			perCPUCounts[cpuIdx] = sus.entriesCount()
		}(i)
	}
	wg.Wait()

	countTotal := uint64(0)
	for _, n := range perCPUCounts {
		countTotal += n
	}
	return countTotal
}

func (sup *statsCountUniqProcessor) updateStateGeneric(v string) int {
	return sup.m.updateStateGeneric(sup.a, v)
}

func (sup *statsCountUniqProcessor) updateStateString(v []byte) int {
	return sup.m.updateStateString(sup.a, bytesutil.ToUnsafeString(v))
}

func (sup *statsCountUniqProcessor) limitReached(su *statsCountUniq) bool {
	limit := su.limit
	if limit <= 0 {
		return false
	}
	return sup.m.entriesCount() > limit
}

func parseStatsCountUniq(lex *lexer) (*statsCountUniq, error) {
	fields, err := parseStatsFuncFields(lex, "count_uniq")
	if err != nil {
		return nil, err
	}
	su := &statsCountUniq{
		fields: fields,
	}
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s' for 'count_uniq': %w", lex.token, err)
		}
		lex.nextToken()
		su.limit = n
	}
	return su, nil
}
