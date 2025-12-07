package influx

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

// Rows contains parsed influx rows.
type Rows struct {
	Rows []Row

	uc unmarshalContext
}

type unmarshalContext struct {
	tagsPool   []Tag
	fieldsPool []Field

	buf []byte

	hasEscapeChars bool

	hasQuotedFields bool
}

// Reset resets rs.
func (rs *Rows) Reset() {
	rs.Rows = rs.Rows[:0]
	rs.uc.reset()
}

func (uc *unmarshalContext) reset() {
	clear(uc.tagsPool)
	uc.tagsPool = uc.tagsPool[:0]

	clear(uc.fieldsPool)
	uc.fieldsPool = uc.fieldsPool[:0]

	uc.buf = uc.buf[:0]

	uc.hasEscapeChars = false
	uc.hasQuotedFields = false
}

func (uc *unmarshalContext) addTag() *Tag {
	if cap(uc.tagsPool) > len(uc.tagsPool) {
		uc.tagsPool = uc.tagsPool[:len(uc.tagsPool)+1]
	} else {
		uc.tagsPool = append(uc.tagsPool, Tag{})
	}
	return &uc.tagsPool[len(uc.tagsPool)-1]
}

func (uc *unmarshalContext) removeLastTag() {
	tag := &uc.tagsPool[len(uc.tagsPool)-1]
	tag.reset()

	uc.tagsPool = uc.tagsPool[:len(uc.tagsPool)-1]
}

func (uc *unmarshalContext) addField() *Field {
	if cap(uc.fieldsPool) > len(uc.fieldsPool) {
		uc.fieldsPool = uc.fieldsPool[:len(uc.fieldsPool)+1]
	} else {
		uc.fieldsPool = append(uc.fieldsPool, Field{})
	}
	return &uc.fieldsPool[len(uc.fieldsPool)-1]
}

func (uc *unmarshalContext) removeLastField() {
	f := &uc.fieldsPool[len(uc.fieldsPool)-1]
	f.reset()

	uc.fieldsPool = uc.fieldsPool[:len(uc.fieldsPool)-1]
}

// Unmarshal unmarshals influx line protocol rows from s.
//
// See https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/
//
// s shouldn't be modified when rs is in use.
//
// if skipInvalidLines=true, then all the invalid lines at s are ignored, the remaining lines are parsed and nil error is always returned.
// if skipInvalidLines=false, then the first parse error is returned.
func (rs *Rows) Unmarshal(s string, skipInvalidLines bool) error {
	rs.Reset()
	return rs.unmarshal(s, skipInvalidLines)
}

// Row is a single influx row.
type Row struct {
	Measurement string
	Tags        []Tag
	Fields      []Field
	Timestamp   int64
}

func (r *Row) reset() {
	r.Measurement = ""
	r.Tags = nil
	r.Fields = nil
	r.Timestamp = 0
}

func (r *Row) unmarshal(s string, uc *unmarshalContext) error {
	r.reset()
	n := nextUnescapedChar(s, ' ', uc)
	if n < 0 {
		return fmt.Errorf("cannot find Whitespace I in %q", s)
	}
	measurementTags := s[:n]
	s = stripLeadingWhitespace(s[n+1:])

	// Parse measurement and tags
	n = nextUnescapedChar(measurementTags, ',', uc)
	if n >= 0 {
		tagsStart := len(uc.tagsPool)
		if err := unmarshalTags(measurementTags[n+1:], uc); err != nil {
			return err
		}
		tags := uc.tagsPool[tagsStart:]
		r.Tags = tags[:len(tags):len(tags)]
		measurementTags = measurementTags[:n]
	}
	r.Measurement = unescapeTagValue(measurementTags, uc)
	// Allow empty r.Measurement. In this case metric name is constructed directly from field keys.

	// Parse fields
	fieldsStart := len(uc.fieldsPool)
	uc.hasQuotedFields = nextUnescapedChar(s, '"', uc) >= 0
	n = nextUnquotedChar(s, ' ', uc)
	if n < 0 {
		// No timestamp.
		if err := unmarshalInfluxFields(s, uc); err != nil {
			return err
		}
		fields := uc.fieldsPool[fieldsStart:]
		r.Fields = fields[:len(fields):len(fields)]
		return nil
	}
	if err := unmarshalInfluxFields(s[:n], uc); err != nil {
		if strings.HasPrefix(s[n+1:], "HTTP/") {
			return fmt.Errorf("please switch from tcp to http protocol for data ingestion; " +
				"do not set `-influxListenAddr` command-line flag, since it is needed for tcp protocol only")
		}
		return err
	}
	r.Fields = uc.fieldsPool[fieldsStart:]
	s = stripLeadingWhitespace(s[n+1:])

	// The timestamp is optional in the InfluxDB line protocol.
	// Whitespace before it may still be present even when the timestamp itself is omitted.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10049
	if len(s) > 0 {
		timestamp, err := fastfloat.ParseInt64(s)
		if err != nil {
			if strings.HasPrefix(s, "HTTP/") {
				return fmt.Errorf("please switch from tcp to http protocol for data ingestion; " +
					"do not set `-influxListenAddr` command-line flag, since it is needed for tcp protocol only")
			}
			return fmt.Errorf("cannot parse timestamp %q: %w", s, err)
		}
		r.Timestamp = timestamp
	}
	return nil
}

