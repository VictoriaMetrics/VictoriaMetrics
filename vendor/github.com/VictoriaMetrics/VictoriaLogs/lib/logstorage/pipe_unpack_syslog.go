package logstorage

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeUnpackSyslog processes '| unpack_syslog ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_syslog-pipe
type pipeUnpackSyslog struct {
	// fromField is the field to unpack syslog fields from
	fromField string

	// the timezone to use when parsing rfc3164 timestamps
	offsetStr      string
	offsetTimezone *time.Location

	// resultPrefix is prefix to add to unpacked field names
	resultPrefix string

	keepOriginalFields bool

	// iff is an optional filter for skipping unpacking syslog
	iff *ifFilter
}

func (pu *pipeUnpackSyslog) String() string {
	s := "unpack_syslog"
	if pu.iff != nil {
		s += " " + pu.iff.String()
	}
	if !isMsgFieldName(pu.fromField) {
		s += " from " + quoteTokenIfNeeded(pu.fromField)
	}
	if pu.offsetStr != "" {
		s += " offset " + pu.offsetStr
	}
	if pu.resultPrefix != "" {
		s += " result_prefix " + quoteTokenIfNeeded(pu.resultPrefix)
	}
	if pu.keepOriginalFields {
		s += " keep_original_fields"
	}
	return s
}

func (pu *pipeUnpackSyslog) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pu, nil
}

func (pu *pipeUnpackSyslog) canLiveTail() bool {
	return true
}

func (pu *pipeUnpackSyslog) updateNeededFields(pf *prefixfilter.Filter) {
	updateNeededFieldsForUnpackPipe(pu.fromField, nil, pu.keepOriginalFields, false, pu.iff, pf)
}

func (pu *pipeUnpackSyslog) hasFilterInWithQuery() bool {
	return pu.iff.hasFilterInWithQuery()
}

func (pu *pipeUnpackSyslog) initFilterInValues(cache *inValuesCache, getFieldValuesFunc getFieldValuesFunc, keepSubquery bool) (pipe, error) {
	iffNew, err := pu.iff.initFilterInValues(cache, getFieldValuesFunc, keepSubquery)
	if err != nil {
		return nil, err
	}
	puNew := *pu
	puNew.iff = iffNew
	return &puNew, nil
}

func (pu *pipeUnpackSyslog) visitSubqueries(visitFunc func(q *Query)) {
	pu.iff.visitSubqueries(visitFunc)
}

func (pu *pipeUnpackSyslog) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	unpackSyslog := func(uctx *fieldsUnpackerContext, s string) {
		year := currentYear.Load()
		p := GetSyslogParser(int(year), pu.offsetTimezone)

		p.Parse(s)
		for _, f := range p.Fields {
			uctx.addField(f.Name, f.Value)
		}

		PutSyslogParser(p)
	}

	return newPipeUnpackProcessor(unpackSyslog, ppNext, pu.fromField, pu.resultPrefix, pu.keepOriginalFields, false, pu.iff)
}

var currentYear atomic.Int64

func init() {
	year := time.Now().UTC().Year()
	currentYear.Store(int64(year))
	go func() {
		for {
			t := time.Now().UTC()
			nextYear := time.Date(t.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
			d := nextYear.Sub(t)
			time.Sleep(d)
			year := time.Now().UTC().Year()
			currentYear.Store(int64(year))
		}
	}()
}

func parsePipeUnpackSyslog(lex *lexer) (pipe, error) {
	if !lex.isKeyword("unpack_syslog") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "unpack_syslog")
	}
	lex.nextToken()

	var iff *ifFilter
	if lex.isKeyword("if") {
		f, err := parseIfFilter(lex)
		if err != nil {
			return nil, err
		}
		iff = f
	}

	fromField := "_msg"
	if !lex.isKeyword("offset", "result_prefix", "keep_original_fields", ")", "|", "") {
		if lex.isKeyword("from") {
			lex.nextToken()
		}
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'from' field name: %w", err)
		}
		fromField = f
	}

	offsetStr := ""
	offsetTimezone := time.Local
	if lex.isKeyword("offset") {
		lex.nextToken()
		s, err := getCompoundToken(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot read 'offset': %w", err)
		}
		offsetStr = s
		nsecs, ok := tryParseDuration(offsetStr)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'offset' from %q", offsetStr)
		}
		secs := nsecs / nsecsPerSecond
		offsetTimezone = time.FixedZone("custom", int(secs))
	}

	resultPrefix := ""
	if lex.isKeyword("result_prefix") {
		lex.nextToken()
		p, err := getCompoundToken(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'result_prefix': %w", err)
		}
		resultPrefix = p
	}

	keepOriginalFields := false
	if lex.isKeyword("keep_original_fields") {
		lex.nextToken()
		keepOriginalFields = true
	}

	pu := &pipeUnpackSyslog{
		fromField:          fromField,
		offsetStr:          offsetStr,
		offsetTimezone:     offsetTimezone,
		resultPrefix:       resultPrefix,
		keepOriginalFields: keepOriginalFields,
		iff:                iff,
	}

	return pu, nil
}
