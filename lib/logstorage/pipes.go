package logstorage

import (
	"fmt"
	"slices"
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
			pf, err := parsePipeFields(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'fields' pipe: %w", err)
			}
			pipes = append(pipes, pf)
		case lex.isKeyword("stats"):
			ps, err := parsePipeStats(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'stats' pipe: %w", err)
			}
			pipes = append(pipes, ps)
		case lex.isKeyword("head"):
			ph, err := parsePipeHead(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'head' pipe: %w", err)
			}
			pipes = append(pipes, ph)
		case lex.isKeyword("skip"):
			ps, err := parseSkipPipe(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'skip' pipe: %w", err)
			}
			pipes = append(pipes, ps)
		default:
			return nil, fmt.Errorf("unexpected pipe %q", lex.token)
		}
	}
	return pipes, nil
}

type pipeFields struct {
	// fields contains list of fields to fetch
	fields []string

	// whether fields contains star
	containsStar bool
}

func (pf *pipeFields) String() string {
	if len(pf.fields) == 0 {
		logger.Panicf("BUG: pipeFields must contain at least a single field")
	}
	return "fields " + fieldNamesString(pf.fields)
}

func (pf *pipeFields) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &pipeFieldsProcessor{
		pf:     pf,
		ppBase: ppBase,
	}
}

type pipeFieldsProcessor struct {
	pf     *pipeFields
	ppBase pipeProcessor
}

func (fpp *pipeFieldsProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	if fpp.pf.containsStar || areSameBlockColumns(columns, fpp.pf.fields) {
		// Fast path - there is no need in additional transformations before writing the block to ppBase.
		fpp.ppBase.writeBlock(workerID, timestamps, columns)
		return
	}

	// Slow path - construct columns for fpp.pf.fields before writing them to ppBase.
	brs := getBlockRows()
	cs := brs.cs
	for _, f := range fpp.pf.fields {
		values := getBlockColumnValues(columns, f, len(timestamps))
		cs = append(cs, BlockColumn{
			Name:   f,
			Values: values,
		})
	}
	fpp.ppBase.writeBlock(workerID, timestamps, cs)
	brs.cs = cs
	putBlockRows(brs)
}

func (fpp *pipeFieldsProcessor) flush() error {
	return nil
}

func parsePipeFields(lex *lexer) (*pipeFields, error) {
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
			pf := &pipeFields{
				fields:       fields,
				containsStar: slices.Contains(fields, "*"),
			}
			return pf, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',', '|' or ')'", lex.token)
		}
	}
}

type pipeStats struct {
	byFields []string
	funcs    []statsFunc
}

type statsFunc interface {
	// String returns string representation of statsFunc
	String() string

	// neededFields returns the needed fields for calculating the given stats
	neededFields() []string

	// newStatsProcessor must create new statsProcessor for calculating stats for the given statsFunc.
	//
	// It also must return the size in bytes of the returned statsProcessor.
	newStatsProcessor() (statsProcessor, int)
}

// statsProcessor must process stats for some statsFunc.
//
// All the statsProcessor methods are called from a single goroutine at a time,
// so there is no need in the internal synchronization.
type statsProcessor interface {
	// updateStatsForAllRows must update statsProcessor stats from all the rows.
	//
	// It must return the increase of internal state size in bytes for the statsProcessor.
	updateStatsForAllRows(timestamps []int64, columns []BlockColumn) int

	// updateStatsForRow must update statsProcessor stats from the row at rowIndex.
	//
	// It must return the increase of internal state size in bytes for the statsProcessor.
	updateStatsForRow(timestamps []int64, columns []BlockColumn, rowIndex int) int

	// mergeState must merge sfp state into statsProcessor state.
	mergeState(sfp statsProcessor)

	// finalizeStats must return the collected stats from statsProcessor.
	finalizeStats() (name, value string)
}

func (ps *pipeStats) String() string {
	s := "stats "
	if len(ps.byFields) > 0 {
		s += "by (" + fieldNamesString(ps.byFields) + ") "
	}

	if len(ps.funcs) == 0 {
		logger.Panicf("BUG: pipeStats must contain at least a single statsFunc")
	}
	a := make([]string, len(ps.funcs))
	for i, f := range ps.funcs {
		a[i] = f.String()
	}
	s += strings.Join(a, ", ")
	return s
}

const stateSizeBudgetChunk = 1 << 20

