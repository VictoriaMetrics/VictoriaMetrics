package logstorage

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

type pipe interface {
	// String returns string representation of the pipe.
	String() string

	// newPipeProcessor must return new pipeProcessor for the given ppBase.
	//
	// workersCount is the number of goroutine workers, which will call writeBlock() method.
	//
	// If stopCh is closed, the returned pipeProcessor must stop performing CPU-intensive tasks which take more than a few milliseconds.
	// It is OK to continue processing pipeProcessor calls if they take less than a few milliseconds.
	//
	// The returned pipeProcessor may call cancel() at any time in order to notify worker goroutines to stop sending new data to pipeProcessor.
	newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor
}

// pipeProcessor must process a single pipe.
type pipeProcessor interface {
	// writeBlock must write the given block of data to the given pipeProcessor.
	//
	// writeBlock is called concurrently from worker goroutines.
	// The workerID is the id of the worker goroutine, which calls the writeBlock.
	// It is in the range 0 ... workersCount-1 .
	//
	// It is forbidden to hold references to columns after returning from writeBlock, since the caller re-uses columns.
	//
	// If any error occurs at writeBlock, then cancel() must be called by pipeProcessor in order to notify worker goroutines
	// to stop sending new data. The occurred error must be returned from flush().
	//
	// cancel() may be called also when the pipeProcessor decides to stop accepting new data, even if there is no any error.
	writeBlock(workerID uint, timestamps []int64, columns []BlockColumn)

	// flush must flush all the data accumulated in the pipeProcessor to the base pipeProcessor.
	//
	// flush is called after all the worker goroutines are stopped.
	//
	// It is guaranteed that flush() is called for every pipeProcessor returned from pipe.newPipeProcessor().
	flush() error
}

type defaultPipeProcessor func(workerID uint, timestamps []int64, columns []BlockColumn)

func newDefaultPipeProcessor(writeBlock func(workerID uint, timestamps []int64, columns []BlockColumn)) pipeProcessor {
	return defaultPipeProcessor(writeBlock)
}

func (dpp defaultPipeProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	dpp(workerID, timestamps, columns)
}

func (dpp defaultPipeProcessor) flush() error {
	return nil
}

func parsePipes(lex *lexer) ([]pipe, error) {
	var pipes []pipe
	for !lex.isKeyword(")", "") {
		if !lex.isKeyword("|") {
			return nil, fmt.Errorf("expecting '|'")
		}
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing token after '|'")
		}
		switch {
		case lex.isKeyword("fields"):
			fp, err := parseFieldsPipe(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'fields' pipe: %w", err)
			}
			pipes = append(pipes, fp)
		case lex.isKeyword("stats"):
			sp, err := parseStatsPipe(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'stats' pipe: %w", err)
			}
			pipes = append(pipes, sp)
		case lex.isKeyword("head"):
			hp, err := parseHeadPipe(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'head' pipe: %w", err)
			}
			pipes = append(pipes, hp)
		case lex.isKeyword("skip"):
			sp, err := parseSkipPipe(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'skip' pipe: %w", err)
			}
			pipes = append(pipes, sp)
		default:
			return nil, fmt.Errorf("unexpected pipe %q", lex.token)
		}
	}
	return pipes, nil
}

type fieldsPipe struct {
	// fields contains list of fields to fetch
	fields []string

	// whether fields contains star
	containsStar bool
}

func (fp *fieldsPipe) String() string {
	if len(fp.fields) == 0 {
		logger.Panicf("BUG: fieldsPipe must contain at least a single field")
	}
	return "fields " + fieldNamesString(fp.fields)
}

func (fp *fieldsPipe) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &fieldsPipeProcessor{
		fp:     fp,
		ppBase: ppBase,
	}
}

type fieldsPipeProcessor struct {
	fp     *fieldsPipe
	ppBase pipeProcessor
}

