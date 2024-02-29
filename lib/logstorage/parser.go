package logstorage

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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

func newLexer(s string) *lexer {
	return &lexer{
		s:                s,
		sOrig:            s,
		currentTimestamp: time.Now().UnixNano(),
	}
}

func (lex *lexer) isEnd() bool {
	return len(lex.s) == 0 && len(lex.token) == 0 && len(lex.rawToken) == 0
}

func (lex *lexer) isQuotedToken() bool {
	return lex.token != lex.rawToken
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
}

// String returns string representation for q.
func (q *Query) String() string {
	return q.f.String()
}

func (q *Query) getResultColumnNames() []string {
	m := make(map[string]struct{})
	q.f.updateReferencedColumnNames(m)

	// Substitute an empty column name with _msg column
	if _, ok := m[""]; ok {
		delete(m, "")
		m["_msg"] = struct{}{}
	}

	// unconditionally select _time, _stream and _msg columns
	// TODO: add the ability to filter out these columns
	m["_time"] = struct{}{}
	m["_stream"] = struct{}{}
	m["_msg"] = struct{}{}

	columnNames := make([]string, 0, len(m))
	for k := range m {
		columnNames = append(columnNames, k)
	}
	sort.Strings(columnNames)
	return columnNames
}

// ParseQuery parses s.
func ParseQuery(s string) (*Query, error) {
	lex := newLexer(s)

	f, err := parseFilter(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse filter expression: %w; context: %s", err, lex.context())
	}
	if !lex.isEnd() {
		return nil, fmt.Errorf("unexpected tail: %q", lex.s)
	}

	q := &Query{
		f: f,
	}
	return q, nil
}

func parseFilter(lex *lexer) (filter, error) {
	if !lex.mustNextToken() || lex.isKeyword("|") {
		return nil, fmt.Errorf("missing query")
	}
	af, err := parseOrFilter(lex, "")
	if err != nil {
		return nil, err
	}
	return af, nil
}

func parseOrFilter(lex *lexer, fieldName string) (filter, error) {
	var filters []filter
	for {
		f, err := parseAndFilter(lex, fieldName)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
		switch {
		case lex.isKeyword("|", ")", ""):
			if len(filters) == 1 {
				return filters[0], nil
			}
			of := &orFilter{
				filters: filters,
			}
			return of, nil
		case lex.isKeyword("or"):
			if !lex.mustNextToken() {
				return nil, fmt.Errorf("missing filter after 'or'")
			}
		}
	}
}

func parseAndFilter(lex *lexer, fieldName string) (filter, error) {
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
			af := &andFilter{
				filters: filters,
			}
			return af, nil
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
		f := &prefixFilter{
			fieldName: fieldName,
			prefix:    "",
		}
		return f, nil
	case lex.isKeyword("("):
		if !lex.isSkippedSpace && !lex.isPrevToken("", ":", "(", "!", "not") {
			return nil, fmt.Errorf("missing whitespace before the search word %q", lex.prevToken)
		}
		return parseParensFilter(lex, fieldName)
	case lex.isKeyword("not", "!"):
		return parseNotFilter(lex, fieldName)
	case lex.isKeyword("exact"):
		return parseExactFilter(lex, fieldName)
	case lex.isKeyword("i"):
		return parseAnyCaseFilter(lex, fieldName)
	case lex.isKeyword("in"):
		return parseInFilter(lex, fieldName)
	case lex.isKeyword("ipv4_range"):
		return parseIPv4RangeFilter(lex, fieldName)
	case lex.isKeyword("len_range"):
		return parseLenRangeFilter(lex, fieldName)
	case lex.isKeyword("range"):
		return parseRangeFilter(lex, fieldName)
	case lex.isKeyword("re"):
		return parseRegexpFilter(lex, fieldName)
	case lex.isKeyword("seq"):
		return parseSequenceFilter(lex, fieldName)
	case lex.isKeyword("string_range"):
		return parseStringRangeFilter(lex, fieldName)
	case lex.isKeyword(`"`, "'", "`"):
		return nil, fmt.Errorf("improperly quoted string")
	case lex.isKeyword(",", ")", "[", "]"):
		return nil, fmt.Errorf("unexpected token %q", lex.token)
	}
	phrase := getCompoundPhrase(lex, fieldName)
	return parseFilterForPhrase(lex, phrase, fieldName)
}

