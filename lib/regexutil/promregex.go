package regexutil

import (
	"regexp"
	"regexp/syntax"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// PromRegex implements an optimized string matching for Prometheus-like regex.
//
// The following regexs are optimized:
//
// - plain string such as "foobar"
// - alternate strings such as "foo|bar|baz"
// - prefix match such as "foo.*" or "foo.+"
// - substring match such as ".*foo.*" or ".+bar.+"
//
// The rest of regexps are also optimized by returning cached match results for the same input strings.
type PromRegex struct {
	// exprStr is the original expression.
	exprStr string

	// prefix contains literal prefix for regex.
	// For example, prefix="foo" for regex="foo(a|b)"
	prefix string

	// isOnlyPrefix is set to true if the regex contains only the prefix.
	isOnlyPrefix bool

	// isSuffixDotStar is set to true if suffix is ".*"
	isSuffixDotStar bool

	// isSuffixDotPlus is set to true if suffix is ".+"
	isSuffixDotPlus bool

	// substrDotStar contains literal string for regex suffix=".*string.*"
	substrDotStar string

	// substrDotPlus contains literal string for regex suffix=".+string.+"
	substrDotPlus string

	// orValues contains or values for the suffix regex.
	// For example, orValues contain ["foo","bar","baz"] for regex="foo|bar|baz"
	orValues []string

	// reSuffixMatcher contains fast matcher for "^suffix$"
	reSuffixMatcher *bytesutil.FastStringMatcher
}

// NewPromRegex returns PromRegex for the given expr.
func NewPromRegex(expr string) (*PromRegex, error) {
	if _, err := regexp.Compile(expr); err != nil {
		return nil, err
	}
	prefix, suffix := SimplifyPromRegex(expr)
	sre := mustParseRegexp(suffix)
	orValues := getOrValues(sre)
	isOnlyPrefix := len(orValues) == 1 && orValues[0] == ""
	isSuffixDotStar := isDotOp(sre, syntax.OpStar)
	isSuffixDotPlus := isDotOp(sre, syntax.OpPlus)
	substrDotStar := getSubstringLiteral(sre, syntax.OpStar)
	substrDotPlus := getSubstringLiteral(sre, syntax.OpPlus)
	// It is expected that Optimize returns valid regexp in suffix, so use MustCompile here.
	// Anchor suffix to the beginning and the end of the matching string.
	suffixExpr := "^(?:" + suffix + ")$"
	reSuffix := regexp.MustCompile(suffixExpr)
	reSuffixMatcher := bytesutil.NewFastStringMatcher(reSuffix.MatchString)
	pr := &PromRegex{
		exprStr:         expr,
		prefix:          prefix,
		isOnlyPrefix:    isOnlyPrefix,
		isSuffixDotStar: isSuffixDotStar,
		isSuffixDotPlus: isSuffixDotPlus,
		substrDotStar:   substrDotStar,
		substrDotPlus:   substrDotPlus,
		orValues:        orValues,
		reSuffixMatcher: reSuffixMatcher,
	}
	return pr, nil
}

// MatchString returns true if s matches pr.
//
// The pr is automatically anchored to the beginning and to the end
// of the matching string with '^' and '$'.
func (pr *PromRegex) MatchString(s string) bool {
	if pr.isOnlyPrefix {
		return s == pr.prefix
	}

	if len(pr.prefix) > 0 {
		if !strings.HasPrefix(s, pr.prefix) {
			// Fast path - s has another prefix than pr.
			return false
		}
		s = s[len(pr.prefix):]
	}

	if pr.isSuffixDotStar {
		// Fast path - the pr contains "prefix.*"
		return true
	}
	if pr.isSuffixDotPlus {
		// Fast path - the pr contains "prefix.+"
		return len(s) > 0
	}
	if pr.substrDotStar != "" {
		// Fast path - pr contains ".*someText.*"
		return strings.Contains(s, pr.substrDotStar)
	}
	if pr.substrDotPlus != "" {
		// Fast path - pr contains ".+someText.+"
		n := strings.Index(s, pr.substrDotPlus)
		return n > 0 && n+len(pr.substrDotPlus) < len(s)
	}

	if len(pr.orValues) > 0 {
		// Fast path - pr contains only alternate strings such as 'foo|bar|baz'
		for _, v := range pr.orValues {
			if s == v {
				return true
			}
		}
		return false
	}

	// Fall back to slow path by matching the original regexp.
	return pr.reSuffixMatcher.Match(s)
}

// String returns string representation of pr.
func (pr *PromRegex) String() string {
	return pr.exprStr
}
