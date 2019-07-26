package graphite

import (
	"fmt"
	"strings"

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
func (rs *Rows) Unmarshal(s string) error {
	var err error
	rs.Rows, rs.tagsPool, err = unmarshalRows(rs.Rows[:0], s, rs.tagsPool[:0])
	if err != nil {
		return err
	}
	return err
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
			return tagsPool, fmt.Errorf("cannot umarshal tags: %s", err)
		}
		tags := tagsPool[tagsStart:]
		r.Tags = tags[:len(tags):len(tags)]
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

func unmarshalRows(dst []Row, s string, tagsPool []Tag) ([]Row, []Tag, error) {
	for len(s) > 0 {
		n := strings.IndexByte(s, '\n')
		if n == 0 {
			// Skip empty line
			s = s[1:]
			continue
		}
		if cap(dst) > len(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, Row{})
		}
		r := &dst[len(dst)-1]
		if n < 0 {
			// The last line.
			var err error
			tagsPool, err = r.unmarshal(s, tagsPool)
			if err != nil {
				err = fmt.Errorf("cannot unmarshal Graphite line %q: %s", s, err)
				return dst, tagsPool, err
			}
			return dst, tagsPool, nil
		}
		var err error
		tagsPool, err = r.unmarshal(s[:n], tagsPool)
		if err != nil {
			err = fmt.Errorf("cannot unmarshal Graphite line %q: %s", s[:n], err)
			return dst, tagsPool, err
		}
		s = s[n+1:]
	}
	return dst, tagsPool, nil
}

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
			return dst, nil
		}
		if err := tag.unmarshal(s[:n]); err != nil {
			return dst[:len(dst)-1], err
		}
		s = s[n+1:]
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
	if len(t.Key) == 0 {
		return fmt.Errorf("tag key cannot be empty for %q", s)
	}
	t.Value = s[n+1:]
	return nil
}
