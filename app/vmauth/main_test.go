package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

func TestRequestHandler(t *testing.T) {
	f := func(cfgStr, requestURL string, backendHandler http.HandlerFunc, responseExpected string) {
		t.Helper()

		ts := httptest.NewServer(backendHandler)
		defer ts.Close()

		cfgStr = strings.ReplaceAll(cfgStr, "{BACKEND}", ts.URL)
		responseExpected = strings.ReplaceAll(responseExpected, "{BACKEND}", ts.URL)

		cfgOrigP := authConfigData.Load()
		if _, err := reloadAuthConfigData([]byte(cfgStr)); err != nil {
			t.Fatalf("cannot load config data: %s", err)
		}
		defer func() {
			cfgOrig := []byte("unauthorized_user:\n  url_prefix: http://foo/bar")
			if cfgOrigP != nil {
				cfgOrig = *cfgOrigP
			}
			_, err := reloadAuthConfigData(cfgOrig)
			if err != nil {
				t.Fatalf("cannot load the original config: %s", err)
			}
		}()

		r, err := http.NewRequest(http.MethodGet, requestURL, nil)
		if err != nil {
			t.Fatalf("cannot initialize http request: %s", err)
		}

		r.RequestURI = r.URL.RequestURI()
		r.RemoteAddr = "42.2.3.84:6789"
		r.Header.Set("X-Forwarded-For", "12.34.56.78")
		r.Header.Set("Connection", "Some-Header,Other-Header")
		r.Header.Set("Some-Header", "foobar")
		r.Header.Set("Pass-Header", "abc")

		w := &fakeResponseWriter{}
		if !requestHandlerWithInternalRoutes(w, r) {
			t.Fatalf("unexpected false is returned from requestHandler")
		}

		response := w.getResponse()
		response = strings.ReplaceAll(response, "\r\n", "\n")
		response = strings.TrimSpace(response)
		responseExpected = strings.TrimSpace(responseExpected)
		if response != responseExpected {
			t.Fatalf("unexpected response\ngot\n%s\nwant\n%s", response, responseExpected)
		}
	}

	// regular url_prefix
	cfgStr := `
unauthorized_user:
  url_prefix: {BACKEND}/foo?bar=baz`
	requestURL := "http://some-host.com/abc/def?some_arg=some_value"
	backendHandler := func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Connection", "close")
		h.Set("Foo", "bar")

		var bb bytes.Buffer
		if err := r.Header.Write(&bb); err != nil {
			panic(fmt.Errorf("unexpected error when marshaling headers: %w", err))
		}
		fmt.Fprintf(w, "requested_url=http://%s%s\n%s", r.Host, r.URL, bb.String())
	}
	responseExpected := `
statusCode=200
Foo: bar
requested_url={BACKEND}/foo/abc/def?bar=baz&some_arg=some_value
Pass-Header: abc
User-Agent: vmauth
X-Forwarded-For: 12.34.56.78, 42.2.3.84`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// routing of all failed to authorize requests to unauthorized_user (issue #7543)
	cfgStr = `
unauthorized_user:
  url_prefix: "{BACKEND}/foo"
  keep_original_host: true`
	requestURL = "http://foo:invalid-secret@some-host.com/abc/def"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url=http://some-host.com/foo/abc/def`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// keep_original_host
	cfgStr = `
unauthorized_user:
  url_prefix: "{BACKEND}/foo?bar=baz"
  keep_original_host: true`
	requestURL = "http://some-host.com/abc/def"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url=http://some-host.com/foo/abc/def?bar=baz`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// override user-agent header
	cfgStr = `
unauthorized_user:
  url_prefix: "{BACKEND}/foo?bar=baz"
  headers:
  - "User-Agent: foobar"`
	requestURL = "http://some-host.com/abc/def"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s\nUser-Agent=%s", r.Host, r.URL, r.Header.Get("User-Agent"))
	}
	responseExpected = `
statusCode=200
requested_url={BACKEND}/foo/abc/def?bar=baz
User-Agent=foobar`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// delete user-agent header
	cfgStr = `
unauthorized_user:
  url_prefix: "{BACKEND}/foo?bar=baz"
  headers:
  - "User-Agent:"`
	requestURL = "http://some-host.com/abc/def"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s\nUser-Agent=%s", r.Host, r.URL, r.Header.Get("User-Agent"))
	}
	responseExpected = `
statusCode=200
requested_url={BACKEND}/foo/abc/def?bar=baz
User-Agent=Go-http-client/1.1`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// override request host with non-empty host
	cfgStr = `
unauthorized_user:
  url_prefix: "{BACKEND}/foo?bar=baz"
  headers:
  - "Host: other-host:12345"
  - "abc:"`
	requestURL = "http://some-host.com/abc/def"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url=http://other-host:12345/foo/abc/def?bar=baz`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// override request host with empty host
	cfgStr = `
unauthorized_user:
  url_prefix: "{BACKEND}/foo?bar=baz"
  headers:
  - "Host:"`
	requestURL = "http://some-host.com/abc/def"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url={BACKEND}/foo/abc/def?bar=baz`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// /-/reload handler failure
	origAuthKey := reloadAuthKey.Get()
	if err := reloadAuthKey.Set("secret"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	cfgStr = `
