package csvimport

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

// Rows represents csv rows.
type Rows struct {
	// Rows contains parsed csv rows after the call to Unmarshal.
	Rows []Row

	sc          scanner
	tagsPool    []Tag
	metricsPool []metric
}

// Reset resets rs.
func (rs *Rows) Reset() {
	rows := rs.Rows
	for i := range rows {
		r := &rows[i]
		r.Metric = ""
		r.Tags = nil
		r.Value = 0
		r.Timestamp = 0
	}
	rs.Rows = rs.Rows[:0]

	rs.sc.Init("")

	tags := rs.tagsPool
	for i := range tags {
		t := &tags[i]
		t.Key = ""
		t.Value = ""
	}
	rs.tagsPool = rs.tagsPool[:0]

	metrics := rs.metricsPool
	for i := range metrics {
		m := &metrics[i]
		m.Name = ""
		m.Value = 0
	}
	rs.metricsPool = rs.metricsPool[:0]
}

// Row represents a single metric row
type Row struct {
	Metric    string
	Tags      []Tag
	Value     float64
	Timestamp int64
}

// Tag represents metric tag
type Tag struct {
	Key   string
	Value string
}

type metric struct {
	Name  string
	Value float64
}

// Unmarshal unmarshal csv lines from s according to the given cds.
func (rs *Rows) Unmarshal(s string, cds []ColumnDescriptor) {
	rs.sc.Init(s)
	rs.Rows, rs.tagsPool, rs.metricsPool = parseRows(&rs.sc, rs.Rows[:0], rs.tagsPool[:0], rs.metricsPool[:0], cds)
}

func parseRows(sc *scanner, dst []Row, tags []Tag, metrics []metric, cds []ColumnDescriptor) ([]Row, []Tag, []metric) {
	for sc.NextLine() {
		line := sc.Line
		var r Row
		col := uint(0)
		metrics = metrics[:0]
		tagsLen := len(tags)
		for sc.NextColumn() {
			if col >= uint(len(cds)) {
				// Skip superflouous column.
				continue
			}
			cd := &cds[col]
			col++
			if parseTimestamp := cd.ParseTimestamp; parseTimestamp != nil {
				timestamp, err := parseTimestamp(sc.Column)
				if err != nil {
					sc.Error = fmt.Errorf("cannot parse timestamp from %q: %w", sc.Column, err)
					break
				}
				r.Timestamp = timestamp
				continue
			}
			if tagName := cd.TagName; tagName != "" {
				tags = append(tags, Tag{
					Key:   tagName,
					Value: sc.Column,
				})
				continue
			}
			metricName := cd.MetricName
			if metricName == "" {
				// The given field is ignored.
				continue
			}
			value := fastfloat.ParseBestEffort(sc.Column)
			metrics = append(metrics, metric{
				Name:  metricName,
				Value: value,
			})
		}
		if col < uint(len(cds)) && sc.Error == nil {
			sc.Error = fmt.Errorf("missing columns in the csv line %q; got %d columns; want at least %d columns", line, col, len(cds))
		}
		if sc.Error != nil {
			logger.Errorf("error when parsing csv line %q: %s; skipping this line", line, sc.Error)
			invalidLines.Inc()
			continue
		}
		if len(metrics) == 0 {
			logger.Panicf("BUG: expecting at least a single metric in columnDescriptors=%#v", cds)
		}
		r.Metric = metrics[0].Name
		r.Tags = tags[tagsLen:]
		r.Value = metrics[0].Value
		dst = append(dst, r)
		for _, m := range metrics[1:] {
			dst = append(dst, Row{
				Metric:    m.Name,
				Tags:      r.Tags,
				Value:     m.Value,
				Timestamp: r.Timestamp,
			})
		}
	}
	return dst, tags, metrics
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="csvimport"}`)
