package logstorage

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
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

	// prevToken contains the previously parsed token
	prevToken string

	// isSkippedSpace is set to true if there was a whitespace before the token in s
	isSkippedSpace bool

	// currentTimestamp is the current timestamp in nanoseconds
	currentTimestamp int64
}

type lexerState struct {
	lex lexer
}

func (lex *lexer) backupState() *lexerState {
	return &lexerState{
		lex: *lex,
	}
}

func (lex *lexer) restoreState(ls *lexerState) {
	*lex = ls.lex
}

// newLexer returns new lexer for the given s.
//
// The lex.token points to the first token in s.
func newLexer(s string) *lexer {
	lex := &lexer{
		s:                s,
		sOrig:            s,
		currentTimestamp: time.Now().UnixNano(),
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

func (lex *lexer) isNumber() bool {
	s := lex.rawToken + lex.s
	return isNumberPrefix(s)
}

func (lex *lexer) isPrevToken(tokens ...string) bool {
	for _, token := range tokens {
		if token == lex.prevToken {
			return true
		}
	}
	return false
}

func (lex *lexer) isKeyword(keywords ...string) bool {
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

func (lex *lexer) mustNextToken() bool {
	lex.nextToken()
	return !lex.isEnd()
}

func (lex *lexer) nextCharToken(s string, size int) {
	lex.token = s[:size]
	lex.rawToken = lex.token
	lex.s = s[size:]
}

// nextToken updates lex.token to the next token.
func (lex *lexer) nextToken() {
	s := lex.s
	lex.prevToken = lex.token
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
	for isTokenRune(r) || r == '.' {
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
			size += len(s[size:]) - len(newTail)
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
	f filter

	pipes []pipe
}

// String returns string representation for q.
func (q *Query) String() string {
	s := q.f.String()

	for _, p := range q.pipes {
		s += " | " + p.String()
	}

	return s
}

// AddCountByTimePipe adds '| stats by (_time:step offset off, field1, ..., fieldN) count() hits' to the end of q.
func (q *Query) AddCountByTimePipe(step, off int64, fields []string) {
	{
		// add 'stats by (_time:step offset off, fields) count() hits'
		stepStr := string(marshalDurationString(nil, step))
		offsetStr := string(marshalDurationString(nil, off))
		byFieldsStr := "_time:" + stepStr + " offset " + offsetStr
		for _, f := range fields {
			byFieldsStr += ", " + quoteTokenIfNeeded(f)
		}
		s := fmt.Sprintf("stats by (%s) count() hits", byFieldsStr)
		lex := newLexer(s)

		ps, err := parsePipeStats(lex, true)
		if err != nil {
			logger.Panicf("BUG: unexpected error when parsing [%s]: %s", s, err)
		}
		if !lex.isEnd() {
			logger.Panicf("BUG: unexpected tail left after parsing [%s]: %q", s, lex.s)
		}

		q.pipes = append(q.pipes, ps)
	}

	{
		// Add 'sort by (_time, fields)' in order to get consistent order of the results.
		sortFieldsStr := "_time"
		for _, f := range fields {
			sortFieldsStr += ", " + quoteTokenIfNeeded(f)
		}
		s := fmt.Sprintf("sort by (%s)", sortFieldsStr)
		lex := newLexer(s)
		ps, err := parsePipeSort(lex)
		if err != nil {
			logger.Panicf("BUG: unexpected error when parsing %q: %s", s, err)
		}
		q.pipes = append(q.pipes, ps)
	}
}

// Clone returns a copy of q.
func (q *Query) Clone() *Query {
	qStr := q.String()
	qCopy, err := ParseQuery(qStr)
	if err != nil {
		logger.Panicf("BUG: cannot parse %q: %s", qStr, err)
	}
	return qCopy
}

// CanReturnLastNResults returns true if time range filter at q can be adjusted for returning the last N results.
func (q *Query) CanReturnLastNResults() bool {
	for _, p := range q.pipes {
		switch p.(type) {
		case *pipeFieldNames,
			*pipeFieldValues,
			*pipeLimit,
			*pipeOffset,
			*pipeSort,
			*pipeStats,
			*pipeUniq:
			return false
		}
	}
	return true
}

// GetFilterTimeRange returns filter time range for the given q.
func (q *Query) GetFilterTimeRange() (int64, int64) {
	return getFilterTimeRange(q.f)
}

// AddTimeFilter adds global filter _time:[start ... end] to q.
func (q *Query) AddTimeFilter(start, end int64) {
	startStr := marshalTimestampRFC3339NanoString(nil, start)
	endStr := marshalTimestampRFC3339NanoString(nil, end)
	ft := &filterTime{
		minTimestamp: start,
		maxTimestamp: end,
		stringRepr:   fmt.Sprintf("[%s, %s]", startStr, endStr),
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
}

// AddPipeLimit adds `| limit n` pipe to q.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe
func (q *Query) AddPipeLimit(n uint64) {
	q.pipes = append(q.pipes, &pipeLimit{
		limit: n,
	})
}

// Optimize tries optimizing the query.
func (q *Query) Optimize() {
	q.pipes = optimizeSortOffsetPipes(q.pipes)
	q.pipes = optimizeSortLimitPipes(q.pipes)
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

	// Substitute '*' prefixFilter with filterNoop in order to avoid reading _msg data.
	q.f = removeStarFilters(q.f)

	// Call Optimize for queries from 'in(query)' filters.
	optimizeFilterIn(q.f)

	// Optimize individual pipes.
	for _, p := range q.pipes {
		p.optimize()
	}
}

func removeStarFilters(f filter) filter {
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
		logger.Fatalf("BUG: unexpected error: %s", err)
	}
	return f
}

func optimizeSortOffsetPipes(pipes []pipe) []pipe {
	// Merge 'sort ... | offset ...' into 'sort ... offset ...'
	i := 1
	for i < len(pipes) {
		po, ok := pipes[i].(*pipeOffset)
		if !ok {
			i++
			continue
		}
		ps, ok := pipes[i-1].(*pipeSort)
		if !ok {
			i++
			continue
		}
		if ps.offset == 0 && ps.limit == 0 {
			ps.offset = po.offset
		}
		pipes = append(pipes[:i], pipes[i+1:]...)
	}
	return pipes
}

func optimizeSortLimitPipes(pipes []pipe) []pipe {
	// Merge 'sort ... | limit ...' into 'sort ... limit ...'
	i := 1
	for i < len(pipes) {
		pl, ok := pipes[i].(*pipeLimit)
		if !ok {
			i++
			continue
		}
		ps, ok := pipes[i-1].(*pipeSort)
		if !ok {
			i++
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

func (q *Query) getNeededColumns() ([]string, []string) {
	neededFields := newFieldsSet()
	neededFields.add("*")
	unneededFields := newFieldsSet()

	pipes := q.pipes
	for i := len(pipes) - 1; i >= 0; i-- {
		pipes[i].updateNeededFields(neededFields, unneededFields)
	}

	return neededFields.getAll(), unneededFields.getAll()
}

// ParseQuery parses s.
func ParseQuery(s string) (*Query, error) {
	lex := newLexer(s)
	q, err := parseQuery(lex)
	if err != nil {
		return nil, err
	}
	if !lex.isEnd() {
		return nil, fmt.Errorf("unexpected unparsed tail after [%s]; context: [%s]; tail: [%s]", q, lex.context(), lex.s)
	}
	return q, nil
}

func parseQuery(lex *lexer) (*Query, error) {
	f, err := parseFilter(lex)
	if err != nil {
		return nil, fmt.Errorf("%w; context: [%s]", err, lex.context())
	}
	q := &Query{
		f: f,
	}

	if lex.isKeyword("|") {
		lex.nextToken()
		pipes, err := parsePipes(lex)
		if err != nil {
			return nil, fmt.Errorf("%w; context: [%s]", err, lex.context())
		}
		q.pipes = pipes
	}

	return q, nil
}

func parseFilter(lex *lexer) (filter, error) {
	if lex.isKeyword("|", "") {
		return nil, fmt.Errorf("missing query")
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
			if !lex.mustNextToken() {
				return nil, fmt.Errorf("missing filter after 'or'")
			}
		}
	}
}

func parseFilterAnd(lex *lexer, fieldName string) (filter, error) {
	var filters []filter
	for {
		f, err := parseGenericFilter(lex, fieldName)
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
			if !lex.mustNextToken() {
				return nil, fmt.Errorf("missing filter after 'and'")
			}
		}
	}
}

func parseGenericFilter(lex *lexer, fieldName string) (filter, error) {
	// Check for special keywords
	switch {
	case lex.isKeyword(":"):
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing filter after ':'")
		}
		return parseGenericFilter(lex, fieldName)
	case lex.isKeyword("*"):
		lex.nextToken()
		f := &filterPrefix{
			fieldName: fieldName,
			prefix:    "",
		}
		return f, nil
	case lex.isKeyword("("):
		if !lex.isSkippedSpace && !lex.isPrevToken("", ":", "(", "!", "not") {
			return nil, fmt.Errorf("missing whitespace before the search word %q", lex.prevToken)
		}
		return parseParensFilter(lex, fieldName)
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
	case lex.isKeyword("not", "!"):
		return parseFilterNot(lex, fieldName)
	case lex.isKeyword("exact"):
		return parseFilterExact(lex, fieldName)
	case lex.isKeyword("i"):
		return parseAnyCaseFilter(lex, fieldName)
	case lex.isKeyword("in"):
		return parseFilterIn(lex, fieldName)
	case lex.isKeyword("ipv4_range"):
		return parseFilterIPv4Range(lex, fieldName)
	case lex.isKeyword("len_range"):
		return parseFilterLenRange(lex, fieldName)
	case lex.isKeyword("range"):
		return parseFilterRange(lex, fieldName)
	case lex.isKeyword("re"):
		return parseFilterRegexp(lex, fieldName)
	case lex.isKeyword("seq"):
		return parseFilterSequence(lex, fieldName)
	case lex.isKeyword("string_range"):
		return parseFilterStringRange(lex, fieldName)
	case lex.isKeyword(`"`, "'", "`"):
		return nil, fmt.Errorf("improperly quoted string")
	case lex.isKeyword(",", ")", "[", "]"):
		return nil, fmt.Errorf("unexpected token %q", lex.token)
	}
	phrase, err := getCompoundPhrase(lex, fieldName != "")
	if err != nil {
		return nil, err
	}
	return parseFilterForPhrase(lex, phrase, fieldName)
}

func getCompoundPhrase(lex *lexer, allowColon bool) (string, error) {
	stopTokens := []string{"*", ",", "(", ")", "[", "]", "|", ""}
	if lex.isKeyword(stopTokens...) {
		return "", fmt.Errorf("compound phrase cannot start with '%s'", lex.token)
	}

	phrase := lex.token
	rawPhrase := lex.rawToken
	lex.nextToken()
	suffix := getCompoundSuffix(lex, allowColon)
	if suffix == "" {
		return phrase, nil
	}
	return rawPhrase + suffix, nil
}

func getCompoundSuffix(lex *lexer, allowColon bool) string {
	s := ""
	stopTokens := []string{"*", ",", "(", ")", "[", "]", "|", ""}
	if !allowColon {
		stopTokens = append(stopTokens, ":")
	}
	for !lex.isSkippedSpace && !lex.isKeyword(stopTokens...) {
		s += lex.rawToken
		lex.nextToken()
	}
	return s
}

func getCompoundToken(lex *lexer) (string, error) {
	stopTokens := []string{",", "(", ")", "[", "]", "|", ""}
	if lex.isKeyword(stopTokens...) {
		return "", fmt.Errorf("compound token cannot start with '%s'", lex.token)
	}

	s := lex.token
	rawS := lex.rawToken
	lex.nextToken()
	suffix := ""
	for !lex.isSkippedSpace && !lex.isKeyword(stopTokens...) {
		s += lex.token
		lex.nextToken()
	}
	if suffix == "" {
		return s, nil
	}
	return rawS + suffix, nil
}

func getCompoundFuncArg(lex *lexer) string {
	if lex.isKeyword("*") {
		return ""
	}
	arg := lex.token
	rawArg := lex.rawToken
	lex.nextToken()
	suffix := ""
	for !lex.isSkippedSpace && !lex.isKeyword("*", ",", "(", ")", "|", "") {
		suffix += lex.rawToken
		lex.nextToken()
	}
	if suffix == "" {
		return arg
	}
	return rawArg + suffix
}

func parseFilterForPhrase(lex *lexer, phrase, fieldName string) (filter, error) {
	if fieldName != "" || !lex.isKeyword(":") {
		// The phrase is either a search phrase or a search prefix.
		if lex.isKeyword("*") && !lex.isSkippedSpace {
			// The phrase is a search prefix in the form `foo*`.
			lex.nextToken()
			f := &filterPrefix{
				fieldName: fieldName,
				prefix:    phrase,
			}
			return f, nil
		}
		// The phrase is a search phrase.
		f := &filterPhrase{
			fieldName: fieldName,
			phrase:    phrase,
		}
		return f, nil
	}

	// The phrase contains the field name.
	fieldName = phrase
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing filter after field name %s", quoteTokenIfNeeded(fieldName))
	}
	switch fieldName {
	case "_time":
		return parseFilterTimeWithOffset(lex)
	case "_stream":
		return parseFilterStream(lex)
	default:
		return parseGenericFilter(lex, fieldName)
	}
}

func parseParensFilter(lex *lexer, fieldName string) (filter, error) {
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing filter after '('")
	}
	f, err := parseFilterOr(lex, fieldName)
	if err != nil {
		return nil, err
	}
	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("unexpected token %q instead of ')'", lex.token)
	}
	lex.nextToken()
	return f, nil
}

func parseFilterNot(lex *lexer, fieldName string) (filter, error) {
	notKeyword := lex.token
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing filters after '%s'", notKeyword)
	}
	f, err := parseGenericFilter(lex, fieldName)
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
	return parseFuncArgMaybePrefix(lex, "i", fieldName, func(phrase string, isFilterPrefix bool) (filter, error) {
		if isFilterPrefix {
			f := &filterAnyCasePrefix{
				fieldName: fieldName,
				prefix:    phrase,
			}
			return f, nil
		}
		f := &filterAnyCasePhrase{
			fieldName: fieldName,
			phrase:    phrase,
		}
		return f, nil
	})
}