func (ps *pipeStats) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.3)

	shards := make([]pipeStatsProcessorShard, workersCount)
	for i := range shards {
		shard := &shards[i]
		shard.ps = ps
		shard.m = make(map[string]*pipeStatsGroup)
		shard.stateSizeBudget = stateSizeBudgetChunk
		maxStateSize -= stateSizeBudgetChunk
	}

	spp := &pipeStatsProcessor{
		ps:     ps,
		stopCh: stopCh,
		cancel: cancel,
		ppBase: ppBase,

		shards: shards,

		maxStateSize: maxStateSize,
	}
	spp.stateSizeBudget.Store(maxStateSize)

	return spp
}

type pipeStatsProcessor struct {
	ps     *pipeStats
	stopCh <-chan struct{}
	cancel func()
	ppBase pipeProcessor

	shards []pipeStatsProcessorShard

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

type pipeStatsProcessorShard struct {
	pipeStatsProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeStatsProcessorShardNopad{})%128]byte
}

type pipeStatsProcessorShardNopad struct {
	ps *pipeStats
	m  map[string]*pipeStatsGroup

	columnValues [][]string
	keyBuf       []byte

	stateSizeBudget int
}

func (shard *pipeStatsProcessorShard) getStatsProcessors(key []byte) []statsProcessor {
	spg := shard.m[string(key)]
	if spg == nil {
		sfps := make([]statsProcessor, len(shard.ps.funcs))
		for i, f := range shard.ps.funcs {
			sfp, stateSize := f.newStatsProcessor()
			sfps[i] = sfp
			shard.stateSizeBudget -= stateSize
		}
		spg = &pipeStatsGroup{
			sfps: sfps,
		}
		shard.m[string(key)] = spg
		shard.stateSizeBudget -= len(key) + int(unsafe.Sizeof("")+unsafe.Sizeof(spg)+unsafe.Sizeof(sfps[0])*uintptr(len(sfps)))
	}
	return spg.sfps
}

type pipeStatsGroup struct {
	sfps []statsProcessor
}

func (spp *pipeStatsProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
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

	byFields := spp.ps.byFields
	if len(byFields) == 0 {
		// Fast path - pass all the rows to a single group with empty key.
		for _, sfp := range shard.getStatsProcessors(nil) {
			shard.stateSizeBudget -= sfp.updateStatsForAllRows(timestamps, columns)
		}
		return
	}
	if len(byFields) == 1 {
		// Special case for grouping by a single column.
		values := getBlockColumnValues(columns, byFields[0], len(timestamps))
		if isConstValue(values) {
			// Fast path for column with constant value.
			shard.keyBuf = encoding.MarshalBytes(shard.keyBuf[:0], bytesutil.ToUnsafeBytes(values[0]))
			for _, sfp := range shard.getStatsProcessors(shard.keyBuf) {
				shard.stateSizeBudget -= sfp.updateStatsForAllRows(timestamps, columns)
			}
			return
		}

		// Slower path for column with different values.
		var sfps []statsProcessor
		keyBuf := shard.keyBuf
		for i := range timestamps {
			if i <= 0 || values[i-1] != values[i] {
				keyBuf = encoding.MarshalBytes(keyBuf[:0], bytesutil.ToUnsafeBytes(values[i]))
				sfps = shard.getStatsProcessors(keyBuf)
			}
			for _, sfp := range sfps {
				shard.stateSizeBudget -= sfp.updateStatsForRow(timestamps, columns, i)
			}
		}
		shard.keyBuf = keyBuf
		return
	}

	// Pre-calculate column values for byFields in order to speed up building group key in the loop below.
	shard.columnValues = appendBlockColumnValues(shard.columnValues[:0], columns, byFields, len(timestamps))
	columnValues := shard.columnValues

	if areConstValues(columnValues) {
		// Fast path for columns with constant values.
		keyBuf := shard.keyBuf[:0]
		for _, values := range columnValues {
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(values[0]))
		}
		for _, sfp := range shard.getStatsProcessors(keyBuf) {
			shard.stateSizeBudget -= sfp.updateStatsForAllRows(timestamps, columns)
		}
		shard.keyBuf = keyBuf
		return
	}

	// The slowest path - group by multiple columns.
	var sfps []statsProcessor
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
			sfps = shard.getStatsProcessors(keyBuf)
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

