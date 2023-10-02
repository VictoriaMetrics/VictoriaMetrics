package logstorage

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func getFilterBitmap(bitsLen int) *filterBitmap {
	v := filterBitmapPool.Get()
	if v == nil {
		v = &filterBitmap{}
	}
	bm := v.(*filterBitmap)
	bm.init(bitsLen)
	return bm
}

func putFilterBitmap(bm *filterBitmap) {
	bm.reset()
	filterBitmapPool.Put(bm)
}

var filterBitmapPool sync.Pool

type filterBitmap struct {
	a       []uint64
	bitsLen int
}

func (bm *filterBitmap) reset() {
	bm.resetBits()
	bm.a = bm.a[:0]

	bm.bitsLen = 0
}

func (bm *filterBitmap) copyFrom(src *filterBitmap) {
	bm.reset()

	bm.a = append(bm.a[:0], src.a...)
	bm.bitsLen = src.bitsLen
}

func (bm *filterBitmap) init(bitsLen int) {
	a := bm.a
	wordsLen := (bitsLen + 63) / 64
	if n := wordsLen - cap(a); n > 0 {
		a = append(a[:cap(a)], make([]uint64, n)...)
	}
	a = a[:wordsLen]
	bm.a = a
	bm.bitsLen = bitsLen
}

func (bm *filterBitmap) resetBits() {
	a := bm.a
	for i := range a {
		a[i] = 0
	}
}

func (bm *filterBitmap) setBits() {
	a := bm.a
	for i := range a {
		a[i] = ^uint64(0)
	}
	tailBits := bm.bitsLen % 64
	if tailBits > 0 && len(a) > 0 {
		// Zero bits outside bitsLen at the last word
		a[len(a)-1] &= (uint64(1) << tailBits) - 1
	}
}

func (bm *filterBitmap) isZero() bool {
	for _, word := range bm.a {
		if word != 0 {
			return false
		}
	}
	return true
}

func (bm *filterBitmap) andNot(x *filterBitmap) {
	if bm.bitsLen != x.bitsLen {
		logger.Panicf("BUG: cannot merge bitmaps with distinct lengths; %d vs %d", bm.bitsLen, x.bitsLen)
	}
	a := bm.a
	b := x.a
	for i := range a {
		a[i] &= ^b[i]
	}
}

func (bm *filterBitmap) or(x *filterBitmap) {
	if bm.bitsLen != x.bitsLen {
		logger.Panicf("BUG: cannot merge bitmaps with distinct lengths; %d vs %d", bm.bitsLen, x.bitsLen)
	}
	a := bm.a
	b := x.a
	for i := range a {
		a[i] |= b[i]
	}
}

// forEachSetBit calls f for each set bit and clears that bit if f returns false
func (bm *filterBitmap) forEachSetBit(f func(idx int) bool) {
	a := bm.a
	bitsLen := bm.bitsLen
	for i, word := range a {
		if word == 0 {
			continue
		}
		for j := 0; j < 64; j++ {
			mask := uint64(1) << j
			if (word & mask) == 0 {
				continue
			}
			idx := i*64 + j
			if idx >= bitsLen {
				break
			}
			if !f(idx) {
				a[i] &= ^mask
			}
		}
	}
}

type filter interface {
	// String returns string representation of the filter
	String() string

	// updateReferencedColumnNames updates m with the column names referenced by the filter
	updateReferencedColumnNames(m map[string]struct{})

	// apply must update bm according to the filter applied to the given bs block
	apply(bs *blockSearch, bm *filterBitmap)
}

// noopFilter does nothing
type noopFilter struct {
}

func (nf *noopFilter) String() string {
	return ""
}

func (nf *noopFilter) updateReferencedColumnNames(_ map[string]struct{}) {
	// nothing to do
}

func (nf *noopFilter) apply(_ *blockSearch, _ *filterBitmap) {
	// nothing to do
}

// orFilter contains filters joined by OR operator.
//
// It is epxressed as `f1 OR f2 ... OR fN` in LogsQL.
type orFilter struct {
	filters []filter
}

func (of *orFilter) String() string {
	filters := of.filters
	a := make([]string, len(filters))
	for i, f := range filters {
		s := f.String()
		a[i] = s
	}
	return strings.Join(a, " or ")
}

func (of *orFilter) updateReferencedColumnNames(m map[string]struct{}) {
	for _, f := range of.filters {
		f.updateReferencedColumnNames(m)
	}
}

func (of *orFilter) apply(bs *blockSearch, bm *filterBitmap) {
	bmResult := getFilterBitmap(bm.bitsLen)
	bmTmp := getFilterBitmap(bm.bitsLen)
	for _, f := range of.filters {
		// Minimize the number of rows to check by the filter by checking only
		// the rows, which may change the output bm:
		// - bm matches them, e.g. the caller wants to get them
		// - bmResult doesn't match them, e.g. all the previous OR filters didn't match them
		bmTmp.copyFrom(bm)
		bmTmp.andNot(bmResult)
		if bmTmp.isZero() {
			// Shortcut - there is no need in applying the remaining filters,
			// since the result already matches all the values from the block.
			break
		}
		f.apply(bs, bmTmp)
		bmResult.or(bmTmp)
	}
	putFilterBitmap(bmTmp)
	bm.copyFrom(bmResult)
	putFilterBitmap(bmResult)
}

// andFilter contains filters joined by AND opertor.
//
// It is expressed as `f1 AND f2 ... AND fN` in LogsQL.
type andFilter struct {
	filters []filter
}

func (af *andFilter) String() string {
	filters := af.filters
	a := make([]string, len(filters))
	for i, f := range filters {
		s := f.String()
		switch f.(type) {
		case *orFilter:
			s = "(" + s + ")"
		}
		a[i] = s
	}
	return strings.Join(a, " ")
}

func (af *andFilter) updateReferencedColumnNames(m map[string]struct{}) {
	for _, f := range af.filters {
		f.updateReferencedColumnNames(m)
	}
}

func (af *andFilter) apply(bs *blockSearch, bm *filterBitmap) {
	for _, f := range af.filters {
		f.apply(bs, bm)
		if bm.isZero() {
			// Shortcut - there is no need in applying the remaining filters,
			// since the result will be zero anyway.
			return
		}
	}
}

// notFilter negates the filter.
//
// It is expressed as `NOT f` or `!f` in LogsQL.
type notFilter struct {
	f filter
}

func (nf *notFilter) String() string {
	s := nf.f.String()
	switch nf.f.(type) {
	case *andFilter, *orFilter:
		s = "(" + s + ")"
	}
	return "!" + s
}

func (nf *notFilter) updateReferencedColumnNames(m map[string]struct{}) {
	nf.f.updateReferencedColumnNames(m)
}

func (nf *notFilter) apply(bs *blockSearch, bm *filterBitmap) {
	// Minimize the number of rows to check by the filter by applying it
	// only to the rows, which match the bm, e.g. they may change the bm result.
	bmTmp := getFilterBitmap(bm.bitsLen)
	bmTmp.copyFrom(bm)
	nf.f.apply(bs, bmTmp)
	bm.andNot(bmTmp)
	putFilterBitmap(bmTmp)
}

// streamFilter is the filter for `_stream:{...}`
type streamFilter struct {
	// f is the filter to apply
	f *StreamFilter

	// tenantIDs is the list of tenantIDs to search for streamIDs.
	tenantIDs []TenantID

	// idb is the indexdb to search for streamIDs.
	idb *indexdb

	streamIDsOnce sync.Once
	streamIDs     map[streamID]struct{}
}

func (sf *streamFilter) String() string {
	s := sf.f.String()
	if s == "{}" {
		return ""
	}
	return "_stream:" + s
}

func (sf *streamFilter) getStreamIDs() map[streamID]struct{} {
	sf.streamIDsOnce.Do(sf.initStreamIDs)
	return sf.streamIDs
}

func (sf *streamFilter) initStreamIDs() {
	streamIDs := sf.idb.searchStreamIDs(sf.tenantIDs, sf.f)
	m := make(map[streamID]struct{}, len(streamIDs))
	for i := range streamIDs {
		m[streamIDs[i]] = struct{}{}
	}
	sf.streamIDs = m
}

