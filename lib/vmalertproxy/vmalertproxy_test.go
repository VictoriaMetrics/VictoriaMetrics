package vmalertproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestEnabled(t *testing.T) {
	f := func(proxyURLs []string, resultExpected bool) {
		t.Helper()

		Init(proxyURLs)
		if result := Enabled(); result != resultExpected {
			t.Fatalf("unexpected result for Init(%q); got %v; want %v", proxyURLs, result, resultExpected)
		}
	}

	f(nil, false)

	// empty -vmalert.proxyURL values must be ignored in the same way as the missing flag
	f([]string{""}, false)
	f([]string{"", ""}, false)

	f([]string{"http://vmalert:8880"}, true)
	f([]string{"http://vmalert-1:8880", "http://vmalert-2:8880"}, true)
	f([]string{"", "http://vmalert:8880"}, true)
}

// TestHandleRequestSingleURL verifies that a single -vmalert.proxyURL keeps the plain reverse proxy behaviour.
func TestHandleRequestSingleURL(t *testing.T) {
	f := func(path, query string) {
		t.Helper()

		respBody := `{"status":"success","data":{"groups":[{"name":"g1"}]}}`
		vmalert := newTestVMAlert(t, respBody)
		Init([]string{vmalert.s.URL})

		w := doRequest(t, path, query, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("unexpected status code; got %d; want %d", w.Code, http.StatusOK)
		}
		// the response must be returned as is
		if body := w.Body.String(); body != respBody {
			t.Fatalf("unexpected response body\ngot\n%s\nwant\n%s", body, respBody)
		}
		vmalert.mustHaveRequests(t, 1)
		if gotPath := vmalert.paths()[0]; gotPath != path {
			t.Fatalf("unexpected path requested from vmalert; got %q; want %q", gotPath, path)
		}
		// query args must be proxied as is
		if gotQuery := vmalert.queries()[0]; gotQuery != query {
			t.Fatalf("unexpected query requested from vmalert; got %q; want %q", gotQuery, query)
		}
	}

	f("/api/v1/rules", "")
	f("/api/v1/rules", "type=alert&exclude_alerts=true")
	f("/api/v1/alerts", "match%5B%5D=%7Bfoo%3D%22bar%22%7D")
	f("/api/v1/notifiers", "")
	f("/vmalert/groups", "")
}

// TestHandleRequestSingleURLGrafana verifies that datasource_type=prometheus is enforced for Grafana requests.
func TestHandleRequestSingleURLGrafana(t *testing.T) {
	vmalert := newTestVMAlert(t, `{"status":"success","data":{"groups":[]}}`)
	Init([]string{vmalert.s.URL})

	doRequest(t, "/api/v1/rules", "type=alert", map[string]string{
		"User-Agent": "Grafana/11.3.0",
	})

	vmalert.mustHaveRequests(t, 1)
	queryExpected := "datasource_type=prometheus&type=alert"
	if query := vmalert.queries()[0]; query != queryExpected {
		t.Fatalf("unexpected query requested from vmalert; got %q; want %q", query, queryExpected)
	}
}

func TestHandleRequestMergeRules(t *testing.T) {
	vmalert1 := newTestVMAlert(t, `{"status":"success","data":{"groups":[{"name":"g1"},{"name":"g2"}]}}`)
	vmalert2 := newTestVMAlert(t, `{"status":"success","data":{"groups":[{"name":"g3"}]}}`)
	Init([]string{vmalert1.s.URL, vmalert2.s.URL})

	w := doRequest(t, "/api/v1/rules", "type=alert", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code; got %d; want %d", w.Code, http.StatusOK)
	}
	// groups must be concatenated in the order of the configured -vmalert.proxyURL
	bodyExpected := `{"status":"success","data":{"groups":[{"name":"g1"},{"name":"g2"},{"name":"g3"}]}}`
	if body := w.Body.String(); body != bodyExpected {
		t.Fatalf("unexpected response body\ngot\n%s\nwant\n%s", body, bodyExpected)
	}

	// query args must be forwarded to every vmalert
	for _, vmalert := range []*testVMAlert{vmalert1, vmalert2} {
		vmalert.mustHaveRequests(t, 1)
		if path := vmalert.paths()[0]; path != "/api/v1/rules" {
			t.Fatalf("unexpected path requested from vmalert; got %q; want %q", path, "/api/v1/rules")
		}
		if query := vmalert.queries()[0]; query != "type=alert" {
			t.Fatalf("unexpected query requested from vmalert; got %q; want %q", query, "type=alert")
		}
	}
}

