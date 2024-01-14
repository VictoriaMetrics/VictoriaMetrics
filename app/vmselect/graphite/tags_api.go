package graphite

import (
	"flag"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	graphiteparser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxGraphiteTagKeysPerSearch = flag.Int("search.maxGraphiteTagKeys", 100e3, "The maximum number of tag keys returned from Graphite API, which returns tags. "+
		"See https://docs.victoriametrics.com/#graphite-tags-api-usage")
	maxGraphiteTagValuesPerSearch = flag.Int("search.maxGraphiteTagValues", 100e3, "The maximum number of tag values returned from Graphite API, which returns tag values. "+
		"See https://docs.victoriametrics.com/#graphite-tags-api-usage")
)

// TagsDelSeriesHandler implements /tags/delSeries handler.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#removing-series-from-the-tagdb
func TagsDelSeriesHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	paths := r.Form["path"]
	totalDeleted := 0
	var row graphiteparser.Row
	var tagsPool []graphiteparser.Tag
	ct := startTime.UnixNano() / 1e6
	etfs, err := searchutils.GetExtraTagFilters(r)
	if err != nil {
		return fmt.Errorf("cannot setup tag filters: %w", err)
	}
	for _, path := range paths {
		var err error
		tagsPool, err = row.UnmarshalMetricAndTags(path, tagsPool[:0])
		if err != nil {
			return fmt.Errorf("cannot parse path=%q: %w", path, err)
		}
		tfs := make([]storage.TagFilter, 0, 1+len(row.Tags)+len(etfs))
		tfs = append(tfs, storage.TagFilter{
			Key:   nil,
			Value: []byte(row.Metric),
		})
		for _, tag := range row.Tags {
			tfs = append(tfs, storage.TagFilter{
				Key:   []byte(tag.Key),
				Value: []byte(tag.Value),
			})
		}
		tfss := joinTagFilterss(tfs, etfs)
		sq := storage.NewSearchQuery(0, ct, tfss, 0)
		n, err := netstorage.DeleteSeries(nil, sq, deadline)
		if err != nil {
			return fmt.Errorf("cannot delete series for %q: %w", sq, err)
		}
		totalDeleted += n
	}

	w.Header().Set("Content-Type", "application/json")
	if totalDeleted > 0 {
		fmt.Fprintf(w, "true")
	} else {
		fmt.Fprintf(w, "false")
	}
	return nil
}

// TagsTagSeriesHandler implements /tags/tagSeries handler.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb
func TagsTagSeriesHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	return registerMetrics(startTime, w, r, false)
}

// TagsTagMultiSeriesHandler implements /tags/tagMultiSeries handler.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb
func TagsTagMultiSeriesHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	return registerMetrics(startTime, w, r, true)
}

