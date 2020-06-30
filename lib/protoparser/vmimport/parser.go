package vmimport

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson"
)

// Rows contains parsed rows from `/api/v1/import` request.
type Rows struct {
	Rows []Row

	tu tagsUnmarshaler
}

// Reset resets rs.
func (rs *Rows) Reset() {
	for i := range rs.Rows {
		rs.Rows[i].reset()
	}
	rs.Rows = rs.Rows[:0]

	rs.tu.reset()
}

// Unmarshal unmarshals influx line protocol rows from s.
//
// See https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/
//
// s must be unchanged until rs is in use.
func (rs *Rows) Unmarshal(s string) {
	rs.tu.reset()
	rs.Rows = unmarshalRows(rs.Rows[:0], s, &rs.tu)
}

// Row is a single row from `/api/v1/import` request.
type Row struct {
	Tags       []Tag
	Values     []float64
	Timestamps []int64
}

func (r *Row) reset() {
	r.Tags = nil
	r.Values = r.Values[:0]
	r.Timestamps = r.Timestamps[:0]
}

func (r *Row) unmarshal(s string, tu *tagsUnmarshaler) error {
	r.reset()
	v, err := tu.p.Parse(s)
	if err != nil {
		return fmt.Errorf("cannot parse json line: %w", err)
	}

	// Unmarshal tags
	metric := v.GetObject("metric")
	if metric == nil {
		return fmt.Errorf("missing `metric` object")
	}
	tagsStart := len(tu.tagsPool)
	if err := tu.unmarshalTags(metric); err != nil {
		return fmt.Errorf("cannot unmarshal `metric`: %w", err)
	}
	tags := tu.tagsPool[tagsStart:]
	r.Tags = tags[:len(tags):len(tags)]
	if len(r.Tags) == 0 {
		return fmt.Errorf("missing tags")
	}

	// Unmarshal values
	values := v.GetArray("values")
	if len(values) == 0 {
		return fmt.Errorf("missing `values` array")
	}
	for i, v := range values {
		f, err := v.Float64()
		if err != nil {
			return fmt.Errorf("cannot unmarshal value at position %d: %w", i, err)
		}
		r.Values = append(r.Values, f)
	}

	// Unmarshal timestamps
	timestamps := v.GetArray("timestamps")
	if len(timestamps) == 0 {
		return fmt.Errorf("missing `timestamps` array")
	}
	for i, v := range timestamps {
		ts, err := v.Int64()
		if err != nil {
			return fmt.Errorf("cannot unmarshal timestamp at position %d: %w", i, err)
		}
		r.Timestamps = append(r.Timestamps, ts)
	}

	if len(r.Timestamps) != len(r.Values) {
		return fmt.Errorf("`timestamps` array size must match `values` array size; got %d; want %d", len(r.Timestamps), len(r.Values))
	}
	return nil
}

// Tag represents `/api/v1/import` tag.
type Tag struct {
	Key   []byte
	Value []byte
}

func (tag *Tag) reset() {
	// tag.Key and tag.Value point to tu.bytesPool, so there is no need in keeping these byte slices here.
	tag.Key = nil
	tag.Value = nil
}

type tagsUnmarshaler struct {
	p         fastjson.Parser
	tagsPool  []Tag
	bytesPool []byte
	err       error
}

func (tu *tagsUnmarshaler) reset() {
	for i := range tu.tagsPool {
		tu.tagsPool[i].reset()
	}
	tu.tagsPool = tu.tagsPool[:0]

	tu.bytesPool = tu.bytesPool[:0]
	tu.err = nil
}

func (tu *tagsUnmarshaler) addTag() *Tag {
	dst := tu.tagsPool
	if cap(dst) > len(dst) {
		dst = dst[:len(dst)+1]
	} else {
		dst = append(dst, Tag{})
	}
	tag := &dst[len(dst)-1]
	tu.tagsPool = dst
	return tag
}

func (tu *tagsUnmarshaler) addBytes(b []byte) []byte {
	bytesPoolLen := len(tu.bytesPool)
	tu.bytesPool = append(tu.bytesPool, b...)
	bCopy := tu.bytesPool[bytesPoolLen:]
	return bCopy[:len(bCopy):len(bCopy)]
}

func (tu *tagsUnmarshaler) unmarshalTags(o *fastjson.Object) error {
	tu.err = nil
	o.Visit(func(key []byte, v *fastjson.Value) {
		tag := tu.addTag()
		tag.Key = tu.addBytes(key)
		sb, err := v.StringBytes()
		if err != nil && tu.err != nil {
			tu.err = fmt.Errorf("cannot parse value for tag %q: %w", tag.Key, err)
		}
		tag.Value = tu.addBytes(sb)
	})
	return tu.err
}

func unmarshalRows(dst []Row, s string, tu *tagsUnmarshaler) []Row {
	for len(s) > 0 {
		n := strings.IndexByte(s, '\n')
		if n < 0 {
			// The last line.
			return unmarshalRow(dst, s, tu)
		}
		dst = unmarshalRow(dst, s[:n], tu)
		s = s[n+1:]
	}
	return dst
}

func unmarshalRow(dst []Row, s string, tu *tagsUnmarshaler) []Row {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	if len(s) == 0 {
		return dst
	}
	if cap(dst) > len(dst) {
		dst = dst[:len(dst)+1]
	} else {
		dst = append(dst, Row{})
	}
	r := &dst[len(dst)-1]
	if err := r.unmarshal(s, tu); err != nil {
		dst = dst[:len(dst)-1]
		logger.Errorf("cannot unmarshal json line %q: %s; skipping it", s, err)
		invalidLines.Inc()
	}
	return dst
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="vmimport"}`)