func TestHandleRequestMergeAlerts(t *testing.T) {
	vmalert1 := newTestVMAlert(t, `{"status":"success","data":{"alerts":[{"id":"1"}]}}`)
	vmalert2 := newTestVMAlert(t, `{"status":"success","data":{"alerts":[{"id":"2"}]}}`)
	Init([]string{vmalert1.s.URL, vmalert2.s.URL})

	w := doRequest(t, "/api/v1/alerts", "", nil)
	bodyExpected := `{"status":"success","data":{"alerts":[{"id":"1"},{"id":"2"}]}}`
	if body := w.Body.String(); body != bodyExpected {
		t.Fatalf("unexpected response body\ngot\n%s\nwant\n%s", body, bodyExpected)
	}
}

func TestHandleRequestMergeEmptyResponses(t *testing.T) {
	vmalert1 := newTestVMAlert(t, `{"status":"success","data":{"groups":[]}}`)
	vmalert2 := newTestVMAlert(t, `{"status":"success","data":{}}`)
	Init([]string{vmalert1.s.URL, vmalert2.s.URL})

	w := doRequest(t, "/api/v1/rules", "", nil)
	// the merged list must be non-nil - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4221
	bodyExpected := `{"status":"success","data":{"groups":[]}}`
	if body := w.Body.String(); body != bodyExpected {
		t.Fatalf("unexpected response body\ngot\n%s\nwant\n%s", body, bodyExpected)
	}
}

// TestHandleRequestMergePartialFailure verifies that unavailable vmalert instances
// do not fail the whole request.
func TestHandleRequestMergePartialFailure(t *testing.T) {
	f := func(newBrokenVMAlert func(t *testing.T) *testVMAlert) {
		t.Helper()

		vmalert1 := newTestVMAlert(t, `{"status":"success","data":{"groups":[{"name":"g1"}]}}`)
		broken := newBrokenVMAlert(t)
		vmalert3 := newTestVMAlert(t, `{"status":"success","data":{"groups":[{"name":"g3"}]}}`)
		Init([]string{vmalert1.s.URL, broken.s.URL, vmalert3.s.URL})

		w := doRequest(t, "/api/v1/rules", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("unexpected status code; got %d; want %d", w.Code, http.StatusOK)
		}
		bodyExpected := `{"status":"success","warnings":["1 out of 3 vmalert instances at -vmalert.proxyURL are unavailable; ` +
			`see logs for details"],"data":{"groups":[{"name":"g1"},{"name":"g3"}]}}`
		if body := w.Body.String(); body != bodyExpected {
			t.Fatalf("unexpected response body\ngot\n%s\nwant\n%s", body, bodyExpected)
		}
	}

	// unreachable vmalert
	f(func(t *testing.T) *testVMAlert {
		vmalert := newTestVMAlert(t, "")
		vmalert.s.Close()
		return vmalert
	})

	// vmalert returning non-200 status code
	f(func(t *testing.T) *testVMAlert {
		return newTestVMAlertFunc(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "foobar")
		})
	})

	// vmalert returning `status: error`
	f(func(t *testing.T) *testVMAlert {
		return newTestVMAlert(t, `{"status":"error","errorType":"422","error":"some error"}`)
	})

	// vmalert returning malformed response
	f(func(t *testing.T) *testVMAlert {
		return newTestVMAlert(t, `foobar`)
	})
}

func TestHandleRequestMergeAllTargetsFail(t *testing.T) {
	vmalert1 := newTestVMAlert(t, "")
	vmalert1.s.Close()
	vmalert2 := newTestVMAlert(t, "")
	vmalert2.s.Close()
	Init([]string{vmalert1.s.URL, vmalert2.s.URL})

	w := doRequest(t, "/api/v1/rules", "", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code; got %d; want %d", w.Code, http.StatusServiceUnavailable)
	}
	bodyExpected := `{"status":"error","errorType":"503","error":"all the 2 vmalert instances at -vmalert.proxyURL are unavailable; see logs for details"}`
	if body := w.Body.String(); body != bodyExpected {
		t.Fatalf("unexpected response body\ngot\n%s\nwant\n%s", body, bodyExpected)
	}
}