unauthorized_user:
  url_prefix: "{BACKEND}/foo"`
	requestURL = "http://some-host.com/-/reload"
	backendHandler = func(_ http.ResponseWriter, _ *http.Request) {
		panic(fmt.Errorf("backend handler shouldn't be called"))
	}
	responseExpected = `
statusCode=401
Expected to receive non-empty authKey when -reloadAuthKey is set`
	f(cfgStr, requestURL, backendHandler, responseExpected)
	if err := reloadAuthKey.Set(origAuthKey); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// missing authorization
	cfgStr = `
users:
- username: foo
  url_prefix: "{BACKEND}/bar"`
	requestURL = "http://some-host.com/a/b"
	backendHandler = func(_ http.ResponseWriter, _ *http.Request) {
		panic(fmt.Errorf("backend handler shouldn't be called"))
	}
	responseExpected = `
statusCode=401
Www-Authenticate: Basic realm="Restricted"
missing 'Authorization' request header`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// incorrect authorization
	cfgStr = `
users:
- username: foo
  password: secret
  url_prefix: "{BACKEND}/bar"`
	requestURL = "http://foo:invalid-secret@some-host.com/a/b"
	backendHandler = func(_ http.ResponseWriter, _ *http.Request) {
		panic(fmt.Errorf("backend handler shouldn't be called"))
	}
	responseExpected = `
statusCode=401
Unauthorized`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// incorrect authorization with logging invalid auth tokens
	origLogInvalidAuthTokens := *logInvalidAuthTokens
	*logInvalidAuthTokens = true
	cfgStr = `
users:
- username: foo
  password: secret
  url_prefix: "{BACKEND}/bar"`
	requestURL = "http://foo:invalid-secret@some-host.com/a/b?c=d"
	backendHandler = func(_ http.ResponseWriter, _ *http.Request) {
		panic(fmt.Errorf("backend handler shouldn't be called"))
	}
	responseExpected = `
statusCode=401
cannot authorize request with auth tokens ["http_auth:Basic Zm9vOmludmFsaWQtc2VjcmV0"]`
	f(cfgStr, requestURL, backendHandler, responseExpected)
	*logInvalidAuthTokens = origLogInvalidAuthTokens

	// correct authorization
	cfgStr = `
users:
- username: foo
  password: secret
  url_prefix: "{BACKEND}/bar"`
	requestURL = "http://foo:secret@some-host.com/a/b"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url={BACKEND}/bar/a/b`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// verify how path cleanup works
	cfgStr = `
unauthorized_user:
  url_prefix: {BACKEND}/foo?bar=baz`
	requestURL = "http://some-host.com/../../a//.///bar/"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url={BACKEND}/foo/a/bar/?bar=baz`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// verify how path cleanup works for url without path
	cfgStr = `
