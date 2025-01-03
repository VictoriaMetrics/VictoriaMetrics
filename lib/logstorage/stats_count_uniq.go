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
	sup.m = make(map[string]struct{})
	return sup
}

type statsCountUniqProcessor struct {
	a  *chunkedAllocator
	su *statsCountUniq

	m  map[string]struct{}
	ms []map[string]struct{}

	columnValues [][]string
	keyBuf       []byte
	tmpNum       int
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
			stateSizeIncrease += sup.updateState(keyBuf)
		}
		sup.keyBuf = keyBuf
		return stateSizeIncrease
	}
	if len(fields) == 1 {
		// Fast path for a single column.
		// The unique key is formed as "<is_time> <value>",
		// This guarantees that keys do not clash for different column types across blocks.
		c := br.getColumnByName(fields[0])
		if c.isTime {
			// Count unique timestamps
			timestamps := br.getTimestamps()
			keyBuf := sup.keyBuf[:0]
			for i, timestamp := range timestamps {
				if i > 0 && timestamps[i-1] == timestamps[i] {
					// This timestamp has been already counted.
					continue
				}
				keyBuf = append(keyBuf[:0], 1)
				keyBuf = encoding.MarshalInt64(keyBuf, timestamp)
				stateSizeIncrease += sup.updateState(keyBuf)
			}
			sup.keyBuf = keyBuf
			return stateSizeIncrease
		}
		if c.isConst {
			// count unique const values
			v := c.valuesEncoded[0]
			if v == "" {
				// Do not count empty values
				return stateSizeIncrease
			}
			keyBuf := sup.keyBuf[:0]
			keyBuf = append(keyBuf[:0], 0)
			keyBuf = append(keyBuf, v...)
			stateSizeIncrease += sup.updateState(keyBuf)
			sup.keyBuf = keyBuf
			return stateSizeIncrease
		}
		if c.valueType == valueTypeDict {
			// count unique non-zero dict values for the selected logs
			sup.tmpNum = 0
			c.forEachDictValue(br, func(v string) {
				if v == "" {
					// Do not count empty values
					return
				}
				keyBuf := append(sup.keyBuf[:0], 0)
				keyBuf = append(keyBuf, v...)
				sup.tmpNum += sup.updateState(keyBuf)
				sup.keyBuf = keyBuf
			})
			stateSizeIncrease += sup.tmpNum
			return stateSizeIncrease
		}

		// Count unique values across values
		values := c.getValues(br)
		keyBuf := sup.keyBuf[:0]
		for i, v := range values {
			if v == "" {
				// Do not count empty values
				continue
			}
			if i > 0 && values[i-1] == v {
				// This value has been already counted.
				continue
			}
			keyBuf = append(keyBuf[:0], 0)
			keyBuf = append(keyBuf, v...)
			stateSizeIncrease += sup.updateState(keyBuf)
		}
		sup.keyBuf = keyBuf
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
		stateSizeIncrease += sup.updateState(keyBuf)
	}
	sup.keyBuf = keyBuf
	return stateSizeIncrease
}