// TestHandleRequestMultipleURLsNonMergeablePath verifies that non-mergeable paths
// are proxied to the first -vmalert.proxyURL.
func TestHandleRequestMultipleURLsNonMergeablePath(t *testing.T) {
	f := func(path string) {
		t.Helper()

		respBody := `some response`
		vmalert1 := newTestVMAlert(t, respBody)
		vmalert2 := newTestVMAlert(t, `must be unused`)
		Init([]string{vmalert1.s.URL, vmalert2.s.URL})

		w := doRequest(t, path, "", nil)
		if body := w.Body.String(); body != respBody {
			t.Fatalf("unexpected response body\ngot\n%s\nwant\n%s", body, respBody)
		}
		vmalert1.mustHaveRequests(t, 1)
		vmalert2.mustHaveRequests(t, 0)
	}

	// vmalert web UI
	f("/vmalert/")
	f("/vmalert/groups")
	f("/vmalert/api/v1/rules")
	// non-mergeable JSON APIs
	f("/api/v1/notifiers")
	f("/rules")
	f("/alerts")
}

func TestHandleRequestMergeGrafana(t *testing.T) {
	vmalert1 := newTestVMAlert(t, `{"status":"success","data":{"groups":[]}}`)
	vmalert2 := newTestVMAlert(t, `{"status":"success","data":{"groups":[]}}`)
	Init([]string{vmalert1.s.URL, vmalert2.s.URL})

	doRequest(t, "/api/v1/rules", "type=alert", map[string]string{
		"User-Agent": "Grafana/11.3.0",
	})

	queryExpected := "datasource_type=prometheus&type=alert"
	for _, vmalert := range []*testVMAlert{vmalert1, vmalert2} {
		vmalert.mustHaveRequests(t, 1)
		if query := vmalert.queries()[0]; query != queryExpected {
			t.Fatalf("unexpected query requested from vmalert; got %q; want %q", query, queryExpected)
		}
	}
}

// TestHandleRequestMergeURLQueryArgs verifies that the query args at -vmalert.proxyURL
// are preserved in the same way as the plain reverse proxy does.
func TestHandleRequestMergeURLQueryArgs(t *testing.T) {
	f := func(requestQuery string, headers map[string]string, queryExpected string) {
		t.Helper()

		vmalert1 := newTestVMAlert(t, `{"status":"success","data":{"groups":[]}}`)
		vmalert2 := newTestVMAlert(t, `{"status":"success","data":{"groups":[]}}`)
		Init([]string{vmalert1.s.URL + "/?authKey=secret", vmalert2.s.URL + "/?authKey=secret"})

		doRequest(t, "/api/v1/rules", requestQuery, headers)

		for _, vmalert := range []*testVMAlert{vmalert1, vmalert2} {
			vmalert.mustHaveRequests(t, 1)
			if query := vmalert.queries()[0]; query != queryExpected {
				t.Fatalf("unexpected query requested from vmalert; got %q; want %q", query, queryExpected)
			}
		}
	}

	f("", nil, "authKey=secret")
	f("type=alert", nil, "authKey=secret&type=alert")
	// the query args at -vmalert.proxyURL must survive the Grafana query rewrite
	f("type=alert", map[string]string{"User-Agent": "Grafana/11.3.0"}, "authKey=secret&datasource_type=prometheus&type=alert")
}