func parseFuncArgMaybePrefix(lex *lexer, funcName, fieldName string, callback func(arg string, isPrefiFilter bool) (filter, error)) (filter, error) {
	phrase := lex.token
	lex.nextToken()
	if !lex.isKeyword("(") {
		phrase += getCompoundSuffix(lex, fieldName != "")
		return parseFilterForPhrase(lex, phrase, fieldName)
	}
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing arg for %s()", funcName)
	}
	phrase = getCompoundFuncArg(lex)
	isFilterPrefix := false
	if lex.isKeyword("*") && !lex.isSkippedSpace {
		isFilterPrefix = true
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing ')' after %s()", funcName)
		}
	}
	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("unexpected token %q instead of ')' in %s()", lex.token, funcName)
	}
	lex.nextToken()
	return callback(phrase, isFilterPrefix)
}

func parseFilterLenRange(lex *lexer, fieldName string) (filter, error) {
	funcName := lex.token
	return parseFuncArgs(lex, fieldName, func(args []string) (filter, error) {
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

		stringRepr := "(" + args[0] + ", " + args[1] + ")"
		fr := &filterLenRange{
			fieldName: fieldName,
			minLen:    minLen,
			maxLen:    maxLen,

			stringRepr: stringRepr,
		}
		return fr, nil
	})
}

