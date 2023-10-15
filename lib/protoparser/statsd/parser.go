package statsd

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

// Statsd metric format with tags: MetricName:value|type|@sample_rate|#tag1:value,tag1...
const statsdSeparator = '|'
const statsdPairsSeparator = ':'
const statsdTagsStartSeparator = '#'
const statsdTagsSeparator = ','

// Rows contains parsed statsd rows.
type Rows struct {
	Rows []Row

	tagsPool []Tag
}

// Reset resets rs.
func (rs *Rows) Reset() {
	// Reset items, so they can be GC'ed

	for i := range rs.Rows {
		rs.Rows[i].reset()
	}
	rs.Rows = rs.Rows[:0]

	for i := range rs.tagsPool {
		rs.tagsPool[i].reset()
	}
	rs.tagsPool = rs.tagsPool[:0]
}

// Unmarshal unmarshals statsd plaintext protocol rows from s.
//
// s shouldn't be modified when rs is in use.
func (rs *Rows) Unmarshal(s string) {
	rs.Rows, rs.tagsPool = unmarshalRows(rs.Rows[:0], s, rs.tagsPool[:0])
}

// Row is a single statsd row.
type Row struct {
	Metric    string
	Tags      []Tag
	Value     float64
	Timestamp int64
}

func (r *Row) reset() {
	r.Metric = ""
	r.Tags = nil
	r.Value = 0
	r.Timestamp = 0
}

func (r *Row) unmarshal(s string, tagsPool []Tag) ([]Tag, error) {
	r.reset()
	originalString := s
	s = stripTrailingWhitespace(s)
	separatorPosition := strings.IndexByte(s, statsdSeparator)
	if separatorPosition < 0 {
		s = stripTrailingWhitespace(s)
	} else {
		s = stripTrailingWhitespace(s[:separatorPosition])
	}

	valuesSeparatorPosition := strings.LastIndexByte(s, statsdPairsSeparator)

	if valuesSeparatorPosition == 0 {
		return tagsPool, fmt.Errorf("cannot find metric name for %q", s)
	}

	if valuesSeparatorPosition < 0 {
		return tagsPool, fmt.Errorf("cannot find separator for %q", s)
	}

	r.Metric = s[:valuesSeparatorPosition]
	valueStr := s[valuesSeparatorPosition+1:]

	v, err := fastfloat.Parse(valueStr)
	if err != nil {
		return tagsPool, fmt.Errorf("cannot unmarshal value from %q: %w; original line: %q", valueStr, err, originalString)
	}
	r.Value = v

	// parsing tags
	tagsSeparatorPosition := strings.LastIndexByte(originalString, statsdTagsStartSeparator)

	if tagsSeparatorPosition < 0 {
		// no tags
		return tagsPool, nil
	}

	tagsStart := len(tagsPool)
	tagsPool = unmarshalTags(tagsPool, originalString[tagsSeparatorPosition+1:])
	tags := tagsPool[tagsStart:]
	r.Tags = tags[:len(tags):len(tags)]

	return tagsPool, nil
}

func unmarshalRows(dst []Row, s string, tagsPool []Tag) ([]Row, []Tag) {
	for len(s) > 0 {
		n := strings.IndexByte(s, '\n')
		if n < 0 {
			// The last line.
			return unmarshalRow(dst, s, tagsPool)
		}
		dst, tagsPool = unmarshalRow(dst, s[:n], tagsPool)
		s = s[n+1:]
	}
	return dst, tagsPool
}

func unmarshalRow(dst []Row, s string, tagsPool []Tag) ([]Row, []Tag) {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	s = stripLeadingWhitespace(s)
	if len(s) == 0 {
		// Skip empty line
		return dst, tagsPool
	}
	if cap(dst) > len(dst) {
		dst = dst[:len(dst)+1]
	} else {
		dst = append(dst, Row{})
	}
	r := &dst[len(dst)-1]
	var err error
	tagsPool, err = r.unmarshal(s, tagsPool)
	if err != nil {
		dst = dst[:len(dst)-1]
		logger.Errorf("cannot unmarshal Statsd line %q: %s", s, err)
		invalidLines.Inc()
	}
	return dst, tagsPool
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="statsd"}`)

func unmarshalTags(dst []Tag, s string) []Tag {
	for {
		if cap(dst) > len(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, Tag{})
		}
		tag := &dst[len(dst)-1]

		n := strings.IndexByte(s, statsdTagsSeparator)

		if n < 0 {
			// The last tag found
			tag.unmarshal(s)
			if len(tag.Key) == 0 || len(tag.Value) == 0 {
				// Skip empty tag
				dst = dst[:len(dst)-1]
			}
			return dst
		}
		tag.unmarshal(s[:n])
		s = s[n+1:]
		if len(tag.Key) == 0 || len(tag.Value) == 0 {
			// Skip empty tag
			dst = dst[:len(dst)-1]
		}
	}
}

// Tag is a statsd tag.
type Tag struct {
	Key   string
	Value string
}

func (t *Tag) reset() {
	t.Key = ""
	t.Value = ""
}

func (t *Tag) unmarshal(s string) {
	t.reset()
	n := strings.IndexByte(s, statsdPairsSeparator)
	if n < 0 {
		// Empty tag value.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1100
		t.Key = s
		t.Value = s[len(s):]
	} else {
		t.Key = s[:n]
		t.Value = s[n+1:]
	}
}

func stripTrailingWhitespace(s string) string {
	n := len(s)
	for {
		n--
		if n < 0 {
			return ""
		}
		ch := s[n]

		if ch != ' ' && ch != '\t' {
			return s[:n+1]
		}
	}
}

func stripLeadingWhitespace(s string) string {
	for len(s) > 0 {
		ch := s[0]
		if ch != ' ' && ch != '\t' {
			return s
		}
		s = s[1:]
	}
	return ""
}
