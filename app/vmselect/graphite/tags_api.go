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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

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
	if len(exprs) == 0 {
		// Fast path: there are no `expr` filters.

		// Escape special chars in tagPrefix as Graphite does.
		// See https://github.com/graphite-project/graphite-web/blob/3ad279df5cb90b211953e39161df416e54a84948/webapp/graphite/tags/base.py#L181
		filter := regexp.QuoteMeta(tagPrefix)
		labels, err = netstorage.GetGraphiteTags(filter, limit, deadline)
		if err != nil {
			return err
		}
	} else {
		// Slow path: use netstorage.SearchMetricNames for applying `expr` filters.
		tfs, err := exprsToTagFilters(exprs)
		if err != nil {
			return err
		}
		ct := time.Now().UnixNano() / 1e6
		sq := &storage.SearchQuery{
			MinTimestamp: 0,
			MaxTimestamp: ct,
			TagFilterss:  [][]storage.TagFilter{tfs},
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
	WriteTagsAutoCompleteTagsResponse(bw, labels, jsonp)
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
	tfs, err := exprsToTagFilters(exprs)
	if err != nil {
		return err
	}
	ct := time.Now().UnixNano() / 1e6
	sq := &storage.SearchQuery{
		MinTimestamp: 0,
		MaxTimestamp: ct,
		TagFilterss:  [][]storage.TagFilter{tfs},
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
	var b []byte
	var tags []storage.Tag
	for _, mn := range mns {
		b = append(b[:0], mn.MetricGroup...)
		tags = append(tags[:0], mn.Tags...)
		sort.Slice(tags, func(i, j int) bool {
			return string(tags[i].Key) < string(tags[j].Key)
		})
		for _, tag := range tags {
			b = append(b, ';')
			b = append(b, tag.Key...)
			b = append(b, '=')
			b = append(b, tag.Value...)
		}
		paths = append(paths, string(b))
	}
	sort.Strings(paths)
	return paths
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
