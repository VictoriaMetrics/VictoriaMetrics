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

type statsCountUniqHash struct {
	fields []string
	limit  uint64
}

func (su *statsCountUniqHash) String() string {
	s := "count_uniq_hash(" + statsFuncFieldsToString(su.fields) + ")"
	if su.limit > 0 {
		s += fmt.Sprintf(" limit %d", su.limit)
	}
	return s
}

func (su *statsCountUniqHash) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, su.fields)
}

func (su *statsCountUniqHash) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	sup := a.newStatsCountUniqHashProcessor()
	sup.a = a
	sup.m.init()
	return sup
}

type statsCountUniqHashProcessor struct {
	a *chunkedAllocator

	m  statsCountUniqHashSet
	ms []*statsCountUniqHashSet

	columnValues [][]string
	keyBuf       []byte
	tmpNum       int
}

type statsCountUniqHashSet struct {
	timestamps map[uint64]struct{}
	u64        map[uint64]struct{}
	negative64 map[uint64]struct{}
	strings    map[uint64]struct{}
}

func (sus *statsCountUniqHashSet) reset() {
	sus.timestamps = nil
	sus.u64 = nil
	sus.negative64 = nil
	sus.strings = nil
}

func (sus *statsCountUniqHashSet) init() {
	sus.timestamps = make(map[uint64]struct{})
	sus.u64 = make(map[uint64]struct{})
	sus.negative64 = make(map[uint64]struct{})
	sus.strings = make(map[uint64]struct{})
}

func (sus *statsCountUniqHashSet) entriesCount() uint64 {
	n := len(sus.timestamps) + len(sus.u64) + len(sus.negative64) + len(sus.strings)
	return uint64(n)
}

func (sus *statsCountUniqHashSet) updateStateTimestamp(ts int64) int {
	_, ok := sus.timestamps[uint64(ts)]
	if ok {
		return 0
	}
	sus.timestamps[uint64(ts)] = struct{}{}
	return 8
}

func (sus *statsCountUniqHashSet) updateStateUint64(n uint64) int {
	_, ok := sus.u64[n]
	if ok {
		return 0
	}
	sus.u64[n] = struct{}{}
	return 8
}

func (sus *statsCountUniqHashSet) updateStateInt64(n int64) int {
	if n >= 0 {
		return sus.updateStateUint64(uint64(n))
	}
	return sus.updateStateNegativeInt64(n)
}

func (sus *statsCountUniqHashSet) updateStateNegativeInt64(n int64) int {
	_, ok := sus.negative64[uint64(n)]
	if ok {
		return 0
	}
	sus.negative64[uint64(n)] = struct{}{}
	return 8
}

func (sus *statsCountUniqHashSet) updateStateGeneric(v string) int {
	if n, ok := tryParseUint64(v); ok {
		return sus.updateStateUint64(n)
	}
	if len(v) > 0 && v[0] == '-' {
		if n, ok := tryParseInt64(v); ok {
			return sus.updateStateNegativeInt64(n)
		}
	}
	return sus.updateStateString(bytesutil.ToUnsafeBytes(v))
}

func (sus *statsCountUniqHashSet) updateStateString(v []byte) int {
	h := xxhash.Sum64(v)
	_, ok := sus.strings[h]
	if ok {
		return 0
	}
	sus.strings[h] = struct{}{}
	return 8
}

func (sus *statsCountUniqHashSet) mergeState(src *statsCountUniqHashSet, stopCh <-chan struct{}) {
	mergeUint64Set(sus.timestamps, src.timestamps, stopCh)
	mergeUint64Set(sus.u64, src.u64, stopCh)
	mergeUint64Set(sus.negative64, src.negative64, stopCh)
	mergeUint64Set(sus.strings, src.strings, stopCh)
}

func (sup *statsCountUniqHashProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	su := sf.(*statsCountUniqHash)
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
			stateSizeIncrease += sup.m.updateStateString(keyBuf)
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
		stateSizeIncrease += sup.m.updateStateString(keyBuf)
	}
	sup.keyBuf = keyBuf
	return stateSizeIncrease
}

func (sup *statsCountUniqHashProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	su := sf.(*statsCountUniqHash)
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
		return sup.m.updateStateString(keyBuf)
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
	return sup.m.updateStateString(keyBuf)
}

func (sup *statsCountUniqHashProcessor) updateStatsForAllRowsSingleColumn(br *blockResult, columnName string) int {
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
		return sup.m.updateStateGeneric(v)
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
			sup.tmpNum += sup.m.updateStateGeneric(v)
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
		// Count unique values across column values
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
			stateSizeIncrease += sup.m.updateStateGeneric(v)
		}
		return stateSizeIncrease
	}
}

func (sup *statsCountUniqHashProcessor) updateStatsForRowSingleColumn(br *blockResult, columnName string, rowIdx int) int {
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
		return sup.m.updateStateGeneric(v)
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
		return sup.m.updateStateGeneric(v)
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
		return sup.m.updateStateGeneric(v)
	}
}

func (sup *statsCountUniqHashProcessor) mergeState(sf statsFunc, sfp statsProcessor) {
	su := sf.(*statsCountUniqHash)
	if sup.limitReached(su) {
		return
	}

	src := sfp.(*statsCountUniqHashProcessor)
	if src.m.entriesCount() > 100_000 {
		// Postpone merging too big number of items in parallel
		sup.ms = append(sup.ms, &src.m)
		return
	}

	sup.m.mergeState(&src.m, nil)
}

func (sup *statsCountUniqHashProcessor) finalizeStats(sf statsFunc, dst []byte, stopCh <-chan struct{}) []byte {
	su := sf.(*statsCountUniqHash)
	n := sup.m.entriesCount()
	if len(sup.ms) > 0 {
		sup.ms = append(sup.ms, &sup.m)
		n = countUniqHashParallel(sup.ms, stopCh)
	}

	if limit := su.limit; limit > 0 && n > limit {
		n = limit
	}
	return strconv.AppendUint(dst, n, 10)
}

func countUniqHashParallel(ms []*statsCountUniqHashSet, stopCh <-chan struct{}) uint64 {
	shardsLen := len(ms)
	cpusCount := cgroup.AvailableCPUs()

	var wg sync.WaitGroup
	msShards := make([][]statsCountUniqHashSet, shardsLen)
	for i := range msShards {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			perCPU := make([]statsCountUniqHashSet, cpusCount)
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
			for h := range sus.strings {
				if needStop(stopCh) {
					return
				}
				cpuIdx := h % uint64(len(perCPU))
				perCPU[cpuIdx].strings[h] = struct{}{}
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

func (sup *statsCountUniqHashProcessor) limitReached(su *statsCountUniqHash) bool {
	limit := su.limit
	if limit <= 0 {
		return false
	}
	return sup.m.entriesCount() > limit
}

func parseStatsCountUniqHash(lex *lexer) (*statsCountUniqHash, error) {
	fields, err := parseStatsFuncFields(lex, "count_uniq_hash")
	if err != nil {
		return nil, err
	}
	su := &statsCountUniqHash{
		fields: fields,
	}
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s' for 'count_uniq_hash': %w", lex.token, err)
		}
		lex.nextToken()
		su.limit = n
	}
	return su, nil
}
