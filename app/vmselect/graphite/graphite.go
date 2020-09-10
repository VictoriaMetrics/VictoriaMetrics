package graphite

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

// MetricsFindHandler implements /metrics/find handler.
//
// See https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find
func MetricsFindHandler(startTime time.Time, at *auth.Token, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	format := r.FormValue("format")
	if format == "" {
		format = "treejson"
	}
	switch format {
	case "treejson", "completer":
	default:
		return fmt.Errorf(`unexpected "format" query arg: %q; expecting "treejson" or "completer"`, format)
	}
	query := r.FormValue("query")
	if len(query) == 0 {
		return fmt.Errorf("expecting non-empty `query` arg")
	}
	delimiter := r.FormValue("delimiter")
	if delimiter == "" {
		delimiter = "."
	}
	if len(delimiter) > 1 {
		return fmt.Errorf("`delimiter` query arg must contain only a single char")
	}
	if searchutils.GetBool(r, "automatic_variants") {
		// See https://github.com/graphite-project/graphite-web/blob/bb9feb0e6815faa73f538af6ed35adea0fb273fd/webapp/graphite/metrics/views.py#L152
		query = addAutomaticVariants(query, delimiter)
	}
	if format == "completer" {
		// See https://github.com/graphite-project/graphite-web/blob/bb9feb0e6815faa73f538af6ed35adea0fb273fd/webapp/graphite/metrics/views.py#L148
		query = strings.ReplaceAll(query, "..", ".*")
		if !strings.HasSuffix(query, "*") {
			query += "*"
		}
	}
	leavesOnly := searchutils.GetBool(r, "leavesOnly")
	wildcards := searchutils.GetBool(r, "wildcards")
	label := r.FormValue("label")
	if label == "__name__" {
		label = ""
	}
	jsonp := r.FormValue("jsonp")
	from, err := searchutils.GetTime(r, "from", 0)
	if err != nil {
		return err
	}
	ct := startTime.UnixNano() / 1e6
	until, err := searchutils.GetTime(r, "until", ct)
	if err != nil {
		return err
	}
	tr := storage.TimeRange{
		MinTimestamp: from,
		MaxTimestamp: until,
	}
	paths, isPartial, err := metricsFind(at, tr, label, query, delimiter[0], deadline)
	if err != nil {
		return err
	}
	if isPartial && searchutils.GetDenyPartialResponse(r) {
		return fmt.Errorf("cannot return full response, since some of vmstorage nodes are unavailable")
	}
	if leavesOnly {
		paths = filterLeaves(paths, delimiter)
	}
	sortPaths(paths, delimiter)
	contentType := "application/json"
	if jsonp != "" {
		contentType = "text/javascript"
	}
	w.Header().Set("Content-Type", contentType)
	WriteMetricsFindResponse(w, paths, delimiter, format, wildcards, jsonp)
	metricsFindDuration.UpdateDuration(startTime)
	return nil
}

