package graphite

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

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
// s must be unchanged until rs is in use.
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

func (r *Row) unmarshal(s string, tagsPool []Tag) ([]Tag, error) {
	r.reset()
	n := strings.IndexByte(s, ' ')
	if n < 0 {
		return tagsPool, fmt.Errorf("cannot find whitespace between metric and value in %q", s)
	}
	metricAndTags := s[:n]
	tail := s[n+1:]

	n = strings.IndexByte(metricAndTags, ';')
	if n < 0 {
		// No tags
		r.Metric = metricAndTags
	} else {
		// Tags found
		r.Metric = metricAndTags[:n]
		tagsStart := len(tagsPool)
		var err error
		tagsPool, err = unmarshalTags(tagsPool, metricAndTags[n+1:])
		if err != nil {
			return tagsPool, fmt.Errorf("cannot umarshal tags: %w", err)
		}
		tags := tagsPool[tagsStart:]
		r.Tags = tags[:len(tags):len(tags)]
	}
	if len(r.Metric) == 0 {
		return tagsPool, fmt.Errorf("metric cannot be empty")
	}

	n = strings.IndexByte(tail, ' ')
	if n < 0 {
		// There is no timestamp. Use default timestamp instead.
		r.Value = fastfloat.ParseBestEffort(tail)
		return tagsPool, nil
	}
	r.Value = fastfloat.ParseBestEffort(tail[:n])
	r.Timestamp = fastfloat.ParseInt64BestEffort(tail[n+1:])
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

// Tag is a graphite tag.
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
