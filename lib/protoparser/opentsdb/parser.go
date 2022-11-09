package opentsdb

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

// Rows contains parsed OpenTSDB rows.
type Rows struct {
	Rows []Row

	tagsPool []Tag
}

// Reset resets rs.
func (rs *Rows) Reset() {
	// Release references to objects, so they can be GC'ed.

	for i := range rs.Rows {
		rs.Rows[i].reset()
	}
	rs.Rows = rs.Rows[:0]

	for i := range rs.tagsPool {
		rs.tagsPool[i].reset()
	}
	rs.tagsPool = rs.tagsPool[:0]
}

// Unmarshal unmarshals OpenTSDB put rows from s.
//
// See http://opentsdb.net/docs/build/html/api_telnet/put.html
//
// s shouldn't be modified when rs is in use.
func (rs *Rows) Unmarshal(s string) {
	rs.Rows, rs.tagsPool = unmarshalRows(rs.Rows[:0], s, rs.tagsPool[:0])
}

// Row is a single OpenTSDB row.
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
	s = trimLeadingSpaces(s)
	if !strings.HasPrefix(s, "put ") {
		return tagsPool, fmt.Errorf("missing `put ` prefix in %q", s)
	}
	s = s[len("put "):]
	s = trimLeadingSpaces(s)
	n := strings.IndexByte(s, ' ')
	if n < 0 {
		return tagsPool, fmt.Errorf("cannot find whitespace between metric and timestamp in %q", s)
	}
	r.Metric = s[:n]
	if len(r.Metric) == 0 {
		return tagsPool, fmt.Errorf("metric cannot be empty")
	}
	tail := trimLeadingSpaces(s[n+1:])
	n = strings.IndexByte(tail, ' ')
	if n < 0 {
		return tagsPool, fmt.Errorf("cannot find whitespace between timestamp and value in %q", s)
	}
	timestamp, err := fastfloat.Parse(tail[:n])
	if err != nil {
		return tagsPool, fmt.Errorf("cannot parse timestamp from %q: %w", tail[:n], err)
	}
	r.Timestamp = int64(timestamp)
	tail = trimLeadingSpaces(tail[n+1:])
	valueStr := ""
	tagsStr := ""
	n = strings.IndexByte(tail, ' ')
	if n < 0 {
		// Missing tags.
		// Accept this case even if OpenTSDB forbids it according to http://opentsdb.net/docs/build/html/api_telnet/put.html:
		// > At least one tag pair must be present.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3290
		valueStr = tail
	} else {
		valueStr = tail[:n]
		tagsStr = tail[n+1:]
	}
	v, err := fastfloat.Parse(valueStr)
	if err != nil {
		return tagsPool, fmt.Errorf("cannot parse value from %q: %w", valueStr, err)
	}
	r.Value = v
	tagsStart := len(tagsPool)
	tagsPool, err = unmarshalTags(tagsPool, tagsStr)
	if err != nil {
		return tagsPool, fmt.Errorf("cannot unmarshal tags in %q: %w", s, err)
	}
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
		logger.Errorf("cannot unmarshal OpenTSDB line %q: %s", s, err)
		invalidLines.Inc()
	}
	return dst, tagsPool
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="opentsdb"}`)

func unmarshalTags(dst []Tag, s string) ([]Tag, error) {
	for {
		s = trimLeadingSpaces(s)
		if len(s) == 0 {
			return dst, nil
		}
		if cap(dst) > len(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, Tag{})
		}
		tag := &dst[len(dst)-1]

		n := strings.IndexByte(s, ' ')
		if n < 0 {
			// The last tag found
			if err := tag.unmarshal(s); err != nil {
				return dst[:len(dst)-1], err
			}
			if len(tag.Key) == 0 || len(tag.Value) == 0 {
				// Skip empty tag
				dst = dst[:len(dst)-1]
			}
			return dst, nil
		}
		if err := tag.unmarshal(s[:n]); err != nil {
			return dst[:len(dst)-1], err
		}
		s = s[n+1:]
		if len(tag.Key) == 0 || len(tag.Value) == 0 {
			// Skip empty tag
			dst = dst[:len(dst)-1]
		}
	}
}

// Tag is an OpenTSDB tag.
type Tag struct {
	Key   string
	Value string
}

func (t *Tag) reset() {
	t.Key = ""
	t.Value = ""
}

func (t *Tag) unmarshal(s string) error {
	t.reset()
	n := strings.IndexByte(s, '=')
	if n < 0 {
		return fmt.Errorf("missing tag value for %q", s)
	}
	t.Key = s[:n]
	t.Value = s[n+1:]
	return nil
}

func trimLeadingSpaces(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return s
}
