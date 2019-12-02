package storage

import (
	"bytes"
	"fmt"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

// TagFilters represents filters used for filtering tags.
type TagFilters struct {
	accountID uint32
	projectID uint32

	tfs []tagFilter

	// Common prefix for all the tag filters.
	// Contains encoded nsPrefixTagToMetricIDs + accountID + projectID.
	commonPrefix []byte
}

// NewTagFilters returns new TagFilters for the given accountID and projectID.
func NewTagFilters(accountID, projectID uint32) *TagFilters {
	return &TagFilters{
		accountID:    accountID,
		projectID:    projectID,
		commonPrefix: marshalCommonPrefix(nil, nsPrefixTagToMetricIDs, accountID, projectID),
	}
}

// Add adds the given tag filter to tfs.
//
// MetricGroup must be encoded with nil key.
func (tfs *TagFilters) Add(key, value []byte, isNegative, isRegexp bool) error {
	// Verify whether tag filter is empty.
	if len(value) == 0 {
		// Substitute an empty tag value with the negative match
		// of `.+` regexp in order to filter out all the values with
		// the given tag.
		isNegative = !isNegative
		isRegexp = true
		value = []byte(".+")
	}
	if isRegexp && string(value) == ".*" {
		if !isNegative {
			// Skip tag filter matching anything, since it equal to no filter.
			return nil
		}

		// Leave negative tag filter matching anything as is,
		// since it must filter out all the time series with the given key.
	}

	if cap(tfs.tfs) > len(tfs.tfs) {
		tfs.tfs = tfs.tfs[:len(tfs.tfs)+1]
	} else {
		tfs.tfs = append(tfs.tfs, tagFilter{})
	}
	tf := &tfs.tfs[len(tfs.tfs)-1]
	err := tf.Init(tfs.commonPrefix, key, value, isNegative, isRegexp)
	if err != nil {
		return fmt.Errorf("cannot initialize tagFilter: %s", err)
	}
	return nil
}

// String returns human-readable value for tfs.
func (tfs *TagFilters) String() string {
	var bb bytes.Buffer
	fmt.Fprintf(&bb, "AccountID=%d, ProjectID=%d ", tfs.accountID, tfs.projectID)
	if len(tfs.tfs) == 0 {
		fmt.Fprintf(&bb, "{}")
		return bb.String()
	}
	fmt.Fprintf(&bb, "{%s", tfs.tfs[0].String())
	for i := range tfs.tfs[1:] {
		fmt.Fprintf(&bb, ", %s", tfs.tfs[i].String())
	}
	fmt.Fprintf(&bb, "}")
	return bb.String()
}

// Reset resets the tf for the given accountID and projectID
func (tfs *TagFilters) Reset(accountID, projectID uint32) {
	tfs.accountID = accountID
	tfs.projectID = projectID
	tfs.tfs = tfs.tfs[:0]
	tfs.commonPrefix = marshalCommonPrefix(tfs.commonPrefix[:0], nsPrefixTagToMetricIDs, accountID, projectID)
}

func (tfs *TagFilters) marshal(dst []byte) []byte {
	dst = encoding.MarshalUint32(dst, tfs.accountID)
	dst = encoding.MarshalUint32(dst, tfs.projectID)
	for i := range tfs.tfs {
		dst = tfs.tfs[i].MarshalNoAccountIDProjectID(dst)
	}
	return dst
}

// tagFilter represents a filter used for filtering tags.
type tagFilter struct {
	key        []byte
	value      []byte
	isNegative bool
	isRegexp   bool

	// Prefix always contains {nsPrefixTagToMetricIDs, AccountID, ProjectID, key}.
	// Additionally it contains:
	//  - value ending with tagSeparatorChar if !isRegexp.
	//  - non-regexp prefix if isRegexp.
	prefix []byte

	// or values obtained from regexp suffix if it equals to "foo|bar|..."
	orSuffixes []string

	// Matches regexp suffix.
	reSuffixMatch func(b []byte) bool
}

// String returns human-readable tf value.
func (tf *tagFilter) String() string {
	op := "="
	if tf.isNegative {
		op = "!="
		if tf.isRegexp {
			op = "!~"
		}
	} else if tf.isRegexp {
		op = "=~"
	}
	return fmt.Sprintf("%s%s%q", tf.key, op, tf.value)
}

func (tf *tagFilter) Marshal(dst []byte, accountID, projectID uint32) []byte {
	dst = encoding.MarshalUint32(dst, accountID)
	dst = encoding.MarshalUint32(dst, projectID)
	return tf.MarshalNoAccountIDProjectID(dst)
}

// MarshalNoAccountIDProjectID appends marshaled tf to dst
// and returns the result.
func (tf *tagFilter) MarshalNoAccountIDProjectID(dst []byte) []byte {
	dst = marshalTagValue(dst, tf.key)
	dst = marshalTagValue(dst, tf.value)

	isNegative := byte(0)
	if tf.isNegative {
		isNegative = 1
	}

	isRegexp := byte(0)
	if tf.isRegexp {
		isRegexp = 1
	}

	dst = append(dst, isNegative, isRegexp)
	return dst
}

// Init initializes the tag filter for the given commonPrefix, key and value.
//
// If isNegaitve is true, then the tag filter matches all the values
// except the given one.
//
// If isRegexp is true, then the value is interpreted as anchored regexp,
// i.e. '^(tag.Value)$'.
//
// MetricGroup must be encoded in the value with nil key.
func (tf *tagFilter) Init(commonPrefix, key, value []byte, isNegative, isRegexp bool) error {
	tf.key = append(tf.key[:0], key...)
	tf.value = append(tf.value[:0], value...)
	tf.isNegative = isNegative
	tf.isRegexp = isRegexp

	tf.prefix = tf.prefix[:0]

	tf.orSuffixes = tf.orSuffixes[:0]
	tf.reSuffixMatch = nil

	tf.prefix = append(tf.prefix, commonPrefix...)
	tf.prefix = marshalTagValue(tf.prefix, key)

	var expr []byte
	prefix := tf.value
	if tf.isRegexp {
		prefix, expr = getRegexpPrefix(tf.value)
		if len(expr) == 0 {
			tf.isRegexp = false
		}
	}
	tf.prefix = marshalTagValueNoTrailingTagSeparator(tf.prefix, prefix)
	if !tf.isRegexp {
		// tf contains plain value without regexp.
		// Add empty orSuffix in order to trigger fast path for orSuffixes
		// during the search for matching metricIDs.
		tf.orSuffixes = append(tf.orSuffixes[:0], "")
		return nil
	}
	rcv, err := getRegexpFromCache(expr)
	if err != nil {
		return err
	}
	tf.orSuffixes = append(tf.orSuffixes[:0], rcv.orValues...)
	tf.reSuffixMatch = rcv.reMatch
	return nil
}

func (tf *tagFilter) matchSuffix(b []byte) (bool, error) {
	// Remove the trailing tagSeparatorChar.
	if len(b) == 0 || b[len(b)-1] != tagSeparatorChar {
		return false, fmt.Errorf("unexpected end of b; want %d; b=%q", tagSeparatorChar, b)
	}
	b = b[:len(b)-1]
	if !tf.isRegexp {
		return len(b) == 0, nil
	}
	ok := tf.reSuffixMatch(b)
	return ok, nil
}

// RegexpCacheSize returns the number of cached regexps for tag filters.
func RegexpCacheSize() int {
	regexpCacheLock.RLock()
	n := len(regexpCacheMap)
	regexpCacheLock.RUnlock()
	return n
}

// RegexpCacheRequests returns the number of requests to regexp cache.
func RegexpCacheRequests() uint64 {
	return atomic.LoadUint64(&regexpCacheRequests)
}

// RegexpCacheMisses returns the number of cache misses for regexp cache.
func RegexpCacheMisses() uint64 {
	return atomic.LoadUint64(&regexpCacheMisses)
}

func getRegexpFromCache(expr []byte) (regexpCacheValue, error) {
	atomic.AddUint64(&regexpCacheRequests, 1)

	// Fast path - search the regexp in the cache.
	regexpCacheLock.RLock()
	rcv, ok := regexpCacheMap[string(expr)]
	regexpCacheLock.RUnlock()

	if ok {
		return rcv, nil
	}

	// Slow path - build the regexp.
	atomic.AddUint64(&regexpCacheMisses, 1)
	exprOrig := string(expr)

	expr = []byte(tagCharsRegexpEscaper.Replace(exprOrig))
	exprStr := fmt.Sprintf("^(%s)$", expr)
	re, err := regexp.Compile(exprStr)
	if err != nil {
		return rcv, fmt.Errorf("invalid regexp %q: %s", exprStr, err)
	}

	sExpr := string(expr)
	orValues := getOrValues(sExpr)
	var reMatch func(b []byte) bool
	if len(orValues) > 0 {
		if len(orValues) == 1 {
			v := orValues[0]
			reMatch = func(b []byte) bool {
				return string(b) == v
			}
		} else {
			reMatch = func(b []byte) bool {
				for _, v := range orValues {
					if string(b) == v {
						return true
					}
				}
				return false
			}
		}
	} else {
		reMatch = getReMatchFunc(sExpr)
	}
	if reMatch == nil {
		reMatch = func(b []byte) bool {
			return re.Match(b)
		}
	}

	// Put the reMatch in the cache.
	rcv.orValues = orValues
	rcv.reMatch = reMatch

	regexpCacheLock.Lock()
	if overflow := len(regexpCacheMap) - getMaxRegexpCacheSize(); overflow > 0 {
		overflow = int(float64(len(regexpCacheMap)) * 0.1)
		for k := range regexpCacheMap {
			delete(regexpCacheMap, k)
			overflow--
			if overflow <= 0 {
				break
			}
		}
	}
	regexpCacheMap[exprOrig] = rcv
	regexpCacheLock.Unlock()

	return rcv, nil
}

// getReMatchFunc returns a function for matching the given expr.
//   '.*'
//   '.+'
//   'literal.*'
//   '.*literal.*'
//   '.*literal'
func getReMatchFunc(expr string) func(b []byte) bool {
	re, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing verified expr=%q: %s", expr, err)
	}
	if isDotStar(re) {
		return func(b []byte) bool {
			return true
		}
	}
	if isDotPlus(re) {
		return func(b []byte) bool {
			return len(b) > 0
		}
	}
	return getSingleValueFuncExt(re)
}

