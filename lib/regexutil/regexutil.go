package regexutil

import (
	"fmt"
	"regexp/syntax"
	"sort"
	"strings"
)

// RemoveStartEndAnchors removes '^' at the start of expr and '$' at the end of the expr.
func RemoveStartEndAnchors(expr string) string {
	for strings.HasPrefix(expr, "^") {
		expr = expr[1:]
	}
	for strings.HasSuffix(expr, "$") && !strings.HasSuffix(expr, "\\$") {
		expr = expr[:len(expr)-1]
	}
	return expr
}

// GetOrValues returns "or" values from the given regexp expr.
//
// It ignores start and end anchors ('^') and ('$') at the start and the end of expr.
// It returns ["foo", "bar"] for "foo|bar" regexp.
// It returns ["foo"] for "foo" regexp.
// It returns [""] for "" regexp.
// It returns an empty list if it is impossible to extract "or" values from the regexp.
func GetOrValues(expr string) []string {
	expr = RemoveStartEndAnchors(expr)
	prefix, tailExpr := Simplify(expr)
	if tailExpr == "" {
		return []string{prefix}
	}
	sre, err := syntax.Parse(tailExpr, syntax.Perl)
	if err != nil {
		panic(fmt.Errorf("BUG: unexpected error when parsing verified tailExpr=%q: %w", tailExpr, err))
	}
	orValues := getOrValuesExt(sre)

	// Sort orValues for faster index seek later
	sort.Strings(orValues)

	if len(prefix) > 0 {
		// Add prefix to orValues
		for i, orValue := range orValues {
			orValues[i] = prefix + orValue
		}
	}

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
		if len(sre.Sub) == 1 {
			return prefixes
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

func isLiteral(sre *syntax.Regexp) bool {
	if sre.Op == syntax.OpCapture {
		return isLiteral(sre.Sub[0])
	}
	return sre.Op == syntax.OpLiteral && sre.Flags&syntax.FoldCase == 0
}

const maxOrValues = 100

// Simplify simplifies the given expr.
//
// It returns plaintext prefix and the remaining regular expression
// with dropped '^' and '$' anchors at the beginning and the end
// of the regular expression.
//
// The function removes capturing parens from the expr,
// so it cannot be used when capturing parens are necessary.
func Simplify(expr string) (string, string) {
	sre, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		// Cannot parse the regexp. Return it all as prefix.
		return expr, ""
	}
	sre = simplifyRegexp(sre, false)
	if sre == emptyRegexp {
		return "", ""
	}
	if isLiteral(sre) {
		return string(sre.Rune), ""
	}
	var prefix string
	if sre.Op == syntax.OpConcat {
		sub0 := sre.Sub[0]
		if isLiteral(sub0) {
			prefix = string(sub0.Rune)
			sre.Sub = sre.Sub[1:]
			if len(sre.Sub) == 0 {
				return prefix, ""
			}
			sre = simplifyRegexp(sre, true)
		}
	}
	if _, err := syntax.Compile(sre); err != nil {
		// Cannot compile the regexp. Return it all as prefix.
		return expr, ""
	}
	s := sre.String()
	s = strings.ReplaceAll(s, "(?:)", "")
	s = strings.ReplaceAll(s, "(?-s:.)", ".")
	s = strings.ReplaceAll(s, "(?-m:$)", "$")
	return prefix, s
}

func simplifyRegexp(sre *syntax.Regexp, hasPrefix bool) *syntax.Regexp {
	s := sre.String()
	for {
		sre = simplifyRegexpExt(sre, hasPrefix, false)
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
			panic(fmt.Errorf("BUG: cannot parse simplified regexp %q: %w", sNew, err))
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
			sub = simplifyRegexpExt(sub, hasPrefix || len(subs) > 0, hasSuffix || i+1 < len(sre.Sub))
			if sub != emptyRegexp {
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
		if len(sre.Sub) == 1 {
			return sre.Sub[0]
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
