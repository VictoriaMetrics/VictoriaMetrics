package logstorage

import (
	"fmt"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

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
	// updateStatsForAllRows must update statsProcessor stats for all the rows in br.
	//
	// It must return the change of internal state size in bytes for the statsProcessor.
	updateStatsForAllRows(br *blockResult) int

	// updateStatsForRow must update statsProcessor stats for the row at rowIndex in br.
	//
	// It must return the change of internal state size in bytes for the statsProcessor.
	updateStatsForRow(br *blockResult, rowIndex int) int

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

func (spp *pipeStatsProcessor) writeBlock(workerID uint, br *blockResult) {
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
			shard.stateSizeBudget -= sfp.updateStatsForAllRows(br)
		}
		return
	}
	if len(byFields) == 1 {
		// Special case for grouping by a single column.
		c := br.getColumnByName(byFields[0])
		if c.isConst {
			// Fast path for column with constant value.
			shard.keyBuf = encoding.MarshalBytes(shard.keyBuf[:0], bytesutil.ToUnsafeBytes(c.encodedValues[0]))
			for _, sfp := range shard.getStatsProcessors(shard.keyBuf) {
				shard.stateSizeBudget -= sfp.updateStatsForAllRows(br)
			}
			return
		}

		// Slower path for column with different values.
		values := c.getValues(br)
		var sfps []statsProcessor
		keyBuf := shard.keyBuf[:0]
		for i := range br.timestamps {
			if i <= 0 || values[i-1] != values[i] {
				keyBuf = encoding.MarshalBytes(keyBuf[:0], bytesutil.ToUnsafeBytes(values[i]))
				sfps = shard.getStatsProcessors(keyBuf)
			}
			for _, sfp := range sfps {
				shard.stateSizeBudget -= sfp.updateStatsForRow(br, i)
			}
		}
		shard.keyBuf = keyBuf
		return
	}

	// Verify whether all the 'by (...)' columns are constant.
	areAllConstColumns := true
	keyBuf := shard.keyBuf[:0]
	for _, f := range byFields {
		c := br.getColumnByName(f)
		if !c.isConst {
			areAllConstColumns = false
			break
		}
		keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(c.encodedValues[0]))
	}
	shard.keyBuf = keyBuf

	if areAllConstColumns {
		// Fast path for constant 'by (...)' columns.
		for _, sfp := range shard.getStatsProcessors(keyBuf) {
			shard.stateSizeBudget -= sfp.updateStatsForAllRows(br)
		}
		return
	}

	// The slowest path - group by multiple columns with different values across rows.

	// Pre-calculate column values for byFields in order to speed up building group key in the loop below.
	shard.columnValues = br.appendColumnValues(shard.columnValues[:0], byFields)
	columnValues := shard.columnValues

	var sfps []statsProcessor
	for i := range br.timestamps {
		// Verify whether the key for 'by (...)' fields equals the previous key
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
			shard.stateSizeBudget -= sfp.updateStatsForRow(br, i)
		}
	}
	shard.keyBuf = keyBuf
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
			// Stop processing data as soon as stopCh is closed without wasting additional CPU time.
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
	var br blockResult
	zeroTimestamps := []int64{0}
	for key, spg := range m {
		// m may be quite big, so this loop can take a lot of time and CPU.
		// Stop processing data as soon as stopCh is closed without wasting additional CPU time.
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

		br.reset()
		br.timestamps = zeroTimestamps

		// construct columns for byFields
		for i, f := range byFields {
			br.addConstColumn(f, values[i])
		}

		// construct columns for stats functions
		for _, sfp := range spg.sfps {
			name, value := sfp.finalizeStats()
			br.addConstColumn(name, value)
		}

		spp.ppBase.writeBlock(0, &br)
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
