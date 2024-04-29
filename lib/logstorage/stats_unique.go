package logstorage

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

type statsUniq struct {
	fields       []string
	containsStar bool
	resultName   string
}

func (su *statsUniq) String() string {
	return "uniq(" + fieldNamesString(su.fields) + ") as " + quoteTokenIfNeeded(su.resultName)
}

func (su *statsUniq) neededFields() []string {
	return su.fields
}

func (su *statsUniq) newStatsFuncProcessor() (statsFuncProcessor, int) {
	sup := &statsUniqProcessor{
		su: su,

		m: make(map[string]struct{}),
	}
	return sup, int(unsafe.Sizeof(*sup))
}

type statsUniqProcessor struct {
	su *statsUniq

	m map[string]struct{}

	columnValues [][]string
	keyBuf       []byte
}

func (sup *statsUniqProcessor) updateStatsForAllRows(timestamps []int64, columns []BlockColumn) int {
	fields := sup.su.fields
	m := sup.m

	stateSizeIncrease := 0
	if len(fields) == 0 || sup.su.containsStar {
		// Count unique rows
		keyBuf := sup.keyBuf
		for i := range timestamps {
			seenKey := true
			for _, c := range columns {
				values := c.Values
				if i == 0 || values[i-1] != values[i] {
					seenKey = false
					break
				}
			}
			if seenKey {
				continue
			}

			allEmptyValues := true
			keyBuf = keyBuf[:0]
			for _, c := range columns {
				v := c.Values[i]
				if v != "" {
					allEmptyValues = false
				}
				// Put column name into key, since every block can contain different set of columns for '*' selector.
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(c.Name))
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
			}
			if allEmptyValues {
				// Do not count empty values
				continue
			}
			if _, ok := m[string(keyBuf)]; !ok {
				m[string(keyBuf)] = struct{}{}
				stateSizeIncrease += len(keyBuf) + int(unsafe.Sizeof(""))
			}
		}
		sup.keyBuf = keyBuf
		return stateSizeIncrease
	}
	if len(fields) == 1 {
		// Fast path for a single column
		if idx := getBlockColumnIndex(columns, fields[0]); idx >= 0 {
			values := columns[idx].Values
			for i, v := range values {
				if v == "" {
					// Do not count empty values
					continue
				}
				if i > 0 && values[i-1] == v {
					continue
				}
				if _, ok := m[v]; !ok {
					vCopy := strings.Clone(v)
					m[vCopy] = struct{}{}
					stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
				}
			}
		}
		return stateSizeIncrease
	}

	// Slow path for multiple columns.

	// Pre-calculate column values for byFields in order to speed up building group key in the loop below.
	sup.columnValues = appendBlockColumnValues(sup.columnValues[:0], columns, fields, len(timestamps))
	columnValues := sup.columnValues

	keyBuf := sup.keyBuf
	for i := range timestamps {
		seenKey := true
		for _, values := range columnValues {
			if i == 0 || values[i-1] != values[i] {
				seenKey = false
			}
		}
		if seenKey {
			continue
		}

		allEmptyValues := true
		keyBuf = keyBuf[:0]
		for _, values := range columnValues {
			v := values[i]
			if v != "" {
				allEmptyValues = false
			}
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
		}
		if allEmptyValues {
			// Do not count empty values
			continue
		}
		if _, ok := m[string(keyBuf)]; !ok {
			m[string(keyBuf)] = struct{}{}
			stateSizeIncrease += len(keyBuf) + int(unsafe.Sizeof(""))
		}
	}
	sup.keyBuf = keyBuf
	return stateSizeIncrease
}

func (sup *statsUniqProcessor) updateStatsForRow(timestamps []int64, columns []BlockColumn, rowIdx int) int {
	fields := sup.su.fields
	m := sup.m

	stateSizeIncrease := 0
	if len(fields) == 0 || sup.su.containsStar {
		// Count unique rows
		allEmptyValues := true
		keyBuf := sup.keyBuf[:0]
		for _, c := range columns {
			v := c.Values[rowIdx]
			if v != "" {
				allEmptyValues = false
			}
			// Put column name into key, since every block can contain different set of columns for '*' selector.
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(c.Name))
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
		}
		sup.keyBuf = keyBuf

		if allEmptyValues {
			// Do not count empty values
			return stateSizeIncrease
		}
		if _, ok := m[string(keyBuf)]; !ok {
			m[string(keyBuf)] = struct{}{}
			stateSizeIncrease += len(keyBuf) + int(unsafe.Sizeof(""))
		}
		return stateSizeIncrease
	}
	if len(fields) == 1 {
		// Fast path for a single column
		if idx := getBlockColumnIndex(columns, fields[0]); idx >= 0 {
			v := columns[idx].Values[rowIdx]
			if v == "" {
				// Do not count empty values
				return stateSizeIncrease
			}
			if _, ok := m[v]; !ok {
				vCopy := strings.Clone(v)
				m[vCopy] = struct{}{}
				stateSizeIncrease += len(vCopy) + int(unsafe.Sizeof(vCopy))
			}
		}
		return stateSizeIncrease
	}

	// Slow path for multiple columns.
	allEmptyValues := true
	keyBuf := sup.keyBuf[:0]
	for _, f := range fields {
		v := ""
		if idx := getBlockColumnIndex(columns, f); idx >= 0 {
			v = columns[idx].Values[rowIdx]
		}
		if v != "" {
			allEmptyValues = false
		}
		keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
	}
	sup.keyBuf = keyBuf

	if allEmptyValues {
		// Do not count empty values
		return stateSizeIncrease
	}
	if _, ok := m[string(keyBuf)]; !ok {
		m[string(keyBuf)] = struct{}{}
		stateSizeIncrease += len(keyBuf) + int(unsafe.Sizeof(""))
	}
	return stateSizeIncrease
}

func (sup *statsUniqProcessor) mergeState(sfp statsFuncProcessor) {
	src := sfp.(*statsUniqProcessor)
	m := sup.m
	for k := range src.m {
		m[k] = struct{}{}
	}
}

func (sup *statsUniqProcessor) finalizeStats() (string, string) {
	n := uint64(len(sup.m))
	value := strconv.FormatUint(n, 10)
	return sup.su.resultName, value
}

func parseStatsUniq(lex *lexer) (*statsUniq, error) {
	lex.nextToken()
	fields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'uniq' args: %w", err)
	}
	resultName, err := parseResultName(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse result name: %w", err)
	}
	su := &statsUniq{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
		resultName:   resultName,
	}
	return su, nil
}