// Tag represents influx tag.
type Tag struct {
	Key   string
	Value string
}

func (tag *Tag) reset() {
	tag.Key = ""
	tag.Value = ""
}

func (tag *Tag) unmarshal(s string, uc *unmarshalContext) error {
	tag.reset()
	n := nextUnescapedChar(s, '=', uc)
	if n < 0 {
		return fmt.Errorf("missing tag value for %q", s)
	}
	tag.Key = unescapeTagValue(s[:n], uc)
	tag.Value = unescapeTagValue(s[n+1:], uc)
	return nil
}

// Field represents influx field.
type Field struct {
	Key   string
	Value float64
}

func (f *Field) reset() {
	f.Key = ""
	f.Value = 0
}

func (f *Field) unmarshal(s string, uc *unmarshalContext) error {
	f.reset()
	n := nextUnescapedChar(s, '=', uc)
	if n < 0 {
		return fmt.Errorf("missing field value for %q", s)
	}
	f.Key = unescapeTagValue(s[:n], uc)
	if len(f.Key) == 0 {
		return fmt.Errorf("field key cannot be empty")
	}
	v, err := parseFieldValue(s[n+1:], uc)
	if err != nil {
		return fmt.Errorf("cannot parse field value for %q: %w", f.Key, err)
	}
	f.Value = v
	return nil
}

func (rs *Rows) unmarshal(s string, skipInvalidLines bool) error {
	rs.uc.hasEscapeChars = strings.IndexByte(s, '\\') >= 0
	for len(s) > 0 {
		n := strings.IndexByte(s, '\n')
		if n < 0 {
			// The last line.
			n = len(s)
		}
		if err := rs.unmarshalRow(s[:n]); err != nil {
			if !skipInvalidLines {
				return fmt.Errorf("incorrect influx line %q: %w", s, err)
			}
			logger.Errorf("skipping InfluxDB line %q because of error: %s", s, err)
			invalidLines.Inc()
		}
		if len(s) == n {
			return nil
		}
		s = s[n+1:]
	}
	return nil
}