// TestHandleRequestMergeStuckInstance verifies that a vmalert instance not responding within
// -vmalert.proxyTimeout doesn't block the merged response.
func TestHandleRequestMergeStuckInstance(t *testing.T) {
	proxyTimeoutOld := *proxyTimeout
	*proxyTimeout = 100 * time.Millisecond
	defer func() {
		*proxyTimeout = proxyTimeoutOld
	}()

	vmalert1 := newTestVMAlert(t, `{"status":"success","data":{"groups":[{"name":"g1"}]}}`)
	stuck := newTestVMAlertFunc(t, func(_ http.ResponseWriter, r *http.Request) {
		// Block until the request is aborted by the -vmalert.proxyTimeout deadline.
		<-r.Context().Done()
	})
	Init([]string{vmalert1.s.URL, stuck.s.URL})

	w := doRequest(t, "/api/v1/rules", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code; got %d; want %d", w.Code, http.StatusOK)
	}
	bodyExpected := `{"status":"success","warnings":["1 out of 2 vmalert instances at -vmalert.proxyURL are unavailable; ` +
		`see logs for details"],"data":{"groups":[{"name":"g1"}]}}`
	if body := w.Body.String(); body != bodyExpected {
		t.Fatalf("unexpected response body\ngot\n%s\nwant\n%s", body, bodyExpected)
	}
}

