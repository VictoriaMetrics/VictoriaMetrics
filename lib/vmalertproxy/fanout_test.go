package vmalertproxy

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// newVMAlertMock returns a mock vmalert, which responds with the given body for the given path.
func newVMAlertMock(t *testing.T, responses map[string]string) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"status":"error","errorType":"not_found","error":%q}`, "unsupported path "+r.URL.Path)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(s.Close)
	return s
}

func doRequest(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	HandleRequest(w, r, r.URL.Path)

	resp := w.Result()
	defer func() {
		_ = resp.Body.Close()
	}()
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("cannot parse response: %s", err)
	}
	return resp.StatusCode, m
}

func groupNamesWithSource(t *testing.T, m map[string]any) []string {
	t.Helper()
	data, ok := m["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing `data` in response %v", m)
	}
	groups, ok := data["groups"].([]any)
	if !ok {
		t.Fatalf("missing `data.groups` in response %v", m)
	}
	result := make([]string, 0, len(groups))
	for _, g := range groups {
		group := g.(map[string]any)
		labels, ok := group["labels"].(map[string]any)
		if !ok {
			t.Fatalf("missing `labels` at group %v", group)
		}
		result = append(result, fmt.Sprintf("%s/%s", group["name"], labels[SourceLabel]))
	}
	return result
}

func warnings(t *testing.T, m map[string]any) int {
	t.Helper()
	ws, ok := m["warnings"]
	if !ok {
		return 0
	}
	return len(ws.([]any))
}

func TestHandleRequestSingleBackendIsProxiedAsIs(t *testing.T) {
	s := newVMAlertMock(t, map[string]string{
		"/api/v1/rules": `{"status":"success","data":{"groups":[{"name":"g1","labels":{"foo":"bar"}}]}}`,
	})
	Init([]string{s.URL}, nil)

	statusCode, m := doRequest(t, "/api/v1/rules")
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code; got %d; want %d", statusCode, http.StatusOK)
	}
	// A single vmalert must be proxied as is, without adding SourceLabel.
	groups := m["data"].(map[string]any)["groups"].([]any)
	labels := groups[0].(map[string]any)["labels"].(map[string]any)
	if _, ok := labels[SourceLabel]; ok {
		t.Fatalf("unexpected %s label in a single-backend response: %v", SourceLabel, labels)
	}
}

func TestHandleRequestFanOutRules(t *testing.T) {
	s1 := newVMAlertMock(t, map[string]string{
		"/api/v1/rules": `{"status":"success","total_groups":2,"total_rules":5,"total_pages":1,"page":1,` +
			`"data":{"groups":[{"name":"g1","labels":{"foo":"bar"}},{"name":"g2"}]}}`,
	})
	s2 := newVMAlertMock(t, map[string]string{
		"/api/v1/rules": `{"status":"success","total_groups":1,"total_rules":3,"total_pages":2,"page":1,` +
			`"data":{"groups":[{"name":"g3"}]}}`,
	})
	Init([]string{s1.URL, s2.URL}, nil)

	statusCode, m := doRequest(t, "/api/v1/rules")
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code; got %d; want %d", statusCode, http.StatusOK)
	}
	got := groupNamesWithSource(t, m)
	want := []string{"g1/vmalert_proxy_1", "g2/vmalert_proxy_1", "g3/vmalert_proxy_2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected groups\ngot\n%v\nwant\n%v", got, want)
	}
	// Existing labels must be preserved.
	g1 := m["data"].(map[string]any)["groups"].([]any)[0].(map[string]any)
	if g1["labels"].(map[string]any)["foo"] != "bar" {
		t.Fatalf("existing labels must be preserved; got %v", g1["labels"])
	}
	if got, want := m["total_groups"], float64(3); got != want {
		t.Fatalf("unexpected total_groups; got %v; want %v", got, want)
	}
	if got, want := m["total_rules"], float64(8); got != want {
		t.Fatalf("unexpected total_rules; got %v; want %v", got, want)
	}
	if got, want := m["total_pages"], float64(2); got != want {
		t.Fatalf("unexpected total_pages; got %v; want %v", got, want)
	}
	if n := warnings(t, m); n != 0 {
		t.Fatalf("unexpected warnings: %v", m["warnings"])
	}
}

func TestHandleRequestFanOutPartialFailure(t *testing.T) {
	s1 := newVMAlertMock(t, map[string]string{
		"/api/v1/rules": `{"status":"success","data":{"groups":[{"name":"g1"}]}}`,
	})
	// s2 is down.
	s2 := newVMAlertMock(t, nil)
	s2URL := s2.URL
	s2.Close()
	Init([]string{s1.URL, s2URL}, []string{"east", "west"})

	statusCode, m := doRequest(t, "/api/v1/rules")
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code; got %d; want %d", statusCode, http.StatusOK)
	}
	got := groupNamesWithSource(t, m)
	want := []string{"g1/east"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected groups\ngot\n%v\nwant\n%v", got, want)
	}
	if n := warnings(t, m); n != 1 {
		t.Fatalf("unexpected number of warnings; got %d; want 1; warnings: %v", n, m["warnings"])
	}
}

func TestHandleRequestFanOutAllBackendsAreDown(t *testing.T) {
	s1 := newVMAlertMock(t, nil)
	s1URL := s1.URL
	s1.Close()
	s2 := newVMAlertMock(t, nil)
	s2URL := s2.URL
	s2.Close()
	Init([]string{s1URL, s2URL}, nil)

	statusCode, m := doRequest(t, "/api/v1/rules")
	if statusCode != http.StatusBadGateway {
		t.Fatalf("unexpected status code; got %d; want %d", statusCode, http.StatusBadGateway)
	}
	if m["status"] != "error" {
		t.Fatalf("unexpected status; got %v; want error", m["status"])
	}
	if n := warnings(t, m); n != 2 {
		t.Fatalf("unexpected number of warnings; got %d; want 2; warnings: %v", n, m["warnings"])
	}
}

func TestHandleRequestFanOutAlerts(t *testing.T) {
	s1 := newVMAlertMock(t, map[string]string{
		"/api/v1/alerts": `{"status":"success","data":{"alerts":[{"id":"1","labels":{"alertname":"a1"}}]}}`,
	})
	s2 := newVMAlertMock(t, map[string]string{
		"/api/v1/alerts": `{"status":"success","data":{"alerts":[{"id":"2","labels":{"alertname":"a2"}}]}}`,
	})
	Init([]string{s1.URL, s2.URL}, nil)

	_, m := doRequest(t, "/api/v1/alerts")
	alerts := m["data"].(map[string]any)["alerts"].([]any)
	if len(alerts) != 2 {
		t.Fatalf("unexpected number of alerts; got %d; want 2", len(alerts))
	}
	for i, want := range []string{"vmalert_proxy_1", "vmalert_proxy_2"} {
		labels := alerts[i].(map[string]any)["labels"].(map[string]any)
		if labels[SourceLabel] != want {
			t.Fatalf("unexpected %s at alert #%d; got %v; want %q", SourceLabel, i, labels[SourceLabel], want)
		}
	}
}

func TestHandleRequestFanOutNotifiers(t *testing.T) {
	s1 := newVMAlertMock(t, map[string]string{
		"/api/v1/notifiers": `{"status":"success","data":{"notifiers":[{"kind":"static","targets":[{"address":"http://am1"}]}]}}`,
	})
	s2 := newVMAlertMock(t, map[string]string{
		"/api/v1/notifiers": `{"status":"success","data":{"notifiers":[{"kind":"static","targets":[{"address":"http://am2","labels":{"foo":"bar"}}]}]}}`,
	})
	Init([]string{s1.URL, s2.URL}, nil)

	_, m := doRequest(t, "/api/v1/notifiers")
	notifiers := m["data"].(map[string]any)["notifiers"].([]any)
	if len(notifiers) != 2 {
		t.Fatalf("unexpected number of notifiers; got %d; want 2", len(notifiers))
	}
	// Notifiers have no labels of their own - SourceLabel is added to their targets.
	for i, want := range []string{"vmalert_proxy_1", "vmalert_proxy_2"} {
		targets := notifiers[i].(map[string]any)["targets"].([]any)
		labels := targets[0].(map[string]any)["labels"].(map[string]any)
		if labels[SourceLabel] != want {
			t.Fatalf("unexpected %s at notifier #%d; got %v; want %q", SourceLabel, i, labels[SourceLabel], want)
		}
	}
	if v := notifiers[1].(map[string]any)["targets"].([]any)[0].(map[string]any)["labels"].(map[string]any)["foo"]; v != "bar" {
		t.Fatalf("existing target labels must be preserved; got %v", v)
	}
}

// Entity IDs are generated by every vmalert independently and vmalerts with identical
// rule files generate identical IDs, so the entity cannot be located by ID alone.
// The request must name the vmalert instead of being fanned out.
func TestHandleRequestSingleEntityWithoutSourceQueryArgIsRejected(t *testing.T) {
	var requested int
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requested++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"g1","id":"123"}`)
	})
	s1 := httptest.NewServer(handler)
	defer s1.Close()
	s2 := httptest.NewServer(handler)
	defer s2.Close()
	Init([]string{s1.URL, s2.URL}, []string{"east", "west"})

	for _, path := range []string{"/api/v1/group", "/api/v1/rule", "/api/v1/alert", "/vmalert/api/v1/group"} {
		statusCode, m := doRequest(t, path+"?group_id=123")
		if statusCode != http.StatusBadRequest {
			t.Fatalf("unexpected status code for %q; got %d; want %d", path, statusCode, http.StatusBadRequest)
		}
		if m["status"] != "error" {
			t.Fatalf("unexpected status for %q; got %v; want error", path, m["status"])
		}
		// The error must name the arg and the configured vmalerts, so the caller can fix the request.
		errMsg, _ := m["error"].(string)
		for _, want := range []string{SourceQueryArg, "east", "west"} {
			if !strings.Contains(errMsg, want) {
				t.Fatalf("error for %q must mention %q; got %q", path, want, errMsg)
			}
		}
	}
	// No vmalert must be contacted at all - the request is rejected before any fan-out.
	if requested != 0 {
		t.Fatalf("no vmalert must be requested; got %d requests", requested)
	}
}

