package datasource

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

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
	query       = "vm_rows"
	queryRender = "constantLine(10)"
)

func TestVMInstantQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("should not be called")
	})
	c := -1
	mux.HandleFunc("/render", func(w http.ResponseWriter, request *http.Request) {
		c++
		switch c {
		case 7:
			w.Write([]byte(`[{"target":"constantLine(10)","tags":{"name":"constantLine(10)"},"datapoints":[[10,1611758343],[10,1611758373],[10,1611758403]]}]`))
		}
	})
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		c++
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method got %s", r.Method)
		}
		if name, pass, _ := r.BasicAuth(); name != basicAuthName || pass != basicAuthPass {
			t.Errorf("expected %s:%s as basic auth got %s:%s", basicAuthName, basicAuthPass, name, pass)
		}
		if r.URL.Query().Get("query") != query {
			t.Errorf("expected %s in query param, got %s", query, r.URL.Query().Get("query"))
		}
		timeParam := r.URL.Query().Get("time")
		if timeParam == "" {
			t.Errorf("expected 'time' in query param, got nil instead")
		}
		if _, err := strconv.ParseInt(timeParam, 10, 64); err != nil {
			t.Errorf("failed to parse 'time' query param: %s", err)
		}
		switch c {
		case 0:
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
		case 1:
			w.WriteHeader(500)
		case 2:
			w.Write([]byte("[]"))
		case 3:
			w.Write([]byte(`{"status":"error", "errorType":"type:", "error":"some error msg"}`))
		case 4:
			w.Write([]byte(`{"status":"unknown"}`))
		case 5:
			w.Write([]byte(`{"status":"success","data":{"resultType":"matrix"}}`))
		case 6:
			w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"vm_rows"},"value":[1583786142,"13763"]},{"metric":{"__name__":"vm_requests"},"value":[1583786140,"2000"]}]}}`))
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	authCfg, err := promauth.NewConfig(".", nil, baCfg, "", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected: %s", err)
	}
	s := NewVMStorage(srv.URL, authCfg, time.Minute, 0, false, srv.Client(), false)

	p := NewPrometheusType()
	pq := s.BuildWithParams(QuerierParams{DataSourceType: &p, EvaluationInterval: 15 * time.Second})

	if _, err := pq.Query(ctx, query); err == nil {
		t.Fatalf("expected connection error got nil")
	}
	if _, err := pq.Query(ctx, query); err == nil {
		t.Fatalf("expected invalid response status error got nil")
	}
	if _, err := pq.Query(ctx, query); err == nil {
		t.Fatalf("expected response body error got nil")
	}
	if _, err := pq.Query(ctx, query); err == nil {
		t.Fatalf("expected error status got nil")
	}
	if _, err := pq.Query(ctx, query); err == nil {
		t.Fatalf("expected unknown status got nil")
	}
	if _, err := pq.Query(ctx, query); err == nil {
		t.Fatalf("expected non-vector resultType error  got nil")
	}
	m, err := pq.Query(ctx, query)
	if err != nil {
		t.Fatalf("unexpected %s", err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 metrics got %d in %+v", len(m), m)
	}
	expected := []Metric{
		{
			Labels:     []Label{{Value: "vm_rows", Name: "__name__"}},
			Timestamps: []int64{1583786142},
			Values:     []float64{13763},
		},
		{
			Labels:     []Label{{Value: "vm_requests", Name: "__name__"}},
			Timestamps: []int64{1583786140},
			Values:     []float64{2000},
		},
	}
	if !reflect.DeepEqual(m, expected) {
		t.Fatalf("unexpected metric %+v want %+v", m, expected)
	}

	g := NewGraphiteType()
	gq := s.BuildWithParams(QuerierParams{DataSourceType: &g})

	m, err = gq.Query(ctx, queryRender)
	if err != nil {
		t.Fatalf("unexpected %s", err)
	}
	if len(m) != 1 {
		t.Fatalf("expected 1 metric  got %d in %+v", len(m), m)
	}
	exp := Metric{
		Labels:     []Label{{Value: "constantLine(10)", Name: "name"}},
		Timestamps: []int64{1611758403},
		Values:     []float64{10},
	}
	if !reflect.DeepEqual(m[0], exp) {
		t.Fatalf("unexpected metric %+v want %+v", m[0], expected)
	}
}

func TestVMRangeQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("should not be called")
	})
	c := -1
	mux.HandleFunc("/api/v1/query_range", func(w http.ResponseWriter, r *http.Request) {
		c++
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method got %s", r.Method)
		}
		if name, pass, _ := r.BasicAuth(); name != basicAuthName || pass != basicAuthPass {
			t.Errorf("expected %s:%s as basic auth got %s:%s", basicAuthName, basicAuthPass, name, pass)
		}
		if r.URL.Query().Get("query") != query {
			t.Errorf("expected %s in query param, got %s", query, r.URL.Query().Get("query"))
		}
		startTS := r.URL.Query().Get("start")
		if startTS == "" {
			t.Errorf("expected 'start' in query param, got nil instead")
		}
		if _, err := strconv.ParseInt(startTS, 10, 64); err != nil {
			t.Errorf("failed to parse 'start' query param: %s", err)
		}
		endTS := r.URL.Query().Get("end")
		if endTS == "" {
			t.Errorf("expected 'end' in query param, got nil instead")
		}
		if _, err := strconv.ParseInt(endTS, 10, 64); err != nil {
			t.Errorf("failed to parse 'end' query param: %s", err)
		}
		switch c {
		case 0:
			w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"vm_rows"},"values":[[1583786142,"13763"]]}]}}`))
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	authCfg, err := promauth.NewConfig(".", nil, baCfg, "", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected: %s", err)
	}
	s := NewVMStorage(srv.URL, authCfg, time.Minute, 0, false, srv.Client(), false)

	p := NewPrometheusType()
	pq := s.BuildWithParams(QuerierParams{DataSourceType: &p, EvaluationInterval: 15 * time.Second})

	_, err = pq.QueryRange(ctx, query, time.Now(), time.Time{})
	expectError(t, err, "is missing")

	_, err = pq.QueryRange(ctx, query, time.Time{}, time.Now())
	expectError(t, err, "is missing")

	start, end := time.Now().Add(-time.Minute), time.Now()

	m, err := pq.QueryRange(ctx, query, start, end)
	if err != nil {
		t.Fatalf("unexpected %s", err)
	}
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

	g := NewGraphiteType()
	gq := s.BuildWithParams(QuerierParams{DataSourceType: &g})

	_, err = gq.QueryRange(ctx, queryRender, start, end)
	expectError(t, err, "is not supported")
}