unauthorized_user:
  url_prefix: {BACKEND}/foo?bar=baz`
	requestURL = "http://some-host.com/"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url={BACKEND}/foo?bar=baz`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// verify how path cleanup works for url without path if url_prefix path ends with /
	cfgStr = `
unauthorized_user:
  url_prefix: {BACKEND}/foo/?bar=baz`
	requestURL = "http://some-host.com/"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url={BACKEND}/foo/?bar=baz`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// verify how path cleanup works for url without path and the url_prefix without path prefix
	cfgStr = `
unauthorized_user:
  url_prefix: {BACKEND}/?bar=baz`
	requestURL = "http://some-host.com/"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url={BACKEND}/?bar=baz`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// verify routing to default_url
	cfgStr = `
unauthorized_user:
  url_map:
  - src_paths: ["/foo/.+"]
    url_prefix: {BACKEND}/x-foo/
  default_url: {BACKEND}/404.html`
	requestURL = "http://some-host.com/abc?de=fg"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url={BACKEND}/404.html?request_path=http%3A%2F%2Fsome-host.com%2Fabc%3Fde%3Dfg`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// verify routing to default url_prefix
	cfgStr = `
unauthorized_user:
  url_map:
  - src_paths: ["/foo/.+"]
    url_prefix: {BACKEND}/x-foo/
  url_prefix: {BACKEND}/default`
	requestURL = "http://some-host.com/abc?de=fg"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url={BACKEND}/default/abc?de=fg`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// missing default_url and default url_prefix for unauthorized user
	cfgStr = `
unauthorized_user:
  url_map:
  - src_paths: ["/foo/.+"]
    url_prefix: {BACKEND}/x-foo/`
	requestURL = "http://some-host.com/abc?de=fg"
	backendHandler = func(_ http.ResponseWriter, _ *http.Request) {
		panic(fmt.Errorf("backend handler shouldn't be called"))
	}
	responseExpected = `
statusCode=400
missing route for "http://some-host.com/abc?de=fg"`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// missing default_url and default url_prefix for unauthorized user with dump_request_on_errors enabled
	cfgStr = `
unauthorized_user:
  dump_request_on_errors: true
  url_map:
  - src_paths: ["/foo/.+"]
    url_prefix: {BACKEND}/x-foo/`
	requestURL = "http://some-host.com/abc?de=fg"
	backendHandler = func(_ http.ResponseWriter, _ *http.Request) {
		panic(fmt.Errorf("backend handler shouldn't be called"))
	}
	responseExpected = `
statusCode=400
missing route for "http://some-host.com/abc?de=fg" (host: "some-host.com"; path: "/abc"; args: "de=fg"; headers:Connection: Some-Header,Other-Header
Pass-Header: abc
Some-Header: foobar
X-Forwarded-For: 12.34.56.78
)`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// missing default_url and default url_prefix for unauthorized user when there are configs for authorized users
	cfgStr = `
users:
- username: some-user
  url_map:
  - src_paths: ["/foo/.+"]
    url_prefix: {BACKEND}/x-foo/
unauthorized_user:
  url_map:
  - src_paths: ["/abc/.*"]
    url_prefix: {BACKEND}/x-bar`
	requestURL = "http://some-host.com/abc?de=fg"
	backendHandler = func(_ http.ResponseWriter, _ *http.Request) {
		panic(fmt.Errorf("backend handler shouldn't be called"))
	}
	responseExpected = `
statusCode=401
Www-Authenticate: Basic realm="Restricted"
missing 'Authorization' request header`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// all the backend_urls are unavailable for unauthorized user
	cfgStr = `
unauthorized_user:
  url_map:
  - src_paths: ["/foo/.*"]
    url_prefix:
    - http://127.0.0.1:1/
    - http://127.0.0.1:2/`
	requestURL = "http://some-host.com/foo/?de=fg"
	backendHandler = func(_ http.ResponseWriter, _ *http.Request) {
		panic(fmt.Errorf("backend handler shouldn't be called"))
	}
	responseExpected = `
statusCode=502
all the 2 backends for the user "" are unavailable`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// all the backend_urls are unavailable for authorized user
	cfgStr = `
users:
- username: some-user
  url_map:
  - src_paths: ["/foo/.*"]
    url_prefix:
    - http://127.0.0.1:1/
    - http://127.0.0.1:2/`
	requestURL = "http://some-user@some-host.com/foo/?de=fg"
	backendHandler = func(_ http.ResponseWriter, _ *http.Request) {
		panic(fmt.Errorf("backend handler shouldn't be called"))
	}
	responseExpected = `
statusCode=502
all the 2 backends for the user "some-user" are unavailable`
	f(cfgStr, requestURL, backendHandler, responseExpected)

	// zero discovered backend IPs
	customResolver := &fakeResolver{
		Resolver: &net.Resolver{},
		lookupIPAddrResults: map[string][]net.IPAddr{
			"some-addr": {},
		},
	}
	origResolver := netutil.Resolver
	netutil.Resolver = customResolver
	cfgStr = `
unauthorized_user:
  url_prefix: ['http://some-addr:1234/foo/bar']
  discover_backend_ips: true`
	requestURL = "http://abc.com/def/?de=fg"
	backendHandler = func(_ http.ResponseWriter, _ *http.Request) {
		panic(fmt.Errorf("backend handler shouldn't be called"))
	}
	responseExpected = `
statusCode=502
all the 0 backends for the user "" are unavailable`
	f(cfgStr, requestURL, backendHandler, responseExpected)
	netutil.Resolver = origResolver

	// retry_status_codes failure
	var retries atomic.Int64
	cfgStr = `
unauthorized_user:
  url_prefix: ['{BACKEND}/path1', '{BACKEND}/path2']
  retry_status_codes: [500, 502]`
	requestURL = "http://some-host.com/foo/?de=fg"
	backendHandler = func(w http.ResponseWriter, _ *http.Request) {
		retries.Add(1)
		w.WriteHeader(500)
	}
	responseExpected = `
statusCode=502
all the 2 backends for the user "" are unavailable`
	f(cfgStr, requestURL, backendHandler, responseExpected)
	if n := retries.Load(); n != 2 {
		t.Fatalf("unexpected number of retries; got %d; want 2", n)
	}

	// retry_status_codes success
	retries.Store(0)
	cfgStr = `
unauthorized_user:
  url_prefix: ['{BACKEND}/path1', '{BACKEND}/path2']
  retry_status_codes: [500, 502]`
	requestURL = "http://some-host.com/foo/?de=fg"
	backendHandler = func(w http.ResponseWriter, r *http.Request) {
		if n := retries.Add(1); n < 2 {
			w.WriteHeader(500)
			return
		}
		fmt.Fprintf(w, "requested_url=http://%s%s", r.Host, r.URL)
	}
	responseExpected = `
statusCode=200
requested_url={BACKEND}/path2/foo/?de=fg`
	f(cfgStr, requestURL, backendHandler, responseExpected)
	if n := retries.Load(); n != 2 {
		t.Fatalf("unexpected number of retries; got %d; want 2", n)
	}
}

