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
	sup.su = su
	return sup
}

type statsCountUniqProcessor struct {
	a  *chunkedAllocator
	su *statsCountUniq

	m  statsCountUniqSet
	ms []statsCountUniqSet

	columnValues [][]string
	keyBuf       []byte
	tmpNum       int
}

type statsCountUniqSet struct {
	m            perSizeSet
	timestamps   map[int64]struct{}
	entriesCount uint64
}

func (sus *statsCountUniqSet) reset() {
	sus.m.reset()
	sus.timestamps = nil
	sus.entriesCount = 0
}

func (sus *statsCountUniqSet) updateStateTimestamp(ts int64) int {
	_, ok := sus.timestamps[ts]
	if ok {
		return 0
	}
	if sus.timestamps == nil {
		sus.timestamps = make(map[int64]struct{})
	}
	sus.timestamps[ts] = struct{}{}
	sus.entriesCount++
	return 8
}

func (sus *statsCountUniqSet) updateState(a *chunkedAllocator, v string) int {
	stateSize := sus.m.add(a, v)
	if stateSize > 0 {
		sus.entriesCount++
	}
	return stateSize
}

func (sus *statsCountUniqSet) mergeState(src *statsCountUniqSet) {
	src.m.forEachKey(func(k string) {
		stateSize := sus.m.add(nil, k)
		if stateSize > 0 {
			sus.entriesCount++
		}
	})
	for ts := range src.timestamps {
		_, ok := sus.timestamps[ts]
		if !ok {
			sus.timestamps[ts] = struct{}{}
			sus.entriesCount++
		}
	}
}

type perSizeSet struct {
	u64      map[uint64]struct{}
	i64      map[int64]struct{}
	mGeneric map[string]struct{}
}

func (s *perSizeSet) reset() {
	s.u64 = nil
	s.i64 = nil
	s.mGeneric = nil
}

func (s *perSizeSet) forEachKey(f func(k string)) {
	bb := bbPool.Get()
	for n := range s.u64 {
		bb.B = marshalUint64String(bb.B[:0], n)
		f(bytesutil.ToUnsafeString(bb.B))
	}
	for n := range s.i64 {
		bb.B = marshalInt64String(bb.B[:0], n)
		f(bytesutil.ToUnsafeString(bb.B))
	}
	bbPool.Put(bb)

	for k := range s.mGeneric {
		f(k)
	}
}

func (s *perSizeSet) add(a *chunkedAllocator, k string) int {
	if n, ok := tryParseUint64(k); ok {
		if s.u64 == nil {
			s.u64 = make(map[uint64]struct{})
		}
		_, ok := s.u64[n]
		if ok {
			return 0
		}
		s.u64[n] = struct{}{}
		return 8
	}
	if n, ok := tryParseInt64(k); ok {
		if s.i64 == nil {
			s.i64 = make(map[int64]struct{})
		}
		_, ok := s.i64[n]
		if ok {
			return 0
		}
		s.i64[n] = struct{}{}
		return 8
	}

	if s.mGeneric == nil {
		s.mGeneric = make(map[string]struct{})
	}
	_, ok := s.mGeneric[k]
	if ok {
		return 0
	}
	stateSize := int(unsafe.Sizeof(k))
	kCopy := k
	if a != nil {
		stateSize += len(k)
		kCopy = a.cloneString(k)
	}
	s.mGeneric[kCopy] = struct{}{}
	return stateSize
}

func (sup *statsCountUniqProcessor) updateStatsForAllRows(br *blockResult) int {
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
			stateSizeIncrease += sup.updateState(bytesutil.ToUnsafeString(keyBuf))
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
			return sup.updateState(v)
		}
		if c.valueType == valueTypeDict {
			// count unique non-zero dict values for the selected logs
			sup.tmpNum = 0
			c.forEachDictValue(br, func(v string) {
				if v == "" {
					// Do not count empty values
					return
				}
				sup.tmpNum += sup.updateState(v)
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
			stateSizeIncrease += sup.updateState(v)
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
		stateSizeIncrease += sup.updateState(bytesutil.ToUnsafeString(keyBuf))
	}
	sup.keyBuf = keyBuf
	return stateSizeIncrease
}

func (sup *statsCountUniqProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
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
		return sup.updateState(bytesutil.ToUnsafeString(keyBuf))
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
			return sup.updateState(v)
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
			return sup.updateState(v)
		}

		// Count unique values for the given rowIdx
		v := c.getValueAtRow(br, rowIdx)
		if v == "" {
			// Do not count empty values
			return 0
		}
		return sup.updateState(v)
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
	return sup.updateState(bytesutil.ToUnsafeString(keyBuf))
}

func (sup *statsCountUniqProcessor) mergeState(sfp statsProcessor) {
	if sup.limitReached() {
		return
	}

	src := sfp.(*statsCountUniqProcessor)
	if src.m.entriesCount > 100_000 {
		// Postpone merging too big number of items in parallel
		sup.ms = append(sup.ms, src.m)
		return
	}

	sup.m.mergeState(&src.m)
}

func (sup *statsCountUniqProcessor) finalizeStats(dst []byte) []byte {
	n := sup.m.entriesCount
	if len(sup.ms) > 0 {
		sup.ms = append(sup.ms, sup.m)
		n = countUniqParallel(sup.ms)
	}

	if limit := sup.su.limit; limit > 0 && n > limit {
		n = limit
	}
	return strconv.AppendUint(dst, n, 10)
}

func countUniqParallel(ms []statsCountUniqSet) uint64 {
	shardsLen := len(ms)
	cpusCount := cgroup.AvailableCPUs()

	var wg sync.WaitGroup
	msShards := make([][]statsCountUniqSet, shardsLen)
	for i := range msShards {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			perCPU := make([]statsCountUniqSet, cpusCount)
			ms[idx].m.forEachKey(func(k string) {
				h := xxhash.Sum64(bytesutil.ToUnsafeBytes(k))
				cpuIdx := h % uint64(len(perCPU))
				perCPU[cpuIdx].updateState(nil, k)
			})
			for ts := range ms[idx].timestamps {
				k := unsafe.Slice((*byte)(unsafe.Pointer(&ts)), 8)
				h := xxhash.Sum64(k)
				cpuIdx := h % uint64(len(perCPU))
				perCPU[cpuIdx].updateStateTimestamp(ts)
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

func (sup *statsCountUniqProcessor) updateState(v string) int {
	return sup.m.updateState(sup.a, v)
}

func (sup *statsCountUniqProcessor) limitReached() bool {
	limit := sup.su.limit
	if limit <= 0 {
		return false
	}
	return sup.m.entriesCount > limit
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
