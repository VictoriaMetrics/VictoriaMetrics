package logstorage

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// filterContainsCommonCase matches words and phrases where every captial letter
// can be replaced with a small letter, plus all capital words.
//
// Example LogsQL: `contains_common_case("Error")` is equivalent to contains_any("Error", "error", "ERROR")
type filterContainsCommonCase struct {
	phrases []string

	containsAny *filterContainsAny
}

func newFilterContainsCommonCase(fieldName string, phrases []string) (*filterContainsCommonCase, error) {
	commonCasePhrases, err := getCommonCasePhrases(phrases)
	if err != nil {
		return nil, err
	}

	fi := &filterContainsCommonCase{
		phrases: phrases,
		containsAny: &filterContainsAny{
			fieldName: fieldName,
		},
	}
	fi.containsAny.values.values = commonCasePhrases

	return fi, nil
}

func (fi *filterContainsCommonCase) String() string {
	a := make([]string, len(fi.phrases))
	for i, phrase := range fi.phrases {
		a[i] = quoteTokenIfNeeded(phrase)
	}
	phrases := strings.Join(a, ",")
	return fmt.Sprintf("%scontains_common_case(%s)", quoteFieldNameIfNeeded(fi.containsAny.fieldName), phrases)
}

func (fi *filterContainsCommonCase) updateNeededFields(pf *prefixfilter.Filter) {
	fi.containsAny.updateNeededFields(pf)
}

func (fi *filterContainsCommonCase) applyToBlockResult(br *blockResult, bm *bitmap) {
	fi.containsAny.applyToBlockResult(br, bm)
}

func (fi *filterContainsCommonCase) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fi.containsAny.applyToBlockSearch(bs, bm)
}

func getCommonCasePhrases(phrases []string) ([]string, error) {
	var dst []string
	for _, phrase := range phrases {
		upper := countUpperRunes(phrase)
		if upper > 10 {
			return nil, fmt.Errorf("too many common_case combinations for the %q; reduce the number of uppercase letters here", phrase)
		}
		dst = appendCommonCasePhrases(dst, "", phrase)
	}

	// Deduplicate dst
	m := make(map[string]struct{}, len(dst))
	for _, s := range dst {
		m[s] = struct{}{}
	}

	dst = dst[:0]
	for s := range m {
		dst = append(dst, s)
	}
	sort.Strings(dst)

	return dst, nil
}

func countUpperRunes(s string) int {
	upper := 0
	for _, c := range s {
		if unicode.IsUpper(c) {
			upper++
		}
	}
	return upper
}

func appendCommonCasePhrases(dst []string, prefix, phrase string) []string {
	dst = append(dst, prefix+phrase)
	dst = append(dst, strings.ToUpper(prefix+phrase))

	for off, c := range phrase {
		if !unicode.IsUpper(c) {
			continue
		}
		charLen := utf8.RuneLen(c)
		if charLen == -1 {
			continue
		}

		cLower := unicode.ToLower(c)

		prefixLocal := prefix + phrase[:off]
		phraseTail := phrase[off+charLen:]

		dst = appendCommonCasePhrases(dst, prefixLocal+string(cLower), phraseTail)
		dst = appendCommonCasePhrases(dst, prefixLocal+string(c), phraseTail)
	}

	return dst
}