type fakeResponseWriter struct {
	h http.Header

	bb bytes.Buffer
}

func (w *fakeResponseWriter) getResponse() string {
	return w.bb.String()
}

// Flush implements net/http.Flusher
func (w *fakeResponseWriter) Flush() {
	// Nothing to do.
}

func (w *fakeResponseWriter) Header() http.Header {
	if w.h == nil {
		w.h = http.Header{}
	}
	return w.h
}

func (w *fakeResponseWriter) Write(p []byte) (int, error) {
	return w.bb.Write(p)
}

func (w *fakeResponseWriter) WriteHeader(statusCode int) {
	fmt.Fprintf(&w.bb, "statusCode=%d\n", statusCode)
	if w.h == nil {
		return
	}
	err := w.h.WriteSubset(&w.bb, map[string]bool{
		"Content-Length":         true,
		"Content-Type":           true,
		"Date":                   true,
		"X-Content-Type-Options": true,
	})
	if err != nil {
		panic(fmt.Errorf("cannot marshal headers: %s", err))
	}
}

func TestReadTrackingBody_RetrySuccess(t *testing.T) {
	f := func(s string, maxBodySize int) {
		t.Helper()

		rtb := newReadTrackingBody(io.NopCloser(bytes.NewBufferString(s)), maxBodySize)

		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true before reading anything")
		}
		for i := 0; i < 5; i++ {
			data, err := io.ReadAll(rtb)
			if err != nil {
				t.Fatalf("unexpected error when reading all the data at iteration %d: %s", i, err)
			}
			if string(data) != s {
				t.Fatalf("unexpected data read at iteration %d\ngot\n%s\nwant\n%s", i, data, s)
			}
			if err := rtb.Close(); err != nil {
				t.Fatalf("unexpected error when closing readTrackingBody at iteration %d: %s", i, err)
			}
			if !rtb.canRetry() {
				t.Fatalf("canRetry() must return true at iteration %d", i)
			}
		}
	}

	f("", 0)
	f("", -1)
	f("", 100)
	f("foo", 100)
	f("foobar", 100)
	f(newTestString(1000), 1000)
}

