package regexutil

import (
	"regexp/syntax"
	"sort"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// RemoveStartEndAnchors removes '^' at the start of expr and '$' at the end of the expr.
func RemoveStartEndAnchors(expr string) string {
	for strings.HasPrefix(expr, "^") {
		expr = expr[1:]
	}
	for strings.HasSuffix(expr, "$") {
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
	case syntax.OpBeginText, syntax.OpEndText:
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

func isLiteral(sre *syntax.Regexp) bool {
	if sre.Op == syntax.OpCapture {
		return isLiteral(sre.Sub[0])
	}
	return sre.Op == syntax.OpLiteral && sre.Flags&syntax.FoldCase == 0
}

const maxOrValues = 100