// A single configured vmalert needs no source, since there is nothing to disambiguate.
func TestHandleRequestSingleEntitySingleBackendNeedsNoSource(t *testing.T) {
	s := newVMAlertMock(t, map[string]string{
		"/api/v1/group": `{"name":"g1","id":"123"}`,
	})
	Init([]string{s.URL}, nil)

	statusCode, group := doRequest(t, "/api/v1/group?group_id=123")
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code; got %d; want %d", statusCode, http.StatusOK)
	}
	if group["name"] != "g1" {
		t.Fatalf("unexpected group; got %v", group)
	}
}

// vmui passes the group's __vmalert_source label as the vmalert_source query arg,
// so entity details are requested from the owning vmalert only.
func TestHandleRequestSingleEntityWithSourceQueryArgSkipsOtherBackends(t *testing.T) {
	var requested1, requested2 int
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requested1++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"error","errorType":404,"error":"group not found"}`)
	}))
	defer s1.Close()
	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requested2++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"g1","id":"639165069573691079"}`)
	}))
	defer s2.Close()
	Init([]string{s1.URL, s2.URL}, []string{"east", "west"})

	statusCode, group := doRequest(t, "/vmalert/api/v1/group?group_id=639165069573691079&"+SourceQueryArg+"=west")
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code; got %d; want %d", statusCode, http.StatusOK)
	}
	if group["name"] != "g1" {
		t.Fatalf("unexpected group; got %v", group)
	}
	// The vmalert, which does not own the group, must not be requested at all.
	if requested1 != 0 {
		t.Fatalf("the non-owning vmalert must not be requested; got %d requests", requested1)
	}
	if requested2 != 1 {
		t.Fatalf("unexpected number of requests to the owning vmalert; got %d; want 1", requested2)
	}
}

