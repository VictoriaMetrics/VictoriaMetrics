package logstorage

import (
	"fmt"
	"slices"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// pipeFields implements '| fields ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#fields-pipe
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

func (pf *pipeFields) canLiveTail() bool {
	return true
}

func (pf *pipeFields) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if pf.containsStar {
		return
	}

	if neededFields.contains("*") {
		// subtract unneeded fields from pf.fields
		neededFields.reset()
		neededFields.addFields(pf.fields)
		for _, f := range unneededFields.getAll() {
			neededFields.remove(f)
		}
	} else {
		// intersect needed fields with pf.fields
		neededFieldsOrig := neededFields.clone()
		neededFields.reset()
		for _, f := range pf.fields {
			if neededFieldsOrig.contains(f) {
				neededFields.add(f)
			}
		}
	}
	unneededFields.reset()
}

func (pf *pipeFields) optimize() {
	// nothing to do
}

func (pf *pipeFields) hasFilterInWithQuery() bool {
	return false
}

func (pf *pipeFields) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pf, nil
}

func (pf *pipeFields) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeFieldsProcessor{
		pf:     pf,
		ppNext: ppNext,
	}
}

type pipeFieldsProcessor struct {
	pf     *pipeFields
	ppNext pipeProcessor
}

func (pfp *pipeFieldsProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	if !pfp.pf.containsStar {
		br.setColumns(pfp.pf.fields)
	}
	pfp.ppNext.writeBlock(workerID, br)
}

func (pfp *pipeFieldsProcessor) flush() error {
	return nil
}

func parsePipeFields(lex *lexer) (*pipeFields, error) {
	if !lex.isKeyword("fields", "keep") {
		return nil, fmt.Errorf("expecting 'fields'; got %q", lex.token)
	}

	var fields []string
	for {
		lex.nextToken()
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse field name: %w", err)
		}
		fields = append(fields, field)
		switch {
		case lex.isKeyword("|", ")", ""):
			if slices.Contains(fields, "*") {
				fields = []string{"*"}
			}
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