func TestReadTrackingBody_RetrySuccessPartialRead(t *testing.T) {
	f := func(s string, maxBodySize int) {
		t.Helper()

		// Check the case with partial read
		rtb := newReadTrackingBody(io.NopCloser(bytes.NewBufferString(s)), maxBodySize)

		for i := 0; i < len(s); i++ {
			buf := make([]byte, i)
			n, err := io.ReadFull(rtb, buf)
			if err != nil {
				t.Fatalf("unexpected error when reading %d bytes: %s", i, err)
			}
			if n != i {
				t.Fatalf("unexpected number of bytes read; got %d; want %d", n, i)
			}
			if string(buf) != s[:i] {
				t.Fatalf("unexpected data read with the length %d\ngot\n%s\nwant\n%s", i, buf, s[:i])
			}
			if err := rtb.Close(); err != nil {
				t.Fatalf("unexpected error when closing reader after reading %d bytes", i)
			}
			if !rtb.canRetry() {
				t.Fatalf("canRetry() must return true after closing the reader after reading %d bytes", i)
			}
		}

		data, err := io.ReadAll(rtb)
		if err != nil {
			t.Fatalf("unexpected error when reading all the data: %s", err)
		}
		if string(data) != s {
			t.Fatalf("unexpected data read\ngot\n%s\nwant\n%s", data, s)
		}
		if err := rtb.Close(); err != nil {
			t.Fatalf("unexpected error when closing readTrackingBody: %s", err)
		}
		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true after closing the reader after reading all the input")
		}
	}

	f("", 0)
	f("", -1)
	f("", 100)
	f("foo", 100)
	f("foobar", 100)
	f(newTestString(1000), 1000)
}