func (sf *streamFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m["_stream"] = struct{}{}
}

func (sf *streamFilter) apply(bs *blockSearch, bm *filterBitmap) {
	if sf.f.isEmpty() {
		return
	}
	streamIDs := sf.getStreamIDs()
	if _, ok := streamIDs[bs.bsw.bh.streamID]; !ok {
		bm.resetBits()
		return
	}
}

// timeFilter filters by time.
//
// It is expressed as `_time:(start, end]` in LogsQL.
type timeFilter struct {
	minTimestamp int64
	maxTimestamp int64

	stringRepr string
}

func (tf *timeFilter) String() string {
	return "_time:" + tf.stringRepr
}

func (tf *timeFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m["_time"] = struct{}{}
}

func (tf *timeFilter) apply(bs *blockSearch, bm *filterBitmap) {
	minTimestamp := tf.minTimestamp
	maxTimestamp := tf.maxTimestamp

	if minTimestamp > maxTimestamp {
		bm.resetBits()
		return
	}

	th := bs.bsw.bh.timestampsHeader
	if minTimestamp > th.maxTimestamp || maxTimestamp < th.minTimestamp {
		bm.resetBits()
		return
	}
	if minTimestamp <= th.minTimestamp && maxTimestamp >= th.maxTimestamp {
		return
	}

	timestamps := bs.getTimestamps()
	bm.forEachSetBit(func(idx int) bool {
		ts := timestamps[idx]
		return ts >= minTimestamp && ts <= maxTimestamp
	})
}

// sequenceFilter matches an ordered sequence of phrases
//
// Example LogsQL: `fieldName:seq(foo, "bar baz")`
type sequenceFilter struct {
	fieldName string
	phrases   []string

	tokensOnce sync.Once
	tokens     []string

	nonEmptyPhrasesOnce sync.Once
	nonEmptyPhrases     []string
}

func (sf *sequenceFilter) String() string {
	phrases := sf.phrases
	a := make([]string, len(phrases))
	for i, phrase := range phrases {
		a[i] = quoteTokenIfNeeded(phrase)
	}
	return fmt.Sprintf("%sseq(%s)", quoteFieldNameIfNeeded(sf.fieldName), strings.Join(a, ","))
}

func (sf *sequenceFilter) getTokens() []string {
	sf.tokensOnce.Do(sf.initTokens)
	return sf.tokens
}

func (sf *sequenceFilter) initTokens() {
	phrases := sf.getNonEmptyPhrases()
	tokens := tokenizeStrings(nil, phrases)
	sf.tokens = tokens
}

func (sf *sequenceFilter) getNonEmptyPhrases() []string {
	sf.nonEmptyPhrasesOnce.Do(sf.initNonEmptyPhrases)
	return sf.nonEmptyPhrases
}

func (sf *sequenceFilter) initNonEmptyPhrases() {
	phrases := sf.phrases
	result := make([]string, 0, len(phrases))
	for _, phrase := range phrases {
		if phrase != "" {
			result = append(result, phrase)
		}
	}
	sf.nonEmptyPhrases = result
}

func (sf *sequenceFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[sf.fieldName] = struct{}{}
}