// httputil.ReverseProxy responds with an empty body by default, which clients cannot parse.
// Selecting an unreachable source must report it instead.
func TestHandleRequestUnreachableSelectedSource(t *testing.T) {
	s1 := newVMAlertMock(t, map[string]string{
		"/api/v1/rules": `{"status":"success","data":{"groups":[{"name":"g1"}]}}`,
	})
	s2 := newVMAlertMock(t, nil)
	s2URL := s2.URL
	s2.Close()
	Init([]string{s1.URL, s2URL}, []string{"east", "west"})

	// The proxied path and the fan-out path must both report the unreachable vmalert.
	for _, path := range []string{"/api/v1/rules", "/vmalert/api/v1/group"} {
		r := httptest.NewRequest(http.MethodGet, path+"?"+SourceQueryArg+"=west", nil)
		w := httptest.NewRecorder()
		HandleRequest(w, r, r.URL.Path)

		resp := w.Result()
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("cannot read response for %q: %s", path, err)
		}
		if len(body) == 0 {
			t.Fatalf("the response body for %q must not be empty, otherwise clients fail to parse it", path)
		}
		if resp.StatusCode != http.StatusBadGateway {
			t.Fatalf("unexpected status code for %q; got %d; want %d", path, resp.StatusCode, http.StatusBadGateway)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("unexpected Content-Type for %q; got %q; want application/json", path, ct)
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("cannot parse response for %q: %s; response: %s", path, err, body)
		}
		if m["status"] != "error" {
			t.Fatalf("unexpected status for %q; got %v; want error", path, m["status"])
		}
		errMsg, _ := m["error"].(string)
		if !strings.Contains(errMsg, "west") {
			t.Fatalf("the error for %q must name the unreachable vmalert; got %q", path, errMsg)
		}
	}
}