func TestReadTrackingBody_RetryFailureTooBigBody(t *testing.T) {
	f := func(s string, maxBodySize int) {
		t.Helper()

		rtb := newReadTrackingBody(io.NopCloser(bytes.NewBufferString(s)), maxBodySize)

		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true before reading anything")
		}
		buf := make([]byte, 1)
		n, err := io.ReadFull(rtb, buf)
		if err != nil {
			t.Fatalf("unexpected error when reading a single byte: %s", err)
		}
		if n != 1 {
			t.Fatalf("unexpected number of bytes read; got %d; want 1", n)
		}
		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true after reading one byte")
		}
		data, err := io.ReadAll(rtb)
		if err != nil {
			t.Fatalf("unexpected error when reading all the data: %s", err)
		}
		dataRead := string(buf) + string(data)
		if dataRead != s {
			t.Fatalf("unexpected data read\ngot\n%s\nwant\n%s", dataRead, s)
		}
		if err := rtb.Close(); err != nil {
			t.Fatalf("unexpected error when closing readTrackingBody: %s", err)
		}
		if rtb.canRetry() {
			t.Fatalf("canRetry() must return false after closing the reader")
		}

		data, err = io.ReadAll(rtb)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if len(data) != 0 {
			t.Fatalf("unexpected non-empty data read: %q", data)
		}
	}

	const maxBodySize = 1000
	f(newTestString(maxBodySize+1), maxBodySize)
	f(newTestString(2*maxBodySize), maxBodySize)
}

func TestReadTrackingBody_RetryFailureZeroOrNegativeMaxBodySize(t *testing.T) {
	f := func(s string, maxBodySize int) {
		t.Helper()

		rtb := newReadTrackingBody(io.NopCloser(bytes.NewBufferString(s)), maxBodySize)

		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true before reading anything")
		}
		data, err := io.ReadAll(rtb)
		if err != nil {
			t.Fatalf("unexpected error when reading all the data: %s", err)
		}
		if string(data) != s {
			t.Fatalf("unexpected data read\ngot\n%s\nwant\n%s", data, s)
		}
		if err := rtb.Close(); err != nil {
			t.Fatalf("unexpected error when closing readTrackingBody: %s", err)
		}

		if rtb.canRetry() {
			t.Fatalf("canRetry() must return false after closing the reader")
		}
		data, err = io.ReadAll(rtb)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if len(data) != 0 {
			t.Fatalf("unexpected non-empty data read: %q", data)
		}
	}

	f("foobar", 0)
	f(newTestString(1000), 0)

	f("foobar", -1)
	f(newTestString(1000), -1)
}

func newTestString(sLen int) string {
	data := make([]byte, sLen)
	for i := range data {
		data[i] = byte(i)
	}
	return string(data)
}

func TestRequestBodyReadTimeout(t *testing.T) {
	f := func(cfgStr string, bodyReader io.Reader, expectTimeout bool, expectedStatus string) {
		t.Helper()

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Backend reads body and responds
			_, _ = io.ReadAll(r.Body)
			fmt.Fprintf(w, "backend_response")
		}))
		defer ts.Close()

		cfgStr = strings.ReplaceAll(cfgStr, "{BACKEND}", ts.URL)

		cfgOrigP := authConfigData.Load()
		if _, err := reloadAuthConfigData([]byte(cfgStr)); err != nil {
			t.Fatalf("cannot load config data: %s", err)
		}
		defer func() {
			cfgOrig := []byte("unauthorized_user:\n  url_prefix: http://foo/bar")
			if cfgOrigP != nil {
				cfgOrig = *cfgOrigP
			}
			_, err := reloadAuthConfigData(cfgOrig)
			if err != nil {
				t.Fatalf("cannot load the original config: %s", err)
			}
		}()

		r, err := http.NewRequest(http.MethodPost, "http://some-host.com/test", bodyReader)
		if err != nil {
			t.Fatalf("cannot initialize http request: %s", err)
		}

		r.RequestURI = r.URL.RequestURI()
		r.RemoteAddr = "1.2.3.4:5678"
		r.Header.Set("X-Forwarded-For", "10.20.30.40")

		w := &fakeResponseWriter{}
		if !requestHandlerWithInternalRoutes(w, r) {
			t.Fatalf("unexpected false is returned from requestHandler")
		}

		response := w.getResponse()

		if expectTimeout {
			if !strings.Contains(response, expectedStatus) {
				t.Fatalf("expected status %s in response, got: %s", expectedStatus, response)
			}
			if !strings.Contains(response, "request body read timeout") {
				t.Fatalf("expected timeout message in response: %s", response)
			}
		} else {
			if strings.Contains(response, "408") {
				t.Fatalf("unexpected timeout in response: %s", response)
			}
			if !strings.Contains(response, expectedStatus) {
				t.Fatalf("expected status %s, got: %s", expectedStatus, response)
			}
		}
	}

	// Test timeout triggers with slow body
	cfgStr := `
unauthorized_user:
  url_prefix: {BACKEND}/foo
  request_body_read_timeout: 100ms`
	slowBody := &slowReader{
		data:  []byte("slow test data"),
		delay: 50 * time.Millisecond,
	}
	f(cfgStr, slowBody, true, "statusCode=408")

	// Test timeout disabled (0 value)
	cfgStr = `
unauthorized_user:
  url_prefix: {BACKEND}/foo
  request_body_read_timeout: 0s`
	slowBody2 := &slowReader{
		data:  []byte("test"),
		delay: 50 * time.Millisecond,
	}
	f(cfgStr, slowBody2, false, "statusCode=200")

	// Test fast body - no timeout
	cfgStr = `
unauthorized_user:
  url_prefix: {BACKEND}/foo
  request_body_read_timeout: 5s`
	fastBody := bytes.NewReader([]byte("fast data"))
	f(cfgStr, fastBody, false, "statusCode=200")
}

