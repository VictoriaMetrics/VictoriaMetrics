package logstorage

import (
	"fmt"
	"runtime"
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
	sup.su = su
	return sup
}

type statsCountUniqHashProcessor struct {
	a  *chunkedAllocator
	su *statsCountUniqHash

	m  statsCountUniqHashSet
	ms []statsCountUniqHashSet

	columnValues [][]string
	keyBuf       []byte
	tmpNum       int
}

type statsCountUniqHashSet struct {
	m            map[uint64]struct{}
	entriesCount uint64
}

func (sus *statsCountUniqHashSet) reset() {
	sus.m = nil
	sus.entriesCount = 0
}

func (sus *statsCountUniqHashSet) updateStateTimestamp(ts int64) int {
	v := unsafe.Slice((*byte)(unsafe.Pointer(&ts)), 8)
	h := xxhash.Sum64(v)
	runtime.KeepAlive(ts)
	return sus.updateStateHash(h)
}

func (sus *statsCountUniqHashSet) updateState(v string) int {
	h := xxhash.Sum64(bytesutil.ToUnsafeBytes(v))
	return sus.updateStateHash(h)
}

func (sus *statsCountUniqHashSet) updateStateHash(h uint64) int {
	_, ok := sus.m[h]
	if ok {
		return 0
	}
	if sus.m == nil {
		sus.m = make(map[uint64]struct{})
	}
	sus.m[h] = struct{}{}
	sus.entriesCount++
	return 8
}

func (sus *statsCountUniqHashSet) mergeState(src *statsCountUniqHashSet) {
	for h := range src.m {
		sus.updateStateHash(h)
	}
}

func (sup *statsCountUniqHashProcessor) updateStatsForAllRows(br *blockResult) int {
	if sup.limitReached() {
		return 0
	}

	fields := sup.su.fields

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
			stateSizeIncrease += sup.m.updateState(bytesutil.ToUnsafeString(keyBuf))
		}
		sup.keyBuf = keyBuf
		return stateSizeIncrease
	}
	if len(fields) == 1 {
		// Fast path for a single column.
		c := br.getColumnByName(fields[0])
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
			return sup.m.updateState(v)
		}
		if c.valueType == valueTypeDict {
			// count unique non-zero dict values for the selected logs
			sup.tmpNum = 0
			c.forEachDictValue(br, func(v string) {
				if v == "" {
					// Do not count empty values
					return
				}
				sup.tmpNum += sup.m.updateState(v)
			})
			return sup.tmpNum
		}

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
			stateSizeIncrease += sup.m.updateState(v)
		}
		return stateSizeIncrease
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
		stateSizeIncrease += sup.m.updateState(bytesutil.ToUnsafeString(keyBuf))
	}
	sup.keyBuf = keyBuf
	return stateSizeIncrease
}

func (sup *statsCountUniqHashProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	if sup.limitReached() {
		return 0
	}

	fields := sup.su.fields

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
		return sup.m.updateState(bytesutil.ToUnsafeString(keyBuf))
	}
	if len(fields) == 1 {
		// Fast path for a single column.
		c := br.getColumnByName(fields[0])
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
			return sup.m.updateState(v)
		}
		if c.valueType == valueTypeDict {
			// count unique non-zero c.dictValues
			valuesEncoded := c.getValuesEncoded(br)
			dictIdx := valuesEncoded[rowIdx][0]
			v := c.dictValues[dictIdx]
			if v == "" {
				// Do not count empty values
				return 0
			}
			return sup.m.updateState(v)
		}

		// Count unique values for the given rowIdx
		v := c.getValueAtRow(br, rowIdx)
		if v == "" {
			// Do not count empty values
			return 0
		}
		return sup.m.updateState(v)
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
	return sup.m.updateState(bytesutil.ToUnsafeString(keyBuf))
}

func (sup *statsCountUniqHashProcessor) mergeState(sfp statsProcessor) {
	if sup.limitReached() {
		return
	}

	src := sfp.(*statsCountUniqHashProcessor)
	if src.m.entriesCount > 100_000 {
		// Postpone merging too big number of items in parallel
		sup.ms = append(sup.ms, src.m)
		return
	}

	sup.m.mergeState(&src.m)
}

func (sup *statsCountUniqHashProcessor) finalizeStats(dst []byte) []byte {
	n := sup.m.entriesCount
	if len(sup.ms) > 0 {
		sup.ms = append(sup.ms, sup.m)
		n = countUniqHashParallel(sup.ms)
	}

	if limit := sup.su.limit; limit > 0 && n > limit {
		n = limit
	}
	return strconv.AppendUint(dst, n, 10)
}

func countUniqHashParallel(ms []statsCountUniqHashSet) uint64 {
	shardsLen := len(ms)
	cpusCount := cgroup.AvailableCPUs()

	var wg sync.WaitGroup
	msShards := make([][]statsCountUniqHashSet, shardsLen)
	for i := range msShards {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			perCPU := make([]statsCountUniqHashSet, cpusCount)
			for h := range ms[idx].m {
				cpuIdx := h % uint64(len(perCPU))
				perCPU[cpuIdx].updateStateHash(h)
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
				sus.mergeState(&perCPU[cpuIdx])
				perCPU[cpuIdx].reset()
			}
			perCPUCounts[cpuIdx] = sus.entriesCount
		}(i)
	}
	wg.Wait()

	countTotal := uint64(0)
	for _, n := range perCPUCounts {
		countTotal += n
	}
	return countTotal
}

func (sup *statsCountUniqHashProcessor) limitReached() bool {
	limit := sup.su.limit
	if limit <= 0 {
		return false
	}
	return sup.m.entriesCount > limit
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