// vmalert may be reachable and still return an error with an empty body - e.g. when a proxy
// in front of it is up while vmalert itself is down. Such a response must not reach the client
// as is, since it cannot be parsed as JSON.
func TestHandleRequestEmptyErrorResponseFromBackend(t *testing.T) {
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer empty.Close()
	ok := newVMAlertMock(t, map[string]string{
		"/api/v1/rules": `{"status":"success","data":{"groups":[{"name":"g1"}]}}`,
	})
	Init([]string{ok.URL, empty.URL}, []string{"east", "poc"})

	// The proxied path must replace the empty body with a JSON error.
	r := httptest.NewRequest(http.MethodGet, "/api/v1/rules?"+SourceQueryArg+"=poc", nil)
	w := httptest.NewRecorder()
	HandleRequest(w, r, r.URL.Path)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if len(body) == 0 {
		t.Fatalf("the response body must not be empty, otherwise clients fail to parse it")
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("cannot parse response: %s; response: %s", err, body)
	}
	if m["status"] != "error" {
		t.Fatalf("unexpected status; got %v; want error", m["status"])
	}
	if errMsg, _ := m["error"].(string); !strings.Contains(errMsg, "poc") {
		t.Fatalf("the error must name the failing vmalert; got %q", errMsg)
	}

	// The fan-out path must report it as a warning without hiding the reason
	// behind a JSON parse failure.
	_, m = doRequest(t, "/api/v1/rules")
	if n := warnings(t, m); n != 1 {
		t.Fatalf("unexpected number of warnings; got %d; want 1; warnings: %v", n, m["warnings"])
	}
	warning := m["warnings"].([]any)[0].(string)
	for _, want := range []string{"poc", "502", "empty response"} {
		if !strings.Contains(warning, want) {
			t.Fatalf("the warning must mention %q; got %q", want, warning)
		}
	}
	if strings.Contains(warning, "unexpected end of JSON input") {
		t.Fatalf("the warning must not bury the reason behind a JSON parse failure; got %q", warning)
	}
	// The reachable vmalert still returns its groups.
	if got := groupNamesWithSource(t, m); !reflect.DeepEqual(got, []string{"g1/east"}) {
		t.Fatalf("unexpected groups; got %v", got)
	}
}

func TestHandleRequestSourceQueryArg(t *testing.T) {
	s1 := newVMAlertMock(t, map[string]string{
		"/api/v1/rules": `{"status":"success","data":{"groups":[{"name":"g1"}]}}`,
	})
	s2 := newVMAlertMock(t, map[string]string{
		"/api/v1/rules": `{"status":"success","data":{"groups":[{"name":"g2"}]}}`,
	})
	Init([]string{s1.URL, s2.URL}, []string{"east", "west"})

	// The request must be routed to `west` only and proxied as is.
	statusCode, m := doRequest(t, "/api/v1/rules?"+SourceQueryArg+"=west")
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code; got %d; want %d", statusCode, http.StatusOK)
	}
	groups := m["data"].(map[string]any)["groups"].([]any)
	if len(groups) != 1 || groups[0].(map[string]any)["name"] != "g2" {
		t.Fatalf("unexpected groups; got %v", groups)
	}

	// Unknown source must be reported as an error.
	statusCode, m = doRequest(t, "/api/v1/rules?"+SourceQueryArg+"=north")
	if statusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status code; got %d; want %d", statusCode, http.StatusBadRequest)
	}
	if m["status"] != "error" {
		t.Fatalf("unexpected status; got %v; want error", m["status"])
	}
}

func TestHandleRequestNonMergeablePathGoesToTheFirstBackend(t *testing.T) {
	s1 := newVMAlertMock(t, map[string]string{
		"/vmalert/groups": `first`,
	})
	s2 := newVMAlertMock(t, map[string]string{
		"/vmalert/groups": `second`,
	})
	Init([]string{s1.URL, s2.URL}, nil)

	r := httptest.NewRequest(http.MethodGet, "/vmalert/groups", nil)
	w := httptest.NewRecorder()
	HandleRequest(w, r, r.URL.Path)
	if got := w.Body.String(); got != "first" {
		t.Fatalf("unexpected body; got %q; want %q", got, "first")
	}
}