func (fpp *fieldsPipeProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	if fpp.fp.containsStar || areSameBlockColumns(columns, fpp.fp.fields) {
		// Fast path - there is no need in additional transformations before writing the block to ppBase.
		fpp.ppBase.writeBlock(workerID, timestamps, columns)
		return
	}

	// Slow path - construct columns for fpp.fp.fields before writing them to ppBase.
	brs := getBlockRows()
	cs := brs.cs
	for _, f := range fpp.fp.fields {
		values := getValuesForBlockColumn(columns, f, len(timestamps))
		cs = append(cs, BlockColumn{
			Name:   f,
			Values: values,
		})
	}
	fpp.ppBase.writeBlock(workerID, timestamps, cs)
	brs.cs = cs
	putBlockRows(brs)
}

func (fpp *fieldsPipeProcessor) flush() error {
	return nil
}

func parseFieldsPipe(lex *lexer) (*fieldsPipe, error) {
	var fields []string
	for {
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing field name")
		}
		if lex.isKeyword(",") {
			return nil, fmt.Errorf("unexpected ','; expecting field name")
		}
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse field name: %w", err)
		}
		fields = append(fields, field)
		switch {
		case lex.isKeyword("|", ")", ""):
			fp := &fieldsPipe{
				fields:       fields,
				containsStar: slices.Contains(fields, "*"),
			}
			return fp, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',', '|' or ')'", lex.token)
		}
	}
}

type statsPipe struct {
	byFields []string
	funcs    []statsFunc
}

type statsFunc interface {
	// String returns string representation of statsFunc
	String() string

	// neededFields returns the needed fields for calculating the given stats
	neededFields() []string

	// newStatsFuncProcessor must create new statsFuncProcessor for calculating stats for the given statsFunc.
	//
	// It also must return the size in bytes of the returned statsFuncProcessor.
	newStatsFuncProcessor() (statsFuncProcessor, int)
}

// statsFuncProcessor must process stats for some statsFunc.
//
// All the statsFuncProcessor methods are called from a single goroutine at a time,
// so there is no need in the internal synchronization.
type statsFuncProcessor interface {
	// updateStatsForAllRows must update statsFuncProcessor stats from all the rows.
	//
	// It must return the increase of internal state size in bytes for the statsFuncProcessor.
	updateStatsForAllRows(timestamps []int64, columns []BlockColumn) int

	// updateStatsForRow must update statsFuncProcessor stats from the row at rowIndex.
	//
	// It must return the increase of internal state size in bytes for the statsFuncProcessor.
	updateStatsForRow(timestamps []int64, columns []BlockColumn, rowIndex int) int

	// mergeState must merge sfp state into statsFuncProcessor state.
	mergeState(sfp statsFuncProcessor)

	// finalizeStats must return the collected stats from statsFuncProcessor.
	finalizeStats() (name, value string)
}

func (sp *statsPipe) String() string {
	s := "stats "
	if len(sp.byFields) > 0 {
		s += "by (" + fieldNamesString(sp.byFields) + ") "
	}

	if len(sp.funcs) == 0 {
		logger.Panicf("BUG: statsPipe must contain at least a single statsFunc")
	}
	a := make([]string, len(sp.funcs))
	for i, f := range sp.funcs {
		a[i] = f.String()
	}
	s += strings.Join(a, ", ")
	return s
}

const stateSizeBudgetChunk = 1 << 20

func (sp *statsPipe) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.3)

	shards := make([]statsPipeProcessorShard, workersCount)
	for i := range shards {
		shard := &shards[i]
		shard.sp = sp
		shard.m = make(map[string]*statsPipeGroup)
		shard.stateSizeBudget = stateSizeBudgetChunk
		maxStateSize -= stateSizeBudgetChunk
	}

	spp := &statsPipeProcessor{
		sp:     sp,
		stopCh: stopCh,
		cancel: cancel,
		ppBase: ppBase,

		shards: shards,

		maxStateSize: maxStateSize,
	}
	spp.stateSizeBudget.Store(maxStateSize)

	return spp
}

type statsPipeProcessor struct {
	sp     *statsPipe
	stopCh <-chan struct{}
	cancel func()
	ppBase pipeProcessor

	shards []statsPipeProcessorShard

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

type statsPipeProcessorShard struct {
	statsPipeProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(statsPipeProcessorShardNopad{})%128]byte
}

