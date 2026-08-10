package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// influxPoint is a single field value at a single timestamp of a single series,
// as served by the mock InfluxDB server.
type influxPoint struct {
	Measurement string
	Tags        map[string]string
	Field       string
	FieldType   string
	Timestamp   int64 // unix seconds
	Value       any   // float64, int64, bool or string
}

// influxRequest is a /query request recorded by the mock server, used to assert
// how vmctl authenticates and which database and retention policy it asks for.
type influxRequest struct {
	Query    url.Values
	User     string
	Password string
	HasAuth  bool
}

// influxMockServer implements the subset of the InfluxDB 1.x query API used by
// `vmctl influx`: /ping plus /query serving `SHOW FIELD KEYS`, `SHOW TAG KEYS`,
// `SHOW SERIES` and `SELECT`.
//
// InfluxDB 2.x is migrated through the very same API - its 1.x compatibility
// endpoint - so one mock covers both -influx-version=1 and -influx-version=2.
type influxMockServer struct {
	server *httptest.Server
	points []influxPoint

	mu       sync.Mutex
	requests []influxRequest
}

// newInfluxMockServer starts an httptest server serving the given points.
func newInfluxMockServer(t *testing.T, points []influxPoint) *influxMockServer {
	t.Helper()
	s := &influxMockServer{points: points}
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", s.handlePing)
	mux.HandleFunc("/query", s.handleQuery)
	s.server = httptest.NewServer(mux)
	return s
}

func (s *influxMockServer) close() { s.server.Close() }

func (s *influxMockServer) httpAddr() string { return s.server.URL }

// recordQuery stores a /query request for later assertions.
//
// Every request is kept, not just the last one: vmctl issues several kinds of
// query - `SHOW FIELD KEYS` unchunked, `SHOW TAG KEYS` and `SHOW SERIES`
// chunked, and one `SELECT` per series - and the assertions verify that all of
// them authenticate and address the database the same way. The assertions
// cannot live in this handler because it runs on a server goroutine, where
// t.Fatalf must not be called.
func (s *influxMockServer) recordQuery(r *http.Request) {
	user, pass, ok := r.BasicAuth()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, influxRequest{
		Query:    r.URL.Query(),
		User:     user,
		Password: pass,
		HasAuth:  ok,
	})
}

// queryRequests returns a copy of the recorded /query requests.
func (s *influxMockServer) queryRequests() []influxRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

// handlePing must answer 204: the client library treats any other status as a
// failed ping, and vmctl pings before querying.
//
// X-Influxdb-Version is set because real InfluxDB sets it. The client returns
// the value from Ping, but vmctl discards it, so it is not asserted anywhere and
// its content is arbitrary.
//
// The request is not recorded: /ping carries no db or rp to assert.
func (s *influxMockServer) handlePing(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("X-Influxdb-Version", "1.8.0-mock")
	w.WriteHeader(http.StatusNoContent)
}

// influxResponse mirrors the InfluxDB 1.x JSON query response.
type influxResponse struct {
	Results []influxResult `json:"results"`
}

type influxResult struct {
	StatementID int         `json:"statement_id"`
	Series      []influxRow `json:"series,omitempty"`
	Err         string      `json:"error,omitempty"`
}

type influxRow struct {
	Name    string   `json:"name,omitempty"`
	Columns []string `json:"columns"`
	Values  [][]any  `json:"values,omitempty"`
}

// parseSelect splits a SELECT built by vmctl into its field, measurement and
// WHERE clause. The statement shape is fixed by Series.fetchQuery:
//
//	select "value" from "cpu" where "host"::tag='h1' and "env"::tag=''
//
// Identifiers containing a double quote are not supported, which is fine
// because no test fixture uses one.
func parseSelect(q string) (field, measurement, where string, err error) {
	rest, ok := strings.CutPrefix(q, `select "`)
	if !ok {
		return "", "", "", fmt.Errorf("cannot parse select statement: %s", q)
	}
	field, rest, ok = strings.Cut(rest, `" from "`)
	if !ok {
		return "", "", "", fmt.Errorf("cannot parse measurement in: %s", q)
	}
	measurement, rest, ok = strings.Cut(rest, `"`)
	if !ok {
		return "", "", "", fmt.Errorf("unterminated measurement in: %s", q)
	}
	return field, measurement, strings.TrimPrefix(rest, " where "), nil
}

