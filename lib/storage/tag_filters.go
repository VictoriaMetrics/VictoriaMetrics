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

// convertToCompositeTagFilterss converts tfss to composite filters.
//
// This converts `foo{bar="baz",x=~"a.+"}` to `{foo=bar="baz",foo=x=~"a.+"} filter.
func convertToCompositeTagFilterss(tfss []*TagFilters) []*TagFilters {
	tfssNew := make([]*TagFilters, 0, len(tfss))
	for _, tfs := range tfss {
		tfssNew = append(tfssNew, convertToCompositeTagFilters(tfs)...)
	}
	return tfssNew
}

func convertToCompositeTagFilters(tfs *TagFilters) []*TagFilters {
	var tfssCompiled []*TagFilters
	// Search for filters on metric name, which will be used for creating composite filters.
	var names [][]byte
	namePrefix := ""
	hasPositiveFilter := false
	for _, tf := range tfs.tfs {
		if len(tf.key) == 0 {
			if !tf.isNegative && !tf.isRegexp {
				names = [][]byte{tf.value}
			} else if !tf.isNegative && tf.isRegexp && len(tf.orSuffixes) > 0 {
				// Split the filter {__name__=~"name1|...|nameN", other_filters}
				// into name1{other_filters}, ..., nameN{other_filters}
				// and generate composite filters for each of them.
				names = names[:0] // override the previous filters on metric name
				for _, orSuffix := range tf.orSuffixes {
					names = append(names, []byte(orSuffix))
				}
				namePrefix = tf.regexpPrefix
			}
		} else if !tf.isNegative && !tf.isEmptyMatch {
			hasPositiveFilter = true
		}
	}
	if len(names) == 0 {
		atomic.AddUint64(&compositeFilterMissingConversions, 1)
		return []*TagFilters{tfs}
	}

	// Create composite filters for the found names.
	var compositeKey, nameWithPrefix []byte
	for _, name := range names {
		compositeFilters := 0
		tfsNew := make([]tagFilter, 0, len(tfs.tfs))
		for _, tf := range tfs.tfs {
			if len(tf.key) == 0 {
				if !hasPositiveFilter || tf.isNegative {
					// Negative filters on metric name cannot be used for building composite filter, so leave them as is.
					tfsNew = append(tfsNew, tf)
					continue
				}
				if tf.isRegexp {
					matchName := false
					for _, orSuffix := range tf.orSuffixes {
						if orSuffix == string(name) {
							matchName = true
							break
						}
					}
					if !matchName {
						// Leave as is the regexp filter on metric name if it doesn't match the current name.
						tfsNew = append(tfsNew, tf)
						continue
					}
					// Skip the tf, since its part (name) is used as a prefix in composite filter.
					continue
				}
				if string(tf.value) != string(name) {
					// Leave as is the filter on another metric name.
					tfsNew = append(tfsNew, tf)
					continue
				}
				// Skip the tf, since it is used as a prefix in composite filter.
				continue
			}
			if string(tf.key) == "__graphite__" || bytes.Equal(tf.key, graphiteReverseTagKey) {
				// Leave as is __graphite__ filters, since they cannot be used for building composite filter.
				tfsNew = append(tfsNew, tf)
				continue
			}
			// Create composite filter on (name, tf)
			nameWithPrefix = append(nameWithPrefix[:0], namePrefix...)
			nameWithPrefix = append(nameWithPrefix, name...)
			compositeKey = marshalCompositeTagKey(compositeKey[:0], nameWithPrefix, tf.key)
			var tfNew tagFilter
			if err := tfNew.Init(tfs.commonPrefix, compositeKey, tf.value, tf.isNegative, tf.isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error when creating composite tag filter for name=%q and key=%q: %s", name, tf.key, err)
			}
			tfsNew = append(tfsNew, tfNew)
			compositeFilters++
		}
		if compositeFilters == 0 {
			// Cannot use tfsNew, since it doesn't contain composite filters, e.g. it may match broader set of series.
			// Fall back to the original tfs.
			atomic.AddUint64(&compositeFilterMissingConversions, 1)
			return []*TagFilters{tfs}
		}
		tfsCompiled := NewTagFilters(tfs.accountID, tfs.projectID)
		tfsCompiled.tfs = tfsNew
		tfssCompiled = append(tfssCompiled, tfsCompiled)
	}
	atomic.AddUint64(&compositeFilterSuccessConversions, 1)
	return tfssCompiled
}

