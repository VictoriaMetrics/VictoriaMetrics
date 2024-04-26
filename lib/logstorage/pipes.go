package logstorage

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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
	// The returned pipeProcessor may call cancel() at any time in order to stop ppBase.
	newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor
}

// pipeProcessor must process a single pipe.
type pipeProcessor interface {
	// writeBlock must write the given block of data to the given pipeProcessor.
	//
	// The workerID is the id of the worker goroutine, which called the writeBlock.
	// It is in the range 0 ... workersCount-1 .
	//
	// It is forbidden to hold references to columns after returning from writeBlock, since the caller re-uses columns.
	writeBlock(workerID uint, timestamps []int64, columns []BlockColumn)

	// flush must flush all the data accumulated in the pipeProcessor to the base pipeProcessor.
	//
	// The pipeProcessor must call ppBase.flush() and cancel(), which has been passed to newPipeProcessor, before returning from the flush.
	flush()
}

type defaultPipeProcessor func(workerID uint, timestamps []int64, columns []BlockColumn)

func newDefaultPipeProcessor(writeBlock func(workerID uint, timestamps []int64, columns []BlockColumn)) pipeProcessor {
	return defaultPipeProcessor(writeBlock)
}

func (dpp defaultPipeProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	dpp(workerID, timestamps, columns)
}

func (dpp defaultPipeProcessor) flush() {
	// Nothing to do
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
		default:
			return nil, fmt.Errorf("unexpected pipe %q", lex.token)
		}
	}
	return pipes, nil
}

type fieldsPipe struct {
	// fields contains list of fields to fetch
	fields []string
}

func (fp *fieldsPipe) String() string {
	if len(fp.fields) == 0 {
		logger.Panicf("BUG: fieldsPipe must contain at least a single field")
	}
	return "fields " + fieldNamesString(fp.fields)
}

