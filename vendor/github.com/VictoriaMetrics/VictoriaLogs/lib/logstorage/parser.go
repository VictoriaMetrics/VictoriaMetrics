package logstorage

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type lexer struct {
	// s contains unparsed tail of sOrig
	s string

	// sOrig contains the original string
	sOrig string

	// token contains the current token
	//
	// an empty token means the end of s
	token string

	// rawToken contains raw token before unquoting
	rawToken string

	// prevRawToken contains the previously parsed token before unquoting
	prevRawToken string

	// isSkippedSpace is set to true if there was a whitespace before the token in s
	isSkippedSpace bool

	// currentTimestamp is the current timestamp in nanoseconds.
	//
	// It is used for proper initializing of _time filters with relative time ranges.
	currentTimestamp int64

	// opts is a stack of options for nested parsed queries
	optss []*queryOptions
}

type lexerState struct {
	lex lexer
}

func (lex *lexer) copyFrom(src *lexer) {
	*lex = *src
	lex.optss = append(lex.optss[:0:0], src.optss...)
}

func (lex *lexer) backupState() *lexerState {
	var ls lexerState
	ls.lex.copyFrom(lex)
	return &ls
}

func (lex *lexer) restoreState(ls *lexerState) {
	lex.copyFrom(&ls.lex)
}

func (lex *lexer) pushQueryOptions(opts *queryOptions) {
	lex.optss = append(lex.optss, opts)
}

func (lex *lexer) popQueryOptions() {
	lex.optss = lex.optss[:len(lex.optss)-1]
}

func (lex *lexer) getQueryOptions() *queryOptions {
	if len(lex.optss) == 0 {
		return nil
	}
	return lex.optss[len(lex.optss)-1]
}

// newLexer returns new lexer for the given s at the given timestamp.
//
// The timestamp is used for properly parsing relative timestamps such as _time:1d.
//
// The lex.token points to the first token in s.
func newLexer(s string, timestamp int64) *lexer {
	lex := &lexer{
		s:                s,
		sOrig:            s,
		currentTimestamp: timestamp,
	}
	lex.nextToken()
	return lex
}

func (lex *lexer) isEnd() bool {
	return len(lex.s) == 0 && len(lex.token) == 0 && len(lex.rawToken) == 0
}

func (lex *lexer) isQuotedToken() bool {
	return lex.token != lex.rawToken
}

func (lex *lexer) nextCompoundToken() (string, error) {
	return lex.nextCompoundTokenExt(nil)
}

func (lex *lexer) nextCompoundMathToken() (string, error) {
	return lex.nextCompoundTokenExt(mathStopCompoundTokens)
}

func (lex *lexer) nextCompoundTokenExt(stopTokens []string) (string, error) {
	if lex.isQuotedToken() {
		// Quoted tokens cannot be a part of compound token, so return them as is.
		s := lex.token
		lex.nextToken()
		return s, nil
	}

	if !lex.isSkippedSpace && lex.isKeywordAny(deniedFirstCompoundTokens) && isWord(lex.prevRawToken) {
		return "", fmt.Errorf("missing whitespace between %q and %q", lex.prevRawToken, lex.token)
	}

	if !lex.isAllowedCompoundToken(stopTokens) {
		return "", fmt.Errorf("compound token cannot start with %q; put it into quotes if needed", lex.token)
	}

	s := lex.token
	lex.nextToken()

	for !lex.isSkippedSpace && lex.isAllowedCompoundToken(stopTokens) {
		s += lex.rawToken
		lex.nextToken()
	}

	if slices.Contains(glueCompoundTokens, s) {
		// Disallow a single-char compound token with glue chars, since this is error-prone.
		// See https://github.com/VictoriaMetrics/VictoriaLogs/issues/590
		return "", fmt.Errorf("compound token cannot be equal to %q; put it into quotes if needed", s)
	}

	return s, nil
}

func (lex *lexer) isAllowedCompoundToken(stopTokens []string) bool {
	if lex.isQuotedToken() {
		// Quoted token cannot be a part of compound token
		return false
	}

	if len(lex.token) == 0 {
		// Missing token (EOF).
		return false
	}

	// Stop tokens are disallowed in the compound token.
	if lex.isKeywordAny(stopTokens) {
		return false
	}

	// Glue tokens are allowed to be a part of compound token.
	if lex.isKeywordAny(glueCompoundTokens) {
		return true
	}

	// Regular word token is allowed to be a part of compound token.
	return isWord(lex.token)
}

func isWord(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isTokenRune(r) {
			return false
		}
	}
	return true
}

// deniedFirstCompoundTokens contains disallowed starting tokens for compound tokens without the whitespace in front of these tokens.
var deniedFirstCompoundTokens = []string{
	"/",
	".",
	"$",
}

// glueCompoundTokens contains tokens allowed inside unquoted compound tokens.
var glueCompoundTokens = []string{
	"+", // Seen in time formats: 2025-07-20T10:20:30+03:00
	"-", // Seen in hostnames: foo-bar-baz
	"/", // Seen in paths: foo/bar/baz
	":", // Seen in tcp addresses: foo:1235
	".", // Seen in hostnames: foobar.com
	"$", // Seen in PHP-like vars: $foo
}

// mathStopCompoundTokens contains tokens from the glueCompoundTokens, which are disallowed in math compound tokens.
var mathStopCompoundTokens = []string{
	"+",
	"-",
	"/",
}

func (lex *lexer) isPrevRawToken(tokens []string) bool {
	prevTokenLower := strings.ToLower(lex.prevRawToken)
	for _, token := range tokens {
		if token == prevTokenLower {
			return true
		}
	}
	return false
}

func (lex *lexer) checkPrevAdjacentToken(tokens ...string) error {
	if lex.isSkippedSpace || lex.prevRawToken == "" {
		return nil
	}

	if !lex.isPrevRawToken(tokens) {
		return fmt.Errorf("missing whitespace or ':' between %q and %q; probably, the whole string must be put into quotes", lex.prevRawToken, lex.token)
	}

	return nil
}

func (lex *lexer) isKeyword(keywords ...string) bool {
	return lex.isKeywordAny(keywords)
}

func (lex *lexer) isKeywordAny(keywords []string) bool {
	if lex.isQuotedToken() {
		return false
	}
	tokenLower := strings.ToLower(lex.token)
	for _, kw := range keywords {
		if kw == tokenLower {
			return true
		}
	}
	return false
}

func (lex *lexer) context() string {
	tail := lex.sOrig
	tail = tail[:len(tail)-len(lex.s)]
	if len(tail) > 50 {
		tail = tail[len(tail)-50:]
	}
	return tail
}

func (lex *lexer) nextCharToken(s string, size int) {
	lex.token = s[:size]
	lex.rawToken = lex.token
	lex.s = s[size:]
}

// nextToken updates lex.token to the next token.
func (lex *lexer) nextToken() {
	s := lex.s
	lex.prevRawToken = lex.rawToken
	lex.token = ""
	lex.rawToken = ""
	lex.isSkippedSpace = false

	if len(s) == 0 {
		return
	}

again:
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		lex.nextCharToken(s, size)
		return
	}

	// Skip whitespace
	for unicode.IsSpace(r) {
		lex.isSkippedSpace = true
		s = s[size:]
		r, size = utf8.DecodeRuneInString(s)
	}

	if r == '#' {
		// skip comment till \n
		n := strings.IndexByte(s, '\n')
		if n < 0 {
			s = ""
		} else {
			s = s[n+1:]
		}
		goto again
	}

	// Try decoding simple token
	tokenLen := 0
	for isTokenRune(r) {
		tokenLen += size
		r, size = utf8.DecodeRuneInString(s[tokenLen:])
	}
	if tokenLen > 0 {
		lex.nextCharToken(s, tokenLen)
		return
	}

	switch r {
	case '"', '`':
		prefix, err := strconv.QuotedPrefix(s)
		if err != nil {
			lex.nextCharToken(s, 1)
			return
		}
		token, err := strconv.Unquote(prefix)
		if err != nil {
			lex.nextCharToken(s, 1)
			return
		}
		lex.token = token
		lex.rawToken = prefix
		lex.s = s[len(prefix):]
		return
	case '\'':
		var b []byte
		for !strings.HasPrefix(s[size:], "'") {
			ch, _, newTail, err := strconv.UnquoteChar(s[size:], '\'')
			if err != nil {
				lex.nextCharToken(s, 1)
				return
			}
			b = utf8.AppendRune(b, ch)
			size = len(s) - len(newTail)
		}
		size++
		lex.token = string(b)
		lex.rawToken = string(s[:size])
		lex.s = s[size:]
		return
	case '=':
		if strings.HasPrefix(s[size:], "~") {
			lex.nextCharToken(s, 2)
			return
		}
		lex.nextCharToken(s, 1)
		return
	case '!':
		if strings.HasPrefix(s[size:], "~") || strings.HasPrefix(s[size:], "=") {
			lex.nextCharToken(s, 2)
			return
		}
		lex.nextCharToken(s, 1)
		return
	default:
		lex.nextCharToken(s, size)
		return
	}
}

// Query represents LogsQL query.
type Query struct {
	opts queryOptions

	f filter

	pipes []pipe

	// timestamp is the timestamp context used for parsing the query.
	timestamp int64
}

type queryOptions struct {
	// needPrint is set to true if the queryOptions must be printed in the queryOptions.String().
	needPrint bool

	// concurrency is the number of concurrent CPU-bound workers to use for a single query execution.
	//
	// By default the number of concurrent workers equals to the number of available CPU cores.
	concurrency uint

	// parallelReaders is the number of concurrent IO-bound data readers to use for a single query execution.
	//
	// By default the number of parallel readers equals to concurrency.
	parallelReaders uint

	// if ignoreGlobalTimeFilter is set, then Query.AddTimeFilter doesn't add the time filter to the query and to all its subqueries.
	ignoreGlobalTimeFilter *bool

	// allowPartialResponse allows returning partial responses in VictoriaLogs cluster setup when some of vlstorage nodes are temporarily unavailable.
	allowPartialResponse *bool

	// timeOffset is the number of nanoseconds to subtracts from all time filters in the query.
	//
	// The timeOffset is also added to the selected _time field values before being passed to query pipes.
	timeOffset int64

	// timeOffsetStr is a string representation of the timeOffset.
	timeOffsetStr string
}

func (opts *queryOptions) String() string {
	if opts == nil || !opts.needPrint {
		return ""
	}
	var a []string
	if opts.concurrency > 0 {
		a = append(a, fmt.Sprintf("concurrency=%d", opts.concurrency))
	}
	if opts.ignoreGlobalTimeFilter != nil {
		a = append(a, fmt.Sprintf("ignore_global_time_filter=%v", *opts.ignoreGlobalTimeFilter))
	}
	if opts.allowPartialResponse != nil {
		a = append(a, fmt.Sprintf("allow_partial_response=%v", *opts.allowPartialResponse))
	}
	if opts.timeOffsetStr != "" {
		a = append(a, fmt.Sprintf("time_offset=%s", opts.timeOffsetStr))
	}
	if len(a) == 0 {
		return ""
	}
	return "options(" + strings.Join(a, ", ") + ")"
}

// String returns string representation for q.
func (q *Query) String() string {
	s := q.opts.String()
	if len(s) > 0 {
		s += " "
	}

	s += q.f.String()

	for _, p := range q.pipes {
		s += " | " + p.String()
	}

	return s
}

// GetParallelReaders returns the number of parallel readers to use for executing the given query.
func (q *Query) GetParallelReaders(defaultParallelReaders int) int {
	n := int(q.opts.parallelReaders)
	if n <= 0 {
		n = int(q.opts.concurrency)
	}
	if n <= 0 {
		n = defaultParallelReaders
	}
	if n <= 0 {
		n = 2 * cgroup.AvailableCPUs()
	}
	if n > maxParallelReaders {
		n = maxParallelReaders
	}
	return n
}

// GetConcurrency returns concurrency for the q.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#query-options
func (q *Query) GetConcurrency() int {
	concurrency := cgroup.AvailableCPUs()
	if q.opts.concurrency > 0 && int(q.opts.concurrency) < concurrency {
		// Limit the number of workers by the number of available CPU cores,
		// since bigger number of workers won't improve CPU-bound query performance -
		// they just increase RAM usage and slow down query execution because
		// of more context switches between workers.
		concurrency = int(q.opts.concurrency)
	}
	return concurrency
}

// CanLiveTail returns true if q can be used in live tailing
func (q *Query) CanLiveTail() bool {
	for _, p := range q.pipes {
		if !p.canLiveTail() {
			return false
		}
	}
	return true
}

func (q *Query) getStreamIDs() []streamID {
	switch t := q.f.(type) {
	case *filterAnd:
		for _, f := range t.filters {
			streamIDs, ok := getStreamIDsFromFilterOr(f)
			if ok {
				return streamIDs
			}
		}
		return nil
	default:
		streamIDs, _ := getStreamIDsFromFilterOr(q.f)
		return streamIDs
	}
}

func getStreamIDsFromFilterOr(f filter) ([]streamID, bool) {
	switch t := f.(type) {
	case *filterOr:
		streamIDsFilters := 0
		var streamIDs []streamID
		for _, f := range t.filters {
			fs, ok := f.(*filterStreamID)
			if !ok {
				return nil, false
			}
			streamIDsFilters++
			streamIDs = append(streamIDs, fs.streamIDs...)
		}
		return streamIDs, streamIDsFilters > 0
	case *filterStreamID:
		return t.streamIDs, true
	default:
		return nil, false
	}
}

// DropAllPipes drops all the pipes from q.
func (q *Query) DropAllPipes() {
	q.pipes = nil
}

func (q *Query) addFieldsFilters(pf *prefixfilter.Filter) {
	qStr := "*" + toFieldsFilters(pf)
	qTmp, err := ParseQueryAtTimestamp(qStr, q.GetTimestamp())
	if err != nil {
		logger.Panicf("BUG: cannot parse query with fields filters: %s", err)
	}
	q.pipes = append(q.pipes, qTmp.pipes...)
}

