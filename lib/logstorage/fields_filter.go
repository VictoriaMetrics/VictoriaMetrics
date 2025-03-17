package logstorage

import (
	"strings"
)

// fieldsFilter is a filter for field names.
type fieldsFilter struct {
	fieldNames []string
	wildcards  []string
}

func (ff *fieldsFilter) reset() {
	ff.fieldNames = ff.fieldNames[:0]
	ff.wildcards = ff.wildcards[:0]
}

// add adds the given filters to ff.
//
// If filters end with '*', then they are treated as a wildcard filter, which match fields with the filter prefix ending with '*'
func (ff *fieldsFilter) addMulti(filters []string) {
	for _, filter := range filters {
		ff.add(filter)
	}
}

// add adds the given filter to ff.
//
// If the filter ends with '*', then it is treated as a wildcard filter, which matches fields with the filter prefix ending with '*'
func (ff *fieldsFilter) add(filter string) {
	if len(filter) == 0 || filter[len(filter)-1] != '*' {
		// The filter is a regular field name
		if !ff.match(filter) {
			ff.fieldNames = append(ff.fieldNames, filter)
		}
		return
	}

	// Add a wildcard filter
	wildcard := filter[:len(filter)-1]

	for _, wc := range ff.wildcards {
		if strings.HasPrefix(wildcard, wc) {
			// More strong wildcard is already registered
			return
		}
	}

	// register the wildcard and drop weaker wildcards if any
	newWildcards := ff.wildcards[:0]
	for _, wc := range ff.wildcards {
		if !strings.HasPrefix(wc, wildcard) {
			newWildcards = append(newWildcards, wc)
		}
	}
	ff.wildcards = append(newWildcards, wildcard)

	// drop field names matching the wildcard
	newFieldNames := ff.fieldNames[:0]
	for _, fieldName := range ff.fieldNames {
		if !strings.HasPrefix(fieldName, wildcard) {
			newFieldNames = append(newFieldNames, fieldName)
		}
	}
	ff.fieldNames = newFieldNames
}

// match returns true if fieldName matches ff.
func (ff *fieldsFilter) match(fieldName string) bool {
	if ff == nil {
		// This allows matching against nil ff
		return false
	}

	for _, wildcard := range ff.wildcards {
		if strings.HasPrefix(fieldName, wildcard) {
			return true
		}
	}
	for _, fn := range ff.fieldNames {
		if fn == fieldName {
			return true
		}
	}
	return false
}
