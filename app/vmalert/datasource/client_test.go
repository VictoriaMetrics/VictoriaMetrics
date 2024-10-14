package datasource

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

var (
	ctx           = context.Background()
	basicAuthName = "foo"
	basicAuthPass = "bar"
	baCfg         = &promauth.BasicAuthConfig{
		Username: basicAuthName,
		Password: promauth.NewSecret(basicAuthPass),
	}
	vmQuery         = "vm_rows"
	queryRender     = "constantLine(10)"
	vlogsQuery      = "_time: 5m | stats by (foo) count() total"
	vlogsRangeQuery = "* | stats by (foo) count() total"
)

func TestVMInstantQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatalf("should not be called")
	})
	c := -1
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		c++
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method got %s", r.Method)
		}
		if name, pass, _ := r.BasicAuth(); name != basicAuthName || pass != basicAuthPass {
			t.Fatalf("expected %s:%s as basic auth got %s:%s", basicAuthName, basicAuthPass, name, pass)
		}
		if r.URL.Query().Get("query") != vmQuery {
			t.Fatalf("expected %s in query param, got %s", vmQuery, r.URL.Query().Get("query"))
		}
		timeParam := r.URL.Query().Get("time")
		if timeParam == "" {
			t.Fatalf("expected 'time' in query param, got nil instead")
		}
		if _, err := time.Parse(time.RFC3339, timeParam); err != nil {
			t.Fatalf("failed to parse 'time' query param %q: %s", timeParam, err)
		}
		switch c {
		case 0:
			w.WriteHeader(500)
		case 1:
			w.Write([]byte("[]"))
		case 2:
			w.Write([]byte(`{"status":"error", "errorType":"type:", "error":"some error msg"}`))
		case 3:
			w.Write([]byte(`{"status":"unknown"}`))
		case 4:
			w.Write([]byte(`{"status":"success","data":{"resultType":"matrix"}}`))
		case 5:
			w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"vm_rows","foo":"bar"},"value":[1583786142,"13763"]},{"metric":{"__name__":"vm_requests","foo":"baz"},"value":[1583786140,"2000"]}]}}`))
		case 6:
			w.Write([]byte(`{"status":"success","data":{"resultType":"scalar","result":[1583786142, "1"]}}`))
		case 7:
			w.Write([]byte(`{"status":"success","data":{"resultType":"scalar","result":[1583786142, "1"]},"stats":{"seriesFetched": "42"}}`))
		}
	})
	mux.HandleFunc("/render", func(w http.ResponseWriter, _ *http.Request) {
		c++
		switch c {
		case 8:
			w.Write([]byte(`[{"target":"constantLine(10)","tags":{"name":"constantLine(10)"},"datapoints":[[10,1611758343],[10,1611758373],[10,1611758403]]}]`))
		}
	})
	mux.HandleFunc("/select/logsql/stats_query", func(w http.ResponseWriter, r *http.Request) {
		c++
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method got %s", r.Method)
		}
		if name, pass, _ := r.BasicAuth(); name != basicAuthName || pass != basicAuthPass {
			t.Fatalf("expected %s:%s as basic auth got %s:%s", basicAuthName, basicAuthPass, name, pass)
		}
		if r.URL.Query().Get("query") != vlogsQuery {
			t.Fatalf("expected %s in query param, got %s", vlogsQuery, r.URL.Query().Get("query"))
		}
		timeParam := r.URL.Query().Get("time")
		if timeParam == "" {
			t.Fatalf("expected 'time' in query param, got nil instead")
		}
		if _, err := time.Parse(time.RFC3339, timeParam); err != nil {
			t.Fatalf("failed to parse 'time' query param %q: %s", timeParam, err)
		}
		switch c {
		case 9:
			w.Write([]byte("[]"))
		case 10:
			w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"total","foo":"bar"},"value":[1583786142,"13763"]},{"metric":{"__name__":"total","foo":"baz"},"value":[1583786140,"2000"]}]}}`))
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	authCfg, err := baCfg.NewConfig(".")
	if err != nil {
		t.Fatalf("unexpected: %s", err)
	}
	s := NewPrometheusClient(srv.URL, authCfg, false, srv.Client())

	p := datasourcePrometheus
	pq := s.BuildWithParams(QuerierParams{DataSourceType: string(p), EvaluationInterval: 15 * time.Second})
	ts := time.Now()

	expErr := func(query, err string) {
		_, _, gotErr := pq.Query(ctx, query, ts)
		if gotErr == nil {
			t.Fatalf("expected %q got nil", err)
		}
		if !strings.Contains(gotErr.Error(), err) {
			t.Fatalf("expected err %q; got %q", err, gotErr)
		}
	}

	expErr(vmQuery, "500")                          // 0
	expErr(vmQuery, "error parsing response")       // 1
	expErr(vmQuery, "response error")               // 2
	expErr(vmQuery, "unknown status")               // 3
	expErr(vmQuery, "unexpected end of JSON input") // 4

	res, _, err := pq.Query(ctx, vmQuery, ts) // 5 - vector
	if err != nil {
		t.Fatalf("unexpected %s", err)
	}
	if len(res.Data) != 2 {
		t.Fatalf("expected 2 metrics got %d in %+v", len(res.Data), res.Data)
	}
	expected := []Metric{
		{
			Labels:     []Label{{Value: "vm_rows", Name: "__name__"}, {Value: "bar", Name: "foo"}},
			Timestamps: []int64{1583786142},
			Values:     []float64{13763},
		},
		{
			Labels:     []Label{{Value: "vm_requests", Name: "__name__"}, {Value: "baz", Name: "foo"}},
			Timestamps: []int64{1583786140},
			Values:     []float64{2000},
		},
	}
	metricsEqual(t, res.Data, expected)

	res, req, err := pq.Query(ctx, vmQuery, ts) // 6 - scalar
	if err != nil {
		t.Fatalf("unexpected %s", err)
	}
	if req == nil {
		t.Fatalf("expected request to be non-nil")
	}
	if len(res.Data) != 1 {
		t.Fatalf("expected 1 metrics got %d in %+v", len(res.Data), res.Data)
	}
	expected = []Metric{
		{
			Timestamps: []int64{1583786142},
			Values:     []float64{1},
		},
	}
	if !reflect.DeepEqual(res.Data, expected) {
		t.Fatalf("unexpected metric %+v want %+v", res.Data, expected)
	}

	if res.SeriesFetched != nil {
		t.Fatalf("expected `seriesFetched` field to be nil when it is missing in datasource response; got %v instead",
			res.SeriesFetched)
	}

	res, _, err = pq.Query(ctx, vmQuery, ts) // 7 - scalar with stats
	if err != nil {
		t.Fatalf("unexpected %s", err)
	}
	if len(res.Data) != 1 {
		t.Fatalf("expected 1 metrics got %d in %+v", len(res.Data), res)
	}
	expected = []Metric{
		{
			Timestamps: []int64{1583786142},
			Values:     []float64{1},
		},
	}
	if !reflect.DeepEqual(res.Data, expected) {
		t.Fatalf("unexpected metric %+v want %+v", res.Data, expected)
	}
	if *res.SeriesFetched != 42 {
		t.Fatalf("expected `seriesFetched` field to be 42; got %d instead",
			*res.SeriesFetched)
	}

	// test graphite
	gq := s.BuildWithParams(QuerierParams{DataSourceType: string(datasourceGraphite)})

	res, _, err = gq.Query(ctx, queryRender, ts) // 8 - graphite
	if err != nil {
		t.Fatalf("unexpected %s", err)
	}
	if len(res.Data) != 1 {
		t.Fatalf("expected 1 metric  got %d in %+v", len(res.Data), res.Data)
	}
	exp := []Metric{
		{
			Labels:     []Label{{Value: "constantLine(10)", Name: "name"}},
			Timestamps: []int64{1611758403},
			Values:     []float64{10},
		},
	}
	metricsEqual(t, res.Data, exp)

	// test victorialogs
	vlogs := datasourceVLogs
	pq = s.BuildWithParams(QuerierParams{DataSourceType: string(vlogs), EvaluationInterval: 15 * time.Second})

	expErr(vlogsQuery, "error parsing response") // 9

	res, _, err = pq.Query(ctx, vlogsQuery, ts) // 10
	if err != nil {
		t.Fatalf("unexpected %s", err)
	}
	if len(res.Data) != 2 {
		t.Fatalf("expected 2 metrics got %d in %+v", len(res.Data), res.Data)
	}
	expected = []Metric{
		{
			Labels:     []Label{{Value: "total", Name: "stats_result"}, {Value: "bar", Name: "foo"}},
			Timestamps: []int64{1583786142},
			Values:     []float64{13763},
		},
		{
			Labels:     []Label{{Value: "total", Name: "stats_result"}, {Value: "baz", Name: "foo"}},
			Timestamps: []int64{1583786140},
			Values:     []float64{2000},
		},
	}
	metricsEqual(t, res.Data, expected)
}