// MetricsExpandHandler implements /metrics/expand handler.
//
// See https://graphite-api.readthedocs.io/en/latest/api.html#metrics-expand
func MetricsExpandHandler(startTime time.Time, at *auth.Token, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	queries := r.Form["query"]
	if len(queries) == 0 {
		return fmt.Errorf("missing `query` arg")
	}
	groupByExpr := searchutils.GetBool(r, "groupByExpr")
	leavesOnly := searchutils.GetBool(r, "leavesOnly")
	label := r.FormValue("label")
	if label == "__name__" {
		label = ""
	}
	delimiter := r.FormValue("delimiter")
	if delimiter == "" {
		delimiter = "."
	}
	if len(delimiter) > 1 {
		return fmt.Errorf("`delimiter` query arg must contain only a single char")
	}
	jsonp := r.FormValue("jsonp")
	from, err := searchutils.GetTime(r, "from", 0)
	if err != nil {
		return err
	}
	ct := startTime.UnixNano() / 1e6
	until, err := searchutils.GetTime(r, "until", ct)
	if err != nil {
		return err
	}
	tr := storage.TimeRange{
		MinTimestamp: from,
		MaxTimestamp: until,
	}
	m := make(map[string][]string, len(queries))
	for _, query := range queries {
		paths, isPartial, err := metricsFind(at, tr, label, query, delimiter[0], deadline)
		if err != nil {
			return err
		}
		if isPartial && searchutils.GetDenyPartialResponse(r) {
			return fmt.Errorf("cannot return full response, since some of vmstorage nodes are unavailable")
		}
		if leavesOnly {
			paths = filterLeaves(paths, delimiter)
		}
		m[query] = paths
	}
	contentType := "application/json"
	if jsonp != "" {
		contentType = "text/javascript"
	}
	w.Header().Set("Content-Type", contentType)
	if groupByExpr {
		for _, paths := range m {
			sortPaths(paths, delimiter)
		}
		WriteMetricsExpandResponseByQuery(w, m, jsonp)
		return nil
	}
	paths := m[queries[0]]
	if len(m) > 1 {
		pathsSet := make(map[string]struct{})
		for _, paths := range m {
			for _, path := range paths {
				pathsSet[path] = struct{}{}
			}
		}
		paths = make([]string, 0, len(pathsSet))
		for path := range pathsSet {
			paths = append(paths, path)
		}
	}
	sortPaths(paths, delimiter)
	WriteMetricsExpandResponseFlat(w, paths, jsonp)
	metricsExpandDuration.UpdateDuration(startTime)
	return nil
}

// MetricsIndexHandler implements /metrics/index.json handler.
//
// See https://graphite-api.readthedocs.io/en/latest/api.html#metrics-index-json
func MetricsIndexHandler(startTime time.Time, at *auth.Token, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	jsonp := r.FormValue("jsonp")
	metricNames, isPartial, err := netstorage.GetLabelValues(at, "__name__", deadline)
	if err != nil {
		return fmt.Errorf(`cannot obtain metric names: %w`, err)
	}
	if isPartial && searchutils.GetDenyPartialResponse(r) {
		return fmt.Errorf("cannot return full response, since some of vmstorage nodes are unavailable")
	}
	contentType := "application/json"
	if jsonp != "" {
		contentType = "text/javascript"
	}
	w.Header().Set("Content-Type", contentType)
	WriteMetricsIndexResponse(w, metricNames, jsonp)
	metricsIndexDuration.UpdateDuration(startTime)
	return nil
}

// metricsFind searches for label values that match the given query.
func metricsFind(at *auth.Token, tr storage.TimeRange, label, query string, delimiter byte, deadline netstorage.Deadline) ([]string, bool, error) {
	expandTail := strings.HasSuffix(query, "*")
	for strings.HasSuffix(query, "*") {
		query = query[:len(query)-1]
	}
	var results []string
	n := strings.IndexAny(query, "*{[")
	if n < 0 {
		suffixes, isPartial, err := netstorage.GetTagValueSuffixes(at, tr, label, query, delimiter, deadline)
		if err != nil {
			return nil, false, err
		}
		if expandTail {
			for _, suffix := range suffixes {
				results = append(results, query+suffix)
			}
		} else if isFullMatch(query, suffixes, delimiter) {
			results = append(results, query)
		}
		return results, isPartial, nil
	}
	subquery := query[:n] + "*"
	paths, isPartial, err := metricsFind(at, tr, label, subquery, delimiter, deadline)
	if err != nil {
		return nil, false, err
	}
	tail := ""
	suffix := query[n:]
	if m := strings.IndexByte(suffix, delimiter); m >= 0 {
		tail = suffix[m+1:]
		suffix = suffix[:m+1]
	}
	q := query[:n] + suffix
	re, err := getRegexpForQuery(q, delimiter)
	if err != nil {
		return nil, false, fmt.Errorf("cannot convert query %q to regexp: %w", q, err)
	}
	if expandTail {
		tail += "*"
	}
	for _, path := range paths {
		if !re.MatchString(path) {
			continue
		}
		subquery := path + tail
		tmp, isPartialLocal, err := metricsFind(at, tr, label, subquery, delimiter, deadline)
		if err != nil {
			return nil, false, err
		}
		if isPartialLocal {
			isPartial = true
		}
		results = append(results, tmp...)
	}
	return results, isPartial, nil
}