func (sf *sequenceFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := sf.fieldName
	phrases := sf.getNonEmptyPhrases()

	if len(phrases) == 0 {
		return
	}

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchSequence(v, phrases) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty phrase.
		if !matchSequence("", phrases) {
			bm.resetBits()
		}
		return
	}

	tokens := sf.getTokens()

	switch ch.valueType {
	case valueTypeString:
		matchStringBySequence(bs, ch, bm, phrases, tokens)
	case valueTypeDict:
		matchValuesDictBySequence(bs, ch, bm, phrases)
	case valueTypeUint8:
		matchUint8BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeUint16:
		matchUint16BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeUint32:
		matchUint32BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeUint64:
		matchUint64BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeFloat64:
		matchFloat64BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeIPv4:
		matchIPv4BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601BySequence(bs, ch, bm, phrases, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// exactPrefixFilter matches the exact prefix.
//
// Example LogsQL: `fieldName:exact("foo bar"*)
type exactPrefixFilter struct {
	fieldName string
	prefix    string

	tokensOnce sync.Once
	tokens     []string
}

func (ef *exactPrefixFilter) String() string {
	return fmt.Sprintf("%sexact(%s*)", quoteFieldNameIfNeeded(ef.fieldName), quoteTokenIfNeeded(ef.prefix))
}

func (ef *exactPrefixFilter) getTokens() []string {
	ef.tokensOnce.Do(ef.initTokens)
	return ef.tokens
}

func (ef *exactPrefixFilter) initTokens() {
	ef.tokens = getTokensSkipLast(ef.prefix)
}

func (ef *exactPrefixFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[ef.fieldName] = struct{}{}
}

func (ef *exactPrefixFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := ef.fieldName
	prefix := ef.prefix

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchExactPrefix(v, prefix) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		if !matchExactPrefix("", prefix) {
			bm.resetBits()
		}
		return
	}

	tokens := ef.getTokens()

	switch ch.valueType {
	case valueTypeString:
		matchStringByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeDict:
		matchValuesDictByExactPrefix(bs, ch, bm, prefix)
	case valueTypeUint8:
		matchUint8ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeUint16:
		matchUint16ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeUint32:
		matchUint32ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeUint64:
		matchUint64ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeFloat64:
		matchFloat64ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeIPv4:
		matchIPv4ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByExactPrefix(bs, ch, bm, prefix, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// exactFilter matches the exact value.
//
// Example LogsQL: `fieldName:exact("foo bar")`
type exactFilter struct {
	fieldName string
	value     string

	tokensOnce sync.Once
	tokens     []string
}

func (ef *exactFilter) String() string {
	return fmt.Sprintf("%sexact(%s)", quoteFieldNameIfNeeded(ef.fieldName), quoteTokenIfNeeded(ef.value))
}

func (ef *exactFilter) getTokens() []string {
	ef.tokensOnce.Do(ef.initTokens)
	return ef.tokens
}

func (ef *exactFilter) initTokens() {
	ef.tokens = tokenizeStrings(nil, []string{ef.value})
}

func (ef *exactFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[ef.fieldName] = struct{}{}
}

func (ef *exactFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := ef.fieldName
	value := ef.value

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if value != v {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty value.
		if value != "" {
			bm.resetBits()
		}
		return
	}

	tokens := ef.getTokens()

	switch ch.valueType {
	case valueTypeString:
		matchStringByExactValue(bs, ch, bm, value, tokens)
	case valueTypeDict:
		matchValuesDictByExactValue(bs, ch, bm, value)
	case valueTypeUint8:
		matchUint8ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeUint16:
		matchUint16ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeUint32:
		matchUint32ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeUint64:
		matchUint64ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeFloat64:
		matchFloat64ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeIPv4:
		matchIPv4ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByExactValue(bs, ch, bm, value, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// inFilter matches any exact value from the values map.
//
// Example LogsQL: `fieldName:in("foo", "bar baz")`
type inFilter struct {
	fieldName string
	values    []string

	tokenSetsOnce sync.Once
	tokenSets     [][]string

	stringValuesOnce sync.Once
	stringValues     map[string]struct{}

	uint8ValuesOnce sync.Once
	uint8Values     map[string]struct{}

	uint16ValuesOnce sync.Once
	uint16Values     map[string]struct{}

	uint32ValuesOnce sync.Once
	uint32Values     map[string]struct{}

	uint64ValuesOnce sync.Once
	uint64Values     map[string]struct{}

	float64ValuesOnce sync.Once
	float64Values     map[string]struct{}

	ipv4ValuesOnce sync.Once
	ipv4Values     map[string]struct{}

	timestampISO8601ValuesOnce sync.Once
	timestampISO8601Values     map[string]struct{}
}

func (af *inFilter) String() string {
	values := af.values
	a := make([]string, len(values))
	for i, value := range values {
		a[i] = quoteTokenIfNeeded(value)
	}
	return fmt.Sprintf("%sin(%s)", quoteFieldNameIfNeeded(af.fieldName), strings.Join(a, ","))
}

func (af *inFilter) getTokenSets() [][]string {
	af.tokenSetsOnce.Do(af.initTokenSets)
	return af.tokenSets
}

// It is faster to match every row in the block instead of checking too big number of tokenSets against bloom filter.
const maxTokenSetsToInit = 1000

func (af *inFilter) initTokenSets() {
	values := af.values
	tokenSetsLen := len(values)
	if tokenSetsLen > maxTokenSetsToInit {
		tokenSetsLen = maxTokenSetsToInit
	}
	tokenSets := make([][]string, 0, tokenSetsLen+1)
	for _, v := range values {
		tokens := tokenizeStrings(nil, []string{v})
		tokenSets = append(tokenSets, tokens)
		if len(tokens) > maxTokenSetsToInit {
			break
		}
	}
	af.tokenSets = tokenSets
}

func (af *inFilter) getStringValues() map[string]struct{} {
	af.stringValuesOnce.Do(af.initStringValues)
	return af.stringValues
}

func (af *inFilter) initStringValues() {
	values := af.values
	m := make(map[string]struct{}, len(values))
	for _, v := range values {
		m[v] = struct{}{}
	}
	af.stringValues = m
}

func (af *inFilter) getUint8Values() map[string]struct{} {
	af.uint8ValuesOnce.Do(af.initUint8Values)
	return af.uint8Values
}

func (af *inFilter) initUint8Values() {
	values := af.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*1)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok || n >= (1<<8) {
			continue
		}
		bufLen := len(buf)
		buf = append(buf, byte(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	af.uint8Values = m
}

func (af *inFilter) getUint16Values() map[string]struct{} {
	af.uint16ValuesOnce.Do(af.initUint16Values)
	return af.uint16Values
}

func (af *inFilter) initUint16Values() {
	values := af.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*2)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok || n >= (1<<16) {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint16(buf, uint16(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	af.uint16Values = m
}

func (af *inFilter) getUint32Values() map[string]struct{} {
	af.uint32ValuesOnce.Do(af.initUint32Values)
	return af.uint32Values
}

func (af *inFilter) initUint32Values() {
	values := af.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*4)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok || n >= (1<<32) {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint32(buf, uint32(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	af.uint32Values = m
}

func (af *inFilter) getUint64Values() map[string]struct{} {
	af.uint64ValuesOnce.Do(af.initUint64Values)
	return af.uint64Values
}

func (af *inFilter) initUint64Values() {
	values := af.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*8)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint64(buf, n)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	af.uint64Values = m
}

func (af *inFilter) getFloat64Values() map[string]struct{} {
	af.float64ValuesOnce.Do(af.initFloat64Values)
	return af.float64Values
}

func (af *inFilter) initFloat64Values() {
	values := af.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*8)
	for _, v := range values {
		f, ok := tryParseFloat64(v)
		if !ok {
			continue
		}
		n := math.Float64bits(f)
		bufLen := len(buf)
		buf = encoding.MarshalUint64(buf, n)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	af.float64Values = m
}

func (af *inFilter) getIPv4Values() map[string]struct{} {
	af.ipv4ValuesOnce.Do(af.initIPv4Values)
	return af.ipv4Values
}

func (af *inFilter) initIPv4Values() {
	values := af.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*4)
	for _, v := range values {
		n, ok := tryParseIPv4(v)
		if !ok {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint32(buf, uint32(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	af.ipv4Values = m
}

func (af *inFilter) getTimestampISO8601Values() map[string]struct{} {
	af.timestampISO8601ValuesOnce.Do(af.initTimestampISO8601Values)
	return af.timestampISO8601Values
}

func (af *inFilter) initTimestampISO8601Values() {
	values := af.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*8)
	for _, v := range values {
		n, ok := tryParseTimestampISO8601(v)
		if !ok {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint64(buf, n)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	af.timestampISO8601Values = m
}

func (af *inFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[af.fieldName] = struct{}{}
}

func (af *inFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := af.fieldName

	if len(af.values) == 0 {
		bm.resetBits()
		return
	}

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		stringValues := af.getStringValues()
		if _, ok := stringValues[v]; !ok {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty phrase.
		stringValues := af.getStringValues()
		if _, ok := stringValues[""]; !ok {
			bm.resetBits()
		}
		return
	}

	tokenSets := af.getTokenSets()

	switch ch.valueType {
	case valueTypeString:
		stringValues := af.getStringValues()
		matchAnyValue(bs, ch, bm, stringValues, tokenSets)
	case valueTypeDict:
		stringValues := af.getStringValues()
		matchValuesDictByAnyValue(bs, ch, bm, stringValues)
	case valueTypeUint8:
		binValues := af.getUint8Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeUint16:
		binValues := af.getUint16Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeUint32:
		binValues := af.getUint32Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeUint64:
		binValues := af.getUint64Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeFloat64:
		binValues := af.getFloat64Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeIPv4:
		binValues := af.getIPv4Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	case valueTypeTimestampISO8601:
		binValues := af.getTimestampISO8601Values()
		matchAnyValue(bs, ch, bm, binValues, tokenSets)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// ipv4RangeFilter matches the given ipv4 range [minValue..maxValue].
//
// Example LogsQL: `fieldName:ipv4_range(127.0.0.1, 127.0.0.255)`
type ipv4RangeFilter struct {
	fieldName string
	minValue  uint32
	maxValue  uint32
}

func (rf *ipv4RangeFilter) String() string {
	minValue := string(encoding.MarshalUint32(nil, rf.minValue))
	maxValue := string(encoding.MarshalUint32(nil, rf.maxValue))
	return fmt.Sprintf("%sipv4_range(%s, %s)", quoteFieldNameIfNeeded(rf.fieldName), toIPv4String(nil, minValue), toIPv4String(nil, maxValue))
}

func (rf *ipv4RangeFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[rf.fieldName] = struct{}{}
}

func (rf *ipv4RangeFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := rf.fieldName
	minValue := rf.minValue
	maxValue := rf.maxValue

	if minValue > maxValue {
		bm.resetBits()
		return
	}

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchIPv4Range(v, minValue, maxValue) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		bm.resetBits()
		return
	}

	switch ch.valueType {
	case valueTypeString:
		matchStringByIPv4Range(bs, ch, bm, minValue, maxValue)
	case valueTypeDict:
		matchValuesDictByIPv4Range(bs, ch, bm, minValue, maxValue)
	case valueTypeUint8:
		bm.resetBits()
	case valueTypeUint16:
		bm.resetBits()
	case valueTypeUint32:
		bm.resetBits()
	case valueTypeUint64:
		bm.resetBits()
	case valueTypeFloat64:
		bm.resetBits()
	case valueTypeIPv4:
		matchIPv4ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeTimestampISO8601:
		bm.resetBits()
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// stringRangeFilter matches tie given string range [minValue..maxValue)
//
// Note that the minValue is included in the range, while the maxValue isn't included in the range.
// This simplifies querying distincts log sets with string_range(A, B), string_range(B, C), etc.
//
// Example LogsQL: `fieldName:string_range(minValue, maxValue)`
type stringRangeFilter struct {
	fieldName string
	minValue  string
	maxValue  string
}

func (rf *stringRangeFilter) String() string {
	return fmt.Sprintf("%sstring_range(%s, %s)", quoteFieldNameIfNeeded(rf.fieldName), quoteTokenIfNeeded(rf.minValue), quoteTokenIfNeeded(rf.maxValue))
}

func (rf *stringRangeFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[rf.fieldName] = struct{}{}
}

func (rf *stringRangeFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := rf.fieldName
	minValue := rf.minValue
	maxValue := rf.maxValue

	if minValue > maxValue {
		bm.resetBits()
		return
	}

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchStringRange(v, minValue, maxValue) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		if !matchStringRange("", minValue, maxValue) {
			bm.resetBits()
		}
		return
	}

	switch ch.valueType {
	case valueTypeString:
		matchStringByStringRange(bs, ch, bm, minValue, maxValue)
	case valueTypeDict:
		matchValuesDictByStringRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint8:
		matchUint8ByStringRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint16:
		matchUint16ByStringRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint32:
		matchUint32ByStringRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint64:
		matchUint64ByStringRange(bs, ch, bm, minValue, maxValue)
	case valueTypeFloat64:
		matchFloat64ByStringRange(bs, ch, bm, minValue, maxValue)
	case valueTypeIPv4:
		matchIPv4ByStringRange(bs, ch, bm, minValue, maxValue)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByStringRange(bs, ch, bm, minValue, maxValue)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// lenRangeFilter matches field values with the length in the given range [minLen, maxLen].
//
// Example LogsQL: `fieldName:len_range(10, 20)`
type lenRangeFilter struct {
	fieldName string
	minLen    uint64
	maxLen    uint64
}

func (rf *lenRangeFilter) String() string {
	return quoteFieldNameIfNeeded(rf.fieldName) + fmt.Sprintf("len_range(%d,%d)", rf.minLen, rf.maxLen)
}

func (rf *lenRangeFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[rf.fieldName] = struct{}{}
}

func (rf *lenRangeFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := rf.fieldName
	minLen := rf.minLen
	maxLen := rf.maxLen

	if minLen > maxLen {
		bm.resetBits()
		return
	}

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchLenRange(v, minLen, maxLen) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		if !matchLenRange("", minLen, maxLen) {
			bm.resetBits()
		}
		return
	}

	switch ch.valueType {
	case valueTypeString:
		matchStringByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeDict:
		matchValuesDictByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeUint8:
		matchUint8ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeUint16:
		matchUint16ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeUint32:
		matchUint32ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeUint64:
		matchUint64ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeFloat64:
		matchFloat64ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeIPv4:
		matchIPv4ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByLenRange(bm, minLen, maxLen)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// rangeFilter matches the given range [minValue..maxValue].
//
// Example LogsQL: `fieldName:range(minValue, maxValue]`
type rangeFilter struct {
	fieldName string
	minValue  float64
	maxValue  float64

	stringRepr string
}

func (rf *rangeFilter) String() string {
	return quoteFieldNameIfNeeded(rf.fieldName) + "range" + rf.stringRepr
}

func (rf *rangeFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[rf.fieldName] = struct{}{}
}

func (rf *rangeFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := rf.fieldName
	minValue := rf.minValue
	maxValue := rf.maxValue

	if minValue > maxValue {
		bm.resetBits()
		return
	}

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchRange(v, minValue, maxValue) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		bm.resetBits()
		return
	}

	switch ch.valueType {
	case valueTypeString:
		matchStringByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeDict:
		matchValuesDictByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint8:
		matchUint8ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint16:
		matchUint16ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint32:
		matchUint32ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint64:
		matchUint64ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeFloat64:
		matchFloat64ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeIPv4:
		bm.resetBits()
	case valueTypeTimestampISO8601:
		bm.resetBits()
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// regexpFilter matches the given regexp
//
// Example LogsQL: `fieldName:re("regexp")`
type regexpFilter struct {
	fieldName string
	re        *regexp.Regexp
}

func (rf *regexpFilter) String() string {
	return fmt.Sprintf("%sre(%q)", quoteFieldNameIfNeeded(rf.fieldName), rf.re.String())
}

func (rf *regexpFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[rf.fieldName] = struct{}{}
}

func (rf *regexpFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := rf.fieldName
	re := rf.re

	// Verify whether filter matches const column
	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !re.MatchString(v) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		if !re.MatchString("") {
			bm.resetBits()
		}
		return
	}

	switch ch.valueType {
	case valueTypeString:
		matchStringByRegexp(bs, ch, bm, re)
	case valueTypeDict:
		matchValuesDictByRegexp(bs, ch, bm, re)
	case valueTypeUint8:
		matchUint8ByRegexp(bs, ch, bm, re)
	case valueTypeUint16:
		matchUint16ByRegexp(bs, ch, bm, re)
	case valueTypeUint32:
		matchUint32ByRegexp(bs, ch, bm, re)
	case valueTypeUint64:
		matchUint64ByRegexp(bs, ch, bm, re)
	case valueTypeFloat64:
		matchFloat64ByRegexp(bs, ch, bm, re)
	case valueTypeIPv4:
		matchIPv4ByRegexp(bs, ch, bm, re)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByRegexp(bs, ch, bm, re)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// anyCasePrefixFilter matches the given prefix in lower, upper and mixed case.
//
// Example LogsQL: `fieldName:i(prefix*)` or `fieldName:i("some prefix"*)`
//
// A special case `fieldName:i(*)` equals to `fieldName:*` and matches non-emtpy value for the given `fieldName` field.
type anyCasePrefixFilter struct {
	fieldName string
	prefix    string

	tokensOnce sync.Once
	tokens     []string
}

func (pf *anyCasePrefixFilter) String() string {
	if pf.prefix == "" {
		return quoteFieldNameIfNeeded(pf.fieldName) + "i(*)"
	}
	return fmt.Sprintf("%si(%s*)", quoteFieldNameIfNeeded(pf.fieldName), quoteTokenIfNeeded(pf.prefix))
}

func (pf *anyCasePrefixFilter) getTokens() []string {
	pf.tokensOnce.Do(pf.initTokens)
	return pf.tokens
}

func (pf *anyCasePrefixFilter) initTokens() {
	pf.tokens = getTokensSkipLast(pf.prefix)
}

func (pf *anyCasePrefixFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[pf.fieldName] = struct{}{}
}

func (pf *anyCasePrefixFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := pf.fieldName
	prefixLowercase := strings.ToLower(pf.prefix)

	// Verify whether pf matches const column
	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchAnyCasePrefix(v, prefixLowercase) {
			bm.resetBits()
		}
		return
	}

	// Verify whether pf matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		bm.resetBits()
		return
	}

	tokens := pf.getTokens()

	switch ch.valueType {
	case valueTypeString:
		matchStringByAnyCasePrefix(bs, ch, bm, prefixLowercase)
	case valueTypeDict:
		matchValuesDictByAnyCasePrefix(bs, ch, bm, prefixLowercase)
	case valueTypeUint8:
		matchUint8ByPrefix(bs, ch, bm, prefixLowercase)
	case valueTypeUint16:
		matchUint16ByPrefix(bs, ch, bm, prefixLowercase)
	case valueTypeUint32:
		matchUint32ByPrefix(bs, ch, bm, prefixLowercase)
	case valueTypeUint64:
		matchUint64ByPrefix(bs, ch, bm, prefixLowercase)
	case valueTypeFloat64:
		matchFloat64ByPrefix(bs, ch, bm, prefixLowercase, tokens)
	case valueTypeIPv4:
		matchIPv4ByPrefix(bs, ch, bm, prefixLowercase, tokens)
	case valueTypeTimestampISO8601:
		prefixUppercase := strings.ToUpper(pf.prefix)
		matchTimestampISO8601ByPrefix(bs, ch, bm, prefixUppercase, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// prefixFilter matches the given prefix.
//
// Example LogsQL: `fieldName:prefix*` or `fieldName:"some prefix"*`
//
// A special case `fieldName:*` matches non-empty value for the given `fieldName` field
type prefixFilter struct {
	fieldName string
	prefix    string

	tokensOnce sync.Once
	tokens     []string
}

func (pf *prefixFilter) String() string {
	if pf.prefix == "" {
		return quoteFieldNameIfNeeded(pf.fieldName) + "*"
	}
	return fmt.Sprintf("%s%s*", quoteFieldNameIfNeeded(pf.fieldName), quoteTokenIfNeeded(pf.prefix))
}

func (pf *prefixFilter) getTokens() []string {
	pf.tokensOnce.Do(pf.initTokens)
	return pf.tokens
}

func (pf *prefixFilter) initTokens() {
	pf.tokens = getTokensSkipLast(pf.prefix)
}

func (pf *prefixFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[pf.fieldName] = struct{}{}
}

func (pf *prefixFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := pf.fieldName
	prefix := pf.prefix

	// Verify whether pf matches const column
	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchPrefix(v, prefix) {
			bm.resetBits()
		}
		return
	}

	// Verify whether pf matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		bm.resetBits()
		return
	}

	tokens := pf.getTokens()

	switch ch.valueType {
	case valueTypeString:
		matchStringByPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeDict:
		matchValuesDictByPrefix(bs, ch, bm, prefix)
	case valueTypeUint8:
		matchUint8ByPrefix(bs, ch, bm, prefix)
	case valueTypeUint16:
		matchUint16ByPrefix(bs, ch, bm, prefix)
	case valueTypeUint32:
		matchUint32ByPrefix(bs, ch, bm, prefix)
	case valueTypeUint64:
		matchUint64ByPrefix(bs, ch, bm, prefix)
	case valueTypeFloat64:
		matchFloat64ByPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeIPv4:
		matchIPv4ByPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByPrefix(bs, ch, bm, prefix, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// anyCasePhraseFilter filters field entries by case-insensitive phrase match.
//
// An example LogsQL query: `fieldName:i(word)` or `fieldName:i("word1 ... wordN")`
type anyCasePhraseFilter struct {
	fieldName string
	phrase    string

	tokensOnce sync.Once
	tokens     []string
}

func (pf *anyCasePhraseFilter) String() string {
	return fmt.Sprintf("%si(%s)", quoteFieldNameIfNeeded(pf.fieldName), quoteTokenIfNeeded(pf.phrase))
}

func (pf *anyCasePhraseFilter) getTokens() []string {
	pf.tokensOnce.Do(pf.initTokens)
	return pf.tokens
}

func (pf *anyCasePhraseFilter) initTokens() {
	pf.tokens = tokenizeStrings(nil, []string{pf.phrase})
}

func (pf *anyCasePhraseFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[pf.fieldName] = struct{}{}
}

func (pf *anyCasePhraseFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := pf.fieldName
	phraseLowercase := strings.ToLower(pf.phrase)

	// Verify whether pf matches const column
	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchAnyCasePhrase(v, phraseLowercase) {
			bm.resetBits()
		}
		return
	}

	// Verify whether pf matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty phrase.
		if len(phraseLowercase) > 0 {
			bm.resetBits()
		}
		return
	}

	tokens := pf.getTokens()

	switch ch.valueType {
	case valueTypeString:
		matchStringByAnyCasePhrase(bs, ch, bm, phraseLowercase)
	case valueTypeDict:
		matchValuesDictByAnyCasePhrase(bs, ch, bm, phraseLowercase)
	case valueTypeUint8:
		matchUint8ByExactValue(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeUint16:
		matchUint16ByExactValue(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeUint32:
		matchUint32ByExactValue(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeUint64:
		matchUint64ByExactValue(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeFloat64:
		matchFloat64ByPhrase(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeIPv4:
		matchIPv4ByPhrase(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeTimestampISO8601:
		phraseUppercase := strings.ToUpper(pf.phrase)
		matchTimestampISO8601ByPhrase(bs, ch, bm, phraseUppercase, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

// phraseFilter filters field entries by phrase match (aka full text search).
//
// A phrase consists of any number of words with delimiters between them.
//
// An empty phrase matches only an empty string.
// A single-word phrase is the simplest LogsQL query: `fieldName:word`
//
// Multi-word phrase is expressed as `fieldName:"word1 ... wordN"` in LogsQL.
//
// A special case `fieldName:""` matches any value without `fieldName` field.
type phraseFilter struct {
	fieldName string
	phrase    string

	tokensOnce sync.Once
	tokens     []string
}

func (pf *phraseFilter) String() string {
	return quoteFieldNameIfNeeded(pf.fieldName) + quoteTokenIfNeeded(pf.phrase)
}

func (pf *phraseFilter) getTokens() []string {
	pf.tokensOnce.Do(pf.initTokens)
	return pf.tokens
}

func (pf *phraseFilter) initTokens() {
	pf.tokens = tokenizeStrings(nil, []string{pf.phrase})
}

func (pf *phraseFilter) updateReferencedColumnNames(m map[string]struct{}) {
	m[pf.fieldName] = struct{}{}
}

func (pf *phraseFilter) apply(bs *blockSearch, bm *filterBitmap) {
	fieldName := pf.fieldName
	phrase := pf.phrase

	// Verify whether pf matches const column
	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchPhrase(v, phrase) {
			bm.resetBits()
		}
		return
	}

	// Verify whether pf matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty phrase.
		if len(phrase) > 0 {
			bm.resetBits()
		}
		return
	}

	tokens := pf.getTokens()

	switch ch.valueType {
	case valueTypeString:
		matchStringByPhrase(bs, ch, bm, phrase, tokens)
	case valueTypeDict:
		matchValuesDictByPhrase(bs, ch, bm, phrase)
	case valueTypeUint8:
		matchUint8ByExactValue(bs, ch, bm, phrase, tokens)
	case valueTypeUint16:
		matchUint16ByExactValue(bs, ch, bm, phrase, tokens)
	case valueTypeUint32:
		matchUint32ByExactValue(bs, ch, bm, phrase, tokens)
	case valueTypeUint64:
		matchUint64ByExactValue(bs, ch, bm, phrase, tokens)
	case valueTypeFloat64:
		matchFloat64ByPhrase(bs, ch, bm, phrase, tokens)
	case valueTypeIPv4:
		matchIPv4ByPhrase(bs, ch, bm, phrase, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByPhrase(bs, ch, bm, phrase, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchTimestampISO8601ByLenRange(bm *filterBitmap, minLen, maxLen uint64) {
	if minLen > uint64(len(iso8601Timestamp)) || maxLen < uint64(len(iso8601Timestamp)) {
		bm.resetBits()
		return
	}
}

func matchTimestampISO8601ByStringRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue string) {
	if minValue > "9" || maxValue < "0" {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601StringExt(bs, bb, v)
		return matchStringRange(s, minValue, maxValue)
	})
	bbPool.Put(bb)
}

func matchTimestampISO8601ByRegexp(bs *blockSearch, ch *columnHeader, bm *filterBitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601StringExt(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchTimestampISO8601ByPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if prefix == "" {
		// Fast path - all the timestamp values match an empty prefix aka `*`
		return
	}
	// There is no sense in trying to parse prefix, since it may contain incomplete timestamp.
	// We cannot compar binary representation of timestamp and need converting
	// the timestamp to string before searching for the prefix there.
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601StringExt(bs, bb, v)
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchTimestampISO8601BySequence(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrases, tokens []string) {
	if len(phrases) == 1 {
		matchTimestampISO8601ByPhrase(bs, ch, bm, phrases[0], tokens)
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	// Slow path - phrases contain incomplete timestamp. Search over string representation of the timestamp.
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601StringExt(bs, bb, v)
		return matchSequence(s, phrases)
	})
	bbPool.Put(bb)
}

func matchTimestampISO8601ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if prefix == "" {
		return
	}
	if prefix < "0" || prefix > "9" || !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601StringExt(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchTimestampISO8601ByExactValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, value string, tokens []string) {
	n, ok := tryParseTimestampISO8601(value)
	if !ok || n < ch.minValue || n > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint64(bb.B, n)
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchTimestampISO8601ByPhrase(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrase string, tokens []string) {
	_, ok := tryParseTimestampISO8601(phrase)
	if ok {
		// Fast path - the phrase contains complete timestamp, so we can use exact search
		matchTimestampISO8601ByExactValue(bs, ch, bm, phrase, tokens)
		return
	}

	// Slow path - the phrase contains incomplete timestamp. Search over string representation of the timestamp.
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601StringExt(bs, bb, v)
		return matchPhrase(s, phrase)
	})
	bbPool.Put(bb)
}

func matchIPv4ByStringRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue string) {
	if minValue > "9" || maxValue < "0" {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4StringExt(bs, bb, v)
		return matchStringRange(s, minValue, maxValue)
	})
	bbPool.Put(bb)
}

func matchIPv4ByLenRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minLen, maxLen uint64) {
	if minLen > uint64(len("255.255.255.255")) || maxLen < uint64(len("0.0.0.0")) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4StringExt(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchIPv4ByRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue uint32) {
	if ch.minValue > uint64(maxValue) || ch.maxValue < uint64(minValue) {
		bm.resetBits()
		return
	}

	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 4 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of IPv4: got %d; want 4", bs.partPath(), len(v))
		}
		b := bytesutil.ToUnsafeBytes(v)
		n := encoding.UnmarshalUint32(b)
		return n >= minValue && n <= maxValue
	})
}

func matchIPv4ByRegexp(bs *blockSearch, ch *columnHeader, bm *filterBitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4StringExt(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchIPv4ByPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if prefix == "" {
		// Fast path - all the ipv4 values match an empty prefix aka `*`
		return
	}
	// There is no sense in trying to parse prefix, since it may contain incomplete ip.
	// We cannot compare binary representation of ip address and need converting
	// the ip to string before searching for the prefix there.
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4StringExt(bs, bb, v)
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchIPv4BySequence(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrases, tokens []string) {
	if len(phrases) == 1 {
		matchIPv4ByPhrase(bs, ch, bm, phrases[0], tokens)
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	// Slow path - phrases contain parts of IP address. For example, `1.23` should match `1.23.4.5` and `4.1.23.54`.
	// We cannot compare binary represetnation of ip address and need converting
	// the ip to string before searching for prefix there.
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4StringExt(bs, bb, v)
		return matchSequence(s, phrases)
	})
	bbPool.Put(bb)
}

func matchIPv4ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if prefix == "" {
		return
	}
	if prefix < "0" || prefix > "9" || len(tokens) > 3 || !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4StringExt(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchIPv4ByExactValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, value string, tokens []string) {
	n, ok := tryParseIPv4(value)
	if !ok || uint64(n) < ch.minValue || uint64(n) > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint32(bb.B, n)
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchIPv4ByPhrase(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrase string, tokens []string) {
	_, ok := tryParseIPv4(phrase)
	if ok {
		// Fast path - phrase contains the full IP address, so we can use exact matching
		matchIPv4ByExactValue(bs, ch, bm, phrase, tokens)
		return
	}

	// Slow path - the phrase may contain a part of IP address. For example, `1.23` should match `1.23.4.5` and `4.1.23.54`.
	// We cannot compare binary represetnation of ip address and need converting
	// the ip to string before searching for prefix there.
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4StringExt(bs, bb, v)
		return matchPhrase(s, phrase)
	})
	bbPool.Put(bb)
}

func matchFloat64ByStringRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue string) {
	if minValue > "9" || maxValue < "+" {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64StringExt(bs, bb, v)
		return matchStringRange(s, minValue, maxValue)
	})
	bbPool.Put(bb)
}

func matchFloat64ByLenRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minLen, maxLen uint64) {
	if minLen > 24 || maxLen == 0 {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64StringExt(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchFloat64ByRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue float64) {
	if minValue > math.Float64frombits(ch.maxValue) || maxValue < math.Float64frombits(ch.minValue) {
		bm.resetBits()
		return
	}

	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 8 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of floating-point number: got %d; want 8", bs.partPath(), len(v))
		}
		b := bytesutil.ToUnsafeBytes(v)
		n := encoding.UnmarshalUint64(b)
		f := math.Float64frombits(n)
		return f >= minValue && f <= maxValue
	})
}

func matchFloat64ByRegexp(bs *blockSearch, ch *columnHeader, bm *filterBitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64StringExt(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchFloat64ByPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if prefix == "" {
		// Fast path - all the float64 values match an empty prefix aka `*`
		return
	}
	// The prefix may contain a part of the floating-point number.
	// For example, `foo:12*` must match `12`, `123.456` and `-0.123`.
	// This means we cannot search in binary representation of floating-point numbers.
	// Instead, we need searching for the whole prefix in string representation
	// of floating-point numbers :(
	_, ok := tryParseFloat64(prefix)
	if !ok && prefix != "." && prefix != "+" && prefix != "-" && !strings.HasPrefix(prefix, "e") && !strings.HasPrefix(prefix, "E") {
		bm.resetBits()
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64StringExt(bs, bb, v)
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchFloat64BySequence(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrases, tokens []string) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	// The phrase may contain a part of the floating-point number.
	// For example, `foo:"123"` must match `123`, `123.456` and `-0.123`.
	// This means we cannot search in binary representation of floating-point numbers.
	// Instead, we need searching for the whole phrase in string representation
	// of floating-point numbers :(
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64StringExt(bs, bb, v)
		return matchSequence(s, phrases)
	})
	bbPool.Put(bb)
}

func matchFloat64ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if prefix == "" {
		// An empty prefix matches all the values
		return
	}
	if len(tokens) > 2 || !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64StringExt(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchFloat64ByExactValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, value string, tokens []string) {
	f, ok := tryParseFloat64(value)
	if !ok || f < math.Float64frombits(ch.minValue) || f > math.Float64frombits(ch.maxValue) {
		bm.resetBits()
		return
	}
	n := math.Float64bits(f)
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint64(bb.B, n)
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchFloat64ByPhrase(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrase string, tokens []string) {
	// The phrase may contain a part of the floating-point number.
	// For example, `foo:"123"` must match `123`, `123.456` and `-0.123`.
	// This means we cannot search in binary representation of floating-point numbers.
	// Instead, we need searching for the whole phrase in string representation
	// of floating-point numbers :(
	_, ok := tryParseFloat64(phrase)
	if !ok && phrase != "." && phrase != "+" && phrase != "-" {
		bm.resetBits()
		return
	}
	if n := strings.IndexByte(phrase, '.'); n > 0 && n < len(phrase)-1 {
		// Fast path - the phrase contains the exact floating-point number, so we can use exact search
		matchFloat64ByExactValue(bs, ch, bm, phrase, tokens)
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64StringExt(bs, bb, v)
		return matchPhrase(s, phrase)
	})
	bbPool.Put(bb)
}

func matchValuesDictByIPv4Range(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue uint32) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if matchIPv4Range(v, minValue, maxValue) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictByStringRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue string) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if matchStringRange(v, minValue, maxValue) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictByLenRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minLen, maxLen uint64) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if matchLenRange(v, minLen, maxLen) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictByRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue float64) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if matchRange(v, minValue, maxValue) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictByRegexp(bs *blockSearch, ch *columnHeader, bm *filterBitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if re.MatchString(v) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictByAnyCasePrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefixLowercase string) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if matchAnyCasePrefix(v, prefixLowercase) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictByAnyCasePhrase(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phraseLowercase string) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if matchAnyCasePhrase(v, phraseLowercase) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictByPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if matchPrefix(v, prefix) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictBySequence(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrases []string) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if matchSequence(v, phrases) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictByExactPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if matchExactPrefix(v, prefix) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictByExactValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, value string) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if v == value {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictByAnyValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, values map[string]struct{}) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if _, ok := values[v]; ok {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchValuesDictByPhrase(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrase string) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if matchPhrase(v, phrase) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchEncodedValuesDict(bs *blockSearch, ch *columnHeader, bm *filterBitmap, encodedValues []byte) {
	if len(encodedValues) == 0 {
		// Fast path - the phrase is missing in the valuesDict
		bm.resetBits()
		return
	}
	// Slow path - iterate over values
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 1 {
			logger.Panicf("FATAL: %s: unexpected length for dict value: got %d; want 1", bs.partPath(), len(v))
		}
		n := bytes.IndexByte(encodedValues, v[0])
		return n >= 0
	})
}

func matchStringByIPv4Range(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue uint32) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchIPv4Range(v, minValue, maxValue)
	})
}

func matchStringByStringRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue string) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchStringRange(v, minValue, maxValue)
	})
}

func matchStringByLenRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minLen, maxLen uint64) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchLenRange(v, minLen, maxLen)
	})
}

func matchStringByRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue float64) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchRange(v, minValue, maxValue)
	})
}

func matchStringByRegexp(bs *blockSearch, ch *columnHeader, bm *filterBitmap, re *regexp.Regexp) {
	visitValues(bs, ch, bm, func(v string) bool {
		return re.MatchString(v)
	})
}

func matchStringByAnyCasePrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefixLowercase string) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchAnyCasePrefix(v, prefixLowercase)
	})
}

