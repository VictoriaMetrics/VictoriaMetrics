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
	// prefix contains literal prefix for regex.
	// For example, prefix="foo" for regex="foo(a|b)"
	prefix string

	// Suffix contains regex suffix left after removing the prefix.
	// For example, suffix="a|b" for regex="foo(a|b)"
	suffix string

	// substrDotStar contains literal string for regex suffix=".*string.*"
	substrDotStar string

	// substrDotPlus contains literal string for regex suffix=".+string.+"
	substrDotPlus string

	// orValues contains or values for the suffix regex.
	// For example, orValues contain ["foo","bar","baz"] for regex suffix="foo|bar|baz"
	orValues []string

	// reSuffixMatcher contains fast matcher for "^suffix$"
	reSuffixMatcher *bytesutil.FastStringMatcher
}

// NewPromRegex returns PromRegex for the given expr.
func NewPromRegex(expr string) (*PromRegex, error) {
	if _, err := regexp.Compile(expr); err != nil {
		return nil, err
	}
	prefix, suffix := Simplify(expr)
	orValues := GetOrValues(suffix)
	substrDotStar := getSubstringLiteral(suffix, ".*")
	substrDotPlus := getSubstringLiteral(suffix, ".+")
	// It is expected that Optimize returns valid regexp in suffix, so use MustCompile here.
	// Anchor suffix to the beginning and the end of the matching string.
	suffixExpr := "^(?:" + suffix + ")$"
	reSuffix := regexp.MustCompile(suffixExpr)
	reSuffixMatcher := bytesutil.NewFastStringMatcher(reSuffix.MatchString)
	pr := &PromRegex{
		prefix:          prefix,
		suffix:          suffix,
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
	if !strings.HasPrefix(s, pr.prefix) {
		// Fast path - s has another prefix than pr.
		return false
	}
	s = s[len(pr.prefix):]
	if len(pr.orValues) > 0 {
		// Fast path - pr contains only alternate strings such as 'foo|bar|baz'
		for _, v := range pr.orValues {
			if s == v {
				return true
			}
		}
		return false
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
	switch pr.suffix {
	case ".*":
		// Fast path - the pr contains "prefix.*"
		return true
	case ".+":
		// Fast path - the pr contains "prefix.+"
		return len(s) > 0
	}
	// Fall back to slow path by matching the original regexp.
	return pr.reSuffixMatcher.Match(s)
}

// getSubstringLiteral returns regex part from expr surrounded by prefixSuffix.
//
// For example, if expr=".+foo.+" and prefixSuffix=".+", then the function returns "foo".
//
// An empty string is returned if expr doesn't contain the given prefixSuffix prefix and suffix
// or if the regex part surrounded by prefixSuffix contains alternate regexps.
func getSubstringLiteral(expr, prefixSuffix string) string {
	// Verify that the expr doesn't contain alternate regexps. In this case it is unsafe removing prefix and suffix.
	sre, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		return ""
	}
	if sre.Op == syntax.OpAlternate {
		return ""
	}

	if !strings.HasPrefix(expr, prefixSuffix) {
		return ""
	}
	expr = expr[len(prefixSuffix):]
	if !strings.HasSuffix(expr, prefixSuffix) {
		return ""
	}
	expr = expr[:len(expr)-len(prefixSuffix)]
	prefix, suffix := Simplify(expr)
	if suffix != "" {
		return ""
	}
	return prefix
}