// AddFacetsPipe adds ' facets <limit> max_values_per_field <maxValuesPerField> max_value_len <maxValueLen> <keepConstFields>` to the end of q.
func (q *Query) AddFacetsPipe(limit, maxValuesPerField, maxValueLen int, keepConstFields bool) {
	s := "facets"
	if limit > 0 {
		s += fmt.Sprintf(" %d", limit)
	}
	if maxValuesPerField > 0 {
		s += fmt.Sprintf(" max_values_per_field %d", maxValuesPerField)
	}
	if maxValueLen > 0 {
		s += fmt.Sprintf(" max_value_len %d", maxValueLen)
	}
	if keepConstFields {
		s += " keep_const_fields"
	}

	q.mustAppendPipe(s)
}

// AddCountByTimePipe adds '| stats by (_time:step offset off, field1, ..., fieldN) count() hits' to the end of q.
func (q *Query) AddCountByTimePipe(step, off int64, fields []string) {
	// Drop pipes from q, which modify or delete _time field, since they make impossible to calculate stats grouped by _time.
	q.dropPipesUnsafeForHits()

	{
		// add 'stats by (_time:step offset off, fields) count() hits'
		stepStr := string(marshalDurationString(nil, step))
		byFieldsStr := "_time:" + stepStr
		if off != 0 {
			offsetStr := string(marshalDurationString(nil, off))
			byFieldsStr += " offset " + offsetStr
		}
		for _, f := range fields {
			byFieldsStr += ", " + quoteTokenIfNeeded(f)
		}
		s := fmt.Sprintf("stats by (%s) count() hits", byFieldsStr)

		q.mustAppendPipe(s)
	}

	{
		// Add 'sort by (_time, fields)' in order to get consistent order of the results.
		sortFieldsStr := "_time"
		for _, f := range fields {
			sortFieldsStr += ", " + quoteTokenIfNeeded(f)
		}
		s := fmt.Sprintf("sort by (%s)", sortFieldsStr)

		q.mustAppendPipe(s)
	}
}

// dropPipesUnsafeForHits drops trailing pipes from q, which are unsafe
// for calculating hits grouped by _time.
func (q *Query) dropPipesUnsafeForHits() {
	for i, p := range q.pipes {
		if !isPipeSafeForHits(p) {
			// Drop the rest of the pipes, including the current pipe,
			// since it modified or deletes the _time field.
			q.pipes = q.pipes[:i]
			return
		}
	}
}

func isPipeSafeForHits(p pipe) bool {
	if p.canReturnLastNResults() {
		return true
	}

	switch t := p.(type) {
	case *pipeUnion:
		// Allow union pipes, but drop pipes unsafe for hits inside them.
		// See https://github.com/VictoriaMetrics/VictoriaLogs/issues/641
		t.q.dropPipesUnsafeForHits()
		return true
	case *pipeJoin:
		// Allow join pipes, since they do not drop _time field.
		return true
	default:
		return false
	}
}

// Clone returns a copy of q at the given timestamp.
func (q *Query) Clone(timestamp int64) *Query {
	qStr := q.String()
	qCopy, err := ParseQueryAtTimestamp(qStr, timestamp)
	if err != nil {
		logger.Panicf("BUG: cannot parse %q: %s", qStr, err)
	}
	return qCopy
}

func (q *Query) cloneShallow() *Query {
	qCopy := *q
	return &qCopy
}

// CloneWithTimeFilter clones q at the given timestamp and adds _time:[start, end] filter to the cloned q.
func (q *Query) CloneWithTimeFilter(timestamp, start, end int64) *Query {
	qCopy := q.Clone(timestamp)
	qCopy.AddTimeFilter(start, end)
	return qCopy
}

// GetLastNResultsQuery() returns a query for optimized querying of the last <limit> results with the biggest _time values with an optional <offset>.
//
// The returned query is nil if q cannot be used for optimized querying of the last N results.
func (q *Query) GetLastNResultsQuery() (qOpt *Query, offset uint64, limit uint64) {
	start, end := q.GetFilterTimeRange()
	if !CanApplyLastNResultsOptimization(start, end) {
		// It is faster to execute the query as is on such a small time range.
		return nil, 0, 0
	}

	pipes := q.pipes

	// Remember the trailing 'fields' and 'delete' pipes - they are moved in front of `sort` pipe below.
	tailPipes := func() []pipe {
		for i := len(pipes) - 1; i >= 0; i-- {
			switch pipes[i].(type) {
			case *pipeFields, *pipeDelete:
				// Skip 'fields' and 'delete' pipes.
			default:
				return pipes[i+1:]
			}
		}
		return pipes
	}()
	pipes = pipes[:len(pipes)-len(tailPipes)]
	if len(pipes) == 0 {
		return nil, 0, 0
	}

	// The query must end with one of the following pipes in order to be eligible for the optimization:
	// - 'sort by (_time desc) offset <offset> limit <limit>'
	// - 'first <limit> by (_time desc)'
	// - 'last <limit> by (_time)'
	pLast := pipes[len(pipes)-1]
	offset, limit, ok := getOffsetLimitFromPipe(pLast)
	if !ok {
		return nil, 0, 0
	}

	// Remove the `| sort ...` pipe from the query, add tailPipes and verify
	// whether it can reliably return last N results with the biggest _time values.
	qCopy := q.Clone(q.GetTimestamp())
	if len(qCopy.pipes) != len(q.pipes) {
		return nil, 0, 0
	}
	qCopy.pipes = qCopy.pipes[:len(pipes)-1]
	qCopy.pipes = append(qCopy.pipes, tailPipes...)
	if !qCopy.CanReturnLastNResults() {
		return nil, 0, 0
	}

	// The query is eligible for last N results optimization.
	return qCopy, offset, limit
}

// CanApplyLastNResultsOptimization returns true if there is sense for applying 'last N' optimization for the query on the time range [start, end]
func CanApplyLastNResultsOptimization(start, end int64) bool {
	return end/2-start/2 > nsecsPerSecond
}

func getOffsetLimitFromPipe(p pipe) (uint64, uint64, bool) {
	switch t := p.(type) {
	case *pipeSort:
		return getOffsetLimitFromPipeSort(t)
	case *pipeFirst:
		return getOffsetLimitFromPipeSort(t.ps)
	case *pipeLast:
		return getOffsetLimitFromPipeSort(t.ps)
	default:
		return 0, 0, false
	}
}

func getOffsetLimitFromPipeSort(ps *pipeSort) (uint64, uint64, bool) {
	if ps.limit <= 0 || ps.limit > 50_000 {
		return 0, 0, false
	}
	if ps.offset > 50_000 {
		return 0, 0, false
	}
	if ps.rankFieldName != "" {
		return 0, 0, false
	}
	if len(ps.partitionByFields) > 0 {
		return 0, 0, false
	}
	if len(ps.byFields) != 1 {
		return 0, 0, false
	}
	if ps.byFields[0].name != "_time" {
		return 0, 0, false
	}
	isDesc := ps.byFields[0].isDesc
	if ps.isDesc {
		isDesc = !isDesc
	}
	if !isDesc {
		return 0, 0, false
	}
	return ps.offset, ps.limit, true
}

// CanReturnLastNResults returns true if time range filter at q can be adjusted for returning the last N results with the biggest _time values.
func (q *Query) CanReturnLastNResults() bool {
	for _, p := range q.pipes {
		if !p.canReturnLastNResults() {
			return false
		}
	}
	return true
}

// GetFilterTimeRange returns filter time range for the given q.
func (q *Query) GetFilterTimeRange() (int64, int64) {
	switch t := q.f.(type) {
	case *filterAnd:
		minTimestamp := int64(math.MinInt64)
		maxTimestamp := int64(math.MaxInt64)
		for _, filter := range t.filters {
			ft, ok := filter.(*filterTime)
			if ok {
				if ft.minTimestamp > minTimestamp {
					minTimestamp = ft.minTimestamp
				}
				if ft.maxTimestamp < maxTimestamp {
					maxTimestamp = ft.maxTimestamp
				}
			}
		}
		return minTimestamp, maxTimestamp
	case *filterTime:
		return t.minTimestamp, t.maxTimestamp
	}
	return math.MinInt64, math.MaxInt64
}

// AddTimeFilter adds global filter _time:[start ... end] to q.
func (q *Query) AddTimeFilter(start, end int64) {
	q.visitSubqueries(func(q *Query) {
		q.addTimeFilterNoSubqueries(start, end)
	})
}

func (q *Query) addTimeFilterNoSubqueries(start, end int64) {
	if q.opts.ignoreGlobalTimeFilter != nil && *q.opts.ignoreGlobalTimeFilter {
		return
	}

	timeOffset := q.opts.timeOffset

	// use nanosecond precision for [start, end] time range in order to avoid
	// automatic adjustement of timestamps for its' string representation.
	// See https://github.com/VictoriaMetrics/VictoriaLogs/issues/587
	//
	// Do not use numeric representation of timestamps, since they are improperly parsed
	// for negative timestamps (they are parsed as relative to the current time)
	// and for timestamps with less than 15 decimal digits (they are parsed as microsends,
	// milliseconds or seconds depending on the number of decimal digit).
	startStr := marshalTimestampRFC3339NanoPreciseString(nil, start)
	endStr := marshalTimestampRFC3339NanoPreciseString(nil, end)

	ft := &filterTime{
		minTimestamp: subNoOverflowInt64(start, timeOffset),
		maxTimestamp: subNoOverflowInt64(end, timeOffset),

		stringRepr: fmt.Sprintf("[%s,%s]", startStr, endStr),
	}

	fa, ok := q.f.(*filterAnd)
	if ok {
		filters := make([]filter, len(fa.filters)+1)
		filters[0] = ft
		copy(filters[1:], fa.filters)
		fa.filters = filters
	} else {
		q.f = &filterAnd{
			filters: []filter{ft, q.f},
		}
	}

	q.f = flattenFiltersAnd(q.f)

	// Remove `*` filters after adding the `_time` filter, since they are no longer needed.
	q.f = removeStarFilters(q.f)

	// Initialize rate functions with the step calculated from HTTP time filter
	// This fixes the bug where rate_sum() doesn't divide by stepSeconds when
	// time filter is specified via HTTP params instead of LogsQL expression
	q.initStatsRateFuncsFromTimeFilter()
}

// AddExtraFilters adds extraFilters to q
func (q *Query) AddExtraFilters(extraFilters *Filter) {
	if extraFilters == nil || extraFilters.f == nil {
		return
	}

	filters := []filter{extraFilters.f}
	q.visitSubqueries(func(q *Query) {
		q.addExtraFiltersNoSubqueries(filters)
	})
}

func (q *Query) addExtraFiltersNoSubqueries(filters []filter) {
	fa, ok := q.f.(*filterAnd)
	if ok {
		fa.filters = append(filters, fa.filters...)
	} else {
		q.f = &filterAnd{
			filters: append(filters, q.f),
		}
	}

	q.optimizeNoSubqueries()
}

// AddPipeSortByTimeDesc adds `| sort (_time) desc` pipe to q.
func (q *Query) AddPipeSortByTimeDesc() {
	s := "sort by (_time) desc"
	q.mustAppendPipe(s)
}

// AddPipeOffsetLimit adds `| offset <offset> | limit <limit>` pipes to q.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#offset-pipe
// and https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe
func (q *Query) AddPipeOffsetLimit(offset, limit uint64) {
	offsetStr := fmt.Sprintf("offset %d", offset)
	q.mustAppendPipe(offsetStr)

	limitStr := fmt.Sprintf("limit %d", limit)
	q.mustAppendPipe(limitStr)

	// optimize the query, so the `offset` and `limit` pipes could be joined with the preceding `sort` pipe.
	q.pipes = optimizeOffsetLimitPipes(q.pipes)
}

func (q *Query) mustAppendPipe(s string) {
	timestamp := q.GetTimestamp()
	p := mustParsePipe(s, timestamp)
	q.pipes = append(q.pipes, p)
}

// optimize applies various optimizations to q.
func (q *Query) optimize() {
	q.visitSubqueries(func(q *Query) {
		q.optimizeNoSubqueries()
	})
}

func (q *Query) optimizeNoSubqueries() {
	q.pipes = optimizeOffsetLimitPipes(q.pipes)
	q.pipes = optimizeUniqLimitPipes(q.pipes)
	q.pipes = optimizeFilterPipes(q.pipes)

	// Merge `q | filter ...` into q.
	if len(q.pipes) > 0 {
		pf, ok := q.pipes[0].(*pipeFilter)
		if ok {
			q.f = mergeFiltersAnd(q.f, pf.f)
			q.pipes = append(q.pipes[:0], q.pipes[1:]...)
		}
	}

	// Optimize `q | field_names ...` by marking pipeFieldNames as first pipe.
	if len(q.pipes) > 0 {
		pf, ok := q.pipes[0].(*pipeFieldNames)
		if ok {
			pf.isFirstPipe = true
		}
	}

	// flatten nested AND filters
	q.f = flattenFiltersAnd(q.f)

	// flatten nested OR filters
	q.f = flattenFiltersOr(q.f)

	// Substitute '*' prefixFilter with filterNoop in order to avoid reading _msg data.
	q.f = removeStarFilters(q.f)

	// Merge multiple {...} filters into a single one.
	q.f = mergeFiltersStream(q.f)
}

func (q *Query) visitSubqueries(visitFunc func(q *Query)) {
	if q == nil {
		return
	}

	// call f for the query itself.
	visitFunc(q)

	// Visit subqueries in all the filters at q.
	visitSubqueriesInFilter(q.f, visitFunc)

	// Visit subqueries in all the pipes at q.
	for _, p := range q.pipes {
		p.visitSubqueries(visitFunc)
	}
}

func visitSubqueriesInFilter(f filter, visitFunc func(q *Query)) {
	if f == nil {
		return
	}
	callback := func(f filter) bool {
		switch t := f.(type) {
		case *filterIn:
			t.values.q.visitSubqueries(visitFunc)
		case *filterContainsAll:
			t.values.q.visitSubqueries(visitFunc)
		case *filterContainsAny:
			t.values.q.visitSubqueries(visitFunc)
		case *filterStreamID:
			t.q.visitSubqueries(visitFunc)
		}
		return false
	}
	_ = visitFilterRecursive(f, callback)
}

