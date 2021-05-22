package datasource

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"
)

var (
	ctx           = context.Background()
	basicAuthName = "foo"
	basicAuthPass = "bar"
	query         = "vm_rows"
	queryRender   = "constantLine(10)"
)

func TestVMSelectQuery(t *testing.T) {
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
			w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"vm_rows"},"value":[1583786142,"13763"]}]}}`))
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := NewVMStorage(srv.URL, basicAuthName, basicAuthPass, time.Minute, 0, false, srv.Client())

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
	if len(m) != 1 {
		t.Fatalf("expected 1 metric  got %d in %+v", len(m), m)
	}
	expected := Metric{
		Labels:    []Label{{Value: "vm_rows", Name: "__name__"}},
		Timestamp: 1583786142,
		Value:     13763,
	}
	if !reflect.DeepEqual(m[0], expected) {
		t.Fatalf("unexpected metric %+v want %+v", m[0], expected)
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
	expected = Metric{
		Labels:    []Label{{Value: "constantLine(10)", Name: "name"}},
		Timestamp: 1611758403,
		Value:     10,
	}
	if !reflect.DeepEqual(m[0], expected) {
		t.Fatalf("unexpected metric %+v want %+v", m[0], expected)
	}
}

func TestPrepareReq(t *testing.T) {
	query := "up"
	timestamp := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	testCases := []struct {
		name    string
		vm      *VMStorage
		checkFn func(t *testing.T, r *http.Request)
	}{
		{
			"prometheus path",
			&VMStorage{
				dataSourceType: NewPrometheusType(),
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, queryPath, r.URL.Path)
			},
		},
		{
			"prometheus prefix",
			&VMStorage{
				dataSourceType:   NewPrometheusType(),
				appendTypePrefix: true,
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, prometheusPrefix+queryPath, r.URL.Path)
			},
		},
		{
			"graphite path",
			&VMStorage{
				dataSourceType: NewGraphiteType(),
			},
			func(t *testing.T, r *http.Request) {
				checkEqualString(t, graphitePath, r.URL.Path)
			},
		},
		{
			"graphite prefix",
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
			&VMStorage{},
			func(t *testing.T, r *http.Request) {
				exp := fmt.Sprintf("query=%s&time=%d", query, timestamp.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"basic auth",
			&VMStorage{
				basicAuthUser: "foo",
				basicAuthPass: "bar",
			},
			func(t *testing.T, r *http.Request) {
				u, p, _ := r.BasicAuth()
				checkEqualString(t, "foo", u)
				checkEqualString(t, "bar", p)
			},
		},
		{
			"lookback",
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
			&VMStorage{
				queryStep: time.Minute,
			},
			func(t *testing.T, r *http.Request) {
				exp := fmt.Sprintf("query=%s&step=%v&time=%d", query, time.Minute, timestamp.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"round digits",
			&VMStorage{
				roundDigits: "10",
			},
			func(t *testing.T, r *http.Request) {
				exp := fmt.Sprintf("query=%s&round_digits=10&time=%d", query, timestamp.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
		{
			"extra labels",
			&VMStorage{
				extraLabels: []string{
					"env=prod",
					"query=es=cape",
				},
			},
			func(t *testing.T, r *http.Request) {
				exp := fmt.Sprintf("extra_label=env%%3Dprod&extra_label=query%%3Des%%3Dcape&query=%s&time=%d", query, timestamp.Unix())
				checkEqualString(t, exp, r.URL.RawQuery)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := tc.vm.prepareReq(query, timestamp)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			tc.checkFn(t, req)
		})
	}
}

func checkEqualString(t *testing.T, exp, got string) {
	t.Helper()
	if got != exp {
		t.Errorf("expected to get %q; got %q", exp, got)
	}
}