type statsPipeProcessorShardNopad struct {
	sp *statsPipe
	m  map[string]*statsPipeGroup

	columnValues [][]string
	keyBuf       []byte

	stateSizeBudget int
}

func (shard *statsPipeProcessorShard) getStatsFuncProcessors(key []byte) []statsFuncProcessor {
	spg := shard.m[string(key)]
	if spg == nil {
		sfps := make([]statsFuncProcessor, len(shard.sp.funcs))
		for i, f := range shard.sp.funcs {
			sfp, stateSize := f.newStatsFuncProcessor()
			sfps[i] = sfp
			shard.stateSizeBudget -= stateSize
		}
		spg = &statsPipeGroup{
			sfps: sfps,
		}
		shard.m[string(key)] = spg
		shard.stateSizeBudget -= len(key) + int(unsafe.Sizeof("")+unsafe.Sizeof(spg)+unsafe.Sizeof(sfps[0])*uintptr(len(sfps)))
	}
	return spg.sfps
}

type statsPipeGroup struct {
	sfps []statsFuncProcessor
}

func (spp *statsPipeProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	shard := &spp.shards[workerID]

	for shard.stateSizeBudget < 0 {
		// steal some budget for the state size from the global budget.
		remaining := spp.stateSizeBudget.Add(-stateSizeBudgetChunk)
		if remaining < 0 {
			// The state size is too big. Stop processing data in order to avoid OOM crash.
			if remaining+stateSizeBudgetChunk >= 0 {
				// Notify worker goroutines to stop calling writeBlock() in order to save CPU time.
				spp.cancel()
			}
			return
		}
		shard.stateSizeBudget += stateSizeBudgetChunk
	}

	byFields := spp.sp.byFields
	if len(byFields) == 0 {
		// Fast path - pass all the rows to a single group with empty key.
		for _, sfp := range shard.getStatsFuncProcessors(nil) {
			shard.stateSizeBudget -= sfp.updateStatsForAllRows(timestamps, columns)
		}
		return
	}
	if len(byFields) == 1 {
		// Special case for grouping by a single column.
		values := getValuesForBlockColumn(columns, byFields[0], len(timestamps))
		if isConstValue(values) {
			// Fast path for column with constant value.
			shard.keyBuf = encoding.MarshalBytes(shard.keyBuf[:0], bytesutil.ToUnsafeBytes(values[0]))
			for _, sfp := range shard.getStatsFuncProcessors(shard.keyBuf) {
				shard.stateSizeBudget -= sfp.updateStatsForAllRows(timestamps, columns)
			}
			return
		}

		// Slower path for column with different values.
		var sfps []statsFuncProcessor
		keyBuf := shard.keyBuf
		for i := range timestamps {
			if i <= 0 || values[i-1] != values[i] {
				keyBuf = encoding.MarshalBytes(keyBuf[:0], bytesutil.ToUnsafeBytes(values[i]))
				sfps = shard.getStatsFuncProcessors(keyBuf)
			}
			for _, sfp := range sfps {
				shard.stateSizeBudget -= sfp.updateStatsForRow(timestamps, columns, i)
			}
		}
		shard.keyBuf = keyBuf
		return
	}

	// Pre-calculate column values for byFields in order to speed up building group key in the loop below.
	shard.columnValues = appendBlockColumnValues(shard.columnValues[:0], columns, spp.sp.byFields, len(timestamps))
	columnValues := shard.columnValues

	if areConstValues(columnValues) {
		// Fast path for columns with constant values.
		keyBuf := shard.keyBuf[:0]
		for _, values := range columnValues {
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(values[0]))
		}
		for _, sfp := range shard.getStatsFuncProcessors(keyBuf) {
			shard.stateSizeBudget -= sfp.updateStatsForAllRows(timestamps, columns)
		}
		shard.keyBuf = keyBuf
		return
	}

	// The slowest path - group by multiple columns.
	var sfps []statsFuncProcessor
	keyBuf := shard.keyBuf
	for i := range timestamps {
		// verify whether the key for 'by (...)' fields equals the previous key
		sameValue := sfps != nil
		for _, values := range columnValues {
			if i <= 0 || values[i-1] != values[i] {
				sameValue = false
				break
			}
		}
		if !sameValue {
			// Construct new key for the 'by (...)' fields
			keyBuf = keyBuf[:0]
			for _, values := range columnValues {
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(values[i]))
			}
			sfps = shard.getStatsFuncProcessors(keyBuf)
		}
		for _, sfp := range sfps {
			shard.stateSizeBudget -= sfp.updateStatsForRow(timestamps, columns, i)
		}
	}
	shard.keyBuf = keyBuf
}