func mergeFiltersStream(f filter) filter {
	fa, ok := f.(*filterAnd)
	if !ok {
		return f
	}
	fss := make([]*filterStream, 0, len(fa.filters))
	otherFilters := make([]filter, 0, len(fa.filters))
	for _, f := range fa.filters {
		fs, ok := f.(*filterStream)
		if ok {
			fss = append(fss, fs)
		} else {
			otherFilters = append(otherFilters, f)
		}
	}
	if len(fss) == 0 {
		// Nothing to merge
		return f
	}

	fss = mergeFiltersStreamInternal(fss)
	filters := make([]filter, 0, len(fss)+len(otherFilters))
	for _, fs := range fss {
		filters = append(filters, fs)
	}
	filters = append(filters, otherFilters...)
	fa = &filterAnd{
		filters: filters,
	}
	return fa
}

func mergeFiltersStreamInternal(fss []*filterStream) []*filterStream {
	if len(fss) < 2 {
		return fss
	}

	for _, fs := range fss {
		if len(fs.f.orFilters) != 1 {
			// Cannot merge or filters :(
			return fss
		}
	}

	var tfs []*streamTagFilter
	for _, fs := range fss {
		tfs = append(tfs, fs.f.orFilters[0].tagFilters...)
	}
	return []*filterStream{
		{
			f: &StreamFilter{
				orFilters: []*andStreamFilter{
					{
						tagFilters: tfs,
					},
				},
			},
		},
	}
}

// GetStatsByFields returns `by (...)` fields from the last `stats` pipe at q.
func (q *Query) GetStatsByFields() ([]string, error) {
	return q.GetStatsByFieldsAddGroupingByTime(0)
}

// GetStatsByFieldsAddGroupingByTime returns `by (...)` fields from the last `stats` pipe at q.
//
// if step > 0, then _time:step is added to the last `stats by (...)` pipe at q.
func (q *Query) GetStatsByFieldsAddGroupingByTime(step int64) ([]string, error) {
	pipes := q.pipes

	idx := getLastPipeStatsIdx(pipes)
	if idx < 0 {
		return nil, fmt.Errorf("missing `| stats ...` pipe in the query [%s]", q)
	}
	ps := pipes[idx].(*pipeStats)

	// For range stats (step > 0), verify that pipes in front of the last `stats` pipe
	// do not modify or delete the `_time` field, since it is required for bucketing by step.
	// For instant stats (step == 0), allow such pipes for broader query flexibility.
	if step > 0 {
		for i := 0; i < idx; i++ {
			p := pipes[i]
			if _, ok := p.(*pipeStats); ok {
				// Skip `stats` pipe, since it is updated with the grouping by `_time` in the addByTimeFieldToStatsPipes() below.
				continue
			}
			if !p.canReturnLastNResults() {
				return nil, fmt.Errorf("the pipe `| %q` cannot be put in front of `| %q`, since it modifies or deletes `_time` field", p, ps)
			}
		}
	}

	// add _time:step to by (...) list at stats pipes.
	q.addByTimeFieldToStatsPipes(step)

	// propagate the step into rate* funcs at stats pipes.
	q.initStatsRateFuncs(step)

	// add 'partition by (_time)' to 'sort', 'first' and 'last' pipes.
	q.addPartitionByTime(step)

	// extract by(...) field names from ps
	byFields := make([]string, len(ps.byFields))
	for i, f := range ps.byFields {
		byFields[i] = f.name
	}

	// extract metric fields from stats pipe
	metricFields := make(map[string]struct{}, len(ps.funcs))
	for i := range ps.funcs {
		f := &ps.funcs[i]
		if slices.Contains(byFields, f.resultName) {
			return nil, fmt.Errorf("the %q field cannot be overridden at %q in the query [%s]", f.resultName, ps, q)
		}
		metricFields[f.resultName] = struct{}{}
	}

	// verify that all the pipes after the idx do not add new fields
	for i := idx + 1; i < len(pipes); i++ {
		p := pipes[i]
		switch t := p.(type) {
		case *pipeFilter:
			// This pipe doesn't change the set of fields.
		case *pipeFirst, *pipeLast, *pipeSort:
			// These pipes do not change the set of fields.
		case *pipeRunningStats:
			// `| running_stats ...` pipe must contain the same byFields as the preceding `stats` pipe.
			if !hasNeededFieldsExceptTime(t.byFields, byFields) {
				return nil, fmt.Errorf("the %q must contain the same list of fields as `stats` pipe in the query [%s]", t, q)
			}
			// `| running_stats ...` pipe cannot override byFields.
			for _, f := range t.funcs {
				if slices.Contains(byFields, f.resultName) {
					return nil, fmt.Errorf("the %q field cannot be overridden at %q in the query [%s]", f.resultName, t, q)
				}
				metricFields[f.resultName] = struct{}{}
			}
		case *pipeMath:
			// Allow `| math ...` pipe, since it adds additional metrics to the given set of fields.
			// Verify that the result fields at math pipe do not override byFields.
			for _, me := range t.entries {
				if slices.Contains(byFields, me.resultField) {
					return nil, fmt.Errorf("the %q field cannot be overridden at %q in the query [%s]", me.resultField, t, q)
				}
				metricFields[me.resultField] = struct{}{}
			}
		case *pipeFields:
			// `| fields ...` pipe must contain all the by(...) fields, otherwise it breaks output.
			for _, f := range byFields {
				if !prefixfilter.MatchFilters(t.fieldFilters, f) {
					return nil, fmt.Errorf("missing %q field at %q pipe in the query [%s]", f, p, q)
				}
			}
			for f := range maps.Clone(metricFields) {
				if !prefixfilter.MatchFilters(t.fieldFilters, f) {
					delete(metricFields, f)
				}
			}
		case *pipeDelete:
			// Disallow deleting by(...) fields, since this breaks output.
			for _, f := range byFields {
				if prefixfilter.MatchFilters(t.fieldFilters, f) {
					return nil, fmt.Errorf("the %q field cannot be deleted via %q in the query [%s]", f, p, q)
				}
			}
			for f := range maps.Clone(metricFields) {
				if prefixfilter.MatchFilters(t.fieldFilters, f) {
					delete(metricFields, f)
				}
			}
		case *pipeCopy:
			// Add copied fields to by(...) fields list.
			for i := range t.srcFieldFilters {
				fSrc := t.srcFieldFilters[i]
				fDst := t.dstFieldFilters[i]

				for _, f := range byFields {
					if prefixfilter.MatchFilter(fDst, f) {
						return nil, fmt.Errorf("the %q field cannot be overridden at %q in the query [%s]", f, t, q)
					}
					if prefixfilter.MatchFilter(fSrc, f) {
						dstFieldName := string(prefixfilter.AppendReplace(nil, fSrc, fDst, f))
						if slices.Contains(byFields, dstFieldName) {
							return nil, fmt.Errorf("the %q field cannot be overridden at %q in the query [%s]", dstFieldName, t, q)
						}
						byFields = append(byFields, dstFieldName)
					}
				}
				for f := range maps.Clone(metricFields) {
					if prefixfilter.MatchFilter(fDst, f) {
						delete(metricFields, f)
					}
					if prefixfilter.MatchFilter(fSrc, f) {
						dstFieldName := string(prefixfilter.AppendReplace(nil, fSrc, fDst, f))
						metricFields[dstFieldName] = struct{}{}
					}
				}
			}
		case *pipeRename:
			// Update by(...) fields with dst fields
			for i := range t.srcFieldFilters {
				fSrc := t.srcFieldFilters[i]
				fDst := t.dstFieldFilters[i]

				for j, f := range byFields {
					if prefixfilter.MatchFilter(fDst, f) {
						return nil, fmt.Errorf("the %q field cannot be overridden at %q in the query [%s]", f, t, q)
					}
					if prefixfilter.MatchFilter(fSrc, f) {
						dstFieldName := string(prefixfilter.AppendReplace(nil, fSrc, fDst, f))
						if slices.Contains(byFields, dstFieldName) {
							return nil, fmt.Errorf("the %q field cannot be overridden at %q in the query [%s]", dstFieldName, t, q)
						}
						byFields[j] = dstFieldName
					}
				}

				for f := range maps.Clone(metricFields) {
					if prefixfilter.MatchFilter(fDst, f) {
						delete(metricFields, f)
					}
					if prefixfilter.MatchFilter(fSrc, f) {
						delete(metricFields, f)
						dstFieldName := string(prefixfilter.AppendReplace(nil, fSrc, fDst, f))
						metricFields[dstFieldName] = struct{}{}
					}
				}
			}
		case *pipeFormat:
			// Assume that `| format ...` pipe generates an additional by(...) label
			if slices.Contains(byFields, t.resultField) {
				return nil, fmt.Errorf("the %q field cannot be overridden at %q in the query [%s]", t.resultField, t, q)
			}
			byFields = append(byFields, t.resultField)
			delete(metricFields, t.resultField)
		default:
			return nil, fmt.Errorf("the %q pipe cannot be put after %q pipe in the query [%s]", p, ps, q)
		}
	}

	if len(metricFields) == 0 {
		return nil, fmt.Errorf("missing metric fields in the results of query [%s]", q)
	}

	return byFields, nil
}

func hasNeededFieldsExceptTime(fields, neededFields []string) bool {
	for _, f := range neededFields {
		if f == "_time" {
			continue
		}
		if !slices.Contains(fields, f) {
			return false
		}
	}

	for _, f := range fields {
		if !slices.Contains(neededFields, f) {
			return false
		}
	}

	return true
}

func getLastPipeStatsIdx(pipes []pipe) int {
	for i := len(pipes) - 1; i >= 0; i-- {
		if _, ok := pipes[i].(*pipeStats); ok {
			return i
		}
	}
	return -1
}

func updateFilterWithTimeOffset(f filter, timeOffset int64) filter {
	if timeOffset == 0 {
		// Nothing to update
		return f
	}

	visitFunc := func(f filter) bool {
		_, ok := f.(*filterTime)
		return ok
	}
	copyFunc := func(f filter) (filter, error) {
		ft := f.(*filterTime)
		ftCopy := *ft
		ftCopy.minTimestamp = subNoOverflowInt64(ft.minTimestamp, timeOffset)
		ftCopy.maxTimestamp = subNoOverflowInt64(ft.maxTimestamp, timeOffset)
		return &ftCopy, nil
	}
	f, err := copyFilter(f, visitFunc, copyFunc)
	if err != nil {
		logger.Panicf("BUG: unexpected error: %s", err)
	}
	return f
}

func flattenFiltersAnd(f filter) filter {
	visitFunc := func(f filter) bool {
		fa, ok := f.(*filterAnd)
		if !ok {
			return false
		}
		for _, f := range fa.filters {
			if _, ok := f.(*filterAnd); ok {
				return true
			}
		}
		return false
	}
	copyFunc := func(f filter) (filter, error) {
		fa := f.(*filterAnd)

		var resultFilters []filter
		for _, f := range fa.filters {
			child, ok := f.(*filterAnd)
			if !ok {
				resultFilters = append(resultFilters, f)
				continue
			}
			resultFilters = append(resultFilters, child.filters...)
		}
		return &filterAnd{
			filters: resultFilters,
		}, nil
	}
	f, err := copyFilter(f, visitFunc, copyFunc)
	if err != nil {
		logger.Panicf("BUG: unexpected error: %s", err)
	}
	return f
}

func flattenFiltersOr(f filter) filter {
	visitFunc := func(f filter) bool {
		fo, ok := f.(*filterOr)
		if !ok {
			return false
		}
		for _, f := range fo.filters {
			if _, ok := f.(*filterOr); ok {
				return true
			}
		}
		return false
	}
	copyFunc := func(f filter) (filter, error) {
		fo := f.(*filterOr)

		var resultFilters []filter
		for _, f := range fo.filters {
			child, ok := f.(*filterOr)
			if !ok {
				resultFilters = append(resultFilters, f)
				continue
			}
			resultFilters = append(resultFilters, child.filters...)
		}
		return &filterOr{
			filters: resultFilters,
		}, nil
	}
	f, err := copyFilter(f, visitFunc, copyFunc)
	if err != nil {
		logger.Panicf("BUG: unexpected error: %s", err)
	}
	return f
}

func removeStarFilters(f filter) filter {
	// Substitute `*` filterPrefix with filterNoop
	visitFunc := func(f filter) bool {
		fp, ok := f.(*filterPrefix)
		return ok && isMsgFieldName(fp.fieldName) && fp.prefix == ""
	}
	copyFunc := func(_ filter) (filter, error) {
		fn := &filterNoop{}
		return fn, nil
	}
	f, err := copyFilter(f, visitFunc, copyFunc)
	if err != nil {
		logger.Panicf("BUG: unexpected error: %s", err)
	}

	// Replace filterOr with filterNoop if one of its sub-filters are filterNoop
	visitFunc = func(f filter) bool {
		fo, ok := f.(*filterOr)
		if !ok {
			return false
		}
		for _, f := range fo.filters {
			if _, ok := f.(*filterNoop); ok {
				return true
			}
		}
		return false
	}
	copyFunc = func(_ filter) (filter, error) {
		fn := &filterNoop{}
		return fn, nil
	}
	f, err = copyFilter(f, visitFunc, copyFunc)
	if err != nil {
		logger.Panicf("BUG: unexpected error: %s", err)
	}

	// Drop filterNoop inside filterAnd
	visitFunc = func(f filter) bool {
		fa, ok := f.(*filterAnd)
		if !ok {
			return false
		}
		for _, f := range fa.filters {
			if _, ok := f.(*filterNoop); ok {
				return true
			}
		}
		return false
	}
	copyFunc = func(f filter) (filter, error) {
		fa := f.(*filterAnd)
		var resultFilters []filter
		for _, f := range fa.filters {
			if _, ok := f.(*filterNoop); !ok {
				resultFilters = append(resultFilters, f)
			}
		}
		if len(resultFilters) == 0 {
			return &filterNoop{}, nil
		}
		if len(resultFilters) == 1 {
			return resultFilters[0], nil
		}
		return &filterAnd{
			filters: resultFilters,
		}, nil
	}
	f, err = copyFilter(f, visitFunc, copyFunc)
	if err != nil {
		logger.Panicf("BUG: unexpected error: %s", err)
	}

	return f
}

