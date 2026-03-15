package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// filterEqualsCommonCase matches words and phrases where every captial letter
// can be replaced with a small letter, plus all capital words.
//
// Example LogsQL: `equals_common_case("Error")` is equivalent to in("Error", "error", "ERROR")
type filterEqualsCommonCase struct {
	phrases []string

	equalsAny *filterIn
}

func newFilterEqualsCommonCase(fieldName string, phrases []string) (*filterEqualsCommonCase, error) {
	commonCasePhrases, err := getCommonCasePhrases(phrases)
	if err != nil {
		return nil, err
	}

	fi := &filterEqualsCommonCase{
		phrases: phrases,
		equalsAny: &filterIn{
			fieldName: fieldName,
		},
	}
	fi.equalsAny.values.values = commonCasePhrases

	return fi, nil
}

func (fi *filterEqualsCommonCase) String() string {
	a := make([]string, len(fi.phrases))
	for i, phrase := range fi.phrases {
		a[i] = quoteTokenIfNeeded(phrase)
	}
	phrases := strings.Join(a, ",")
	return fmt.Sprintf("%sequals_common_case(%s)", quoteFieldNameIfNeeded(fi.equalsAny.fieldName), phrases)
}

func (fi *filterEqualsCommonCase) updateNeededFields(pf *prefixfilter.Filter) {
	fi.equalsAny.updateNeededFields(pf)
}

func (fi *filterEqualsCommonCase) matchRow(fields []Field) bool {
	return fi.equalsAny.matchRow(fields)
}

func (fi *filterEqualsCommonCase) applyToBlockResult(br *blockResult, bm *bitmap) {
	fi.equalsAny.applyToBlockResult(br, bm)
}

func (fi *filterEqualsCommonCase) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fi.equalsAny.applyToBlockSearch(bs, bm)
}