func (fp *fieldsPipe) newPipeProcessor(_ int, _ <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor {
	return &fieldsPipeProcessor{
		fp:     fp,
		cancel: cancel,
		ppBase: ppBase,
	}
}

type fieldsPipeProcessor struct {
	fp     *fieldsPipe
	cancel func()
	ppBase pipeProcessor
}

func (fpp *fieldsPipeProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	if slices.Contains(fpp.fp.fields, "*") || areSameBlockColumns(columns, fpp.fp.fields) {
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

func (fpp *fieldsPipeProcessor) flush() {
	fpp.ppBase.flush()
	fpp.cancel()
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
		field := parseFieldName(lex)
		fields = append(fields, field)
		switch {
		case lex.isKeyword("|", ")", ""):
			fp := &fieldsPipe{
				fields: fields,
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
	newStatsFuncProcessor() statsFuncProcessor
}

// statsFuncProcessor must process stats for some statsFunc.
//
// All the statsFuncProcessor methods are called from a single goroutine at a time,
// so there is no need in the internal synchronization.
type statsFuncProcessor interface {
	// updateStatsForAllRows must update statsFuncProcessor stats from all the rows.
	updateStatsForAllRows(timestamps []int64, columns []BlockColumn)

	// updateStatsForRow must update statsFuncProcessor stats from the row at rowIndex.
	updateStatsForRow(timestamps []int64, columns []BlockColumn, rowIndex int)

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

func (sp *statsPipe) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor {
	shards := make([]statsPipeProcessorShard, workersCount)
	for i := range shards {
		shard := &shards[i]
		shard.m = make(map[string]*statsPipeGroup)
		shard.funcs = sp.funcs
	}

	return &statsPipeProcessor{
		sp:     sp,
		stopCh: stopCh,
		cancel: cancel,
		ppBase: ppBase,

		shards: shards,
	}
}

type statsPipeProcessor struct {
	sp     *statsPipe
	stopCh <-chan struct{}
	cancel func()
	ppBase pipeProcessor

	shards []statsPipeProcessorShard
}

type statsPipeProcessorShard struct {
	statsPipeProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(statsPipeProcessorShardNopad{})%128]byte
}

type statsPipeProcessorShardNopad struct {
	m     map[string]*statsPipeGroup
	funcs []statsFunc

	columnIdxs []int
	keyBuf     []byte
}

func (shard *statsPipeProcessorShard) getStatsPipeGroup(key []byte) *statsPipeGroup {
	spg := shard.m[string(key)]
	if spg != nil {
		return spg
	}
	sfps := make([]statsFuncProcessor, len(shard.funcs))
	for i, f := range shard.funcs {
		sfps[i] = f.newStatsFuncProcessor()
	}
	spg = &statsPipeGroup{
		sfps: sfps,
	}
	shard.m[string(key)] = spg
	return spg
}

type statsPipeGroup struct {
	sfps []statsFuncProcessor
}

func (spp *statsPipeProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	shard := &spp.shards[workerID]

	if len(spp.sp.byFields) == 0 {
		// Fast path - pass all the rows to a single group
		spg := shard.getStatsPipeGroup(nil)
		for _, sfp := range spg.sfps {
			sfp.updateStatsForAllRows(timestamps, columns)
		}
		return
	}

	// Slow path - update per-row stats

	// Pre-calculate column indexes for byFields in order to speed up building group key in the loop below.
	columnIdxs := shard.columnIdxs[:0]
	for _, f := range spp.sp.byFields {
		idx := getBlockColumnIndex(columns, f)
		columnIdxs = append(columnIdxs, idx)
	}
	shard.columnIdxs = columnIdxs

	keyBuf := shard.keyBuf
	for i := range timestamps {
		// Construct key for the by (...) fields
		keyBuf = keyBuf[:0]
		for _, idx := range columnIdxs {
			v := ""
			if idx >= 0 {
				v = columns[idx].Values[i]
			}
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
		}

		spg := shard.getStatsPipeGroup(keyBuf)
		for _, sfp := range spg.sfps {
			sfp.updateStatsForRow(timestamps, columns, i)
		}
	}
	shard.keyBuf = keyBuf
}

func (spp *statsPipeProcessor) flush() {
	defer func() {
		spp.ppBase.flush()
		spp.cancel()
	}()

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
				return
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
	var values []string
	var columns []BlockColumn
	for key, spg := range m {
		// m may be quite big, so this loop can take a lot of time and CPU.
		// Stop processing data as soon as stopCh is closed without wasting CPU time.
		select {
		case <-spp.stopCh:
			return
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
	default:
		return nil, fmt.Errorf("unknown stats func %q", lex.token)
	}
}

type statsFuncCount struct {
	fields     []string
	resultName string
}

func (sfc *statsFuncCount) String() string {
	return "count(" + fieldNamesString(sfc.fields) + ") as " + quoteTokenIfNeeded(sfc.resultName)
}

func (sfc *statsFuncCount) newStatsFuncProcessor() statsFuncProcessor {
	return &statsFuncCountProcessor{
		sfc: sfc,
	}
}

func (sfc *statsFuncCount) neededFields() []string {
	return getFieldsIgnoreStar(sfc.fields)
}

type statsFuncCountProcessor struct {
	sfc       *statsFuncCount
	rowsCount uint64
}

func (sfcp *statsFuncCountProcessor) updateStatsForAllRows(timestamps []int64, columns []BlockColumn) {
	fields := sfcp.sfc.fields
	if len(fields) == 0 || slices.Contains(fields, "*") {
		// Fast path - count all the columns.
		sfcp.rowsCount += uint64(len(timestamps))
		return
	}

	// Slow path - count rows containing at least a single non-empty value for the fields enumerated inside count().
	bm := getFilterBitmap(len(timestamps))
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
}

func (sfcp *statsFuncCountProcessor) updateStatsForRow(_ []int64, columns []BlockColumn, rowIdx int) {
	fields := sfcp.sfc.fields
	if len(fields) == 0 || slices.Contains(fields, "*") {
		// Fast path - count the given column
		sfcp.rowsCount++
		return
	}

	// Slow path - count the row at rowIdx if at least a single field enumerated inside count() is non-empty
	for _, f := range fields {
		if idx := getBlockColumnIndex(columns, f); idx >= 0 && columns[idx].Values[rowIdx] != "" {
			sfcp.rowsCount++
			return
		}
	}
}

func (sfcp *statsFuncCountProcessor) mergeState(sfp statsFuncProcessor) {
	src := sfp.(*statsFuncCountProcessor)
	sfcp.rowsCount += src.rowsCount
}

func (sfcp *statsFuncCountProcessor) finalizeStats() (string, string) {
	value := strconv.FormatUint(sfcp.rowsCount, 10)
	return sfcp.sfc.resultName, value
}

func parseStatsFuncCount(lex *lexer) (*statsFuncCount, error) {
	lex.nextToken()
	fields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'count' args: %w", err)
	}

	if !lex.isKeyword("as") {
		return nil, fmt.Errorf("missing 'as' keyword")
	}
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing token after 'as' keyword")
	}
	resultName := parseFieldName(lex)

	sfc := &statsFuncCount{
		fields:     fields,
		resultName: resultName,
	}
	return sfc, nil
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
		field := parseFieldName(lex)
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

func parseFieldName(lex *lexer) string {
	s := lex.token
	lex.nextToken()
	for !lex.isSkippedSpace && !lex.isKeyword(",", "|", ")", "") {
		s += lex.rawToken
		lex.nextToken()
	}
	return s
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