func optimizeOffsetLimitPipes(pipes []pipe) []pipe {
	for {
		pipesLen := len(pipes)
		pipes = optimizeOffsetLimitPipesInternal(pipes)
		if len(pipes) == pipesLen {
			return pipes
		}
	}
}

func optimizeOffsetLimitPipesInternal(pipes []pipe) []pipe {
	// Replace '| offset X | limit Y' with '| limit X+Y | offset X'.
	// This reduces the number of rows processed by remote storage.
	// See: https://github.com/VictoriaMetrics/VictoriaLogs/issues/620#issuecomment-3276624504
	for i := 0; i < len(pipes)-1; i++ {
		po, ok := pipes[i].(*pipeOffset)
		if !ok {
			continue
		}
		pl, ok := pipes[i+1].(*pipeLimit)
		if !ok {
			continue
		}

		pl.limit += po.offset
		pipes[i] = pl
		pipes[i+1] = po
	}

	// Merge 'offset X | offset Y' into 'offset X+Y'.
	i := 1
	for i < len(pipes) {
		po1, ok1 := pipes[i-1].(*pipeOffset)
		po2, ok2 := pipes[i].(*pipeOffset)
		if !ok1 || !ok2 {
			i++
			continue
		}

		po1.offset += po2.offset
		pipes = append(pipes[:i], pipes[i+1:]...)
	}

	// Merge 'limit N | limit M' into 'limit min(N, M)'.
	i = 1
	for i < len(pipes) {
		pl1, ok1 := pipes[i-1].(*pipeLimit)
		pl2, ok2 := pipes[i].(*pipeLimit)
		if !ok1 || !ok2 {
			i++
			continue
		}

		pl1.limit = min(pl1.limit, pl2.limit)
		pipes = append(pipes[:i], pipes[i+1:]...)
	}

	// Replace '| limit X | offset Y' with 'limit 0' if Y >= X.
	i = 1
	for i < len(pipes) {
		pl, ok1 := pipes[i-1].(*pipeLimit)
		po, ok2 := pipes[i].(*pipeOffset)
		if !ok1 || !ok2 || po.offset < pl.limit {
			i++
			continue
		}

		pl.limit = 0
		pipes = append(pipes[:i], pipes[i+1:]...)
	}

	// Remove `offset 0`.
	i = 0
	for i < len(pipes) {
		po, ok := pipes[i].(*pipeOffset)
		if !ok || po.offset != 0 {
			i++
			continue
		}
		pipes = append(pipes[:i], pipes[i+1:]...)
	}

	// Merge '| sort ... | offset ... | limit ...' into '| sort ... offset ... limit ...'.
	pipes = optimizeSortOffsetPipes(pipes)
	pipes = optimizeSortLimitPipes(pipes)

	return pipes
}

func optimizeSortOffsetPipes(pipes []pipe) []pipe {
	// Merge 'sort ... | offset ...' into 'sort ... offset ...'
	i := 1
	for i < len(pipes) {
		ps, ok1 := pipes[i-1].(*pipeSort)
		po, ok2 := pipes[i].(*pipeOffset)
		if !ok1 || !ok2 {
			i++
			continue
		}

		if ps.limit > 0 && po.offset >= ps.limit {
			pipes[i-1] = &pipeLimit{}
			pipes = append(pipes[:i], pipes[i+1:]...)
			continue
		}

		ps.offset += po.offset
		if ps.limit > 0 {
			ps.limit -= po.offset
		}
		pipes = append(pipes[:i], pipes[i+1:]...)
	}
	return pipes
}

func optimizeSortLimitPipes(pipes []pipe) []pipe {
	// Merge 'sort ... | limit ...' into 'sort ... limit ...'
	i := 1
	for i < len(pipes) {
		ps, ok1 := pipes[i-1].(*pipeSort)
		pl, ok2 := pipes[i].(*pipeLimit)
		if !ok1 || !ok2 {
			i++
			continue
		}

		if pl.limit == 0 {
			pipes = append(pipes[:i-1], pipes[i:]...)
			continue
		}
		if ps.limit == 0 || pl.limit < ps.limit {
			ps.limit = pl.limit
		}
		pipes = append(pipes[:i], pipes[i+1:]...)
	}
	return pipes
}

func optimizeUniqLimitPipes(pipes []pipe) []pipe {
	// Merge 'uniq ... | limit ...' into 'uniq ... limit ...'
	i := 1
	for i < len(pipes) {
		pl, ok := pipes[i].(*pipeLimit)
		if !ok {
			i++
			continue
		}
		pu, ok := pipes[i-1].(*pipeUniq)
		if !ok {
			i++
			continue
		}
		if pu.limit == 0 || pl.limit < pu.limit {
			pu.limit = pl.limit
		}
		pipes = append(pipes[:i], pipes[i+1:]...)
	}
	return pipes
}

func optimizeFilterPipes(pipes []pipe) []pipe {
	// Merge multiple `| filter ...` pipes into a single `filter ...` pipe
	i := 1
	for i < len(pipes) {
		pf1, ok := pipes[i-1].(*pipeFilter)
		if !ok {
			i++
			continue
		}
		pf2, ok := pipes[i].(*pipeFilter)
		if !ok {
			i++
			continue
		}

		pf1.f = mergeFiltersAnd(pf1.f, pf2.f)
		pipes = append(pipes[:i], pipes[i+1:]...)
	}
	return pipes
}

func mergeFiltersAnd(f1, f2 filter) filter {
	fa1, ok := f1.(*filterAnd)
	if ok {
		fa1.filters = append(fa1.filters, f2)
		return fa1
	}

	fa2, ok := f2.(*filterAnd)
	if ok {
		filters := make([]filter, len(fa2.filters)+1)
		filters[0] = f1
		copy(filters[1:], fa2.filters)
		fa2.filters = filters
		return fa2
	}

	return &filterAnd{
		filters: []filter{f1, f2},
	}
}

func getNeededColumns(pipes []pipe) *prefixfilter.Filter {
	var pf prefixfilter.Filter
	pf.AddAllowFilter("*")

	for i := len(pipes) - 1; i >= 0; i-- {
		pipes[i].updateNeededFields(&pf)
	}

	return &pf
}

// ParseQuery parses s.
func ParseQuery(s string) (*Query, error) {
	timestamp := time.Now().UnixNano()
	return ParseQueryAtTimestamp(s, timestamp)
}

// ParseStatsQuery parses LogsQL query s at the given timestamp with the needed stats query checks.
func ParseStatsQuery(s string, timestamp int64) (*Query, error) {
	q, err := ParseQueryAtTimestamp(s, timestamp)
	if err != nil {
		return nil, err
	}
	if _, err := q.GetStatsByFields(); err != nil {
		return nil, err
	}
	return q, nil
}

// HasGlobalTimeFilter returns true when query contains a global time filter.
func (q *Query) HasGlobalTimeFilter() bool {
	start, end := q.GetFilterTimeRange()
	return start != math.MinInt64 && end != math.MaxInt64
}

// ParseQueryAtTimestamp parses s in the context of the given timestamp.
//
// E.g. _time:duration filters are adjusted according to the provided timestamp as _time:[timestamp-duration, duration].
func ParseQueryAtTimestamp(s string, timestamp int64) (*Query, error) {
	lex := newLexer(s, timestamp)

	q, err := parseQuery(lex)
	if err != nil {
		return nil, err
	}
	if !lex.isEnd() {
		return nil, fmt.Errorf("unexpected unparsed tail after [%s]; context: [%s]; tail: [%s]", q, lex.context(), lex.rawToken+lex.s)
	}
	q.optimize()
	q.initStatsRateFuncsFromTimeFilter()

	return q, nil
}

func (q *Query) initStatsRateFuncsFromTimeFilter() {
	start, end := q.GetFilterTimeRange()
	if start != math.MinInt64 && end != math.MaxInt64 {
		step := end - start

		// The increment of the step is needed in order to cover the
		// last nanosecond in the selected time range [start, end].
		step++

		q.initStatsRateFuncs(step)
	}
}

func (q *Query) initStatsRateFuncs(step int64) {
	for _, p := range q.pipes {
		if ps, ok := p.(*pipeStats); ok {
			ps.initRateFuncs(step)
		}
	}
}

func (q *Query) addByTimeFieldToStatsPipes(step int64) {
	for _, p := range q.pipes {
		if ps, ok := p.(*pipeStats); ok {
			ps.addByTimeField(step)
		}
	}
}

func (q *Query) addPartitionByTime(step int64) {
	for _, p := range q.pipes {
		switch t := p.(type) {
		case *pipeFirst:
			t.addPartitionByTime(step)
		case *pipeLast:
			t.addPartitionByTime(step)
		case *pipeSort:
			t.addPartitionByTime(step)
		}
	}
}

// GetTimestamp returns timestamp context for the given q, which was passed to ParseQueryAtTimestamp().
func (q *Query) GetTimestamp() int64 {
	return q.timestamp
}

func parseQueryInParens(lex *lexer) (*Query, error) {
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing '('")
	}
	lex.nextToken()

	q, err := parseQuery(lex)
	if err != nil {
		return nil, err
	}

	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("missing ')' after '(%s'", q)
	}
	lex.nextToken()

	return q, nil
}

func parseQuery(lex *lexer) (*Query, error) {
	var q Query
	if err := parseQueryOptions(&q.opts, lex); err != nil {
		return nil, fmt.Errorf("cannot parse query options: %w; context: [%s]; see https://docs.victoriametrics.com/victorialogs/logsql/#query-options", err, lex.context())
	}
	lex.pushQueryOptions(&q.opts)
	defer lex.popQueryOptions()

	f, err := parseFilter(lex, true)
	if err != nil {
		return nil, fmt.Errorf("%w; context: [%s]", err, lex.context())
	}

	q.f = updateFilterWithTimeOffset(f, q.opts.timeOffset)
	q.timestamp = lex.currentTimestamp

	if lex.isKeyword("|") {
		lex.nextToken()
		pipes, err := parsePipes(lex)
		if err != nil {
			return nil, fmt.Errorf("%w; context: [%s]", err, lex.context())
		}
		q.pipes = pipes
	}

	return &q, nil
}

// Filter represents LogsQL filter
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#filters
type Filter struct {
	f filter
}

// String returns string representation of f.
func (f *Filter) String() string {
	if f == nil || f.f == nil {
		return ""
	}
	return f.f.String()
}

// ParseFilter parses LogsQL filter
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#filters
func ParseFilter(s string) (*Filter, error) {
	q, err := ParseQuery(s)
	if err != nil {
		return nil, err
	}
	if len(q.pipes) > 0 {
		return nil, fmt.Errorf("unexpected pipes after the filter [%s]; pipes: %s", q.f, q.pipes)
	}
	f := &Filter{
		f: q.f,
	}
	return f, nil
}

// parseQueryOptions parses option(...) from lex into dstOpts.
func parseQueryOptions(dstOpts *queryOptions, lex *lexer) error {
	defaultOpts := lex.getQueryOptions()
	if defaultOpts != nil {
		*dstOpts = *defaultOpts
	}
	dstOpts.needPrint = false

	if !lex.isKeyword("options") {
		return nil
	}
	lex.nextToken()

	if !lex.isKeyword("(") {
		return fmt.Errorf("missing '(' after 'options' keyword; wrap 'options' into quotes if you are searching for this word in the log message")
	}
	lex.nextToken()

	for {
		if lex.isKeyword(")") {
			lex.nextToken()
			return nil
		}

		k, v, err := parseKeyValuePair(lex)
		if err != nil {
			return fmt.Errorf("cannot parse 'options': %w", err)
		}
		switch k {
		case "concurrency":
			n, ok := tryParseUint64(v)
			if !ok {
				return fmt.Errorf("cannot parse 'concurrency=%q' option as unsigned integer", v)
			}
			dstOpts.concurrency = uint(n)
			dstOpts.needPrint = true
		case "parallel_readers":
			n, ok := tryParseUint64(v)
			if !ok {
				return fmt.Errorf("cannot parse 'parallel_readers=%q' option as unsigned integer", v)
			}
			dstOpts.parallelReaders = uint(n)
			dstOpts.needPrint = true
		case "ignore_global_time_filter":
			ignoreGlobalTimeFilter, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("cannot parse 'ignore_global_time_filter=%q' option as boolean: %w", v, err)
			}
			dstOpts.ignoreGlobalTimeFilter = &ignoreGlobalTimeFilter
			dstOpts.needPrint = true
		case "allow_partial_response":
			allowPartialResponse, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("cannot parse 'allow_partial_response=%q' option as boolean: %w", v, err)
			}
			dstOpts.allowPartialResponse = &allowPartialResponse
			dstOpts.needPrint = true
		case "time_offset":
			timeOffset, ok := tryParseDuration(v)
			if !ok {
				return fmt.Errorf("cannot parse 'time_offset=%q' option as duration", v)
			}
			dstOpts.timeOffset = timeOffset
			dstOpts.timeOffsetStr = v
			dstOpts.needPrint = true
		default:
			return fmt.Errorf("unexpected option %q with value %q", k, v)
		}

		if lex.isKeyword(")") {
			lex.nextToken()
			return nil
		}
		if !lex.isKeyword(",") {
			return fmt.Errorf("unexpected token inside the 'options(...)': %q; want ',' or ')'", lex.token)
		}
		lex.nextToken()
	}
}