func parseFilterStringRange(lex *lexer, fieldName string) (filter, error) {
	funcName := lex.token
	return parseFuncArgs(lex, fieldName, func(args []string) (filter, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("unexpected number of args for %s(); got %d; want 2", funcName, len(args))
		}
		fr := &filterStringRange{
			fieldName: fieldName,
			minValue:  args[0],
			maxValue:  args[1],

			stringRepr: fmt.Sprintf("string_range(%s, %s)", quoteTokenIfNeeded(args[0]), quoteTokenIfNeeded(args[1])),
		}
		return fr, nil
	})
}

func parseFilterIPv4Range(lex *lexer, fieldName string) (filter, error) {
	funcName := lex.token
	return parseFuncArgs(lex, fieldName, func(args []string) (filter, error) {
		if len(args) == 1 {
			minValue, maxValue, ok := tryParseIPv4CIDR(args[0])
			if !ok {
				return nil, fmt.Errorf("cannot parse IPv4 address or IPv4 CIDR %q at %s()", args[0], funcName)
			}
			fr := &filterIPv4Range{
				fieldName: fieldName,
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
			fieldName: fieldName,
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

func parseFilterIn(lex *lexer, fieldName string) (filter, error) {
	if !lex.isKeyword("in") {
		return nil, fmt.Errorf("expecting 'in' keyword")
	}

	// Try parsing in(arg1, ..., argN) at first
	lexState := lex.backupState()
	fi, err := parseFuncArgs(lex, fieldName, func(args []string) (filter, error) {
		fi := &filterIn{
			fieldName: fieldName,
			values:    args,
		}
		return fi, nil
	})
	if err == nil {
		return fi, nil
	}

	// Parse in(query | fields someField) then
	lex.restoreState(lexState)
	lex.nextToken()
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing '(' after 'in'")
	}
	lex.nextToken()

	q, err := parseQuery(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse query inside 'in(...)': %w", err)
	}

	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("missing ')' after 'in(%s)'", q)
	}
	lex.nextToken()

	qFieldName, err := getFieldNameFromPipes(q.pipes)
	if err != nil {
		return nil, fmt.Errorf("cannot determine field name for values in 'in(%s)': %w", q, err)
	}
	fi = &filterIn{
		fieldName:        fieldName,
		needExecuteQuery: true,
		q:                q,
		qFieldName:       qFieldName,
	}
	return fi, nil
}

func getFieldNameFromPipes(pipes []pipe) (string, error) {
	if len(pipes) == 0 {
		return "", fmt.Errorf("missing 'fields' or 'uniq' pipes at the end of query")
	}
	switch t := pipes[len(pipes)-1].(type) {
	case *pipeFields:
		if t.containsStar || len(t.fields) != 1 {
			return "", fmt.Errorf("'%s' pipe must contain only a single non-star field name", t)
		}
		return t.fields[0], nil
	case *pipeUniq:
		if len(t.byFields) != 1 {
			return "", fmt.Errorf("'%s' pipe must contain only a single non-star field name", t)
		}
		return t.byFields[0], nil
	default:
		return "", fmt.Errorf("missing 'fields' or 'uniq' pipe at the end of query")
	}
}

func parseFilterSequence(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgs(lex, fieldName, func(args []string) (filter, error) {
		fs := &filterSequence{
			fieldName: fieldName,
			phrases:   args,
		}
		return fs, nil
	})
}

func parseFilterExact(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgMaybePrefix(lex, "exact", fieldName, func(phrase string, isFilterPrefix bool) (filter, error) {
		if isFilterPrefix {
			f := &filterExactPrefix{
				fieldName: fieldName,
				prefix:    phrase,
			}
			return f, nil
		}
		f := &filterExact{
			fieldName: fieldName,
			value:     phrase,
		}
		return f, nil
	})
}

func parseFilterRegexp(lex *lexer, fieldName string) (filter, error) {
	funcName := lex.token
	return parseFuncArg(lex, fieldName, func(arg string) (filter, error) {
		re, err := regexutil.NewRegex(arg)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp %q for %s(): %w", arg, funcName, err)
		}
		fr := &filterRegexp{
			fieldName: fieldName,
			re:        re,
		}
		return fr, nil
	})
}

func parseFilterTilda(lex *lexer, fieldName string) (filter, error) {
	lex.nextToken()
	arg := getCompoundFuncArg(lex)
	re, err := regexutil.NewRegex(arg)
	if err != nil {
		return nil, fmt.Errorf("invalid regexp %q: %w", arg, err)
	}
	fr := &filterRegexp{
		fieldName: fieldName,
		re:        re,
	}
	return fr, nil
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
	lex.nextToken()
	phrase := getCompoundFuncArg(lex)
	if lex.isKeyword("*") && !lex.isSkippedSpace {
		lex.nextToken()
		f := &filterExactPrefix{
			fieldName: fieldName,
			prefix:    phrase,
		}
		return f, nil
	}
	f := &filterExact{
		fieldName: fieldName,
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
	if lex.isKeyword("=") {
		lex.nextToken()
		includeMinValue = true
		op = ">="
	}

	if !lex.isNumber() {
		lexState := lex.backupState()
		fr := tryParseFilterGTString(lex, fieldName, op, includeMinValue)
		if fr != nil {
			return fr, nil
		}
		lex.restoreState(lexState)
	}

	minValue, fStr, err := parseFloat64(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse number after '%s': %w", op, err)
	}

	if !includeMinValue {
		minValue = nextafter(minValue, inf)
	}
	fr := &filterRange{
		fieldName: fieldName,
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
	if lex.isKeyword("=") {
		lex.nextToken()
		includeMaxValue = true
		op = "<="
	}

	if !lex.isNumber() {
		lexState := lex.backupState()
		fr := tryParseFilterLTString(lex, fieldName, op, includeMaxValue)
		if fr != nil {
			return fr, nil
		}
		lex.restoreState(lexState)
	}

	maxValue, fStr, err := parseFloat64(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse number after '%s': %w", op, err)
	}

	if !includeMaxValue {
		maxValue = nextafter(maxValue, -inf)
	}
	fr := &filterRange{
		fieldName: fieldName,
		minValue:  -inf,
		maxValue:  maxValue,

		stringRepr: op + fStr,
	}
	return fr, nil
}

func tryParseFilterGTString(lex *lexer, fieldName, op string, includeMinValue bool) filter {
	minValueOrig, err := getCompoundToken(lex)
	if err != nil {
		return nil
	}
	minValue := minValueOrig
	if !includeMinValue {
		minValue = string(append([]byte(minValue), 0))
	}
	fr := &filterStringRange{
		fieldName: fieldName,
		minValue:  minValue,
		maxValue:  maxStringRangeValue,

		stringRepr: op + quoteStringTokenIfNeeded(minValueOrig),
	}
	return fr
}

func tryParseFilterLTString(lex *lexer, fieldName, op string, includeMaxValue bool) filter {
	maxValueOrig, err := getCompoundToken(lex)
	if err != nil {
		return nil
	}
	maxValue := maxValueOrig
	if includeMaxValue {
		maxValue = string(append([]byte(maxValue), 0))
	}
	fr := &filterStringRange{
		fieldName: fieldName,
		maxValue:  maxValue,

		stringRepr: op + quoteStringTokenIfNeeded(maxValueOrig),
	}
	return fr
}

func parseFilterRange(lex *lexer, fieldName string) (filter, error) {
	funcName := lex.token
	lex.nextToken()

	// Parse minValue
	includeMinValue := false
	switch {
	case lex.isKeyword("("):
		includeMinValue = false
	case lex.isKeyword("["):
		includeMinValue = true
	default:
		phrase := funcName + getCompoundSuffix(lex, fieldName != "")
		return parseFilterForPhrase(lex, phrase, fieldName)
	}
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing args for %s()", funcName)
	}
	minValue, minValueStr, err := parseFloat64(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse minValue in %s(): %w", funcName, err)
	}

	// Parse comma
	if !lex.isKeyword(",") {
		return nil, fmt.Errorf("unexpected token %q ater %q in %s(); want ','", lex.token, minValueStr, funcName)
	}
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing maxValue in %s()", funcName)
	}

	// Parse maxValue
	maxValue, maxValueStr, err := parseFloat64(lex)
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
		fieldName: fieldName,
		minValue:  minValue,
		maxValue:  maxValue,

		stringRepr: stringRepr,
	}
	return fr, nil
}

func parseFloat64(lex *lexer) (float64, string, error) {
	s, err := getCompoundToken(lex)
	if err != nil {
		return 0, "", fmt.Errorf("cannot parse float64 from %q: %w", s, err)
	}
	f, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return f, s, nil
	}

	// Try parsing s as integer.
	// This handles 0x..., 0b... and 0... prefixes, alongside '_' delimiters.
	n, err := parseInt(s)
	if err == nil {
		return float64(n), s, nil
	}
	return 0, "", fmt.Errorf("cannot parse %q as float64: %w", s, err)
}