// handleQuery serves the InfluxQL statements vmctl issues. Both the plain and
// the chunked code paths of the client accept a single JSON response object,
// so one implementation covers both.
func (s *influxMockServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	s.recordQuery(r)

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	lower := strings.ToLower(q)

	// Content-Type is load-bearing: the client rejects any response that is not
	// application/json. X-Influxdb-Version only matters for 5xx replies, where
	// the client uses its absence to report a downstream proxy error instead of
	// an InfluxDB one; it is set here to mirror real InfluxDB.
	w.Header().Set("X-Influxdb-Version", "1.8.0-mock")
	w.Header().Set("Content-Type", "application/json")

	var resp influxResponse
	switch {
	case strings.HasPrefix(lower, "show field keys"):
		resp = influxResponse{Results: []influxResult{{Series: s.fieldKeys()}}}
	case strings.HasPrefix(lower, "show tag keys"):
		resp = influxResponse{Results: []influxResult{{Series: s.tagKeys()}}}
	case strings.HasPrefix(lower, "show series"):
		resp = influxResponse{Results: []influxResult{{Series: s.series()}}}
	case strings.HasPrefix(lower, "select"):
		rows, err := s.selectRows(q)
		if err != nil {
			resp = influxResponse{Results: []influxResult{{Err: err.Error()}}}
			break
		}
		resp = influxResponse{Results: []influxResult{{Series: rows}}}
	default:
		resp = influxResponse{Results: []influxResult{{Err: fmt.Sprintf("unsupported query: %s", q)}}}
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// fieldKeys serves `SHOW FIELD KEYS`: one row per measurement listing every
// field key together with its type. vmctl uses the type to skip string fields.
//
// Rows are built by walking the points in order, so the output is deterministic
// without sorting.
func (s *influxMockServer) fieldKeys() []influxRow {
	rows := make(map[string]*influxRow)
	var order []string
	seen := make(map[string]struct{})
	for _, p := range s.points {
		row, ok := rows[p.Measurement]
		if !ok {
			row = &influxRow{Name: p.Measurement, Columns: []string{"fieldKey", "fieldType"}}
			rows[p.Measurement] = row
			order = append(order, p.Measurement)
		}
		key := p.Measurement + "\x00" + p.Field
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		row.Values = append(row.Values, []any{p.Field, p.FieldType})
	}
	out := make([]influxRow, 0, len(order))
	for _, m := range order {
		out = append(out, *rows[m])
	}
	return out
}

// tagKeys serves `SHOW TAG KEYS`: one row per measurement listing its tag keys.
//
// Tag keys come from a map, so each row is sorted to keep the output stable.
func (s *influxMockServer) tagKeys() []influxRow {
	rows := make(map[string]*influxRow)
	var order []string
	seen := make(map[string]struct{})
	for _, p := range s.points {
		row, ok := rows[p.Measurement]
		if !ok {
			row = &influxRow{Name: p.Measurement, Columns: []string{"tagKey"}}
			rows[p.Measurement] = row
			order = append(order, p.Measurement)
		}
		for k := range p.Tags {
			key := p.Measurement + "\x00" + k
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			row.Values = append(row.Values, []any{k})
		}
	}
	out := make([]influxRow, 0, len(order))
	for _, m := range order {
		row := *rows[m]
		sort.Slice(row.Values, func(a, b int) bool {
			return row.Values[a][0].(string) < row.Values[b][0].(string)
		})
		out = append(out, row)
	}
	return out
}

// series serves `SHOW SERIES`: a single row of series keys in the
// `measurement,tag=value,...` form.
func (s *influxMockServer) series() []influxRow {
	seen := make(map[string]struct{})
	var keys []string
	for _, p := range s.points {
		key := influxSeriesKey(p.Measurement, p.Tags)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	row := influxRow{Columns: []string{"key"}}
	for _, k := range keys {
		row.Values = append(row.Values, []any{k})
	}
	return []influxRow{row}
}

// selectRows serves the per-series SELECT issued for every measurement/field
// combination, returning the `time` and field columns.
func (s *influxMockServer) selectRows(q string) ([]influxRow, error) {
	field, measurement, where, err := parseSelect(q)
	if err != nil {
		return nil, err
	}

	// Tags the series must carry. An empty value means the tag must be absent,
	// which is how vmctl addresses series that lack a tag of the measurement.
	wantTags := make(map[string]string)
	for _, cond := range strings.Split(where, " and ") {
		// Conditions that are not tag comparisons - such as the time filter
		// added by -influx-filter-time-start/-end - are not series selectors.
		name, value, ok := strings.Cut(cond, "::tag=")
		if !ok {
			continue
		}
		wantTags[strings.Trim(name, `"`)] = strings.Trim(value, `'`)
	}

	row := influxRow{Name: measurement, Columns: []string{"time", field}}
	for _, p := range s.points {
		if p.Measurement != measurement || p.Field != field {
			continue
		}
		if !influxTagsMatch(p.Tags, wantTags) {
			continue
		}
		ts := time.Unix(p.Timestamp, 0).UTC().Format(time.RFC3339)
		row.Values = append(row.Values, []any{ts, p.Value})
	}
	if len(row.Values) == 0 {
		return nil, nil
	}
	return []influxRow{row}, nil
}

// influxTagsMatch reports whether the series tags satisfy the conditions of a
// SELECT built by vmctl. Every requested tag must match exactly; a requested
// empty value requires the tag to be absent from the series.
func influxTagsMatch(got, want map[string]string) bool {
	for k, v := range want {
		if v == "" {
			if _, ok := got[k]; ok {
				return false
			}
			continue
		}
		if got[k] != v {
			return false
		}
	}
	// Every tag of the series must be constrained, otherwise the SELECT would
	// address more than one series.
	for k := range got {
		if _, ok := want[k]; !ok {
			return false
		}
	}
	return true
}

// influxSeriesKey builds the `measurement,tag=value,...` key used by SHOW SERIES.
//
// The tags must be sorted: the key doubles as the deduplication key, so an
// unstable order would report the same series more than once. tagsKey already
// sorts, so it is reused here.
func influxSeriesKey(measurement string, tags map[string]string) string {
	if len(tags) == 0 {
		return measurement
	}
	return measurement + "," + tagsKey(tags)
}