func parseKeyValuePair(lex *lexer) (string, string, error) {
	k, err := lex.nextCompoundToken()
	if err != nil {
		return "", "", fmt.Errorf("cannot read key in the 'key=value' pair: %w", err)
	}

	if !lex.isKeyword("=") {
		return "", "", fmt.Errorf("missing '=' after %q key; got %q instead", k, lex.token)
	}
	lex.nextToken()

	v, err := lex.nextCompoundToken()
	if err != nil {
		return "", "", fmt.Errorf("cannot read value after '%q=': %w", k, err)
	}

	return k, v, nil
}

func parseFilter(lex *lexer, allowPipeKeywords bool) (filter, error) {
	if lex.isKeyword("|", ")", "") {
		return nil, fmt.Errorf("missing query")
	}

	if !allowPipeKeywords {
		// Verify the first token in the filter doesn't match pipe names.
		firstToken := strings.ToLower(lex.rawToken)
		if firstToken == "by" || isPipeName(firstToken) || isStatsFuncName(firstToken) {
			return nil, fmt.Errorf("query filter cannot start with pipe keyword %q; see https://docs.victoriametrics.com/victorialogs/logsql/#query-syntax; "+
				"please put the first word of the filter into quotes", firstToken)
		}
	}

	fo, err := parseFilterOr(lex, "")
	if err != nil {
		return nil, err
	}
	return fo, nil
}

func parseFilterOr(lex *lexer, fieldName string) (filter, error) {
	var filters []filter
	for {
		f, err := parseFilterAnd(lex, fieldName)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
		switch {
		case lex.isKeyword("|", ")", ""):
			if len(filters) == 1 {
				return filters[0], nil
			}
			fo := &filterOr{
				filters: filters,
			}
			return fo, nil
		case lex.isKeyword("or"):
			lex.nextToken()
		}
	}
}

func parseFilterAnd(lex *lexer, fieldName string) (filter, error) {
	var filters []filter
	for {
		f, err := parseFilterGeneric(lex, fieldName)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
		switch {
		case lex.isKeyword("or", "|", ")", ""):
			if len(filters) == 1 {
				return filters[0], nil
			}
			fa := &filterAnd{
				filters: filters,
			}
			return fa, nil
		case lex.isKeyword("and"):
			lex.nextToken()
		}
	}
}

func parseFilterGeneric(lex *lexer, fieldName string) (filter, error) {
	// Verify the previous adjacent token
	if lex.isKeyword("(") {
		if err := lex.checkPrevAdjacentToken("|", ":", "(", "!", "-", "not", "and", "or"); err != nil {
			return nil, err
		}
	} else {
		if err := lex.checkPrevAdjacentToken("|", ":", "(", "!", "-"); err != nil {
			return nil, err
		}
	}

	// Detect the filter.
	switch {
	case lex.isKeyword("{"):
		return parseFilterStream(lex, fieldName)
	case lex.isKeyword("*"):
		return parseFilterStar(lex, fieldName)
	case lex.isKeyword("("):
		return parseFilterParens(lex, fieldName)
	case lex.isKeyword(">"):
		return parseFilterGT(lex, fieldName)
	case lex.isKeyword("<"):
		return parseFilterLT(lex, fieldName)
	case lex.isKeyword("="):
		return parseFilterEQ(lex, fieldName)
	case lex.isKeyword("!="):
		return parseFilterNEQ(lex, fieldName)
	case lex.isKeyword("~"):
		return parseFilterTilda(lex, fieldName)
	case lex.isKeyword("!~"):
		return parseFilterNotTilda(lex, fieldName)
	case lex.isKeyword("not", "!", "-"):
		return parseFilterNot(lex, fieldName)
	case lex.isKeyword("contains_all"):
		return parseFilterContainsAll(lex, fieldName)
	case lex.isKeyword("contains_any"):
		return parseFilterContainsAny(lex, fieldName)
	case lex.isKeyword("contains_common_case"):
		return parseFilterContainsCommonCase(lex, fieldName)
	case lex.isKeyword("eq_field"):
		return parseFilterEqField(lex, fieldName)
	case lex.isKeyword("equals_common_case"):
		return parseFilterEqualsCommonCase(lex, fieldName)
	case lex.isKeyword("exact"):
		return parseFilterExact(lex, fieldName)
	case lex.isKeyword("i"):
		return parseAnyCaseFilter(lex, fieldName)
	case lex.isKeyword("in"):
		return parseFilterIn(lex, fieldName)
	case lex.isKeyword("ipv4_range"):
		return parseFilterIPv4Range(lex, fieldName)
	case lex.isKeyword("le_field"):
		return parseFilterLeField(lex, fieldName)
	case lex.isKeyword("len_range"):
		return parseFilterLenRange(lex, fieldName)
	case lex.isKeyword("lt_field"):
		return parseFilterLtField(lex, fieldName)
	case lex.isKeyword("pattern_match"):
		return parseFilterPatternMatch(lex, fieldName)
	case lex.isKeyword("pattern_match_full"):
		return parseFilterPatternMatch(lex, fieldName)
	case lex.isKeyword("range"):
		return parseFilterRange(lex, fieldName)
	case lex.isKeyword("re"):
		return parseFilterRegexp(lex, fieldName)
	case lex.isKeyword("seq"):
		return parseFilterSequence(lex, fieldName)
	case lex.isKeyword("string_range"):
		return parseFilterStringRange(lex, fieldName)
	case lex.isKeyword("value_type"):
		return parseFilterValueType(lex, fieldName)
	case lex.isKeyword("_time"):
		return parseFilterTimeGeneric(lex, fieldName)
	case lex.isKeyword("_stream_id"):
		return parseFilterStreamID(lex, fieldName)
	case lex.isKeyword("_stream"):
		if fieldName != "" {
			return parseFilterPhrase(lex, fieldName)
		}
		lexState := lex.backupState()
		lex.nextToken()

		if !lex.isKeyword(":") {
			lex.restoreState(lexState)
			return parseFilterPhrase(lex, "")
		}
		lex.nextToken()

		return parseFilterStream(lex, "_stream")
	default:
		return parseFilterPhrase(lex, fieldName)
	}
}

func parseFilterPhrase(lex *lexer, fieldName string) (filter, error) {
	var stopTokens []string
	if fieldName == "" {
		stopTokens = []string{":"}
	}
	phrase, err := lex.nextCompoundTokenExt(stopTokens)
	if err != nil {
		return nil, err
	}

	if fieldName == "" && lex.isKeyword(":") {
		// The phrase contains a field name for the filter
		lex.nextToken()
		return parseFilterGeneric(lex, phrase)
	}

	// The phrase is either a search phrase or a search prefix.
	if !lex.isSkippedSpace && lex.isKeyword("*") {
		// The phrase is a search prefix in the form `foo*`.
		lex.nextToken()
		f := &filterPrefix{
			fieldName: getCanonicalColumnName(fieldName),
			prefix:    phrase,
		}
		return f, nil
	}

	// The phrase is a search phrase.
	f := &filterPhrase{
		fieldName: getCanonicalColumnName(fieldName),
		phrase:    phrase,
	}
	return f, nil
}

func parseFilterParens(lex *lexer, fieldName string) (filter, error) {
	lex.nextToken()

	f, err := parseFilterOr(lex, fieldName)
	if err != nil {
		return nil, err
	}

	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("missing ')'; got %q", lex.token)
	}
	lex.nextToken()

	return f, nil
}

func parseFilterNot(lex *lexer, fieldName string) (filter, error) {
	lex.nextToken()

	f, err := parseFilterGeneric(lex, fieldName)
	if err != nil {
		return nil, err
	}
	fn, ok := f.(*filterNot)
	if ok {
		return fn.f, nil
	}
	fn = &filterNot{
		f: f,
	}
	return fn, nil
}

func parseAnyCaseFilter(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgMaybePrefix(lex, fieldName, func(phrase string, isFilterPrefix bool) (filter, error) {
		if isFilterPrefix {
			f := &filterAnyCasePrefix{
				fieldName: getCanonicalColumnName(fieldName),
				prefix:    phrase,
			}
			return f, nil
		}
		f := &filterAnyCasePhrase{
			fieldName: getCanonicalColumnName(fieldName),
			phrase:    phrase,
		}
		return f, nil
	})
}

func parseFuncArgMaybePrefix(lex *lexer, fieldName string, callback func(arg string, isPrefiFilter bool) (filter, error)) (filter, error) {
	lexState := lex.backupState()

	funcName, err := lex.nextCompoundToken()
	if err != nil {
		return nil, err
	}

	if !lex.isKeyword("(") {
		lex.restoreState(lexState)
		return parseFilterPhrase(lex, fieldName)
	}
	lex.nextToken()

	arg := ""
	isWildcard := lex.isKeyword("*")
	if isWildcard {
		lex.nextToken()
	} else {
		token, err := lex.nextCompoundToken()
		if err != nil {
			return nil, fmt.Errorf("cannot read %s() arg: %w", funcName, err)
		}
		arg = token
		if !lex.isSkippedSpace && lex.isKeyword("*") {
			lex.nextToken()
			isWildcard = true
		}
	}

	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("missing ')' for %s; got %q", funcName, lex.token)
	}
	lex.nextToken()

	return callback(arg, isWildcard)
}

func parseFilterLenRange(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgs(lex, fieldName, func(funcName string, args []string) (filter, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("unexpected number of args for %s(); got %d; want 2", funcName, len(args))
		}

		minLen, err := parseUint(args[0])
		if err != nil {
			return nil, fmt.Errorf("cannot parse minLen at %s(): %w", funcName, err)
		}

		maxLen, err := parseUint(args[1])
		if err != nil {
			return nil, fmt.Errorf("cannot parse maxLen at %s(): %w", funcName, err)
		}

		// There is no need in quoting the args, since they are integers.
		stringRepr := fmt.Sprintf("(%s, %s)", args[0], args[1])

		fr := &filterLenRange{
			fieldName: getCanonicalColumnName(fieldName),
			minLen:    minLen,
			maxLen:    maxLen,

			stringRepr: stringRepr,
		}
		return fr, nil
	})
}

func parseFilterStringRange(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgs(lex, fieldName, func(funcName string, args []string) (filter, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("unexpected number of args for %s(); got %d; want 2", funcName, len(args))
		}
		fr := &filterStringRange{
			fieldName: getCanonicalColumnName(fieldName),
			minValue:  args[0],
			maxValue:  args[1],

			stringRepr: fmt.Sprintf("%s(%s, %s)", funcName, quoteTokenIfNeeded(args[0]), quoteTokenIfNeeded(args[1])),
		}
		return fr, nil
	})
}

func parseFilterValueType(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArg(lex, fieldName, func(arg string) (filter, error) {
		fv := &filterValueType{
			fieldName: getCanonicalColumnName(fieldName),
			valueType: arg,
		}
		return fv, nil
	})
}

func parseFilterIPv4Range(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgs(lex, fieldName, func(funcName string, args []string) (filter, error) {
		if len(args) == 1 {
			minValue, maxValue, ok := tryParseIPv4CIDR(args[0])
			if !ok {
				return nil, fmt.Errorf("cannot parse IPv4 address or IPv4 CIDR %q at %s()", args[0], funcName)
			}
			fr := &filterIPv4Range{
				fieldName: getCanonicalColumnName(fieldName),
				minValue:  minValue,
				maxValue:  maxValue,
			}
			return fr, nil
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("unexpected number of args for %s(); got %d; want 2", funcName, len(args))
		}
		minValue, ok := tryParseIPv4(args[0])
		if !ok {
			return nil, fmt.Errorf("cannot parse lower bound ip %q in %s()", funcName, args[0])
		}
		maxValue, ok := tryParseIPv4(args[1])
		if !ok {
			return nil, fmt.Errorf("cannot parse upper bound ip %q in %s()", funcName, args[1])
		}
		fr := &filterIPv4Range{
			fieldName: getCanonicalColumnName(fieldName),
			minValue:  minValue,
			maxValue:  maxValue,
		}
		return fr, nil
	})
}

func tryParseIPv4CIDR(s string) (uint32, uint32, bool) {
	n := strings.IndexByte(s, '/')
	if n < 0 {
		n, ok := tryParseIPv4(s)
		return n, n, ok
	}
	ip, ok := tryParseIPv4(s[:n])
	if !ok {
		return 0, 0, false
	}
	maskBits, ok := tryParseUint64(s[n+1:])
	if !ok || maskBits > 32 {
		return 0, 0, false
	}
	mask := uint32((1 << (32 - maskBits)) - 1)
	minValue := ip &^ mask
	maxValue := ip | mask
	return minValue, maxValue, true
}

func parseFilterContainsAll(lex *lexer, fieldName string) (filter, error) {
	fi := &filterContainsAll{
		fieldName: getCanonicalColumnName(fieldName),
	}
	return parseInValues(lex, fieldName, fi, &fi.values)
}

func parseFilterContainsAny(lex *lexer, fieldName string) (filter, error) {
	fi := &filterContainsAny{
		fieldName: getCanonicalColumnName(fieldName),
	}
	return parseInValues(lex, fieldName, fi, &fi.values)
}

func parseFilterIn(lex *lexer, fieldName string) (filter, error) {
	fi := &filterIn{
		fieldName: getCanonicalColumnName(fieldName),
	}
	return parseInValues(lex, fieldName, fi, &fi.values)
}

func parseFilterContainsCommonCase(lex *lexer, fieldName string) (filter, error) {
	lex.nextToken()

	phrases, err := parseArgsInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'contains_common_case(...)' args: %w", err)
	}

	fi, err := newFilterContainsCommonCase(fieldName, phrases)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'contains_common_case(...)': %w", err)
	}
	return fi, nil
}

func parseFilterEqualsCommonCase(lex *lexer, fieldName string) (filter, error) {
	lex.nextToken()

	phrases, err := parseArgsInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'equals_common_case(...)' args: %w", err)
	}

	fi, err := newFilterEqualsCommonCase(fieldName, phrases)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'equals_common_case(...)': %w", err)
	}
	return fi, nil
}