func parseFuncArg(lex *lexer, fieldName string, callback func(args string) (filter, error)) (filter, error) {
	funcName := lex.token
	return parseFuncArgs(lex, fieldName, func(args []string) (filter, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("unexpected number of args for %s(); got %d; want 1", funcName, len(args))
		}
		return callback(args[0])
	})
}

func parseFuncArgs(lex *lexer, fieldName string, callback func(args []string) (filter, error)) (filter, error) {
	funcName := lex.token
	lex.nextToken()
	if !lex.isKeyword("(") {
		phrase := funcName + getCompoundSuffix(lex, fieldName != "")
		return parseFilterForPhrase(lex, phrase, fieldName)
	}
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing args for %s()", funcName)
	}
	var args []string
	for !lex.isKeyword(")") {
		if lex.isKeyword(",") {
			return nil, fmt.Errorf("unexpected ',' - missing arg in %s()", funcName)
		}
		if lex.isKeyword("(") {
			return nil, fmt.Errorf("unexpected '(' - missing arg in %s()", funcName)
		}
		arg := getCompoundFuncArg(lex)
		args = append(args, arg)
		if lex.isKeyword(")") {
			break
		}
		if !lex.isKeyword(",") {
			return nil, fmt.Errorf("missing ',' after %q in %s()", arg, funcName)
		}
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing the next arg after %q in %s()", arg, funcName)
		}
	}
	lex.nextToken()

	return callback(args)
}

