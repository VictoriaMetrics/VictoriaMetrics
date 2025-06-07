package logstorage

import (
	"fmt"
	"strconv"
	"sync"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

type statsCountUniqHash struct {
	fields []string
	limit  uint64
}

func (su *statsCountUniqHash) String() string {
	s := "count_uniq_hash(" + fieldNamesString(su.fields) + ")"
	if su.limit > 0 {
		s += fmt.Sprintf(" limit %d", su.limit)
	}
	return s
}

func (su *statsCountUniqHash) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(su.fields)
}

func (su *statsCountUniqHash) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	sup := a.newStatsCountUniqHashProcessor()
	sup.a = a
	return sup
}

type statsCountUniqHashProcessor struct {
	a *chunkedAllocator

	// concurrency is the number of parallel workers to use when merging shards.
	//
	// this field must be updated by the caller before using statsCountUniqHashProcessor.
	concurrency uint

	// uniqValues is used for tracking small number of unique values until it reaches statsCountUniqHashValuesMaxLen.
	// After that the unique values are tracked by shards.
	uniqValues statsCountUniqHashSet

	// shards are used for tracking big number of unique values.
	//
	// Every shard contains a share of unique values, which are merged in parallel at finalizeStats().
	shards []statsCountUniqHashSet

	// shardss is used for collecting shards from other statsCountUniqProcessor instances at mergeState().
	shardss [][]statsCountUniqHashSet

	columnValues [][]string
	keyBuf       []byte
	tmpNum       int
}

// the maximum number of values to track in statsCountUniqHashProcessor.uniqValues before switching to statsCountUniqHashProcessor.shards
//
// Too big value may slow down mergeState() across big number of CPU cores.
// Too small value may significantly increase RAM usage when coun_uniq_hash() is applied individually to big number of groups.
const statsCountUniqHashValuesMaxLen = 4 << 10

type statsCountUniqHashSet struct {
	timestamps map[uint64]struct{}
	u64        map[uint64]struct{}
	negative64 map[uint64]struct{}
	strings    map[uint64]struct{}
}

func (sus *statsCountUniqHashSet) reset() {
	*sus = statsCountUniqHashSet{}
}

func (sus *statsCountUniqHashSet) entriesCount() uint64 {
	n := len(sus.timestamps) + len(sus.u64) + len(sus.negative64) + len(sus.strings)
	return uint64(n)
}

func (sus *statsCountUniqHashSet) exportState(dst []byte, stopCh <-chan struct{}) []byte {
	dst = marshalUint64Set(dst, sus.timestamps, stopCh)
	dst = marshalUint64Set(dst, sus.u64, stopCh)
	dst = marshalUint64Set(dst, sus.negative64, stopCh)
	dst = marshalUint64Set(dst, sus.strings, stopCh)
	return dst
}

func (sus *statsCountUniqHashSet) importState(src []byte, stopCh <-chan struct{}) ([]byte, int, error) {
	stateSizeIncrease := 0

	tail, stateSize, err := unmarshalUint64Set(&sus.timestamps, src, stopCh)
	if err != nil {
		return tail, 0, fmt.Errorf("cannot unmarshal timestamps: %w", err)
	}
	src = tail
	stateSizeIncrease += stateSize

	tail, stateSize, err = unmarshalUint64Set(&sus.u64, src, stopCh)
	if err != nil {
		return tail, 0, fmt.Errorf("cannot unmarshal uint64 values: %w", err)
	}
	src = tail
	stateSizeIncrease += stateSize

	tail, stateSize, err = unmarshalUint64Set(&sus.negative64, src, stopCh)
	if err != nil {
		return tail, 0, fmt.Errorf("cannot unmarshal negative64 values: %w", err)
	}
	src = tail
	stateSizeIncrease += stateSize

	tail, stateSize, err = unmarshalUint64Set(&sus.strings, src, stopCh)
	if err != nil {
		return tail, 0, fmt.Errorf("cannot unmarshal string values: %w", err)
	}
	src = tail
	stateSizeIncrease += stateSize

	return src, stateSizeIncrease, nil
}

func (sus *statsCountUniqHashSet) updateStateTimestamp(ts int64) int {
	return updateUint64Set(&sus.timestamps, uint64(ts))
}

func (sus *statsCountUniqHashSet) updateStateUint64(n uint64) int {
	return updateUint64Set(&sus.timestamps, n)
}

func (sus *statsCountUniqHashSet) updateStateNegativeInt64(n int64) int {
	return updateUint64Set(&sus.negative64, uint64(n))
}