func parseInValues(lex *lexer, fieldName string, f filter, iv *inValues) (filter, error) {
	// Try parsing in(arg1, ..., argN) at first
	lexState := lex.backupState()
	fi, err := parseFuncArgsPossibleWildcard(lex, fieldName, func(args []string) (filter, error) {
		iv.values = args
		return f, nil
	})
	if err == nil {
		return fi, nil
	}
	errFirst := err
	stateFirst := lex.backupState()

	// Parse in(query | fields someField) then
	lex.restoreState(lexState)
	lex.nextToken()

	q, qFieldName, err := parseInQuery(lex)
	if err != nil {
		// Return the previous error from parsing in(arg1, ..., argN) for simpler debugging.
		lex.restoreState(stateFirst)
		return nil, errFirst
	}
	if q == nil {
		return &filterNoop{}, nil
	}

	iv.q = q
	iv.qFieldName = qFieldName
	return f, nil
}

func parseFilterSequence(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgs(lex, fieldName, func(_ string, args []string) (filter, error) {
		fs := &filterSequence{
			fieldName: getCanonicalColumnName(fieldName),
			phrases:   args,
		}
		return fs, nil
	})
}

func parseFilterEqField(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArg(lex, fieldName, func(arg string) (filter, error) {
		fe := &filterEqField{
			fieldName:      getCanonicalColumnName(fieldName),
			otherFieldName: arg,
		}
		return fe, nil
	})
}

func parseFilterLeField(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArg(lex, fieldName, func(arg string) (filter, error) {
		fe := &filterLeField{
			fieldName:      getCanonicalColumnName(fieldName),
			otherFieldName: arg,
		}
		return fe, nil
	})
}

func parseFilterLtField(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArg(lex, fieldName, func(arg string) (filter, error) {
		fe := &filterLeField{
			fieldName:      getCanonicalColumnName(fieldName),
			otherFieldName: arg,

			excludeEqualValues: true,
		}
		return fe, nil
	})
}

func parseFilterExact(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgMaybePrefix(lex, fieldName, func(phrase string, isFilterPrefix bool) (filter, error) {
		if isFilterPrefix {
			f := &filterExactPrefix{
				fieldName: getCanonicalColumnName(fieldName),
				prefix:    phrase,
			}
			return f, nil
		}
		f := &filterExact{
			fieldName: getCanonicalColumnName(fieldName),
			value:     phrase,
		}
		return f, nil
	})
}

func parseFilterPatternMatch(lex *lexer, fieldName string) (filter, error) {
	isFull := lex.isKeyword("pattern_match_full")
	return parseFuncArg(lex, fieldName, func(arg string) (filter, error) {
		fp := &filterPatternMatch{
			fieldName: getCanonicalColumnName(fieldName),
			pm:        newPatternMatcher(arg, isFull),
		}
		return fp, nil
	})
}

func parseFilterRegexp(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArg(lex, fieldName, func(arg string) (filter, error) {
		return newFilterRegexp(fieldName, arg)
	})
}

func newFilterRegexp(fieldName, arg string) (filter, error) {
	// Optimizations for typical regexps generated by Grafana
	if arg == "" || arg == ".*" {
		return &filterNoop{}, nil
	}
	if arg == ".+" {
		fp := &filterPrefix{
			fieldName: getCanonicalColumnName(fieldName),
		}
		return fp, nil
	}

	re, err := regexutil.NewRegex(arg)
	if err != nil {
		return nil, fmt.Errorf("invalid regexp %q:%q: %w", getCanonicalColumnName(fieldName), arg, err)
	}
	fr := &filterRegexp{
		fieldName: getCanonicalColumnName(fieldName),
		re:        re,
	}
	return fr, nil
}

func parseFilterStar(lex *lexer, fieldName string) (filter, error) {
	lex.nextToken()

	if lex.isSkippedSpace || lex.isKeyword("", ")", "|") {
		// '*' or 'fieldName:*' filter
		f := &filterPrefix{
			fieldName: getCanonicalColumnName(fieldName),
			prefix:    "",
		}
		return f, nil
	}

	// Read '*substr*' filter
	phrase, err := lex.nextCompoundToken()
	if err != nil {
		return nil, fmt.Errorf("cannot read *substr* filter: %w", err)
	}
	if lex.isSkippedSpace || !lex.isKeyword("*") {
		return nil, fmt.Errorf("missing ending '*' in the *%q* filter", phrase)
	}
	lex.nextToken()

	if !lex.isSkippedSpace && !lex.isKeyword("", ")", "|") {
		return nil, fmt.Errorf("missing whitespace between *%q* and %q", phrase, lex.token)
	}
	f := &filterSubstring{
		fieldName: getCanonicalColumnName(fieldName),
		substring: phrase,
	}
	return f, nil
}

func parseFilterTilda(lex *lexer, fieldName string) (filter, error) {
	op := lex.token
	lex.nextToken()

	if lex.isKeyword("-") {
		return nil, fmt.Errorf("regexp, which start with %q, must be put in quotes", lex.token)
	}

	if lex.isSkippedSpace && fieldName == "" {
		// Deny invalid filters `foo ~ bar` and `foo !~ bar`. They must be written as `foo:~bar` and `foo:!~bar`
		return nil, fmt.Errorf("missing ':' in front of %q; see https://docs.victoriametrics.com/victorialogs/logsql/#filters", op)
	}

	arg, err := lex.nextCompoundToken()
	if err != nil {
		return nil, fmt.Errorf("cannot read regexp for field %q: %w", getCanonicalColumnName(fieldName), err)
	}

	return newFilterRegexp(fieldName, arg)
}

func parseFilterNotTilda(lex *lexer, fieldName string) (filter, error) {
	f, err := parseFilterTilda(lex, fieldName)
	if err != nil {
		return nil, err
	}
	fn := &filterNot{
		f: f,
	}
	return fn, nil
}

func parseFilterEQ(lex *lexer, fieldName string) (filter, error) {
	op := lex.token
	lex.nextToken()
	if lex.isSkippedSpace && fieldName == "" {
		// Deny invalid filters 'foo = bar`. It must be written as `foo:=bar`
		return nil, fmt.Errorf("missing ':' in front of %q; see https://docs.victoriametrics.com/victorialogs/logsql/#filters", op)
	}

	phrase, err := lex.nextCompoundToken()
	if err != nil {
		return nil, fmt.Errorf("cannot parse token after %q: %w", op, err)
	}

	if !lex.isSkippedSpace && lex.isKeyword("*") {
		lex.nextToken()
		f := &filterExactPrefix{
			fieldName: getCanonicalColumnName(fieldName),
			prefix:    phrase,
		}
		return f, nil
	}
	f := &filterExact{
		fieldName: getCanonicalColumnName(fieldName),
		value:     phrase,
	}
	return f, nil
}

func parseFilterNEQ(lex *lexer, fieldName string) (filter, error) {
	f, err := parseFilterEQ(lex, fieldName)
	if err != nil {
		return nil, err
	}
	fn := &filterNot{
		f: f,
	}
	return fn, nil
}

func parseFilterGT(lex *lexer, fieldName string) (filter, error) {
	lex.nextToken()

	includeMinValue := false
	op := ">"
	if !lex.isSkippedSpace && lex.isKeyword("=") {
		lex.nextToken()
		includeMinValue = true
		op = ">="
	}

	if lex.isSkippedSpace && fieldName == "" {
		// Deny invalid filters 'foo > bar' and 'foo >= bar'. They must be written as 'foo:>bar` and `foo:>=bar`
		return nil, fmt.Errorf("missing ':' in front of %q; see https://docs.victoriametrics.com/victorialogs/logsql/#filters", op)
	}

	lexState := lex.backupState()
	minValue, fStr, err := parseNumber(lex)
	if err != nil {
		lex.restoreState(lexState)
		fr := tryParseFilterGTString(lex, fieldName, op, includeMinValue)
		if fr == nil {
			return nil, fmt.Errorf("cannot parse [%s] as number: %w", fStr, err)
		}
		return fr, nil
	}

	if !includeMinValue {
		minValue = nextafter(minValue, inf)
	}
	fr := &filterRange{
		fieldName: getCanonicalColumnName(fieldName),
		minValue:  minValue,
		maxValue:  inf,

		stringRepr: op + fStr,
	}
	return fr, nil
}

func parseFilterLT(lex *lexer, fieldName string) (filter, error) {
	lex.nextToken()

	includeMaxValue := false
	op := "<"
	if !lex.isSkippedSpace && lex.isKeyword("=") {
		lex.nextToken()
		includeMaxValue = true
		op = "<="
	}

	if lex.isSkippedSpace && fieldName == "" {
		// Deny invalid filters 'foo < bar' and 'foo <= bar'. They must be written as 'foo:<bar' and 'foo:<=bar'
		return nil, fmt.Errorf("missing ':' in front of %q; see https://docs.victoriametrics.com/victorialogs/logsql/#filters", op)
	}

	lexState := lex.backupState()
	maxValue, fStr, err := parseNumber(lex)
	if err != nil {
		lex.restoreState(lexState)
		fr := tryParseFilterLTString(lex, fieldName, op, includeMaxValue)
		if fr == nil {
			return nil, fmt.Errorf("cannot parse [%s] as number: %w", fStr, err)
		}
		return fr, nil
	}

	if !includeMaxValue {
		maxValue = nextafter(maxValue, -inf)
	}
	fr := &filterRange{
		fieldName: getCanonicalColumnName(fieldName),
		minValue:  -inf,
		maxValue:  maxValue,

		stringRepr: op + fStr,
	}
	return fr, nil
}

func tryParseFilterGTString(lex *lexer, fieldName, op string, includeMinValue bool) filter {
	minValueOrig, err := lex.nextCompoundToken()
	if err != nil {
		return nil
	}
	minValue := minValueOrig
	if !includeMinValue {
		minValue = string(append([]byte(minValue), 0))
	}
	fr := &filterStringRange{
		fieldName: getCanonicalColumnName(fieldName),
		minValue:  minValue,
		maxValue:  maxStringRangeValue,

		stringRepr: op + quoteStringTokenIfNeeded(minValueOrig),
	}
	return fr
}

func tryParseFilterLTString(lex *lexer, fieldName, op string, includeMaxValue bool) filter {
	maxValueOrig, err := lex.nextCompoundToken()
	if err != nil {
		return nil
	}
	maxValue := maxValueOrig
	if includeMaxValue {
		maxValue = string(append([]byte(maxValue), 0))
	}
	fr := &filterStringRange{
		fieldName: getCanonicalColumnName(fieldName),
		maxValue:  maxValue,

		stringRepr: op + quoteStringTokenIfNeeded(maxValueOrig),
	}
	return fr
}

func parseFilterRange(lex *lexer, fieldName string) (filter, error) {
	lexState := lex.backupState()

	funcName, err := lex.nextCompoundToken()
	if err != nil {
		return nil, err
	}

	// Parse minValue
	includeMinValue := false
	switch {
	case lex.isKeyword("("):
		includeMinValue = false
	case lex.isKeyword("["):
		includeMinValue = true
	default:
		lex.restoreState(lexState)
		return parseFilterPhrase(lex, fieldName)
	}
	lex.nextToken()

	minValue, minValueStr, err := parseNumber(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse minValue in %s(): %w", funcName, err)
	}

	// Parse comma
	if !lex.isKeyword(",") {
		return nil, fmt.Errorf("unexpected token %q ater %q in %s(); want ','", lex.token, minValueStr, funcName)
	}
	lex.nextToken()

	// Parse maxValue
	maxValue, maxValueStr, err := parseNumber(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse maxValue in %s(): %w", funcName, err)
	}
	includeMaxValue := false
	switch {
	case lex.isKeyword(")"):
		includeMaxValue = false
	case lex.isKeyword("]"):
		includeMaxValue = true
	default:
		return nil, fmt.Errorf("unexpected closing token %q in %s(); want ')' or ']'", lex.token, funcName)
	}
	lex.nextToken()

	stringRepr := "range"
	if includeMinValue {
		stringRepr += "["
	} else {
		stringRepr += "("
		minValue = nextafter(minValue, inf)
	}
	stringRepr += minValueStr + ", " + maxValueStr
	if includeMaxValue {
		stringRepr += "]"
	} else {
		stringRepr += ")"
		maxValue = nextafter(maxValue, -inf)
	}

	fr := &filterRange{
		fieldName: getCanonicalColumnName(fieldName),
		minValue:  minValue,
		maxValue:  maxValue,

		stringRepr: stringRepr,
	}
	return fr, nil
}

func parseNumber(lex *lexer) (float64, string, error) {
	s, err := lex.nextCompoundToken()
	if err != nil {
		return 0, "", fmt.Errorf("cannot read number: %w", err)
	}

	f := parseMathNumber(s)
	if !math.IsNaN(f) || strings.EqualFold(s, "nan") {
		return f, s, nil
	}

	return 0, s, fmt.Errorf("cannot parse %q as float64", s)
}

func parseFuncArg(lex *lexer, fieldName string, callback func(arg string) (filter, error)) (filter, error) {
	return parseFuncArgs(lex, fieldName, func(funcName string, args []string) (filter, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("unexpected number of args for %s(); got %d; want 1", funcName, len(args))
		}
		return callback(args[0])
	})
}

func parseFuncArgs(lex *lexer, fieldName string, callback func(funcName string, args []string) (filter, error)) (filter, error) {
	lexState := lex.backupState()

	funcName, err := lex.nextCompoundToken()
	if err != nil {
		return nil, err
	}

	if !lex.isKeyword("(") {
		lex.restoreState(lexState)
		return parseFilterPhrase(lex, fieldName)
	}

	args, err := parseArgsInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s(): %w", funcName, err)
	}

	return callback(funcName, args)
}

func parseFuncArgsPossibleWildcard(lex *lexer, fieldName string, callback func(args []string) (filter, error)) (filter, error) {
	lexState := lex.backupState()

	funcName, err := lex.nextCompoundToken()
	if err != nil {
		return nil, err
	}

	if !lex.isKeyword("(") {
		lex.restoreState(lexState)
		return parseFilterPhrase(lex, fieldName)
	}

	args, isWildcard, err := parseArgsInParensPossibleWildcard(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s(): %w", funcName, err)
	}
	if isWildcard {
		return &filterNoop{}, nil
	}

	return callback(args)
}

