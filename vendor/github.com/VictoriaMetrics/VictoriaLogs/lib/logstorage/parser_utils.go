package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

func parseFieldNamesInParens(lex *lexer) ([]string, error) {
	fieldNames, err := parseFieldFiltersInParens(lex)
	if err != nil {
		return nil, err
	}
	for _, fieldName := range fieldNames {
		if prefixfilter.IsWildcardFilter(fieldName) {
			return nil, fmt.Errorf("the field name %q cannot end with '*'", fieldName)
		}
	}
	return fieldNames, nil
}

func parseFieldFiltersInParens(lex *lexer) ([]string, error) {
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing `(`")
	}
	var fields []string
	for {
		lex.nextToken()
		if lex.isKeyword(")") {
			lex.nextToken()
			return fields, nil
		}
		if lex.isKeyword(",") {
			return nil, fmt.Errorf("unexpected `,`")
		}
		field, err := parseFieldFilter(lex)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
		switch {
		case lex.isKeyword(")"):
			lex.nextToken()
			return fields, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',' or ')'", lex.token)
		}
	}
}

func parseCommaSeparatedFieldNames(lex *lexer) ([]string, error) {
	fieldNames, err := parseCommaSeparatedFieldFilters(lex)
	if err != nil {
		return nil, err
	}
	for _, fieldName := range fieldNames {
		if prefixfilter.IsWildcardFilter(fieldName) {
			return nil, fmt.Errorf("the field name %q cannot end with '*'", fieldName)
		}
	}
	return fieldNames, nil
}

func parseCommaSeparatedFieldFilters(lex *lexer) ([]string, error) {
	var fields []string
	for {
		field, err := parseFieldFilter(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse field name: %w", err)
		}
		fields = append(fields, field)
		if !lex.isKeyword(",") {
			return fields, nil
		}
		lex.nextToken()
	}
}

func parseFieldName(lex *lexer) (string, error) {
	fieldName, err := lex.nextCompoundToken()
	if err != nil {
		return "", err
	}
	fieldName = getCanonicalColumnName(fieldName)
	return fieldName, nil
}

func parseFieldFilter(lex *lexer) (string, error) {
	if lex.isKeyword("*") {
		lex.nextToken()
		return "*", nil
	}

	fieldName, err := lex.nextCompoundToken()
	if err != nil {
		return "", err
	}
	fieldName = getCanonicalColumnName(fieldName)
	if !lex.isSkippedSpace && lex.isKeyword("*") {
		lex.nextToken()
		fieldName += "*"
	}

	return fieldName, nil
}

func fieldNamesString(fieldNames []string) string {
	a := make([]string, len(fieldNames))
	for i, f := range fieldNames {
		a[i] = quoteTokenIfNeeded(f)
	}
	return strings.Join(a, ", ")
}

func fieldFiltersString(fieldFilters []string) string {
	a := make([]string, len(fieldFilters))
	for i, f := range fieldFilters {
		a[i] = quoteFieldFilterIfNeeded(f)
	}
	return strings.Join(a, ", ")
}
