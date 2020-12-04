package datasource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

var (
	ctx           = context.Background()
	basicAuthName = "foo"
	basicAuthPass = "bar"
	query         = "vm_rows"
)

func TestVMSelectQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("should not be called")
	})
	c := -1
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
	am, _ := NewVMStorage(srv.URL, basicAuthName, basicAuthPass, false, time.Minute, srv.Client())
	if _, err := am.Query(ctx, nil, query); err == nil {
		t.Fatalf("expected connection error got nil")
	}
	if _, err := am.Query(ctx, nil, query); err == nil {
		t.Fatalf("expected invalid response status error got nil")
	}
	if _, err := am.Query(ctx, nil, query); err == nil {
		t.Fatalf("expected response body error got nil")
	}
	if _, err := am.Query(ctx, nil, query); err == nil {
		t.Fatalf("expected error status got nil")
	}
	if _, err := am.Query(ctx, nil, query); err == nil {
		t.Fatalf("expected unkown status got nil")
	}
	if _, err := am.Query(ctx, nil, query); err == nil {
		t.Fatalf("expected non-vector resultType error  got nil")
	}
	m, err := am.Query(ctx, nil, query)
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
	if m[0].Timestamp != expected.Timestamp &&
		m[0].Value != expected.Value &&
		m[0].Labels[0].Value != expected.Labels[0].Value &&
		m[0].Labels[0].Name != expected.Labels[0].Name {
		t.Fatalf("unexpected metric %+v want %+v", m[0], expected)
	}
}