func TestHandleRequestVMUIPathsAreFannedOut(t *testing.T) {
	s1 := newVMAlertMock(t, map[string]string{
		"/vmalert/api/v1/rules": `{"status":"success","data":{"groups":[{"name":"g1"}]}}`,
	})
	s2 := newVMAlertMock(t, map[string]string{
		"/vmalert/api/v1/rules": `{"status":"success","data":{"groups":[{"name":"g2"}]}}`,
	})
	Init([]string{s1.URL, s2.URL}, nil)

	_, m := doRequest(t, "/vmalert/api/v1/rules")
	got := groupNamesWithSource(t, m)
	want := []string{"g1/vmalert_proxy_1", "g2/vmalert_proxy_2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected groups\ngot\n%v\nwant\n%v", got, want)
	}
}

func TestHandleRequestQueryArgsArePassedToBackends(t *testing.T) {
	var gotArgs []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotArgs = append(gotArgs, r.URL.RawQuery)
		fmt.Fprint(w, `{"status":"success","data":{"groups":[]}}`)
	}))
	defer s.Close()
	// The auth key from the proxy URL must be preserved.
	Init([]string{s.URL + "?authKey=secret", s.URL + "?authKey=secret"}, nil)

	_, _ = doRequest(t, "/api/v1/rules?type=alert&"+SourceQueryArg+"=vmalert_proxy_1")
	if len(gotArgs) != 1 {
		t.Fatalf("unexpected number of requests; got %d; want 1", len(gotArgs))
	}
	// SourceQueryArg must be stripped, while the rest of args must be passed to vmalert.
	if got, want := gotArgs[0], "authKey=secret&type=alert"; got != want {
		t.Fatalf("unexpected query args; got %q; want %q", got, want)
	}
}

// vmalert gzips the response if the request contains `Accept-Encoding: gzip`.
// The fan-out request must not forward this header, so http.Transport decompresses
// the response transparently.
func TestHandleRequestGzippedBackendResponse(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := `{"status":"success","data":{"groups":[{"name":"g1"}]}}`
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			fmt.Fprint(w, body)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		fmt.Fprint(zw, body)
		_ = zw.Close()
	}))
	defer s.Close()
	Init([]string{s.URL, s.URL}, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/rules", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	HandleRequest(w, r, r.URL.Path)

	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("cannot parse response: %s", err)
	}
	if n := warnings(t, m); n != 0 {
		t.Fatalf("unexpected warnings: %v", m["warnings"])
	}
	got := groupNamesWithSource(t, m)
	want := []string{"g1/vmalert_proxy_1", "g1/vmalert_proxy_2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected groups\ngot\n%v\nwant\n%v", got, want)
	}
}

// The fan-out request must carry the same headers as the request proxied by
// httputil.ReverseProxy on the single-backend path.
func TestHandleRequestForwardedHeaders(t *testing.T) {
	var got http.Header
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		fmt.Fprint(w, `{"status":"success","data":{"groups":[]}}`)
	}))
	defer s.Close()
	Init([]string{s.URL, s.URL}, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/rules", nil)
	r.Header.Set("Authorization", "Bearer secret")
	r.Header.Set("X-Scope-OrgID", "42")
	// Hop-by-hop headers, and a header named by Connection, must not reach vmalert.
	r.Header.Set("Connection", "X-Custom-Hop, Keep-Alive")
	r.Header.Set("X-Custom-Hop", "must be dropped")
	r.Header.Set("Keep-Alive", "timeout=5")
	r.Header.Set("Proxy-Authorization", "Basic Zm9v")
	// The fan-out request has no body and must let the transport pick the encoding.
	r.Header.Set("Content-Length", "123")
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	HandleRequest(w, r, r.URL.Path)

	// Client headers vmalert may need for authorization must be preserved.
	if v := got.Get("Authorization"); v != "Bearer secret" {
		t.Fatalf("Authorization must be forwarded; got %q", v)
	}
	if v := got.Get("X-Scope-OrgID"); v != "42" {
		t.Fatalf("X-Scope-OrgID must be forwarded; got %q", v)
	}
	for _, key := range []string{"Connection", "X-Custom-Hop", "Keep-Alive", "Proxy-Authorization", "Content-Length"} {
		if v := got.Get(key); v != "" {
			t.Fatalf("%s must not be forwarded; got %q", key, v)
		}
	}
	// http.Transport sets its own Accept-Encoding, so it must not be the forwarded one.
	if got.Get("Accept-Encoding") != "gzip" {
		t.Fatalf("http.Transport must set Accept-Encoding on its own; got %q", got.Get("Accept-Encoding"))
	}
}

func TestMarkSourceKeepsMalformedEntityAsIs(t *testing.T) {
	e := json.RawMessage(`"not an object"`)
	if got := string(markSource(e, "foo", nil)); got != string(e) {
		t.Fatalf("unexpected result; got %s; want %s", got, e)
	}
}
