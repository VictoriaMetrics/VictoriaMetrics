package graphite

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	graphiteparser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

// TagsTagSeriesHandler implements /tags/tagSeries handler.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb
func TagsTagSeriesHandler(w http.ResponseWriter, r *http.Request) error {
	return registerMetrics(w, r, false)
}

// TagsTagMultiSeriesHandler implements /tags/tagMultiSeries handler.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb
func TagsTagMultiSeriesHandler(w http.ResponseWriter, r *http.Request) error {
	return registerMetrics(w, r, true)
}

func registerMetrics(w http.ResponseWriter, r *http.Request, isJSONResponse bool) error {
	startTime := time.Now()
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	paths := r.Form["path"]
	var row graphiteparser.Row
	var labels []prompb.Label
	var b []byte
	var tagsPool []graphiteparser.Tag
	mrs := make([]storage.MetricRow, len(paths))
	ct := time.Now().UnixNano() / 1e6
	canonicalPaths := make([]string, len(paths))
	for i, path := range paths {
		var err error
		tagsPool, err = row.UnmarshalMetricAndTags(path, tagsPool[:0])
		if err != nil {
			return fmt.Errorf("cannot parse path=%q: %w", path, err)
		}

		// Construct canonical path according to https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb
		sort.Slice(row.Tags, func(i, j int) bool {
			return row.Tags[i].Key < row.Tags[j].Key
		})
		b = append(b[:0], row.Metric...)
		for _, tag := range row.Tags {
			b = append(b, ';')
			b = append(b, tag.Key...)
			b = append(b, '=')
			b = append(b, tag.Value...)
		}
		canonicalPaths[i] = string(b)

		// Convert parsed metric and tags to labels.
		labels = append(labels[:0], prompb.Label{
			Name:  []byte("__name__"),
			Value: []byte(row.Metric),
		})
		for _, tag := range row.Tags {
			labels = append(labels, prompb.Label{
				Name:  []byte(tag.Key),
				Value: []byte(tag.Value),
			})
		}

		// Put labels with the current timestamp to MetricRow
		mr := &mrs[i]
		mr.MetricNameRaw = storage.MarshalMetricNameRaw(mr.MetricNameRaw[:0], labels)
		mr.Timestamp = ct
	}
	if err := vmstorage.RegisterMetricNames(mrs); err != nil {
		return fmt.Errorf("cannot register paths: %w", err)
	}

	// Return response
	contentType := "text/plain; charset=utf-8"
	if isJSONResponse {
		contentType = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	WriteTagsTagMultiSeriesResponse(w, canonicalPaths, isJSONResponse)
	if isJSONResponse {
		tagsTagMultiSeriesDuration.UpdateDuration(startTime)
	} else {
		tagsTagSeriesDuration.UpdateDuration(startTime)
	}
	return nil
}

var (
	tagsTagSeriesDuration      = metrics.NewSummary(`vm_request_duration_seconds{path="/tags/tagSeries"}`)
	tagsTagMultiSeriesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/tags/tagMultiSeries"}`)
)
