package graphite

import (
	"flag"
	"fmt"
	"regexp"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

var (
	sanitizeMetricName = flag.Bool("graphite.sanitizeMetricName", false, "Sanitize metric names for the ingested Graphite data. "+
		"See https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#ingesting")
)

// graphite text line protocol may use white space or tab as separator
// See https://github.com/grobian/carbon-c-relay/commit/f3ffe6cc2b52b07d14acbda649ad3fd6babdd528
const graphiteSeparators = " \t"

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
		// No tags
		r.Metric = s
	} else {
		// Tags found
		r.Metric = s[:n]
		tagsStart := len(tagsPool)
		tagsPool = unmarshalTags(tagsPool, s[n+1:])
		tags := tagsPool[tagsStart:]
		r.Tags = tags[:len(tags):len(tags)]
	}
	if *sanitizeMetricName {
		r.Metric = sanitizer.Transform(r.Metric)
	}
	if len(r.Metric) == 0 {
		return tagsPool, fmt.Errorf("metric cannot be empty")
	}
	return tagsPool, nil
}

func (r *Row) unmarshal(s string, tagsPool []Tag) ([]Tag, error) {
	r.reset()
	sOrig := s
	s = stripTrailingWhitespace(s)
	n := strings.LastIndexAny(s, graphiteSeparators)
	if n < 0 {
		return tagsPool, fmt.Errorf("cannot find separator between value and timestamp in %q", s)
	}
	timestampStr := s[n+1:]
	valueStr := ""
	metricAndTags := ""
	s = stripTrailingWhitespace(s[:n])
	n = strings.LastIndexAny(s, graphiteSeparators)
	if n < 0 {
		// Missing timestamp
		metricAndTags = stripLeadingWhitespace(s)
		valueStr = timestampStr
		timestampStr = ""
	} else {
		metricAndTags = stripLeadingWhitespace(s[:n])
		valueStr = s[n+1:]
	}
	metricAndTags = stripTrailingWhitespace(metricAndTags)
	tagsPool, err := r.UnmarshalMetricAndTags(metricAndTags, tagsPool)
	if err != nil {
		return tagsPool, fmt.Errorf("cannot parse metric and tags from %q: %w; original line: %q", metricAndTags, err, sOrig)
	}
	if len(timestampStr) > 0 {
		ts, err := fastfloat.Parse(timestampStr)
		if err != nil {
			return tagsPool, fmt.Errorf("cannot unmarshal timestamp from %q: %w; original line: %q", timestampStr, err, sOrig)
		}
		r.Timestamp = int64(ts)
	}
	v, err := fastfloat.Parse(valueStr)
	if err != nil {
		return tagsPool, fmt.Errorf("cannot unmarshal metric value from %q: %w; original line: %q", valueStr, err, sOrig)
	}
	r.Value = v
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
		logger.Errorf("cannot unmarshal Graphite line %q: %s", s, err)
		invalidLines.Inc()
	}
	return dst, tagsPool
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="graphite"}`)

func unmarshalTags(dst []Tag, s string) []Tag {
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
	if *sanitizeMetricName {
		t.Key = sanitizer.Transform(t.Key)
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

var sanitizer = bytesutil.NewFastStringTransformer(func(s string) string {
	// Apply rule to drop some chars to preserve backwards compatibility
	s = repeatedDots.ReplaceAllLiteralString(s, ".")

	// Replace any remaining illegal chars
	return allowedChars.ReplaceAllLiteralString(s, "_")
})

var (
	repeatedDots = regexp.MustCompile(`[.]+`)
	allowedChars = regexp.MustCompile(`[^a-zA-Z0-9:_.]`)
)
