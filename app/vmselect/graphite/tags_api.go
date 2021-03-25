package graphite

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	graphiteparser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

// TagsDelSeriesHandler implements /tags/delSeries handler.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#removing-series-from-the-tagdb
func TagsDelSeriesHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	paths := r.Form["path"]
	totalDeleted := 0
	var row graphiteparser.Row
	var tagsPool []graphiteparser.Tag
	ct := startTime.UnixNano() / 1e6
	etfs, err := searchutils.GetEnforcedTagFiltersFromRequest(r)
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
		tfs = append(tfs, etfs...)
		sq := storage.NewSearchQuery(0, ct, [][]storage.TagFilter{tfs})
		n, err := netstorage.DeleteSeries(sq, deadline)
		if err != nil {
			return fmt.Errorf("cannot delete series for %q: %w", sq, err)
		}
		totalDeleted += n
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
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
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
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

// TagsAutoCompleteValuesHandler implements /tags/autoComplete/values endpoint from Graphite Tags API.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#auto-complete-support
func TagsAutoCompleteValuesHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	limit, err := getInt(r, "limit")
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
	etfs, err := searchutils.GetEnforcedTagFiltersFromRequest(r)
	if err != nil {
		return fmt.Errorf("cannot setup tag filters: %w", err)
	}
	if len(exprs) == 0 && len(etfs) == 0 {
		// Fast path: there are no `expr` filters, so use netstorage.GetGraphiteTagValues.
		// Escape special chars in tagPrefix as Graphite does.
		// See https://github.com/graphite-project/graphite-web/blob/3ad279df5cb90b211953e39161df416e54a84948/webapp/graphite/tags/base.py#L228
		filter := regexp.QuoteMeta(valuePrefix)
		tagValues, err = netstorage.GetGraphiteTagValues(tag, filter, limit, deadline)
		if err != nil {
			return err
		}
	} else {
		// Slow path: use netstorage.SearchMetricNames for applying `expr` filters.
		sq, err := getSearchQueryForExprs(startTime, etfs, exprs)
		if err != nil {
			return err
		}
		mns, err := netstorage.SearchMetricNames(sq, deadline)
		if err != nil {
			return fmt.Errorf("cannot fetch metric names for %q: %w", sq, err)
		}
		m := make(map[string]struct{})
		if tag == "name" {
			tag = "__name__"
		}
		for _, mn := range mns {
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
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	limit, err := getInt(r, "limit")
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
	etfs, err := searchutils.GetEnforcedTagFiltersFromRequest(r)
	if err != nil {
		return fmt.Errorf("cannot setup tag filters: %w", err)
	}
	if len(exprs) == 0 && len(etfs) == 0 {
		// Fast path: there are no `expr` filters, so use netstorage.GetGraphiteTags.

		// Escape special chars in tagPrefix as Graphite does.
		// See https://github.com/graphite-project/graphite-web/blob/3ad279df5cb90b211953e39161df416e54a84948/webapp/graphite/tags/base.py#L181
		filter := regexp.QuoteMeta(tagPrefix)
		labels, err = netstorage.GetGraphiteTags(filter, limit, deadline)
		if err != nil {
			return err
		}
	} else {
		// Slow path: use netstorage.SearchMetricNames for applying `expr` filters.
		sq, err := getSearchQueryForExprs(startTime, etfs, exprs)
		if err != nil {
			return err
		}
		mns, err := netstorage.SearchMetricNames(sq, deadline)
		if err != nil {
			return fmt.Errorf("cannot fetch metric names for %q: %w", sq, err)
		}
		m := make(map[string]struct{})
		for _, mn := range mns {
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
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	limit, err := getInt(r, "limit")
	if err != nil {
		return err
	}
	exprs := r.Form["expr"]
	if len(exprs) == 0 {
		return fmt.Errorf("expecting at least one `expr` query arg")
	}
	etfs, err := searchutils.GetEnforcedTagFiltersFromRequest(r)
	if err != nil {
		return fmt.Errorf("cannot setup tag filters: %w", err)
	}
	sq, err := getSearchQueryForExprs(startTime, etfs, exprs)
	if err != nil {
		return err
	}
	mns, err := netstorage.SearchMetricNames(sq, deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch metric names for %q: %w", sq, err)
	}
	paths := getCanonicalPaths(mns)
	if limit > 0 && limit < len(paths) {
		paths = paths[:limit]
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTagsFindSeriesResponse(bw, paths)
	if err := bw.Flush(); err != nil {
		return err
	}
	tagsFindSeriesDuration.UpdateDuration(startTime)
	return nil
}

func getCanonicalPaths(mns []storage.MetricName) []string {
	paths := make([]string, 0, len(mns))
	for _, mn := range mns {
		path := getCanonicalPath(&mn)
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
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
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	limit, err := getInt(r, "limit")
	if err != nil {
		return err
	}
	filter := r.FormValue("filter")
	tagValues, err := netstorage.GetGraphiteTagValues(tagName, filter, limit, deadline)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
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
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	limit, err := getInt(r, "limit")
	if err != nil {
		return err
	}
	filter := r.FormValue("filter")
	labels, err := netstorage.GetGraphiteTags(filter, limit, deadline)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
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

func getInt(r *http.Request, argName string) (int, error) {
	argValue := r.FormValue(argName)
	if len(argValue) == 0 {
		return 0, nil
	}
	n, err := strconv.Atoi(argValue)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q=%q: %w", argName, argValue, err)
	}
	return n, nil
}

func getSearchQueryForExprs(startTime time.Time, etfs []storage.TagFilter, exprs []string) (*storage.SearchQuery, error) {
	tfs, err := exprsToTagFilters(exprs)
	if err != nil {
		return nil, err
	}
	ct := startTime.UnixNano() / 1e6
	tfs = append(tfs, etfs...)
	sq := storage.NewSearchQuery(0, ct, [][]storage.TagFilter{tfs})
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
