package statsd

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

// Statsd metric format with tags: MetricName:value|type|@sample_rate|#tag1:value,tag1...
// https://docs.datadoghq.com/developers/dogstatsd/datagram_shell?tab=metrics#the-dogstatsd-protocol
const (
	statsdSeparator          = '|'
	statsdPairsSeparator     = ':'
	statsdTagsStartSeparator = '#'
	statsdTagsSeparator      = ','
)

const statsdTypeTagName = "__statsd_metric_type__"

// https://github.com/b/statsd_spec
var validTypes = []string{
	// counter
	"c",
	// gauge
	"g",
	// histogram
	"h",
	// timer
	"ms",
	// distribution
	"d",
	// set
	"s",
	// meters
	"m",
}

func isValidType(src string) bool {
	for _, t := range validTypes {
		if src == t {
			return true
		}
	}
	return false
}

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
	Values    []float64
	Timestamp int64
}

func (r *Row) reset() {
	r.Metric = ""
	r.Tags = nil
	r.Values = r.Values[:0]
	r.Timestamp = 0
}

func (r *Row) unmarshal(s string, tagsPool []Tag) ([]Tag, error) {
	r.reset()
	originalString := s
	s = stripTrailingWhitespace(s)
	nextSeparator := strings.IndexByte(s, statsdSeparator)
	if nextSeparator <= 0 {
		return tagsPool, fmt.Errorf("cannot find type separator %q position at: %q", statsdSeparator, originalString)
	}
	metricWithValues := s[:nextSeparator]
	s = s[nextSeparator+1:]
	valuesSeparatorPosition := strings.IndexByte(metricWithValues, statsdPairsSeparator)
	if valuesSeparatorPosition <= 0 {
		return tagsPool, fmt.Errorf("cannot find metric name value separator=%q at: %q; original line: %q", statsdPairsSeparator, metricWithValues, originalString)
	}

	r.Metric = metricWithValues[:valuesSeparatorPosition]
	metricWithValues = metricWithValues[valuesSeparatorPosition+1:]
	// datadog extension v1.1 for statsd allows multiple packed values at single line
	for {
		nextSeparator = strings.IndexByte(metricWithValues, statsdPairsSeparator)
		if nextSeparator <= 0 {
			// last element
			metricWithValues = stripTrailingWhitespace(metricWithValues)
			v, err := fastfloat.Parse(metricWithValues)
			if err != nil {
				return tagsPool, fmt.Errorf("cannot unmarshal value from %q: %w; original line: %q", metricWithValues, err, originalString)
			}
			r.Values = append(r.Values, v)
			break
		}
		valueStr := metricWithValues[:nextSeparator]
		v, err := fastfloat.Parse(valueStr)
		if err != nil {
			return tagsPool, fmt.Errorf("cannot unmarshal value from %q: %w; original line: %q", valueStr, err, originalString)
		}
		r.Values = append(r.Values, v)
		metricWithValues = metricWithValues[nextSeparator+1:]
	}
	// search for the type end
	nextSeparator = strings.IndexByte(s, statsdSeparator)
	typeValue := s
	if nextSeparator >= 0 {
		typeValue = s[:nextSeparator]
		s = s[nextSeparator+1:]
	}
	if !isValidType(typeValue) {
		return tagsPool, fmt.Errorf("provided type=%q is not supported; original line: %q", typeValue, originalString)
	}
	tagsStart := len(tagsPool)
	tagsPool = slicesutil.SetLength(tagsPool, len(tagsPool)+1)
	// add metric type as tag
	tag := &tagsPool[len(tagsPool)-1]
	tag.Key = statsdTypeTagName
	tag.Value = typeValue

	// process tags
	nextSeparator = strings.IndexByte(s, statsdTagsStartSeparator)
	if nextSeparator < 0 {
		tags := tagsPool[tagsStart:]
		r.Tags = tags[:len(tags):len(tags)]
		return tagsPool, nil
	}
	tagsStr := s[nextSeparator+1:]
	// search for end of tags
	nextSeparator = strings.IndexByte(tagsStr, statsdSeparator)
	if nextSeparator >= 0 {
		tagsStr = tagsStr[:nextSeparator]
	}

	tagsPool = unmarshalTags(tagsPool, tagsStr)
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
		dst = slicesutil.SetLength(dst, len(dst)+1)
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