var (
	compositeFilterSuccessConversions uint64
	compositeFilterMissingConversions uint64
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

// AddGraphiteQuery adds the given Graphite query that matches the given paths to tfs.
func (tfs *TagFilters) AddGraphiteQuery(query []byte, paths []string, isNegative bool) {
	tf := tfs.addTagFilter()
	tf.InitFromGraphiteQuery(tfs.commonPrefix, query, paths, isNegative)
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

		// Substitute negative tag filter matching anything with negative tag filter matching non-empty value
		// in order to filter out all the time series with the given key.
		value = []byte(".+")
	}

	tf := tfs.addTagFilter()
	if err := tf.Init(tfs.commonPrefix, key, value, isNegative, isRegexp); err != nil {
		return fmt.Errorf("cannot initialize tagFilter: %w", err)
	}
	if tf.isNegative && tf.isEmptyMatch {
		// We have {key!~"|foo"} tag filter, which matches non=empty key values.
		// So add {key=~".+"} tag filter in order to enforce this.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/546 for details.
		tfNew := tfs.addTagFilter()
		if err := tfNew.Init(tfs.commonPrefix, key, []byte(".+"), false, true); err != nil {
			return fmt.Errorf(`cannot initialize {%s=".+"} tag filter: %w`, key, err)
		}
	}
	if len(tf.graphiteReverseSuffix) > 0 {
		re := regexp.QuoteMeta(string(tf.graphiteReverseSuffix)) + ".*"
		tfNew := tfs.addTagFilter()
		if err := tfNew.Init(tfs.commonPrefix, graphiteReverseTagKey, []byte(re), false, true); err != nil {
			return fmt.Errorf("cannot initialize reverse tag filter for Graphite wildcard: %w", err)
		}
	}
	return nil
}