func registerMetrics(startTime time.Time, w http.ResponseWriter, r *http.Request, isJSONResponse bool) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	_ = deadline // TODO: use the deadline as in the cluster branch
	paths := r.Form["path"]
	var row graphiteparser.Row
	var labels []prompb.Label
	var b []byte
	var tagsPool []graphiteparser.Tag
	mrs := make([]storage.MetricRow, len(paths))
	ct := startTime.UnixNano() / 1e6
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
			Name:  "__name__",
			Value: row.Metric,
		})
		for _, tag := range row.Tags {
			labels = append(labels, prompb.Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}

		// Put labels with the current timestamp to MetricRow
		mr := &mrs[i]
		mr.MetricNameRaw = storage.MarshalMetricNameRaw(mr.MetricNameRaw[:0], labels)
		mr.Timestamp = ct
	}
	vmstorage.RegisterMetricNames(nil, mrs)

	// Return response
	contentType := "text/plain; charset=utf-8"
	if isJSONResponse {
		contentType = "application/json"
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

// TagsAutoCompleteValuesHandler implements /tags/autoComplete/values endpoint from Graphite Tags API.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#auto-complete-support
func TagsAutoCompleteValuesHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	limit, err := httputils.GetInt(r, "limit")
	if err != nil {
		return err
	}
	if limit <= 0 {
		// Use limit=100 by default. See https://graphite.readthedocs.io/en/stable/tags.html#auto-complete-support
		limit = 100
	}
	tag := r.FormValue("tag")
	if len(tag) == 0 {
		return fmt.Errorf("missing `tag` query arg")
	}
	valuePrefix := r.FormValue("valuePrefix")
	exprs := r.Form["expr"]
	var tagValues []string
	etfs, err := searchutils.GetExtraTagFilters(r)
	if err != nil {
		return fmt.Errorf("cannot setup tag filters: %w", err)
	}
	if len(exprs) == 0 && len(etfs) == 0 {
		// Fast path: there are no `expr` filters, so use netstorage.GraphiteTagValues.
		// Escape special chars in tagPrefix as Graphite does.
		// See https://github.com/graphite-project/graphite-web/blob/3ad279df5cb90b211953e39161df416e54a84948/webapp/graphite/tags/base.py#L228
		filter := regexp.QuoteMeta(valuePrefix)
		tagValues, err = netstorage.GraphiteTagValues(nil, tag, filter, *maxGraphiteTagValuesPerSearch, deadline)
		if err != nil {
			return err
		}
	} else {
		// Slow path: use netstorage.SearchMetricNames for applying `expr` filters.
		sq, err := getSearchQueryForExprs(startTime, etfs, exprs, *maxGraphiteTagValuesPerSearch)
		if err != nil {
			return err
		}
		metricNames, err := netstorage.SearchMetricNames(nil, sq, deadline)
		if err != nil {
			return fmt.Errorf("cannot fetch metric names for %q: %w", sq, err)
		}
		m := make(map[string]struct{})
		if tag == "name" {
			tag = "__name__"
		}
		var mn storage.MetricName
		for _, metricName := range metricNames {
			if err := mn.UnmarshalString(metricName); err != nil {
				return fmt.Errorf("cannot unmarshal metricName=%q: %w", metricName, err)
			}
			tagValue := mn.GetTagValue(tag)
			if len(tagValue) == 0 {
				continue
			}
			m[string(tagValue)] = struct{}{}
		}
		if len(valuePrefix) > 0 {
			for tagValue := range m {
				if !strings.HasPrefix(tagValue, valuePrefix) {
					delete(m, tagValue)
				}
			}
		}
		tagValues = make([]string, 0, len(m))
		for tagValue := range m {
			tagValues = append(tagValues, tagValue)
		}
		sort.Strings(tagValues)
		if limit > 0 && limit < len(tagValues) {
			tagValues = tagValues[:limit]
		}
	}

	jsonp := r.FormValue("jsonp")
	contentType := getContentType(jsonp)
	w.Header().Set("Content-Type", contentType)
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTagsAutoCompleteResponse(bw, tagValues, jsonp)
	if err := bw.Flush(); err != nil {
		return err
	}
	tagsAutoCompleteValuesDuration.UpdateDuration(startTime)
	return nil
}

var tagsAutoCompleteValuesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/tags/autoComplete/values"}`)