func areConstValues(valuess [][]string) bool {
	for _, values := range valuess {
		if !isConstValue(values) {
			return false
		}
	}
	return true
}

func isConstValue(values []string) bool {
	if len(values) == 0 {
		// Return false, since it is impossible to get values[0] value from empty values.
		return false
	}
	vFirst := values[0]
	for _, v := range values[1:] {
		if v != vFirst {
			return false
		}
	}
	return true
}

func (spp *statsPipeProcessor) flush() error {
	if n := spp.stateSizeBudget.Load(); n <= 0 {
		return fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", spp.sp.String(), spp.maxStateSize/(1<<20))
	}

	// Merge states across shards
	shards := spp.shards
	m := shards[0].m
	shards = shards[1:]
	for i := range shards {
		shard := &shards[i]
		for key, spg := range shard.m {
			// shard.m may be quite big, so this loop can take a lot of time and CPU.
			// Stop processing data as soon as stopCh is closed without wasting CPU time.
			select {
			case <-spp.stopCh:
				return nil
			default:
			}

			spgBase := m[key]
			if spgBase == nil {
				m[key] = spg
			} else {
				for i, sfp := range spgBase.sfps {
					sfp.mergeState(spg.sfps[i])
				}
			}
		}
	}

	// Write per-group states to ppBase
	byFields := spp.sp.byFields
	if len(byFields) == 0 && len(m) == 0 {
		// Special case - zero matching rows.
		_ = shards[0].getStatsFuncProcessors(nil)
		m = shards[0].m
	}

	var values []string
	var columns []BlockColumn
	for key, spg := range m {
		// m may be quite big, so this loop can take a lot of time and CPU.
		// Stop processing data as soon as stopCh is closed without wasting CPU time.
		select {
		case <-spp.stopCh:
			return nil
		default:
		}

		// Unmarshal values for byFields from key.
		values = values[:0]
		keyBuf := bytesutil.ToUnsafeBytes(key)
		for len(keyBuf) > 0 {
			tail, v, err := encoding.UnmarshalBytes(keyBuf)
			if err != nil {
				logger.Panicf("BUG: cannot unmarshal value from keyBuf=%q: %w", keyBuf, err)
			}
			values = append(values, bytesutil.ToUnsafeString(v))
			keyBuf = tail
		}
		if len(values) != len(byFields) {
			logger.Panicf("BUG: unexpected number of values decoded from keyBuf; got %d; want %d", len(values), len(byFields))
		}

		// construct columns for byFields
		columns = columns[:0]
		for i, f := range byFields {
			columns = append(columns, BlockColumn{
				Name:   f,
				Values: values[i : i+1],
			})
		}

		// construct columns for stats functions
		for _, sfp := range spg.sfps {
			name, value := sfp.finalizeStats()
			columns = append(columns, BlockColumn{
				Name:   name,
				Values: []string{value},
			})
		}
		spp.ppBase.writeBlock(0, []int64{0}, columns)
	}

	return nil
}

func (sp *statsPipe) neededFields() []string {
	var neededFields []string
	m := make(map[string]struct{})
	updateNeededFields := func(fields []string) {
		for _, field := range fields {
			if _, ok := m[field]; !ok {
				m[field] = struct{}{}
				neededFields = append(neededFields, field)
			}
		}
	}

	updateNeededFields(sp.byFields)

	for _, f := range sp.funcs {
		fields := f.neededFields()
		updateNeededFields(fields)
	}

	return neededFields
}