func (tfs *TagFilters) addTagFilter() *tagFilter {
	if cap(tfs.tfs) > len(tfs.tfs) {
		tfs.tfs = tfs.tfs[:len(tfs.tfs)+1]
	} else {
		tfs.tfs = append(tfs.tfs, tagFilter{})
	}
	return &tfs.tfs[len(tfs.tfs)-1]
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
		fmt.Fprintf(&bb, ", %s", tfs.tfs[i+1].String())
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

// tagFilter represents a filter used for filtering tags.
type tagFilter struct {
	key        []byte
	value      []byte
	isNegative bool
	isRegexp   bool

	// matchCost is a cost for matching a filter against a single string.
	matchCost uint64

	// contains the prefix for regexp filter if isRegexp==true.
	regexpPrefix string

	// Prefix contains either {nsPrefixTagToMetricIDs, AccountID, ProjectID, key} or {nsPrefixDateTagToMetricIDs, AccountID, ProjectID, date, key}.
	// Additionally it contains:
	//  - value if !isRegexp.
	//  - regexpPrefix if isRegexp.
	prefix []byte

	// `or` values obtained from regexp suffix if it equals to "foo|bar|..."
	//
	// the regexp prefix is stored in regexpPrefix.
	//
	// This array is also populated with matching Graphite metrics if key="__graphite__"
	orSuffixes []string

	// Matches regexp suffix.
	reSuffixMatch func(b []byte) bool

	// Set to true for filters matching empty value.
	isEmptyMatch bool

	// Contains reverse suffix for Graphite wildcard.
	// I.e. for `{__name__=~"foo\\.[^.]*\\.bar\\.baz"}` the value will be `zab.rab.`
	graphiteReverseSuffix []byte
}

func (tf *tagFilter) isComposite() bool {
	k := tf.key
	return len(k) > 0 && k[0] == compositeTagKeyPrefix
}

func (tf *tagFilter) Less(other *tagFilter) bool {
	// Move composite filters to the top, since they usually match lower number of time series.
	// Move regexp filters to the bottom, since they require scanning all the entries for the given label.
	isCompositeA := tf.isComposite()
	isCompositeB := tf.isComposite()
	if isCompositeA != isCompositeB {
		return isCompositeA
	}
	if tf.matchCost != other.matchCost {
		return tf.matchCost < other.matchCost
	}
	if tf.isRegexp != other.isRegexp {
		return !tf.isRegexp
	}
	if len(tf.orSuffixes) != len(other.orSuffixes) {
		return len(tf.orSuffixes) < len(other.orSuffixes)
	}
	if tf.isNegative != other.isNegative {
		return !tf.isNegative
	}
	return bytes.Compare(tf.prefix, other.prefix) < 0
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
	key := tf.key
	if len(key) == 0 {
		key = []byte("__name__")
	}
	return fmt.Sprintf("%s%s%q", key, op, tf.value)
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

// InitFromGraphiteQuery initializes tf from the given graphite query expanded to the given paths.
func (tf *tagFilter) InitFromGraphiteQuery(commonPrefix, query []byte, paths []string, isNegative bool) {
	if len(paths) == 0 {
		// explicitly add empty path in order match zero metric names.
		paths = []string{""}
	}
	prefix, orSuffixes := getCommonPrefix(paths)
	if len(orSuffixes) == 0 {
		orSuffixes = append(orSuffixes, "")
	}
	// Sort orSuffixes for faster seek later.
	sort.Strings(orSuffixes)

	tf.key = append(tf.key[:0], "__graphite__"...)
	tf.value = append(tf.value[:0], query...)
	tf.isNegative = isNegative
	tf.isRegexp = true // this is needed for tagFilter.matchSuffix
	tf.regexpPrefix = prefix
	tf.prefix = append(tf.prefix[:0], commonPrefix...)
	tf.prefix = marshalTagValue(tf.prefix, nil)
	tf.prefix = marshalTagValueNoTrailingTagSeparator(tf.prefix, []byte(prefix))
	tf.orSuffixes = append(tf.orSuffixes[:0], orSuffixes...)
	tf.reSuffixMatch, tf.matchCost = newMatchFuncForOrSuffixes(orSuffixes)
}

func getCommonPrefix(ss []string) (string, []string) {
	if len(ss) == 0 {
		return "", nil
	}
	prefix := ss[0]
	for _, s := range ss[1:] {
		i := 0
		for i < len(s) && i < len(prefix) && s[i] == prefix[i] {
			i++
		}
		prefix = prefix[:i]
		if len(prefix) == 0 {
			return "", ss
		}
	}
	result := make([]string, len(ss))
	for i, s := range ss {
		result[i] = s[len(prefix):]
	}
	return prefix, result
}

// Init initializes the tag filter for the given commonPrefix, key and value.
//
// commonPrefix must contain either {nsPrefixTagToMetricIDs, AccountID, ProjectID} or {nsPrefixDateTagToMetricIDs, AccountID, ProjectID, date}.
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
	tf.matchCost = 0

	tf.regexpPrefix = ""
	tf.prefix = tf.prefix[:0]

	tf.orSuffixes = tf.orSuffixes[:0]
	tf.reSuffixMatch = nil
	tf.isEmptyMatch = false
	tf.graphiteReverseSuffix = tf.graphiteReverseSuffix[:0]

	tf.prefix = append(tf.prefix, commonPrefix...)
	tf.prefix = marshalTagValue(tf.prefix, key)

	var expr []byte
	prefix := tf.value
	if tf.isRegexp {
		prefix, expr = getRegexpPrefix(tf.value)
		if len(expr) == 0 {
			tf.value = append(tf.value[:0], prefix...)
			tf.isRegexp = false
		} else {
			tf.regexpPrefix = string(prefix)
		}
	}
	tf.prefix = marshalTagValueNoTrailingTagSeparator(tf.prefix, prefix)
	if !tf.isRegexp {
		// tf contains plain value without regexp.
		// Add empty orSuffix in order to trigger fast path for orSuffixes
		// during the search for matching metricIDs.
		tf.orSuffixes = append(tf.orSuffixes[:0], "")
		tf.isEmptyMatch = len(prefix) == 0
		tf.matchCost = fullMatchCost
		return nil
	}
	rcv, err := getRegexpFromCache(expr)
	if err != nil {
		return err
	}
	tf.orSuffixes = append(tf.orSuffixes[:0], rcv.orValues...)
	tf.reSuffixMatch = rcv.reMatch
	tf.matchCost = rcv.reCost
	tf.isEmptyMatch = len(prefix) == 0 && tf.reSuffixMatch(nil)
	if !tf.isNegative && len(key) == 0 && strings.IndexByte(rcv.literalSuffix, '.') >= 0 {
		// Reverse suffix is needed only for non-negative regexp filters on __name__ that contains dots.
		tf.graphiteReverseSuffix = reverseBytes(tf.graphiteReverseSuffix[:0], []byte(rcv.literalSuffix))
	}
	return nil
}

func (tf *tagFilter) match(b []byte) (bool, error) {
	prefix := tf.prefix
	if !bytes.HasPrefix(b, prefix) {
		return tf.isNegative, nil
	}
	ok, err := tf.matchSuffix(b[len(prefix):])
	if err != nil {
		return false, err
	}
	if !ok {
		return tf.isNegative, nil
	}
	return !tf.isNegative, nil
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

	regexpCacheLock.RLock()
	rcv, ok := regexpCacheMap[string(expr)]
	regexpCacheLock.RUnlock()
	if ok {
		// Fast path - the regexp found in the cache.
		return rcv, nil
	}

	// Slow path - build the regexp.
	atomic.AddUint64(&regexpCacheMisses, 1)
	exprOrig := string(expr)

	expr = []byte(tagCharsRegexpEscaper.Replace(exprOrig))
	exprStr := fmt.Sprintf("^(%s)$", expr)
	re, err := regexp.Compile(exprStr)
	if err != nil {
		return rcv, fmt.Errorf("invalid regexp %q: %w", exprStr, err)
	}

	sExpr := string(expr)
	orValues := getOrValues(sExpr)
	var reMatch func(b []byte) bool
	var reCost uint64
	var literalSuffix string
	if len(orValues) > 0 {
		reMatch, reCost = newMatchFuncForOrSuffixes(orValues)
	} else {
		reMatch, literalSuffix, reCost = getOptimizedReMatchFunc(re.Match, sExpr)
	}

	// Put the reMatch in the cache.
	rcv.orValues = orValues
	rcv.reMatch = reMatch
	rcv.reCost = reCost
	rcv.literalSuffix = literalSuffix

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

func newMatchFuncForOrSuffixes(orValues []string) (reMatch func(b []byte) bool, reCost uint64) {
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
	reCost = uint64(len(orValues)) * literalMatchCost
	return reMatch, reCost
}

// getOptimizedReMatchFunc tries returning optimized function for matching the given expr.
//   '.*'
//   '.+'
//   'literal.*'
//   'literal.+'
//   '.*literal'
//   '.+literal
//   '.*literal.*'
//   '.*literal.+'
//   '.+literal.*'
//   '.+literal.+'
//
// It returns reMatch if it cannot find optimized function.
//
// It also returns literal suffix from the expr.
func getOptimizedReMatchFunc(reMatch func(b []byte) bool, expr string) (func(b []byte) bool, string, uint64) {
	sre, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing verified expr=%q: %s", expr, err)
	}
	if matchFunc, literalSuffix, reCost := getOptimizedReMatchFuncExt(reMatch, sre); matchFunc != nil {
		// Found optimized function for matching the expr.
		suffixUnescaped := tagCharsReverseRegexpEscaper.Replace(literalSuffix)
		return matchFunc, suffixUnescaped, reCost
	}
	// Fall back to un-optimized reMatch.
	return reMatch, "", reMatchCost
}

// These cost values are used for sorting tag filters in ascending order or the required CPU time for execution.
//
// These values are obtained from BenchmarkOptimizedReMatchCost benchmark.
const (
	fullMatchCost    = 1
	prefixMatchCost  = 2
	literalMatchCost = 3
	suffixMatchCost  = 4
	middleMatchCost  = 6
	reMatchCost      = 100
)

func getOptimizedReMatchFuncExt(reMatch func(b []byte) bool, sre *syntax.Regexp) (func(b []byte) bool, string, uint64) {
	if isDotStar(sre) {
		// '.*'
		return func(b []byte) bool {
			return true
		}, "", fullMatchCost
	}
	if isDotPlus(sre) {
		// '.+'
		return func(b []byte) bool {
			return len(b) > 0
		}, "", fullMatchCost
	}
	switch sre.Op {
	case syntax.OpCapture:
		// Remove parenthesis from expr, i.e. '(expr) -> expr'
		return getOptimizedReMatchFuncExt(reMatch, sre.Sub[0])
	case syntax.OpLiteral:
		if !isLiteral(sre) {
			return nil, "", 0
		}
		s := string(sre.Rune)
		// Literal match
		return func(b []byte) bool {
			return string(b) == s
		}, s, literalMatchCost
	case syntax.OpConcat:
		if len(sre.Sub) == 2 {
			if isLiteral(sre.Sub[0]) {
				prefix := []byte(string(sre.Sub[0].Rune))
				if isDotStar(sre.Sub[1]) {
					// 'prefix.*'
					return func(b []byte) bool {
						return bytes.HasPrefix(b, prefix)
					}, "", prefixMatchCost
				}
				if isDotPlus(sre.Sub[1]) {
					// 'prefix.+'
					return func(b []byte) bool {
						return len(b) > len(prefix) && bytes.HasPrefix(b, prefix)
					}, "", prefixMatchCost
				}
			}
			if isLiteral(sre.Sub[1]) {
				suffix := []byte(string(sre.Sub[1].Rune))
				if isDotStar(sre.Sub[0]) {
					// '.*suffix'
					return func(b []byte) bool {
						return bytes.HasSuffix(b, suffix)
					}, string(suffix), suffixMatchCost
				}
				if isDotPlus(sre.Sub[0]) {
					// '.+suffix'
					return func(b []byte) bool {
						return len(b) > len(suffix) && bytes.HasSuffix(b[1:], suffix)
					}, string(suffix), suffixMatchCost
				}
			}
		}
		if len(sre.Sub) == 3 && isLiteral(sre.Sub[1]) {
			middle := []byte(string(sre.Sub[1].Rune))
			if isDotStar(sre.Sub[0]) {
				if isDotStar(sre.Sub[2]) {
					// '.*middle.*'
					return func(b []byte) bool {
						return bytes.Contains(b, middle)
					}, "", middleMatchCost
				}
				if isDotPlus(sre.Sub[2]) {
					// '.*middle.+'
					return func(b []byte) bool {
						return len(b) > len(middle) && bytes.Contains(b[:len(b)-1], middle)
					}, "", middleMatchCost
				}
			}
			if isDotPlus(sre.Sub[0]) {
				if isDotStar(sre.Sub[2]) {
					// '.+middle.*'
					return func(b []byte) bool {
						return len(b) > len(middle) && bytes.Contains(b[1:], middle)
					}, "", middleMatchCost
				}
				if isDotPlus(sre.Sub[2]) {
					// '.+middle.+'
					return func(b []byte) bool {
						return len(b) > len(middle)+1 && bytes.Contains(b[1:len(b)-1], middle)
					}, "", middleMatchCost
				}
			}
		}
		// Verify that the string matches all the literals found in the regexp
		// before applying the regexp.
		// This should optimize the case when the regexp doesn't match the string.
		var literals [][]byte
		for _, sub := range sre.Sub {
			if isLiteral(sub) {
				literals = append(literals, []byte(string(sub.Rune)))
			}
		}
		var suffix []byte
		if isLiteral(sre.Sub[len(sre.Sub)-1]) {
			suffix = literals[len(literals)-1]
			literals = literals[:len(literals)-1]
		}
		return func(b []byte) bool {
			if len(suffix) > 0 && !bytes.HasSuffix(b, suffix) {
				// Fast path - b has no the given suffix
				return false
			}
			bOrig := b
			for _, literal := range literals {
				n := bytes.Index(b, literal)
				if n < 0 {
					// Fast path - b doesn't match the regexp.
					return false
				}
				b = b[n+len(literal):]
			}
			// Fall back to slow path.
			return reMatch(bOrig)
		}, string(suffix), reMatchCost
	default:
		return nil, "", 0
	}
}

func isDotStar(sre *syntax.Regexp) bool {
	switch sre.Op {
	case syntax.OpCapture:
		return isDotStar(sre.Sub[0])
	case syntax.OpAlternate:
		for _, reSub := range sre.Sub {
			if isDotStar(reSub) {
				return true
			}
		}
		return false
	case syntax.OpStar:
		switch sre.Sub[0].Op {
		case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func isDotPlus(sre *syntax.Regexp) bool {
	switch sre.Op {
	case syntax.OpCapture:
		return isDotPlus(sre.Sub[0])
	case syntax.OpAlternate:
		for _, reSub := range sre.Sub {
			if isDotPlus(reSub) {
				return true
			}
		}
		return false
	case syntax.OpPlus:
		switch sre.Sub[0].Op {
		case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func isLiteral(sre *syntax.Regexp) bool {
	if sre.Op == syntax.OpCapture {
		return isLiteral(sre.Sub[0])
	}
	return sre.Op == syntax.OpLiteral && sre.Flags&syntax.FoldCase == 0
}

func getOrValues(expr string) []string {
	sre, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing verified expr=%q: %s", expr, err)
	}
	orValues := getOrValuesExt(sre)

	// Sort orValues for faster index seek later
	sort.Strings(orValues)

	return orValues
}

func getOrValuesExt(sre *syntax.Regexp) []string {
	switch sre.Op {
	case syntax.OpCapture:
		return getOrValuesExt(sre.Sub[0])
	case syntax.OpLiteral:
		if !isLiteral(sre) {
			return nil
		}
		return []string{string(sre.Rune)}
	case syntax.OpEmptyMatch:
		return []string{""}
	case syntax.OpAlternate:
		a := make([]string, 0, len(sre.Sub))
		for _, reSub := range sre.Sub {
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
		a := make([]string, 0, len(sre.Rune)/2)
		for i := 0; i < len(sre.Rune); i += 2 {
			start := sre.Rune[i]
			end := sre.Rune[i+1]
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
		if len(sre.Sub) < 1 {
			return []string{""}
		}
		prefixes := getOrValuesExt(sre.Sub[0])
		if len(prefixes) == 0 {
			return nil
		}
		sre.Sub = sre.Sub[1:]
		suffixes := getOrValuesExt(sre)
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
	"\\x00", "\\x000", // escapeChar
	"\x00", "\\x000", // escapeChar
	"\\x01", "\\x001", // tagSeparatorChar
	"\x01", "\\x001", // tagSeparatorChar
	"\\x02", "\\x002", // kvSeparatorChar
	"\x02", "\\x002", // kvSeparatorChar
)

var tagCharsReverseRegexpEscaper = strings.NewReplacer(
	"\\x000", "\x00", // escapeChar
	"\x000", "\x00", // escapeChar
	"\\x001", "\x01", // tagSeparatorChar
	"\x001", "\x01", // tagSeparatorChar
	"\\x002", "\x02", // kvSeparatorChar
	"\x002", "\x02", // kvSeparatorChar
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
	orValues      []string
	reMatch       func(b []byte) bool
	reCost        uint64
	literalSuffix string
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
	sre, err := syntax.Parse(string(b), syntax.Perl)
	if err != nil {
		// Cannot parse the regexp. Return it all as prefix.
		return b, nil
	}
	sre = simplifyRegexp(sre)
	if sre == emptyRegexp {
		return nil, nil
	}
	if isLiteral(sre) {
		return []byte(string(sre.Rune)), nil
	}
	var prefix []byte
	if sre.Op == syntax.OpConcat {
		sub0 := sre.Sub[0]
		if isLiteral(sub0) {
			prefix = []byte(string(sub0.Rune))
			sre.Sub = sre.Sub[1:]
			if len(sre.Sub) == 0 {
				return nil, nil
			}
		}
	}
	if _, err := syntax.Compile(sre); err != nil {
		// Cannot compile the regexp. Return it all as prefix.
		return b, nil
	}
	return prefix, []byte(sre.String())
}

func simplifyRegexp(sre *syntax.Regexp) *syntax.Regexp {
	s := sre.String()
	for {
		sre = simplifyRegexpExt(sre, false, false)
		sre = sre.Simplify()
		if sre.Op == syntax.OpBeginText || sre.Op == syntax.OpEndText {
			sre = emptyRegexp
		}
		sNew := sre.String()
		if sNew == s {
			return sre
		}
		var err error
		sre, err = syntax.Parse(sNew, syntax.Perl)
		if err != nil {
			logger.Panicf("BUG: cannot parse simplified regexp %q: %s", sNew, err)
		}
		s = sNew
	}
}

func simplifyRegexpExt(sre *syntax.Regexp, hasPrefix, hasSuffix bool) *syntax.Regexp {
	switch sre.Op {
	case syntax.OpCapture:
		// Substitute all the capture regexps with non-capture regexps.
		sre.Op = syntax.OpAlternate
		sre.Sub[0] = simplifyRegexpExt(sre.Sub[0], hasPrefix, hasSuffix)
		if sre.Sub[0] == emptyRegexp {
			return emptyRegexp
		}
		return sre
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		sre.Sub[0] = simplifyRegexpExt(sre.Sub[0], hasPrefix, hasSuffix)
		if sre.Sub[0] == emptyRegexp {
			return emptyRegexp
		}
		return sre
	case syntax.OpAlternate:
		// Do not remove empty captures from OpAlternate, since this may break regexp.
		for i, sub := range sre.Sub {
			sre.Sub[i] = simplifyRegexpExt(sub, hasPrefix, hasSuffix)
		}
		return sre
	case syntax.OpConcat:
		subs := sre.Sub[:0]
		for i, sub := range sre.Sub {
			if sub = simplifyRegexpExt(sub, i > 0, i+1 < len(sre.Sub)); sub != emptyRegexp {
				subs = append(subs, sub)
			}
		}
		sre.Sub = subs
		// Remove anchros from the beginning and the end of regexp, since they
		// will be added later.
		if !hasPrefix {
			for len(sre.Sub) > 0 && sre.Sub[0].Op == syntax.OpBeginText {
				sre.Sub = sre.Sub[1:]
			}
		}
		if !hasSuffix {
			for len(sre.Sub) > 0 && sre.Sub[len(sre.Sub)-1].Op == syntax.OpEndText {
				sre.Sub = sre.Sub[:len(sre.Sub)-1]
			}
		}
		if len(sre.Sub) == 0 {
			return emptyRegexp
		}
		return sre
	case syntax.OpEmptyMatch:
		return emptyRegexp
	default:
		return sre
	}
}

var emptyRegexp = &syntax.Regexp{
	Op: syntax.OpEmptyMatch,
}