// TagsAutoCompleteTagsHandler implements /tags/autoComplete/tags endpoint from Graphite Tags API.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#auto-complete-support
func TagsAutoCompleteTagsHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	limit, err := httputils.GetInt(r, "limit")
	if err != nil {
		return err
	}
	if limit <= 0 {
		// Use limit=100 by default. See https://graphite.readthedocs.io/en/stable/tags.html#auto-complete-support
		limit = 100
	}
	tagPrefix := r.FormValue("tagPrefix")
	exprs := r.Form["expr"]
	var labels []string
	etfs, err := searchutils.GetExtraTagFilters(r)
	if err != nil {
		return fmt.Errorf("cannot setup tag filters: %w", err)
	}
	if len(exprs) == 0 && len(etfs) == 0 {
		// Fast path: there are no `expr` filters, so use netstorage.GraphiteTags.

		// Escape special chars in tagPrefix as Graphite does.
		// See https://github.com/graphite-project/graphite-web/blob/3ad279df5cb90b211953e39161df416e54a84948/webapp/graphite/tags/base.py#L181
		filter := regexp.QuoteMeta(tagPrefix)
		labels, err = netstorage.GraphiteTags(nil, filter, *maxGraphiteTagKeysPerSearch, deadline)
		if err != nil {
			return err
		}
	} else {
		// Slow path: use netstorage.SearchMetricNames for applying `expr` filters.
		sq, err := getSearchQueryForExprs(startTime, etfs, exprs, *maxGraphiteTagKeysPerSearch)
		if err != nil {
			return err
		}
		metricNames, err := netstorage.SearchMetricNames(nil, sq, deadline)
		if err != nil {
			return fmt.Errorf("cannot fetch metric names for %q: %w", sq, err)
		}
		m := make(map[string]struct{})
		var mn storage.MetricName
		for _, metricName := range metricNames {
			if err := mn.UnmarshalString(metricName); err != nil {
				return fmt.Errorf("cannot unmarshal metricName=%q: %w", metricName, err)
			}
			m["name"] = struct{}{}
			for _, tag := range mn.Tags {
				m[string(tag.Key)] = struct{}{}
			}
		}
		if len(tagPrefix) > 0 {
			for label := range m {
				if !strings.HasPrefix(label, tagPrefix) {
					delete(m, label)
				}
			}
		}
		labels = make([]string, 0, len(m))
		for label := range m {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		if limit > 0 && limit < len(labels) {
			labels = labels[:limit]
		}
	}

	jsonp := r.FormValue("jsonp")
	contentType := getContentType(jsonp)
	w.Header().Set("Content-Type", contentType)
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTagsAutoCompleteResponse(bw, labels, jsonp)
	if err := bw.Flush(); err != nil {
		return err
	}
	tagsAutoCompleteTagsDuration.UpdateDuration(startTime)
	return nil
}

var tagsAutoCompleteTagsDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/tags/autoComplete/tags"}`)

// TagsFindSeriesHandler implements /tags/findSeries endpoint from Graphite Tags API.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags
func TagsFindSeriesHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	limit, err := httputils.GetInt(r, "limit")
	if err != nil {
		return err
	}
	exprs := r.Form["expr"]
	if len(exprs) == 0 {
		return fmt.Errorf("expecting at least one `expr` query arg")
	}
	etfs, err := searchutils.GetExtraTagFilters(r)
	if err != nil {
		return fmt.Errorf("cannot setup tag filters: %w", err)
	}
	sq, err := getSearchQueryForExprs(startTime, etfs, exprs, *maxGraphiteSeries)
	if err != nil {
		return err
	}
	metricNames, err := netstorage.SearchMetricNames(nil, sq, deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch metric names for %q: %w", sq, err)
	}
	paths, err := getCanonicalPaths(metricNames)
	if err != nil {
		return fmt.Errorf("cannot obtain canonical paths: %w", err)
	}
	if limit > 0 && limit < len(paths) {
		paths = paths[:limit]
	}

	w.Header().Set("Content-Type", "application/json")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTagsFindSeriesResponse(bw, paths)
	if err := bw.Flush(); err != nil {
		return err
	}
	tagsFindSeriesDuration.UpdateDuration(startTime)
	return nil
}

