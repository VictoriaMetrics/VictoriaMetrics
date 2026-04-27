package logstorage

import (
	"fmt"
	"strings"
)

// filterEqualsCommonCase matches words and phrases where every captial letter
// can be replaced with a small letter, plus all capital words.
//
// Example LogsQL: `equals_common_case("Error")` is equivalent to in("Error", "error", "ERROR")
type filterEqualsCommonCase struct {
	phrases []string

	equalsAny filterIn
}

func newFilterEqualsCommonCase(fieldName string, phrases []string) (*filterGeneric, error) {
	commonCasePhrases, err := getCommonCasePhrases(phrases)
	if err != nil {
		return nil, err
	}

	fi := &filterEqualsCommonCase{
		phrases: phrases,
	}
	fi.equalsAny.values.values = commonCasePhrases

	fg := newFilterGeneric(fieldName, fi)
	return fg, nil
}

func (fi *filterEqualsCommonCase) String() string {
	a := make([]string, len(fi.phrases))
	for i, phrase := range fi.phrases {
		a[i] = quoteTokenIfNeeded(phrase)
	}
	phrases := strings.Join(a, ",")
	return fmt.Sprintf("equals_common_case(%s)", phrases)
}

func (fi *filterEqualsCommonCase) matchRowByField(fields []Field, fieldName string) bool {
	return fi.equalsAny.matchRowByField(fields, fieldName)
}

func (fi *filterEqualsCommonCase) applyToBlockResultByField(br *blockResult, bm *bitmap, fieldName string) {
	fi.equalsAny.applyToBlockResultByField(br, bm, fieldName)
}

func (fi *filterEqualsCommonCase) applyToBlockSearchByField(bs *blockSearch, bm *bitmap, fieldName string) {
	fi.equalsAny.applyToBlockSearchByField(bs, bm, fieldName)
}