func TestVMInstantQueryWithRetry(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatalf("should not be called")
	})
	c := -1
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		c++
		if r.URL.Query().Get("query") != vmQuery {
			t.Fatalf("expected %s in query param, got %s", vmQuery, r.URL.Query().Get("query"))
		}
		switch c {
		case 0:
			w.Write([]byte(`{"status":"success","data":{"resultType":"scalar","result":[1583786142, "1"]}}`))
		case 1:
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
		case 2:
			w.Write([]byte(`{"status":"success","data":{"resultType":"scalar","result":[1583786142, "2"]}}`))
		case 3:
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
		case 4:
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := NewPrometheusClient(srv.URL, nil, false, srv.Client())
	pq := s.BuildWithParams(QuerierParams{DataSourceType: string(datasourcePrometheus)})

	expErr := func(err string) {
		_, _, gotErr := pq.Query(ctx, vmQuery, time.Now())
		if gotErr == nil {
			t.Fatalf("expected %q got nil", err)
		}
		if !strings.Contains(gotErr.Error(), err) {
			t.Fatalf("expected err %q; got %q", err, gotErr)
		}
	}

	expValue := func(v float64) {
		res, _, err := pq.Query(ctx, vmQuery, time.Now())
		if err != nil {
			t.Fatalf("unexpected %s", err)
		}
		m := res.Data
		if len(m) != 1 {
			t.Fatalf("expected 1 metrics got %d in %+v", len(m), m)
		}
		expected := []Metric{
			{
				Timestamps: []int64{1583786142},
				Values:     []float64{v},
			},
		}
		if !reflect.DeepEqual(m, expected) {
			t.Fatalf("unexpected metric %+v want %+v", m, expected)
		}
	}

	expValue(1)   // 0
	expValue(2)   // 1 - fail, 2 - retry
	expErr("EOF") // 3, 4 - retries
}