func getCompoundPhrase(lex *lexer, fieldName string) string {
	phrase := lex.token
	rawPhrase := lex.rawToken
	lex.nextToken()
	suffix := getCompoundSuffix(lex, fieldName)
	if suffix == "" {
		return phrase
	}
	return rawPhrase + suffix
}

func getCompoundSuffix(lex *lexer, fieldName string) string {
	s := ""
	stopTokens := []string{"*", ",", "(", ")", "[", "]", "|", ""}
	if fieldName == "" {
		stopTokens = append(stopTokens, ":")
	}
	for !lex.isSkippedSpace && !lex.isKeyword(stopTokens...) {
		s += lex.rawToken
		lex.nextToken()
	}
	return s
}

func getCompoundToken(lex *lexer) string {
	s := lex.token
	rawS := lex.rawToken
	lex.nextToken()
	suffix := ""
	for !lex.isSkippedSpace && !lex.isKeyword(",", "(", ")", "[", "]", "|", "") {
		s += lex.token
		lex.nextToken()
	}
	if suffix == "" {
		return s
	}
	return rawS + suffix
}

func getCompoundFuncArg(lex *lexer) string {
	if lex.isKeyword("*") {
		return ""
	}
	arg := lex.token
	rawArg := lex.rawToken
	lex.nextToken()
	suffix := ""
	for !lex.isSkippedSpace && !lex.isKeyword("*", ",", ")", "") {
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
			f := &prefixFilter{
				fieldName: fieldName,
				prefix:    phrase,
			}
			return f, nil
		}
		// The phrase is a search phrase.
		f := &phraseFilter{
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
		return parseTimeFilterWithOffset(lex)
	case "_stream":
		return parseStreamFilter(lex)
	default:
		return parseGenericFilter(lex, fieldName)
	}
}

func parseParensFilter(lex *lexer, fieldName string) (filter, error) {
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing filter after '('")
	}
	f, err := parseOrFilter(lex, fieldName)
	if err != nil {
		return nil, err
	}
	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("unexpected token %q instead of ')'", lex.token)
	}
	lex.nextToken()
	return f, nil
}

func parseNotFilter(lex *lexer, fieldName string) (filter, error) {
	notKeyword := lex.token
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing filters after '%s'", notKeyword)
	}
	f, err := parseGenericFilter(lex, fieldName)
	if err != nil {
		return nil, err
	}
	nf, ok := f.(*notFilter)
	if ok {
		return nf.f, nil
	}
	nf = &notFilter{
		f: f,
	}
	return nf, nil
}

func parseAnyCaseFilter(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgMaybePrefix(lex, "i", fieldName, func(phrase string, isPrefixFilter bool) (filter, error) {
		if isPrefixFilter {
			f := &anyCasePrefixFilter{
				fieldName: fieldName,
				prefix:    phrase,
			}
			return f, nil
		}
		f := &anyCasePhraseFilter{
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
		phrase += getCompoundSuffix(lex, fieldName)
		return parseFilterForPhrase(lex, phrase, fieldName)
	}
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing arg for %s()", funcName)
	}
	phrase = getCompoundFuncArg(lex)
	isPrefixFilter := false
	if lex.isKeyword("*") && !lex.isSkippedSpace {
		isPrefixFilter = true
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing ')' after %s()", funcName)
		}
	}
	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("unexpected token %q instead of ')' in %s()", lex.token, funcName)
	}
	lex.nextToken()
	return callback(phrase, isPrefixFilter)
}

func parseLenRangeFilter(lex *lexer, fieldName string) (filter, error) {
	funcName := lex.token
	return parseFuncArgs(lex, fieldName, func(args []string) (filter, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("unexpected number of args for %s(); got %d; want 2", funcName, len(args))
		}
		minLen, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse minLen at %s(): %w", funcName, err)
		}
		maxLen, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse maxLen at %s(): %w", funcName, err)
		}
		rf := &lenRangeFilter{
			fieldName: fieldName,
			minLen:    minLen,
			maxLen:    maxLen,
		}
		return rf, nil
	})
}

func parseStringRangeFilter(lex *lexer, fieldName string) (filter, error) {
	funcName := lex.token
	return parseFuncArgs(lex, fieldName, func(args []string) (filter, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("unexpected number of args for %s(); got %d; want 2", funcName, len(args))
		}
		rf := &stringRangeFilter{
			fieldName: fieldName,
			minValue:  args[0],
			maxValue:  args[1],
		}
		return rf, nil
	})
}