func (sup *statsCountUniqProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	if sup.limitReached() {
		return 0
	}

	fields := sup.su.fields

	stateSizeIncrease := 0
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
			return stateSizeIncrease
		}
		stateSizeIncrease += sup.updateState(keyBuf)
		return stateSizeIncrease
	}
	if len(fields) == 1 {
		// Fast path for a single column.
		// The unique key is formed as "<is_time> <value>",
		// This guarantees that keys do not clash for different column types across blocks.
		c := br.getColumnByName(fields[0])
		if c.isTime {
			// Count unique timestamps
			timestamps := br.getTimestamps()
			keyBuf := sup.keyBuf[:0]
			keyBuf = append(keyBuf[:0], 1)
			keyBuf = encoding.MarshalInt64(keyBuf, timestamps[rowIdx])
			stateSizeIncrease += sup.updateState(keyBuf)
			sup.keyBuf = keyBuf
			return stateSizeIncrease
		}
		if c.isConst {
			// count unique const values
			v := c.valuesEncoded[0]
			if v == "" {
				// Do not count empty values
				return stateSizeIncrease
			}
			keyBuf := sup.keyBuf[:0]
			keyBuf = append(keyBuf[:0], 0)
			keyBuf = append(keyBuf, v...)
			stateSizeIncrease += sup.updateState(keyBuf)
			sup.keyBuf = keyBuf
			return stateSizeIncrease
		}
		if c.valueType == valueTypeDict {
			// count unique non-zero c.dictValues
			valuesEncoded := c.getValuesEncoded(br)
			dictIdx := valuesEncoded[rowIdx][0]
			v := c.dictValues[dictIdx]
			if v == "" {
				// Do not count empty values
				return stateSizeIncrease
			}
			keyBuf := sup.keyBuf[:0]
			keyBuf = append(keyBuf[:0], 0)
			keyBuf = append(keyBuf, v...)
			stateSizeIncrease += sup.updateState(keyBuf)
			sup.keyBuf = keyBuf
			return stateSizeIncrease
		}

		// Count unique values for the given rowIdx
		v := c.getValueAtRow(br, rowIdx)
		if v == "" {
			// Do not count empty values
			return stateSizeIncrease
		}
		keyBuf := sup.keyBuf[:0]
		keyBuf = append(keyBuf[:0], 0)
		keyBuf = append(keyBuf, v...)
		stateSizeIncrease += sup.updateState(keyBuf)
		sup.keyBuf = keyBuf
		return stateSizeIncrease
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
		return stateSizeIncrease
	}
	stateSizeIncrease += sup.updateState(keyBuf)
	return stateSizeIncrease
}

func (sup *statsCountUniqProcessor) mergeState(sfp statsProcessor) {
	if sup.limitReached() {
		return
	}

	src := sfp.(*statsCountUniqProcessor)
	if len(src.m) > 100_000 {
		// Postpone merging too big number of items in parallel
		sup.ms = append(sup.ms, src.m)
		return
	}

	m := sup.m
	for k := range src.m {
		if _, ok := m[k]; !ok {
			m[k] = struct{}{}
		}
	}
}

func (sup *statsCountUniqProcessor) finalizeStats(dst []byte) []byte {
	var n uint64
	if len(sup.ms) > 0 {
		sup.ms = append(sup.ms, sup.m)
		n = countUniqParallel(sup.ms)
	} else {
		n = uint64(len(sup.m))
	}

	if limit := sup.su.limit; limit > 0 && n > limit {
		n = limit
	}
	return strconv.AppendUint(dst, n, 10)
}

func countUniqParallel(ms []map[string]struct{}) uint64 {
	shardsLen := len(ms)
	cpusCount := cgroup.AvailableCPUs()

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
				h := xxhash.Sum64(bytesutil.ToUnsafeBytes(s))
				m := perCPU[h%uint64(len(perCPU))]
				m[s] = struct{}{}
			}

			msShards[idx] = perCPU
			ms[idx] = nil
		}(i)
	}
	wg.Wait()

	perCPUCounts := make([]int, cpusCount)
	for i := range perCPUCounts {
		wg.Add(1)
		go func(cpuIdx int) {
			defer wg.Done()

			m := msShards[0][cpuIdx]
			for _, perCPU := range msShards[1:] {
				for s := range perCPU[cpuIdx] {
					if _, ok := m[s]; !ok {
						m[s] = struct{}{}
					}
				}
				perCPU[cpuIdx] = nil
			}
			perCPUCounts[cpuIdx] = len(m)
		}(i)
	}
	wg.Wait()

	countTotal := uint64(0)
	for _, count := range perCPUCounts {
		countTotal += uint64(count)
	}
	return countTotal
}

func (sup *statsCountUniqProcessor) updateState(v []byte) int {
	stateSizeIncrease := 0

	if _, ok := sup.m[string(v)]; !ok {
		vCopy := sup.a.cloneBytesToString(v)
		sup.m[vCopy] = struct{}{}
		stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
	}

	return stateSizeIncrease
}

func (sup *statsCountUniqProcessor) limitReached() bool {
	limit := sup.su.limit
	if limit <= 0 {
		return false
	}
	return uint64(len(sup.m)) > limit
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