// TestHandleRequestMergePagination verifies that `group_limit` and `page_num` are applied
// to the merged groups list instead of being forwarded to vmalert instances.
func TestHandleRequestMergePagination(t *testing.T) {
	f := func(query, bodyExpected, vmalertQueryExpected string) {
		t.Helper()

		vmalert1 := newTestVMAlert(t, `{"status":"success","data":{"groups":[{"name":"g1","rules":[{"name":"r1"},{"name":"r2"}]},{"name":"g2","rules":[{"name":"r3"}]}]}}`)
		vmalert2 := newTestVMAlert(t, `{"status":"success","data":{"groups":[{"name":"g3","rules":[{"name":"r4"}]}]}}`)
		Init([]string{vmalert1.s.URL, vmalert2.s.URL})

		w := doRequest(t, "/api/v1/rules", query, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("unexpected status code; got %d; want %d; response: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if body := w.Body.String(); body != bodyExpected {
			t.Fatalf("unexpected response body\ngot\n%s\nwant\n%s", body, bodyExpected)
		}
		for _, vmalert := range []*testVMAlert{vmalert1, vmalert2} {
			vmalert.mustHaveRequests(t, 1)
			if query := vmalert.queries()[0]; query != vmalertQueryExpected {
				t.Fatalf("unexpected query requested from vmalert; got %q; want %q", query, vmalertQueryExpected)
			}
		}
	}

	// group_limit without page_num returns the first page without the `page` field -
	// the same as vmalert does
	f("group_limit=2",
		`{"status":"success","total_pages":2,"total_groups":3,"total_rules":4,"data":{"groups":[{"name":"g1","rules":[{"name":"r1"},{"name":"r2"}]},{"name":"g2","rules":[{"name":"r3"}]}]}}`,
		"")
	f("group_limit=2&page_num=1",
		`{"status":"success","page":1,"total_pages":2,"total_groups":3,"total_rules":4,"data":{"groups":[{"name":"g1","rules":[{"name":"r1"},{"name":"r2"}]},{"name":"g2","rules":[{"name":"r3"}]}]}}`,
		"")
	// the last page contains groups from the second vmalert instance
	f("group_limit=2&page_num=2",
		`{"status":"success","page":2,"total_pages":2,"total_groups":3,"total_rules":4,"data":{"groups":[{"name":"g3","rules":[{"name":"r4"}]}]}}`,
		"")
	// the other query args must be forwarded to every vmalert instance
	f("group_limit=1&page_num=3&type=alert",
		`{"status":"success","page":3,"total_pages":3,"total_groups":3,"total_rules":4,"data":{"groups":[{"name":"g3","rules":[{"name":"r4"}]}]}}`,
		"type=alert")
}

// TestHandleRequestMergePaginationEmptyResult verifies the pagination metadata
// for the empty merged groups list.
func TestHandleRequestMergePaginationEmptyResult(t *testing.T) {
	vmalert1 := newTestVMAlert(t, `{"status":"success","data":{"groups":[]}}`)
	vmalert2 := newTestVMAlert(t, `{"status":"success","data":{"groups":[]}}`)
	Init([]string{vmalert1.s.URL, vmalert2.s.URL})

	w := doRequest(t, "/api/v1/rules", "group_limit=2&page_num=1", nil)
	// zero metadata fields are skipped in the same way as vmalert does
	bodyExpected := `{"status":"success","page":1,"total_pages":1,"data":{"groups":[]}}`
	if body := w.Body.String(); body != bodyExpected {
		t.Fatalf("unexpected response body\ngot\n%s\nwant\n%s", body, bodyExpected)
	}
}

// TestHandleRequestMergePaginationFailure verifies that invalid pagination args
// are rejected with the same errors as vmalert returns.
func TestHandleRequestMergePaginationFailure(t *testing.T) {
	f := func(query, errExpected string) {
		t.Helper()

		vmalert1 := newTestVMAlert(t, `{"status":"success","data":{"groups":[{"name":"g1"}]}}`)
		vmalert2 := newTestVMAlert(t, `{"status":"success","data":{"groups":[{"name":"g2"}]}}`)
		Init([]string{vmalert1.s.URL, vmalert2.s.URL})

		w := doRequest(t, "/api/v1/rules", query, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status code; got %d; want %d; response: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
		bodyExpected := fmt.Sprintf(`{"status":"error","errorType":"400","error":%q}`, errExpected)
		if body := w.Body.String(); body != bodyExpected {
			t.Fatalf("unexpected response body\ngot\n%s\nwant\n%s", body, bodyExpected)
		}
	}

	f("page_num=1", `"group_limit" needs to be present in order to paginate over the groups`)
	f("group_limit=0", `"group_limit" is expected to be a positive number, found "0"`)
	f("group_limit=foo", `"group_limit" is expected to be a positive number, found "foo"`)
	f("group_limit=1&page_num=-1", `"page_num" is expected to be a positive number, found "-1"`)
	f("group_limit=1&page_num=foo", `"page_num" is expected to be a positive number, found "foo"`)
	f("group_limit=2&page_num=2", `page_num=2 exceeds total amount of pages in result=1`)
}

// TestHandleRequestMergeURLPathPrefix verifies that the path prefix at -vmalert.proxyURL
// is preserved in the same way as the plain reverse proxy does.
func TestHandleRequestMergeURLPathPrefix(t *testing.T) {
	vmalert1 := newTestVMAlert(t, `{"status":"success","data":{"groups":[]}}`)
	vmalert2 := newTestVMAlert(t, `{"status":"success","data":{"groups":[]}}`)
	Init([]string{vmalert1.s.URL + "/prefix", vmalert2.s.URL + "/prefix/"})

	doRequest(t, "/api/v1/rules", "", nil)

	for _, vmalert := range []*testVMAlert{vmalert1, vmalert2} {
		vmalert.mustHaveRequests(t, 1)
		if path := vmalert.paths()[0]; path != "/prefix/api/v1/rules" {
			t.Fatalf("unexpected path requested from vmalert; got %q; want %q", path, "/prefix/api/v1/rules")
		}
	}
}

func doRequest(t *testing.T, path, query string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	target := path
	if len(query) > 0 {
		target += "?" + query
	}
	r := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	HandleRequest(w, r, path)
	return w
}

// testVMAlert is a mock for vmalert, which records the received requests.
type testVMAlert struct {
	s *httptest.Server

	mu        sync.Mutex
	reqPaths  []string
	reqQuerys []string
}

func newTestVMAlert(t *testing.T, respBody string) *testVMAlert {
	t.Helper()

	return newTestVMAlertFunc(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, respBody)
	})
}

func newTestVMAlertFunc(t *testing.T, h http.HandlerFunc) *testVMAlert {
	t.Helper()

	va := &testVMAlert{}
	va.s = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		va.mu.Lock()
		va.reqPaths = append(va.reqPaths, r.URL.Path)
		va.reqQuerys = append(va.reqQuerys, r.URL.RawQuery)
		va.mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(va.s.Close)
	return va
}

func (va *testVMAlert) paths() []string {
	va.mu.Lock()
	defer va.mu.Unlock()
	return append([]string(nil), va.reqPaths...)
}

func (va *testVMAlert) queries() []string {
	va.mu.Lock()
	defer va.mu.Unlock()
	return append([]string(nil), va.reqQuerys...)
}

func (va *testVMAlert) mustHaveRequests(t *testing.T, n int) {
	t.Helper()

	if got := len(va.paths()); got != n {
		t.Fatalf("unexpected number of requests received by vmalert; got %d; want %d", got, n)
	}
}
