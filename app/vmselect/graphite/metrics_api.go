package graphite

import (
	"flag"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
)

var maxTagValueSuffixes = flag.Int("search.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned from /metrics/find")

// MetricsFindHandler implements /metrics/find handler.
//
// See https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find
func MetricsFindHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
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
	if httputils.GetBool(r, "automatic_variants") {
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
	leavesOnly := httputils.GetBool(r, "leavesOnly")
	wildcards := httputils.GetBool(r, "wildcards")
	label := r.FormValue("label")
	if label == "__name__" {
		label = ""
	}
	jsonp := r.FormValue("jsonp")
	from, err := httputils.GetTime(r, "from", 0)
	if err != nil {
		return err
	}
	ct := startTime.UnixNano() / 1e6
	until, err := httputils.GetTime(r, "until", ct)
	if err != nil {
		return err
	}
	tr := storage.TimeRange{
		MinTimestamp: from,
		MaxTimestamp: until,
	}
	paths, err := metricsFind(tr, label, "", query, delimiter[0], false, deadline)
	if err != nil {
		return err
	}
	if leavesOnly {
		paths = filterLeaves(paths, delimiter)
	}
	paths = deduplicatePaths(paths)
	sortPaths(paths, delimiter)
	contentType := getContentType(jsonp)
	w.Header().Set("Content-Type", contentType)
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteMetricsFindResponse(bw, paths, delimiter, format, wildcards, jsonp)
	if err := bw.Flush(); err != nil {
		return err
	}
	metricsFindDuration.UpdateDuration(startTime)
	return nil
}

func deduplicatePaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	dst := paths[:1]
	for _, path := range paths[1:] {
		prevPath := dst[len(dst)-1]
		if path == prevPath {
			// Skip duplicate path.
			continue
		}
		dst = append(dst, path)
	}
	return dst
}

// MetricsExpandHandler implements /metrics/expand handler.
//
// See https://graphite-api.readthedocs.io/en/latest/api.html#metrics-expand
func MetricsExpandHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	queries := r.Form["query"]
	if len(queries) == 0 {
		return fmt.Errorf("missing `query` arg")
	}
	groupByExpr := httputils.GetBool(r, "groupByExpr")
	leavesOnly := httputils.GetBool(r, "leavesOnly")
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
	from, err := httputils.GetTime(r, "from", 0)
	if err != nil {
		return err
	}
	ct := startTime.UnixNano() / 1e6
	until, err := httputils.GetTime(r, "until", ct)
	if err != nil {
		return err
	}
	tr := storage.TimeRange{
		MinTimestamp: from,
		MaxTimestamp: until,
	}
	m := make(map[string][]string, len(queries))
	for _, query := range queries {
		paths, err := metricsFind(tr, label, "", query, delimiter[0], true, deadline)
		if err != nil {
			return err
		}
		if leavesOnly {
			paths = filterLeaves(paths, delimiter)
		}
		m[query] = paths
	}
	contentType := getContentType(jsonp)
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
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteMetricsExpandResponseFlat(bw, paths, jsonp)
	if err := bw.Flush(); err != nil {
		return err
	}
	metricsExpandDuration.UpdateDuration(startTime)
	return nil
}

// MetricsIndexHandler implements /metrics/index.json handler.
//
// See https://graphite-api.readthedocs.io/en/latest/api.html#metrics-index-json
func MetricsIndexHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	jsonp := r.FormValue("jsonp")
	sq := storage.NewSearchQuery(0, 0, nil, 0)
	metricNames, err := netstorage.LabelValues(nil, "__name__", sq, 0, deadline)
	if err != nil {
		return fmt.Errorf(`cannot obtain metric names: %w`, err)
	}
	contentType := getContentType(jsonp)
	w.Header().Set("Content-Type", contentType)
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteMetricsIndexResponse(bw, metricNames, jsonp)
	if err := bw.Flush(); err != nil {
		return err
	}
	metricsIndexDuration.UpdateDuration(startTime)
	return nil
}

