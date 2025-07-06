package logstorage

import (
	"encoding/base64"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/valyala/quicktemplate"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

// pipeFormat processes '| format ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#format-pipe
type pipeFormat struct {
	formatStr string
	steps     []patternStep

	resultField string

	keepOriginalFields bool
	skipEmptyResults   bool

	// iff is an optional filter for skipping the format func
	iff *ifFilter
}

func (pf *pipeFormat) String() string {
	s := "format"
	if pf.iff != nil {
		s += " " + pf.iff.String()
	}
	s += " " + quoteTokenIfNeeded(pf.formatStr)
	if !isMsgFieldName(pf.resultField) {
		s += " as " + quoteTokenIfNeeded(pf.resultField)
	}
	if pf.keepOriginalFields {
		s += " keep_original_fields"
	}
	if pf.skipEmptyResults {
		s += " skip_empty_results"
	}
	return s
}

func (pf *pipeFormat) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pf, nil
}

func (pf *pipeFormat) canLiveTail() bool {
	return true
}

func (pf *pipeFormat) updateNeededFields(f *prefixfilter.Filter) {
	if f.MatchNothing() {
		if pf.iff != nil {
			f.AddAllowFilters(pf.iff.allowFilters)
		}
		return
	}

	if f.MatchString(pf.resultField) {
		if !pf.keepOriginalFields && !pf.skipEmptyResults {
			f.AddDenyFilter(pf.resultField)
		}
		if pf.iff != nil {
			f.AddAllowFilters(pf.iff.allowFilters)
		}
		for _, step := range pf.steps {
			if step.field != "" {
				f.AddAllowFilter(step.field)
			}
		}
	}
}

func (pf *pipeFormat) hasFilterInWithQuery() bool {
	return pf.iff.hasFilterInWithQuery()
}

func (pf *pipeFormat) initFilterInValues(cache *inValuesCache, getFieldValuesFunc getFieldValuesFunc, keepSubquery bool) (pipe, error) {
	iffNew, err := pf.iff.initFilterInValues(cache, getFieldValuesFunc, keepSubquery)
	if err != nil {
		return nil, err
	}
	pfNew := *pf
	pfNew.iff = iffNew
	return &pfNew, nil
}

func (pf *pipeFormat) visitSubqueries(visitFunc func(q *Query)) {
	pf.iff.visitSubqueries(visitFunc)
}

func (pf *pipeFormat) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeFormatProcessor{
		pf:     pf,
		ppNext: ppNext,
	}
}

type pipeFormatProcessor struct {
	pf     *pipeFormat
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeFormatProcessorShard]
}

type pipeFormatProcessorShard struct {
	bm bitmap

	a  arena
	rc resultColumn
}

func (pfp *pipeFormatProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := pfp.shards.Get(workerID)
	pf := pfp.pf

	bm := &shard.bm
	if iff := pf.iff; iff != nil {
		bm.init(br.rowsLen)
		bm.setBits()
		iff.f.applyToBlockResult(br, bm)
		if bm.isZero() {
			pfp.ppNext.writeBlock(workerID, br)
			return
		}
	}

	shard.rc.name = pf.resultField

	resultColumn := br.getColumnByName(pf.resultField)
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		v := ""
		if pf.iff == nil || bm.isSetBit(rowIdx) {
			v = shard.formatRow(pf, br, rowIdx)
			if v == "" && pf.skipEmptyResults || pf.keepOriginalFields {
				if vOrig := resultColumn.getValueAtRow(br, rowIdx); vOrig != "" {
					v = vOrig
				}
			}
		} else {
			v = resultColumn.getValueAtRow(br, rowIdx)
		}
		shard.rc.addValue(v)
	}

	br.addResultColumn(shard.rc)
	pfp.ppNext.writeBlock(workerID, br)

	shard.a.reset()
	shard.rc.reset()
}

func (pfp *pipeFormatProcessor) flush() error {
	return nil
}