func parseArgsInParens(lex *lexer) ([]string, error) {
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing '('")
	}
	lex.nextToken()

	var args []string
	for !lex.isKeyword(")") {
		if lex.isKeyword(",") {
			return nil, fmt.Errorf("unexpected ','")
		}
		if lex.isKeyword("(") {
			return nil, fmt.Errorf("unexpected '('")
		}
		arg, err := lex.nextCompoundToken()
		if err != nil {
			return nil, fmt.Errorf("cannot parse arg")
		}
		args = append(args, arg)
		if lex.isKeyword(")") {
			break
		}
		if !lex.isKeyword(",") {
			return nil, fmt.Errorf("missing ',' after %q; got %q instead", arg, lex.token)
		}
		lex.nextToken()
	}
	lex.nextToken()
	return args, nil
}

func parseArgsInParensPossibleWildcard(lex *lexer) ([]string, bool, error) {
	if !lex.isKeyword("(") {
		return nil, false, fmt.Errorf("missing '('")
	}
	lex.nextToken()

	var args []string
	isWildcard := false
	for !lex.isKeyword(")") {
		if lex.isKeyword(",") {
			return nil, false, fmt.Errorf("unexpected ','")
		}
		if lex.isKeyword("(") {
			return nil, false, fmt.Errorf("unexpected '('")
		}
		arg := ""
		if lex.isKeyword("*") {
			lex.nextToken()
			isWildcard = true
			arg = "*"
		} else {
			token, err := lex.nextCompoundToken()
			if err != nil {
				return nil, false, fmt.Errorf("cannot parse arg")
			}
			arg = token
		}
		args = append(args, arg)
		if lex.isKeyword(")") {
			break
		}
		if !lex.isKeyword(",") {
			return nil, false, fmt.Errorf("missing ',' after %q; got %q instead", arg, lex.token)
		}
		lex.nextToken()
	}
	lex.nextToken()

	if isWildcard {
		return nil, isWildcard, nil
	}
	return args, false, nil
}

// startsWithYear returns true if s starts with YYYY
func startsWithYear(s string) bool {
	if len(s) < 4 {
		return false
	}
	for i := 0; i < 4; i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
	}
	s = s[4:]
	if len(s) == 0 {
		return true
	}
	c := s[0]
	return c == '-' || c == '+' || c == 'Z' || c == 'z'
}

func parseFilterTimeGeneric(lex *lexer, fieldName string) (filter, error) {
	if fieldName != "" {
		return parseFilterPhrase(lex, fieldName)
	}

	lexState := lex.backupState()
	lex.nextToken()

	if !lex.isKeyword(":") {
		lex.restoreState(lexState)
		return parseFilterPhrase(lex, "")
	}
	lex.nextToken()

	switch {
	case lex.isKeyword("day_range"):
		return parseFilterDayRange(lex)
	case lex.isKeyword("week_range"):
		return parseFilterWeekRange(lex)
	default:
		return parseFilterTimeRange(lex)
	}
}

func parseFilterDayRange(lex *lexer) (*filterDayRange, error) {
	if !lex.isKeyword("day_range") {
		return nil, fmt.Errorf("unexpected token %q; want 'day_range'", lex.token)
	}
	lex.nextToken()

	startBrace := "["
	switch {
	case lex.isKeyword("["):
		lex.nextToken()
	case lex.isKeyword("("):
		lex.nextToken()
		startBrace = "("
	default:
		return nil, fmt.Errorf("missing '[' or '(' at day_range filter")
	}

	start, startStr, err := getDayRangeArg(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot read `start` arg at day_range filter: %w", err)
	}

	if !lex.isKeyword(",") {
		return nil, fmt.Errorf("unexpected token %q; want ','", lex.token)
	}
	lex.nextToken()

	end, endStr, err := getDayRangeArg(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot read `end` arg at day_range filter: %w", err)
	}

	endBrace := "]"
	switch {
	case lex.isKeyword("]"):
		lex.nextToken()
	case lex.isKeyword(")"):
		lex.nextToken()
		endBrace = ")"
	default:
		return nil, fmt.Errorf("missing ']' or ')' after day_range filter")
	}

	offset := int64(0)
	offsetStr := ""
	if lex.isKeyword("offset") {
		lex.nextToken()
		d, s, err := parseDuration(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse offset in day_range filter: %w", err)
		}
		offset = d
		offsetStr = " offset " + s
	}

	if startBrace == "(" {
		start++
	}
	if endBrace == ")" {
		end--
	}

	fr := &filterDayRange{
		start:  start,
		end:    end,
		offset: offset,

		stringRepr: fmt.Sprintf("%s%s, %s%s%s", startBrace, startStr, endStr, endBrace, offsetStr),
	}
	return fr, nil
}

func parseFilterWeekRange(lex *lexer) (*filterWeekRange, error) {
	if !lex.isKeyword("week_range") {
		return nil, fmt.Errorf("unexpected token %q; want 'week_range'", lex.token)
	}
	lex.nextToken()

	startBrace := "["
	switch {
	case lex.isKeyword("["):
		lex.nextToken()
	case lex.isKeyword("("):
		lex.nextToken()
		startBrace = "("
	default:
		return nil, fmt.Errorf("missing '[' or '(' at week_range filter")
	}

	startDay, startStr, err := getWeekRangeArg(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot read `start` arg at week_range filter: %w", err)
	}

	if !lex.isKeyword(",") {
		return nil, fmt.Errorf("unexpected token %q; want ','", lex.token)
	}
	lex.nextToken()

	endDay, endStr, err := getWeekRangeArg(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot read `end` arg at week_range filter: %w", err)
	}

	endBrace := "]"
	switch {
	case lex.isKeyword("]"):
		lex.nextToken()
	case lex.isKeyword(")"):
		lex.nextToken()
		endBrace = ")"
	default:
		return nil, fmt.Errorf("missing ']' or ')' after week_range filter")
	}

	offset := int64(0)
	offsetStr := ""
	if lex.isKeyword("offset") {
		lex.nextToken()
		d, s, err := parseDuration(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse offset in week_range filter: %w", err)
		}
		offset = d
		offsetStr = " offset " + s
	}

	if startBrace == "(" {
		startDay++
	}
	if endBrace == ")" {
		endDay--
	}

	fr := &filterWeekRange{
		startDay: startDay,
		endDay:   endDay,
		offset:   offset,

		stringRepr: fmt.Sprintf("%s%s, %s%s%s", startBrace, startStr, endStr, endBrace, offsetStr),
	}
	return fr, nil
}

func getDayRangeArg(lex *lexer) (int64, string, error) {
	argStr, err := lex.nextCompoundToken()
	if err != nil {
		return 0, "", err
	}
	offset, ok := tryParseHHMM(argStr)
	if !ok {
		return 0, "", fmt.Errorf("cannot parse %q as 'hh:mm'", argStr)
	}
	if offset >= nsecsPerDay {
		offset = nsecsPerDay - 1
	}
	return offset, argStr, nil
}

func getWeekRangeArg(lex *lexer) (time.Weekday, string, error) {
	argStr, err := lex.nextCompoundToken()
	if err != nil {
		return 0, "", err
	}

	var day time.Weekday
	switch strings.ToLower(argStr) {
	case "sun", "sunday":
		day = time.Sunday
	case "mon", "monday":
		day = time.Monday
	case "tue", "tuesday":
		day = time.Tuesday
	case "wed", "wednesday":
		day = time.Wednesday
	case "thu", "thursday":
		day = time.Thursday
	case "fri", "friday":
		day = time.Friday
	case "sat", "saturday":
		day = time.Saturday
	}

	return day, argStr, nil
}

func parseFilterTimeRange(lex *lexer) (*filterTime, error) {
	if lex.isKeyword("offset") {
		ft := &filterTime{
			minTimestamp: math.MinInt64,
			maxTimestamp: lex.currentTimestamp,
		}
		offset, offsetStr, err := parseTimeOffset(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse offset for _time filter []: %w", err)
		}
		ft.maxTimestamp = subNoOverflowInt64(ft.maxTimestamp, offset)
		ft.stringRepr = offsetStr
		return ft, nil
	}

	ft, err := parseFilterTime(lex)
	if err != nil {
		return nil, err
	}
	if !lex.isKeyword("offset") {
		return ft, nil
	}

	offset, offsetStr, err := parseTimeOffset(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse offset for _time filter [%s]: %w", ft, err)
	}
	ft.minTimestamp = subNoOverflowInt64(ft.minTimestamp, offset)
	ft.maxTimestamp = subNoOverflowInt64(ft.maxTimestamp, offset)
	ft.stringRepr += " " + offsetStr
	return ft, nil
}

func parseTimeOffset(lex *lexer) (int64, string, error) {
	if !lex.isKeyword("offset") {
		return 0, "", fmt.Errorf("unexpected token %q; want 'offset'", lex.token)
	}
	lex.nextToken()

	d, s, err := parseDuration(lex)
	if err != nil {
		return 0, "", fmt.Errorf("cannot parse duration: %w", err)
	}
	offset := d
	return offset, "offset " + s, nil
}

func parseFilterTime(lex *lexer) (*filterTime, error) {
	startTimeInclude := false
	switch {
	case lex.isKeyword(">"):
		return parseFilterTimeGt(lex)
	case lex.isKeyword("<"):
		return parseFilterTimeLt(lex)
	case lex.isKeyword("["):
		lex.nextToken()
		startTimeInclude = true
	case lex.isKeyword("("):
		lex.nextToken()
		startTimeInclude = false
	default:
		return parseFilterTimeEq(lex)
	}

	// Parse start time
	startTime, startTimeString, err := parseTime(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse start time in _time filter: %w", err)
	}
	if !lex.isKeyword(",") {
		return nil, fmt.Errorf("unexpected token after start time in _time filter: %q; want ','", lex.token)
	}
	lex.nextToken()

	// Parse end time
	endTime, endTimeString, err := parseTime(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse end time in _time filter: %w", err)
	}

	endTimeInclude := false
	switch {
	case lex.isKeyword("]"):
		endTimeInclude = true
	case lex.isKeyword(")"):
		endTimeInclude = false
	default:
		return nil, fmt.Errorf("_time filter ends with unexpected token %q; it must end with ']' or ')'", lex.token)
	}
	lex.nextToken()

	stringRepr := ""
	if startTimeInclude {
		stringRepr += "["
	} else {
		stringRepr += "("
		startTime++
	}
	stringRepr += startTimeString + "," + endTimeString
	if endTimeInclude {
		stringRepr += "]"
		endTime = adjustEndTimestamp(endTime, endTimeString)
	} else {
		stringRepr += ")"
		endTime--
	}

	ft := &filterTime{
		minTimestamp: startTime,
		maxTimestamp: endTime,

		stringRepr: stringRepr,
	}
	return ft, nil
}

func parseFilterTimeGt(lex *lexer) (*filterTime, error) {
	if !lex.isKeyword(">") {
		return nil, fmt.Errorf("missing '>' in _time filter; got %q instead", lex.token)
	}
	lex.nextToken()

	prefix := ">"
	if lex.isKeyword("=") {
		lex.nextToken()
		prefix = ">="
	}

	if isLikelyTimestamp(lex) {
		startTime, startTimeString, err := parseTime(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse start time in _time filter: %w", err)
		}

		if prefix == ">" {
			startTime++
		}
		ft := &filterTime{
			minTimestamp: startTime,
			maxTimestamp: math.MaxInt64,

			stringRepr: prefix + startTimeString,
		}
		return ft, nil
	}

	d, s, err := parseDuration(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse duration at _time filter: %w", err)
	}
	if d < 0 {
		d = -d
	}
	if prefix == ">" {
		d++
	}
	ft := &filterTime{
		minTimestamp: math.MinInt64,
		maxTimestamp: subNoOverflowInt64(lex.currentTimestamp, d),

		stringRepr: prefix + s,
	}
	return ft, nil
}

func parseFilterTimeLt(lex *lexer) (*filterTime, error) {
	if !lex.isKeyword("<") {
		return nil, fmt.Errorf("missing '<' in _time filter; got %q instead", lex.token)
	}
	lex.nextToken()

	prefix := "<"
	if lex.isKeyword("=") {
		lex.nextToken()
		prefix = "<="
	}

	if isLikelyTimestamp(lex) {
		endTime, endTimeString, err := parseTime(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse end time in _time filter: %w", err)
		}
		if prefix == "<" {
			endTime--
		} else {
			endTime = adjustEndTimestamp(endTime, endTimeString)
		}
		ft := &filterTime{
			minTimestamp: math.MinInt64,
			maxTimestamp: endTime,

			stringRepr: prefix + endTimeString,
		}
		return ft, nil
	}

	d, s, err := parseDuration(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse duration at _time filter: %w", err)
	}
	if d < 0 {
		d = -d
	}
	if prefix == "<" {
		d--
	}
	ft := &filterTime{
		minTimestamp: subNoOverflowInt64(lex.currentTimestamp, d),
		maxTimestamp: lex.currentTimestamp,

		stringRepr: prefix + s,
	}
	return ft, nil
}

func parseFilterTimeEq(lex *lexer) (*filterTime, error) {
	prefix := ""
	if lex.isKeyword("=") {
		lex.nextToken()
		prefix = "="
	}

	if isLikelyTimestamp(lex) {
		// Parse '_time:YYYY-MM-DD', which transforms to '_time:[YYYY-MM-DD, YYYY-MM-DD+1)'
		nsecs, s, err := parseTime(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse _time filter: %w", err)
		}
		// Round to milliseconds
		startTime := nsecs
		endTime := adjustEndTimestamp(startTime, s)
		ft := &filterTime{
			minTimestamp: startTime,
			maxTimestamp: endTime,

			stringRepr: prefix + s,
		}
		return ft, nil
	}

	// Parse _time:duration, which transforms to '_time:(now-duration, now]'
	d, s, err := parseDuration(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse duration at _time filter: %w", err)
	}
	if d < 0 {
		d = -d
	}
	ft := &filterTime{
		minTimestamp: subNoOverflowInt64(lex.currentTimestamp, d),
		maxTimestamp: lex.currentTimestamp,

		stringRepr: prefix + s,
	}
	return ft, nil
}