func parseStatsPipe(lex *lexer) (*statsPipe, error) {
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing stats config")
	}

	var sp statsPipe
	if lex.isKeyword("by") {
		lex.nextToken()
		fields, err := parseFieldNamesInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'by': %w", err)
		}
		sp.byFields = fields
	}

	var funcs []statsFunc
	for {
		sf, err := parseStatsFunc(lex)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, sf)
		if lex.isKeyword("|", ")", "") {
			sp.funcs = funcs
			return &sp, nil
		}
		if !lex.isKeyword(",") {
			return nil, fmt.Errorf("unexpected token %q; want ',', '|' or ')'", lex.token)
		}
		lex.nextToken()
	}
}

func parseStatsFunc(lex *lexer) (statsFunc, error) {
	switch {
	case lex.isKeyword("count"):
		sfc, err := parseStatsFuncCount(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'count' func: %w", err)
		}
		return sfc, nil
	case lex.isKeyword("uniq"):
		sfu, err := parseStatsFuncUniq(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'uniq' func: %w", err)
		}
		return sfu, nil
	default:
		return nil, fmt.Errorf("unknown stats func %q", lex.token)
	}
}

type statsFuncCount struct {
	fields       []string
	containsStar bool

	resultName string
}

func (sfc *statsFuncCount) String() string {
	return "count(" + fieldNamesString(sfc.fields) + ") as " + quoteTokenIfNeeded(sfc.resultName)
}

func (sfc *statsFuncCount) neededFields() []string {
	return getFieldsIgnoreStar(sfc.fields)
}

func (sfc *statsFuncCount) newStatsFuncProcessor() (statsFuncProcessor, int) {
	sfcp := &statsFuncCountProcessor{
		sfc: sfc,
	}
	return sfcp, int(unsafe.Sizeof(*sfcp))
}

type statsFuncCountProcessor struct {
	sfc *statsFuncCount

	rowsCount uint64
}

func (sfcp *statsFuncCountProcessor) updateStatsForAllRows(timestamps []int64, columns []BlockColumn) int {
	fields := sfcp.sfc.fields
	if len(fields) == 0 || sfcp.sfc.containsStar {
		// Fast path - count all the columns.
		sfcp.rowsCount += uint64(len(timestamps))
		return 0
	}

	// Slow path - count rows containing at least a single non-empty value for the fields enumerated inside count().
	bm := getFilterBitmap(len(timestamps))
	defer putFilterBitmap(bm)

	bm.setBits()
	for _, f := range fields {
		if idx := getBlockColumnIndex(columns, f); idx >= 0 {
			values := columns[idx].Values
			bm.forEachSetBit(func(i int) bool {
				return values[i] == ""
			})
		}
	}

	emptyValues := 0
	bm.forEachSetBit(func(i int) bool {
		emptyValues++
		return true
	})

	sfcp.rowsCount += uint64(len(timestamps) - emptyValues)
	return 0
}

func (sfcp *statsFuncCountProcessor) updateStatsForRow(_ []int64, columns []BlockColumn, rowIdx int) int {
	fields := sfcp.sfc.fields
	if len(fields) == 0 || sfcp.sfc.containsStar {
		// Fast path - count the given column
		sfcp.rowsCount++
		return 0
	}

	// Slow path - count the row at rowIdx if at least a single field enumerated inside count() is non-empty
	for _, f := range fields {
		if idx := getBlockColumnIndex(columns, f); idx >= 0 && columns[idx].Values[rowIdx] != "" {
			sfcp.rowsCount++
			return 0
		}
	}
	return 0
}

func (sfcp *statsFuncCountProcessor) mergeState(sfp statsFuncProcessor) {
	src := sfp.(*statsFuncCountProcessor)
	sfcp.rowsCount += src.rowsCount
}

func (sfcp *statsFuncCountProcessor) finalizeStats() (string, string) {
	value := strconv.FormatUint(sfcp.rowsCount, 10)
	return sfcp.sfc.resultName, value
}

type statsFuncUniq struct {
	fields       []string
	containsStar bool
	resultName   string
}