func (shard *pipeFormatProcessorShard) formatRow(pf *pipeFormat, br *blockResult, rowIdx int) string {
	b := shard.a.b
	bLen := len(b)
	for _, step := range pf.steps {
		b = append(b, step.prefix...)
		if step.field == "" {
			continue
		}

		c := br.getColumnByName(step.field)
		v := c.getValueAtRow(br, rowIdx)
		switch step.fieldOpt {
		case "base64decode":
			result, ok := appendBase64Decode(b, v)
			if !ok {
				b = append(b, v...)
			} else {
				b = result
			}
		case "base64encode":
			b = appendBase64Encode(b, v)
		case "duration":
			nsecs, ok := tryParseInt64(v)
			if !ok {
				b = append(b, v...)
			} else {
				b = marshalDurationString(b, nsecs)
			}
		case "duration_seconds":
			nsecs, ok := tryParseDuration(v)
			if !ok {
				b = append(b, v...)
			} else {
				secs := float64(nsecs) / 1e9
				b = marshalFloat64String(b, secs)
			}
		case "hexdecode":
			b = appendHexDecode(b, v)
		case "hexencode":
			b = appendHexEncode(b, v)
		case "hexnumdecode":
			b = appendHexUint64Decode(b, v)
		case "hexnumencode":
			n, ok := tryParseUint64(v)
			if !ok {
				b = append(b, v...)
			} else {
				b = appendHexUint64Encode(b, n)
			}
		case "ipv4":
			ipNum, ok := tryParseUint64(v)
			if !ok || ipNum > math.MaxUint32 {
				b = append(b, v...)
			} else {
				b = marshalIPv4String(b, uint32(ipNum))
			}
		case "lc":
			b = appendLowercase(b, v)
		case "time":
			nsecs, ok := timeutil.TryParseUnixTimestamp(v)
			if !ok {
				b = append(b, v...)
			} else {
				b = marshalTimestampRFC3339NanoString(b, nsecs)
			}
		case "q":
			b = quicktemplate.AppendJSONString(b, v, true)
		case "uc":
			b = appendUppercase(b, v)
		case "urldecode":
			b = appendURLDecode(b, v)
		case "urlencode":
			b = appendURLEncode(b, v)
		default:
			b = append(b, v...)
		}
	}
	shard.a.b = b

	return bytesutil.ToUnsafeString(b[bLen:])
}

func parsePipeFormat(lex *lexer) (pipe, error) {
	if !lex.isKeyword("format") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "format")
	}
	lex.nextToken()

	// parse optional if (...)
	var iff *ifFilter
	if lex.isKeyword("if") {
		f, err := parseIfFilter(lex)
		if err != nil {
			return nil, err
		}
		iff = f
	}

	// parse format
	formatStr, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot read 'format': %w", err)
	}
	steps, err := parsePatternSteps(formatStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'pattern' %q: %w", formatStr, err)
	}

	// Verify that all the fields mentioned in the format pattern do not end with '*'
	for _, step := range steps {
		if prefixfilter.IsWildcardFilter(step.field) {
			return nil, fmt.Errorf("'pattern' %q cannot contain wildcard fields like %q", formatStr, step.field)
		}
	}

	// parse optional 'as ...` part
	resultField := "_msg"
	if lex.isKeyword("as") {
		lex.nextToken()
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result field after 'format %q as': %w", formatStr, err)
		}
		resultField = field
	}

	keepOriginalFields := false
	skipEmptyResults := false
	switch {
	case lex.isKeyword("keep_original_fields"):
		lex.nextToken()
		keepOriginalFields = true
	case lex.isKeyword("skip_empty_results"):
		lex.nextToken()
		skipEmptyResults = true
	}

	pf := &pipeFormat{
		formatStr:          formatStr,
		steps:              steps,
		resultField:        resultField,
		keepOriginalFields: keepOriginalFields,
		skipEmptyResults:   skipEmptyResults,
		iff:                iff,
	}

	return pf, nil
}