func matchStringByAnyCasePhrase(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phraseLowercase string) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchAnyCasePhrase(v, phraseLowercase)
	})
}

func matchStringByPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return matchPrefix(v, prefix)
	})
}

func matchStringBySequence(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrases []string, tokens []string) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return matchSequence(v, phrases)
	})
}

func matchStringByExactPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return matchExactPrefix(v, prefix)
	})
}

func matchStringByExactValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, value string, tokens []string) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return v == value
	})
}

func matchStringByPhrase(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrase string, tokens []string) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return matchPhrase(v, phrase)
	})
}

func matchUint8ByStringRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue string) {
	if minValue > "9" || maxValue < "0" {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint8String(bs, bb, v)
		return matchStringRange(s, minValue, maxValue)
	})
	bbPool.Put(bb)
}

func matchUint16ByStringRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue string) {
	if minValue > "9" || maxValue < "0" {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint16String(bs, bb, v)
		return matchStringRange(s, minValue, maxValue)
	})
	bbPool.Put(bb)
}

func matchUint32ByStringRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue string) {
	if minValue > "9" || maxValue < "0" {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint32String(bs, bb, v)
		return matchStringRange(s, minValue, maxValue)
	})
	bbPool.Put(bb)
}

func matchUint64ByStringRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue string) {
	if minValue > "9" || maxValue < "0" {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint64String(bs, bb, v)
		return matchStringRange(s, minValue, maxValue)
	})
	bbPool.Put(bb)
}