// metricsFind searches for label values that match the given qHead and qTail.
func metricsFind(tr storage.TimeRange, label, qHead, qTail string, delimiter byte, isExpand bool, deadline searchutils.Deadline) ([]string, error) {
	n := strings.IndexAny(qTail, "*{[")
	if n < 0 {
		query := qHead + qTail
		suffixes, err := netstorage.TagValueSuffixes(nil, tr, label, query, delimiter, *maxTagValueSuffixes, deadline)
		if err != nil {
			return nil, err
		}
		if len(suffixes) == 0 {
			return nil, nil
		}
		if len(query) > 0 && query[len(query)-1] == delimiter {
			return []string{query}, nil
		}
		results := make([]string, 0, len(suffixes))
		for _, suffix := range suffixes {
			if len(suffix) == 0 || len(suffix) == 1 && suffix[0] == delimiter {
				results = append(results, query+suffix)
			}
		}
		return results, nil
	}
	if n == len(qTail)-1 && strings.HasSuffix(qTail, "*") {
		query := qHead + qTail[:len(qTail)-1]
		suffixes, err := netstorage.TagValueSuffixes(nil, tr, label, query, delimiter, *maxTagValueSuffixes, deadline)
		if err != nil {
			return nil, err
		}
		if len(suffixes) == 0 {
			return nil, nil
		}
		results := make([]string, 0, len(suffixes))
		for _, suffix := range suffixes {
			results = append(results, query+suffix)
		}
		return results, nil
	}
	qHead += qTail[:n]
	paths, err := metricsFind(tr, label, qHead, "*", delimiter, isExpand, deadline)
	if err != nil {
		return nil, err
	}
	suffix := qTail[n:]
	qTail = ""
	if m := strings.IndexByte(suffix, delimiter); m >= 0 {
		qTail = suffix[m+1:]
		suffix = suffix[:m+1]
	}
	qPrefix := qHead + suffix
	rePrefix, err := getRegexpForQuery(qPrefix, delimiter)
	if err != nil {
		return nil, fmt.Errorf("cannot convert query %q to regexp: %w", qPrefix, err)
	}
	results := make([]string, 0, len(paths))
	for _, path := range paths {
		if !rePrefix.MatchString(path) {
			continue
		}
		if qTail == "" {
			results = append(results, path)
			continue
		}
		fullPaths, err := metricsFind(tr, label, path, qTail, delimiter, isExpand, deadline)
		if err != nil {
			return nil, err
		}
		if isExpand {
			results = append(results, fullPaths...)
		} else {
			for _, fullPath := range fullPaths {
				results = append(results, qPrefix+fullPath[len(path):])
			}
		}
	}
	return results, nil
}

var (
	metricsFindDuration   = metrics.NewSummary(`vm_request_duration_seconds{path="/metrics/find"}`)
	metricsExpandDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/metrics/expand"}`)
	metricsIndexDuration  = metrics.NewSummary(`vm_request_duration_seconds{path="/metrics/index.json"}`)
)

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
	rs, tail := getRegexpStringForQuery(query, delimiter, false)
	if len(tail) > 0 {
		return nil, fmt.Errorf("unexpected tail left after parsing query %q; tail: %q", query, tail)
	}
	re, err := metricsql.CompileRegexp(rs)
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

func getRegexpStringForQuery(query string, delimiter byte, isSubquery bool) (string, string) {
	var a []string
	var tail string
	quotedDelimiter := regexp.QuoteMeta(string([]byte{delimiter}))
	for {
		n := strings.IndexAny(query, "*{[,}")
		if n < 0 {
			a = append(a, regexp.QuoteMeta(query))
			tail = ""
			goto end
		}
		a = append(a, regexp.QuoteMeta(query[:n]))
		query = query[n:]
		switch query[0] {
		case ',', '}':
			if isSubquery {
				tail = query
				goto end
			}
			a = append(a, regexp.QuoteMeta(query[:1]))
			query = query[1:]
		case '*':
			a = append(a, "[^"+quotedDelimiter+"]*")
			query = query[1:]
		case '{':
			var opts []string
			for {
				var x string
				x, tail = getRegexpStringForQuery(query[1:], delimiter, true)
				opts = append(opts, x)
				if len(tail) == 0 {
					a = append(a, regexp.QuoteMeta("{"))
					a = append(a, strings.Join(opts, ","))
					goto end
				}
				if tail[0] == ',' {
					query = tail
					continue
				}
				if tail[0] == '}' {
					a = append(a, "(?:"+strings.Join(opts, "|")+")")
					query = tail[1:]
					break
				}
				logger.Panicf("BUG: unexpected first char at tail %q; want `.` or `}`", tail)
			}
		case '[':
			n := strings.IndexByte(query, ']')
			if n < 0 {
				a = append(a, regexp.QuoteMeta(query))
				tail = ""
				goto end
			}
			a = append(a, query[:n+1])
			query = query[n+1:]
		}
	}
end:
	s := strings.Join(a, "")
	if isSubquery {
		return s, tail
	}
	if !strings.HasSuffix(s, quotedDelimiter) {
		s += quotedDelimiter + "?"
	}
	s = "^" + s + "$"
	return s, tail
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

func getContentType(jsonp string) string {
	if jsonp == "" {
		return "application/json"
	}
	return "text/javascript; charset=utf-8"
}
