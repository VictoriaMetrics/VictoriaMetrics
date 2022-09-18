package graphite

import (
	"flag"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

// graphite text line protocol may use white space or tab as separator
// See https://github.com/grobian/carbon-c-relay/commit/f3ffe6cc2b52b07d14acbda649ad3fd6babdd528
const graphiteSeparators = " \t"

var metricHasTimestamp = flag.Bool("graphiteHasTimestamp", false, "Parse whitespaces in metrics")

// Rows contains parsed graphite rows.
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

// Unmarshal unmarshals grahite plaintext protocol rows from s.
//
// See https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol
//
// s shouldn't be modified when rs is in use.
func (rs *Rows) Unmarshal(s string) {
	rs.Rows, rs.tagsPool = unmarshalRows(rs.Rows[:0], s, rs.tagsPool[:0])
}

// Row is a single graphite row.
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

// UnmarshalMetricAndTags unmarshals metric and optional tags from s.
func (r *Row) UnmarshalMetricAndTags(s string, tagsPool []Tag) ([]Tag, error) {
	n := strings.IndexByte(s, ';')
	if n < 0 {
		if m := strings.IndexAny(s, graphiteSeparators); m >= 0 {
			return tagsPool, fmt.Errorf("cannot unmarshal line as it contains separator but no colon: %q", s)
		}
	} else {
		if m := strings.IndexAny(s[:n], graphiteSeparators); m >= 0 {
			return tagsPool, fmt.Errorf("cannot unmarshal line as it contains separator before colon: %q", s)
		}
	}

	if n < 0 {
		// No tags
		r.Metric = s
	} else {
		// Tags found
		r.Metric = s[:n]
		tagsStart := len(tagsPool)
		var err error
		tagsPool, err = unmarshalTags(tagsPool, s[n+1:])
		if err != nil {
			return tagsPool, fmt.Errorf("cannot unmarshal tags: %w", err)
		}
		tags := tagsPool[tagsStart:]
		r.Tags = tags[:len(tags):len(tags)]
	}
	if len(r.Metric) == 0 {
		return tagsPool, fmt.Errorf("metric cannot be empty")
	}
	return tagsPool, nil
}

func (r *Row) unmarshal(s string, tagsPool []Tag) ([]Tag, error) {
	r.reset()
	n := strings.LastIndexAny(stripTrailingWhitespace(s), graphiteSeparators)
	// last possible metrics should be 'm 1'
	if n < 2 {
		return tagsPool, fmt.Errorf("cannot find separator between metric and value in %q", s)
	}

	var value, timestamp string
	// timestamp or value
	last := stripLeadingWhitespace(stripTrailingWhitespace(s[n:]))
	// metric value or metric
	head := stripTrailingWhitespace(s[:n+1])

	if *metricHasTimestamp {
		n = strings.LastIndexAny(head, graphiteSeparators)
		if n < 0 {
			return tagsPool, fmt.Errorf("cannot find separator between metric and value in %q", head)
		}

		value = stripLeadingWhitespace(head[n:])
		timestamp = last
		head = stripTrailingWhitespace(head[:n])
	} else {
		n = strings.LastIndexAny(head, graphiteSeparators)
		if n < 0 {
			value = last
			// use default timestamp
		} else {
			str := stripTrailingWhitespace(head[:n])
			if strings.LastIndexAny(str, graphiteSeparators) >= 0 {
				return tagsPool, fmt.Errorf("whitespaces not allowed in tags (%q), use --graphiteHasTimestamp", str)
			}
			value = stripLeadingWhitespace(stripTrailingWhitespace(head[n:]))
			timestamp = last
			head = stripTrailingWhitespace(head[:n])
		}
	}

	v, err := fastfloat.Parse(value)
	if err != nil {
		return tagsPool, fmt.Errorf("cannot unmarshal value from %q: %w", head, err)
	}

	r.Value = v

	if timestamp != "" {
		ts, err := fastfloat.Parse(timestamp)
		if err != nil {
			return tagsPool, fmt.Errorf("cannot unmarshal timestamp from %q: %w", head, err)
		}

		r.Timestamp = int64(ts)
	}

	tagsPool, err = r.UnmarshalMetricAndTags(head, tagsPool)
	if err != nil {
		return tagsPool, err
	}

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
		logger.Errorf("cannot unmarshal Graphite line %q: %s", s, err)
		invalidLines.Inc()
	}
	return dst, tagsPool
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="graphite"}`)

func unmarshalTags(dst []Tag, s string) ([]Tag, error) {
	for {
		if cap(dst) > len(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, Tag{})
		}
		tag := &dst[len(dst)-1]

		n := strings.IndexByte(s, ';')
		if n < 0 {
			// The last tag found
			tag.unmarshal(s)
			if len(tag.Key) == 0 || len(tag.Value) == 0 {
				// Skip empty tag
				dst = dst[:len(dst)-1]
			}
			return dst, nil
		}
		tag.unmarshal(s[:n])
		s = s[n+1:]
		if len(tag.Key) == 0 || len(tag.Value) == 0 {
			// Skip empty tag
			dst = dst[:len(dst)-1]
		}
	}
}

// Tag is a graphite tag.
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
	n := strings.IndexByte(s, '=')
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
		// graphite text line protocol may use white space or tab as separator
		// See https://github.com/grobian/carbon-c-relay/commit/f3ffe6cc2b52b07d14acbda649ad3fd6babdd528
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