func (sfu *statsFuncUniq) String() string {
	return "uniq(" + fieldNamesString(sfu.fields) + ") as " + quoteTokenIfNeeded(sfu.resultName)
}

func (sfu *statsFuncUniq) neededFields() []string {
	return sfu.fields
}

func (sfu *statsFuncUniq) newStatsFuncProcessor() (statsFuncProcessor, int) {
	sfup := &statsFuncUniqProcessor{
		sfu: sfu,

		m: make(map[string]struct{}),
	}
	return sfup, int(unsafe.Sizeof(*sfup))
}

type statsFuncUniqProcessor struct {
	sfu *statsFuncUniq

	m map[string]struct{}

	columnValues [][]string
	keyBuf       []byte
}

func (sfup *statsFuncUniqProcessor) updateStatsForAllRows(timestamps []int64, columns []BlockColumn) int {
	fields := sfup.sfu.fields
	m := sfup.m

	stateSizeIncrease := 0
	if len(fields) == 0 || sfup.sfu.containsStar {
		// Count unique rows
		keyBuf := sfup.keyBuf
		for i := range timestamps {
			seenKey := true
			for _, c := range columns {
				values := c.Values
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
			for _, c := range columns {
				v := c.Values[i]
				if v != "" {
					allEmptyValues = false
				}
				// Put column name into key, since every block can contain different set of columns for '*' selector.
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(c.Name))
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
			}
			if allEmptyValues {
				// Do not count empty values
				continue
			}
			if _, ok := m[string(keyBuf)]; !ok {
				m[string(keyBuf)] = struct{}{}
				stateSizeIncrease += len(keyBuf) + int(unsafe.Sizeof(""))
			}
		}
		sfup.keyBuf = keyBuf
		return stateSizeIncrease
	}
	if len(fields) == 1 {
		// Fast path for a single column
		if idx := getBlockColumnIndex(columns, fields[0]); idx >= 0 {
			values := columns[idx].Values
			for i, v := range values {
				if v == "" {
					// Do not count empty values
					continue
				}
				if i > 0 && values[i-1] == v {
					continue
				}
				if _, ok := m[v]; !ok {
					vCopy := strings.Clone(v)
					m[vCopy] = struct{}{}
					stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
				}
			}
		}
		return stateSizeIncrease
	}

	// Slow path for multiple columns.

	// Pre-calculate column values for byFields in order to speed up building group key in the loop below.
	sfup.columnValues = appendBlockColumnValues(sfup.columnValues[:0], columns, fields, len(timestamps))
	columnValues := sfup.columnValues

	keyBuf := sfup.keyBuf
	for i := range timestamps {
		seenKey := true
		for _, values := range columnValues {
			if i == 0 || values[i-1] != values[i] {
				seenKey = false
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
		if _, ok := m[string(keyBuf)]; !ok {
			m[string(keyBuf)] = struct{}{}
			stateSizeIncrease += len(keyBuf) + int(unsafe.Sizeof(""))
		}
	}
	sfup.keyBuf = keyBuf
	return stateSizeIncrease
}

func (sfup *statsFuncUniqProcessor) updateStatsForRow(timestamps []int64, columns []BlockColumn, rowIdx int) int {
	fields := sfup.sfu.fields
	m := sfup.m

	stateSizeIncrease := 0
	if len(fields) == 0 || sfup.sfu.containsStar {
		// Count unique rows
		allEmptyValues := true
		keyBuf := sfup.keyBuf[:0]
		for _, c := range columns {
			v := c.Values[rowIdx]
			if v != "" {
				allEmptyValues = false
			}
			// Put column name into key, since every block can contain different set of columns for '*' selector.
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(c.Name))
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
		}
		sfup.keyBuf = keyBuf

		if allEmptyValues {
			// Do not count empty values
			return stateSizeIncrease
		}
		if _, ok := m[string(keyBuf)]; !ok {
			m[string(keyBuf)] = struct{}{}
			stateSizeIncrease += len(keyBuf) + int(unsafe.Sizeof(""))
		}
		return stateSizeIncrease
	}
	if len(fields) == 1 {
		// Fast path for a single column
		if idx := getBlockColumnIndex(columns, fields[0]); idx >= 0 {
			v := columns[idx].Values[rowIdx]
			if v == "" {
				// Do not count empty values
				return stateSizeIncrease
			}
			if _, ok := m[v]; !ok {
				vCopy := strings.Clone(v)
				m[vCopy] = struct{}{}
				stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
			}
		}
		return stateSizeIncrease
	}

	// Slow path for multiple columns.
	allEmptyValues := true
	keyBuf := sfup.keyBuf[:0]
	for _, f := range fields {
		v := ""
		if idx := getBlockColumnIndex(columns, f); idx >= 0 {
			v = columns[idx].Values[rowIdx]
		}
		if v != "" {
			allEmptyValues = false
		}
		keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
	}
	sfup.keyBuf = keyBuf

	if allEmptyValues {
		// Do not count empty values
		return stateSizeIncrease
	}
	if _, ok := m[string(keyBuf)]; !ok {
		m[string(keyBuf)] = struct{}{}
		stateSizeIncrease += len(keyBuf) + int(unsafe.Sizeof(""))
	}
	return stateSizeIncrease
}

func (sfup *statsFuncUniqProcessor) mergeState(sfp statsFuncProcessor) {
	src := sfp.(*statsFuncUniqProcessor)
	m := sfup.m
	for k := range src.m {
		m[k] = struct{}{}
	}
}

func (sfup *statsFuncUniqProcessor) finalizeStats() (string, string) {
	n := uint64(len(sfup.m))
	value := strconv.FormatUint(n, 10)
	return sfup.sfu.resultName, value
}

func parseStatsFuncUniq(lex *lexer) (*statsFuncUniq, error) {
	lex.nextToken()
	fields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'uniq' args: %w", err)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("'uniq' must contain at least a single arg")
	}
	resultName, err := parseResultName(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse result name: %w", err)
	}
	sfu := &statsFuncUniq{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
		resultName:   resultName,
	}
	return sfu, nil
}

