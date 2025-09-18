package logstorage

import (
	"fmt"
	"strconv"
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsCountUniq struct {
	fields []string
	limit  uint64
}

func (su *statsCountUniq) String() string {
	s := "count_uniq(" + fieldNamesString(su.fields) + ")"
	if su.limit > 0 {
		s += fmt.Sprintf(" limit %d", su.limit)
	}
	return s
}

func (su *statsCountUniq) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(su.fields)
}

func (su *statsCountUniq) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	sup := a.newStatsCountUniqProcessor()
	sup.a = a
	return sup
}

type statsCountUniqProcessor struct {
	a *chunkedAllocator

	// concurrency is the number of parallel workers to use when merging shards.
	//
	// this field must be updated by the caller before using statsCountUniqProcessor.
	concurrency uint

	// uniqValues is used for tracking small number of unique values until it reaches statsCountUniqValuesMaxLen.
	// After that the unique values are tracked by shards.
	uniqValues statsCountUniqSet

	// shards are used for tracking big number of unique values.
	//
	// Every shard contains a share of unique values, which are merged in parallel at finalizeStats().
	shards []statsCountUniqSet

	// shardss is used for collecting shards from other statsCountUniqProcessor instances at mergeState().
	shardss [][]statsCountUniqSet

	columnValues [][]string
	keyBuf       []byte
	tmpNum       int
}

// the maximum number of values to track in statsCountUniqProcessor.uniqValues before switching to statsCountUniqProcessor.shards
//
// Too big value may slow down mergeState() across big number of CPU cores.
// Too small value may significantly increase RAM usage when coun_uniq() is applied individually to big number of groups.
const statsCountUniqValuesMaxLen = 4 << 10

type statsCountUniqSet struct {
	timestamps map[uint64]struct{}
	u64        map[uint64]struct{}
	negative64 map[uint64]struct{}
	strings    map[string]struct{}
}

func (sus *statsCountUniqSet) reset() {
	*sus = statsCountUniqSet{}
}

func (sus *statsCountUniqSet) entriesCount() uint64 {
	n := len(sus.timestamps) + len(sus.u64) + len(sus.negative64) + len(sus.strings)
	return uint64(n)
}

func (sus *statsCountUniqSet) exportState(dst []byte, stopCh <-chan struct{}) []byte {
	dst = marshalUint64Set(dst, sus.timestamps, stopCh)
	dst = marshalUint64Set(dst, sus.u64, stopCh)
	dst = marshalUint64Set(dst, sus.negative64, stopCh)
	dst = marshalStringSet(dst, sus.strings, stopCh)
	return dst
}

func (sus *statsCountUniqSet) importState(a *chunkedAllocator, src []byte, stopCh <-chan struct{}) ([]byte, int, error) {
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

	tail, stateSize, err = unmarshalStringSet(a, &sus.strings, src, stopCh)
	if err != nil {
		return tail, 0, fmt.Errorf("cannot unmarshal string values: %w", err)
	}
	src = tail
	stateSizeIncrease += stateSize

	return src, stateSizeIncrease, nil
}

func marshalUint64Set(dst []byte, m map[uint64]struct{}, stopCh <-chan struct{}) []byte {
	dst = encoding.MarshalVarUint64(dst, uint64(len(m)))
	for k := range m {
		dst = encoding.MarshalUint64(dst, k)
		if needStop(stopCh) {
			return dst
		}
	}
	return dst
}

func unmarshalUint64Set(dst *map[uint64]struct{}, src []byte, stopCh <-chan struct{}) ([]byte, int, error) {
	if needStop(stopCh) {
		return nil, 0, nil
	}

	entriesLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return src, 0, fmt.Errorf("cannot unmarshal the number of uint64 entries")
	}
	src = src[n:]

	if uint64(len(src)) < 8*entriesLen {
		return src, 0, fmt.Errorf("cannot unmarshal %d uint64 values from %d bytes; need %d bytes", entriesLen, len(src), 8*entriesLen)
	}
	m := make(map[uint64]struct{}, entriesLen)
	for i := uint64(0); i < entriesLen; i++ {
		u64 := encoding.UnmarshalUint64(src)
		src = src[8:]

		m[u64] = struct{}{}

		if needStop(stopCh) {
			return nil, 0, nil
		}
	}
	if entriesLen == 0 {
		m = nil
	}
	*dst = m

	stateSize := int(8 * entriesLen)
	return src, stateSize, nil
}