// startsWithYear returns true if s starts from YYYY
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

func parseFilterTimeWithOffset(lex *lexer) (*filterTime, error) {
	ft, err := parseFilterTime(lex)
	if err != nil {
		return nil, err
	}
	if !lex.isKeyword("offset") {
		return ft, nil
	}
	lex.nextToken()
	s, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse offset in _time filter: %w", err)
	}
	d, ok := tryParseDuration(s)
	if !ok {
		return nil, fmt.Errorf("cannot parse offset %q for _time filter %s", s, ft)
	}
	offset := int64(d)
	ft.minTimestamp -= offset
	ft.maxTimestamp -= offset
	ft.stringRepr += " offset " + s
	return ft, nil
}

func parseFilterTime(lex *lexer) (*filterTime, error) {
	startTimeInclude := false
	switch {
	case lex.isKeyword("["):
		startTimeInclude = true
	case lex.isKeyword("("):
		startTimeInclude = false
	default:
		s, err := getCompoundToken(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse _time filter: %w", err)
		}
		sLower := strings.ToLower(s)
		if sLower == "now" || startsWithYear(s) {
			// Parse '_time:YYYY-MM-DD', which transforms to '_time:[YYYY-MM-DD, YYYY-MM-DD+1)'
			nsecs, err := promutils.ParseTimeAt(s, lex.currentTimestamp)
			if err != nil {
				return nil, fmt.Errorf("cannot parse _time filter: %w", err)
			}
			// Round to milliseconds
			startTime := nsecs
			endTime := getMatchingEndTime(startTime, s)
			ft := &filterTime{
				minTimestamp: startTime,
				maxTimestamp: endTime,

				stringRepr: s,
			}
			return ft, nil
		}
		// Parse _time:duration, which transforms to '_time:(now-duration, now]'
		d, ok := tryParseDuration(s)
		if !ok {
			return nil, fmt.Errorf("cannot parse duration %q in _time filter", s)
		}
		if d < 0 {
			d = -d
		}
		ft := &filterTime{
			minTimestamp: lex.currentTimestamp - int64(d),
			maxTimestamp: lex.currentTimestamp,

			stringRepr: s,
		}
		return ft, nil
	}
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing start time in _time filter")
	}

	// Parse start time
	startTime, startTimeString, err := parseTime(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse start time in _time filter: %w", err)
	}
	if !lex.isKeyword(",") {
		return nil, fmt.Errorf("unexpected token after start time in _time filter: %q; want ','", lex.token)
	}
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing end time in _time filter")
	}

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
		endTime = getMatchingEndTime(endTime, endTimeString)
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