func matchMinMaxValueLen(ch *columnHeader, minLen, maxLen uint64) bool {
	bb := bbPool.Get()
	defer bbPool.Put(bb)

	bb.B = strconv.AppendUint(bb.B[:0], ch.minValue, 10)
	s := bytesutil.ToUnsafeString(bb.B)
	if maxLen < uint64(len(s)) {
		return false
	}
	bb.B = strconv.AppendUint(bb.B[:0], ch.maxValue, 10)
	s = bytesutil.ToUnsafeString(bb.B)
	return minLen <= uint64(len(s))
}

func matchUint8ByLenRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minLen, maxLen uint64) {
	if !matchMinMaxValueLen(ch, minLen, maxLen) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint8String(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchUint16ByLenRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minLen, maxLen uint64) {
	if !matchMinMaxValueLen(ch, minLen, maxLen) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint16String(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchUint32ByLenRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minLen, maxLen uint64) {
	if !matchMinMaxValueLen(ch, minLen, maxLen) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint32String(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchUint64ByLenRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minLen, maxLen uint64) {
	if !matchMinMaxValueLen(ch, minLen, maxLen) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint64String(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchUint8ByRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 1 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint8 number: got %d; want 1", bs.partPath(), len(v))
		}
		n := uint64(v[0])
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchUint16ByRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 2 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint16 number: got %d; want 2", bs.partPath(), len(v))
		}
		b := bytesutil.ToUnsafeBytes(v)
		n := uint64(encoding.UnmarshalUint16(b))
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchUint32ByRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 4 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint8 number: got %d; want 4", bs.partPath(), len(v))
		}
		b := bytesutil.ToUnsafeBytes(v)
		n := uint64(encoding.UnmarshalUint32(b))
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchUint64ByRange(bs *blockSearch, ch *columnHeader, bm *filterBitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 8 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint8 number: got %d; want 8", bs.partPath(), len(v))
		}
		b := bytesutil.ToUnsafeBytes(v)
		n := encoding.UnmarshalUint64(b)
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchUint8ByRegexp(bs *blockSearch, ch *columnHeader, bm *filterBitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint8String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchUint16ByRegexp(bs *blockSearch, ch *columnHeader, bm *filterBitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint16String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchUint32ByRegexp(bs *blockSearch, ch *columnHeader, bm *filterBitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint32String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchUint64ByRegexp(bs *blockSearch, ch *columnHeader, bm *filterBitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint64String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchUint8ByPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string) {
	if prefix == "" {
		// Fast path - all the uint8 values match an empty prefix aka `*`
		return
	}
	// The prefix may contain a part of the number.
	// For example, `foo:12*` must match `12` and `123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole prefix in string representation of numbers :(
	n, ok := tryParseUint64(prefix)
	if !ok || n > ch.maxValue {
		bm.resetBits()
		return
	}
	// There is no need in matching against bloom filters, since tokens is empty.
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint8String(bs, bb, v)
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint16ByPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string) {
	if prefix == "" {
		// Fast path - all the uint16 values match an empty prefix aka `*`
		return
	}
	// The prefix may contain a part of the number.
	// For example, `foo:12*` must match `12` and `123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole prefix in string representation of numbers :(
	n, ok := tryParseUint64(prefix)
	if !ok || n > ch.maxValue {
		bm.resetBits()
		return
	}
	// There is no need in matching against bloom filters, since tokens is empty.
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint16String(bs, bb, v)
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint32ByPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string) {
	if prefix == "" {
		// Fast path - all the uint32 values match an empty prefix aka `*`
		return
	}
	// The prefix may contain a part of the number.
	// For example, `foo:12*` must match `12` and `123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole prefix in string representation of numbers :(
	n, ok := tryParseUint64(prefix)
	if !ok || n > ch.maxValue {
		bm.resetBits()
		return
	}
	// There is no need in matching against bloom filters, since tokens is empty.
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint32String(bs, bb, v)
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint64ByPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string) {
	if prefix == "" {
		// Fast path - all the uint64 values match an empty prefix aka `*`
		return
	}
	// The prefix may contain a part of the number.
	// For example, `foo:12*` must match `12` and `123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole prefix in string representation of numbers :(
	n, ok := tryParseUint64(prefix)
	if !ok || n > ch.maxValue {
		bm.resetBits()
		return
	}
	// There is no need in matching against bloom filters, since tokens is empty.
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint64String(bs, bb, v)
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint8BySequence(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrases, tokens []string) {
	if len(phrases) > 1 {
		bm.resetBits()
		return
	}
	matchUint8ByExactValue(bs, ch, bm, phrases[0], tokens)
}

func matchUint16BySequence(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrases, tokens []string) {
	if len(phrases) > 1 {
		bm.resetBits()
		return
	}
	matchUint16ByExactValue(bs, ch, bm, phrases[0], tokens)
}

func matchUint32BySequence(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrases, tokens []string) {
	if len(phrases) > 1 {
		bm.resetBits()
		return
	}
	matchUint32ByExactValue(bs, ch, bm, phrases[0], tokens)
}

func matchUint64BySequence(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrases, tokens []string) {
	if len(phrases) > 1 {
		bm.resetBits()
		return
	}
	matchUint64ByExactValue(bs, ch, bm, phrases[0], tokens)
}

func matchUint8ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if !matchMinMaxExactPrefix(ch, bm, prefix, tokens) {
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint8String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint16ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if !matchMinMaxExactPrefix(ch, bm, prefix, tokens) {
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint16String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint32ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if !matchMinMaxExactPrefix(ch, bm, prefix, tokens) {
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint32String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint64ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) {
	if !matchMinMaxExactPrefix(ch, bm, prefix, tokens) {
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint64String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchMinMaxExactPrefix(ch *columnHeader, bm *filterBitmap, prefix string, tokens []string) bool {
	if prefix == "" {
		// An empty prefix matches all the values
		return false
	}
	if len(tokens) > 0 {
		// Non-empty tokens means that the prefix contains at least two tokens.
		// Multiple tokens cannot match any uint value.
		bm.resetBits()
		return false
	}
	n, ok := tryParseUint64(prefix)
	if !ok || n > ch.maxValue {
		bm.resetBits()
		return false
	}
	return true
}

func matchUint8ByExactValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrase string, tokens []string) {
	n, ok := tryParseUint64(phrase)
	if !ok || n < ch.minValue || n > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = append(bb.B, byte(n))
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchUint16ByExactValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrase string, tokens []string) {
	n, ok := tryParseUint64(phrase)
	if !ok || n < ch.minValue || n > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint16(bb.B, uint16(n))
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchUint32ByExactValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrase string, tokens []string) {
	n, ok := tryParseUint64(phrase)
	if !ok || n < ch.minValue || n > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint32(bb.B, uint32(n))
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchUint64ByExactValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, phrase string, tokens []string) {
	n, ok := tryParseUint64(phrase)
	if !ok || n < ch.minValue || n > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint64(bb.B, n)
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchBinaryValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, binValue []byte, tokens []string) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return v == string(binValue)
	})
}

func matchAnyValue(bs *blockSearch, ch *columnHeader, bm *filterBitmap, values map[string]struct{}, tokenSets [][]string) {
	if !matchBloomFilterAnyTokenSet(bs, ch, tokenSets) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		_, ok := values[v]
		return ok
	})
}

func matchBloomFilterAnyTokenSet(bs *blockSearch, ch *columnHeader, tokenSets [][]string) bool {
	if len(tokenSets) == 0 {
		return false
	}
	if len(tokenSets) > maxTokenSetsToInit || uint64(len(tokenSets)) > 10*bs.bsw.bh.rowsCount {
		// It is faster to match every row in the block against all the values
		// instead of using bloom filter for too big number of tokenSets.
		return true
	}
	bf := bs.getBloomFilterForColumn(ch)
	for _, tokens := range tokenSets {
		if bf.containsAll(tokens) {
			return true
		}
	}
	return false
}

func matchBloomFilterAllTokens(bs *blockSearch, ch *columnHeader, tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}
	bf := bs.getBloomFilterForColumn(ch)
	return bf.containsAll(tokens)
}

func visitValues(bs *blockSearch, ch *columnHeader, bm *filterBitmap, f func(value string) bool) {
	if bm.isZero() {
		// Fast path - nothing to visit
		return
	}
	values := bs.getValuesForColumn(ch)
	bm.forEachSetBit(func(idx int) bool {
		return f(values[idx])
	})
}

func matchAnyCasePrefix(s, prefixLowercase string) bool {
	sLowercase := strings.ToLower(s)
	return matchPrefix(sLowercase, prefixLowercase)
}

func matchPrefix(s, prefix string) bool {
	if len(prefix) == 0 {
		return len(s) > 0
	}
	r, _ := utf8.DecodeRuneInString(prefix)
	startsWithToken := isTokenRune(r)
	offset := 0
	for {
		n := strings.Index(s[offset:], prefix)
		if n < 0 {
			return false
		}
		offset += n
		// Make sure that the found phrase contains non-token chars at the beginning
		if startsWithToken && offset > 0 {
			r, _ := utf8.DecodeLastRuneInString(s[:offset])
			if r == utf8.RuneError || isTokenRune(r) {
				offset++
				continue
			}
		}
		return true
	}
}

func matchIPv4Range(s string, minValue, maxValue uint32) bool {
	n, ok := tryParseIPv4(s)
	if !ok {
		return false
	}
	return n >= minValue && n <= maxValue
}

func matchStringRange(s, minValue, maxValue string) bool {
	return s >= minValue && s < maxValue
}

func matchLenRange(s string, minLen, maxLen uint64) bool {
	sLen := uint64(utf8.RuneCountInString(s))
	return sLen >= minLen && sLen <= maxLen
}

func matchRange(s string, minValue, maxValue float64) bool {
	f, ok := tryParseFloat64(s)
	if !ok {
		return false
	}
	return f >= minValue && f <= maxValue
}

func matchSequence(s string, phrases []string) bool {
	for _, phrase := range phrases {
		n := getPhrasePos(s, phrase)
		if n < 0 {
			return false
		}
		s = s[n+len(phrase):]
	}
	return true
}

func matchAnyCasePhrase(s, phraseLowercase string) bool {
	sLowercase := strings.ToLower(s)
	return matchPhrase(sLowercase, phraseLowercase)
}

func matchExactPrefix(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

func matchPhrase(s, phrase string) bool {
	if len(phrase) == 0 {
		return len(s) == 0
	}
	n := getPhrasePos(s, phrase)
	return n >= 0
}

func getPhrasePos(s, phrase string) int {
	r, _ := utf8.DecodeRuneInString(phrase)
	startsWithToken := isTokenRune(r)
	r, _ = utf8.DecodeLastRuneInString(phrase)
	endsWithToken := isTokenRune(r)
	pos := 0
	for {
		n := strings.Index(s[pos:], phrase)
		if n < 0 {
			return -1
		}
		pos += n
		// Make sure that the found phrase contains non-token chars at the beginning and at the end
		if startsWithToken && pos > 0 {
			r, _ := utf8.DecodeLastRuneInString(s[:pos])
			if r == utf8.RuneError || isTokenRune(r) {
				pos++
				continue
			}
		}
		if endsWithToken && pos+len(phrase) < len(s) {
			r, _ := utf8.DecodeRuneInString(s[pos+len(phrase):])
			if r == utf8.RuneError || isTokenRune(r) {
				pos++
				continue
			}
		}
		return pos
	}
}

type stringBucket struct {
	a []string
}

func (sb *stringBucket) reset() {
	a := sb.a
	for i := range a {
		a[i] = ""
	}
	sb.a = a[:0]
}

func getStringBucket() *stringBucket {
	v := stringBucketPool.Get()
	if v == nil {
		return &stringBucket{}
	}
	return v.(*stringBucket)
}

func putStringBucket(sb *stringBucket) {
	sb.reset()
	stringBucketPool.Put(sb)
}

var stringBucketPool sync.Pool

func getTokensSkipLast(s string) []string {
	for {
		r, runeSize := utf8.DecodeLastRuneInString(s)
		if !isTokenRune(r) {
			break
		}
		s = s[:len(s)-runeSize]
	}
	return tokenizeStrings(nil, []string{s})
}

func toUint64Range(minValue, maxValue float64) (uint64, uint64) {
	minValue = math.Ceil(minValue)
	maxValue = math.Floor(maxValue)
	return toUint64Clamp(minValue), toUint64Clamp(maxValue)
}

func toUint64Clamp(f float64) uint64 {
	if f < 0 {
		return 0
	}
	if f > math.MaxUint64 {
		return math.MaxUint64
	}
	return uint64(f)
}

func quoteFieldNameIfNeeded(s string) string {
	if s == "_msg" || s == "" {
		return ""
	}
	return quoteTokenIfNeeded(s) + ":"
}

func toUint8String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 1 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of uint8 number: got %d; want 1", bs.partPath(), len(v))
	}
	n := uint64(v[0])
	bb.B = strconv.AppendUint(bb.B[:0], n, 10)
	return bytesutil.ToUnsafeString(bb.B)
}

func toUint16String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 2 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of uint16 number: got %d; want 2", bs.partPath(), len(v))
	}
	b := bytesutil.ToUnsafeBytes(v)
	n := uint64(encoding.UnmarshalUint16(b))
	bb.B = strconv.AppendUint(bb.B[:0], n, 10)
	return bytesutil.ToUnsafeString(bb.B)
}