func getSingleValueFuncExt(re *syntax.Regexp) func(b []byte) bool {
	switch re.Op {
	case syntax.OpCapture:
		return getSingleValueFuncExt(re.Sub[0])
	case syntax.OpLiteral:
		if !isLiteral(re) {
			return nil
		}
		s := string(re.Rune)
		return func(b []byte) bool {
			return string(b) == s
		}
	case syntax.OpConcat:
		if len(re.Sub) == 2 {
			if isDotStar(re.Sub[0]) && isLiteral(re.Sub[1]) {
				suffix := []byte(string(re.Sub[1].Rune))
				return func(b []byte) bool {
					return bytes.HasSuffix(b, suffix)
				}
			}
			if isLiteral(re.Sub[0]) && isDotStar(re.Sub[1]) {
				prefix := []byte(string(re.Sub[0].Rune))
				return func(b []byte) bool {
					return bytes.HasPrefix(b, prefix)
				}
			}
			return nil
		}
		if len(re.Sub) != 3 || !isDotStar(re.Sub[0]) || !isDotStar(re.Sub[2]) || !isLiteral(re.Sub[1]) {
			return nil
		}
		middle := []byte(string(re.Sub[1].Rune))
		return func(b []byte) bool {
			return bytes.Contains(b, middle)
		}
	default:
		return nil
	}
}