func getCanonicalPaths(metricNames []string) ([]string, error) {
	paths := make([]string, 0, len(metricNames))
	var mn storage.MetricName
	for _, metricName := range metricNames {
		if err := mn.UnmarshalString(metricName); err != nil {
			return nil, fmt.Errorf("cannot unmarshal metricName=%q: %w", metricName, err)
		}
		path := getCanonicalPath(&mn)
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func getCanonicalPath(mn *storage.MetricName) string {
	b := append([]byte{}, mn.MetricGroup...)
	tags := append([]storage.Tag{}, mn.Tags...)
	sort.Slice(tags, func(i, j int) bool {
		return string(tags[i].Key) < string(tags[j].Key)
	})
	for _, tag := range tags {
		b = append(b, ';')
		b = append(b, tag.Key...)
		b = append(b, '=')
		b = append(b, tag.Value...)
	}
	return string(b)
}

var tagsFindSeriesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/tags/findSeries"}`)

// TagValuesHandler implements /tags/<tag_name> endpoint from Graphite Tags API.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags
func TagValuesHandler(startTime time.Time, tagName string, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	limit, err := httputils.GetInt(r, "limit")
	if err != nil {
		return err
	}
	filter := r.FormValue("filter")
	tagValues, err := netstorage.GraphiteTagValues(nil, tagName, filter, *maxGraphiteTagValuesPerSearch, deadline)
	if err != nil {
		return err
	}

	if limit > 0 && limit < len(tagValues) {
		tagValues = tagValues[:limit]
	}
	w.Header().Set("Content-Type", "application/json")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTagValuesResponse(bw, tagName, tagValues)
	if err := bw.Flush(); err != nil {
		return err
	}
	tagValuesDuration.UpdateDuration(startTime)
	return nil
}

var tagValuesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/tags/<tag_name>"}`)

// TagsHandler implements /tags endpoint from Graphite Tags API.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags
func TagsHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	limit, err := httputils.GetInt(r, "limit")
	if err != nil {
		return err
	}
	filter := r.FormValue("filter")
	labels, err := netstorage.GraphiteTags(nil, filter, *maxGraphiteTagKeysPerSearch, deadline)
	if err != nil {
		return err
	}

	if limit > 0 && limit < len(labels) {
		labels = labels[:limit]
	}
	w.Header().Set("Content-Type", "application/json")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTagsResponse(bw, labels)
	if err := bw.Flush(); err != nil {
		return err
	}
	tagsDuration.UpdateDuration(startTime)
	return nil
}

var tagsDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/tags"}`)

func getSearchQueryForExprs(startTime time.Time, etfs [][]storage.TagFilter, exprs []string, maxMetrics int) (*storage.SearchQuery, error) {
	tfs, err := exprsToTagFilters(exprs)
	if err != nil {
		return nil, err
	}
	ct := startTime.UnixNano() / 1e6
	tfss := joinTagFilterss(tfs, etfs)
	sq := storage.NewSearchQuery(0, ct, tfss, maxMetrics)
	return sq, nil
}

func exprsToTagFilters(exprs []string) ([]storage.TagFilter, error) {
	tfs := make([]storage.TagFilter, 0, len(exprs))
	for _, expr := range exprs {
		tf, err := parseFilterExpr(expr)
		if err != nil {
			return nil, fmt.Errorf("cannot parse `expr` query arg: %w", err)
		}
		tfs = append(tfs, *tf)
	}
	return tfs, nil
}

func parseFilterExpr(s string) (*storage.TagFilter, error) {
	n := strings.Index(s, "=")
	if n < 0 {
		return nil, fmt.Errorf("missing tag value in filter expression %q", s)
	}
	tagName := s[:n]
	tagValue := s[n+1:]
	isNegative := false
	if strings.HasSuffix(tagName, "!") {
		isNegative = true
		tagName = tagName[:len(tagName)-1]
	}
	if tagName == "name" {
		tagName = ""
	}
	isRegexp := false
	if strings.HasPrefix(tagValue, "~") {
		isRegexp = true
		tagValue = "^(?:" + tagValue[1:] + ").*"
	}
	return &storage.TagFilter{
		Key:        []byte(tagName),
		Value:      []byte(tagValue),
		IsNegative: isNegative,
		IsRegexp:   isRegexp,
	}, nil
}

func joinTagFilterss(tfs []storage.TagFilter, extraFilters [][]storage.TagFilter) [][]storage.TagFilter {
	return searchutils.JoinTagFilterss([][]storage.TagFilter{tfs}, extraFilters)
}
