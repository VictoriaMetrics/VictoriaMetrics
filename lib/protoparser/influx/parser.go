package influx

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

// Rows contains parsed influx rows.
type Rows struct {
	Rows       []Row
	IgnoreErrs bool

	tagsPool   []Tag
	fieldsPool []Field
}

// Reset resets rs.
func (rs *Rows) Reset() {
	// Reset rows, tags and fields in order to remove references to old data,
	// so GC could collect it.

	for i := range rs.Rows {
		rs.Rows[i].reset()
	}
	rs.Rows = rs.Rows[:0]

	for i := range rs.tagsPool {
		rs.tagsPool[i].reset()
	}
	rs.tagsPool = rs.tagsPool[:0]

	for i := range rs.fieldsPool {
		rs.fieldsPool[i].reset()
	}
	rs.fieldsPool = rs.fieldsPool[:0]
}

// Unmarshal unmarshals influx line protocol rows from s.
//
// See https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/
//
// s shouldn't be modified when rs is in use.
func (rs *Rows) Unmarshal(s string) error {
	rs.reset()
	return rs.unmarshal(s)
}

func (rs *Rows) reset() {
	rs.Rows = rs.Rows[:0]
	rs.tagsPool = rs.tagsPool[:0]
	rs.fieldsPool = rs.fieldsPool[:0]
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

func (r *Row) unmarshal(s string, tagsPool []Tag, fieldsPool []Field, noEscapeChars bool) ([]Tag, []Field, error) {
	r.reset()
	n := nextUnescapedChar(s, ' ', noEscapeChars)
	if n < 0 {
		return tagsPool, fieldsPool, fmt.Errorf("cannot find Whitespace I in %q", s)
	}
	measurementTags := s[:n]
	s = stripLeadingWhitespace(s[n+1:])

	// Parse measurement and tags
	var err error
	n = nextUnescapedChar(measurementTags, ',', noEscapeChars)
	if n >= 0 {
		tagsStart := len(tagsPool)
		tagsPool, err = unmarshalTags(tagsPool, measurementTags[n+1:], noEscapeChars)
		if err != nil {
			return tagsPool, fieldsPool, err
		}
		tags := tagsPool[tagsStart:]
		r.Tags = tags[:len(tags):len(tags)]
		measurementTags = measurementTags[:n]
	}
	r.Measurement = unescapeTagValue(measurementTags, noEscapeChars)
	// Allow empty r.Measurement. In this case metric name is constructed directly from field keys.

	// Parse fields
	fieldsStart := len(fieldsPool)
	hasQuotedFields := nextUnescapedChar(s, '"', noEscapeChars) >= 0
	n = nextUnquotedChar(s, ' ', noEscapeChars, hasQuotedFields)
	if n < 0 {
		// No timestamp.
		fieldsPool, err = unmarshalInfluxFields(fieldsPool, s, noEscapeChars, hasQuotedFields)
		if err != nil {
			return tagsPool, fieldsPool, err
		}
		fields := fieldsPool[fieldsStart:]
		r.Fields = fields[:len(fields):len(fields)]
		return tagsPool, fieldsPool, nil
	}
	fieldsPool, err = unmarshalInfluxFields(fieldsPool, s[:n], noEscapeChars, hasQuotedFields)
	if err != nil {
		if strings.HasPrefix(s[n+1:], "HTTP/") {
			return tagsPool, fieldsPool, fmt.Errorf("please switch from tcp to http protocol for data ingestion; " +
				"do not set `-influxListenAddr` command-line flag, since it is needed for tcp protocol only")
		}
		return tagsPool, fieldsPool, err
	}
	r.Fields = fieldsPool[fieldsStart:]
	s = stripLeadingWhitespace(s[n+1:])

	// Parse timestamp
	timestamp, err := fastfloat.ParseInt64(s)
	if err != nil {
		if strings.HasPrefix(s, "HTTP/") {
			return tagsPool, fieldsPool, fmt.Errorf("please switch from tcp to http protocol for data ingestion; " +
				"do not set `-influxListenAddr` command-line flag, since it is needed for tcp protocol only")
		}
		return tagsPool, fieldsPool, fmt.Errorf("cannot parse timestamp %q: %w", s, err)
	}
	r.Timestamp = timestamp
	return tagsPool, fieldsPool, nil
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

func (tag *Tag) unmarshal(s string, noEscapeChars bool) error {
	tag.reset()
	n := nextUnescapedChar(s, '=', noEscapeChars)
	if n < 0 {
		return fmt.Errorf("missing tag value for %q", s)
	}
	tag.Key = unescapeTagValue(s[:n], noEscapeChars)
	tag.Value = unescapeTagValue(s[n+1:], noEscapeChars)
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

func (f *Field) unmarshal(s string, noEscapeChars, hasQuotedFields bool) error {
	f.reset()
	n := nextUnescapedChar(s, '=', noEscapeChars)
	if n < 0 {
		return fmt.Errorf("missing field value for %q", s)
	}
	f.Key = unescapeTagValue(s[:n], noEscapeChars)
	if len(f.Key) == 0 {
		return fmt.Errorf("field key cannot be empty")
	}
	v, err := parseFieldValue(s[n+1:], hasQuotedFields)
	if err != nil {
		return fmt.Errorf("cannot parse field value for %q: %w", f.Key, err)
	}
	f.Value = v
	return nil
}

func (rs *Rows) unmarshal(s string) error {
	noEscapeChars := strings.IndexByte(s, '\\') < 0
	for len(s) > 0 {
		n := strings.IndexByte(s, '\n')
		if n < 0 {
			// The last line.
			n = len(s)
		}
		err := rs.unmarshalRow(s[:n], noEscapeChars)
		if err != nil {
			if !rs.IgnoreErrs {
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

func (rs *Rows) unmarshalRow(s string, noEscapeChars bool) error {
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
	var err error
	rs.tagsPool, rs.fieldsPool, err = r.unmarshal(s, rs.tagsPool, rs.fieldsPool, noEscapeChars)
	if err != nil {
		rs.Rows = rs.Rows[:len(rs.Rows)-1]
	}
	return err
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="influx"}`)

func unmarshalTags(dst []Tag, s string, noEscapeChars bool) ([]Tag, error) {
	for {
		if cap(dst) > len(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, Tag{})
		}
		tag := &dst[len(dst)-1]
		n := nextUnescapedChar(s, ',', noEscapeChars)
		if n < 0 {
			if err := tag.unmarshal(s, noEscapeChars); err != nil {
				return dst[:len(dst)-1], err
			}
			if len(tag.Key) == 0 || len(tag.Value) == 0 {
				// Skip empty tag
				dst = dst[:len(dst)-1]
			}
			return dst, nil
		}
		if err := tag.unmarshal(s[:n], noEscapeChars); err != nil {
			return dst[:len(dst)-1], err
		}
		s = s[n+1:]
		if len(tag.Key) == 0 || len(tag.Value) == 0 {
			// Skip empty tag
			dst = dst[:len(dst)-1]
		}
	}
}

func unmarshalInfluxFields(dst []Field, s string, noEscapeChars, hasQuotedFields bool) ([]Field, error) {
	for {
		if cap(dst) > len(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, Field{})
		}
		f := &dst[len(dst)-1]
		n := nextUnquotedChar(s, ',', noEscapeChars, hasQuotedFields)
		if n < 0 {
			if err := f.unmarshal(s, noEscapeChars, hasQuotedFields); err != nil {
				return dst, err
			}
			return dst, nil
		}
		if err := f.unmarshal(s[:n], noEscapeChars, hasQuotedFields); err != nil {
			return dst, err
		}
		s = s[n+1:]
	}
}

func unescapeTagValue(s string, noEscapeChars bool) string {
	if noEscapeChars {
		// Fast path - no escape chars.
		return s
	}
	n := strings.IndexByte(s, '\\')
	if n < 0 {
		return s
	}

	// Slow path. Remove escape chars.
	dst := make([]byte, 0, len(s))
	for {
		dst = append(dst, s[:n]...)
		s = s[n+1:]
		if len(s) == 0 {
			return string(append(dst, '\\'))
		}
		ch := s[0]
		if ch != ' ' && ch != ',' && ch != '=' && ch != '\\' {
			dst = append(dst, '\\')
		}
		dst = append(dst, ch)
		s = s[1:]
		n = strings.IndexByte(s, '\\')
		if n < 0 {
			return string(append(dst, s...))
		}
	}
}

func parseFieldValue(s string, hasQuotedFields bool) (float64, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("field value cannot be empty")
	}
	if hasQuotedFields && s[0] == '"' {
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

func nextUnescapedChar(s string, ch byte, noEscapeChars bool) int {
	if noEscapeChars {
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

func nextUnquotedChar(s string, ch byte, noEscapeChars, hasQuotedFields bool) int {
	if !hasQuotedFields {
		return nextUnescapedChar(s, ch, noEscapeChars)
	}
	sOrig := s
	for {
		n := nextUnescapedChar(s, ch, noEscapeChars)
		if n < 0 {
			return -1
		}
		if !isInQuote(s[:n], noEscapeChars) {
			return n + len(sOrig) - len(s)
		}
		s = s[n+1:]
		n = nextUnescapedChar(s, '"', noEscapeChars)
		if n < 0 {
			return -1
		}
		s = s[n+1:]
	}
}

func isInQuote(s string, noEscapeChars bool) bool {
	isQuote := false
	for {
		n := nextUnescapedChar(s, '"', noEscapeChars)
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