func parseStatsFuncCount(lex *lexer) (*statsFuncCount, error) {
	lex.nextToken()
	fields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'count' args: %w", err)
	}
	resultName, err := parseResultName(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse result name: %w", err)
	}
	sfc := &statsFuncCount{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
		resultName:   resultName,
	}
	return sfc, nil
}

func parseResultName(lex *lexer) (string, error) {
	if lex.isKeyword("as") {
		if !lex.mustNextToken() {
			return "", fmt.Errorf("missing token after 'as' keyword")
		}
	}
	resultName, err := parseFieldName(lex)
	if err != nil {
		return "", fmt.Errorf("cannot parse 'as' field name: %w", err)
	}
	return resultName, nil
}

type headPipe struct {
	n uint64
}

func (hp *headPipe) String() string {
	return fmt.Sprintf("head %d", hp.n)
}

func (hp *headPipe) newPipeProcessor(_ int, _ <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor {
	if hp.n == 0 {
		// Special case - notify the caller to stop writing data to the returned headPipeProcessor
		cancel()
	}
	return &headPipeProcessor{
		hp:     hp,
		cancel: cancel,
		ppBase: ppBase,
	}
}

type headPipeProcessor struct {
	hp     *headPipe
	cancel func()
	ppBase pipeProcessor

	rowsProcessed atomic.Uint64
}

func (hpp *headPipeProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	rowsProcessed := hpp.rowsProcessed.Add(uint64(len(timestamps)))
	if rowsProcessed <= hpp.hp.n {
		// Fast path - write all the rows to ppBase.
		hpp.ppBase.writeBlock(workerID, timestamps, columns)
		return
	}

	// Slow path - overflow. Write the remaining rows if needed.
	rowsProcessed -= uint64(len(timestamps))
	if rowsProcessed >= hpp.hp.n {
		// Nothing to write. There is no need in cancel() call, since it has been called by another goroutine.
		return
	}

	// Write remaining rows.
	rowsRemaining := hpp.hp.n - rowsProcessed
	cs := make([]BlockColumn, len(columns))
	for i, c := range columns {
		cDst := &cs[i]
		cDst.Name = c.Name
		cDst.Values = c.Values[:rowsRemaining]
	}
	timestamps = timestamps[:rowsRemaining]
	hpp.ppBase.writeBlock(workerID, timestamps, cs)

	// Notify the caller that it should stop passing more data to writeBlock().
	hpp.cancel()
}

func (hpp *headPipeProcessor) flush() error {
	return nil
}

func parseHeadPipe(lex *lexer) (*headPipe, error) {
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing the number of head rows to return")
	}
	n, err := parseUint(lex.token)
	if err != nil {
		return nil, fmt.Errorf("cannot parse the number of head rows to return %q: %w", lex.token, err)
	}
	lex.nextToken()
	hp := &headPipe{
		n: n,
	}
	return hp, nil
}