func (sus *statsCountUniqHashSet) updateStateStringHash(h uint64) int {
	return updateUint64Set(&sus.strings, h)
}

func (sus *statsCountUniqHashSet) mergeState(src *statsCountUniqHashSet, stopCh <-chan struct{}) {
	mergeUint64Set(&sus.timestamps, src.timestamps, stopCh)
	mergeUint64Set(&sus.u64, src.u64, stopCh)
	mergeUint64Set(&sus.negative64, src.negative64, stopCh)
	mergeUint64Set(&sus.strings, src.strings, stopCh)
}

func (sup *statsCountUniqHashProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	su := sf.(*statsCountUniqHash)
	if sup.limitReached(su) {
		return 0
	}

	stateSizeIncrease := 0

	if len(su.fields) == 1 {
		// Fast path for a single column.
		return sup.updateStatsForAllRowsSingleColumn(br, su.fields[0])
	}

	// Slow path for multiple columns.

	// Pre-calculate column values for byFields in order to speed up building group key in the loop below.
	columnValues := sup.columnValues[:0]
	for _, f := range su.fields {
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

func (sup *statsCountUniqHashProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	su := sf.(*statsCountUniqHash)
	if sup.limitReached(su) {
		return 0
	}

	if len(su.fields) == 1 {
		// Fast path for a single column.
		return sup.updateStatsForRowSingleColumn(br, su.fields[0], rowIdx)
	}

	// Slow path for multiple columns.
	allEmptyValues := true
	keyBuf := sup.keyBuf[:0]

	for _, f := range su.fields {
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

func (sup *statsCountUniqHashProcessor) updateStatsForAllRowsSingleColumn(br *blockResult, columnName string) int {
	stateSizeIncrease := 0
	c := br.getColumnByName(columnName)
	if c.isTime {
		// Count unique timestamps
		timestamps := br.getTimestamps()
		for i := range timestamps {
			if i > 0 && timestamps[i-1] == timestamps[i] {
				// This timestamp has been already counted.
				continue
			}
			stateSizeIncrease += sup.updateStateTimestamp(timestamps[i])
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
			stateSizeIncrease += sup.updateStateUint64(uint64(n))
		}
		return stateSizeIncrease
	case valueTypeUint16:
		values := c.getValuesEncoded(br)
		for i, v := range values {
			if i > 0 && values[i-1] == v {
				continue
			}
			n := unmarshalUint16(v)
			stateSizeIncrease += sup.updateStateUint64(uint64(n))
		}
		return stateSizeIncrease
	case valueTypeUint32:
		values := c.getValuesEncoded(br)
		for i, v := range values {
			if i > 0 && values[i-1] == v {
				continue
			}
			n := unmarshalUint32(v)
			stateSizeIncrease += sup.updateStateUint64(uint64(n))
		}
		return stateSizeIncrease
	case valueTypeUint64:
		values := c.getValuesEncoded(br)
		for i, v := range values {
			if i > 0 && values[i-1] == v {
				continue
			}
			n := unmarshalUint64(v)
			stateSizeIncrease += sup.updateStateUint64(n)
		}
		return stateSizeIncrease
	case valueTypeInt64:
		values := c.getValuesEncoded(br)
		for i, v := range values {
			if i > 0 && values[i-1] == v {
				continue
			}
			n := unmarshalInt64(v)
			stateSizeIncrease += sup.updateStateInt64(n)
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
			stateSizeIncrease += sup.updateStateGeneric(v)
		}
		return stateSizeIncrease
	}
}

func (sup *statsCountUniqHashProcessor) updateStatsForRowSingleColumn(br *blockResult, columnName string, rowIdx int) int {
	c := br.getColumnByName(columnName)
	if c.isTime {
		// Count unique timestamps
		timestamps := br.getTimestamps()
		return sup.updateStateTimestamp(timestamps[rowIdx])
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
		return sup.updateStateUint64(uint64(n))
	case valueTypeUint16:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalUint16(v)
		return sup.updateStateUint64(uint64(n))
	case valueTypeUint32:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalUint32(v)
		return sup.updateStateUint64(uint64(n))
	case valueTypeUint64:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalUint64(v)
		return sup.updateStateUint64(n)
	case valueTypeInt64:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalInt64(v)
		return sup.updateStateInt64(n)
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

func (sup *statsCountUniqHashProcessor) mergeState(a *chunkedAllocator, sf statsFunc, sfp statsProcessor) {
	su := sf.(*statsCountUniqHash)
	if sup.limitReached(su) {
		return
	}

	src := sfp.(*statsCountUniqHashProcessor)

	if sup.shards == nil {
		if src.shards == nil {
			sup.uniqValues.mergeState(&src.uniqValues, nil)
			src.uniqValues.reset()
			sup.probablyMoveUniqValuesToShards(a)
			return
		}
		sup.moveUniqValuesToShards(a)
	}

	if src.shards == nil {
		src.moveUniqValuesToShards(a)
	}
	sup.shardss = append(sup.shardss, src.shards)
	src.shards = nil
}

func (sup *statsCountUniqHashProcessor) exportState(dst []byte, stopCh <-chan struct{}) []byte {
	sup.mergeShardssParallel(stopCh)

	if sup.shards == nil {
		dst = encoding.MarshalVarUint64(dst, uint64(1))
		dst = sup.uniqValues.exportState(dst, stopCh)
	} else {
		dst = encoding.MarshalVarUint64(dst, uint64(len(sup.shards)))
		for i := range sup.shards {
			dst = sup.shards[i].exportState(dst, stopCh)
		}
	}
	return dst
}

func (sup *statsCountUniqHashProcessor) importState(src []byte, stopCh <-chan struct{}) (int, error) {
	shardsLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot read the number of shards")
	}
	if shardsLen < 1 {
		return 0, fmt.Errorf("the number of shards must be at least 1")
	}
	src = src[1:]

	if shardsLen == 1 {
		tail, stateSize, err := sup.uniqValues.importState(src, stopCh)
		if err != nil {
			return 0, fmt.Errorf("cannot read uniqValues state: %w", err)
		}
		if len(tail) > 0 {
			return 0, fmt.Errorf("unexpected tail left after importing uniqValues state; len(tail)=%d", len(tail))
		}
		return stateSize, nil
	}

	if shardsLen != uint64(sup.concurrency) {
		return 0, fmt.Errorf("unexpected number of imported shards: %d; want %d", shardsLen, sup.concurrency)
	}

	shards := make([]statsCountUniqHashSet, shardsLen)
	stateSizeIncrease := int(unsafe.Sizeof(shards[0])) * len(shards)
	for i := range shards {
		tail, stateSize, err := shards[i].importState(src, stopCh)
		if err != nil {
			return 0, fmt.Errorf("cannot read state for shard[%d]: %w", i, err)
		}
		src = tail

		stateSizeIncrease += stateSize
	}
	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected tail left after importing shards' state; len(tail)=%d", len(src))
	}
	sup.shards = shards

	return stateSizeIncrease, nil
}

func (sup *statsCountUniqHashProcessor) finalizeStats(sf statsFunc, dst []byte, stopCh <-chan struct{}) []byte {
	sup.mergeShardssParallel(stopCh)

	n := sup.entriesCount()

	su := sf.(*statsCountUniqHash)
	if limit := su.limit; limit > 0 && n > limit {
		n = limit
	}
	return strconv.AppendUint(dst, n, 10)
}

func (sup *statsCountUniqHashProcessor) mergeShardssParallel(stopCh <-chan struct{}) {
	if len(sup.shardss) == 0 {
		// nothing to merge
		return
	}

	shardss := sup.shardss
	sup.shardss = nil
	if sup.shards != nil {
		shardss = append(shardss, sup.shards)
		sup.shards = nil
	}

	result := make([]statsCountUniqHashSet, len(shardss[0]))
	var wg sync.WaitGroup
	for i := range result {
		wg.Add(1)
		go func(cpuIdx int) {
			defer wg.Done()

			sus := &shardss[0][cpuIdx]
			for _, perCPU := range shardss[1:] {
				sus.mergeState(&perCPU[cpuIdx], stopCh)
				perCPU[cpuIdx].reset()
			}
			result[cpuIdx] = *sus
		}(i)
	}
	wg.Wait()

	sup.shards = result
}

func (sup *statsCountUniqHashProcessor) entriesCount() uint64 {
	if sup.shards == nil {
		return sup.uniqValues.entriesCount()
	}
	n := uint64(0)
	shards := sup.shards
	for i := range shards {
		n += shards[i].entriesCount()
	}
	return n
}

func (sup *statsCountUniqHashProcessor) updateStateGeneric(v string) int {
	if n, ok := tryParseUint64(v); ok {
		return sup.updateStateUint64(n)
	}
	if len(v) > 0 && v[0] == '-' {
		if n, ok := tryParseInt64(v); ok {
			return sup.updateStateNegativeInt64(n)
		}
	}
	return sup.updateStateString(bytesutil.ToUnsafeBytes(v))
}

func (sup *statsCountUniqHashProcessor) updateStateInt64(n int64) int {
	if n >= 0 {
		return sup.updateStateUint64(uint64(n))
	}
	return sup.updateStateNegativeInt64(n)
}

func (sup *statsCountUniqHashProcessor) updateStateString(v []byte) int {
	h := xxhash.Sum64(v)
	if sup.shards == nil {
		stateSizeIncrease := sup.uniqValues.updateStateStringHash(h)
		if stateSizeIncrease > 0 {
			stateSizeIncrease += sup.probablyMoveUniqValuesToShards(sup.a)
		}
		return stateSizeIncrease
	}
	return sup.updateStateStringHash(h)
}

func (sup *statsCountUniqHashProcessor) updateStateStringHash(h uint64) int {
	sus := sup.getShardByStringHash(h)
	return sus.updateStateStringHash(h)
}

func (sup *statsCountUniqHashProcessor) updateStateTimestamp(ts int64) int {
	if sup.shards == nil {
		stateSizeIncrease := sup.uniqValues.updateStateTimestamp(ts)
		if stateSizeIncrease > 0 {
			stateSizeIncrease += sup.probablyMoveUniqValuesToShards(sup.a)
		}
		return stateSizeIncrease
	}
	sus := sup.getShardByUint64(uint64(ts))
	return sus.updateStateTimestamp(ts)
}

func (sup *statsCountUniqHashProcessor) updateStateUint64(n uint64) int {
	if sup.shards == nil {
		stateSizeIncrease := sup.uniqValues.updateStateUint64(n)
		if stateSizeIncrease > 0 {
			stateSizeIncrease += sup.probablyMoveUniqValuesToShards(sup.a)
		}
		return stateSizeIncrease
	}
	sus := sup.getShardByUint64(n)
	return sus.updateStateUint64(n)
}

func (sup *statsCountUniqHashProcessor) updateStateNegativeInt64(n int64) int {
	if sup.shards == nil {
		stateSizeIncrease := sup.uniqValues.updateStateNegativeInt64(n)
		if stateSizeIncrease > 0 {
			stateSizeIncrease += sup.probablyMoveUniqValuesToShards(sup.a)
		}
		return stateSizeIncrease
	}
	sus := sup.getShardByUint64(uint64(n))
	return sus.updateStateNegativeInt64(n)
}

func (sup *statsCountUniqHashProcessor) probablyMoveUniqValuesToShards(a *chunkedAllocator) int {
	if sup.uniqValues.entriesCount() < statsCountUniqHashValuesMaxLen {
		return 0
	}
	return sup.moveUniqValuesToShards(a)
}

func (sup *statsCountUniqHashProcessor) moveUniqValuesToShards(a *chunkedAllocator) int {
	cpusCount := sup.concurrency
	bytesAllocatedPrev := a.bytesAllocated
	sup.shards = a.newStatsCountUniqHashSets(cpusCount)
	stateSizeIncrease := a.bytesAllocated - bytesAllocatedPrev

	for ts := range sup.uniqValues.timestamps {
		sus := sup.getShardByUint64(ts)
		setUint64Set(&sus.timestamps, ts)
	}
	for n := range sup.uniqValues.u64 {
		sus := sup.getShardByUint64(n)
		setUint64Set(&sus.u64, n)
	}
	for n := range sup.uniqValues.negative64 {
		sus := sup.getShardByUint64(n)
		setUint64Set(&sus.negative64, n)
	}
	for h := range sup.uniqValues.strings {
		sus := sup.getShardByStringHash(h)
		setUint64Set(&sus.strings, h)
	}

	sup.uniqValues.reset()

	return stateSizeIncrease
}

func (sup *statsCountUniqHashProcessor) getShardByStringHash(h uint64) *statsCountUniqHashSet {
	cpuIdx := h % uint64(len(sup.shards))
	return &sup.shards[cpuIdx]
}

func (sup *statsCountUniqHashProcessor) getShardByUint64(n uint64) *statsCountUniqHashSet {
	h := fastHashUint64(n)
	cpuIdx := h % uint64(len(sup.shards))
	return &sup.shards[cpuIdx]
}

func (sup *statsCountUniqHashProcessor) limitReached(su *statsCountUniqHash) bool {
	limit := su.limit
	if limit <= 0 {
		return false
	}
	return sup.entriesCount() > limit
}

func parseStatsCountUniqHash(lex *lexer) (*statsCountUniqHash, error) {
	fields, err := parseStatsFuncFields(lex, "count_uniq_hash")
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("expecting at least a single field")
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

func fastHashUint64(x uint64) uint64 {
	x ^= x >> 12 // a
	x ^= x << 25 // b
	x ^= x >> 27 // c
	return x * 2685821657736338717
}