func parseIPv4RangeFilter(lex *lexer, fieldName string) (filter, error) {
	funcName := lex.token
	return parseFuncArgs(lex, fieldName, func(args []string) (filter, error) {
		if len(args) == 1 {
			minValue, maxValue, ok := tryParseIPv4CIDR(args[0])
			if !ok {
				return nil, fmt.Errorf("cannot parse IPv4 address or IPv4 CIDR %q at %s()", args[0], funcName)
			}
			rf := &ipv4RangeFilter{
				fieldName: fieldName,
				minValue:  minValue,
				maxValue:  maxValue,
			}
			return rf, nil
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
		rf := &ipv4RangeFilter{
			fieldName: fieldName,
			minValue:  minValue,
			maxValue:  maxValue,
		}
		return rf, nil
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

func parseInFilter(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgs(lex, fieldName, func(args []string) (filter, error) {
		f := &inFilter{
			fieldName: fieldName,
			values:    args,
		}
		return f, nil
	})
}

func parseSequenceFilter(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgs(lex, fieldName, func(args []string) (filter, error) {
		sf := &sequenceFilter{
			fieldName: fieldName,
			phrases:   args,
		}
		return sf, nil
	})
}

func parseExactFilter(lex *lexer, fieldName string) (filter, error) {
	return parseFuncArgMaybePrefix(lex, "exact", fieldName, func(phrase string, isPrefixFilter bool) (filter, error) {
		if isPrefixFilter {
			f := &exactPrefixFilter{
				fieldName: fieldName,
				prefix:    phrase,
			}
			return f, nil
		}
		f := &exactFilter{
			fieldName: fieldName,
			value:     phrase,
		}
		return f, nil
	})
}

func parseRegexpFilter(lex *lexer, fieldName string) (filter, error) {
	funcName := lex.token
	return parseFuncArg(lex, fieldName, func(arg string) (filter, error) {
		re, err := regexp.Compile(arg)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp %q for %s(): %w", arg, funcName, err)
		}
		rf := &regexpFilter{
			fieldName: fieldName,
			re:        re,
		}
		return rf, nil
	})
}

func parseRangeFilter(lex *lexer, fieldName string) (filter, error) {
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
		phrase := funcName + getCompoundSuffix(lex, fieldName)
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

	stringRepr := ""
	if includeMinValue {
		stringRepr += "["
	} else {
		stringRepr += "("
		minValue = math.Nextafter(minValue, math.Inf(1))
	}
	stringRepr += minValueStr + "," + maxValueStr
	if includeMaxValue {
		stringRepr += "]"
	} else {
		stringRepr += ")"
		maxValue = math.Nextafter(maxValue, math.Inf(-1))
	}

	rf := &rangeFilter{
		fieldName: fieldName,
		minValue:  minValue,
		maxValue:  maxValue,

		stringRepr: stringRepr,
	}
	return rf, nil
}

func parseFloat64(lex *lexer) (float64, string, error) {
	s := getCompoundToken(lex)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, "", fmt.Errorf("cannot parse %q as float64: %w", lex.token, err)
	}
	return f, s, nil
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
		phrase := funcName + getCompoundSuffix(lex, fieldName)
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

func parseTimeFilterWithOffset(lex *lexer) (*timeFilter, error) {
	tf, err := parseTimeFilter(lex)
	if err != nil {
		return nil, err
	}
	if !lex.isKeyword("offset") {
		return tf, nil
	}
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing offset for _time filter %s", tf)
	}
	s := getCompoundToken(lex)
	d, err := promutils.ParseDuration(s)
	if err != nil {
		return nil, fmt.Errorf("cannot parse offset for _time filter %s: %w", tf, err)
	}
	offset := int64(d)
	tf.minTimestamp -= offset
	tf.maxTimestamp -= offset
	tf.stringRepr += " offset " + s
	return tf, nil
}

func parseTimeFilter(lex *lexer) (*timeFilter, error) {
	startTimeInclude := false
	switch {
	case lex.isKeyword("["):
		startTimeInclude = true
	case lex.isKeyword("("):
		startTimeInclude = false
	default:
		s := getCompoundToken(lex)
		sLower := strings.ToLower(s)
		if sLower == "now" || startsWithYear(s) {
			// Parse '_time:YYYY-MM-DD', which transforms to '_time:[YYYY-MM-DD, YYYY-MM-DD+1)'
			t, err := promutils.ParseTimeAt(s, float64(lex.currentTimestamp)/1e9)
			if err != nil {
				return nil, fmt.Errorf("cannot parse _time filter: %w", err)
			}
			startTime := int64(t * 1e9)
			endTime := getMatchingEndTime(startTime, s)
			tf := &timeFilter{
				minTimestamp: startTime,
				maxTimestamp: endTime,

				stringRepr: s,
			}
			return tf, nil
		}
		// Parse _time:duration, which transforms to '_time:(now-duration, now]'
		d, err := promutils.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse duration in _time filter: %w", err)
		}
		if d < 0 {
			d = -d
		}
		tf := &timeFilter{
			minTimestamp: lex.currentTimestamp - int64(d),
			maxTimestamp: lex.currentTimestamp,

			stringRepr: s,
		}
		return tf, nil
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

	tf := &timeFilter{
		minTimestamp: startTime,
		maxTimestamp: endTime,

		stringRepr: stringRepr,
	}
	return tf, nil
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

func parseStreamFilter(lex *lexer) (*streamFilter, error) {
	if !lex.isKeyword("{") {
		return nil, fmt.Errorf("unexpected token %q instead of '{' in _stream filter", lex.token)
	}
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("incomplete _stream filter after '{'")
	}
	var filters []*andStreamFilter
	for {
		f, err := parseAndStreamFilter(lex)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
		switch {
		case lex.isKeyword("}"):
			lex.nextToken()
			sf := &streamFilter{
				f: &StreamFilter{
					orFilters: filters,
				},
			}
			return sf, nil
		case lex.isKeyword("or"):
			if !lex.mustNextToken() {
				return nil, fmt.Errorf("incomplete _stream filter after 'or'")
			}
			if lex.isKeyword("}") {
				return nil, fmt.Errorf("unexpected '}' after 'or' in _stream filter")
			}
		default:
			return nil, fmt.Errorf("unexpected token in _stream filter: %q; want '}' or 'or'", lex.token)
		}
	}
}

func newStreamFilter(s string) (*StreamFilter, error) {
	lex := newLexer(s)
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing '{' in _stream filter")
	}
	sf, err := parseStreamFilter(lex)
	if err != nil {
		return nil, err
	}
	return sf.f, nil
}

func parseAndStreamFilter(lex *lexer) (*andStreamFilter, error) {
	var filters []*streamTagFilter
	for {
		if lex.isKeyword("}") {
			asf := &andStreamFilter{
				tagFilters: filters,
			}
			return asf, nil
		}
		f, err := parseStreamTagFilter(lex)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
		switch {
		case lex.isKeyword("or", "}"):
			asf := &andStreamFilter{
				tagFilters: filters,
			}
			return asf, nil
		case lex.isKeyword(","):
			if !lex.mustNextToken() {
				return nil, fmt.Errorf("missing stream filter after ','")
			}
		default:
			return nil, fmt.Errorf("unexpected token %q in _stream filter; want 'or', 'and', '}' or ','", lex.token)
		}
	}
}

func parseStreamTagFilter(lex *lexer) (*streamTagFilter, error) {
	tagName := lex.token
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing operation in _stream filter for %q field", tagName)
	}
	if !lex.isKeyword("=", "!=", "=~", "!~") {
		return nil, fmt.Errorf("unsupported operation %q in _steam filter for %q field; supported operations: =, !=, =~, !~", lex.token, tagName)
	}
	op := lex.token
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing _stream filter value for %q field", tagName)
	}
	value := lex.token
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing token after %q%s%q filter", tagName, op, value)
	}
	stf := &streamTagFilter{
		tagName: tagName,
		op:      op,
		value:   value,
	}
	if op == "=~" || op == "!~" {
		re, err := regexutil.NewPromRegex(value)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp %q for stream filter: %w", value, err)
		}
		stf.regexp = re
	}
	return stf, nil
}

func parseTime(lex *lexer) (int64, string, error) {
	s := getCompoundToken(lex)
	t, err := promutils.ParseTimeAt(s, float64(lex.currentTimestamp)/1e9)
	if err != nil {
		return 0, "", err
	}
	return int64(t * 1e9), s, nil
}

func quoteTokenIfNeeded(s string) string {
	if !needQuoteToken(s) {
		return s
	}
	return strconv.Quote(s)
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