type skipPipe struct {
	n uint64
}

func (sp *skipPipe) String() string {
	return fmt.Sprintf("skip %d", sp.n)
}

func (sp *skipPipe) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &skipPipeProcessor{
		sp:     sp,
		ppBase: ppBase,
	}
}

type skipPipeProcessor struct {
	sp     *skipPipe
	ppBase pipeProcessor

	rowsProcessed atomic.Uint64
}

func (spp *skipPipeProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	rowsProcessed := spp.rowsProcessed.Add(uint64(len(timestamps)))
	if rowsProcessed <= spp.sp.n {
		return
	}

	rowsProcessed -= uint64(len(timestamps))
	if rowsProcessed >= spp.sp.n {
		spp.ppBase.writeBlock(workerID, timestamps, columns)
		return
	}

	rowsRemaining := spp.sp.n - rowsProcessed
	cs := make([]BlockColumn, len(columns))
	for i, c := range columns {
		cDst := &cs[i]
		cDst.Name = c.Name
		cDst.Values = c.Values[rowsRemaining:]
	}
	timestamps = timestamps[rowsRemaining:]
	spp.ppBase.writeBlock(workerID, timestamps, cs)
}

func (spp *skipPipeProcessor) flush() error {
	return nil
}

func parseSkipPipe(lex *lexer) (*skipPipe, error) {
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing the number of rows to skip")
	}
	n, err := parseUint(lex.token)
	if err != nil {
		return nil, fmt.Errorf("cannot parse the number of rows to skip %q: %w", lex.token, err)
	}
	lex.nextToken()
	sp := &skipPipe{
		n: n,
	}
	return sp, nil
}

func parseFieldNamesInParens(lex *lexer) ([]string, error) {
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing `(`")
	}
	var fields []string
	for {
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing field name or ')'")
		}
		if lex.isKeyword(")") {
			lex.nextToken()
			return fields, nil
		}
		if lex.isKeyword(",") {
			return nil, fmt.Errorf("unexpected `,`")
		}
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse field name: %w", err)
		}
		fields = append(fields, field)
		switch {
		case lex.isKeyword(")"):
			lex.nextToken()
			return fields, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',' or ')'", lex.token)
		}
	}
}

func parseFieldName(lex *lexer) (string, error) {
	if lex.isKeyword(",", "(", ")", "[", "]", "|", "") {
		return "", fmt.Errorf("unexpected token: %q", lex.token)
	}
	token := getCompoundToken(lex)
	return token, nil
}

func fieldNamesString(fields []string) string {
	a := make([]string, len(fields))
	for i, f := range fields {
		if f != "*" {
			f = quoteTokenIfNeeded(f)
		}
		a[i] = f
	}
	return strings.Join(a, ", ")
}

func getFieldsIgnoreStar(fields []string) []string {
	var result []string
	for _, f := range fields {
		if f != "*" {
			result = append(result, f)
		}
	}
	return result
}

func appendBlockColumnValues(dst [][]string, columns []BlockColumn, fields []string, rowsCount int) [][]string {
	for _, f := range fields {
		values := getValuesForBlockColumn(columns, f, rowsCount)
		dst = append(dst, values)
	}
	return dst
}