func getMatchingEndTime(startTime int64, stringRepr string) int64 {
	tStart := time.Unix(0, startTime).UTC()
	tEnd := tStart
	timeStr := stripTimezoneSuffix(stringRepr)
	switch {
	case len(timeStr) == len("YYYY"):
		y, m, d := tStart.Date()
		nsec := startTime % (24 * 3600 * 1e9)
		tEnd = time.Date(y+1, m, d, 0, 0, int(nsec/1e9), int(nsec%1e9), time.UTC)
	case len(timeStr) == len("YYYY-MM") && timeStr[len("YYYY")] == '-':
		y, m, d := tStart.Date()
		nsec := startTime % (24 * 3600 * 1e9)
		if d != 1 {
			d = 0
			m++
		}
		tEnd = time.Date(y, m+1, d, 0, 0, int(nsec/1e9), int(nsec%1e9), time.UTC)
	case len(timeStr) == len("YYYY-MM-DD") && timeStr[len("YYYY")] == '-':
		tEnd = tStart.Add(24 * time.Hour)
	case len(timeStr) == len("YYYY-MM-DDThh") && timeStr[len("YYYY")] == '-':
		tEnd = tStart.Add(time.Hour)
	case len(timeStr) == len("YYYY-MM-DDThh:mm") && timeStr[len("YYYY")] == '-':
		tEnd = tStart.Add(time.Minute)
	case len(timeStr) == len("YYYY-MM-DDThh:mm:ss") && timeStr[len("YYYY")] == '-':
		tEnd = tStart.Add(time.Second)
	case len(timeStr) == len("YYYY-MM-DDThh:mm:ss.SSS") && timeStr[len("YYYY")] == '-':
		tEnd = tStart.Add(time.Millisecond)
	default:
		tEnd = tStart.Add(time.Nanosecond)
	}
	return tEnd.UnixNano() - 1
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

func parseFilterStream(lex *lexer) (*filterStream, error) {
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
	s, err := getCompoundToken(lex)
	if err != nil {
		return 0, "", err
	}
	nsecs, err := promutils.ParseTimeAt(s, lex.currentTimestamp)
	if err != nil {
		return 0, "", err
	}
	return nsecs, s, nil
}

func quoteStringTokenIfNeeded(s string) string {
	if !needQuoteStringToken(s) {
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
	return s[0] >= '0' && s[0] <= '9'
}

func needQuoteToken(s string) bool {
	sLower := strings.ToLower(s)
	if _, ok := reservedKeywords[sLower]; ok {
		return true
	}
	for _, r := range s {
		if !isTokenRune(r) && r != '.' && r != '-' {
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
		"exact",
		"i",
		"in",
		"ipv4_range",
		"len_range",
		"range",
		"re",
		"seq",
		"string_range",
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
		if nn < 0 {
			return 0, fmt.Errorf("cannot parse negative value %q as unsigned integer", s)
		}
	}
	return uint64(nn), nil
}

func parseInt(s string) (int64, error) {
	switch {
	case strings.EqualFold(s, "inf"), strings.EqualFold(s, "+inf"):
		return math.MaxInt64, nil
	case strings.EqualFold(s, "-inf"):
		return math.MinInt64, nil
	}

	n, err := strconv.ParseInt(s, 0, 64)
	if err == nil {
		return n, nil
	}
	nn, ok := tryParseBytes(s)
	if !ok {
		nn, ok = tryParseDuration(s)
		if !ok {
			return 0, fmt.Errorf("cannot parse %q as integer: %w", s, err)
		}
	}
	return nn, nil
}

func nextafter(f, xInf float64) float64 {
	if math.IsInf(f, 0) {
		return f
	}
	return math.Nextafter(f, xInf)
}