func TestRequestBodyReadTimeoutPerUser(t *testing.T) {
	f := func(cfgStr, requestURL string, bodyReader io.Reader, expectTimeout bool) {
		t.Helper()

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.ReadAll(r.Body)
			fmt.Fprintf(w, "ok")
		}))
		defer ts.Close()

		cfgStr = strings.ReplaceAll(cfgStr, "{BACKEND}", ts.URL)

		cfgOrigP := authConfigData.Load()
		if _, err := reloadAuthConfigData([]byte(cfgStr)); err != nil {
			t.Fatalf("cannot load config data: %s", err)
		}
		defer func() {
			cfgOrig := []byte("unauthorized_user:\n  url_prefix: http://foo/bar")
			if cfgOrigP != nil {
				cfgOrig = *cfgOrigP
			}
			_, err := reloadAuthConfigData(cfgOrig)
			if err != nil {
				t.Fatalf("cannot load the original config: %s", err)
			}
		}()

		r, err := http.NewRequest(http.MethodPost, requestURL, bodyReader)
		if err != nil {
			t.Fatalf("cannot initialize http request: %s", err)
		}

		r.RequestURI = r.URL.RequestURI()
		r.RemoteAddr = "1.2.3.4:5678"

		w := &fakeResponseWriter{}
		requestHandlerWithInternalRoutes(w, r)

		response := w.getResponse()

		if expectTimeout {
			if !strings.Contains(response, "statusCode=408") {
				t.Fatalf("expected timeout (408), got: %s", response)
			}
		} else {
			if strings.Contains(response, "statusCode=408") {
				t.Fatalf("unexpected timeout, got: %s", response)
			}
		}
	}

	// User with short timeout - should timeout.
	// This also verifies per-user timeout overrides global -requestBodyReadTimeout (30s default).
	cfgStr := `
users:
- username: "fast-user"
  password: "secret1"
  url_prefix: {BACKEND}/foo
  request_body_read_timeout: 50ms`
	slowBody := &slowReader{
		data:  []byte("data"),
		delay: 30 * time.Millisecond,
	}
	requestURL := "http://fast-user:secret1@some-host.com/test"
	f(cfgStr, requestURL, slowBody, true)

	// User with long timeout - should NOT timeout
	cfgStr = `
users:
- username: "slow-user"
  password: "secret2"
  url_prefix: {BACKEND}/bar
  request_body_read_timeout: 5s`
	slowBody2 := &slowReader{
		data:  []byte("data"),
		delay: 30 * time.Millisecond,
	}
	requestURL = "http://slow-user:secret2@some-host.com/test"
	f(cfgStr, requestURL, slowBody2, false)
}

