package tests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestSingleVMAuthRouterWithAuth(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	var authorizedRequestsCount, unauthorizedRequestsCount int
	backendWithAuth := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		authorizedRequestsCount++
	}))
	defer backendWithAuth.Close()
	backend := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		unauthorizedRequestsCount++
	}))
	defer backend.Close()

	authConfig := fmt.Sprintf(`
users:
- name: user1
  username: ba-username
  password: ba-password
  url_prefix: %s
unauthorized_user:
   url_map:
   - src_paths:
     - /backend/health
     - /backend/ready
     url_prefix: %s
  `, backendWithAuth.URL, backend.URL)

	vmauth := tc.MustStartVmauth("vmauth", nil, authConfig)

	makeGetRequestExpectCode := func(prepareRequest func(*http.Request), expectCode int) {
		t.Helper()
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", vmauth.GetHTTPListenAddr()), nil)
		if err != nil {
			t.Fatalf("cannot build http.Request: %s", err)
		}
		prepareRequest(req)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("cannot make http.Get request for target=%q: %s", req.URL, err)
		}
		responseText, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("cannot read response body: %s", err)
		}
		resp.Body.Close()
		if resp.StatusCode != expectCode {
			t.Fatalf("unexpected http response code: %d, want: %d, response text: %s", resp.StatusCode, expectCode, responseText)
		}
	}
	assertBackendsRequestsCount := func(expectAuthorized, expectUnauthorized int) {
		t.Helper()
		if expectAuthorized != authorizedRequestsCount {
			t.Fatalf("expected to have %d authorized proxied requests, got: %d", expectAuthorized, authorizedRequestsCount)
		}

		if expectUnauthorized != unauthorizedRequestsCount {
			t.Fatalf("expected to have %d unauthorized proxied requests, got: %d", expectUnauthorized, unauthorizedRequestsCount)
		}

	}

	makeGetRequestExpectCode(func(r *http.Request) {
		r.URL.Path = "/backend/api"
		r.URL.User = url.UserPassword("ba-username", "ba-password")
	}, http.StatusOK)
	assertBackendsRequestsCount(1, 0)

	makeGetRequestExpectCode(func(r *http.Request) {
		r.URL.Path = "/backend/health"
	}, http.StatusOK)
	assertBackendsRequestsCount(1, 1)

	// remove unauthorized section and proxy only specified path for authorized
	vmauth.UpdateConfiguration(t, fmt.Sprintf(`
users:
- name: user1
  username: ba-username
  password: ba-password
  url_map:
  - src_paths:
    - /backend/health
    url_prefix: %s
`, backendWithAuth.URL))

	// ensure unauthorized requests no longer served
	makeGetRequestExpectCode(func(r *http.Request) {
		r.URL.Path = "/backend/health"
	}, http.StatusUnauthorized)
	assertBackendsRequestsCount(1, 1)

	makeGetRequestExpectCode(func(r *http.Request) {
		r.URL.User = url.UserPassword("ba-username", "ba-password")
		r.URL.Path = "/backend/health"
	}, http.StatusOK)
	assertBackendsRequestsCount(2, 1)

	// url path is missing at proxy configuration
	makeGetRequestExpectCode(func(r *http.Request) {
		r.URL.User = url.UserPassword("ba-username", "ba-password")
		r.URL.Path = "/backend"
	}, http.StatusBadRequest)
	assertBackendsRequestsCount(2, 1)

}

func TestSingleVMAuthRouterWithInternalAddr(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	var proxiedRequestsCount int
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		proxiedRequestsCount++
	}))
	defer backend.Close()

	authConfig := fmt.Sprintf(`
unauthorized_user:
   url_prefix: %s
  `, backend.URL)

	const (
		// it's not possible to use random ports
		// since it makes test flaky
		listenPortPublic  = "50127"
		listenPortPrivate = "50126"
	)

	vmauthFlags := []string{
		fmt.Sprintf("-httpListenAddr=127.0.0.1:%s", listenPortPublic),
		fmt.Sprintf("-httpInternalListenAddr=127.0.0.1:%s", listenPortPrivate),
		"-flagsAuthKey=protected",
	}
	vmauth := tc.MustStartVmauth("vmauth", vmauthFlags, authConfig)

	makeGetRequestExpectCode := func(targetURL string, expectCode int) {
		t.Helper()
		resp, err := http.Get(targetURL)
		if err != nil {
			t.Fatalf("cannot make http.Get request for target=%q: %s", targetURL, err)
		}
		responseText, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("cannot read response body: %s", err)
		}
		resp.Body.Close()
		if resp.StatusCode != expectCode {
			t.Fatalf("unexpected http response code: %d, want: %d, response text: %s", resp.StatusCode, expectCode, responseText)
		}
	}
	assertBackendRequestsCount := func(expected int) {
		t.Helper()
		if proxiedRequestsCount != expected {
			t.Fatalf("expected to have %d proxied requests, got: %d", expected, proxiedRequestsCount)
		}
	}
	// built-in http server must reject request, since it protected with authKey
	makeGetRequestExpectCode(fmt.Sprintf("http://127.0.0.1:%s/flags", listenPortPrivate), http.StatusUnauthorized)
	assertBackendRequestsCount(0)

	makeGetRequestExpectCode(fmt.Sprintf("http://127.0.0.1:%s/flags", listenPortPublic), http.StatusOK)
	assertBackendRequestsCount(1)

	// reload config and ensure that vmauth no longer proxies requests to the backend
	vmauth.UpdateConfiguration(t, "")
	makeGetRequestExpectCode(fmt.Sprintf("http://127.0.0.1:%s/flags", listenPortPrivate), http.StatusUnauthorized)
	assertBackendRequestsCount(1)
}