func marshalStringSet(dst []byte, m map[string]struct{}, stopCh <-chan struct{}) []byte {
	dst = encoding.MarshalVarUint64(dst, uint64(len(m)))
	for k := range m {
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(k))
		if needStop(stopCh) {
			return dst
		}
	}
	return dst
}

func unmarshalStringSet(a *chunkedAllocator, dst *map[string]struct{}, src []byte, stopCh <-chan struct{}) ([]byte, int, error) {
	if needStop(stopCh) {
		return nil, 0, nil
	}

	entriesLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return src, 0, fmt.Errorf("cannot unmarshal the number of string entries")
	}
	src = src[n:]

	stateSize := 0

	m := make(map[string]struct{}, entriesLen)
	for i := uint64(0); i < entriesLen; i++ {
		v, n := encoding.UnmarshalBytes(src)
		if n <= 0 {
			return src, 0, fmt.Errorf("cannot unmarshal string entry")
		}
		src = src[n:]

		vCopy := a.cloneBytesToString(v)
		m[vCopy] = struct{}{}

		stateSize += int(unsafe.Sizeof(vCopy)) + len(vCopy)

		if needStop(stopCh) {
			return nil, 0, nil
		}
	}
	if entriesLen == 0 {
		m = nil
	}
	*dst = m

	return src, stateSize, nil
}

func (sus *statsCountUniqSet) updateStateTimestamp(ts int64) int {
	return updateUint64Set(&sus.timestamps, uint64(ts))
}

func (sus *statsCountUniqSet) updateStateUint64(n uint64) int {
	return updateUint64Set(&sus.u64, n)
}

func (sus *statsCountUniqSet) updateStateNegativeInt64(n int64) int {
	return updateUint64Set(&sus.negative64, uint64(n))
}

func (sus *statsCountUniqSet) updateStateString(a *chunkedAllocator, v []byte) int {
	if _, ok := sus.strings[string(v)]; ok {
		return 0
	}
	vCopy := a.cloneBytesToString(v)
	return setStringSet(&sus.strings, vCopy) + len(vCopy)
}

func (sus *statsCountUniqSet) mergeState(src *statsCountUniqSet, stopCh <-chan struct{}) {
	mergeUint64Set(&sus.timestamps, src.timestamps, stopCh)
	mergeUint64Set(&sus.u64, src.u64, stopCh)
	mergeUint64Set(&sus.negative64, src.negative64, stopCh)

	mergeStringSet(&sus.strings, src.strings, stopCh)
}

func updateUint64Set(dstPtr *map[uint64]struct{}, n uint64) int {
	dst := *dstPtr
	if _, ok := dst[n]; ok {
		return 0
	}
	return setUint64Set(dstPtr, n)
}

func setUint64Set(dstPtr *map[uint64]struct{}, n uint64) int {
	dst := *dstPtr
	if dst == nil {
		dst = map[uint64]struct{}{
			n: {},
		}
		*dstPtr = dst
		return int(unsafe.Sizeof(dst) + unsafe.Sizeof(n))
	}
	dst[n] = struct{}{}
	return int(unsafe.Sizeof(n))
}

func setStringSet(dstPtr *map[string]struct{}, v string) int {
	dst := *dstPtr
	if dst == nil {
		dst = map[string]struct{}{
			v: {},
		}
		*dstPtr = dst
		return int(unsafe.Sizeof(dst) + unsafe.Sizeof(v))
	}
	dst[v] = struct{}{}
	return int(unsafe.Sizeof(v))
}

func mergeUint64Set(dstPtr *map[uint64]struct{}, src map[uint64]struct{}, stopCh <-chan struct{}) {
	if len(src) == 0 {
		return
	}

	dst := *dstPtr
	if dst == nil {
		dst = make(map[uint64]struct{})
		*dstPtr = dst
	}
	for n := range src {
		if needStop(stopCh) {
			return
		}
		if _, ok := dst[n]; !ok {
			dst[n] = struct{}{}
		}
	}
}

func mergeStringSet(dstPtr *map[string]struct{}, src map[string]struct{}, stopCh <-chan struct{}) {
	if len(src) == 0 {
		return
	}

	dst := *dstPtr
	if dst == nil {
		dst = make(map[string]struct{})
		*dstPtr = dst
	}
	for k := range src {
		if needStop(stopCh) {
			return
		}
		if _, ok := dst[k]; !ok {
			dst[k] = struct{}{}
		}
	}
}

