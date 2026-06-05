package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// openTSDBPoint is a single data point served by the mock OpenTSDB server.
type openTSDBPoint struct {
	Metric    string
	Tags      map[string]string
	Timestamp int64
	Value     float64
}

// openTSDBMockServer implements the minimal subset of the OpenTSDB HTTP API
// used by vmctl opentsdb: /api/suggest, /api/search/lookup, /api/query.
type openTSDBMockServer struct {
	server *httptest.Server
	points []openTSDBPoint
}

// newOpenTSDBMockServer starts an httptest server serving the given points.
func newOpenTSDBMockServer(t *testing.T, points []openTSDBPoint) *openTSDBMockServer {
	t.Helper()
	s := &openTSDBMockServer{points: points}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/suggest", s.handleSuggest)
	mux.HandleFunc("/api/search/lookup", s.handleLookup)
	mux.HandleFunc("/api/query", s.handleQuery)
	s.server = httptest.NewServer(mux)
	return s
}

// close shuts down the server.
func (s *openTSDBMockServer) close() { s.server.Close() }

// httpAddr returns the server URL.
func (s *openTSDBMockServer) httpAddr() string { return s.server.URL }

// handleSuggest serves https://opentsdb.net/docs/build/html/api_http/suggest.html
func (s *openTSDBMockServer) handleSuggest(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	seen := make(map[string]bool, len(s.points))
	var out []string
	for _, p := range s.points {
		if seen[p.Metric] {
			continue
		}
		if q != "" && !strings.Contains(p.Metric, q) {
			continue
		}
		seen[p.Metric] = true
		out = append(out, p.Metric)
	}
	_ = json.NewEncoder(w).Encode(out)
}

// handleLookup serves https://opentsdb.net/docs/build/html/api_http/search/lookup.html
func (s *openTSDBMockServer) handleLookup(w http.ResponseWriter, r *http.Request) {
	metric := r.URL.Query().Get("m")
	type meta struct {
		Metric string            `json:"metric"`
		Tags   map[string]string `json:"tags"`
	}
	seen := make(map[string]bool, len(s.points))
	var results []meta
	for _, p := range s.points {
		if p.Metric != metric {
			continue
		}
		key := tagsKey(p.Tags)
		if seen[key] {
			continue
		}
		seen[key] = true
		results = append(results, meta{p.Metric, p.Tags})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":    "LOOKUP",
		"metric":  metric,
		"results": results,
	})
}

// handleQuery serves https://opentsdb.net/docs/build/html/api_http/query/index.html
func (s *openTSDBMockServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	m := r.URL.Query().Get("m")
	metric, tagFilter, ok := parseQuery(m)
	if !ok {
		http.Error(w, "bad query param", http.StatusBadRequest)
		return
	}
	start, err := strconv.ParseInt(r.URL.Query().Get("start"), 10, 64)
	if err != nil {
		http.Error(w, "bad start param", http.StatusBadRequest)
		return
	}
	end, err := strconv.ParseInt(r.URL.Query().Get("end"), 10, 64)
	if err != nil {
		http.Error(w, "bad end param", http.StatusBadRequest)
		return
	}
	type resp struct {
		Metric        string             `json:"metric"`
		Tags          map[string]string  `json:"tags"`
		AggregateTags []string           `json:"aggregateTags"`
		Dps           map[string]float64 `json:"dps"`
	}
	grouped := make(map[string]*resp, len(s.points))
	for _, p := range s.points {
		if p.Metric != metric {
			continue
		}
		if !matchTags(p.Tags, tagFilter) {
			continue
		}
		if p.Timestamp < start || p.Timestamp > end {
			continue
		}
		key := tagsKey(p.Tags)
		if _, exists := grouped[key]; !exists {
			grouped[key] = &resp{
				Metric:        p.Metric,
				Tags:          p.Tags,
				AggregateTags: []string{},
				Dps:           map[string]float64{},
			}
		}
		grouped[key].Dps[fmt.Sprintf("%d", p.Timestamp)] = p.Value
	}
	out := make([]*resp, 0, len(grouped))
	for _, v := range grouped {
		out = append(out, v)
	}
	_ = json.NewEncoder(w).Encode(out)
}

// parseQuery parses the OpenTSDB m= query parameter.
// Format: "<agg>:<bucket>-<agg>-none:<metric>{k=v,k=v}"
func parseQuery(m string) (string, map[string]string, bool) {
	parts := strings.SplitN(m, ":", 3)
	if len(parts) != 3 {
		return "", nil, false
	}
	metric, tagStr, _ := strings.Cut(parts[2], "{")
	tags := make(map[string]string, 4)
	tagStr = strings.TrimSuffix(tagStr, "}")
	for _, kv := range strings.Split(tagStr, ",") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			tags[k] = v
		}
	}
	return metric, tags, true
}

func matchTags(got, filter map[string]string) bool {
	for k, v := range filter {
		if v == "*" {
			continue
		}
		if got[k] != v {
			return false
		}
	}
	return true
}

func tagsKey(tags map[string]string) string {
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+tags[k])
	}
	return strings.Join(parts, ",")
}