func TestRequestParams(t *testing.T) {
	authCfg, err := promauth.NewConfig(".", nil, baCfg, "", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected: %s", err)
	}
	query := "up"
	timestamp := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	testCases := []struct {
		name       string
		queryRange bool
		vm         *VMStorage
		checkFn    func(t *testing.T, r *http.Request)
	}{
		{
			"prometheus path",
			false,
			&VMStorage{
				dataSourceType: NewPrometheusType(),
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, prometheusInstantPath, r.URL.Path)
			},
		},
		{
			"prometheus path with disablePathAppend",
			false,
			&VMStorage{
				dataSourceType:    NewPrometheusType(),
				disablePathAppend: true,
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, "", r.URL.Path)
			},
		},
		{
			"prometheus prefix",
			false,
			&VMStorage{
				dataSourceType:   NewPrometheusType(),
				appendTypePrefix: true,
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, prometheusPrefix+prometheusInstantPath, r.URL.Path)
			},
		},
		{
			"prometheus prefix with disablePathAppend",
			false,
			&VMStorage{
				dataSourceType:    NewPrometheusType(),
				appendTypePrefix:  true,
				disablePathAppend: true,
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, prometheusPrefix, r.URL.Path)
			},
		},
		{
			"prometheus range path",
			true,
			&VMStorage{
				dataSourceType: NewPrometheusType(),
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, prometheusRangePath, r.URL.Path)
			},
		},
		{
			"prometheus range path with disablePathAppend",
			true,
			&VMStorage{
				dataSourceType:    NewPrometheusType(),
				disablePathAppend: true,
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, "", r.URL.Path)
			},
		},
		{
			"prometheus range prefix",
			true,
			&VMStorage{
				dataSourceType:   NewPrometheusType(),
				appendTypePrefix: true,
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, prometheusPrefix+prometheusRangePath, r.URL.Path)
			},
		},
		{
			"prometheus range prefix with disablePathAppend",
			true,
			&VMStorage{
				dataSourceType:    NewPrometheusType(),
				appendTypePrefix:  true,
				disablePathAppend: true,
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, prometheusPrefix, r.URL.Path)
			},
		},
		{
			"graphite path",
			false,
			&VMStorage{
				dataSourceType: NewGraphiteType(),
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, graphitePath, r.URL.Path)
			},
		},
		{
			"graphite prefix",
			false,
			&VMStorage{
				dataSourceType:   NewGraphiteType(),
				appendTypePrefix: true,
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, graphitePrefix+graphitePath, r.URL.Path)
			},
		},
		{
			"default params",
			false,
			&VMStorage{},
			func(t *testing.T, r *http.Request) {
				exp := fmt.Sprintf("query=%s&time=%d", query, timestamp.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"default range params",
			true,
			&VMStorage{},
			func(t *testing.T, r *http.Request) {
				exp := fmt.Sprintf("end=%d&query=%s&start=%d", timestamp.Unix(), query, timestamp.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"basic auth",
			false,
			&VMStorage{authCfg: authCfg},
			func(t *testing.T, r *http.Request) {
				u, p, _ := r.BasicAuth()
				checkEqualString(t, "foo", u)
				checkEqualString(t, "bar", p)
			},
		},
		{
			"basic auth range",
			true,
			&VMStorage{authCfg: authCfg},
			func(t *testing.T, r *http.Request) {
				u, p, _ := r.BasicAuth()
				checkEqualString(t, "foo", u)
				checkEqualString(t, "bar", p)
			},
		},
		{
			"lookback",
			false,
			&VMStorage{
				lookBack: time.Minute,
			},
			func(t *testing.T, r *http.Request) {
				exp := fmt.Sprintf("query=%s&time=%d", query, timestamp.Add(-time.Minute).Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"evaluation interval",
			false,
			&VMStorage{
				evaluationInterval: 15 * time.Second,
			},
			func(t *testing.T, r *http.Request) {
				evalInterval := 15 * time.Second
				tt := timestamp.Truncate(evalInterval)
				exp := fmt.Sprintf("query=%s&step=%v&time=%d", query, evalInterval, tt.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"lookback + evaluation interval",
			false,
			&VMStorage{
				lookBack:           time.Minute,
				evaluationInterval: 15 * time.Second,
			},
			func(t *testing.T, r *http.Request) {
				evalInterval := 15 * time.Second
				tt := timestamp.Add(-time.Minute)
				tt = tt.Truncate(evalInterval)
				exp := fmt.Sprintf("query=%s&step=%v&time=%d", query, evalInterval, tt.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"step override",
			false,
			&VMStorage{
				queryStep: time.Minute,
			},
			func(t *testing.T, r *http.Request) {
				exp := fmt.Sprintf("query=%s&step=%ds&time=%d", query, int(time.Minute.Seconds()), timestamp.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"step to seconds",
			false,
			&VMStorage{
				evaluationInterval: 3 * time.Hour,
			},
			func(t *testing.T, r *http.Request) {
				evalInterval := 3 * time.Hour
				tt := timestamp.Truncate(evalInterval)
				exp := fmt.Sprintf("query=%s&step=%ds&time=%d", query, int(evalInterval.Seconds()), tt.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"prometheus extra params",
			false,
			&VMStorage{
				extraParams: url.Values{"round_digits": {"10"}},
			},
			func(t *testing.T, r *http.Request) {
				exp := fmt.Sprintf("query=%s&round_digits=10&time=%d", query, timestamp.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"prometheus extra params range",
			true,
			&VMStorage{
				extraParams: url.Values{
					"nocache":      {"1"},
					"max_lookback": {"1h"},
				},
			},
			func(t *testing.T, r *http.Request) {
				exp := fmt.Sprintf("end=%d&max_lookback=1h&nocache=1&query=%s&start=%d",
					timestamp.Unix(), query, timestamp.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"graphite extra params",
			false,
			&VMStorage{
				dataSourceType: NewGraphiteType(),
				extraParams: url.Values{
					"nocache":      {"1"},
					"max_lookback": {"1h"},
				},
			},
			func(t *testing.T, r *http.Request) {
				exp := fmt.Sprintf("format=json&from=-5min&max_lookback=1h&nocache=1&target=%s&until=now", query)
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := tc.vm.newRequestPOST()
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			switch tc.vm.dataSourceType.String() {
			case "prometheus":
				if tc.queryRange {
					tc.vm.setPrometheusRangeReqParams(req, query, timestamp, timestamp)
				} else {
					tc.vm.setPrometheusInstantReqParams(req, query, timestamp)
				}
			case "graphite":
				tc.vm.setGraphiteReqParams(req, query, timestamp)
			}
			tc.checkFn(t, req)
		})
	}
}

func checkEqualString(t *testing.T, exp, got string) {
	t.Helper()
	if got != exp {
		t.Errorf("expected to get: \n%q; \ngot: \n%q", exp, got)
	}
}

func expectError(t *testing.T, err error, exp string) {
	t.Helper()
	if err == nil {
		t.Errorf("expected non-nil error")
	}
	if !strings.Contains(err.Error(), exp) {
		t.Errorf("expected error %q to contain %q", err, exp)
	}
}