var (
	metricsFindDuration   = metrics.NewSummary(`vm_request_duration_seconds{path="/select/{}/graphite/metrics/find"}`)
	metricsExpandDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/select/{}/graphite/metrics/expand"}`)
	metricsIndexDuration  = metrics.NewSummary(`vm_request_duration_seconds{path="/select/{}/graphite/metrics/expand"}`)
)

func isFullMatch(tagValuePrefix string, suffixes []string, delimiter byte) bool {
	if len(suffixes) == 0 {
		return false
	}
	if strings.LastIndexByte(tagValuePrefix, delimiter) == len(tagValuePrefix)-1 {
		return true
	}
	for _, suffix := range suffixes {
		if suffix == "" {
			return true
		}
	}
	return false
}

func addAutomaticVariants(query, delimiter string) string {
	// See https://github.com/graphite-project/graphite-web/blob/bb9feb0e6815faa73f538af6ed35adea0fb273fd/webapp/graphite/metrics/views.py#L152
	parts := strings.Split(query, delimiter)
	for i, part := range parts {
		if strings.Contains(part, ",") && !strings.Contains(part, "{") {
			parts[i] = "{" + part + "}"
		}
	}
	return strings.Join(parts, delimiter)
}

func filterLeaves(paths []string, delimiter string) []string {
	leaves := paths[:0]
	for _, path := range paths {
		if !strings.HasSuffix(path, delimiter) {
			leaves = append(leaves, path)
		}
	}
	return leaves
}

func sortPaths(paths []string, delimiter string) {
	sort.Slice(paths, func(i, j int) bool {
		a, b := paths[i], paths[j]
		isNodeA := strings.HasSuffix(a, delimiter)
		isNodeB := strings.HasSuffix(b, delimiter)
		if isNodeA == isNodeB {
			return a < b
		}
		return isNodeA
	})
}

func getRegexpForQuery(query string, delimiter byte) (*regexp.Regexp, error) {
	regexpCacheLock.Lock()
	defer regexpCacheLock.Unlock()

	k := regexpCacheKey{
		query:     query,
		delimiter: delimiter,
	}
	if re := regexpCache[k]; re != nil {
		return re.re, re.err
	}
	a := make([]string, 0, len(query))
	tillNextDelimiter := "[^" + regexp.QuoteMeta(string([]byte{delimiter})) + "]*"
	for i := 0; i < len(query); i++ {
		switch query[i] {
		case '*':
			a = append(a, tillNextDelimiter)
		case '{':
			tmp := query[i+1:]
			if n := strings.IndexByte(tmp, '}'); n < 0 {
				a = append(a, regexp.QuoteMeta(query[i:]))
				i = len(query)
			} else {
				a = append(a, "(?:")
				opts := strings.Split(tmp[:n], ",")
				for j, opt := range opts {
					opts[j] = regexp.QuoteMeta(opt)
				}
				a = append(a, strings.Join(opts, "|"))
				a = append(a, ")")
				i += n + 1
			}
		case '[':
			tmp := query[i:]
			if n := strings.IndexByte(tmp, ']'); n < 0 {
				a = append(a, regexp.QuoteMeta(query[i:]))
				i = len(query)
			} else {
				a = append(a, tmp[:n+1])
				i += n
			}
		default:
			a = append(a, regexp.QuoteMeta(query[i:i+1]))
		}
	}
	s := strings.Join(a, "")
	re, err := regexp.Compile(s)
	regexpCache[k] = &regexpCacheEntry{
		re:  re,
		err: err,
	}
	if len(regexpCache) >= maxRegexpCacheSize {
		for k := range regexpCache {
			if len(regexpCache) < maxRegexpCacheSize {
				break
			}
			delete(regexpCache, k)
		}
	}
	return re, err
}

type regexpCacheEntry struct {
	re  *regexp.Regexp
	err error
}

type regexpCacheKey struct {
	query     string
	delimiter byte
}

var regexpCache = make(map[regexpCacheKey]*regexpCacheEntry)
var regexpCacheLock sync.Mutex

const maxRegexpCacheSize = 10000