func metricsEqual(t *testing.T, gotM, expectedM []Metric) {
	for i, exp := range expectedM {
		got := gotM[i]
		gotTS, expTS := got.Timestamps, exp.Timestamps
		if !reflect.DeepEqual(gotTS, expTS) {
			t.Fatalf("unexpected timestamps %+v want %+v", gotTS, expTS)
		}
		gotV, expV := got.Values, exp.Values
		if !reflect.DeepEqual(gotV, expV) {
			t.Fatalf("unexpected values %+v want %+v", gotV, expV)
		}
		sort.Slice(got.Labels, func(i, j int) bool {
			return got.Labels[i].Name < got.Labels[j].Name
		})
		sort.Slice(exp.Labels, func(i, j int) bool {
			return exp.Labels[i].Name < exp.Labels[j].Name
		})
		if !reflect.DeepEqual(exp.Labels, got.Labels) {
			t.Fatalf("unexpected labels %+v want %+v", got.Labels, exp.Labels)
		}
	}
}

func TestVMRangeQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatalf("should not be called")
	})
	c := -1
	mux.HandleFunc("/api/v1/query_range", func(w http.ResponseWriter, r *http.Request) {
		c++
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method got %s", r.Method)
		}
		if name, pass, _ := r.BasicAuth(); name != basicAuthName || pass != basicAuthPass {
			t.Fatalf("expected %s:%s as basic auth got %s:%s", basicAuthName, basicAuthPass, name, pass)
		}
		if r.URL.Query().Get("query") != vmQuery {
			t.Fatalf("expected %s in query param, got %s", vmQuery, r.URL.Query().Get("query"))
		}
		startTS := r.URL.Query().Get("start")
		if startTS == "" {
			t.Fatalf("expected 'start' in query param, got nil instead")
		}
		if _, err := time.Parse(time.RFC3339, startTS); err != nil {
			t.Fatalf("failed to parse 'start' query param: %s", err)
		}
		endTS := r.URL.Query().Get("end")
		if endTS == "" {
			t.Fatalf("expected 'end' in query param, got nil instead")
		}
		if _, err := time.Parse(time.RFC3339, endTS); err != nil {
			t.Fatalf("failed to parse 'end' query param: %s", err)
		}
		step := r.URL.Query().Get("step")
		if step != "15s" {
			t.Fatalf("expected 'step' query param to be 15s; got %q instead", step)
		}
		switch c {
		case 0:
			w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"vm_rows"},"values":[[1583786142,"13763"]]}]}}`))
		}
	})
	mux.HandleFunc("/select/logsql/stats_query_range", func(w http.ResponseWriter, r *http.Request) {
		c++
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method got %s", r.Method)
		}
		if name, pass, _ := r.BasicAuth(); name != basicAuthName || pass != basicAuthPass {
			t.Fatalf("expected %s:%s as basic auth got %s:%s", basicAuthName, basicAuthPass, name, pass)
		}
		if r.URL.Query().Get("query") != vlogsRangeQuery {
			t.Fatalf("expected %s in query param, got %s", vmQuery, r.URL.Query().Get("query"))
		}
		startTS := r.URL.Query().Get("start")
		if startTS == "" {
			t.Fatalf("expected 'start' in query param, got nil instead")
		}
		if _, err := time.Parse(time.RFC3339, startTS); err != nil {
			t.Fatalf("failed to parse 'start' query param: %s", err)
		}
		endTS := r.URL.Query().Get("end")
		if endTS == "" {
			t.Fatalf("expected 'end' in query param, got nil instead")
		}
		if _, err := time.Parse(time.RFC3339, endTS); err != nil {
			t.Fatalf("failed to parse 'end' query param: %s", err)
		}
		step := r.URL.Query().Get("step")
		if step != "60s" {
			t.Fatalf("expected 'step' query param to be 60s; got %q instead", step)
		}
		switch c {
		case 1:
			w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"total"},"values":[[1583786142,"10"]]}]}}`))
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	authCfg, err := baCfg.NewConfig(".")
	if err != nil {
		t.Fatalf("unexpected: %s", err)
	}
	s := NewPrometheusClient(srv.URL, authCfg, false, srv.Client())

	pq := s.BuildWithParams(QuerierParams{DataSourceType: string(datasourcePrometheus), EvaluationInterval: 15 * time.Second})

	_, err = pq.QueryRange(ctx, vmQuery, time.Now(), time.Time{})
	expectError(t, err, "is missing")

	_, err = pq.QueryRange(ctx, vmQuery, time.Time{}, time.Now())
	expectError(t, err, "is missing")

	start, end := time.Now().Add(-time.Minute), time.Now()

	res, err := pq.QueryRange(ctx, vmQuery, start, end)
	if err != nil {
		t.Fatalf("unexpected %s", err)
	}
	m := res.Data
	if len(m) != 1 {
		t.Fatalf("expected 1 metric  got %d in %+v", len(m), m)
	}
	expected := Metric{
		Labels:     []Label{{Value: "vm_rows", Name: "__name__"}},
		Timestamps: []int64{1583786142},
		Values:     []float64{13763},
	}
	if !reflect.DeepEqual(m[0], expected) {
		t.Fatalf("unexpected metric %+v want %+v", m[0], expected)
	}

	// test unsupported graphite
	gq := s.BuildWithParams(QuerierParams{DataSourceType: string(datasourceGraphite)})

	_, err = gq.QueryRange(ctx, queryRender, start, end)
	expectError(t, err, "is not supported")

	// test victorialogs
	gq = s.BuildWithParams(QuerierParams{DataSourceType: string(datasourceVLogs), EvaluationInterval: 60 * time.Second})

	res, err = gq.QueryRange(ctx, vlogsRangeQuery, start, end)
	if err != nil {
		t.Fatalf("unexpected %s", err)
	}
	m = res.Data
	if len(m) != 1 {
		t.Fatalf("expected 1 metric  got %d in %+v", len(m), m)
	}
	expected = Metric{
		Labels:     []Label{{Value: "total", Name: "stats_result"}},
		Timestamps: []int64{1583786142},
		Values:     []float64{10},
	}
	if !reflect.DeepEqual(m[0], expected) {
		t.Fatalf("unexpected metric %+v want %+v", m[0], expected)
	}
}