func (spp *pipeStatsProcessor) flush() error {
	if n := spp.stateSizeBudget.Load(); n <= 0 {
		return fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", spp.ps.String(), spp.maxStateSize/(1<<20))
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
	byFields := spp.ps.byFields
	if len(byFields) == 0 && len(m) == 0 {
		// Special case - zero matching rows.
		_ = shards[0].getStatsProcessors(nil)
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

func (ps *pipeStats) neededFields() []string {
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

	updateNeededFields(ps.byFields)

	for _, f := range ps.funcs {
		fields := f.neededFields()
		updateNeededFields(fields)
	}

	return neededFields
}

func parsePipeStats(lex *lexer) (*pipeStats, error) {
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing stats config")
	}

	var ps pipeStats
	if lex.isKeyword("by") {
		lex.nextToken()
		fields, err := parseFieldNamesInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'by': %w", err)
		}
		ps.byFields = fields
	}

	var funcs []statsFunc
	for {
		sf, err := parseStatsFunc(lex)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, sf)
		if lex.isKeyword("|", ")", "") {
			ps.funcs = funcs
			return &ps, nil
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
		sfc, err := parseStatsCount(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'count' func: %w", err)
		}
		return sfc, nil
	case lex.isKeyword("uniq"):
		sfu, err := parseStatsUniq(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'uniq' func: %w", err)
		}
		return sfu, nil
	case lex.isKeyword("sum"):
		sfs, err := parseStatsSum(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'sum' func: %w", err)
		}
		return sfs, nil
	default:
		return nil, fmt.Errorf("unknown stats func %q", lex.token)
	}
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

type pipeHead struct {
	n uint64
}

func (ph *pipeHead) String() string {
	return fmt.Sprintf("head %d", ph.n)
}

func (ph *pipeHead) newPipeProcessor(_ int, _ <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor {
	if ph.n == 0 {
		// Special case - notify the caller to stop writing data to the returned pipeHeadProcessor
		cancel()
	}
	return &pipeHeadProcessor{
		ph:     ph,
		cancel: cancel,
		ppBase: ppBase,
	}
}

type pipeHeadProcessor struct {
	ph     *pipeHead
	cancel func()
	ppBase pipeProcessor

	rowsProcessed atomic.Uint64
}

func (hpp *pipeHeadProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	rowsProcessed := hpp.rowsProcessed.Add(uint64(len(timestamps)))
	if rowsProcessed <= hpp.ph.n {
		// Fast path - write all the rows to ppBase.
		hpp.ppBase.writeBlock(workerID, timestamps, columns)
		return
	}

	// Slow path - overflow. Write the remaining rows if needed.
	rowsProcessed -= uint64(len(timestamps))
	if rowsProcessed >= hpp.ph.n {
		// Nothing to write. There is no need in cancel() call, since it has been called by another goroutine.
		return
	}

	// Write remaining rows.
	rowsRemaining := hpp.ph.n - rowsProcessed
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

func (hpp *pipeHeadProcessor) flush() error {
	return nil
}

func parsePipeHead(lex *lexer) (*pipeHead, error) {
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing the number of head rows to return")
	}
	n, err := parseUint(lex.token)
	if err != nil {
		return nil, fmt.Errorf("cannot parse the number of head rows to return %q: %w", lex.token, err)
	}
	lex.nextToken()
	ph := &pipeHead{
		n: n,
	}
	return ph, nil
}

type skipPipe struct {
	n uint64
}

func (ps *skipPipe) String() string {
	return fmt.Sprintf("skip %d", ps.n)
}

func (ps *skipPipe) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &skipPipeProcessor{
		ps:     ps,
		ppBase: ppBase,
	}
}

type skipPipeProcessor struct {
	ps     *skipPipe
	ppBase pipeProcessor

	rowsProcessed atomic.Uint64
}

func (spp *skipPipeProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	rowsProcessed := spp.rowsProcessed.Add(uint64(len(timestamps)))
	if rowsProcessed <= spp.ps.n {
		return
	}

	rowsProcessed -= uint64(len(timestamps))
	if rowsProcessed >= spp.ps.n {
		spp.ppBase.writeBlock(workerID, timestamps, columns)
		return
	}

	rowsRemaining := spp.ps.n - rowsProcessed
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
	ps := &skipPipe{
		n: n,
	}
	return ps, nil
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
		values := getBlockColumnValues(columns, f, rowsCount)
		dst = append(dst, values)
	}
	return dst
}