func toUint32String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 4 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of uint32 number: got %d; want 4", bs.partPath(), len(v))
	}
	b := bytesutil.ToUnsafeBytes(v)
	n := uint64(encoding.UnmarshalUint32(b))
	bb.B = strconv.AppendUint(bb.B[:0], n, 10)
	return bytesutil.ToUnsafeString(bb.B)
}

func toUint64String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 8 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of uint64 number: got %d; want 8", bs.partPath(), len(v))
	}
	b := bytesutil.ToUnsafeBytes(v)
	n := encoding.UnmarshalUint64(b)
	bb.B = strconv.AppendUint(bb.B[:0], n, 10)
	return bytesutil.ToUnsafeString(bb.B)
}

func toFloat64StringExt(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 8 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of floating-point number: got %d; want 8", bs.partPath(), len(v))
	}
	bb.B = toFloat64String(bb.B[:0], v)
	return bytesutil.ToUnsafeString(bb.B)
}

func toIPv4StringExt(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 4 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of IPv4: got %d; want 4", bs.partPath(), len(v))
	}
	bb.B = toIPv4String(bb.B[:0], v)
	return bytesutil.ToUnsafeString(bb.B)
}

func toTimestampISO8601StringExt(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 8 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of ISO8601 timestamp: got %d; want 8", bs.partPath(), len(v))
	}
	bb.B = toTimestampISO8601String(bb.B[:0], v)
	return bytesutil.ToUnsafeString(bb.B)
}