func TestReadTrackingBodyDuration(t *testing.T) {
	f := func(data []byte) {
		t.Helper()

		// Use slowReader which adds actual delays during Read() calls
		slowBody := &slowReader{
			data:  data,
			delay: 10 * time.Millisecond,
		}

		rtb := newReadTrackingBody(io.NopCloser(slowBody), len(data))

		// Read all data
		_, err := io.ReadAll(rtb)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		duration := rtb.getTotalReadDuration()

		// With slowReader reading byte-by-byte with 10ms delay,
		// duration should be at least (len(data) * 10ms)
		minExpected := time.Duration(len(data)) * 10 * time.Millisecond
		if duration < minExpected {
			t.Fatalf("duration too short: got %v, want >= %v", duration, minExpected)
		}

		// Should not be unreasonably long (allow 3x overhead for test variability)
		maxExpected := minExpected * 3
		if duration > maxExpected {
			t.Fatalf("duration too long: got %v, want <= %v", duration, maxExpected)
		}
	}

	// Test with small data to keep test fast
	f(make([]byte, 5))
	f(make([]byte, 10))
}

func TestTimeoutReadCloser(t *testing.T) {
	f := func(data []byte, timeout time.Duration, readDelay time.Duration, expectTimeout bool) {
		t.Helper()

		rc := io.NopCloser(bytes.NewReader(data))
		trc := newTimeoutReadCloser(rc, timeout)

		if readDelay > 0 {
			time.Sleep(readDelay)
		}

		buf := make([]byte, len(data))
		_, err := trc.Read(buf)

		if expectTimeout {
			if err == nil {
				t.Fatal("expected timeout error, got nil")
			}
			// Check if it's our timeout error type
			var timeoutErr *requestBodyTimeoutError
			if !errors.As(err, &timeoutErr) {
				t.Fatalf("expected requestBodyTimeoutError, got: %T: %v", err, err)
			}
		} else {
			if err != nil && err != io.EOF {
				t.Fatalf("unexpected error: %s", err)
			}
		}
	}

	data := []byte("test data")

	// Read before timeout - should succeed
	f(data, 100*time.Millisecond, 0, false)

	// Read after timeout - should fail
	f(data, 50*time.Millisecond, 100*time.Millisecond, true)
}

func TestTimeoutResponseWriter(t *testing.T) {
	f := func(timeout time.Duration, writeDelay time.Duration, expectTimeout bool) {
		t.Helper()

		w := &fakeResponseWriter{}
		tw := newTimeoutResponseWriter(w, timeout)

		if writeDelay > 0 {
			time.Sleep(writeDelay)
		}

		_, err := tw.Write([]byte("test data"))

		if expectTimeout {
			if err == nil {
				t.Fatal("expected timeout error, got nil")
			}
			var timeoutErr *responseWriteTimeoutError
			if !errors.As(err, &timeoutErr) {
				t.Fatalf("expected responseWriteTimeoutError, got: %T: %v", err, err)
			}
		} else {
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if !strings.Contains(w.bb.String(), "test data") {
				t.Fatal("data not written to underlying writer")
			}
		}
	}

	// Write before timeout - should succeed
	f(100*time.Millisecond, 0, false)

	// Write after timeout - should fail
	f(50*time.Millisecond, 100*time.Millisecond, true)
}

// slowReader simulates a slow client sending data byte-by-byte
type slowReader struct {
	data  []byte
	pos   int
	delay time.Duration
}

func (r *slowReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	// Simulate slow network by adding delay
	if r.delay > 0 {
		time.Sleep(r.delay)
	}

	// Read one byte at a time to simulate slow client
	n := 1
	if n > len(p) {
		n = len(p)
	}
	if r.pos+n > len(r.data) {
		n = len(r.data) - r.pos
	}

	copy(p, r.data[r.pos:r.pos+n])
	r.pos += n

	return n, nil
}