func TestRequestParams(t *testing.T) {
	query := "up"
	vlogsQuery := "_time: 5m | stats count() total"
	timestamp := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)

	f := func(isQueryRange bool, c *Client, checkFn func(t *testing.T, r *http.Request)) {
		t.Helper()

		req, err := c.newRequest(ctx)
		if err != nil {
			t.Fatalf("error in newRequest: %s", err)
		}

		switch c.dataSourceType {
		case datasourcePrometheus:
			if isQueryRange {
				c.setPrometheusRangeReqParams(req, query, timestamp, timestamp)
			} else {
				c.setPrometheusInstantReqParams(req, query, timestamp)
			}
		case datasourceGraphite:
			c.setGraphiteReqParams(req, query)
		case datasourceVLogs:
			if isQueryRange {
				c.setVLogsRangeReqParams(req, vlogsQuery, timestamp, timestamp)
			} else {
				c.setVLogsInstantReqParams(req, vlogsQuery, timestamp)
			}
		}

		checkFn(t, req)
	}

	authCfg, err := baCfg.NewConfig(".")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	storage := Client{
		extraParams: url.Values{"round_digits": {"10"}},
	}

	// prometheus path
	f(false, &Client{
		dataSourceType: datasourcePrometheus,
	}, func(t *testing.T, r *http.Request) {
		checkEqualString(t, "/api/v1/query", r.URL.Path)
	})

	// prometheus prefix
	f(false, &Client{
		dataSourceType:   datasourcePrometheus,
		appendTypePrefix: true,
	}, func(t *testing.T, r *http.Request) {
		checkEqualString(t, "/prometheus/api/v1/query", r.URL.Path)
	})

	// prometheus range path
	f(true, &Client{
		dataSourceType: datasourcePrometheus,
	}, func(t *testing.T, r *http.Request) {
		checkEqualString(t, "/api/v1/query_range", r.URL.Path)
	})

	// prometheus range prefix
	f(true, &Client{
		dataSourceType:   datasourcePrometheus,
		appendTypePrefix: true,
	}, func(t *testing.T, r *http.Request) {
		checkEqualString(t, "/prometheus/api/v1/query_range", r.URL.Path)
	})

	// graphite path
	f(false, &Client{
		dataSourceType: datasourceGraphite,
	}, func(t *testing.T, r *http.Request) {
		checkEqualString(t, graphitePath, r.URL.Path)
	})

	// graphite prefix
	f(false, &Client{
		dataSourceType:   datasourceGraphite,
		appendTypePrefix: true,
	}, func(t *testing.T, r *http.Request) {
		checkEqualString(t, graphitePrefix+graphitePath, r.URL.Path)
	})

	// default params
	f(false, &Client{dataSourceType: datasourcePrometheus}, func(t *testing.T, r *http.Request) {
		exp := url.Values{"query": {query}, "time": {timestamp.Format(time.RFC3339)}}
		checkEqualString(t, exp.Encode(), r.URL.RawQuery)
	})

	// default range params
	f(true, &Client{dataSourceType: datasourcePrometheus}, func(t *testing.T, r *http.Request) {
		ts := timestamp.Format(time.RFC3339)
		exp := url.Values{"query": {query}, "start": {ts}, "end": {ts}}
		checkEqualString(t, exp.Encode(), r.URL.RawQuery)
	})

	// basic auth
	f(false, &Client{
		dataSourceType: datasourcePrometheus,
		authCfg:        authCfg,
	}, func(t *testing.T, r *http.Request) {
		u, p, _ := r.BasicAuth()
		checkEqualString(t, "foo", u)
		checkEqualString(t, "bar", p)
	})

	// basic auth range
	f(true, &Client{
		dataSourceType: datasourcePrometheus,
		authCfg:        authCfg,
	}, func(t *testing.T, r *http.Request) {
		u, p, _ := r.BasicAuth()
		checkEqualString(t, "foo", u)
		checkEqualString(t, "bar", p)
	})

	// evaluation interval
	f(false, &Client{
		dataSourceType:     datasourcePrometheus,
		evaluationInterval: 15 * time.Second,
	}, func(t *testing.T, r *http.Request) {
		evalInterval := 15 * time.Second
		exp := url.Values{"query": {query}, "step": {evalInterval.String()}, "time": {timestamp.Format(time.RFC3339)}}
		checkEqualString(t, exp.Encode(), r.URL.RawQuery)
	})

	// step override
	f(false, &Client{
		dataSourceType: datasourcePrometheus,
		queryStep:      time.Minute,
	}, func(t *testing.T, r *http.Request) {
		exp := url.Values{
			"query": {query},
			"step":  {fmt.Sprintf("%ds", int(time.Minute.Seconds()))},
			"time":  {timestamp.Format(time.RFC3339)},
		}
		checkEqualString(t, exp.Encode(), r.URL.RawQuery)
	})

	//  step to seconds
	f(false, &Client{
		dataSourceType:     datasourcePrometheus,
		evaluationInterval: 3 * time.Hour,
	}, func(t *testing.T, r *http.Request) {
		evalInterval := 3 * time.Hour
		exp := url.Values{"query": {query}, "step": {fmt.Sprintf("%ds", int(evalInterval.Seconds()))}, "time": {timestamp.Format(time.RFC3339)}}
		checkEqualString(t, exp.Encode(), r.URL.RawQuery)
	})

	// prometheus extra params
	f(false, &Client{
		dataSourceType: datasourcePrometheus,
		extraParams:    url.Values{"round_digits": {"10"}},
	}, func(t *testing.T, r *http.Request) {
		exp := url.Values{"query": {query}, "round_digits": {"10"}, "time": {timestamp.Format(time.RFC3339)}}
		checkEqualString(t, exp.Encode(), r.URL.RawQuery)
	})

	// prometheus extra params range
	f(true, &Client{
		dataSourceType: datasourcePrometheus,
		extraParams: url.Values{
			"nocache":      {"1"},
			"max_lookback": {"1h"},
		},
	}, func(t *testing.T, r *http.Request) {
		exp := url.Values{
			"query":        {query},
			"end":          {timestamp.Format(time.RFC3339)},
			"start":        {timestamp.Format(time.RFC3339)},
			"nocache":      {"1"},
			"max_lookback": {"1h"},
		}
		checkEqualString(t, exp.Encode(), r.URL.RawQuery)
	})

	// custom params overrides the original params
	f(false, storage.Clone().ApplyParams(QuerierParams{
		DataSourceType: string(datasourcePrometheus),
		QueryParams:    url.Values{"round_digits": {"2"}},
	}), func(t *testing.T, r *http.Request) {
		exp := url.Values{"query": {query}, "round_digits": {"2"}, "time": {timestamp.Format(time.RFC3339)}}
		checkEqualString(t, exp.Encode(), r.URL.RawQuery)
	})

	// allow duplicates in query params
	f(false, storage.Clone().ApplyParams(QuerierParams{
		DataSourceType: string(datasourcePrometheus),
		QueryParams:    url.Values{"extra_labels": {"env=dev", "foo=bar"}},
	}), func(t *testing.T, r *http.Request) {
		exp := url.Values{"query": {query}, "round_digits": {"10"}, "extra_labels": {"env=dev", "foo=bar"}, "time": {timestamp.Format(time.RFC3339)}}
		checkEqualString(t, exp.Encode(), r.URL.RawQuery)
	})

	// graphite extra params
	f(false, &Client{
		dataSourceType: datasourceGraphite,
		extraParams: url.Values{
			"nocache":      {"1"},
			"max_lookback": {"1h"},
		},
	}, func(t *testing.T, r *http.Request) {
		exp := fmt.Sprintf("format=json&from=-5min&max_lookback=1h&nocache=1&target=%s&until=now", query)
		checkEqualString(t, exp, r.URL.RawQuery)
	})

	// graphite extra params allows to override from
	f(false, &Client{
		dataSourceType: datasourceGraphite,
		extraParams: url.Values{
			"from": {"-10m"},
		},
	}, func(t *testing.T, r *http.Request) {
		exp := fmt.Sprintf("format=json&from=-10m&target=%s&until=now", query)
		checkEqualString(t, exp, r.URL.RawQuery)
	})

	f(false, &Client{
		dataSourceType:     datasourceVLogs,
		evaluationInterval: time.Minute,
	}, func(t *testing.T, r *http.Request) {
		exp := url.Values{"query": {vlogsQuery}, "time": {timestamp.Format(time.RFC3339)}}
		checkEqualString(t, exp.Encode(), r.URL.RawQuery)
	})

	f(true, &Client{
		dataSourceType:     datasourceVLogs,
		evaluationInterval: time.Minute,
	}, func(t *testing.T, r *http.Request) {
		ts := timestamp.Format(time.RFC3339)
		exp := url.Values{"query": {vlogsQuery}, "start": {ts}, "end": {ts}, "step": {"60s"}}
		checkEqualString(t, exp.Encode(), r.URL.RawQuery)
	})
}

