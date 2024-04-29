package logstorage

import (
	"fmt"
	"slices"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type pipeFields struct {
	// fields contains list of fields to fetch
	fields []string

	// whether fields contains star
	containsStar bool
}

func (pf *pipeFields) String() string {
	if len(pf.fields) == 0 {
		logger.Panicf("BUG: pipeFields must contain at least a single field")
	}
	return "fields " + fieldNamesString(pf.fields)
}

func (pf *pipeFields) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &pipeFieldsProcessor{
		pf:     pf,
		ppBase: ppBase,
	}
}

type pipeFieldsProcessor struct {
	pf     *pipeFields
	ppBase pipeProcessor
}

func (fpp *pipeFieldsProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	if fpp.pf.containsStar || areSameBlockColumns(columns, fpp.pf.fields) {
		// Fast path - there is no need in additional transformations before writing the block to ppBase.
		fpp.ppBase.writeBlock(workerID, timestamps, columns)
		return
	}

	// Slow path - construct columns for fpp.pf.fields before writing them to ppBase.
	brs := getBlockRows()
	cs := brs.cs
	for _, f := range fpp.pf.fields {
		values := getBlockColumnValues(columns, f, len(timestamps))
		cs = append(cs, BlockColumn{
			Name:   f,
			Values: values,
		})
	}
	fpp.ppBase.writeBlock(workerID, timestamps, cs)
	brs.cs = cs
	putBlockRows(brs)
}

func (fpp *pipeFieldsProcessor) flush() error {
	return nil
}

func parsePipeFields(lex *lexer) (*pipeFields, error) {
	var fields []string
	for {
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing field name")
		}
		if lex.isKeyword(",") {
			return nil, fmt.Errorf("unexpected ','; expecting field name")
		}
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse field name: %w", err)
		}
		fields = append(fields, field)
		switch {
		case lex.isKeyword("|", ")", ""):
			pf := &pipeFields{
				fields:       fields,
				containsStar: slices.Contains(fields, "*"),
			}
			return pf, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',', '|' or ')'", lex.token)
		}
	}
}