func isLikelyTimestamp(lex *lexer) bool {
	return lex.isKeyword("now") || startsWithYear(lex.token)
}

// adjustEndTimestamp returns an adjusted timestamp for t according to its' string representation tStr.
//
// The t is adjusted for the interval [start, tStr] depending on the tStr value. Examples:
//
// - If tStr='2025', then the full year is added to t, so it points to the last nanosecond of the 2025 year.
// - If tStr='2025-05-20', then the full day is added to t, so it points to the last nanosecond of the 2025-05-20.
func adjustEndTimestamp(t int64, tStr string) int64 {
	tStart := time.Unix(0, t).UTC()
	tEnd := tStart

	tStr = stripTimezoneSuffix(tStr)

	if len(tStr) == len("YYYY") {
		// tStr contains only a year, such as "2025"
		y, m, d := tStart.Date()
		nsec := t % (24 * 3600 * 1e9)
		tEnd = time.Date(y+1, m, d, 0, 0, int(nsec/1e9), int(nsec%1e9), time.UTC)
		return tEnd.UnixNano() - 1
	}

	// Check if this is a Unix timestamp (all digits)
	if isAllDigits(tStr) {
		switch {
		case len(tStr) <= 10:
			tEnd = tStart.Add(time.Second)
		case len(tStr) <= 13:
			tEnd = tStart.Add(time.Millisecond)
		case len(tStr) <= 16:
			tEnd = tStart.Add(time.Microsecond)
		default:
			tEnd = tStart.Add(time.Nanosecond)
		}
		return tEnd.UnixNano() - 1
	}

	if len(tStr) <= len("YYYY") || tStr[len("YYYY")] != '-' {
		n := strings.IndexByte(tStr, '.')
		if n < 0 || !isAllDigits(tStr[:n]) || !isAllDigits(tStr[n+1:]) {
			// Unknown tStr format
			return tEnd.UnixNano()
		}
		// Fractional seconds unix timestamp format.
		switch len(tStr[n+1:]) {
		case 3:
			tEnd = tStart.Add(time.Millisecond)
		case 6:
			tEnd = tStart.Add(time.Microsecond)
		default:
			tEnd = tStart.Add(time.Nanosecond)
		}
		return tEnd.UnixNano() - 1
	}

	// RFC3339 timestamp handling
	switch {
	case len(tStr) == len("YYYY-MM"):
		y, m, d := tStart.Date()
		nsec := t % (24 * 3600 * 1e9)
		if d != 1 {
			d = 0
			m++
		}
		tEnd = time.Date(y, m+1, d, 0, 0, int(nsec/1e9), int(nsec%1e9), time.UTC)
	case len(tStr) == len("YYYY-MM-DD"):
		tEnd = tStart.Add(24 * time.Hour)
	case len(tStr) == len("YYYY-MM-DDThh"):
		tEnd = tStart.Add(time.Hour)
	case len(tStr) == len("YYYY-MM-DDThh:mm"):
		tEnd = tStart.Add(time.Minute)
	case len(tStr) == len("YYYY-MM-DDThh:mm:ss"):
		tEnd = tStart.Add(time.Second)
	case len(tStr) == len("YYYY-MM-DDThh:mm:ss.SSS"):
		tEnd = tStart.Add(time.Millisecond)
	case len(tStr) == len("YYYY-MM-DDThh:mm:ss.SSSSSS"):
		tEnd = tStart.Add(time.Microsecond)
	default:
		tEnd = tStart.Add(time.Nanosecond)
	}
	return tEnd.UnixNano() - 1
}

func isAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func stripTimezoneSuffix(s string) string {
	if strings.HasSuffix(s, "Z") {
		return s[:len(s)-1]
	}
	if len(s) < 6 {
		return s
	}
	tz := s[len(s)-6:]
	if tz[0] != '-' && tz[0] != '+' {
		return s
	}
	if tz[3] != ':' {
		return s
	}
	return s[:len(s)-len(tz)]
}

func parseFilterStreamID(lex *lexer, fieldName string) (filter, error) {
	if fieldName != "" {
		return parseFilterPhrase(lex, fieldName)
	}

	lexState := lex.backupState()
	lex.nextToken()

	if !lex.isKeyword(":") {
		lex.restoreState(lexState)
		return parseFilterPhrase(lex, "")
	}
	lex.nextToken()

	if lex.isKeyword("in") {
		return parseFilterStreamIDIn(lex)
	}

	sid, err := parseStreamID(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse _stream_id: %w", err)
	}
	fs := &filterStreamID{
		streamIDs: []streamID{sid},
	}
	return fs, nil
}

func parseFilterStreamIDIn(lex *lexer) (filter, error) {
	if !lex.isKeyword("in") {
		return nil, fmt.Errorf("unexpected token %q; expecting 'in'", lex.token)
	}

	// Try parsing in(arg1, ..., argN) at first
	lexState := lex.backupState()
	fs, err := parseFuncArgsPossibleWildcard(lex, "_stream_id", func(args []string) (filter, error) {
		streamIDs := make([]streamID, len(args))
		for i, arg := range args {
			if !streamIDs[i].tryUnmarshalFromString(arg) {
				return nil, fmt.Errorf("cannot unmarshal _stream_id from %q", arg)
			}
		}
		fs := &filterStreamID{
			streamIDs: streamIDs,
		}
		return fs, nil
	})
	if err == nil {
		return fs, nil
	}
	errFirst := err
	stateFirst := lex.backupState()

	// Try parsing in(query)
	lex.restoreState(lexState)
	lex.nextToken()

	q, qFieldName, err := parseInQuery(lex)
	if err != nil {
		// Return the previous error from parsing in(arg1, ..., argN) for simpler debugging.
		lex.restoreState(stateFirst)
		return nil, errFirst
	}
	if q == nil {
		return &filterNoop{}, nil
	}

	fs = &filterStreamID{
		q:          q,
		qFieldName: qFieldName,
	}
	return fs, nil
}

func parseInQuery(lex *lexer) (*Query, string, error) {
	q, err := parseQueryInParens(lex)
	if err != nil {
		return nil, "", fmt.Errorf("cannot parse in(...) query: %w", err)
	}
	if q.isStarQuery() {
		return nil, "", nil
	}
	qFieldName, err := getFieldNameFromPipes(q.pipes)
	if err != nil {
		return nil, "", fmt.Errorf("cannot determine field name for values in 'in(%s)': %w", q, err)
	}
	return q, qFieldName, nil
}

func (q *Query) isStarQuery() bool {
	if len(q.pipes) > 0 {
		return false
	}
	switch t := q.f.(type) {
	case *filterNoop:
		return true
	case *filterPrefix:
		return len(t.prefix) == 0
	default:
		return false
	}
}

func getFieldNameFromPipes(pipes []pipe) (string, error) {
	if len(pipes) == 0 {
		return "", fmt.Errorf("missing 'fields' or 'uniq' pipes at the end of query")
	}
	switch t := pipes[len(pipes)-1].(type) {
	case *pipeFields:
		if !isSingleField(t.fieldFilters) {
			return "", fmt.Errorf("'%s' pipe must contain only a single field name", t)
		}
		return t.fieldFilters[0], nil
	case *pipeUniq:
		if len(t.byFields) != 1 {
			return "", fmt.Errorf("'%s' pipe must contain only a single non-star field name", t)
		}
		return t.byFields[0], nil
	default:
		return "", fmt.Errorf("missing 'fields' or 'uniq' pipe at the end of query")
	}
}

func parseStreamID(lex *lexer) (streamID, error) {
	var sid streamID

	s, err := lex.nextCompoundToken()
	if err != nil {
		return sid, err
	}

	if !sid.tryUnmarshalFromString(s) {
		return sid, fmt.Errorf("cannot unmarshal _stream_id from %q", s)
	}
	return sid, nil
}

func parseFilterStream(lex *lexer, fieldName string) (*filterStream, error) {
	if fieldName != "" && fieldName != "_stream" {
		return nil, fmt.Errorf("stream filter cannot be applied to %q field; it can be applied only to _stream field", fieldName)
	}

	sf, err := parseStreamFilter(lex)
	if err != nil {
		return nil, err
	}
	fs := &filterStream{
		f: sf,
	}
	return fs, nil
}

func parseTime(lex *lexer) (int64, string, error) {
	s, err := lex.nextCompoundToken()
	if err != nil {
		return 0, "", err
	}
	nsecs, err := timeutil.ParseTimeAt(s, lex.currentTimestamp)
	if err != nil {
		return 0, "", err
	}
	return nsecs, s, nil
}

func parseDuration(lex *lexer) (int64, string, error) {
	s, err := lex.nextCompoundToken()
	if err != nil {
		return 0, "", err
	}
	d, ok := tryParseDuration(s)
	if !ok {
		return 0, s, fmt.Errorf("cannot parse duration %q", s)
	}
	return d, s, nil
}

func quoteStringTokenIfNeeded(s string) string {
	if !needQuoteStringToken(s) {
		return s
	}
	return strconv.Quote(s)
}

func quoteFieldFilterIfNeeded(s string) string {
	if !prefixfilter.IsWildcardFilter(s) {
		return quoteTokenIfNeeded(s)
	}

	wildcard := s[:len(s)-1]
	if wildcard == "" || !needQuoteToken(wildcard) {
		return s
	}
	return strconv.Quote(s)
}

func quoteTokenIfNeeded(s string) string {
	if !needQuoteToken(s) {
		return s
	}
	return strconv.Quote(s)
}

func needQuoteStringToken(s string) bool {
	return isNumberPrefix(s) || needQuoteToken(s)
}

func isNumberPrefix(s string) bool {
	if len(s) == 0 {
		return false
	}
	if s[0] == '-' || s[0] == '+' {
		s = s[1:]
		if len(s) == 0 {
			return false
		}
	}
	if len(s) >= 3 && strings.EqualFold(s, "inf") {
		return true
	}
	if s[0] == '.' {
		s = s[1:]
		if len(s) == 0 {
			return false
		}
	}
	return s[0] >= '0' && s[0] <= '9'
}

func needQuoteToken(s string) bool {
	sLower := strings.ToLower(s)
	if _, ok := reservedKeywords[sLower]; ok {
		return true
	}
	if isPipeName(sLower) || isStatsFuncName(sLower) {
		return true
	}
	for _, r := range s {
		if !isTokenRune(r) && r != '.' {
			return true
		}
	}
	return false
}

var reservedKeywords = func() map[string]struct{} {
	kws := []string{
		// An empty keyword means end of parsed string
		"",

		// boolean operator tokens for 'foo and bar or baz not xxx'
		"and",
		"or",
		"not",
		"!", // synonym for "not"

		// parens for '(foo or bar) and baz'
		"(",
		")",

		// stream filter tokens for '_stream:{foo=~"bar", baz="a"}'
		"{",
		"}",
		"=",
		"!=",
		"=~",
		"!~",
		",",

		// delimiter between query parts:
		// 'foo and bar | extract "<*> foo <time>" | filter x:y | ...'
		"|",

		// delimiter between field name and query in filter: 'foo:bar'
		":",

		// prefix search: 'foo*'
		"*",

		// keywords for _time filter: '_time:(now-1h, now]'
		"[",
		"]",
		"now",
		"offset",
		"-",

		// functions
		"contains_all",
		"contains_any",
		"contains_common_case",
		"eq_field",
		"equals_common_case",
		"exact",
		"i",
		"in",
		"ipv4_range",
		"le_field",
		"len_range",
		"lt_field",
		"pattern_match",
		"pattern_match_full",
		"range",
		"re",
		"seq",
		"string_range",
		"value_type",

		// queryOptions start with this keyword
		"options",

		// 'if' is used in conditional pipes such as `format if (...) ...`
		"if",

		// 'by' is used in various pipes such as `stats by (...) ...`
		"by",

		// 'as' is used in various pipes such as `format ... as ...`
		"as",
	}
	m := make(map[string]struct{}, len(kws))
	for _, kw := range kws {
		m[kw] = struct{}{}
	}
	return m
}()

func parseUint(s string) (uint64, error) {
	if strings.EqualFold(s, "inf") || strings.EqualFold(s, "+inf") {
		return math.MaxUint64, nil
	}

	n, err := strconv.ParseUint(s, 0, 64)
	if err == nil {
		return n, nil
	}
	nn, ok := tryParseBytes(s)
	if !ok {
		nn, ok = tryParseDuration(s)
		if !ok {
			return 0, fmt.Errorf("cannot parse %q as unsigned integer: %w", s, err)
		}
	}
	if nn < 0 {
		return 0, fmt.Errorf("cannot parse negative value %q as unsigned integer", s)
	}
	return uint64(nn), nil
}

func nextafter(f, xInf float64) float64 {
	if math.IsInf(f, 0) {
		return f
	}
	return math.Nextafter(f, xInf)
}

func toFieldsFilters(pf *prefixfilter.Filter) string {
	if pf.MatchNothing() {
		return " | delete *"
	}
	if pf.MatchAll() {
		return ""
	}

	qStr := ""

	denyFilters := pf.GetDenyFilters()
	if len(denyFilters) > 0 {
		qStr += " | delete " + fieldNamesString(denyFilters)
	}

	allowFilters := pf.GetAllowFilters()
	if len(allowFilters) > 0 && !prefixfilter.MatchAll(allowFilters) {
		qStr += " | fields " + fieldNamesString(allowFilters)
	}

	return qStr
}

func subNoOverflowInt64(a, b int64) int64 {
	if a == math.MinInt64 || a == math.MaxInt64 {
		// Assume that a is either +Inf or -Inf.
		// Subtracting any number from Inf must result in Inf.
		return a
	}
	if b >= 0 {
		if a < math.MinInt64+b {
			return math.MinInt64
		}
		return a - b
	}
	if a > math.MaxInt64+b {
		return math.MaxInt64
	}
	return a - b
}