func TestHeaders(t *testing.T) {
	f := func(vmFn func() *Client, checkFn func(t *testing.T, r *http.Request)) {
		t.Helper()

		vm := vmFn()
		req, err := vm.newQueryRequest(ctx, "foo", time.Now())
		if err != nil {
			t.Fatalf("error in newQueryRequest: %s", err)
		}
		checkFn(t, req)
	}

	// basic auth
	f(func() *Client {
		cfg, err := utils.AuthConfig(utils.WithBasicAuth("foo", "bar", ""))
		if err != nil {
			t.Fatalf("Error get auth config: %s", err)
		}
		return NewPrometheusClient("", cfg, false, nil)
	}, func(t *testing.T, r *http.Request) {
		u, p, _ := r.BasicAuth()
		checkEqualString(t, "foo", u)
		checkEqualString(t, "bar", p)
	})

	// bearer auth
	f(func() *Client {
		cfg, err := utils.AuthConfig(utils.WithBearer("foo", ""))
		if err != nil {
			t.Fatalf("Error get auth config: %s", err)
		}
		return NewPrometheusClient("", cfg, false, nil)
	}, func(t *testing.T, r *http.Request) {
		reqToken := r.Header.Get("Authorization")
		splitToken := strings.Split(reqToken, "Bearer ")
		if len(splitToken) != 2 {
			t.Fatalf("expected two items got %d", len(splitToken))
		}
		token := splitToken[1]
		checkEqualString(t, "foo", token)
	})

	// custom extraHeaders
	f(func() *Client {
		c := NewPrometheusClient("", nil, false, nil)
		c.extraHeaders = []keyValue{
			{key: "Foo", value: "bar"},
			{key: "Baz", value: "qux"},
		}
		return c
	}, func(t *testing.T, r *http.Request) {
		h1 := r.Header.Get("Foo")
		checkEqualString(t, "bar", h1)
		h2 := r.Header.Get("Baz")
		checkEqualString(t, "qux", h2)
	})

	// custom header overrides basic auth
	f(func() *Client {
		cfg, err := utils.AuthConfig(utils.WithBasicAuth("foo", "bar", ""))
		if err != nil {
			t.Fatalf("Error get auth config: %s", err)
		}
		c := NewPrometheusClient("", cfg, false, nil)
		c.extraHeaders = []keyValue{
			{key: "Authorization", value: "Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ=="},
		}
		return c
	}, func(t *testing.T, r *http.Request) {
		u, p, _ := r.BasicAuth()
		checkEqualString(t, "Aladdin", u)
		checkEqualString(t, "open sesame", p)
	})
}

func checkEqualString(t *testing.T, exp, got string) {
	t.Helper()

	if got != exp {
		t.Fatalf("expected to get: \n%q; \ngot: \n%q", exp, got)
	}
}

func expectError(t *testing.T, err error, exp string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected non-nil error")
	}
	if !strings.Contains(err.Error(), exp) {
		t.Fatalf("expected error %q to contain %q", err, exp)
	}
}