func (rs *Rows) unmarshalRow(s string) error {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	if len(s) == 0 {
		// Skip empty line
		return nil
	}
	if s[0] == '#' {
		// Skip comment
		return nil
	}

	if cap(rs.Rows) > len(rs.Rows) {
		rs.Rows = rs.Rows[:len(rs.Rows)+1]
	} else {
		rs.Rows = append(rs.Rows, Row{})
	}
	r := &rs.Rows[len(rs.Rows)-1]
	if err := r.unmarshal(s, &rs.uc); err != nil {
		rs.Rows = rs.Rows[:len(rs.Rows)-1]
		return err
	}

	return nil
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="influx"}`)

func unmarshalTags(s string, uc *unmarshalContext) error {
	for {
		tag := uc.addTag()
		n := nextUnescapedChar(s, ',', uc)
		if n < 0 {
			if err := tag.unmarshal(s, uc); err != nil {
				uc.removeLastTag()
				return err
			}
			if len(tag.Key) == 0 || len(tag.Value) == 0 {
				// Skip empty tag
				uc.removeLastTag()
			}
			return nil
		}
		if err := tag.unmarshal(s[:n], uc); err != nil {
			uc.removeLastTag()
			return err
		}
		s = s[n+1:]
		if len(tag.Key) == 0 || len(tag.Value) == 0 {
			// Skip empty tag
			uc.removeLastTag()
		}
	}
}

func unmarshalInfluxFields(s string, uc *unmarshalContext) error {
	for {
		f := uc.addField()
		n := nextUnquotedChar(s, ',', uc)
		if n < 0 {
			if err := f.unmarshal(s, uc); err != nil {
				uc.removeLastField()
				return err
			}
			return nil
		}
		if err := f.unmarshal(s[:n], uc); err != nil {
			uc.removeLastField()
			return err
		}
		s = s[n+1:]
	}
}

func unescapeTagValue(s string, uc *unmarshalContext) string {
	if !uc.hasEscapeChars {
		// Fast path - no escape chars.
		return s
	}
	n := strings.IndexByte(s, '\\')
	if n < 0 {
		return s
	}

	// Slow path. Remove escape chars.
	bufLen := len(uc.buf)
	for {
		uc.buf = append(uc.buf, s[:n]...)
		s = s[n+1:]
		if len(s) == 0 {
			uc.buf = append(uc.buf, '\\')
			return bytesutil.ToUnsafeString(uc.buf[bufLen:])
		}
		ch := s[0]
		if ch != ' ' && ch != ',' && ch != '=' && ch != '\\' {
			uc.buf = append(uc.buf, '\\')
		}
		uc.buf = append(uc.buf, ch)
		s = s[1:]
		n = strings.IndexByte(s, '\\')
		if n < 0 {
			uc.buf = append(uc.buf, s...)
			return bytesutil.ToUnsafeString(uc.buf[bufLen:])
		}
	}
}

func parseFieldValue(s string, uc *unmarshalContext) (float64, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("field value cannot be empty")
	}
	if uc.hasQuotedFields && s[0] == '"' {
		if len(s) < 2 || s[len(s)-1] != '"' {
			return 0, fmt.Errorf("missing closing quote for quoted field value %s", s)
		}
		// Try converting quoted string to number, since sometimes InfluxDB agents
		// send numbers as strings.
		s = s[1 : len(s)-1]
		return fastfloat.ParseBestEffort(s), nil
	}
	ch := s[len(s)-1]
	if ch == 'i' {
		// Integer value
		ss := s[:len(s)-1]
		n, err := fastfloat.ParseInt64(ss)
		if err != nil {
			return 0, err
		}
		return float64(n), nil
	}
	if ch == 'u' {
		// Unsigned integer value
		ss := s[:len(s)-1]
		n, err := fastfloat.ParseUint64(ss)
		if err != nil {
			return 0, err
		}
		return float64(n), nil
	}
	if s == "t" || s == "T" || s == "true" || s == "True" || s == "TRUE" {
		return 1, nil
	}
	if s == "f" || s == "F" || s == "false" || s == "False" || s == "FALSE" {
		return 0, nil
	}
	return fastfloat.ParseBestEffort(s), nil
}

func nextUnescapedChar(s string, ch byte, uc *unmarshalContext) int {
	if !uc.hasEscapeChars {
		// Fast path: just search for ch in s, since s has no escape chars.
		return strings.IndexByte(s, ch)
	}

	sOrig := s
again:
	n := strings.IndexByte(s, ch)
	if n < 0 {
		return -1
	}
	if n == 0 {
		return len(sOrig) - len(s) + n
	}
	if s[n-1] != '\\' {
		return len(sOrig) - len(s) + n
	}
	nOrig := n
	slashes := 0
	for n > 0 && s[n-1] == '\\' {
		slashes++
		n--
	}
	if slashes&1 == 0 {
		return len(sOrig) - len(s) + nOrig
	}
	s = s[nOrig+1:]
	goto again
}

func nextUnquotedChar(s string, ch byte, uc *unmarshalContext) int {
	if !uc.hasQuotedFields {
		return nextUnescapedChar(s, ch, uc)
	}
	sOrig := s
	for {
		n := nextUnescapedChar(s, ch, uc)
		if n < 0 {
			return -1
		}
		if !isInQuote(s[:n], uc) {
			return n + len(sOrig) - len(s)
		}
		s = s[n+1:]
		n = nextUnescapedChar(s, '"', uc)
		if n < 0 {
			return -1
		}
		s = s[n+1:]
	}
}

func isInQuote(s string, uc *unmarshalContext) bool {
	isQuote := false
	for {
		n := nextUnescapedChar(s, '"', uc)
		if n < 0 {
			return isQuote
		}
		isQuote = !isQuote
		s = s[n+1:]
	}
}

func stripLeadingWhitespace(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return s
}
