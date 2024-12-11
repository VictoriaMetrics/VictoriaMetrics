package logstorage

import (
	"fmt"
	"unsafe"
)

type statsCountUniqHash struct {
	fields []string
	limit  uint64
}

func (su *statsCountUniqHash) String() string {
	s := "count_uniq_hash(" + statsFuncFieldsToString(su.fields) + ")"
	if su.limit > 0 {
		s += fmt.Sprintf(" limit %d", su.limit)
	}
	return s
}

func (su *statsCountUniqHash) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, su.fields)
}

func (su *statsCountUniqHash) newStatsProcessor() (statsProcessor, int) {
	sup := &statsCountUniqProcessor{
		su: &statsCountUniq{
			fields: su.fields,
			limit:  su.limit,
		},

		mHash: make(map[uint64]struct{}),
	}
	return sup, int(unsafe.Sizeof(*sup))
}

func parseStatsCountUniqHash(lex *lexer) (*statsCountUniqHash, error) {
	fields, err := parseStatsFuncFields(lex, "count_uniq_hash")
	if err != nil {
		return nil, err
	}
	su := &statsCountUniqHash{
		fields: fields,
	}
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s' for 'count_uniq_hash': %w", lex.token, err)
		}
		lex.nextToken()
		su.limit = n
	}
	return su, nil
}