func appendUppercase(dst []byte, s string) []byte {
	for _, r := range s {
		r = unicode.ToUpper(r)
		dst = utf8.AppendRune(dst, r)
	}
	return dst
}

func appendLowercase(dst []byte, s string) []byte {
	for _, r := range s {
		r = unicode.ToLower(r)
		dst = utf8.AppendRune(dst, r)
	}
	return dst
}

func appendURLDecode(dst []byte, s string) []byte {
	for len(s) > 0 {
		n := strings.IndexAny(s, "%+")
		if n < 0 {
			return append(dst, s...)
		}
		dst = append(dst, s[:n]...)
		ch := s[n]
		s = s[n+1:]
		if ch == '+' {
			dst = append(dst, ' ')
			continue
		}
		if len(s) < 2 {
			dst = append(dst, '%')
			continue
		}
		hi, ok1 := unhexChar(s[0])
		lo, ok2 := unhexChar(s[1])
		if !ok1 || !ok2 {
			dst = append(dst, '%')
			continue
		}
		ch = (hi << 4) | lo
		dst = append(dst, ch)
		s = s[2:]
	}
	return dst
}

func appendURLEncode(dst []byte, s string) []byte {
	n := len(s)
	for i := 0; i < n; i++ {
		c := s[i]

		// See http://www.w3.org/TR/html5/forms.html#form-submission-algorithm
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '-' || c == '.' || c == '_' {
			dst = append(dst, c)
		} else {
			if c == ' ' {
				dst = append(dst, '+')
			} else {
				dst = append(dst, '%', hexCharUpper(c>>4), hexCharUpper(c&15))
			}
		}
	}
	return dst
}

func hexCharUpper(c byte) byte {
	if c < 10 {
		return '0' + c
	}
	return c - 10 + 'A'
}

func unhexChar(c byte) (byte, bool) {
	if c >= '0' && c <= '9' {
		return c - '0', true
	}
	if c >= 'A' && c <= 'F' {
		return c - 'A' + 10, true
	}
	if c >= 'a' && c <= 'f' {
		return c - 'a' + 10, true
	}
	return 0, false
}

func appendHexUint64Encode(dst []byte, n uint64) []byte {
	for shift := 60; shift >= 0; shift -= 4 {
		dst = append(dst, hexCharUpper(byte(n>>shift)&15))
	}
	return dst
}

func appendHexUint64Decode(dst []byte, s string) []byte {
	if len(s) > 16 {
		return append(dst, s...)
	}
	sOrig := s
	n := uint64(0)
	for len(s) > 0 {
		x, ok := unhexChar(s[0])
		if !ok {
			return append(dst, sOrig...)
		}
		n = (n << 4) | uint64(x)
		s = s[1:]
	}
	return marshalUint64String(dst, n)
}

func appendHexEncode(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		hi := hexCharUpper(c >> 4)
		lo := hexCharUpper(c & 15)
		dst = append(dst, hi, lo)
	}
	return dst
}

func appendHexDecode(dst []byte, s string) []byte {
	for len(s) >= 2 {
		hi, ok1 := unhexChar(s[0])
		lo, ok2 := unhexChar(s[1])
		if !ok1 || !ok2 {
			dst = append(dst, s[0], s[1])
		} else {
			ch := (hi << 4) | lo
			dst = append(dst, ch)
		}
		s = s[2:]
	}
	return append(dst, s...)
}

func appendBase64Encode(dst []byte, s string) []byte {
	return base64.StdEncoding.AppendEncode(dst, bytesutil.ToUnsafeBytes(s))
}

func appendBase64Decode(dst []byte, s string) ([]byte, bool) {
	result, err := base64.StdEncoding.AppendDecode(dst, bytesutil.ToUnsafeBytes(s))
	if err != nil {
		return dst, false
	}
	return result, true
}