func (sup *statsCountUniqProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	su := sf.(*statsCountUniq)
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

func (sup *statsCountUniqProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	su := sf.(*statsCountUniq)
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

func (sup *statsCountUniqProcessor) updateStatsForAllRowsSingleColumn(br *blockResult, columnName string) int {
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

func (sup *statsCountUniqProcessor) mergeState(a *chunkedAllocator, sf statsFunc, sfp statsProcessor) {
	su := sf.(*statsCountUniq)
	if sup.limitReached(su) {
		return
	}

	src := sfp.(*statsCountUniqProcessor)

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

func (sup *statsCountUniqProcessor) exportState(dst []byte, stopCh <-chan struct{}) []byte {
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

func (sup *statsCountUniqProcessor) importState(src []byte, stopCh <-chan struct{}) (int, error) {
	shardsLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot read the number of shards")
	}
	if shardsLen < 1 {
		return 0, fmt.Errorf("the number of shards must be at least 1")
	}
	src = src[1:]

	if shardsLen == 1 {
		tail, stateSize, err := sup.uniqValues.importState(sup.a, src, stopCh)
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

	shards := make([]statsCountUniqSet, shardsLen)
	stateSizeIncrease := int(unsafe.Sizeof(shards[0])) * len(shards)
	for i := range shards {
		tail, stateSize, err := shards[i].importState(sup.a, src, stopCh)
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

func (sup *statsCountUniqProcessor) finalizeStats(sf statsFunc, dst []byte, stopCh <-chan struct{}) []byte {
	sup.mergeShardssParallel(stopCh)

	n := sup.entriesCount()

	su := sf.(*statsCountUniq)
	if limit := su.limit; limit > 0 && n > limit {
		n = limit
	}
	return strconv.AppendUint(dst, n, 10)
}

func (sup *statsCountUniqProcessor) mergeShardssParallel(stopCh <-chan struct{}) {
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

	result := make([]statsCountUniqSet, len(shardss[0]))
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

func (sup *statsCountUniqProcessor) entriesCount() uint64 {
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

func (sup *statsCountUniqProcessor) updateStateGeneric(v string) int {
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

func (sup *statsCountUniqProcessor) updateStateInt64(n int64) int {
	if n >= 0 {
		return sup.updateStateUint64(uint64(n))
	}
	return sup.updateStateNegativeInt64(n)
}

func (sup *statsCountUniqProcessor) updateStateString(v []byte) int {
	if sup.shards == nil {
		stateSizeIncrease := sup.uniqValues.updateStateString(sup.a, v)
		if stateSizeIncrease > 0 {
			stateSizeIncrease += sup.probablyMoveUniqValuesToShards(sup.a)
		}
		return stateSizeIncrease
	}
	sus := sup.getShardByString(v)
	return sus.updateStateString(sup.a, v)
}

func (sup *statsCountUniqProcessor) updateStateTimestamp(ts int64) int {
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

func (sup *statsCountUniqProcessor) updateStateUint64(n uint64) int {
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

func (sup *statsCountUniqProcessor) updateStateNegativeInt64(n int64) int {
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

func (sup *statsCountUniqProcessor) probablyMoveUniqValuesToShards(a *chunkedAllocator) int {
	if sup.uniqValues.entriesCount() < statsCountUniqValuesMaxLen {
		return 0
	}
	return sup.moveUniqValuesToShards(a)
}

func (sup *statsCountUniqProcessor) moveUniqValuesToShards(a *chunkedAllocator) int {
	cpusCount := sup.concurrency
	bytesAllocatedPrev := a.bytesAllocated
	sup.shards = a.newStatsCountUniqSets(cpusCount)
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
	for s := range sup.uniqValues.strings {
		sus := sup.getShardByString(bytesutil.ToUnsafeBytes(s))
		setStringSet(&sus.strings, s)
	}

	sup.uniqValues.reset()

	return stateSizeIncrease
}

func (sup *statsCountUniqProcessor) getShardByString(v []byte) *statsCountUniqSet {
	h := xxhash.Sum64(v)
	cpuIdx := h % uint64(len(sup.shards))
	return &sup.shards[cpuIdx]
}

func (sup *statsCountUniqProcessor) getShardByUint64(n uint64) *statsCountUniqSet {
	h := fastHashUint64(n)
	cpuIdx := h % uint64(len(sup.shards))
	return &sup.shards[cpuIdx]
}

func (sup *statsCountUniqProcessor) limitReached(su *statsCountUniq) bool {
	limit := su.limit
	if limit <= 0 {
		return false
	}
	return sup.entriesCount() > limit
}

func parseStatsCountUniq(lex *lexer) (*statsCountUniq, error) {
	fields, err := parseStatsFuncFields(lex, "count_uniq")
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("expecting at least a single field")
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
