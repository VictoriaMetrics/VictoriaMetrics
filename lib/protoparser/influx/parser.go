package influx

import (
	"bytes"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
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

// Unmarshal unmarshals influx line protocol rows from b.
//
// See https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/
//
// s shouldn't be modified when rs is in use.
func (rs *Rows) Unmarshal(b []byte) error {
	rs.reset()
	return rs.unmarshal(b)
}

func (rs *Rows) reset() {
	rs.Rows = rs.Rows[:0]
	rs.tagsPool = rs.tagsPool[:0]
	rs.fieldsPool = rs.fieldsPool[:0]
}

// Row is a single influx row.
type Row struct {
	Measurement []byte
	Tags        []Tag
	Fields      []Field
	Timestamp   int64
}

func (r *Row) reset() {
	r.Measurement = nil
	r.Tags = nil
	r.Fields = nil
	r.Timestamp = 0
}

func (r *Row) unmarshal(b []byte, tagsPool []Tag, fieldsPool []Field, noEscapeChars bool) ([]Tag, []Field, error) {
	r.reset()
	n := nextUnescapedChar(b, ' ', noEscapeChars)
	if n < 0 {
		return tagsPool, fieldsPool, fmt.Errorf("cannot find Whitespace I in %q", b)
	}
	measurementTags := b[:n]
	b = stripLeadingWhitespace(b[n+1:])

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
	hasQuotedFields := nextUnescapedChar(b, '"', noEscapeChars) >= 0
	n = nextUnquotedChar(b, ' ', noEscapeChars, hasQuotedFields)
	if n < 0 {
		// No timestamp.
		fieldsPool, err = unmarshalInfluxFields(fieldsPool, b, noEscapeChars, hasQuotedFields)
		if err != nil {
			return tagsPool, fieldsPool, err
		}
		fields := fieldsPool[fieldsStart:]
		r.Fields = fields[:len(fields):len(fields)]
		return tagsPool, fieldsPool, nil
	}
	fieldsPool, err = unmarshalInfluxFields(fieldsPool, b[:n], noEscapeChars, hasQuotedFields)
	if err != nil {
		if bytes.HasPrefix(b[n+1:], []byte("HTTP/")) {
			return tagsPool, fieldsPool, fmt.Errorf("please switch from tcp to http protocol for data ingestion; " +
				"do not set `-influxListenAddr` command-line flag, since it is needed for tcp protocol only")
		}
		return tagsPool, fieldsPool, err
	}
	r.Fields = fieldsPool[fieldsStart:]
	b = stripLeadingWhitespace(b[n+1:])

	// The timestamp is optional in the InfluxDB line protocol.
	// Whitespace before it may still be present even when the timestamp itself is omitted.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10049
	if len(b) > 0 {
		timestamp, err := fastfloat.ParseInt64(bytesutil.ToUnsafeString(b))
		if err != nil {
			if bytes.HasPrefix(b, []byte("HTTP/")) {
				return tagsPool, fieldsPool, fmt.Errorf("please switch from tcp to http protocol for data ingestion; " +
					"do not set `-influxListenAddr` command-line flag, since it is needed for tcp protocol only")
			}
			return tagsPool, fieldsPool, fmt.Errorf("cannot parse timestamp %q: %w", b, err)
		}
		r.Timestamp = timestamp
	}
	return tagsPool, fieldsPool, nil
}

// Tag represents influx tag.
type Tag struct {
	Key   []byte
	Value []byte
}

func (tag *Tag) reset() {
	tag.Key = nil
	tag.Value = nil
}

func (tag *Tag) unmarshal(b []byte, noEscapeChars bool) error {
	tag.reset()
	n := nextUnescapedChar(b, '=', noEscapeChars)
	if n < 0 {
		return fmt.Errorf("missing tag value for %q", b)
	}
	tag.Key = unescapeTagValue(b[:n], noEscapeChars)
	tag.Value = unescapeTagValue(b[n+1:], noEscapeChars)
	return nil
}

// Field represents influx field.
type Field struct {
	Key   []byte
	Value float64
}

func (f *Field) reset() {
	f.Key = nil
	f.Value = 0
}

func (f *Field) unmarshal(b []byte, noEscapeChars, hasQuotedFields bool) error {
	f.reset()
	n := nextUnescapedChar(b, '=', noEscapeChars)
	if n < 0 {
		return fmt.Errorf("missing field value for %q", b)
	}
	f.Key = unescapeTagValue(b[:n], noEscapeChars)
	if len(f.Key) == 0 {
		return fmt.Errorf("field key cannot be empty")
	}
	v, err := parseFieldValue(b[n+1:], hasQuotedFields)
	if err != nil {
		return fmt.Errorf("cannot parse field value for %q: %w", f.Key, err)
	}
	f.Value = v
	return nil
}

func (rs *Rows) unmarshal(b []byte) error {
	noEscapeChars := bytes.IndexByte(b, '\\') < 0
	for len(b) > 0 {
		n := bytes.IndexByte(b, '\n')
		if n < 0 {
			// The last line.
			n = len(b)
		}
		err := rs.unmarshalRow(b[:n], noEscapeChars)
		if err != nil {
			if !rs.IgnoreErrs {
				return fmt.Errorf("incorrect influx line %q: %w", b, err)
			}
			logger.Errorf("skipping InfluxDB line %q because of error: %s", b, err)
			invalidLines.Inc()
		}
		if len(b) == n {
			return nil
		}
		b = b[n+1:]
	}
	return nil
}

func (rs *Rows) unmarshalRow(b []byte, noEscapeChars bool) error {
	if len(b) > 0 && b[len(b)-1] == '\r' {
		b = b[:len(b)-1]
	}
	if len(b) == 0 {
		// Skip empty line
		return nil
	}
	if b[0] == '#' {
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
	rs.tagsPool, rs.fieldsPool, err = r.unmarshal(b, rs.tagsPool, rs.fieldsPool, noEscapeChars)
	if err != nil {
		rs.Rows = rs.Rows[:len(rs.Rows)-1]
	}
	return err
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="influx"}`)

func unmarshalTags(dst []Tag, b []byte, noEscapeChars bool) ([]Tag, error) {
	for {
		if cap(dst) > len(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, Tag{})
		}
		tag := &dst[len(dst)-1]
		n := nextUnescapedChar(b, ',', noEscapeChars)
		if n < 0 {
			if err := tag.unmarshal(b, noEscapeChars); err != nil {
				return dst[:len(dst)-1], err
			}
			if len(tag.Key) == 0 || len(tag.Value) == 0 {
				// Skip empty tag
				dst = dst[:len(dst)-1]
			}
			return dst, nil
		}
		if err := tag.unmarshal(b[:n], noEscapeChars); err != nil {
			return dst[:len(dst)-1], err
		}
		b = b[n+1:]
		if len(tag.Key) == 0 || len(tag.Value) == 0 {
			// Skip empty tag
			dst = dst[:len(dst)-1]
		}
	}
}

func unmarshalInfluxFields(dst []Field, b []byte, noEscapeChars, hasQuotedFields bool) ([]Field, error) {
	for {
		if cap(dst) > len(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, Field{})
		}
		f := &dst[len(dst)-1]
		n := nextUnquotedChar(b, ',', noEscapeChars, hasQuotedFields)
		if n < 0 {
			if err := f.unmarshal(b, noEscapeChars, hasQuotedFields); err != nil {
				return dst, err
			}
			return dst, nil
		}
		if err := f.unmarshal(b[:n], noEscapeChars, hasQuotedFields); err != nil {
			return dst, err
		}
		b = b[n+1:]
	}
}

func unescapeTagValue(s []byte, noEscapeChars bool) []byte {
	if noEscapeChars {
		// Fast path - no escape chars.
		return s
	}
	n := bytes.IndexByte(s, '\\')
	if n < 0 {
		return s
	}

	// Slow path. Remove escape chars.
	dst := s[:0]
	for {
		dst = append(dst, s[:n]...)
		s = s[n+1:]
		if len(s) == 0 {
			return append(dst, '\\')
		}
		ch := s[0]
		if ch != ' ' && ch != ',' && ch != '=' && ch != '\\' {
			dst = append(dst, '\\')
		}
		dst = append(dst, ch)
		s = s[1:]
		n = bytes.IndexByte(s, '\\')
		if n < 0 {
			return append(dst, s...)
		}
	}
}

func parseFieldValue(b []byte, hasQuotedFields bool) (float64, error) {
	if len(b) == 0 {
		return 0, fmt.Errorf("field value cannot be empty")
	}
	s := bytesutil.ToUnsafeString(b)
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

func nextUnescapedChar(b []byte, ch byte, noEscapeChars bool) int {
	if noEscapeChars {
		// Fast path: just search for ch in s, since s has no escape chars.
		return bytes.IndexByte(b, ch)
	}

	sOrig := b
again:
	n := bytes.IndexByte(b, ch)
	if n < 0 {
		return -1
	}
	if n == 0 {
		return len(sOrig) - len(b) + n
	}
	if b[n-1] != '\\' {
		return len(sOrig) - len(b) + n
	}
	nOrig := n
	slashes := 0
	for n > 0 && b[n-1] == '\\' {
		slashes++
		n--
	}
	if slashes&1 == 0 {
		return len(sOrig) - len(b) + nOrig
	}
	b = b[nOrig+1:]
	goto again
}

func nextUnquotedChar(b []byte, ch byte, noEscapeChars, hasQuotedFields bool) int {
	if !hasQuotedFields {
		return nextUnescapedChar(b, ch, noEscapeChars)
	}
	sOrig := b
	for {
		n := nextUnescapedChar(b, ch, noEscapeChars)
		if n < 0 {
			return -1
		}
		if !isInQuote(b[:n], noEscapeChars) {
			return n + len(sOrig) - len(b)
		}
		b = b[n+1:]
		n = nextUnescapedChar(b, '"', noEscapeChars)
		if n < 0 {
			return -1
		}
		b = b[n+1:]
	}
}

func isInQuote(b []byte, noEscapeChars bool) bool {
	isQuote := false
	for {
		n := nextUnescapedChar(b, '"', noEscapeChars)
		if n < 0 {
			return isQuote
		}
		isQuote = !isQuote
		b = b[n+1:]
	}
}

func stripLeadingWhitespace(b []byte) []byte {
	for len(b) > 0 && b[0] == ' ' {
		b = b[1:]
	}
	return b
}