func isDotStar(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpCapture:
		return isDotStar(re.Sub[0])
	case syntax.OpAlternate:
		for _, reSub := range re.Sub {
			if isDotStar(reSub) {
				return true
			}
		}
		return false
	case syntax.OpStar:
		switch re.Sub[0].Op {
		case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func isDotPlus(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpCapture:
		return isDotPlus(re.Sub[0])
	case syntax.OpAlternate:
		for _, reSub := range re.Sub {
			if isDotPlus(reSub) {
				return true
			}
		}
		return false
	case syntax.OpPlus:
		switch re.Sub[0].Op {
		case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func isLiteral(re *syntax.Regexp) bool {
	if re.Op == syntax.OpCapture {
		return isLiteral(re.Sub[0])
	}
	return re.Op == syntax.OpLiteral && re.Flags&syntax.FoldCase == 0
}

func getOrValues(expr string) []string {
	re, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing verified expr=%q: %s", expr, err)
	}
	orValues := getOrValuesExt(re)

	// Sort orValues for faster index seek later
	sort.Strings(orValues)

	return orValues
}

func getOrValuesExt(re *syntax.Regexp) []string {
	switch re.Op {
	case syntax.OpCapture:
		return getOrValuesExt(re.Sub[0])
	case syntax.OpLiteral:
		if !isLiteral(re) {
			return nil
		}
		return []string{string(re.Rune)}
	case syntax.OpEmptyMatch:
		return []string{""}
	case syntax.OpAlternate:
		a := make([]string, 0, len(re.Sub))
		for _, reSub := range re.Sub {
			ca := getOrValuesExt(reSub)
			if len(ca) == 0 {
				return nil
			}
			a = append(a, ca...)
			if len(a) > maxOrValues {
				// It is cheaper to use regexp here.
				return nil
			}
		}
		return a
	case syntax.OpCharClass:
		a := make([]string, 0, len(re.Rune)/2)
		for i := 0; i < len(re.Rune); i += 2 {
			start := re.Rune[i]
			end := re.Rune[i+1]
			for start <= end {
				a = append(a, string(start))
				start++
				if len(a) > maxOrValues {
					// It is cheaper to use regexp here.
					return nil
				}
			}
		}
		return a
	case syntax.OpConcat:
		if len(re.Sub) < 1 {
			return []string{""}
		}
		prefixes := getOrValuesExt(re.Sub[0])
		if len(prefixes) == 0 {
			return nil
		}
		re.Sub = re.Sub[1:]
		suffixes := getOrValuesExt(re)
		if len(suffixes) == 0 {
			return nil
		}
		if len(prefixes)*len(suffixes) > maxOrValues {
			// It is cheaper to use regexp here.
			return nil
		}
		a := make([]string, 0, len(prefixes)*len(suffixes))
		for _, prefix := range prefixes {
			for _, suffix := range suffixes {
				s := prefix + suffix
				a = append(a, s)
			}
		}
		return a
	default:
		return nil
	}
}

const maxOrValues = 20

var tagCharsRegexpEscaper = strings.NewReplacer(
	"\\x00", "(?:\\x000)", // escapeChar
	"\x00", "(?:\\x000)", // escapeChar
	"\\x01", "(?:\\x001)", // tagSeparatorChar
	"\x01", "(?:\\x001)", // tagSeparatorChar
	"\\x02", "(?:\\x002)", // kvSeparatorChar
	"\x02", "(?:\\x002)", // kvSeparatorChar
)

func getMaxRegexpCacheSize() int {
	maxRegexpCacheSizeOnce.Do(func() {
		n := memory.Allowed() / 1024 / 1024
		if n < 100 {
			n = 100
		}
		maxRegexpCacheSize = n
	})
	return maxRegexpCacheSize
}

var (
	maxRegexpCacheSize     int
	maxRegexpCacheSizeOnce sync.Once
)

var (
	regexpCacheMap  = make(map[string]regexpCacheValue)
	regexpCacheLock sync.RWMutex

	regexpCacheRequests uint64
	regexpCacheMisses   uint64
)

type regexpCacheValue struct {
	orValues []string
	reMatch  func(b []byte) bool
}

func getRegexpPrefix(b []byte) ([]byte, []byte) {
	// Fast path - search the prefix in the cache.
	prefixesCacheLock.RLock()
	ps, ok := prefixesCacheMap[string(b)]
	prefixesCacheLock.RUnlock()

	if ok {
		return ps.prefix, ps.suffix
	}

	// Slow path - extract the regexp prefix from b.
	prefix, suffix := extractRegexpPrefix(b)

	// Put the prefix and the suffix to the cache.
	prefixesCacheLock.Lock()
	if overflow := len(prefixesCacheMap) - getMaxPrefixesCacheSize(); overflow > 0 {
		overflow = int(float64(len(prefixesCacheMap)) * 0.1)
		for k := range prefixesCacheMap {
			delete(prefixesCacheMap, k)
			overflow--
			if overflow <= 0 {
				break
			}
		}
	}
	prefixesCacheMap[string(b)] = prefixSuffix{
		prefix: prefix,
		suffix: suffix,
	}
	prefixesCacheLock.Unlock()

	return prefix, suffix
}

func getMaxPrefixesCacheSize() int {
	maxPrefixesCacheSizeOnce.Do(func() {
		n := memory.Allowed() / 1024 / 1024
		if n < 100 {
			n = 100
		}
		maxPrefixesCacheSize = n
	})
	return maxPrefixesCacheSize
}

var (
	maxPrefixesCacheSize     int
	maxPrefixesCacheSizeOnce sync.Once
)

var (
	prefixesCacheMap  = make(map[string]prefixSuffix)
	prefixesCacheLock sync.RWMutex
)

type prefixSuffix struct {
	prefix []byte
	suffix []byte
}

func extractRegexpPrefix(b []byte) ([]byte, []byte) {
	re, err := syntax.Parse(string(b), syntax.Perl)
	if err != nil {
		// Cannot parse the regexp. Return it all as prefix.
		return b, nil
	}
	re = simplifyRegexp(re)
	if re == emptyRegexp {
		return nil, nil
	}
	if isLiteral(re) {
		return []byte(string(re.Rune)), nil
	}
	var prefix []byte
	if re.Op == syntax.OpConcat {
		sub0 := re.Sub[0]
		if isLiteral(sub0) {
			prefix = []byte(string(sub0.Rune))
			re.Sub = re.Sub[1:]
			if len(re.Sub) == 0 {
				return nil, nil
			}
		}
	}
	if _, err := syntax.Compile(re); err != nil {
		// Cannot compile the regexp. Return it all as prefix.
		return b, nil
	}
	return prefix, []byte(re.String())
}

func simplifyRegexp(re *syntax.Regexp) *syntax.Regexp {
	s := re.String()
	for {
		re = simplifyRegexpExt(re, false, false)
		re = re.Simplify()
		if re.Op == syntax.OpBeginText || re.Op == syntax.OpEndText {
			re = emptyRegexp
		}
		sNew := re.String()
		if sNew == s {
			return re
		}
		var err error
		re, err = syntax.Parse(sNew, syntax.Perl)
		if err != nil {
			logger.Panicf("BUG: cannot parse simplified regexp %q: %s", sNew, err)
		}
		s = sNew
	}
}

func simplifyRegexpExt(re *syntax.Regexp, hasPrefix, hasSuffix bool) *syntax.Regexp {
	switch re.Op {
	case syntax.OpCapture:
		// Substitute all the capture regexps with non-capture regexps.
		re.Op = syntax.OpAlternate
		re.Sub[0] = simplifyRegexpExt(re.Sub[0], hasPrefix, hasSuffix)
		if re.Sub[0] == emptyRegexp {
			return emptyRegexp
		}
		return re
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		re.Sub[0] = simplifyRegexpExt(re.Sub[0], hasPrefix, hasSuffix)
		if re.Sub[0] == emptyRegexp {
			return emptyRegexp
		}
		return re
	case syntax.OpAlternate:
		// Do not remove empty captures from OpAlternate, since this may break regexp.
		for i, sub := range re.Sub {
			re.Sub[i] = simplifyRegexpExt(sub, hasPrefix, hasSuffix)
		}
		return re
	case syntax.OpConcat:
		subs := re.Sub[:0]
		for i, sub := range re.Sub {
			if sub = simplifyRegexpExt(sub, i > 0, i+1 < len(re.Sub)); sub != emptyRegexp {
				subs = append(subs, sub)
			}
		}
		re.Sub = subs
		// Remove anchros from the beginning and the end of regexp, since they
		// will be added later.
		if !hasPrefix {
			for len(re.Sub) > 0 && re.Sub[0].Op == syntax.OpBeginText {
				re.Sub = re.Sub[1:]
			}
		}
		if !hasSuffix {
			for len(re.Sub) > 0 && re.Sub[len(re.Sub)-1].Op == syntax.OpEndText {
				re.Sub = re.Sub[:len(re.Sub)-1]
			}
		}
		if len(re.Sub) == 0 {
			return emptyRegexp
		}
		return re
	case syntax.OpEmptyMatch:
		return emptyRegexp
	default:
		return re
	}
}

var emptyRegexp = &syntax.Regexp{
	Op: syntax.OpEmptyMatch,
}
